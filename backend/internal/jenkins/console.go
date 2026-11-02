package jenkins

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const maxConsoleChunk = 512 * 1024

// consoleExportName 返回导出文件名：任务全名的 / 换成 -，避免落盘时变成子目录。
func consoleExportName(fullName string, number int) string {
	trimmed := strings.Trim(strings.TrimSpace(fullName), "/")
	if trimmed == "" {
		return fmt.Sprintf("build-%d.log", number)
	}
	return strings.ReplaceAll(trimmed, "/", "-") + "-" + strconv.Itoa(number) + ".log"
}

// StartConsoleSave 把完整控制台日志流式落盘到下载目录，前端轮询任务进度。
// 与前端内存里那份日志不同：这里直接拉 Jenkins 的 consoleText，日志再大也不截断。
func (c *Client) StartConsoleSave(fullName string, number int) (ArtifactTaskSnapshot, error) {
	if _, err := JobPath(fullName); err != nil {
		return ArtifactTaskSnapshot{}, &APIError{Kind: ErrInvalidConfig, Message: err.Error()}
	}
	if number <= 0 {
		return ArtifactTaskSnapshot{}, &APIError{Kind: ErrInvalidConfig, Message: "build number must be positive"}
	}

	id, err := newTaskID()
	if err != nil {
		return ArtifactTaskSnapshot{}, &APIError{Kind: ErrRequest, Message: "cannot allocate a task id"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), artifactDownloadTimeout)
	task := &artifactTask{
		id:       id,
		state:    taskRunning,
		fileName: consoleExportName(fullName, number),
		cancel:   cancel,
		updated:  time.Now(),
	}
	c.storeTask(task)

	go func() {
		defer cancel()
		saved, saveErr := c.saveConsoleWithProgress(ctx, fullName, number, task.progress)
		if saveErr != nil {
			switch {
			case errors.Is(ctx.Err(), context.Canceled):
				task.finish(taskCanceled, "", "", "")
			case errors.Is(ctx.Err(), context.DeadlineExceeded):
				task.finish(taskFailed, "", "", "timeout: console download timed out")
			default:
				task.finish(taskFailed, "", "", errorMessage(saveErr))
			}
			return
		}
		task.finish(taskSaved, saved.Path, saved.FileName, "")
	}()

	return task.snapshot(), nil
}

func (c *Client) saveConsoleWithProgress(
	ctx context.Context,
	fullName string,
	number int,
	onProgress func(done, total int64),
) (*SavedFile, error) {
	jobPath, err := JobPath(fullName)
	if err != nil {
		return nil, &APIError{Kind: ErrInvalidConfig, Message: err.Error()}
	}

	path := fmt.Sprintf("%s/%d/consoleText", jobPath, number)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.resolve(path), nil)
	if err != nil {
		return nil, &APIError{Kind: ErrRequest, Message: err.Error()}
	}
	request.Header.Set("Accept", "text/plain")

	response, err := c.downloadClient.Do(request)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, ctx.Err()
		}
		return nil, &APIError{Kind: ErrNetwork, Message: "cannot download Jenkins console output"}
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 8192))
		return nil, newAPIError(response.StatusCode, body)
	}

	total := response.ContentLength
	if total < 0 {
		total = 0
	}
	return c.writeStreamToDownloads(
		ctx,
		response.Body,
		consoleExportName(fullName, number),
		total,
		onProgress,
	)
}

// GetConsole 读取从 start 开始的增量控制台日志。
// Jenkins 用 X-Text-Size 表示日志总长度、X-More-Data 表示是否还有后续内容。
func (c *Client) GetConsole(ctx context.Context, fullName string, number int, start int64) (*ConsoleChunk, error) {
	jobPath, err := JobPath(fullName)
	if err != nil {
		return nil, &APIError{Kind: ErrInvalidConfig, Message: err.Error()}
	}
	if number <= 0 {
		return nil, &APIError{Kind: ErrInvalidConfig, Message: "build number must be positive"}
	}
	if start < 0 {
		start = 0
	}

	path := fmt.Sprintf("%s/%d/logText/progressiveText?start=%d", jobPath, number, start)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.resolve(path), nil)
	if err != nil {
		return nil, &APIError{Kind: ErrRequest, Message: err.Error()}
	}
	request.Header.Set("Accept", "text/plain")

	response, err := c.do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxConsoleChunk+1))
	if readErr != nil {
		return nil, &APIError{Kind: ErrNetwork, Message: "cannot read Jenkins console output"}
	}
	truncated := len(body) > maxConsoleChunk
	if truncated {
		body = body[:maxConsoleChunk]
	}
	if response.StatusCode >= 400 {
		return nil, newAPIError(response.StatusCode, body)
	}

	nextStart := start + int64(len(body))
	headerSize, _ := strconv.ParseInt(strings.TrimSpace(response.Header.Get("X-Text-Size")), 10, 64)
	if !truncated && headerSize > 0 {
		nextStart = headerSize
	}

	moreData := truncated ||
		strings.EqualFold(response.Header.Get("X-More-Data"), "true") ||
		headerSize > nextStart

	return &ConsoleChunk{
		Text:      string(body),
		NextStart: nextStart,
		MoreData:  moreData,
	}, nil
}
