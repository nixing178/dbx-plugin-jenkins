package jenkins

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

const testArtifactBody = "abcdefghij"

// artifactServer 模拟 Jenkins 的制品端点：HEAD 只给长度，GET 才发内容。
func artifactServer(t *testing.T, body string, hits *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/job/team/job/demo/12/artifact/target/app.jar" {
			t.Errorf("path = %q, want /job/team/job/demo/12/artifact/target/app.jar", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if user, pass, ok := r.BasicAuth(); !ok || user != "alice" || pass != "token-123" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/java-archive")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		if r.Method == http.MethodHead {
			return
		}
		*hits++
		_, _ = io.WriteString(w, body)
	}))
}

func decodeChunk(t *testing.T, chunk *ArtifactChunk) string {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(chunk.DataBase64)
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	return string(data)
}

func TestArtifactPathEscapesEverySegment(t *testing.T) {
	got, err := ArtifactPath("dist/我的 包.jar")
	if err != nil {
		t.Fatalf("ArtifactPath() error = %v", err)
	}
	want := "/artifact/dist/%E6%88%91%E7%9A%84%20%E5%8C%85.jar"
	if got != want {
		t.Errorf("ArtifactPath() = %q, want %q", got, want)
	}
}

func TestArtifactPathRejectsUnsafeInput(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "empty", path: ""},
		{name: "blank", path: "   "},
		{name: "absolute", path: "/etc/passwd"},
		{name: "parent", path: "../secret.jar"},
		{name: "nested parent", path: "target/../../secret.jar"},
		{name: "current", path: "./app.jar"},
		{name: "empty segment", path: "target//app.jar"},
		{name: "trailing slash", path: "target/"},
		{name: "backslash", path: `target\app.jar`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ArtifactPath(test.path); err == nil {
				t.Errorf("ArtifactPath(%q) error = nil, want error", test.path)
			}
		})
	}
}

func TestGetArtifactInfoUsesHead(t *testing.T) {
	hits := 0
	server := artifactServer(t, testArtifactBody, &hits)
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	info, err := client.GetArtifactInfo(context.Background(), "team/demo", 12, "target/app.jar")
	if err != nil {
		t.Fatalf("GetArtifactInfo() error = %v", err)
	}
	if info.Size != int64(len(testArtifactBody)) {
		t.Errorf("size = %d, want %d", info.Size, len(testArtifactBody))
	}
	if info.FileName != "app.jar" {
		t.Errorf("fileName = %q, want app.jar", info.FileName)
	}
	if info.ContentType != "application/java-archive" {
		t.Errorf("contentType = %q, want application/java-archive", info.ContentType)
	}
	if hits != 0 {
		t.Errorf("Jenkins GET hits = %d, want 0 (HEAD only)", hits)
	}
}

// 一些反代 / 老版本 Jenkins 的 HEAD 不带 Content-Length：回退成下载后 stat。
func TestGetArtifactInfoFallsBackWhenHeadHasNoLength(t *testing.T) {
	t.Setenv("DBX_PLUGIN_DATA_DIR", t.TempDir())
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "0")
			return
		}
		hits++
		_, _ = io.WriteString(w, testArtifactBody)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	info, err := client.GetArtifactInfo(context.Background(), "team/demo", 12, "target/app.jar")
	if err != nil {
		t.Fatalf("GetArtifactInfo() error = %v", err)
	}
	if info.Size != int64(len(testArtifactBody)) {
		t.Errorf("size = %d, want %d", info.Size, len(testArtifactBody))
	}
	if hits != 1 {
		t.Errorf("Jenkins GET hits = %d, want 1", hits)
	}
}

func TestGetArtifactInfoMapsNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no such artifact", http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	_, err := client.GetArtifactInfo(context.Background(), "team/demo", 12, "target/app.jar")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Kind != ErrNotFound {
		t.Fatalf("error = %v, want kind %q", err, ErrNotFound)
	}
}

