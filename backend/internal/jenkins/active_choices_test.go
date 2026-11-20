package jenkins

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

const testActiveChoiceForm = `<!doctype html><html><body><form>
<div class="active-choice"><input name="name" value="BRANCH"><select name="value"></select></div>
<span class="cascade-choice-parameter-data-holder" data-name="BRANCH" data-proxy-name="cascadeChoiceParameter_branch" data-referenced-parameters=""></span>
<script src="/$stapler/bound/script/$stapler/bound/11111111-1111-4111-8111-111111111111?var=cascadeChoiceParameter_branch&amp;methods=doUpdate,getChoicesForUI"></script>
<div class="active-choice"><input name="name" value="BUILD_MODULES"><div><input name="value" type="checkbox" value="请选择"></div></div>
<span class="cascade-choice-parameter-data-holder" data-name="BUILD_MODULES" data-proxy-name="cascadeChoiceParameter_modules" data-referenced-parameters="BRANCH"></span>
<script src="/$stapler/bound/script/$stapler/bound/22222222-2222-4222-8222-222222222222?var=cascadeChoiceParameter_modules&amp;methods=doUpdate,getChoicesForUI"></script>
</form></body></html>`

func TestGetParameterOptionsCascade(t *testing.T) {
	var moduleUpdate []string
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/job/demo/api/json":
			fmt.Fprint(w, `{"name":"demo","fullName":"demo","property":[{"parameterDefinitions":[
				{"name":"BRANCH","type":"CascadeChoiceParameter","_class":"org.biouno.unochoice.CascadeChoiceParameter","defaultParameterValue":{"value":"origin/test"}},
				{"name":"BUILD_MODULES","type":"CascadeChoiceParameter","_class":"org.biouno.unochoice.CascadeChoiceParameter","defaultParameterValue":{"value":"请选择"}},
				{"name":"SKIP_TESTS","type":"BooleanParameterDefinition","_class":"hudson.model.BooleanParameterDefinition","defaultParameterValue":{"value":true}}
			]}]}`)
		case "/job/demo/build":
			if r.Method != http.MethodGet {
				t.Fatalf("parameter form method = %s, want GET", r.Method)
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprint(w, testActiveChoiceForm)
		case "/crumbIssuer/api/json":
			fmt.Fprint(w, `{"crumb":"crumb-1","crumbRequestField":"Jenkins-Crumb"}`)
		case "/$stapler/bound/11111111-1111-4111-8111-111111111111/doUpdate", "/$stapler/bound/22222222-2222-4222-8222-222222222222/doUpdate":
			if r.Method != http.MethodPost || r.Header.Get("Jenkins-Crumb") != "crumb-1" || r.Header.Get("Crumb") != "crumb-1" {
				t.Errorf("invalid dynamic parameter authentication headers: %+v", r.Header)
			}
			if r.Header.Get("Referer") != activeChoiceServerURL(r)+"/job/demo/build?delay=0sec" {
				t.Errorf("Referer = %q", r.Header.Get("Referer"))
			}
			var payload []string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("doUpdate payload: %v", err)
			}
			if r.URL.Path == "/$stapler/bound/22222222-2222-4222-8222-222222222222/doUpdate" {
				moduleUpdate = payload
			}
			w.WriteHeader(http.StatusNoContent)
		case "/$stapler/bound/11111111-1111-4111-8111-111111111111/getChoicesForUI":
			fmt.Fprint(w, `[["Test:selected","Disabled:disabled"],["origin/test","origin/old"]]`)
		case "/$stapler/bound/22222222-2222-4222-8222-222222222222/getChoicesForUI":
			fmt.Fprint(w, `[["codec","logger"],["iot-codec:selected","iot-log:selected"]]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	parameters, err := newTestClient(t, server.URL, true).GetParameterOptions(context.Background(), "demo", map[string]string{"BRANCH": "origin/test"})
	if err != nil {
		t.Fatalf("GetParameterOptions() error = %v", err)
	}
	if len(parameters) != 3 {
		t.Fatalf("parameters = %d, want 3", len(parameters))
	}
	if !reflect.DeepEqual(moduleUpdate, []string{"BRANCH=origin/test"}) {
		t.Errorf("module update = %v", moduleUpdate)
	}
	branch := parameters[0]
	if !reflect.DeepEqual(branch.Choices, []string{"origin/test", "origin/old"}) || branch.DefaultValue != "origin/test" {
		t.Errorf("branch = %+v", branch)
	}
	if !reflect.DeepEqual(branch.DisabledChoices, []string{"origin/old"}) {
		t.Errorf("disabled choices = %v", branch.DisabledChoices)
	}
	modules := parameters[1]
	if !modules.MultiSelect || modules.DefaultValue != "iot-codec,iot-log" {
		t.Errorf("modules = %+v", modules)
	}
	if !reflect.DeepEqual(modules.ReferencedParameters, []string{"BRANCH"}) {
		t.Errorf("referenced parameters = %v", modules.ReferencedParameters)
	}
	if parameters[2].DefaultValue != "true" || parameters[2].Dynamic {
		t.Errorf("boolean parameter changed: %+v", parameters[2])
	}
	for _, request := range requests {
		if request == "POST /job/demo/build" || request == "POST /job/demo/buildWithParameters" {
			t.Errorf("unexpected build request: %s", request)
		}
	}
}

func TestGetParameterOptionsStaticJobDoesNotReadForm(t *testing.T) {
	var formRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/job/demo/build" {
			formRequests++
		}
		fmt.Fprint(w, `{"name":"demo","fullName":"demo","property":[{"parameterDefinitions":[{"name":"ENV","type":"ChoiceParameterDefinition","choices":["dev","prod"]}]}]}`)
	}))
	defer server.Close()
	parameters, err := newTestClient(t, server.URL, true).GetParameterOptions(context.Background(), "demo", nil)
	if err != nil {
		t.Fatalf("GetParameterOptions() error = %v", err)
	}
	if formRequests != 0 || len(parameters) != 1 || len(parameters[0].Choices) != 2 {
		t.Errorf("form requests = %d, parameters = %+v", formRequests, parameters)
	}
}

func TestParameterProxyPathRejectsUnrelatedURL(t *testing.T) {
	for _, source := range []string{"https://evil.example/options", "/job/demo/build", "/other/bound/id"} {
		if path, err := parameterProxyPath(source); err == nil {
			t.Errorf("parameterProxyPath(%q) = %q, want error", source, path)
		}
	}
	path, err := parameterProxyPath("/$stapler/bound/script/$stapler/bound/11111111-1111-4111-8111-111111111111?var=x&amp;methods=doUpdate")
	if err != nil || path != "/$stapler/bound/11111111-1111-4111-8111-111111111111" {
		t.Errorf("path = %q, err = %v", path, err)
	}
}

func TestInvokeParameterMethodRejectsOutsideProxy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s", r.URL.Path)
	}))
	defer server.Close()
	_, err := newTestClient(t, server.URL, true).invokeParameterMethod(context.Background(), server.URL, "/job/demo/build", []byte("[]"))
	if err == nil {
		t.Fatal("invokeParameterMethod() error = nil")
	}
}

func activeChoiceServerURL(r *http.Request) string {
	return "http://" + r.Host
}

func TestDecodeActiveChoicesRejectsInvalidShape(t *testing.T) {
	for _, body := range []string{`{}`, `[["a"]]`, `[["a","b"],["a"]]`, `[["a"],[1]]`} {
		parameter := ParameterDefinition{}
		if err := decodeActiveChoices(&parameter, []byte(body)); err == nil {
			t.Errorf("decodeActiveChoices(%s) succeeded", body)
		}
	}
}
