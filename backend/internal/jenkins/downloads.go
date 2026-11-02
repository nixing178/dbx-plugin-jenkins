package jenkins

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// SavedFile 是一次落盘的结果，Path 是绝对路径。
type SavedFile struct {
	Path     string `json:"path"`
	FileName string `json:"fileName"`
	Size     int64  `json:"size"`
}

// ArtifactTaskSnapshot 是下载任务的一次快照。
type ArtifactTaskSnapshot struct {
	TaskID   string `json:"taskId"`
	State    string `json:"state"`
	Done     int64  `json:"done"`
	Total    int64  `json:"total"`
	FileName string `json:"fileName,omitempty"`
	Path     string `json:"path,omitempty"`
	Error    string `json:"error,omitempty"`
}

// 任务状态：running → saved / failed / canceled。
const (
	taskRunning  = "running"
	taskSaved    = "saved"
	taskFailed   = "failed"
	taskCanceled = "canceled"
)

// DownloadDir 返回默认下载目录：优先 <home>/Downloads，没有则退回 home。
// DBX 工作台的 iframe 没有 allow-downloads，浏览器下载通道不可用，
// 所以制品与日志导出统一由 sidecar 落盘到这里。
func DownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return os.TempDir()
	}
	downloads := filepath.Join(home, "Downloads")
	if info, statErr := os.Stat(downloads); statErr == nil && info.IsDir() {
		return downloads
	}
	return home
}

