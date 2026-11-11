package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeJenkinsMCP(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/whoAmI/api/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Jenkins", "2.440.3")
		fmt.Fprint(w, `{"name":"alice","authenticated":true}`)
	})
	mux.HandleFunc("/crumbIssuer/api/json", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api/json", func(w http.ResponseWriter, r *http.Request) {
		tree := r.URL.Query().Get("tree")
		if strings.HasPrefix(tree, "jobs") {
			fmt.Fprint(w, `{"jobs":[
				{"_class":"com.cloudbees.hudson.plugins.folder.Folder","name":"team","fullName":"team","color":"blue"},
				{"_class":"hudson.model.FreeStyleProject","name":"demo","fullName":"demo","color":"red","lastBuild":{"number":3,"result":"FAILURE","building":false}}
			]}`)
			return
		}
		fmt.Fprint(w, `{"mode":"NORMAL","nodeDescription":"master","numExecutors":4,"quietingDown":false}`)
	})
	mux.HandleFunc("/job/team/job/demo/api/json", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Query().Get("tree"), "builds[") {
			fmt.Fprint(w, `{"builds":[{"number":12,"result":"SUCCESS","building":false,"timestamp":1700000000000,"duration":1000,"displayName":"#12","actions":[{"causes":[{"shortDescription":"Started by user Alice"}]}]}]}`)
			return
		}
		fmt.Fprint(w, `{"name":"demo","fullName":"team/demo","description":"Build demo","color":"blue","buildable":true,
			"lastBuild":{"number":12,"result":"SUCCESS","building":false},
			"property":[{"parameterDefinitions":[{"name":"BRANCH","type":"StringParameterDefinition","defaultParameterValue":{"value":"main"}}]}]}`)
	})
	mux.HandleFunc("/job/team/job/demo/12/api/json", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"number":12,"result":"SUCCESS","building":false,"timestamp":1700000000000,"duration":1000,
			"actions":[{"causes":[{"shortDescription":"Started by user Alice"}]}],
			"changeSet":{"items":[{"commitId":"abc","msg":"fix bug","author":{"fullName":"Bob"}}]}}`)
	})
	mux.HandleFunc("/job/team/job/demo/12/logText/progressiveText", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Text-Size", "6")
		w.Header().Set("X-More-Data", "true")
		fmt.Fprint(w, "hello\n")
	})
	triggerHandler := func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("BRANCH") != "main" {
			t.Errorf("BRANCH = %q, want main", r.Form.Get("BRANCH"))
		}
		w.Header().Set("Location", "/queue/item/42/")
		w.WriteHeader(http.StatusCreated)
	}
	mux.HandleFunc("/job/team/job/demo/build", triggerHandler)
	mux.HandleFunc("/job/team/job/demo/buildWithParameters", triggerHandler)
	mux.HandleFunc("/job/team/job/demo/12/stop", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/queue/api/json", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"items":[{"id":42,"task":{"name":"team/demo"},"why":"Waiting for next available executor","stuck":false,"blocked":false,"inQueueSince":1700000000000}]}`)
	})
	mux.HandleFunc("/queue/cancelItem", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/computer/api/json", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"computer":[{"displayName":"Built-In Node","offline":false,"numExecutors":4,"idle":false,"assignedLabels":[{"name":"built-in"}],"executors":[{"idle":false},{"idle":false},{"idle":true},{"idle":true}]}]}`)
	})

	return httptest.NewServer(mux)
}

type mcpToolList struct {
	Tools []struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		InputSchema map[string]any `json:"inputSchema"`
		Annotations map[string]any `json:"annotations"`
	} `json:"tools"`
}

type mcpCallResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
}

func handleMCP(t *testing.T, service *Service, method string, payload any) any {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	result, rpcErr := service.Handle(context.Background(), method, encoded)
	if rpcErr != nil {
		t.Fatalf("%s error = %+v", method, rpcErr)
	}
	return result
}

func TestMCPToolsReadOnly(t *testing.T) {
	server := fakeJenkinsMCP(t)
	defer server.Close()
	service := connectM2(t, server, true)

	result := handleMCP(t, service, "mcp/tools", map[string]any{"connectionId": "conn-1"})
	list := decodeResult[mcpToolList](t, result)
	if len(list.Tools) != 7 {
		t.Fatalf("tools = %d, want 7 read-only tools", len(list.Tools))
	}
	names := map[string]bool{}
	for _, tool := range list.Tools {
		names[tool.Name] = true
		if tool.Description == "" || tool.InputSchema == nil {
			t.Errorf("tool %q missing description or schema", tool.Name)
		}
		if tool.Annotations["readOnlyHint"] != true {
			t.Errorf("tool %q readOnlyHint = %v, want true", tool.Name, tool.Annotations["readOnlyHint"])
		}
	}
	for _, write := range []string{"trigger_build", "stop_build", "cancel_queue_item"} {
		if names[write] {
			t.Errorf("read-only tool list contains write tool %q", write)
		}
	}
}

