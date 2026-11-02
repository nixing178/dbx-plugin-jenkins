package rpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nixing178/dbx-plugin-jenkins/internal/jenkins"
)

func fakeJenkinsArtifacts(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/whoAmI/api/json", func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != "alice" || pass != "token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("X-Jenkins", "2.440.3")
		fmt.Fprint(w, `{"name":"alice","authenticated":true}`)
	})
	mux.HandleFunc("/job/team/job/demo/12/artifact/target/app.jar", func(w http.ResponseWriter, r *http.Request) {
		body := "abcdefghij"
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		w.Header().Set("Content-Type", "application/java-archive")
		if r.Method == http.MethodHead {
			return
		}
		fmt.Fprint(w, body)
	})
	return httptest.NewServer(mux)
}

func artifactRequest(t *testing.T, extra map[string]any) json.RawMessage {
	t.Helper()
	payload := map[string]any{
		"connectionId": "conn-1",
		"job":          "team/demo",
		"number":       12,
		"path":         "target/app.jar",
	}
	for key, value := range extra {
		payload[key] = value
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return encoded
}

func connectArtifactSession(t *testing.T, service *Service, serverURL string) {
	t.Helper()
	payload := lifecyclePayload(t, serverURL, "token", "api_token", "", false)
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", payload); rpcErr != nil {
		t.Fatalf("connection/connect error = %+v", rpcErr)
	}
}

func TestArtifactRPCPreparesAndServesChunks(t *testing.T) {
	t.Setenv("DBX_PLUGIN_DATA_DIR", t.TempDir())
	server := fakeJenkinsArtifacts(t)
	defer server.Close()

	service := NewService()
	connectArtifactSession(t, service, server.URL)

	infoResult, rpcErr := service.Handle(
		context.Background(),
		"jenkins/artifactInfo",
		artifactRequest(t, nil),
	)
	if rpcErr != nil {
		t.Fatalf("jenkins/artifactInfo error = %+v", rpcErr)
	}
	info := decodeResult[jenkins.ArtifactInfo](t, infoResult)
	if info.Size != 10 || info.FileName != "app.jar" {
		t.Errorf("info = %+v, want app.jar with size 10", info)
	}

	chunkResult, rpcErr := service.Handle(
		context.Background(),
		"jenkins/artifactChunk",
		artifactRequest(t, map[string]any{"offset": 4, "length": 4}),
	)
	if rpcErr != nil {
		t.Fatalf("jenkins/artifactChunk error = %+v", rpcErr)
	}
	chunk := decodeResult[jenkins.ArtifactChunk](t, chunkResult)
	data, err := base64.StdEncoding.DecodeString(chunk.DataBase64)
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	if string(data) != "efgh" {
		t.Errorf("chunk data = %q, want efgh", string(data))
	}
	if chunk.Total != 10 || chunk.EOF {
		t.Errorf("chunk = %+v, want total 10 and eof false", chunk)
	}
}

func TestArtifactRPCRejectsUnsafePath(t *testing.T) {
	server := fakeJenkinsArtifacts(t)
	defer server.Close()

	service := NewService()
	connectArtifactSession(t, service, server.URL)

	_, rpcErr := service.Handle(
		context.Background(),
		"jenkins/artifactChunk",
		artifactRequest(t, map[string]any{"path": "../secret.jar"}),
	)
	if rpcErr == nil {
		t.Fatal("jenkins/artifactChunk error = nil, want invalid params")
	}
	if rpcErr.Code != -32602 {
		t.Errorf("code = %d, want -32602", rpcErr.Code)
	}
}

func TestArtifactSaveRPCWritesToDownloads(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Downloads"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	t.Setenv("DBX_PLUGIN_DATA_DIR", t.TempDir())

	server := fakeJenkinsArtifacts(t)
	defer server.Close()

	service := NewService()
	connectArtifactSession(t, service, server.URL)

	result, rpcErr := service.Handle(
		context.Background(),
		"jenkins/artifactSave",
		artifactRequest(t, nil),
	)
	if rpcErr != nil {
		t.Fatalf("jenkins/artifactSave error = %+v", rpcErr)
	}
	saved := decodeResult[jenkins.SavedFile](t, result)
	if saved.Size != 10 {
		t.Errorf("size = %d, want 10", saved.Size)
	}
	if saved.Path != filepath.Join(home, "Downloads", "app.jar") {
		t.Errorf("path = %q, want Downloads/app.jar", saved.Path)
	}
}

func TestSaveTextRPCWritesToDownloads(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Downloads"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	server := fakeJenkinsArtifacts(t)
	defer server.Close()

	service := NewService()
	connectArtifactSession(t, service, server.URL)

	payload, err := json.Marshal(map[string]any{
		"connectionId": "conn-1",
		"name":         "demo-12.log",
		"content":      "hello\n",
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	result, rpcErr := service.Handle(context.Background(), "jenkins/saveText", payload)
	if rpcErr != nil {
		t.Fatalf("jenkins/saveText error = %+v", rpcErr)
	}
	saved := decodeResult[jenkins.SavedFile](t, result)
	if saved.Path != filepath.Join(home, "Downloads", "demo-12.log") {
		t.Errorf("path = %q, want Downloads/demo-12.log", saved.Path)
	}
	if saved.Size != 6 {
		t.Errorf("size = %d, want 6", saved.Size)
	}
}

func TestArtifactTaskRPCStartAndStatus(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Downloads"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	t.Setenv("DBX_PLUGIN_DATA_DIR", t.TempDir())

	server := fakeJenkinsArtifacts(t)
	defer server.Close()

	service := NewService()
	connectArtifactSession(t, service, server.URL)

	startResult, rpcErr := service.Handle(
		context.Background(),
		"jenkins/artifactStart",
		artifactRequest(t, nil),
	)
	if rpcErr != nil {
		t.Fatalf("jenkins/artifactStart error = %+v", rpcErr)
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
			if snapshot.Path != filepath.Join(home, "Downloads", "app.jar") {
				t.Errorf("path = %q, want Downloads/app.jar", snapshot.Path)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("task did not finish in time")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRevealFileRPCRejectsMissingPath(t *testing.T) {
	server := fakeJenkinsArtifacts(t)
	defer server.Close()

	service := NewService()
	connectArtifactSession(t, service, server.URL)

	payload, err := json.Marshal(map[string]any{
		"connectionId": "conn-1",
		"path":         filepath.Join(t.TempDir(), "missing.jar"),
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	_, rpcErr := service.Handle(context.Background(), "jenkins/revealFile", payload)
	if rpcErr == nil {
		t.Fatal("jenkins/revealFile error = nil, want not_found")
	}
	if rpcErr.Kind != string(jenkins.ErrNotFound) {
		t.Errorf("kind = %q, want %q", rpcErr.Kind, jenkins.ErrNotFound)
	}
}

func TestArtifactRPCMapsMissingArtifact(t *testing.T) {
	t.Setenv("DBX_PLUGIN_DATA_DIR", t.TempDir())
	server := fakeJenkinsArtifacts(t)
	defer server.Close()

	service := NewService()
	connectArtifactSession(t, service, server.URL)

	_, rpcErr := service.Handle(
		context.Background(),
		"jenkins/artifactInfo",
		artifactRequest(t, map[string]any{"path": "target/missing.jar"}),
	)
	if rpcErr == nil {
		t.Fatal("jenkins/artifactInfo error = nil, want not_found")
	}
	if rpcErr.Kind != string(jenkins.ErrNotFound) {
		t.Errorf("kind = %q, want %q", rpcErr.Kind, jenkins.ErrNotFound)
	}
}
