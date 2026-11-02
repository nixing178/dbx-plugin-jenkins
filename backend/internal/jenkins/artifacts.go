package jenkins

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// maxArtifactChunk 是单次分块上限：base64 后约 5.33 MiB，低于 SDK 的 8 MiB JSON 帧上限。
	maxArtifactChunk = 4 << 20
	// artifactCacheBudget 是制品缓存目录的磁盘预算，超出按 mtime 从旧到新清理。
	artifactCacheBudget = 512 << 20
	// artifactDownloadTimeout 只约束从 Jenkins 拉取制品的这一次请求（普通请求仍是连接超时）。
	artifactDownloadTimeout = 10 * time.Minute
)

// ArtifactPath 把构建产物的相对路径转成 REST 路径（/artifact/...），逐段转义。
// 空段、"."、".."、反斜杠与绝对路径一律拒绝，避免拼出越界 URL。
func ArtifactPath(relativePath string) (string, error) {
	raw := strings.TrimSpace(relativePath)
	if raw == "" {
		return "", errors.New("artifact path is required")
	}
	if strings.Contains(raw, `\`) {
		return "", fmt.Errorf("invalid artifact path %q", relativePath)
	}
	if strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("artifact path must be relative: %q", relativePath)
	}

	segments := strings.Split(raw, "/")
	parts := make([]string, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("invalid artifact path segment %q", segment)
		}
		parts = append(parts, url.PathEscape(segment))
	}
	return "/artifact/" + strings.Join(parts, "/"), nil
}

// ArtifactCacheDir 返回制品缓存目录：插件数据目录下的 artifacts/。
func ArtifactCacheDir() string {
	base := strings.TrimSpace(os.Getenv("DBX_PLUGIN_DATA_DIR"))
	if base == "" {
		if dir, err := os.UserCacheDir(); err == nil {
			base = filepath.Join(dir, "dbx-jenkins")
		} else {
			base = os.TempDir()
		}
	}
	return filepath.Join(base, "artifacts")
}

// GetArtifactInfo 读取制品的大小与类型：优先 HEAD，拿不到长度时回退到下载后 stat。
func (c *Client) GetArtifactInfo(
	ctx context.Context,
	fullName string,
	number int,
	relativePath string,
) (*ArtifactInfo, error) {
	artifactPath, err := c.artifactURLPath(fullName, number, relativePath)
	if err != nil {
		return nil, err
	}

	response, requestErr := c.headArtifact(ctx, artifactPath)
	fileName := pathpkg.Base(relativePath)
	contentType := ""
	if requestErr == nil {
		contentType = strings.TrimSpace(response.Header.Get("Content-Type"))
		switch {
		case response.StatusCode < http.StatusBadRequest:
			if size, ok := parseContentLength(response.Header.Get("Content-Length")); ok && size > 0 {
				return &ArtifactInfo{
					FileName:    fileName,
					Size:        size,
					ContentType: contentType,
				}, nil
			}
		case response.StatusCode == http.StatusMethodNotAllowed ||
			response.StatusCode == http.StatusNotImplemented:
			// 反代 / 老版本 Jenkins 不支持 HEAD，落到下载分支。
		default:
			return nil, newAPIError(response.StatusCode, nil)
		}
	} else {
		return nil, requestErr
	}

	cached, err := c.prepareArtifact(ctx, fullName, number, relativePath)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(cached)
	if err != nil {
		return nil, &APIError{Kind: ErrRequest, Message: "cannot read cached artifact"}
	}
	return &ArtifactInfo{
		FileName:    fileName,
		Size:        stat.Size(),
		ContentType: contentType,
	}, nil
}

// GetArtifactChunk 读取制品的 [offset, offset+length) 段；首次调用会先把整份制品落盘缓存。
func (c *Client) GetArtifactChunk(
	ctx context.Context,
	fullName string,
	number int,
	relativePath string,
	offset int64,
	length int,
) (*ArtifactChunk, error) {
	cached, err := c.prepareArtifact(ctx, fullName, number, relativePath)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(cached)
	if err != nil {
		return nil, &APIError{Kind: ErrRequest, Message: "cannot read cached artifact"}
	}
	total := stat.Size()

	if offset < 0 {
		offset = 0
	}
	if length <= 0 || length > maxArtifactChunk {
		length = maxArtifactChunk
	}
	if offset >= total {
		return &ArtifactChunk{Offset: offset, Length: 0, Total: total, EOF: true}, nil
	}

	size := int64(length)
	if remaining := total - offset; size > remaining {
		size = remaining
	}
	buffer := make([]byte, size)
	file, openErr := os.Open(cached)
	if openErr != nil {
		return nil, &APIError{Kind: ErrRequest, Message: "cannot read cached artifact"}
	}
	defer file.Close()

	read, readErr := file.ReadAt(buffer, offset)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, &APIError{Kind: ErrRequest, Message: "cannot read cached artifact"}
	}
	buffer = buffer[:read]

	return &ArtifactChunk{
		Offset:     offset,
		Length:     read,
		Total:      total,
		EOF:        offset+int64(read) >= total,
		DataBase64: base64.StdEncoding.EncodeToString(buffer),
	}, nil
}

func (c *Client) headArtifact(ctx context.Context, artifactPath string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, c.resolve(artifactPath), nil)
	if err != nil {
		return nil, &APIError{Kind: ErrRequest, Message: err.Error()}
	}
	response, err := c.do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	return response, nil
}

// prepareArtifact 确保制品已落在本地缓存，返回缓存文件路径。
func (c *Client) prepareArtifact(
	ctx context.Context,
	fullName string,
	number int,
	relativePath string,
) (string, error) {
	return c.prepareArtifactWithProgress(ctx, fullName, number, relativePath, nil)
}

// prepareArtifactWithProgress 与 prepareArtifact 相同，额外按 256 KiB 上报下载进度。
func (c *Client) prepareArtifactWithProgress(
	ctx context.Context,
	fullName string,
	number int,
	relativePath string,
	onProgress func(done, total int64),
) (string, error) {
	dir := ArtifactCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", &APIError{Kind: ErrRequest, Message: "cannot create artifact cache directory"}
	}

	target := filepath.Join(dir, artifactCacheFile(fullName, number, relativePath))
	if info, err := os.Stat(target); err == nil && info.Mode().IsRegular() {
		// 命中缓存时刷新 mtime，避免同一制品被预算清理优先淘汰。
		_ = os.Chtimes(target, time.Now(), time.Now())
		return target, nil
	}

	artifactPath, err := c.artifactURLPath(fullName, number, relativePath)
	if err != nil {
		return "", err
	}

	unlock := c.lockArtifact(target)
	defer unlock()

	// 等锁期间可能已被其它请求下好。
	if info, err := os.Stat(target); err == nil && info.Mode().IsRegular() {
		return target, nil
	}

	downloadCtx, cancel := context.WithTimeout(ctx, artifactDownloadTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, c.resolve(artifactPath), nil)
	if err != nil {
		return "", &APIError{Kind: ErrRequest, Message: err.Error()}
	}
	request.Header.Set("Accept", "*/*")

	response, err := c.doWith(request, c.downloadClient)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 64*1024))
		return "", newAPIError(response.StatusCode, body)
	}

	temp, err := os.CreateTemp(dir, "artifact-*.part")
	if err != nil {
		return "", &APIError{Kind: ErrRequest, Message: "cannot create artifact cache file"}
	}
	tempName := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempName)
	}()

	total := response.ContentLength
	if total < 0 {
		total = 0
	}
	if onProgress != nil {
		onProgress(0, total)
	}
	source := &artifactProgressReader{
		reader: response.Body,
		total:  total,
		report: onProgress,
	}
	if _, err := io.Copy(temp, source); err != nil {
		return "", &APIError{Kind: ErrNetwork, Message: "cannot download Jenkins artifact"}
	}
	if err := temp.Sync(); err != nil {
		return "", &APIError{Kind: ErrRequest, Message: "cannot flush artifact cache"}
	}
	if err := temp.Close(); err != nil {
		return "", &APIError{Kind: ErrRequest, Message: "cannot flush artifact cache"}
	}
	if err := os.Rename(tempName, target); err != nil {
		return "", &APIError{Kind: ErrRequest, Message: "cannot store artifact in cache"}
	}

	// 清理失败不影响本次下载：下次下载前还会再试。
	_ = pruneArtifactCache(dir, artifactCacheBudget, target)
	return target, nil
}

func (c *Client) artifactURLPath(fullName string, number int, relativePath string) (string, error) {
	jobPath, err := JobPath(fullName)
	if err != nil {
		return "", &APIError{Kind: ErrInvalidConfig, Message: err.Error()}
	}
	if number <= 0 {
		return "", &APIError{Kind: ErrInvalidConfig, Message: "build number must be positive"}
	}
	artifactPath, err := ArtifactPath(relativePath)
	if err != nil {
		return "", &APIError{Kind: ErrInvalidConfig, Message: err.Error()}
	}
	return fmt.Sprintf("%s/%d%s", jobPath, number, artifactPath), nil
}

func (c *Client) lockArtifact(key string) func() {
	c.artifactMu.Lock()
	if c.artifactLocks == nil {
		c.artifactLocks = map[string]*sync.Mutex{}
	}
	lock, ok := c.artifactLocks[key]
	if !ok {
		lock = &sync.Mutex{}
		c.artifactLocks[key] = lock
	}
	c.artifactMu.Unlock()

	lock.Lock()
	return lock.Unlock
}

// artifactCacheFile 用 sha1 命名缓存文件，避免把任务名 / 路径直接落到文件名上。
func artifactCacheFile(fullName string, number int, relativePath string) string {
	sum := sha1.Sum([]byte(fullName + "\x00" + strconv.Itoa(number) + "\x00" + relativePath))
	return hex.EncodeToString(sum[:]) + ".bin"
}

func parseContentLength(raw string) (int64, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, false
	}
	size, err := strconv.ParseInt(value, 10, 64)
	if err != nil || size < 0 {
		return 0, false
	}
	return size, true
}

// pruneArtifactCache 把目录里的缓存压到预算内，keep 指向当前正在用的文件。
func pruneArtifactCache(dir string, budget int64, keep string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	type cachedFile struct {
		path string
		size int64
		mod  time.Time
	}
	files := make([]cachedFile, 0, len(entries))
	var total int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		files = append(files, cachedFile{
			path: filepath.Join(dir, entry.Name()),
			size: info.Size(),
			mod:  info.ModTime(),
		})
		total += info.Size()
	}
	if total <= budget {
		return nil
	}

	sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
	for _, file := range files {
		if total <= budget {
			break
		}
		if file.path == keep {
			continue
		}
		if err := os.Remove(file.path); err != nil {
			continue
		}
		total -= file.size
	}
	return nil
}

// artifactProgressReader 在读取响应体的同时按 256 KiB 上报进度。
type artifactProgressReader struct {
	reader io.Reader
	total  int64
	done   int64
	last   int64
	report func(done, total int64)
}

func (p *artifactProgressReader) Read(buffer []byte) (int, error) {
	read, err := p.reader.Read(buffer)
	p.done += int64(read)
	if p.report != nil && (p.done-p.last >= 256<<10 || (p.total > 0 && p.done >= p.total)) {
		p.last = p.done
		p.report(p.done, p.total)
	}
	return read, err
}