// resolveDownloadDir 解析连接里配置的下载目录：留空回落默认，支持 ~，拒绝相对路径。
func resolveDownloadDir(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return DownloadDir(), nil
	}

	if value == "~" || strings.HasPrefix(value, "~/") || strings.HasPrefix(value, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(home) == "" {
			return "", &APIError{Kind: ErrInvalidConfig, Message: "cannot resolve the home directory"}
		}
		rest := strings.TrimLeft(value[1:], `/\`)
		if rest == "" {
			return home, nil
		}
		value = filepath.Join(home, filepath.FromSlash(rest))
	}

	if !filepath.IsAbs(value) {
		return "", &APIError{Kind: ErrInvalidConfig, Message: "download directory must be an absolute path"}
	}
	return filepath.Clean(value), nil
}

// ResolveDownloadDir 解析连接配置里的下载目录（给 RPC 层用）。
func ResolveDownloadDir(raw string) (string, error) {
	return resolveDownloadDir(raw)
}

// DownloadDir 返回当前连接实际使用的下载目录。
func (c *Client) DownloadDir() string {
	if dir := strings.TrimSpace(c.downloadDir); dir != "" {
		return dir
	}
	return DownloadDir()
}

// SaveArtifact 把制品同步落到下载目录（重名直接覆盖）。
func (c *Client) SaveArtifact(
	ctx context.Context,
	fullName string,
	number int,
	relativePath string,
) (*SavedFile, error) {
	return c.saveArtifactWithProgress(ctx, fullName, number, relativePath, nil)
}

// StartArtifactSave 启动后台下载任务并立即返回快照；前端按 taskId 轮询进度。
func (c *Client) StartArtifactSave(
	fullName string,
	number int,
	relativePath string,
) (ArtifactTaskSnapshot, error) {
	if _, err := c.artifactURLPath(fullName, number, relativePath); err != nil {
		return ArtifactTaskSnapshot{}, err
	}
	id, err := newTaskID()
	if err != nil {
		return ArtifactTaskSnapshot{}, &APIError{Kind: ErrRequest, Message: "cannot allocate a task id"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), artifactDownloadTimeout)
	task := &artifactTask{
		id:       id,
		state:    taskRunning,
		fileName: pathpkg.Base(strings.TrimSpace(relativePath)),
		cancel:   cancel,
		updated:  time.Now(),
	}
	c.storeTask(task)

	go func() {
		defer cancel()
		saved, saveErr := c.saveArtifactWithProgress(ctx, fullName, number, relativePath, task.progress)
		if saveErr != nil {
			switch {
			case errors.Is(ctx.Err(), context.Canceled):
				task.finish(taskCanceled, "", "", "")
			case errors.Is(ctx.Err(), context.DeadlineExceeded):
				task.finish(taskFailed, "", "", "timeout: artifact download timed out")
			default:
				task.finish(taskFailed, "", "", errorMessage(saveErr))
			}
			return
		}
		task.finish(taskSaved, saved.Path, saved.FileName, "")
	}()

	return task.snapshot(), nil
}

// ArtifactTaskStatus 返回任务当前快照。
func (c *Client) ArtifactTaskStatus(taskID string) (ArtifactTaskSnapshot, error) {
	task := c.findTask(taskID)
	if task == nil {
		return ArtifactTaskSnapshot{}, &APIError{
			Kind:    ErrNotFound,
			Message: "unknown artifact task: " + taskID,
		}
	}
	return task.snapshot(), nil
}

// CancelArtifactTask 取消任务；已完成的任务按幂等成功处理。
func (c *Client) CancelArtifactTask(taskID string) error {
	task := c.findTask(taskID)
	if task == nil {
		return &APIError{Kind: ErrNotFound, Message: "unknown artifact task: " + taskID}
	}
	task.mu.Lock()
	cancel := task.cancel
	running := task.state == taskRunning
	task.mu.Unlock()

	if !running || cancel == nil {
		return nil
	}
	cancel()
	return nil
}

// SaveText 把文本写成文件放到默认下载目录（重名直接覆盖）。
func SaveText(name string, content []byte) (*SavedFile, error) {
	return writeDownloadFile(DownloadDir(), name, content)
}

// SaveText 把文本写成文件放到当前连接的下载目录。
func (c *Client) SaveText(name string, content []byte) (*SavedFile, error) {
	return writeDownloadFile(c.DownloadDir(), name, content)
}

// RevealFile 在系统文件管理器里定位文件。
func RevealFile(path string) error {
	cleaned := strings.TrimSpace(path)
	if cleaned == "" {
		return &APIError{Kind: ErrInvalidConfig, Message: "missing file path"}
	}
	absolute, err := filepath.Abs(cleaned)
	if err != nil {
		return &APIError{Kind: ErrInvalidConfig, Message: "invalid file path"}
	}
	if _, statErr := os.Stat(absolute); statErr != nil {
		return &APIError{Kind: ErrNotFound, Message: "file was not found: " + absolute}
	}

	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("explorer.exe", "/select,"+absolute)
	case "darwin":
		command = exec.Command("open", "-R", absolute)
	default:
		command = exec.Command("xdg-open", filepath.Dir(absolute))
	}
	if err := command.Start(); err != nil {
		return &APIError{Kind: ErrRequest, Message: "cannot open the system file manager"}
	}
	go func() { _ = command.Wait() }()
	return nil
}

func (c *Client) saveArtifactWithProgress(
	ctx context.Context,
	fullName string,
	number int,
	relativePath string,
	onProgress func(done, total int64),
) (*SavedFile, error) {
	cached, err := c.prepareArtifactWithProgress(ctx, fullName, number, relativePath, onProgress)
	if err != nil {
		return nil, err
	}
	return c.copyCachedToDownloads(ctx, cached, pathpkg.Base(strings.TrimSpace(relativePath)))
}

func (c *Client) copyCachedToDownloads(
	ctx context.Context,
	sourcePath string,
	name string,
) (*SavedFile, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return nil, &APIError{Kind: ErrRequest, Message: "cannot read cached artifact"}
	}
	defer source.Close()

	var total int64
	if info, statErr := source.Stat(); statErr == nil {
		total = info.Size()
	}
	return c.writeStreamToDownloads(ctx, source, name, total, nil)
}

// writeStreamToDownloads 把任意流写入下载目录：先写 .part 再 rename，
// 中断不会留下半个文件，同卷 rename 还能原子覆盖旧文件。
func (c *Client) writeStreamToDownloads(
	ctx context.Context,
	source io.Reader,
	name string,
	total int64,
	onProgress func(done, total int64),
) (*SavedFile, error) {
	dir := c.DownloadDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, &APIError{Kind: ErrRequest, Message: "cannot create download directory"}
	}

	target := filepath.Join(dir, sanitizeFileName(name))
	temp := target + ".part"

	destination, err := os.Create(temp)
	if err != nil {
		return nil, &APIError{Kind: ErrRequest, Message: "cannot create download file"}
	}

	reader := source
	if onProgress != nil {
		onProgress(0, total)
		reader = &artifactProgressReader{reader: source, total: total, report: onProgress}
	}

	written, copyErr := copyWithCancel(ctx, destination, reader)
	closeErr := destination.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(temp)
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, ctx.Err()
		}
		return nil, &APIError{Kind: ErrRequest, Message: "cannot write download file"}
	}

	if err := os.Rename(temp, target); err != nil {
		_ = os.Remove(temp)
		return nil, &APIError{Kind: ErrRequest, Message: "cannot finalize download file"}
	}

	return &SavedFile{
		Path:     target,
		FileName: filepath.Base(target),
		Size:     written,
	}, nil
}

func copyWithCancel(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
	buffer := make([]byte, 256*1024)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			count, writeErr := destination.Write(buffer[:read])
			written += int64(count)
			if writeErr != nil {
				return written, writeErr
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return written, nil
			}
			return written, readErr
		}
	}
}

func writeDownloadFile(dir, name string, content []byte) (*SavedFile, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, &APIError{Kind: ErrRequest, Message: "cannot create download directory"}
	}
	target := filepath.Join(dir, sanitizeFileName(name))
	temp := target + ".part"
	if err := os.WriteFile(temp, content, 0o644); err != nil {
		_ = os.Remove(temp)
		return nil, &APIError{Kind: ErrRequest, Message: "cannot write download file"}
	}
	if err := os.Rename(temp, target); err != nil {
		_ = os.Remove(temp)
		return nil, &APIError{Kind: ErrRequest, Message: "cannot finalize download file"}
	}
	return &SavedFile{
		Path:     target,
		FileName: filepath.Base(target),
		Size:     int64(len(content)),
	}, nil
}

// sanitizeFileName 只保留文件名本身，并替换 Windows 不允许的字符。
func sanitizeFileName(name string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	cleaned := strings.TrimSpace(pathpkg.Base(normalized))
	cleaned = strings.Map(func(character rune) rune {
		switch character {
		case '\\', '/', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		return character
	}, cleaned)
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return "download"
	}
	return cleaned
}

func errorMessage(err error) string {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return string(apiErr.Kind) + ": " + apiErr.Message
	}
	return err.Error()
}

func newTaskID() (string, error) {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

// artifactTask 是后台下载任务的共享状态。
type artifactTask struct {
	mu       sync.Mutex
	id       string
	state    string
	done     int64
	total    int64
	fileName string
	path     string
	errMsg   string
	cancel   context.CancelFunc
	updated  time.Time
}

func (t *artifactTask) snapshot() ArtifactTaskSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return ArtifactTaskSnapshot{
		TaskID:   t.id,
		State:    t.state,
		Done:     t.done,
		Total:    t.total,
		FileName: t.fileName,
		Path:     t.path,
		Error:    t.errMsg,
	}
}

func (t *artifactTask) progress(done, total int64) {
	t.mu.Lock()
	t.done = done
	if total > 0 {
		t.total = total
	}
	t.updated = time.Now()
	t.mu.Unlock()
}

func (t *artifactTask) finish(state, path, fileName, message string) {
	t.mu.Lock()
	t.state = state
	t.path = path
	t.errMsg = message
	if state == taskSaved {
		if fileName != "" {
			t.fileName = fileName
		}
		if t.total > 0 {
			t.done = t.total
		}
	}
	t.updated = time.Now()
	t.mu.Unlock()
}

func (c *Client) storeTask(task *artifactTask) {
	c.taskMu.Lock()
	if c.tasks == nil {
		c.tasks = map[string]*artifactTask{}
	}
	// 顺手清掉 10 分钟前就结束的任务，避免任务表无限增长。
	for id, existing := range c.tasks {
		existing.mu.Lock()
		finished := existing.state != taskRunning
		updated := existing.updated
		existing.mu.Unlock()
		if finished && time.Since(updated) > 10*time.Minute {
			delete(c.tasks, id)
		}
	}
	c.tasks[task.id] = task
	c.taskMu.Unlock()
}

func (c *Client) findTask(id string) *artifactTask {
	c.taskMu.Lock()
	defer c.taskMu.Unlock()
	return c.tasks[strings.TrimSpace(id)]
}
