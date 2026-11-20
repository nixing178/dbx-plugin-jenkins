package jenkins

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const cachedBranchUUID = "11111111-1111-4111-8111-111111111111"

func cachedParameterForm(moduleUUID string) string {
	return fmt.Sprintf(`<html><body><form>
	<div class="active-choice"><input name="name" value="BRANCH"><select name="value">
		<option value="origin/test" selected="true">origin/test</option>
		<option value="origin/old" disabled="true">origin/old</option>
	</select></div>
	<span class="cascade-choice-parameter-data-holder" data-name="BRANCH" data-proxy-name="branch"></span>
	<script src="/$stapler/bound/script/$stapler/bound/%[1]s?var=branch"></script>
	<div class="active-choice"><input name="name" value="BUILD_MODULES"><input name="value" type="checkbox" value="请选择"></div>
	<span class="cascade-choice-parameter-data-holder" data-name="BUILD_MODULES" data-proxy-name="modules" data-referenced-parameters="BRANCH"></span>
	<script src="/$stapler/bound/script/$stapler/bound/%[2]s?var=modules"></script>
	</form></body></html>`, cachedBranchUUID, moduleUUID)
}

func cachedParameterJobJSON() string {
	return `{"name":"demo","fullName":"demo","property":[{"parameterDefinitions":[
		{"name":"BRANCH","type":"CascadeChoiceParameter","_class":"org.biouno.unochoice.CascadeChoiceParameter","defaultParameterValue":{"value":"origin/test"}},
		{"name":"BUILD_MODULES","type":"CascadeChoiceParameter","_class":"org.biouno.unochoice.CascadeChoiceParameter","defaultParameterValue":{"value":"请选择"}}
	]}]}`
}

func TestParameterOptionsReuseInitialChoicesAndCache(t *testing.T) {
	var jobRequests, formRequests, branchRequests, moduleRequests int
	var moduleUpdates []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/job/demo/api/json":
			jobRequests++
			fmt.Fprint(w, cachedParameterJobJSON())
		case "/job/demo/build":
			formRequests++
			fmt.Fprint(w, cachedParameterForm("22222222-2222-4222-8222-222222222222"))
		case "/crumbIssuer/api/json":
			fmt.Fprint(w, `{"crumb":"crumb-1","crumbRequestField":"Jenkins-Crumb"}`)
		case "/$stapler/bound/22222222-2222-4222-8222-222222222222/doUpdate":
			var payload []string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("module update: %v", err)
			}
			moduleUpdates = append(moduleUpdates, payload...)
			w.WriteHeader(http.StatusNoContent)
		case "/$stapler/bound/22222222-2222-4222-8222-222222222222/getChoicesForUI":
			moduleRequests++
			fmt.Fprint(w, `[["codec","logger"],["iot-codec","iot-log"]]`)
		default:
			if r.URL.Path == "/$stapler/bound/"+cachedBranchUUID+"/doUpdate" ||
				r.URL.Path == "/$stapler/bound/"+cachedBranchUUID+"/getChoicesForUI" {
				branchRequests++
			}
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, true)
	parameters, err := client.GetParameterOptions(context.Background(), "demo", map[string]string{"BRANCH": "origin/test"})
	if err != nil {
		t.Fatalf("first options error = %v", err)
	}
	if got := parameters[0].Choices; len(got) != 2 || got[0] != "origin/test" || got[1] != "origin/old" {
		t.Errorf("initial branch choices = %v", got)
	}
	if parameters[0].DefaultValue != "origin/test" || len(parameters[0].DisabledChoices) != 1 {
		t.Errorf("initial branch state = %+v", parameters[0])
	}
	if parameters[1].DefaultValue != "" || !parameters[1].MultiSelect {
		t.Errorf("modules = %+v", parameters[1])
	}
	if formRequests != 1 || jobRequests != 1 || branchRequests != 0 || moduleRequests != 1 {
		t.Errorf("first requests: job=%d form=%d branch=%d modules=%d", jobRequests, formRequests, branchRequests, moduleRequests)
	}

	// 调用方修改返回值不能污染缓存。
	parameters[0].Choices[0] = "changed-by-caller"
	parameters, err = client.GetParameterOptions(context.Background(), "demo", map[string]string{"BRANCH": "origin/dev"})
	if err != nil {
		t.Fatalf("second options error = %v", err)
	}
	if parameters[0].Choices[0] != "origin/test" {
		t.Errorf("cached branch was mutated: %v", parameters[0].Choices)
	}
	if len(moduleUpdates) != 2 || moduleUpdates[1] != "BRANCH=origin/dev" {
		t.Errorf("module updates = %v", moduleUpdates)
	}
	if formRequests != 1 || jobRequests != 1 || moduleRequests != 2 {
		t.Errorf("warm requests: job=%d form=%d modules=%d", jobRequests, formRequests, moduleRequests)
	}

	parameters, err = client.GetParameterOptions(context.Background(), "demo", map[string]string{"BRANCH": "origin/test"})
	if err != nil {
		t.Fatalf("cached options error = %v", err)
	}
	if formRequests != 1 || jobRequests != 1 || moduleRequests != 2 || len(moduleUpdates) != 2 {
		t.Errorf("repeat should not send requests: job=%d form=%d modules=%d updates=%v", jobRequests, formRequests, moduleRequests, moduleUpdates)
	}
	if len(parameters[1].Choices) != 2 || parameters[1].Choices[1] != "iot-log" {
		t.Errorf("cached modules = %v", parameters[1].Choices)
	}
}

