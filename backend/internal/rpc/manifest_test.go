package rpc

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

type manifestOption struct {
	Value string `json:"value"`
}

type manifestField struct {
	Key          string           `json:"key"`
	Type         string           `json:"type"`
	Binding      string           `json:"binding"`
	Default      any              `json:"default"`
	Options      []manifestOption `json:"options"`
	VisibleWhen  json.RawMessage  `json:"visible_when"`
	RequiredWhen json.RawMessage  `json:"required_when"`
}

// TestManifestSupportsPasswordAuth 把连接表单的认证契约钉在 manifest 上：
// 表单由 DBX 宿主渲染，字段缺失或绑定写错时插件本身没有任何编译期报错。
func TestManifestSupportsPasswordAuth(t *testing.T) {
	raw, err := os.ReadFile("../../../manifest.json")
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	var manifest struct {
		Contributions []struct {
			Type   string          `json:"type"`
			Fields []manifestField `json:"fields"`
		} `json:"contributions"`
		Localizations map[string]struct {
			Contributions map[string]struct {
				Fields map[string]json.RawMessage `json:"fields"`
			} `json:"contributions"`
		} `json:"localizations"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("unmarshal manifest.json: %v", err)
	}

	fields := map[string]manifestField{}
	for _, contribution := range manifest.Contributions {
		if contribution.Type != "connection-provider" {
			continue
		}
		for _, field := range contribution.Fields {
			fields[field.Key] = field
		}
	}

	method, ok := fields["auth_method"]
	if !ok {
		t.Fatal("manifest connection fields missing auth_method")
	}
	if method.Type != "select" {
		t.Errorf("auth_method.Type = %q, want select", method.Type)
	}
	if method.Binding != "config" {
		t.Errorf("auth_method.Binding = %q, want config", method.Binding)
	}
	if method.Default != "token" {
		t.Errorf("auth_method.Default = %v, want token (old connections must stay on token auth)", method.Default)
	}
	values := make([]string, 0, len(method.Options))
	for _, option := range method.Options {
		values = append(values, option.Value)
	}
	for _, want := range []string{"token", "password"} {
		if !stringSliceContains(values, want) {
			t.Errorf("auth_method options = %v, missing %q", values, want)
		}
	}

	password, ok := fields["password"]
	if !ok {
		t.Fatal("manifest connection fields missing password")
	}
	if password.Type != "password" {
		t.Errorf("password.Type = %q, want password", password.Type)
	}
	if password.Binding != "password" {
		t.Errorf("password.Binding = %q, want password (host credential storage)", password.Binding)
	}
	if !conditionTargets(password.VisibleWhen, "auth_method") {
		t.Error("password.visible_when must key off auth_method")
	}
	if !conditionTargets(password.RequiredWhen, "auth_method") {
		t.Error("password.required_when must key off auth_method")
	}

	token, ok := fields["api_token"]
	if !ok {
		t.Fatal("manifest connection fields missing api_token")
	}
	if len(token.VisibleWhen) == 0 {
		t.Error("api_token.visible_when must be set so password mode can hide it")
	} else if !strings.Contains(string(token.VisibleWhen), "auth_method") {
		t.Errorf("api_token.visible_when = %s, want a condition on auth_method", token.VisibleWhen)
	}

	for _, locale := range []string{"en", "zh-CN"} {
		localization, ok := manifest.Localizations[locale]
		if !ok {
			t.Fatalf("manifest localizations missing %q", locale)
		}
		connection, ok := localization.Contributions["io.github.nixing178.jenkins.connection"]
		if !ok {
			t.Fatalf("manifest localizations[%q] missing connection contribution", locale)
		}
		for _, key := range []string{"auth_method", "password"} {
			if _, ok := connection.Fields[key]; !ok {
				t.Errorf("manifest localizations[%q] missing field %q", locale, key)
			}
		}
	}
}

// manifestMCPContribution 只取外部 MCP 暴露相关字段。
type manifestMCPContribution struct {
	Type          string  `json:"type"`
	ID            string  `json:"id"`
	AITools       *bool   `json:"ai_tools"`
	ExternalTools *bool   `json:"external_tools"`
	Toolsets      []struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
	} `json:"toolsets"`
	Tools map[string]struct {
		Toolset string `json:"toolset"`
		Access  string `json:"access"`
	} `json:"tools"`
}

// TestManifestExposesMCPToolsExternally 把 dbx-mcp 的白名单契约钉在 manifest 上：
// external_tools 只表示“允许对外”，真正决定暴露面的是 mcp.tools 里逐条的 access；
// 声明与 mcp/tools 返回的工具名一旦漂移，外部客户端就会出现“看着有、调不到”或反之。
func TestManifestExposesMCPToolsExternally(t *testing.T) {
	raw, err := os.ReadFile("../../../manifest.json")
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	var manifest struct {
		Contributions []manifestMCPContribution `json:"contributions"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("unmarshal manifest.json: %v", err)
	}

	var mcp *manifestMCPContribution
	for index := range manifest.Contributions {
		contribution := &manifest.Contributions[index]
		if contribution.Type != "mcp" {
			continue
		}
		if mcp != nil {
			t.Fatal("manifest declares more than one mcp contribution; the host reads only one")
		}
		mcp = contribution
	}
	if mcp == nil {
		t.Fatal("manifest missing mcp contribution")
	}
	if mcp.ExternalTools == nil || !*mcp.ExternalTools {
		t.Fatal("mcp.external_tools must be true, otherwise dbx-mcp exposes no tool at all")
	}
	if mcp.AITools == nil || !*mcp.AITools {
		t.Error("mcp.ai_tools must stay true; external exposure must not disable the built-in agent")
	}

	implemented := map[string]bool{}
	for _, tool := range mcpTools(false) {
		implemented[tool.Name] = true
	}
	if len(mcp.Tools) == 0 {
		t.Fatal("mcp.tools is empty; external_tools without per-tool access exposes nothing")
	}
	if len(mcp.Tools) > 64 {
		t.Errorf("mcp.tools has %d entries, want at most 64", len(mcp.Tools))
	}
	for name := range implemented {
		if _, ok := mcp.Tools[name]; !ok {
			t.Errorf("mcp.tools missing %q; undeclared tools are never exposed externally", name)
		}
	}
	for name := range mcp.Tools {
		if !implemented[name] {
			t.Errorf("mcp.tools declares %q but mcp/tools never returns it", name)
		}
	}

	readOnly := map[string]bool{}
	for _, tool := range mcpTools(true) {
		readOnly[tool.Name] = true
	}
	wantAccess := map[string]string{
		"trigger_build":     "write",
		"cancel_queue_item": "write",
		"stop_build":        "full",
	}
	validAccess := map[string]bool{"read": true, "write": true, "full": true}
	for name, entry := range mcp.Tools {
		if !validAccess[entry.Access] {
			t.Errorf("mcp.tools[%q].access = %q, want read|write|full", name, entry.Access)
		}
		if readOnly[name] {
			if entry.Access != "read" {
				t.Errorf("mcp.tools[%q].access = %q, want read", name, entry.Access)
			}
			continue
		}
		if want := wantAccess[name]; want != "" && entry.Access != want {
			t.Errorf("mcp.tools[%q].access = %q, want %q", name, entry.Access, want)
		}
		if entry.Access == "read" {
			t.Errorf("write tool %q must not be declared read", name)
		}
	}

	if len(mcp.Toolsets) == 0 {
		t.Fatal("mcp.toolsets is empty; the host groups external tools per toolset")
	}
	toolsetID := regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	declaredToolsets := map[string]bool{}
	for _, toolset := range mcp.Toolsets {
		if toolset.Title == "" {
			t.Errorf("mcp.toolsets[%q].title is required", toolset.ID)
		}
		if !toolsetID.MatchString(toolset.ID) {
			t.Errorf("mcp.toolsets[%q].id must match %s", toolset.ID, toolsetID)
		}
		if declaredToolsets[toolset.ID] {
			t.Errorf("mcp.toolsets declares %q twice", toolset.ID)
		}
		declaredToolsets[toolset.ID] = true
	}
	for name, entry := range mcp.Tools {
		if entry.Toolset == "" {
			t.Errorf("mcp.tools[%q] has no toolset, want one of %v", name, declaredToolsets)
			continue
		}
		if !declaredToolsets[entry.Toolset] {
			t.Errorf("mcp.tools[%q].toolset = %q, not declared in mcp.toolsets", name, entry.Toolset)
		}
	}
}

func conditionTargets(condition json.RawMessage, wantField string) bool {
	if len(condition) == 0 {
		return false
	}
	var decoded struct {
		Field string `json:"field"`
	}
	if err := json.Unmarshal(condition, &decoded); err != nil {
		return false
	}
	return decoded.Field == wantField
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
