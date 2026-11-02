package jenkins

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const consoleBody = "Started by user demo\n[Pipeline] sh\nbuild ok\nFinished: SUCCESS\n"

func consoleServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/job/demo/12/consoleText" {
			http.NotFound(w, r)
			return
		}
		if status >= 400 {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "text/plain;charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, consoleBody)
	}))
}

func TestConsoleExportNameFlattensPath(t *testing.T) {
	if got := consoleExportName("team/demo", 12); got != "team-demo-12.log" {
		t.Errorf("consoleExportName(team/demo, 12) = %q, want team-demo-12.log", got)
	}
	if got := consoleExportName("", 5); got != "build-5.log" {
		t.Errorf("consoleExportName(empty, 5) = %q, want build-5.log", got)
	}
}

// 关键点：导出的是 Jenkins 上的完整日志，而不是前端内存里被截断的尾部。
func TestStartConsoleSaveStreamsFullLog(t *testing.T) {
	home := useTempHome(t)
	t.Setenv("DBX_PLUGIN_DATA_DIR", t.TempDir())

	server := consoleServer(t, http.StatusOK)
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	started, err := client.StartConsoleSave("demo", 12)
	if err != nil {
		t.Fatalf("StartConsoleSave() error = %v", err)
	}
	if started.TaskID == "" {
		t.Fatal("taskId is empty")
	}

	final := waitForTask(t, client, started.TaskID)
	if final.State != "saved" {
		t.Fatalf("state = %q (%s), want saved", final.State, final.Error)
	}

	wantPath := filepath.Join(home, "Downloads", "demo-12.log")
	if final.Path != wantPath {
		t.Errorf("path = %q, want %q", final.Path, wantPath)
	}
	if final.Total != int64(len(consoleBody)) {
		t.Errorf("total = %d, want %d", final.Total, len(consoleBody))
	}

	written, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(written) != consoleBody {
		t.Errorf("content = %q, want %q", string(written), consoleBody)
	}
}

func TestStartConsoleSaveMapsMissingBuild(t *testing.T) {
	useTempHome(t)
	t.Setenv("DBX_PLUGIN_DATA_DIR", t.TempDir())

	server := consoleServer(t, http.StatusNotFound)
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	started, err := client.StartConsoleSave("demo", 12)
	if err != nil {
		t.Fatalf("StartConsoleSave() error = %v", err)
	}

	final := waitForTask(t, client, started.TaskID)
	if final.State != "failed" {
		t.Fatalf("state = %q, want failed", final.State)
	}
	if !strings.Contains(final.Error, "not_found") {
		t.Errorf("error = %q, want not_found", final.Error)
	}
}
