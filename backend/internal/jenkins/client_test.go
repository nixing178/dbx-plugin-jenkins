package jenkins

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T, baseURL string, readOnly bool) *Client {
	t.Helper()
	client, err := NewClient(Config{
		BaseURL:  baseURL,
		Username: "alice",
		Secret:   "token-123",
		ReadOnly: readOnly,
		Timeout:  2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func TestNewClientValidation(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		wantErr bool
	}{
		{name: "valid http", baseURL: "http://127.0.0.1:8080"},
		{name: "valid https with base path", baseURL: "https://ci.example.com:443/jenkins"},
		{name: "empty", baseURL: "", wantErr: true},
		{name: "missing scheme", baseURL: "ci.example.com:8080", wantErr: true},
		{name: "unsupported scheme", baseURL: "ftp://ci.example.com", wantErr: true},
		{name: "missing host", baseURL: "https://", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewClient(Config{BaseURL: test.baseURL})
			if test.wantErr {
				if err == nil {
					t.Fatalf("NewClient() error = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
		})
	}
}

func TestClientBasicAuthAndVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "alice" || pass != "token-123" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/whoAmI/api/json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Jenkins", "2.440.3")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"alice","authenticated":true,"authorities":["authenticated"]}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	who, err := client.TestConnection(context.Background())
	if err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}
	if who.Name != "alice" || !who.Authenticated {
		t.Errorf("WhoAmI = %+v, want alice/authenticated", who)
	}
	if who.Version != "2.440.3" {
		t.Errorf("Version = %q, want 2.440.3", who.Version)
	}
}

func TestClientAnonymousHasNoAuthorization(t *testing.T) {
	var sawAuth atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			sawAuth.Store(true)
		}
		fmt.Fprint(w, `{"name":"anonymous","authenticated":false}`)
	}))
	defer server.Close()

	client, err := NewClient(Config{BaseURL: server.URL, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if _, err := client.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}
	if sawAuth.Load() {
		t.Error("anonymous client sent an Authorization header")
	}
}

func TestTestConnectionFallsBackToApiJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/whoAmI/api/json" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path != "/api/json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Jenkins", "2.319.1")
		fmt.Fprint(w, `{"mode":"NORMAL","nodeDescription":"master","useSecurity":true}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	who, err := client.TestConnection(context.Background())
	if err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}
	if who.Mode != "NORMAL" {
		t.Errorf("Mode = %q, want NORMAL", who.Mode)
	}
	if who.Version != "2.319.1" {
		t.Errorf("Version = %q, want 2.319.1", who.Version)
	}
}

func TestPostFormRetriesWithFreshCrumb(t *testing.T) {
	var crumbCalls atomic.Int32
	var postCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			n := crumbCalls.Add(1)
			fmt.Fprintf(w, `{"crumb":"crumb-%d","crumbRequestField":"Jenkins-Crumb"}`, n)
		case "/job/demo/build":
			n := postCalls.Add(1)
			if n == 1 {
				http.Error(w, "No valid crumb was included in the request", http.StatusForbidden)
				return
			}
			if got := r.Header.Get("Jenkins-Crumb"); got != "crumb-2" {
				t.Errorf("retry crumb = %q, want crumb-2", got)
			}
			if err := r.ParseForm(); err != nil {
				t.Errorf("ParseForm() error = %v", err)
			}
			if got := r.Form.Get("BRANCH"); got != "main" {
				t.Errorf("form BRANCH = %q, want main", got)
			}
			w.Header().Set("Location", serverURL(r)+"/queue/item/42/")
			w.WriteHeader(http.StatusCreated)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, false)
	response, err := client.PostForm(context.Background(), "/job/demo/build", url.Values{"BRANCH": {"main"}})
	if err != nil {
		t.Fatalf("PostForm() error = %v", err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Errorf("StatusCode = %d, want 201", response.StatusCode)
	}
	if postCalls.Load() != 2 {
		t.Errorf("post calls = %d, want 2", postCalls.Load())
	}
	if crumbCalls.Load() != 2 {
		t.Errorf("crumb calls = %d, want 2", crumbCalls.Load())
	}
}

func TestPostFormWithoutCSRF(t *testing.T) {
	var postCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			http.NotFound(w, r)
		case "/job/demo/build":
			postCalls.Add(1)
			if got := r.Header.Get("Jenkins-Crumb"); got != "" {
				t.Errorf("crumb header = %q, want empty", got)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, false)
	if _, err := client.PostForm(context.Background(), "/job/demo/build", nil); err != nil {
		t.Fatalf("PostForm() error = %v", err)
	}
	if postCalls.Load() != 1 {
		t.Errorf("post calls = %d, want 1", postCalls.Load())
	}
}

func TestReadOnlyBlocksWrites(t *testing.T) {
	var postCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	_, err := client.PostForm(context.Background(), "/job/demo/build", nil)
	if err == nil {
		t.Fatal("PostForm() error = nil, want read-only error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Kind != ErrReadOnly {
		t.Fatalf("error = %v, want kind %q", err, ErrReadOnly)
	}
	if postCalls.Load() != 0 {
		t.Errorf("server received %d requests, want 0", postCalls.Load())
	}
}

func TestGetMapsHTTPErrors(t *testing.T) {
	tests := []struct {
		path   string
		status int
		kind   ErrorKind
	}{
		{path: "/status/401", status: http.StatusUnauthorized, kind: ErrUnauthorized},
		{path: "/status/403", status: http.StatusForbidden, kind: ErrForbidden},
		{path: "/status/404", status: http.StatusNotFound, kind: ErrNotFound},
		{path: "/status/500", status: http.StatusInternalServerError, kind: ErrServer},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, test := range tests {
			if r.URL.Path == test.path {
				http.Error(w, "boom", test.status)
				return
			}
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			err := client.Get(context.Background(), test.path, nil)
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("Get() error = %v, want *APIError", err)
			}
			if apiErr.Kind != test.kind || apiErr.StatusCode != test.status {
				t.Errorf("APIError = %+v, want kind=%s status=%d", apiErr, test.kind, test.status)
			}
		})
	}
}

func TestGetMapsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:  server.URL,
		Username: "alice",
		Secret:   "token",
		Timeout:  30 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	err = client.Get(context.Background(), "/slow", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Kind != ErrTimeout {
		t.Fatalf("error = %v, want kind %q", err, ErrTimeout)
	}
}

func TestClientCanSkipTLSVerification(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"name":"tls","authenticated":true}`)
	}))
	defer server.Close()

	strict, err := NewClient(Config{BaseURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if _, err := strict.TestConnection(context.Background()); err == nil {
		t.Fatal("strict client accepted a self-signed certificate")
	}

	insecure, err := NewClient(Config{
		BaseURL:            server.URL,
		InsecureSkipVerify: true,
		Timeout:            time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if _, err := insecure.TestConnection(context.Background()); err != nil {
		t.Fatalf("insecure TestConnection() error = %v", err)
	}
}

func serverURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
