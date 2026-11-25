package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRPCParameterOptionsCascade(t *testing.T) {
	var moduleUpdate []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/whoAmI/api/json":
			fmt.Fprint(w, `{"name":"alice","authenticated":true}`)
		case "/job/demo/api/json":
			fmt.Fprint(w, `{"name":"demo","fullName":"demo","property":[{"parameterDefinitions":[
				{"name":"BRANCH","type":"CascadeChoiceParameter","_class":"org.biouno.unochoice.CascadeChoiceParameter","defaultParameterValue":{"value":"origin/test"}},
				{"name":"BUILD_MODULES","type":"CascadeChoiceParameter","_class":"org.biouno.unochoice.CascadeChoiceParameter"}
			]}]}`)
		case "/job/demo/build":
			if r.Method != http.MethodGet {
				t.Errorf("parameter form method = %s", r.Method)
			}
			fmt.Fprint(w, `<div class="active-choice"><input name="name" value="BRANCH"><select name="value"></select></div>
				<span class="cascade-choice-parameter-data-holder" data-name="BRANCH" data-proxy-name="branch"></span>
				<script src="/$stapler/bound/script/$stapler/bound/11111111-1111-4111-8111-111111111111?var=branch"></script>
				<div class="active-choice"><input name="name" value="BUILD_MODULES"><input name="value" type="checkbox"></div>
				<span class="cascade-choice-parameter-data-holder" data-name="BUILD_MODULES" data-proxy-name="modules" data-referenced-parameters="BRANCH"></span>
				<script src="/$stapler/bound/script/$stapler/bound/22222222-2222-4222-8222-222222222222?var=modules"></script>`)
		case "/crumbIssuer/api/json":
			http.NotFound(w, r)
		case "/job/demo/config.xml":
			// GetJob 的静态 Active Choices 回退会尝试读取；动态表单才是选项来源。
			fmt.Fprint(w, `<flow-definition/>`)
		case "/$stapler/bound/11111111-1111-4111-8111-111111111111/doUpdate",
			"/$stapler/bound/22222222-2222-4222-8222-222222222222/doUpdate":
			if r.URL.Path == "/$stapler/bound/22222222-2222-4222-8222-222222222222/doUpdate" {
				if err := json.NewDecoder(r.Body).Decode(&moduleUpdate); err != nil {
					t.Errorf("module update: %v", err)
				}
			}
			w.WriteHeader(http.StatusNoContent)
		case "/$stapler/bound/11111111-1111-4111-8111-111111111111/getChoicesForUI":
			fmt.Fprint(w, `[["test"],["origin/test"]]`)
		case "/$stapler/bound/22222222-2222-4222-8222-222222222222/getChoicesForUI":
			fmt.Fprint(w, `[["codec"],["iot-codec"]]`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	service := connectM2(t, server, true)
	result, rpcErr := service.Handle(context.Background(), "jenkins/parameterOptions", json.RawMessage(
		`{"connectionId":"conn-1","job":"demo","params":{"BRANCH":"origin/test"}}`,
	))
	if rpcErr != nil {
		t.Fatalf("parameterOptions error = %+v", rpcErr)
	}
	response := decodeResult[struct {
		Parameters []struct {
			Name                 string   `json:"name"`
			Choices              []string `json:"choices"`
			MultiSelect          bool     `json:"multiSelect"`
			Dynamic              bool     `json:"dynamic"`
			ReferencedParameters []string `json:"referencedParameters"`
		} `json:"parameters"`
	}](t, result)
	if len(response.Parameters) != 2 || len(response.Parameters[0].Choices) != 1 {
		t.Fatalf("parameters = %+v", response.Parameters)
	}
	modules := response.Parameters[1]
	if !modules.Dynamic || !modules.MultiSelect || len(modules.ReferencedParameters) != 1 ||
		modules.ReferencedParameters[0] != "BRANCH" || modules.Choices[0] != "iot-codec" {
		t.Errorf("modules = %+v", modules)
	}
	if len(moduleUpdate) != 1 || moduleUpdate[0] != "BRANCH=origin/test" {
		t.Errorf("module update = %v", moduleUpdate)
	}
}