func TestMCPToolsWritable(t *testing.T) {
	server := fakeJenkinsMCP(t)
	defer server.Close()
	service := connectM2(t, server, false)

	result := handleMCP(t, service, "mcp/tools", map[string]any{"connectionId": "conn-1"})
	list := decodeResult[mcpToolList](t, result)
	if len(list.Tools) != 10 {
		t.Fatalf("tools = %d, want 10", len(list.Tools))
	}
	for _, tool := range list.Tools {
		switch tool.Name {
		case "trigger_build":
			if tool.Annotations["readOnlyHint"] != false {
				t.Errorf("trigger_build readOnlyHint = %v, want false", tool.Annotations["readOnlyHint"])
			}
		case "stop_build", "cancel_queue_item":
			if tool.Annotations["destructiveHint"] != true {
				t.Errorf("%s destructiveHint = %v, want true", tool.Name, tool.Annotations["destructiveHint"])
			}
		}
	}
}

func TestMCPCallListJobs(t *testing.T) {
	server := fakeJenkinsMCP(t)
	defer server.Close()
	service := connectM2(t, server, true)

	result := handleMCP(t, service, "mcp/call", map[string]any{
		"connectionId": "conn-1",
		"tool":         "list_jobs",
		"arguments":    map[string]any{"filter": "team"},
	})
	call := decodeResult[mcpCallResult](t, result)
	if call.IsError {
		t.Fatalf("call error = %+v", call)
	}
	if len(call.Content) == 0 || !strings.Contains(call.Content[0].Text, `"name":"team"`) {
		t.Errorf("content = %+v, want team folder", call.Content)
	}
	if strings.Contains(call.Content[0].Text, `"name":"demo"`) {
		t.Errorf("filter did not remove the demo job: %+v", call.Content[0].Text)
	}
}

func TestMCPCallGetJobAndBuild(t *testing.T) {
	server := fakeJenkinsMCP(t)
	defer server.Close()
	service := connectM2(t, server, true)
	ctx := context.Background()

	result := handleMCP(t, service, "mcp/call", map[string]any{
		"connectionId": "conn-1",
		"tool":         "get_job",
		"arguments":    map[string]any{"job": "team/demo"},
	})
	job := decodeResult[mcpCallResult](t, result)
	if job.IsError || !strings.Contains(job.Content[0].Text, "BRANCH") {
		t.Errorf("job result = %+v, want BRANCH parameter", job)
	}

	result = handleMCP(t, service, "mcp/call", map[string]any{
		"connectionId": "conn-1",
		"tool":         "get_build",
		"arguments":    map[string]any{"job": "team/demo", "number": 12},
	})
	build := decodeResult[mcpCallResult](t, result)
	if build.IsError || !strings.Contains(build.Content[0].Text, "abc") {
		t.Errorf("build result = %+v, want commit abc", build)
	}

	if _, rpcErr := service.Handle(ctx, "mcp/call", json.RawMessage(`{"connectionId":"conn-1","tool":"list_builds","arguments":{"job":"team/demo"}}`)); rpcErr != nil {
		t.Fatalf("list_builds error = %+v", rpcErr)
	}
}

func TestMCPCallConsole(t *testing.T) {
	server := fakeJenkinsMCP(t)
	defer server.Close()
	service := connectM2(t, server, true)

	result := handleMCP(t, service, "mcp/call", map[string]any{
		"connectionId": "conn-1",
		"tool":         "get_console_log",
		"arguments":    map[string]any{"job": "team/demo", "number": 12, "start": 0},
	})
	call := decodeResult[mcpCallResult](t, result)
	if call.IsError || len(call.Content) != 2 {
		t.Fatalf("console result = %+v, want two content blocks", call)
	}
	if call.Content[0].Text != "hello\n" {
		t.Errorf("log = %q, want hello", call.Content[0].Text)
	}
	if !strings.Contains(call.Content[1].Text, `"nextStart":6`) {
		t.Errorf("meta = %q, want nextStart 6", call.Content[1].Text)
	}
}

func TestMCPCallTriggerReadOnly(t *testing.T) {
	server := fakeJenkinsMCP(t)
	defer server.Close()
	service := connectM2(t, server, true)

	result := handleMCP(t, service, "mcp/call", map[string]any{
		"connectionId": "conn-1",
		"tool":         "trigger_build",
		"arguments":    map[string]any{"job": "team/demo"},
	})
	call := decodeResult[mcpCallResult](t, result)
	if !call.IsError || !strings.Contains(call.Content[0].Text, "read_only") {
		t.Fatalf("result = %+v, want read_only error", call)
	}
}

