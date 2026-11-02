package rpc

import (
	"encoding/json"
	"os"
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
