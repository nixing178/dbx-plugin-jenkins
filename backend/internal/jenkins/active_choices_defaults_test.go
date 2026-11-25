package jenkins

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const cascadeDefaultsForm = `<form>
<div class="active-choice"><input name="name" value="BRANCH"><select name="value"><option value="请先选择代码仓库"></option></select></div>
<span class="cascade-choice-parameter-data-holder" data-name="BRANCH" data-proxy-name="branch" data-referenced-parameters="REPOSITORY"></span>
<script src="/$stapler/bound/script/$stapler/bound/11111111-1111-4111-8111-111111111111?var=branch"></script>
<div class="active-choice"><input name="name" value="BUILD_MODULES"><input name="value" type="checkbox" value="请先选择代码仓库和分支"></div>
<span class="cascade-choice-parameter-data-holder" data-name="BUILD_MODULES" data-proxy-name="modules" data-referenced-parameters="REPOSITORY,BRANCH"></span>
<script src="/$stapler/bound/script/$stapler/bound/22222222-2222-4222-8222-222222222222?var=modules"></script>
</form>`

func TestParameterOptionsNormalizesDefaultsBeforeDependentEvaluation(t *testing.T) {
	var branchUpdates, moduleUpdates []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/job/demo/api/json":
			fmt.Fprint(w, `{"name":"demo","fullName":"demo","property":[{"parameterDefinitions":[
				{"name":"REPOSITORY","type":"ChoiceParameterDefinition","_class":"hudson.model.ChoiceParameterDefinition","choices":["repo-a","repo-b"],"defaultParameterValue":{"value":""}},
				{"name":"BRANCH","type":"CascadeChoiceParameter","_class":"org.biouno.unochoice.CascadeChoiceParameter","defaultParameterValue":{"value":"请先选择代码仓库"}},
				{"name":"BUILD_MODULES","type":"CascadeChoiceParameter","_class":"org.biouno.unochoice.CascadeChoiceParameter","defaultParameterValue":{"value":"请先选择代码仓库和分支"}}
			]}]}`)
		case "/job/demo/build":
			fmt.Fprint(w, cascadeDefaultsForm)
		case "/crumbIssuer/api/json":
			http.NotFound(w, r)
		case "/$stapler/bound/11111111-1111-4111-8111-111111111111/doUpdate",
			"/$stapler/bound/22222222-2222-4222-8222-222222222222/doUpdate":
			var payload []string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || len(payload) != 1 {
				t.Errorf("doUpdate payload = %v, %v", payload, err)
			}
			if strings.Contains(r.URL.Path, "11111111") {
				branchUpdates = append(branchUpdates, payload...)
			} else {
				moduleUpdates = append(moduleUpdates, payload...)
			}
			w.WriteHeader(http.StatusNoContent)
		case "/$stapler/bound/11111111-1111-4111-8111-111111111111/getChoicesForUI":
			fmt.Fprint(w, `[["master"],["origin/master"]]`)
		case "/$stapler/bound/22222222-2222-4222-8222-222222222222/getChoicesForUI":
			fmt.Fprint(w, `[["module-a","module-b"],["module-a","module-b"]]`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, true)
	values := map[string]string{
		"REPOSITORY":    "",
		"BRANCH":        "请先选择代码仓库",
		"BUILD_MODULES": "请先选择代码仓库和分支",
	}
	parameters, err := client.GetParameterOptions(context.Background(), "demo", values)
	if err != nil {
		t.Fatalf("GetParameterOptions() error = %v", err)
	}
	if len(branchUpdates) != 1 || branchUpdates[0] != "REPOSITORY=repo-a" {
		t.Errorf("branch updates = %v, want normalized default repository", branchUpdates)
	}
	if len(moduleUpdates) != 1 || moduleUpdates[0] != "REPOSITORY=repo-a__LESEP__BRANCH=origin/master" {
		t.Errorf("module updates = %v, want normalized repository and branch", moduleUpdates)
	}
	if strings.Contains(strings.Join(moduleUpdates, ","), "请先选择") {
		t.Errorf("placeholder leaked into dependent request: %v", moduleUpdates)
	}
	if parameters[1].DefaultValue != "origin/master" || len(parameters[2].Choices) != 2 {
		t.Errorf("parameters = %+v", parameters)
	}

	// 有效用户选择必须保留，不能被默认分支覆盖。
	moduleUpdates = nil
	values["REPOSITORY"] = "repo-b"
	values["BRANCH"] = "origin/master"
	if _, err := client.GetParameterOptions(context.Background(), "demo", values); err != nil {
		t.Fatalf("cached GetParameterOptions() error = %v", err)
	}
	if len(moduleUpdates) != 1 || moduleUpdates[0] != "REPOSITORY=repo-b__LESEP__BRANCH=origin/master" {
		t.Errorf("changed repository must recompute modules with the selected branch: %v", moduleUpdates)
	}
}

func TestNormalizeParameterValuesPreservesPlainValuesAndEmptyChoice(t *testing.T) {
	parameters := []ParameterDefinition{
		{Name: "TEXT", Type: "StringParameterDefinition", DefaultValue: "custom"},
		{Name: "ENABLED", Type: "BooleanParameterDefinition", DefaultValue: "true"},
		{Name: "OPTIONAL", Type: "ChoiceParameterDefinition", Choices: []string{"", "prod"}},
	}
	values := map[string]string{"TEXT": "custom", "ENABLED": "true", "OPTIONAL": ""}
	normalizeParameterValues(parameters, values, true)
	if values["TEXT"] != "custom" || values["ENABLED"] != "true" || values["OPTIONAL"] != "" {
		t.Errorf("plain values changed: %v", values)
	}
}