func TestMCPCallTriggerAndCancel(t *testing.T) {
	server := fakeJenkinsMCP(t)
	defer server.Close()
	service := connectM2(t, server, false)

	result := handleMCP(t, service, "mcp/call", map[string]any{
		"connectionId": "conn-1",
		"tool":         "trigger_build",
		"arguments":    map[string]any{"job": "team/demo", "params": map[string]any{"BRANCH": "main"}},
	})
	call := decodeResult[mcpCallResult](t, result)
	if call.IsError || !strings.Contains(call.Content[0].Text, `"queueId":42`) {
		t.Fatalf("trigger result = %+v, want queueId 42", call)
	}

	for _, test := range []struct {
		tool      string
		arguments map[string]any
	}{
		{tool: "stop_build", arguments: map[string]any{"job": "team/demo", "number": 12}},
		{tool: "cancel_queue_item", arguments: map[string]any{"id": 42}},
	} {
		result := handleMCP(t, service, "mcp/call", map[string]any{
			"connectionId": "conn-1",
			"tool":         test.tool,
			"arguments":    test.arguments,
		})
		call := decodeResult[mcpCallResult](t, result)
		if call.IsError {
			t.Errorf("%s result = %+v, want success", test.tool, call)
		}
	}
}

func TestMCPCallWithLifecycle(t *testing.T) {
	server := fakeJenkinsMCP(t)
	defer server.Close()

	var lifecycle map[string]any
	if err := json.Unmarshal(lifecyclePayload(t, server.URL, "token", "api_token", "", true), &lifecycle); err != nil {
		t.Fatalf("unmarshal lifecycle error = %v", err)
	}
	delete(lifecycle, "connectionId")

	service := NewService()
	result := handleMCP(t, service, "mcp/call", map[string]any{
		"tool":      "list_jobs",
		"arguments": map[string]any{},
		"lifecycle": lifecycle,
	})
	call := decodeResult[mcpCallResult](t, result)
	if call.IsError || !strings.Contains(call.Content[0].Text, `"name":"team"`) {
		t.Fatalf("lifecycle call = %+v, want jobs", call)
	}
}

func TestMCPCallUnknownTool(t *testing.T) {
	server := fakeJenkinsMCP(t)
	defer server.Close()
	service := connectM2(t, server, true)

	result := handleMCP(t, service, "mcp/call", map[string]any{
		"connectionId": "conn-1",
		"tool":         "nope",
		"arguments":    map[string]any{},
	})
	call := decodeResult[mcpCallResult](t, result)
	if !call.IsError || !strings.Contains(call.Content[0].Text, "unknown tool") {
		t.Fatalf("result = %+v, want unknown tool error", call)
	}
}

// TestMCPToolsWithLifecycle 钉住外部 MCP 的发现路径：
// dbx-mcp 转发时连接未必在 DBX 里打开，此时只带 lifecycle 也要能列出工具，
// 否则外部客户端看到的是空工具面。
func TestMCPToolsWithLifecycle(t *testing.T) {
	server := fakeJenkinsMCP(t)
	defer server.Close()
	service := NewService()

	for _, test := range []struct {
		name     string
		readOnly bool
		want     int
	}{
		{name: "writable", readOnly: false, want: 10},
		{name: "read_only", readOnly: true, want: 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			var lifecycle map[string]any
			payload := lifecyclePayload(t, server.URL, "token", "api_token", "", test.readOnly)
			if err := json.Unmarshal(payload, &lifecycle); err != nil {
				t.Fatalf("unmarshal lifecycle error = %v", err)
			}
			delete(lifecycle, "connectionId")

			result := handleMCP(t, service, "mcp/tools", map[string]any{"lifecycle": lifecycle})
			list := decodeResult[mcpToolList](t, result)
			if len(list.Tools) != test.want {
				t.Fatalf("tools = %d, want %d discovered via lifecycle", len(list.Tools), test.want)
			}
		})
	}
}

// TestMCPToolsWithoutContext 钉住宿主 MCP 能力页的静态发现路径。
// 设置页做工具发现时既没有打开连接、也不下发 lifecycle（真机返回
// “工具发现失败: invalid_params: missing connectionId”），此时应返回完整工具面，
// 读写能力交给 manifest 的 access 声明与运行期只读拦截决定。
func TestMCPToolsWithoutContext(t *testing.T) {
	service := NewService()
	ctx := context.Background()

	for _, raw := range []json.RawMessage{nil, json.RawMessage(`{}`), json.RawMessage(`{"connectionId":""}`)} {
		result, rpcErr := service.Handle(ctx, "mcp/tools", raw)
		if rpcErr != nil {
			t.Fatalf("mcp/tools(%s) error = %+v, want static tool list", raw, rpcErr)
		}
		list := decodeResult[mcpToolList](t, result)
		if len(list.Tools) != 10 {
			t.Fatalf("mcp/tools(%s) tools = %d, want 10", raw, len(list.Tools))
		}
		names := map[string]bool{}
		for _, tool := range list.Tools {
			names[tool.Name] = true
		}
		for _, want := range []string{"list_jobs", "trigger_build", "stop_build"} {
			if !names[want] {
				t.Errorf("mcp/tools(%s) missing tool %q in static list", raw, want)
			}
		}
	}
}
