package rpc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nixing178/dbx-plugin-jenkins/internal/jenkins"
)

func TestConsoleSaveRPCStreamsLogToDownloads(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Downloads"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	t.Setenv("DBX_PLUGIN_DATA_DIR", t.TempDir())

	const body = "Started by user demo\nFinished: SUCCESS\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"mode":"NORMAL","useSecurity":true}`)
		case "/job/demo/12/consoleText":
			w.Header().Set("Content-Type", "text/plain;charset=UTF-8")
			_, _ = io.WriteString(w, body)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewService()
	connectArtifactSession(t, service, server.URL)

	payload, err := json.Marshal(map[string]any{
		"connectionId": "conn-1",
		"job":          "demo",
		"number":       12,
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	startResult, rpcErr := service.Handle(context.Background(), "jenkins/consoleSave", payload)
	if rpcErr != nil {
		t.Fatalf("jenkins/consoleSave error = %+v", rpcErr)
	}
	started := decodeResult[jenkins.ArtifactTaskSnapshot](t, startResult)
	if started.TaskID == "" {
		t.Fatal("taskId is empty")
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		statusPayload, err := json.Marshal(map[string]any{
			"connectionId": "conn-1",
			"taskId":       started.TaskID,
		})
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		statusResult, statusErr := service.Handle(
			context.Background(),
			"jenkins/artifactStatus",
			statusPayload,
		)
		if statusErr != nil {
			t.Fatalf("jenkins/artifactStatus error = %+v", statusErr)
		}
		snapshot := decodeResult[jenkins.ArtifactTaskSnapshot](t, statusResult)
		if snapshot.State != "running" {
			if snapshot.State != "saved" {
				t.Fatalf("state = %q (%s), want saved", snapshot.State, snapshot.Error)
			}
			want := filepath.Join(home, "Downloads", "demo-12.log")
			if snapshot.Path != want {
				t.Errorf("path = %q, want %q", snapshot.Path, want)
			}
			written, readErr := os.ReadFile(want)
			if readErr != nil {
				t.Fatalf("ReadFile() error = %v", readErr)
			}
			if string(written) != body {
				t.Errorf("content = %q, want %q", string(written), body)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("console save task did not finish in time")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
