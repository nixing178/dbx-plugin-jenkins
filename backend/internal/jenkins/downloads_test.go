package jenkins

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// useTempHome 把用户主目录指到临时目录，避免测试往真实 Downloads 里写东西。
func useTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Downloads"), 0o755); err != nil {
		t.Fatalf("MkdirAll(Downloads) error = %v", err)
	}
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	return home
}

// waitForTask 轮询到终态，超时即失败。
func waitForTask(t *testing.T, client *Client, id string) ArtifactTaskSnapshot {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, err := client.ArtifactTaskStatus(id)
		if err != nil {
			t.Fatalf("ArtifactTaskStatus(%q) error = %v", id, err)
		}
		if snapshot.State != "running" {
			return snapshot
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("task %q did not finish in time", id)
	return ArtifactTaskSnapshot{}
}

func TestSaveArtifactWritesToDownloads(t *testing.T) {
	home := useTempHome(t)
	t.Setenv("DBX_PLUGIN_DATA_DIR", t.TempDir())

	hits := 0
	server := artifactServer(t, testArtifactBody, &hits)
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	saved, err := client.SaveArtifact(context.Background(), "team/demo", 12, "target/app.jar")
	if err != nil {
		t.Fatalf("SaveArtifact() error = %v", err)
	}

	wantPath := filepath.Join(home, "Downloads", "app.jar")
	if saved.Path != wantPath {
		t.Errorf("path = %q, want %q", saved.Path, wantPath)
	}
	if saved.FileName != "app.jar" {
		t.Errorf("fileName = %q, want app.jar", saved.FileName)
	}
	if saved.Size != int64(len(testArtifactBody)) {
		t.Errorf("size = %d, want %d", saved.Size, len(testArtifactBody))
	}

	written, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(written) != testArtifactBody {
		t.Errorf("content = %q, want %q", string(written), testArtifactBody)
	}
	if hits != 1 {
		t.Errorf("Jenkins GET hits = %d, want 1", hits)
	}
}

// D5=覆盖：同名制品再下一次直接覆盖，不再产生 app (1).jar。
func TestSaveArtifactOverwritesExisting(t *testing.T) {
	home := useTempHome(t)
	t.Setenv("DBX_PLUGIN_DATA_DIR", t.TempDir())

	hits := 0
	server := artifactServer(t, testArtifactBody, &hits)
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	if _, err := client.SaveArtifact(context.Background(), "team/demo", 12, "target/app.jar"); err != nil {
		t.Fatalf("first SaveArtifact() error = %v", err)
	}

	target := filepath.Join(home, "Downloads", "app.jar")
	if err := os.WriteFile(target, []byte("stale"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	second, err := client.SaveArtifact(context.Background(), "team/demo", 12, "target/app.jar")
	if err != nil {
		t.Fatalf("second SaveArtifact() error = %v", err)
	}
	if second.Path != target {
		t.Errorf("path = %q, want %q (overwrite)", second.Path, target)
	}

	written, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(written) != testArtifactBody {
		t.Errorf("content = %q, want overwritten %q", string(written), testArtifactBody)
	}

	entries, err := os.ReadDir(filepath.Join(home, "Downloads"))
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("downloads entries = %d, want 1 (no (1) copy)", len(entries))
	}
}

func TestSaveArtifactUsesConfiguredDir(t *testing.T) {
	useTempHome(t)
	t.Setenv("DBX_PLUGIN_DATA_DIR", t.TempDir())

	custom := filepath.Join(t.TempDir(), "artifacts-out")
	hits := 0
	server := artifactServer(t, testArtifactBody, &hits)
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:     server.URL,
		Username:    "alice",
		APIToken:    "token-123",
		ReadOnly:    true,
		Timeout:     2 * time.Second,
		DownloadDir: custom,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	saved, err := client.SaveArtifact(context.Background(), "team/demo", 12, "target/app.jar")
	if err != nil {
		t.Fatalf("SaveArtifact() error = %v", err)
	}
	if saved.Path != filepath.Join(custom, "app.jar") {
		t.Errorf("path = %q, want %q", saved.Path, filepath.Join(custom, "app.jar"))
	}
	if _, err := os.Stat(saved.Path); err != nil {
		t.Errorf("configured dir file missing: %v", err)
	}
}

func TestSaveTextWritesToDownloads(t *testing.T) {
	home := useTempHome(t)

	saved, err := SaveText("build-12.log", []byte("hello\nworld\n"))
	if err != nil {
		t.Fatalf("SaveText() error = %v", err)
	}
	if saved.Path != filepath.Join(home, "Downloads", "build-12.log") {
		t.Errorf("path = %q, want Downloads/build-12.log", saved.Path)
	}
	if saved.Size != 12 {
		t.Errorf("size = %d, want 12", saved.Size)
	}

	again, err := SaveText("build-12.log", []byte("second"))
	if err != nil {
		t.Fatalf("second SaveText() error = %v", err)
	}
	if again.Path != saved.Path {
		t.Errorf("second path = %q, want overwrite %q", again.Path, saved.Path)
	}
	written, err := os.ReadFile(saved.Path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(written) != "second" {
		t.Errorf("content = %q, want overwritten 'second'", string(written))
	}
}

func TestSaveTextSanitizesFileName(t *testing.T) {
	useTempHome(t)

	saved, err := SaveText(`..\..\evil:name?.log`, []byte("x"))
	if err != nil {
		t.Fatalf("SaveText() error = %v", err)
	}
	if saved.FileName != "evil_name_.log" {
		t.Errorf("fileName = %q, want evil_name_.log", saved.FileName)
	}
	if filepath.Dir(saved.Path) != DownloadDir() {
		t.Errorf("path escaped download dir: %q", saved.Path)
	}
}

func TestDownloadDirFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	if got := DownloadDir(); got != home {
		t.Errorf("DownloadDir() = %q, want home %q when Downloads is missing", got, home)
	}
}

// D3：download_dir 支持留空回落、~ 展开、拒绝相对路径。
func TestResolveDownloadDir(t *testing.T) {
	home := useTempHome(t)

	if got, err := resolveDownloadDir(""); err != nil || got != filepath.Join(home, "Downloads") {
		t.Errorf("empty = (%q, %v), want Downloads", got, err)
	}
	if got, err := resolveDownloadDir("~/ci-artifacts"); err != nil || got != filepath.Join(home, "ci-artifacts") {
		t.Errorf("~/ci-artifacts = (%q, %v), want home/ci-artifacts", got, err)
	}
	if _, err := resolveDownloadDir("./relative"); err == nil {
		t.Error("relative path error = nil, want error")
	}
	if _, err := resolveDownloadDir("artifacts"); err == nil {
		t.Error("bare relative path error = nil, want error")
	}
}

func TestStartArtifactSaveReportsProgress(t *testing.T) {
	home := useTempHome(t)
	t.Setenv("DBX_PLUGIN_DATA_DIR", t.TempDir())

	hits := 0
	server := artifactServer(t, testArtifactBody, &hits)
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	task, err := client.StartArtifactSave("team/demo", 12, "target/app.jar")
	if err != nil {
		t.Fatalf("StartArtifactSave() error = %v", err)
	}
	if task.TaskID == "" {
		t.Fatal("taskId is empty")
	}

	final := waitForTask(t, client, task.TaskID)
	if final.State != "saved" {
		t.Fatalf("state = %q (%s), want saved", final.State, final.Error)
	}
	if final.Total != int64(len(testArtifactBody)) || final.Done != final.Total {
		t.Errorf("progress = %d/%d, want %d/%d", final.Done, final.Total, len(testArtifactBody), len(testArtifactBody))
	}
	if final.Path != filepath.Join(home, "Downloads", "app.jar") {
		t.Errorf("path = %q, want Downloads/app.jar", final.Path)
	}
}

func TestCancelArtifactTask(t *testing.T) {
	useTempHome(t)
	t.Setenv("DBX_PLUGIN_DATA_DIR", t.TempDir())

	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1048576")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)

	client := newTestClient(t, server.URL, true)
	task, err := client.StartArtifactSave("team/demo", 12, "target/app.jar")
	if err != nil {
		t.Fatalf("StartArtifactSave() error = %v", err)
	}

	time.Sleep(80 * time.Millisecond)
	if err := client.CancelArtifactTask(task.TaskID); err != nil {
		t.Fatalf("CancelArtifactTask() error = %v", err)
	}

	final := waitForTask(t, client, task.TaskID)
	if final.State != "canceled" {
		t.Errorf("state = %q, want canceled", final.State)
	}
	if final.Path != "" {
		t.Errorf("path = %q, want empty after cancel", final.Path)
	}
}

func TestArtifactTaskStatusUnknown(t *testing.T) {
	useTempHome(t)

	hits := 0
	server := artifactServer(t, testArtifactBody, &hits)
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	_, err := client.ArtifactTaskStatus("missing-task")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Kind != ErrNotFound {
		t.Fatalf("error = %v, want kind %q", err, ErrNotFound)
	}
}

func TestRevealFileMissing(t *testing.T) {
	err := RevealFile(filepath.Join(t.TempDir(), "nope.jar"))
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Kind != ErrNotFound {
		t.Fatalf("error = %v, want kind %q", err, ErrNotFound)
	}
}

// 大文件下载时 Done 必须单调递增，不能只在结束时跳到 100%。
func TestArtifactTaskProgressIncrements(t *testing.T) {
	useTempHome(t)
	t.Setenv("DBX_PLUGIN_DATA_DIR", t.TempDir())

	const size = 3 << 20
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "0")
			return
		}
		w.Header().Set("Content-Length", "3145728")
		chunk := strings.Repeat("x", 64*1024)
		for written := 0; written < size; written += len(chunk) {
			if _, err := io.WriteString(w, chunk); err != nil {
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	task, err := client.StartArtifactSave("team/demo", 12, "target/app.jar")
	if err != nil {
		t.Fatalf("StartArtifactSave() error = %v", err)
	}

	sawPartial := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, err := client.ArtifactTaskStatus(task.TaskID)
		if err != nil {
			t.Fatalf("ArtifactTaskStatus() error = %v", err)
		}
		if snapshot.State == "running" && snapshot.Done > 0 && snapshot.Done < snapshot.Total {
			sawPartial = true
		}
		if snapshot.State != "running" {
			if snapshot.State != "saved" {
				t.Fatalf("state = %q (%s), want saved", snapshot.State, snapshot.Error)
			}
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !sawPartial {
		t.Error("never observed a partial progress snapshot")
	}
}