func TestParameterOptionsCacheExpiryAndProxyRecovery(t *testing.T) {
	var formRequests int
	var newProxyRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/job/demo/api/json":
			fmt.Fprint(w, cachedParameterJobJSON())
		case "/job/demo/build":
			formRequests++
			if formRequests == 1 {
				fmt.Fprint(w, cachedParameterForm("22222222-2222-4222-8222-222222222222"))
			} else {
				fmt.Fprint(w, cachedParameterForm("33333333-3333-4333-8333-333333333333"))
			}
		case "/crumbIssuer/api/json":
			http.NotFound(w, r)
		case "/$stapler/bound/22222222-2222-4222-8222-222222222222/doUpdate":
			w.WriteHeader(http.StatusNoContent)
		case "/$stapler/bound/22222222-2222-4222-8222-222222222222/getChoicesForUI":
			http.NotFound(w, r)
		case "/$stapler/bound/33333333-3333-4333-8333-333333333333/doUpdate":
			newProxyRequests++
			w.WriteHeader(http.StatusNoContent)
		case "/$stapler/bound/33333333-3333-4333-8333-333333333333/getChoicesForUI":
			newProxyRequests++
			fmt.Fprint(w, `[["codec"],["iot-codec"]]`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, true)
	parameters, err := client.GetParameterOptions(context.Background(), "demo", map[string]string{"BRANCH": "origin/test"})
	if err != nil {
		t.Fatalf("proxy recovery error = %v", err)
	}
	if len(parameters[1].Choices) != 1 || parameters[1].Choices[0] != "iot-codec" {
		t.Errorf("recovered modules = %+v", parameters[1])
	}
	if formRequests != 2 || newProxyRequests != 2 {
		t.Errorf("proxy recovery requests: form=%d newProxy=%d", formRequests, newProxyRequests)
	}

	client.parameterMu.Lock()
	client.parameterSessions["demo"].expires = time.Now().Add(-time.Second)
	client.parameterMu.Unlock()
	if _, err := client.GetParameterOptions(context.Background(), "demo", map[string]string{"BRANCH": "origin/test"}); err != nil {
		t.Fatalf("expired cache reload error = %v", err)
	}
	if formRequests != 3 {
		t.Errorf("form requests after expiry = %d, want 3", formRequests)
	}
}

func TestParameterOptionsConcurrentRequestsShareSession(t *testing.T) {
	var formRequests, moduleRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/job/demo/api/json":
			fmt.Fprint(w, cachedParameterJobJSON())
		case "/job/demo/build":
			formRequests.Add(1)
			fmt.Fprint(w, cachedParameterForm("22222222-2222-4222-8222-222222222222"))
		case "/crumbIssuer/api/json":
			http.NotFound(w, r)
		case "/$stapler/bound/22222222-2222-4222-8222-222222222222/doUpdate":
			w.WriteHeader(http.StatusNoContent)
		case "/$stapler/bound/22222222-2222-4222-8222-222222222222/getChoicesForUI":
			moduleRequests.Add(1)
			time.Sleep(10 * time.Millisecond)
			fmt.Fprint(w, `[["codec"],["iot-codec"]]`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, true)
	const workers = 6
	var wg sync.WaitGroup
	errorsChannel := make(chan error, workers)
	for index := 0; index < workers; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			parameters, err := client.GetParameterOptions(context.Background(), "demo", map[string]string{"BRANCH": "origin/test"})
			if err == nil && (len(parameters) != 2 || len(parameters[1].Choices) != 1) {
				err = fmt.Errorf("unexpected parameters: %+v", parameters)
			}
			errorsChannel <- err
		}()
	}
	wg.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		if err != nil {
			t.Errorf("concurrent options error = %v", err)
		}
	}
	if formRequests.Load() != 1 || moduleRequests.Load() != 1 {
		t.Errorf("concurrent requests: form=%d modules=%d", formRequests.Load(), moduleRequests.Load())
	}
}

func TestParameterOptionsConcurrentBranchesDoNotMixValues(t *testing.T) {
	var stateMu sync.Mutex
	currentBranch := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/job/demo/api/json":
			fmt.Fprint(w, cachedParameterJobJSON())
		case "/job/demo/build":
			fmt.Fprint(w, cachedParameterForm("22222222-2222-4222-8222-222222222222"))
		case "/crumbIssuer/api/json":
			http.NotFound(w, r)
		case "/$stapler/bound/22222222-2222-4222-8222-222222222222/doUpdate":
			var payload []string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("update payload: %v", err)
			}
			stateMu.Lock()
			currentBranch = payload[0]
			stateMu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		case "/$stapler/bound/22222222-2222-4222-8222-222222222222/getChoicesForUI":
			stateMu.Lock()
			branch := currentBranch
			stateMu.Unlock()
			fmt.Fprintf(w, `[["module"],["module-%s"]]`, branch)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, true)
	branches := []string{"origin/test", "origin/dev", "origin/master", "origin/release"}
	var wg sync.WaitGroup
	for _, branch := range branches {
		branch := branch
		wg.Add(1)
		go func() {
			defer wg.Done()
			parameters, err := client.GetParameterOptions(context.Background(), "demo", map[string]string{"BRANCH": branch})
			if err != nil {
				t.Errorf("options for %s: %v", branch, err)
				return
			}
			want := "module-BRANCH=" + branch
			if len(parameters[1].Choices) != 1 || parameters[1].Choices[0] != want {
				t.Errorf("modules for %s = %v, want %s", branch, parameters[1].Choices, want)
			}
		}()
	}
	wg.Wait()
}