// 只读连接也要能下载：下载是 GET，不碰写守卫。
func TestGetArtifactChunkServesFromCache(t *testing.T) {
	t.Setenv("DBX_PLUGIN_DATA_DIR", t.TempDir())
	hits := 0
	server := artifactServer(t, testArtifactBody, &hits)
	defer server.Close()

	client := newTestClient(t, server.URL, true)

	first, err := client.GetArtifactChunk(context.Background(), "team/demo", 12, "target/app.jar", 0, 4)
	if err != nil {
		t.Fatalf("GetArtifactChunk(0,4) error = %v", err)
	}
	if got := decodeChunk(t, first); got != "abcd" {
		t.Errorf("first chunk = %q, want abcd", got)
	}
	if first.Offset != 0 || first.Length != 4 || first.Total != 10 || first.EOF {
		t.Errorf("first = %+v, want offset 0 length 4 total 10 eof false", first)
	}

	second, err := client.GetArtifactChunk(context.Background(), "team/demo", 12, "target/app.jar", 4, 4)
	if err != nil {
		t.Fatalf("GetArtifactChunk(4,4) error = %v", err)
	}
	if got := decodeChunk(t, second); got != "efgh" {
		t.Errorf("second chunk = %q, want efgh", got)
	}

	third, err := client.GetArtifactChunk(context.Background(), "team/demo", 12, "target/app.jar", 8, 4)
	if err != nil {
		t.Fatalf("GetArtifactChunk(8,4) error = %v", err)
	}
	if got := decodeChunk(t, third); got != "ij" {
		t.Errorf("third chunk = %q, want ij", got)
	}
	if third.Length != 2 || !third.EOF {
		t.Errorf("third = %+v, want length 2 eof true", third)
	}

	if hits != 1 {
		t.Errorf("Jenkins GET hits = %d, want 1 (cached after first chunk)", hits)
	}
}

func TestGetArtifactChunkClampsLength(t *testing.T) {
	t.Setenv("DBX_PLUGIN_DATA_DIR", t.TempDir())
	hits := 0
	server := artifactServer(t, testArtifactBody, &hits)
	defer server.Close()

	client := newTestClient(t, server.URL, true)

	chunk, err := client.GetArtifactChunk(context.Background(), "team/demo", 12, "target/app.jar", 0, 0)
	if err != nil {
		t.Fatalf("GetArtifactChunk(length=0) error = %v", err)
	}
	if chunk.Length != 10 || !chunk.EOF {
		t.Errorf("default length chunk = %+v, want length 10 eof true", chunk)
	}

	huge, err := client.GetArtifactChunk(context.Background(), "team/demo", 12, "target/app.jar", 0, 1<<30)
	if err != nil {
		t.Fatalf("GetArtifactChunk(length=1GiB) error = %v", err)
	}
	if huge.Length != 10 {
		t.Errorf("clamped chunk length = %d, want 10", huge.Length)
	}
	if maxArtifactChunk != 4<<20 {
		t.Errorf("maxArtifactChunk = %d, want 4 MiB", maxArtifactChunk)
	}
}

func TestGetArtifactChunkOffsetPastEnd(t *testing.T) {
	t.Setenv("DBX_PLUGIN_DATA_DIR", t.TempDir())
	hits := 0
	server := artifactServer(t, testArtifactBody, &hits)
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	chunk, err := client.GetArtifactChunk(context.Background(), "team/demo", 12, "target/app.jar", 20, 4)
	if err != nil {
		t.Fatalf("GetArtifactChunk(offset=20) error = %v", err)
	}
	if chunk.Length != 0 || chunk.DataBase64 != "" || !chunk.EOF {
		t.Errorf("chunk = %+v, want empty tail with eof true", chunk)
	}
	if chunk.Total != 10 {
		t.Errorf("total = %d, want 10", chunk.Total)
	}
}

func TestGetArtifactChunkMapsNotFound(t *testing.T) {
	t.Setenv("DBX_PLUGIN_DATA_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no such artifact", http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	_, err := client.GetArtifactChunk(context.Background(), "team/demo", 12, "target/app.jar", 0, 4)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Kind != ErrNotFound {
		t.Fatalf("error = %v, want kind %q", err, ErrNotFound)
	}
}

func TestArtifactCacheDirUsesPluginDataDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DBX_PLUGIN_DATA_DIR", dir)
	if got, want := ArtifactCacheDir(), filepath.Join(dir, "artifacts"); got != want {
		t.Errorf("ArtifactCacheDir() = %q, want %q", got, want)
	}
}

func TestPruneArtifactCacheDropsOldest(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, size int, age time.Duration) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, bytes.Repeat([]byte("x"), size), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", name, err)
		}
		when := time.Now().Add(-age)
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatalf("Chtimes(%q) error = %v", name, err)
		}
		return path
	}

	oldest := write("a.bin", 100, 3*time.Hour)
	middle := write("b.bin", 100, 2*time.Hour)
	newest := write("c.bin", 100, time.Hour)

	if err := pruneArtifactCache(dir, 250, newest); err != nil {
		t.Fatalf("pruneArtifactCache() error = %v", err)
	}
	if _, err := os.Stat(oldest); !os.IsNotExist(err) {
		t.Errorf("oldest file should be pruned, stat error = %v", err)
	}
	if _, err := os.Stat(middle); err != nil {
		t.Errorf("middle file should survive: %v", err)
	}
	if _, err := os.Stat(newest); err != nil {
		t.Errorf("kept file must survive: %v", err)
	}
}
