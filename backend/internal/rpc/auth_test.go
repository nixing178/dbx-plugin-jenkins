package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// authStub 是只实现 /whoAmI/api/json 的假 Jenkins。
// secret 非空时要求 Basic Auth 的密码严格等于它；同时记录请求次数，
// 用来断言「配置非法时不应发起网络请求」。
type authStub struct {
	server   *httptest.Server
	requests atomic.Int32
}

func newAuthStub(t *testing.T, secret string) *authStub {
	t.Helper()
	stub := &authStub{}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.requests.Add(1)
		if r.URL.Path != "/whoAmI/api/json" {
			http.NotFound(w, r)
			return
		}
		if secret != "" {
			user, pass, ok := r.BasicAuth()
			if !ok || user != "alice" || pass != secret {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		w.Header().Set("X-Jenkins", "2.440.3")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"alice","authenticated":true}`)
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

// authPayload 构造 connection/test 请求；authMethod 为空表示老连接（没有该字段）。
func authPayload(t *testing.T, serverURL, authMethod, password, apiToken string) json.RawMessage {
	t.Helper()
	scheme, host, port := splitServerURL(t, serverURL)
	config := map[string]any{"scheme": scheme, "base_path": ""}
	if authMethod != "" {
		config["auth_method"] = authMethod
	}
	connection := map[string]any{
		"id":              "conn-1",
		"name":            "test",
		"host":            host,
		"port":            port,
		"username":        "alice",
		"external_config": config,
	}
	if password != "" {
		connection["password"] = password
	}
	if apiToken != "" {
		connection["api_token"] = apiToken
	}
	payload := map[string]any{
		"connection":   connection,
		"runtime":      map[string]any{"host": host, "port": port},
		"connectionId": "conn-1",
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return encoded
}

func callConnectionTest(t *testing.T, payload json.RawMessage) *Error {
	t.Helper()
	service := NewService()
	_, rpcErr := service.Handle(context.Background(), "connection/test", payload)
	return rpcErr
}

func TestPasswordAuthConnects(t *testing.T) {
	stub := newAuthStub(t, "pwd-value")
	if rpcErr := callConnectionTest(t, authPayload(t, stub.server.URL, "password", "pwd-value", "")); rpcErr != nil {
		t.Fatalf("connection/test error = %+v, want password auth to succeed", rpcErr)
	}
}

func TestPasswordAuthIgnoresLeftoverAPIToken(t *testing.T) {
	// 服务端只接受密码；残留的 api_token 必须被忽略，否则会 401。
	stub := newAuthStub(t, "pwd-value")
	if rpcErr := callConnectionTest(t, authPayload(t, stub.server.URL, "password", "pwd-value", "stale-token")); rpcErr != nil {
		t.Fatalf("connection/test error = %+v, want password to win over api_token", rpcErr)
	}
}

func TestPasswordAuthRequiresPassword(t *testing.T) {
	stub := newAuthStub(t, "")
	rpcErr := callConnectionTest(t, authPayload(t, stub.server.URL, "password", "", "stale-token"))
	if rpcErr == nil {
		t.Fatal("connection/test error = nil, want invalid_config")
	}
	if rpcErr.Kind != "invalid_config" {
		t.Errorf("Kind = %q, want invalid_config", rpcErr.Kind)
	}
	if got := stub.requests.Load(); got != 0 {
		t.Errorf("requests = %d, want 0 (must fail before calling Jenkins)", got)
	}
}

func TestTokenAuthRemainsDefault(t *testing.T) {
	stub := newAuthStub(t, "api-token")
	if rpcErr := callConnectionTest(t, authPayload(t, stub.server.URL, "", "", "api-token")); rpcErr != nil {
		t.Fatalf("connection/test error = %+v, want token auth to keep working", rpcErr)
	}
}

func TestUnknownAuthMethodRejected(t *testing.T) {
	stub := newAuthStub(t, "")
	rpcErr := callConnectionTest(t, authPayload(t, stub.server.URL, "basic", "pwd-value", ""))
	if rpcErr == nil {
		t.Fatal("connection/test error = nil, want invalid_config")
	}
	if rpcErr.Kind != "invalid_config" {
		t.Errorf("Kind = %q, want invalid_config", rpcErr.Kind)
	}
	if got := stub.requests.Load(); got != 0 {
		t.Errorf("requests = %d, want 0 (unknown auth method must not reach Jenkins)", got)
	}
}

func TestPasswordAuthUnauthorizedMentionsAPIToken(t *testing.T) {
	stub := newAuthStub(t, "right-password")
	rpcErr := callConnectionTest(t, authPayload(t, stub.server.URL, "password", "wrong-password", ""))
	if rpcErr == nil {
		t.Fatal("connection/test error = nil, want unauthorized")
	}
	if rpcErr.Kind != "unauthorized" {
		t.Errorf("Kind = %q, want unauthorized", rpcErr.Kind)
	}
	if !strings.Contains(strings.ToLower(rpcErr.Message), "api token") {
		t.Errorf("Message = %q, want hint about API token", rpcErr.Message)
	}
}

func TestTokenAuthUnauthorizedHasNoPasswordHint(t *testing.T) {
	stub := newAuthStub(t, "right-token")
	rpcErr := callConnectionTest(t, authPayload(t, stub.server.URL, "", "", "wrong-token"))
	if rpcErr == nil {
		t.Fatal("connection/test error = nil, want unauthorized")
	}
	if rpcErr.Kind != "unauthorized" {
		t.Errorf("Kind = %q, want unauthorized", rpcErr.Kind)
	}
	if strings.Contains(strings.ToLower(rpcErr.Message), "password") {
		t.Errorf("Message = %q, token auth must not hint at password", rpcErr.Message)
	}
}
