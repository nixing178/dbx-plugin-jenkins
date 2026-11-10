package jenkins

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetBuilds(t *testing.T) {
	var path, tree string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		tree = r.URL.Query().Get("tree")
		fmt.Fprint(w, `{"builds":[
			{"number":12,"result":"SUCCESS","building":false,"timestamp":1700000000000,"duration":1000,"displayName":"#12","actions":[{"causes":[{"shortDescription":"Started by user Alice","userId":"alice","userName":"Alice"}]}]},
			{"number":11,"result":null,"building":true,"timestamp":1699999999000,"duration":500,"displayName":"#11","actions":[]}
		]}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	builds, err := client.GetBuilds(context.Background(), "team/demo", 5)
	if err != nil {
		t.Fatalf("GetBuilds() error = %v", err)
	}
	if path != "/job/team/job/demo/api/json" {
		t.Errorf("path = %q, want /job/team/job/demo/api/json", path)
	}
	if !strings.Contains(tree, "{0,5}") {
		t.Errorf("tree = %q, want range {0,5}", tree)
	}
	if len(builds) != 2 {
		t.Fatalf("builds = %d, want 2", len(builds))
	}
	if builds[0].Status != StatusSuccess || len(builds[0].Causes) != 1 {
		t.Errorf("build 12 = %+v, want success with one cause", builds[0])
	}
	if builds[0].Causes[0].ShortDescription != "Started by user Alice" {
		t.Errorf("cause = %+v, want user Alice", builds[0].Causes[0])
	}
	if builds[1].Status != StatusBuilding {
		t.Errorf("build 11 status = %q, want building", builds[1].Status)
	}
}

// 构建列表要显示进度条，必须带上 estimatedDuration（Jenkins 侧估算耗时）。
func TestGetBuildsReadsEstimatedDuration(t *testing.T) {
	var tree string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tree = r.URL.Query().Get("tree")
		fmt.Fprint(w, `{"builds":[
			{"number":13,"result":null,"building":true,"timestamp":1700000000000,"duration":30000,"estimatedDuration":120000}
		]}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	builds, err := client.GetBuilds(context.Background(), "demo", 5)
	if err != nil {
		t.Fatalf("GetBuilds() error = %v", err)
	}
	if !strings.Contains(tree, "estimatedDuration") {
		t.Fatalf("tree missing estimatedDuration: %s", tree)
	}
	if len(builds) != 1 || builds[0].EstimatedDuration != 120000 {
		t.Fatalf("builds = %+v, want estimatedDuration 120000", builds)
	}
}

func TestGetBuildsKeepsChangesInternal(t *testing.T) {
	var tree string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tree = r.URL.Query().Get("tree")
		fmt.Fprint(w, `{"builds":[{"number":7,"result":"SUCCESS","building":false,"changeSet":{"items":[
			{"commitId":"a1","msg":"feat: 第一处","author":{"fullName":"Alice"}},
			{"commitId":"b2","msg":"fix: 第二处","author":{"fullName":"Bob"}}
		]}}]}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	builds, err := client.GetBuilds(context.Background(), "demo", 5)
	if err != nil {
		t.Fatalf("GetBuilds() error = %v", err)
	}
	if !strings.Contains(tree, "changeSets[items[commitId,msg,timestamp,date,affectedPaths,author[fullName]]]") {
		t.Fatalf("tree missing changeSets details: %s", tree)
	}
	if len(builds) != 1 || builds[0].CommitCount != 2 {
		t.Fatalf("builds = %+v, want commitCount 2", builds)
	}
	if len(builds[0].Changes) != 2 || builds[0].Changes[1].Message != "fix: 第二处" {
		t.Errorf("changes = %+v, want two parsed entries", builds[0].Changes)
	}

	raw, err := json.Marshal(builds[0])
	if err != nil {
		t.Fatalf("marshal build: %v", err)
	}
	if strings.Contains(string(raw), `"changes"`) {
		t.Errorf("json = %s, want changes kept internal for jenkins/builds", raw)
	}
}

func TestGetBuildDetailReadsChangeSets(t *testing.T) {
	var tree string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tree = r.URL.Query().Get("tree")
		// Pipeline 构建详情：changelog 在 changeSets（复数）。
		fmt.Fprint(w, `{
			"number":2,"result":"SUCCESS","building":false,
			"changeSets":[{"items":[{"commitId":"945f1594aa","msg":"feat(export): 联调","author":{"fullName":"源哥"}}]}]
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	build, err := client.GetBuild(context.Background(), "demo", 2)
	if err != nil {
		t.Fatalf("GetBuild() error = %v", err)
	}
	if !strings.Contains(tree, "changeSets[items[commitId,msg,comment,timestamp,date,affectedPaths,author[fullName]]]") {
		t.Fatalf("tree missing changeSets details: %s", tree)
	}
	if len(build.ChangeSet) != 1 {
		t.Fatalf("changeSet = %d, want 1", len(build.ChangeSet))
	}
	if build.ChangeSet[0].CommitID != "945f1594aa" || build.ChangeSet[0].Author != "源哥" {
		t.Errorf("change = %+v", build.ChangeSet[0])
	}
}

func TestGetBuildDetail(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		fmt.Fprint(w, `{
			"number":12,"result":"FAILURE","building":false,"timestamp":1700000000000,
			"duration":1000,"estimatedDuration":2000,"displayName":"#12",
			"fullDisplayName":"team » demo #12","url":"http://ci/job/team/job/demo/12/",
			"actions":[{"causes":[{"shortDescription":"Started by timer"}]}],
			"changeSet":{"items":[{"commitId":"abc123","msg":"fix bug","author":{"fullName":"Bob"}}]},
			"artifacts":[{"fileName":"app.jar","relativePath":"target/app.jar"}]
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	build, err := client.GetBuild(context.Background(), "team/demo", 12)
	if err != nil {
		t.Fatalf("GetBuild() error = %v", err)
	}
	if path != "/job/team/job/demo/12/api/json" {
		t.Errorf("path = %q, want /job/team/job/demo/12/api/json", path)
	}
	if build.Status != StatusFailure || build.EstimatedDuration != 2000 {
		t.Errorf("build = %+v, want failure with estimate 2000", build)
	}
	if len(build.Causes) != 1 || build.Causes[0].ShortDescription != "Started by timer" {
		t.Errorf("causes = %+v, want timer", build.Causes)
	}
	if len(build.ChangeSet) != 1 || build.ChangeSet[0].CommitID != "abc123" {
		t.Errorf("changeSet = %+v, want abc123", build.ChangeSet)
	}
	if len(build.Artifacts) != 1 || build.Artifacts[0].FileName != "app.jar" {
		t.Errorf("artifacts = %+v, want app.jar", build.Artifacts)
	}
}

// Issue #7：构建详情要能直接看到本次构建实际使用的参数（环境、分支等），
// 数据来自 actions.parameters，不需要再发一次请求。
func TestGetBuildDetailReadsParameters(t *testing.T) {
	var tree string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tree = r.URL.Query().Get("tree")
		fmt.Fprint(w, `{
			"number":9,"result":"SUCCESS","building":false,
			"actions":[
				{"causes":[{"shortDescription":"Started by user Alice"}]},
				{"parameters":[
					{"name":"ENV","value":"prod"},
					{"name":"BRANCH_NAME","value":"release/1.4"},
					{"name":"SKIP_TESTS","value":true},
					{"name":"RETRY","value":2},
					{"name":"EMPTY","value":null}
				]}
			]
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	build, err := client.GetBuild(context.Background(), "demo", 9)
	if err != nil {
		t.Fatalf("GetBuild() error = %v", err)
	}
	if !strings.Contains(tree, "parameters[name,value]") {
		t.Fatalf("tree missing parameters: %s", tree)
	}

	want := []BuildParameter{
		{Name: "ENV", Value: "prod"},
		{Name: "BRANCH_NAME", Value: "release/1.4"},
		{Name: "SKIP_TESTS", Value: "true"},
		{Name: "RETRY", Value: "2"},
		{Name: "EMPTY", Value: ""},
	}
	if len(build.Parameters) != len(want) {
		t.Fatalf("parameters = %+v, want %+v", build.Parameters, want)
	}
	for index := range want {
		if build.Parameters[index] != want[index] {
			t.Errorf("parameter[%d] = %+v, want %+v", index, build.Parameters[index], want[index])
		}
	}
	if len(build.Causes) != 1 || build.Causes[0].ShortDescription != "Started by user Alice" {
		t.Errorf("causes = %+v, want the same action still parsed", build.Causes)
	}
}

// 同名参数只保留一个（位置按首次出现），值以最后一次为准；空名称直接丢弃。
func TestGetBuildDetailNormalizesParameterNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"number":10,"result":"SUCCESS","building":false,
			"actions":[{"parameters":[
				{"name":"ENV","value":"dev"},
				{"name":"   ","value":"ignored"},
				{"name":" ENV ","value":"prod"},
				{"name":"LOG_LEVEL","value":"debug"}
			]}]}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	build, err := client.GetBuild(context.Background(), "demo", 10)
	if err != nil {
		t.Fatalf("GetBuild() error = %v", err)
	}

	want := []BuildParameter{
		{Name: "ENV", Value: "prod"},
		{Name: "LOG_LEVEL", Value: "debug"},
	}
	if len(build.Parameters) != len(want) {
		t.Fatalf("parameters = %+v, want %+v", build.Parameters, want)
	}
	for index := range want {
		if build.Parameters[index] != want[index] {
			t.Errorf("parameter[%d] = %+v, want %+v", index, build.Parameters[index], want[index])
		}
	}
}

// 构建列表一次装 20 条构建，不请求参数，避免响应体随参数数量膨胀。
func TestGetBuildsDoesNotRequestParameters(t *testing.T) {
	var tree string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tree = r.URL.Query().Get("tree")
		fmt.Fprint(w, `{"builds":[]}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	if _, err := client.GetBuilds(context.Background(), "demo", 5); err != nil {
		t.Fatalf("GetBuilds() error = %v", err)
	}
	if strings.Contains(tree, "parameters") {
		t.Errorf("build list tree should not request parameters: %s", tree)
	}
}

func TestGetBuildsDefaultLimit(t *testing.T) {
	var tree string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tree = r.URL.Query().Get("tree")
		fmt.Fprint(w, `{"builds":[]}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	if _, err := client.GetBuilds(context.Background(), "demo", 0); err != nil {
		t.Fatalf("GetBuilds() error = %v", err)
	}
	if !strings.Contains(tree, "{0,20}") {
		t.Errorf("tree = %q, want default range {0,20}", tree)
	}
}

// 构建详情要把提交时间、改动文件数、正文一起取回来；这三样在 changeSet 里分别是
// timestamp / affectedPaths / comment，缺一个前端就只能显示裸哈希。
func TestGetBuildDetailReadsChangeDetails(t *testing.T) {
	var tree string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tree = r.URL.Query().Get("tree")
		fmt.Fprint(w, `{
			"number":3,"result":"SUCCESS","building":false,
			"changeSets":[{"items":[{
				"commitId":"8e45f64f84f0a6c4515058d3cf8043ef8eb90527",
				"msg":"probe B: 第二条提交",
				"comment":"probe B: 第二条提交\n\n- 明细一：验证多行消息的展示\n- 明细二：应默认折叠为单行摘要\n",
				"timestamp":1792659786000,
				"date":"2026-10-22 17:03:06 +0800",
				"author":{"fullName":"源哥"},
				"affectedPaths":["a.txt","b/c.java"]
			}]}]
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	build, err := client.GetBuild(context.Background(), "demo", 3)
	if err != nil {
		t.Fatalf("GetBuild() error = %v", err)
	}
	for _, want := range []string{"comment", "timestamp", "date", "affectedPaths", "author[fullName]"} {
		if !strings.Contains(tree, want) {
			t.Errorf("tree missing %q: %s", want, tree)
		}
	}
	if len(build.ChangeSet) != 1 {
		t.Fatalf("changeSet = %d, want 1", len(build.ChangeSet))
	}
	change := build.ChangeSet[0]
	if change.Timestamp != 1792659786000 {
		t.Errorf("timestamp = %d, want 1792659786000", change.Timestamp)
	}
	if change.FileCount != 2 {
		t.Errorf("fileCount = %d, want 2", change.FileCount)
	}
	if change.Message != "probe B: 第二条提交" {
		t.Errorf("message = %q, want subject line", change.Message)
	}
	wantBody := "- 明细一：验证多行消息的展示\n- 明细二：应默认折叠为单行摘要"
	if change.Body != wantBody {
		t.Errorf("body = %q, want %q", change.Body, wantBody)
	}
}

// 有些 SCM 只给 date 字符串（如 "2026-10-22 17:03:06 +0800"），没有 timestamp；
// 必须由后端归一化成 epoch 毫秒，前端 new Date() 解析这个格式并不通用。
func TestGetBuildChangeTimestampFallsBackToDate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"number":1,"result":"SUCCESS","building":false,
			"changeSet":{"items":[{"commitId":"abc","msg":"fix","date":"2026-10-22 17:03:06 +0800"}]}}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	build, err := client.GetBuild(context.Background(), "demo", 1)
	if err != nil {
		t.Fatalf("GetBuild() error = %v", err)
	}
	if len(build.ChangeSet) != 1 {
		t.Fatalf("changeSet = %d, want 1", len(build.ChangeSet))
	}
	want := time.Date(2026, 10, 22, 17, 3, 6, 0, time.FixedZone("", 8*3600)).UnixMilli()
	if build.ChangeSet[0].Timestamp != want {
		t.Errorf("timestamp = %d, want %d", build.ChangeSet[0].Timestamp, want)
	}
}

// 单行提交没有正文；comment 与 msg 完全相同时不能重复展示一遍。
func TestGetBuildChangeBodyEmptyForSingleLine(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"number":1,"result":"SUCCESS","building":false,
			"changeSet":{"items":[{"commitId":"abc","msg":"fix bug","comment":"fix bug\n"}]}}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	build, err := client.GetBuild(context.Background(), "demo", 1)
	if err != nil {
		t.Fatalf("GetBuild() error = %v", err)
	}
	if got := build.ChangeSet[0].Body; got != "" {
		t.Errorf("body = %q, want empty", got)
	}
}

// 阶段矩阵是 10 次构建 × N 条提交，绝不能把正文塞进去：既不请求 comment，也要清空 Body。
func TestGetStageMatrixDropsChangeBody(t *testing.T) {
	var tree string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/job/demo/api/json" {
			tree = r.URL.Query().Get("tree")
			fmt.Fprint(w, `{"builds":[{"number":2,"result":"SUCCESS","building":false,
				"changeSet":{"items":[{"commitId":"abc","msg":"fix bug",
					"comment":"fix bug\n\n- 不该出现在矩阵里",
					"timestamp":1792659786000,"affectedPaths":["a.txt"]}]}}]}`)
			return
		}
		fmt.Fprint(w, `{"stages":[{"name":"Build","status":"SUCCESS","durationMillis":100}]}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	matrix, err := client.GetStageMatrix(context.Background(), "demo", 10)
	if err != nil {
		t.Fatalf("GetStageMatrix() error = %v", err)
	}
	if strings.Contains(tree, "comment") {
		t.Errorf("matrix tree should not request comment: %s", tree)
	}
	if len(matrix.Builds) != 1 || len(matrix.Builds[0].Changes) != 1 {
		t.Fatalf("builds = %+v, want one change", matrix.Builds)
	}
	change := matrix.Builds[0].Changes[0]
	if change.Body != "" {
		t.Errorf("matrix body = %q, want empty", change.Body)
	}
	if change.Timestamp != 1792659786000 || change.FileCount != 1 {
		t.Errorf("change = %+v, want timestamp and fileCount kept", change)
	}
}
