package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nixing178/dbx-plugin-jenkins/internal/jenkins"
)

const pluginID = "io.github.nixing178.jenkins"

// Error 是 RPC 层的统一错误，main 会把它转换成 SDK 的 PluginError。
// Message 以 "<kind>: <detail>" 形式返回，UI 可据此做本地化。
type Error struct {
	Code    int
	Kind    string
	Message string
}

func (e *Error) Error() string { return e.Message }

// LifecycleRequest 是 DBX 生命周期请求的子集（只取插件需要的字段）。
type LifecycleRequest struct {
	Connection struct {
		ID                string         `json:"id"`
		Name              string         `json:"name"`
		Host              string         `json:"host"`
		Port              int            `json:"port"`
		Username          string         `json:"username"`
		Password          string         `json:"password"`
		APIToken          string         `json:"api_token"`
		ReadOnly          bool           `json:"read_only"`
		ExternalConfig    map[string]any `json:"external_config"`
		Secrets           map[string]any `json:"secrets"`
		ConnectionSecrets map[string]any `json:"connection_secrets"`
	} `json:"connection"`
	Runtime struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"runtime"`
	Secrets      map[string]any `json:"secrets"`
	ConnectionID string         `json:"connectionId"`

	Folder string         `json:"folder"`
	View   string         `json:"view"`
	Job    string         `json:"job"`
	Number int            `json:"number"`
	Limit  int            `json:"limit"`
	Start  int64          `json:"start"`
	ID     int            `json:"id"`
	Params map[string]any `json:"params"`

	// 制品下载：Path 是 Jenkins 的 relativePath，offset/length 是分块窗口。
	Path   string `json:"path"`
	Offset int64  `json:"offset"`
	Length int    `json:"length"`

	// 文本落盘（控制台日志导出）：Content 是要写入文件的内容。
	Content string `json:"content"`

	// TaskID 是异步下载任务的标识。
	TaskID string `json:"taskId"`

	// 批量构建：skipBusy 缺省为 true（*bool 才能区分「没传」与「显式 false」）。
	SkipBusy       *bool `json:"skipBusy"`
	MaxConcurrency int   `json:"maxConcurrency"`

	// 任务 / 视图 CRUD
	Name         string         `json:"name"`
	NewName      string         `json:"newName"`
	Kind         string         `json:"kind"`
	Description  string         `json:"description"`
	Script       string         `json:"script"`
	XML          string         `json:"xml"`
	Jobs         []string       `json:"jobs"`
	IncludeRegex string         `json:"includeRegex"`
	Recurse      bool           `json:"recurse"`
	Fields       map[string]any `json:"fields"`
}

// Service 管理每个连接对应的 Jenkins 客户端。
type Service struct {
	mu       sync.RWMutex
	sessions map[string]*jenkins.Client
}

// NewService 创建 RPC 服务。
func NewService() *Service {
	return &Service{sessions: map[string]*jenkins.Client{}}
}

// Handle 分发一个 RPC 方法。
func (s *Service) Handle(ctx context.Context, method string, raw json.RawMessage) (any, *Error) {
	if method == "mcp/tools" {
		return s.handleMCPTools(raw)
	}
	if method == "mcp/call" {
		return s.handleMCPCall(ctx, raw)
	}

	var request LifecycleRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, invalidParams("invalid request parameters")
		}
	}

	switch method {
	case "connection/test":
		client, rpcErr := buildClient(request)
		if rpcErr != nil {
			return nil, rpcErr
		}
		defer client.CloseIdleConnections()

		who, err := client.TestConnection(ctx)
		if err != nil {
			return nil, mapError(err)
		}
		return connectionResult(client, who), nil

	case "connection/connect":
		if strings.TrimSpace(request.Connection.ID) == "" {
			return nil, invalidParams("missing connection id")
		}
		client, rpcErr := buildClient(request)
		if rpcErr != nil {
			return nil, rpcErr
		}
		who, err := client.TestConnection(ctx)
		if err != nil {
			client.CloseIdleConnections()
			return nil, mapError(err)
		}

		s.mu.Lock()
		previous := s.sessions[request.Connection.ID]
		s.sessions[request.Connection.ID] = client
		s.mu.Unlock()
		if previous != nil {
			previous.CloseIdleConnections()
		}
		return connectionResult(client, who), nil

	case "connection/disconnect":
		if strings.TrimSpace(request.Connection.ID) == "" {
			return nil, invalidParams("missing connection id")
		}
		s.mu.Lock()
		previous := s.sessions[request.Connection.ID]
		delete(s.sessions, request.Connection.ID)
		s.mu.Unlock()
		if previous != nil {
			previous.CloseIdleConnections()
		}
		return map[string]any{"success": true}, nil

	case "jenkins/ping":
		return map[string]any{
			"ok":           true,
			"plugin":       pluginID,
			"language":     "go",
			"connectionId": request.ConnectionID,
		}, nil
	}

	client, rpcErr := s.session(request.ConnectionID)
	if rpcErr != nil {
		return nil, rpcErr
	}

	switch method {
	case "jenkins/whoami":
		who, err := client.TestConnection(ctx)
		if err != nil {
			return nil, mapError(err)
		}
		return connectionResult(client, who), nil

	case "jenkins/stages":
		if strings.TrimSpace(request.Job) == "" {
			return nil, invalidParams("missing job name")
		}
		matrix, err := client.GetStageMatrix(ctx, request.Job, request.Limit)
		if err != nil {
			return nil, mapError(err)
		}
		return matrix, nil

	case "jenkins/buildStages":
		if strings.TrimSpace(request.Job) == "" {
			return nil, invalidParams("missing job name")
		}
		if request.Number <= 0 {
			return nil, invalidParams("build number must be positive")
		}
		stages, err := client.GetBuildStages(ctx, request.Job, request.Number)
		if err != nil {
			return nil, mapError(err)
		}
		if stages == nil {
			stages = []jenkins.Stage{}
		}
		return map[string]any{"stages": stages}, nil

	case "jenkins/views":
		views, err := client.GetViews(ctx)
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"views": views}, nil

	case "jenkins/jobs":
		var (
			jobs []jenkins.JobSummary
			err  error
		)
		switch {
		case strings.TrimSpace(request.Folder) != "":
			jobs, err = client.GetJobs(ctx, request.Folder)
		case strings.TrimSpace(request.View) != "":
			jobs, err = client.GetViewJobs(ctx, request.View)
		default:
			jobs, err = client.GetJobs(ctx, "")
		}
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"jobs": jobs}, nil

	case "jenkins/job":
		if strings.TrimSpace(request.Job) == "" {
			return nil, invalidParams("missing job name")
		}
		job, err := client.GetJob(ctx, request.Job)
		if err != nil {
			return nil, mapError(err)
		}
		return job, nil

	case "jenkins/builds":
		if strings.TrimSpace(request.Job) == "" {
			return nil, invalidParams("missing job name")
		}
		builds, err := client.GetBuilds(ctx, request.Job, request.Limit)
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"builds": builds}, nil

	case "jenkins/build":
		if strings.TrimSpace(request.Job) == "" {
			return nil, invalidParams("missing job name")
		}
		build, err := client.GetBuild(ctx, request.Job, request.Number)
		if err != nil {
			return nil, mapError(err)
		}
		return build, nil

	case "jenkins/console":
		if strings.TrimSpace(request.Job) == "" {
			return nil, invalidParams("missing job name")
		}
		chunk, err := client.GetConsole(ctx, request.Job, request.Number, request.Start)
		if err != nil {
			return nil, mapError(err)
		}
		return chunk, nil

	case "jenkins/artifactInfo":
		if rpcErr := validateArtifactRequest(request); rpcErr != nil {
			return nil, rpcErr
		}
		info, err := client.GetArtifactInfo(ctx, request.Job, request.Number, request.Path)
		if err != nil {
			return nil, mapError(err)
		}
		return info, nil

	case "jenkins/artifactChunk":
		if rpcErr := validateArtifactRequest(request); rpcErr != nil {
			return nil, rpcErr
		}
		chunk, err := client.GetArtifactChunk(
			ctx,
			request.Job,
			request.Number,
			request.Path,
			request.Offset,
			request.Length,
		)
		if err != nil {
			return nil, mapError(err)
		}
		return chunk, nil

	case "jenkins/artifactSave":
		if rpcErr := validateArtifactRequest(request); rpcErr != nil {
			return nil, rpcErr
		}
		saved, err := client.SaveArtifact(ctx, request.Job, request.Number, request.Path)
		if err != nil {
			return nil, mapError(err)
		}
		return saved, nil

	case "jenkins/saveText":
		if strings.TrimSpace(request.Name) == "" {
			return nil, invalidParams("missing file name")
		}
		saved, err := client.SaveText(request.Name, []byte(request.Content))
		if err != nil {
			return nil, mapError(err)
		}
		return saved, nil

	case "jenkins/consoleSave":
		if strings.TrimSpace(request.Job) == "" {
			return nil, invalidParams("missing job name")
		}
		if request.Number <= 0 {
			return nil, invalidParams("build number must be positive")
		}
		snapshot, err := client.StartConsoleSave(request.Job, request.Number)
		if err != nil {
			return nil, mapError(err)
		}
		return snapshot, nil

	case "jenkins/artifactStart":
		if rpcErr := validateArtifactRequest(request); rpcErr != nil {
			return nil, rpcErr
		}
		snapshot, err := client.StartArtifactSave(request.Job, request.Number, request.Path)
		if err != nil {
			return nil, mapError(err)
		}
		return snapshot, nil

	case "jenkins/artifactStatus":
		if strings.TrimSpace(request.TaskID) == "" {
			return nil, invalidParams("missing task id")
		}
		snapshot, err := client.ArtifactTaskStatus(request.TaskID)
		if err != nil {
			return nil, mapError(err)
		}
		return snapshot, nil

	case "jenkins/artifactCancel":
		if strings.TrimSpace(request.TaskID) == "" {
			return nil, invalidParams("missing task id")
		}
		if err := client.CancelArtifactTask(request.TaskID); err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"success": true}, nil

	case "jenkins/revealFile":
		if strings.TrimSpace(request.Path) == "" {
			return nil, invalidParams("missing file path")
		}
		if err := jenkins.RevealFile(request.Path); err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"success": true}, nil

	case "jenkins/downloadDir":
		return map[string]any{"dir": client.DownloadDir()}, nil

	case "jenkins/trigger":
		if strings.TrimSpace(request.Job) == "" {
			return nil, invalidParams("missing job name")
		}
		queueID, err := client.TriggerBuild(ctx, request.Job, toForm(request.Params))
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"success": true, "queueId": queueID}, nil

	case "jenkins/stop":
		if strings.TrimSpace(request.Job) == "" {
			return nil, invalidParams("missing job name")
		}
		if err := client.StopBuild(ctx, request.Job, request.Number); err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"success": true}, nil

	case "jenkins/batchBuild":
		if len(request.Jobs) == 0 {
			return nil, invalidParams("missing job names")
		}
		skipBusy := true
		if request.SkipBusy != nil {
			skipBusy = *request.SkipBusy
		}
		results, err := client.BatchBuild(ctx, request.Jobs, jenkins.BatchBuildOptions{
			SkipBusy:       skipBusy,
			MaxConcurrency: request.MaxConcurrency,
			Params:         toForm(request.Params),
		})
		if err != nil {
			return nil, mapError(err)
		}
		if results == nil {
			results = []jenkins.BatchBuildResult{}
		}
		return map[string]any{"results": results}, nil

	case "jenkins/batchInspect":
		if len(request.Jobs) == 0 {
			return nil, invalidParams("missing job names")
		}
		targets, err := client.InspectBatch(ctx, request.Jobs, request.MaxConcurrency)
		if err != nil {
			return nil, mapError(err)
		}
		if targets == nil {
			targets = []jenkins.BatchTargetInfo{}
		}
		return map[string]any{"targets": targets}, nil

	case "jenkins/overview":
		overview, err := client.GetOverview(ctx)
		if err != nil {
			return nil, mapError(err)
		}
		return overview, nil

	case "jenkins/queue":
		items, err := client.GetQueue(ctx)
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"items": items}, nil

	case "jenkins/cancelQueue":
		if request.ID <= 0 {
			return nil, invalidParams("missing queue item id")
		}
		if err := client.CancelQueueItem(ctx, request.ID); err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"success": true}, nil

	case "jenkins/nodes":
		nodes, err := client.GetNodes(ctx)
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"nodes": nodes}, nil

	case "jenkins/executors":
		executors, err := client.GetExecutors(ctx)
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"executors": executors}, nil

	case "jenkins/jobConfig":
		if strings.TrimSpace(request.Job) == "" {
			return nil, invalidParams("missing job name")
		}
		xmlText, err := client.GetJobConfig(ctx, request.Job)
		if err != nil {
			return nil, mapError(err)
		}
		fields, err := jenkins.ParseJobFields(xmlText, request.Job)
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"xml": xmlText, "fields": fields}, nil

	case "jenkins/saveJob":
		if strings.TrimSpace(request.Job) == "" {
			return nil, invalidParams("missing job name")
		}
		if strings.TrimSpace(request.XML) == "" {
			return nil, invalidParams("missing job config XML")
		}
		if err := client.SaveJobConfig(ctx, request.Job, request.XML); err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"success": true}, nil

	case "jenkins/createJob":
		if strings.TrimSpace(request.Name) == "" {
			return nil, invalidParams("missing job name")
		}
		kind := jenkins.JobKind(strings.TrimSpace(request.Kind))
		if kind == "" {
			kind = jenkins.JobKindFreeStyle
		}
		fullName, err := client.CreateJob(
			ctx,
			request.Folder,
			request.Name,
			kind,
			request.Description,
			request.Script,
		)
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"success": true, "fullName": fullName}, nil

	case "jenkins/updateJob":
		if strings.TrimSpace(request.Job) == "" {
			return nil, invalidParams("missing job name")
		}
		fullName, err := client.UpdateJob(ctx, request.Job, jobUpdate(request.Fields))
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"success": true, "fullName": fullName}, nil

	case "jenkins/renameJob":
		if strings.TrimSpace(request.Job) == "" {
			return nil, invalidParams("missing job name")
		}
		if strings.TrimSpace(request.NewName) == "" {
			return nil, invalidParams("missing new name")
		}
		fullName, err := client.RenameJob(ctx, request.Job, request.NewName)
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"success": true, "fullName": fullName}, nil

	case "jenkins/deleteJob":
		if strings.TrimSpace(request.Job) == "" {
			return nil, invalidParams("missing job name")
		}
		if err := client.DeleteJob(ctx, request.Job); err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"success": true}, nil

	case "jenkins/viewConfig":
		if strings.TrimSpace(request.View) == "" {
			return nil, invalidParams("missing view name")
		}
		xmlText, err := client.GetViewConfig(ctx, request.View)
		if err != nil {
			return nil, mapError(err)
		}
		fields, err := jenkins.ParseListViewFields(xmlText)
		if err != nil {
			// 非 ListView（如 AllView）仍允许走原始 XML 编辑，只是不给表单字段。
			var apiErr *jenkins.APIError
			if errors.As(err, &apiErr) &&
				apiErr.Kind == jenkins.ErrInvalidConfig &&
				strings.Contains(apiErr.Message, "not a ListView") {
				return map[string]any{"xml": xmlText, "fields": nil}, nil
			}
			return nil, mapError(err)
		}
		return map[string]any{"xml": xmlText, "fields": fields}, nil

	case "jenkins/saveView":
		if strings.TrimSpace(request.View) == "" {
			return nil, invalidParams("missing view name")
		}
		if strings.TrimSpace(request.XML) == "" {
			return nil, invalidParams("missing view config XML")
		}
		if err := client.SaveViewConfig(ctx, request.View, request.XML); err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"success": true}, nil

	case "jenkins/createView":
		if strings.TrimSpace(request.Name) == "" {
			return nil, invalidParams("missing view name")
		}
		cfg := jenkins.ListViewConfig{
			Name:         request.Name,
			Description:  request.Description,
			JobNames:     request.Jobs,
			IncludeRegex: request.IncludeRegex,
			Recurse:      request.Recurse,
		}
		if err := client.CreateView(ctx, cfg); err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"success": true}, nil

	case "jenkins/updateView":
		if strings.TrimSpace(request.View) == "" {
			return nil, invalidParams("missing view name")
		}
		fullName, err := client.UpdateView(ctx, request.View, viewUpdate(request.Fields))
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"success": true, "fullName": fullName}, nil

	case "jenkins/deleteView":
		if strings.TrimSpace(request.View) == "" {
			return nil, invalidParams("missing view name")
		}
		if err := client.DeleteView(ctx, request.View); err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"success": true}, nil

	default:
		return nil, &Error{
			Code:    -32601,
			Kind:    "method_not_found",
			Message: "method not found: " + method,
		}
	}
}

func (s *Service) session(connectionID string) (*jenkins.Client, *Error) {
	if strings.TrimSpace(connectionID) == "" {
		return nil, invalidParams("missing connectionId")
	}
	s.mu.RLock()
	client := s.sessions[connectionID]
	s.mu.RUnlock()
	if client == nil {
		return nil, &Error{
			Code:    -32000,
			Kind:    "not_connected",
			Message: "connection is not established; reconnect the Jenkins connection",
		}
	}
	return client, nil
}

func buildClient(request LifecycleRequest) (*jenkins.Client, *Error) {
	host := strings.TrimSpace(request.Runtime.Host)
	if host == "" {
		host = strings.TrimSpace(request.Connection.Host)
	}
	port := request.Runtime.Port
	if port == 0 {
		port = request.Connection.Port
	}
	if host == "" || port <= 0 {
		return nil, invalidConfig("Jenkins endpoint is missing; check host and port")
	}

	scheme := configString(request.Connection.ExternalConfig, "scheme", "https")
	basePath := configString(request.Connection.ExternalConfig, "base_path", "")
	baseURL, err := jenkins.BaseURL(scheme, host, port, basePath)
	if err != nil {
		return nil, invalidConfig(err.Error())
	}

	timeoutSeconds := configInt(request.Connection.ExternalConfig, "connect_timeout_secs", 15)
	if timeoutSeconds < 1 {
		timeoutSeconds = 1
	}
	if timeoutSeconds > 300 {
		timeoutSeconds = 300
	}

	downloadDir, dirErr := jenkins.ResolveDownloadDir(
		configString(request.Connection.ExternalConfig, "download_dir", ""),
	)
	if dirErr != nil {
		return nil, mapError(dirErr)
	}

	client, err := jenkins.NewClient(jenkins.Config{
		BaseURL:            baseURL,
		Username:           strings.TrimSpace(request.Connection.Username),
		APIToken:           connectionToken(request),
		ReadOnly:           configBool(request.Connection.ExternalConfig, "read_only", request.Connection.ReadOnly),
		InsecureSkipVerify: configBool(request.Connection.ExternalConfig, "insecure_skip_verify", false),
		Timeout:            timeSeconds(timeoutSeconds),
		DownloadDir:        downloadDir,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return client, nil
}

func connectionToken(request LifecycleRequest) string {
	if token := stringValue(request.Connection.APIToken); token != "" {
		return token
	}
	for _, secrets := range []map[string]any{
		request.Connection.Secrets,
		request.Connection.ConnectionSecrets,
		request.Secrets,
	} {
		for _, key := range []string{"api_token", "token", "password"} {
			if token := stringValue(secrets[key]); token != "" {
				return token
			}
		}
	}
	return stringValue(request.Connection.Password)
}

func connectionResult(client *jenkins.Client, who *jenkins.WhoAmI) map[string]any {
	message := "Connected to Jenkins"
	if who.Version != "" {
		message = "Connected to Jenkins " + who.Version
	}
	return map[string]any{
		"success":  true,
		"message":  message,
		"readOnly": client.ReadOnly(),
		"user":     who,
	}
}

func configString(values map[string]any, key, fallback string) string {
	value, ok := values[key]
	if !ok || value == nil {
		return fallback
	}
	text := strings.TrimSpace(stringValue(value))
	if text == "" {
		return fallback
	}
	return text
}

func configBool(values map[string]any, key string, fallback bool) bool {
	value, ok := values[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	case float64:
		return typed != 0
	}
	return fallback
}

func configInt(values map[string]any, key string, fallback int) int {
	value, ok := values[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func toForm(params map[string]any) url.Values {
	form := url.Values{}
	for key, value := range params {
		switch typed := value.(type) {
		case []any:
			for _, item := range typed {
				form.Add(key, stringValue(item))
			}
		case []string:
			for _, item := range typed {
				form.Add(key, item)
			}
		default:
			form.Set(key, stringValue(value))
		}
	}
	return form
}

func timeSeconds(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}

// jobUpdate 把前端提交的部分字段转成 jenkins.JobUpdate；字段缺失表示不修改。
func jobUpdate(fields map[string]any) jenkins.JobUpdate {
	var update jenkins.JobUpdate
	if value, ok := fields["title"]; ok {
		update.SetTitle = true
		update.Title = stringValue(value)
	}
	if value, ok := fields["description"]; ok {
		update.SetDescription = true
		update.Description = stringValue(value)
	}
	if value, ok := fields["disabled"]; ok {
		update.SetDisabled = true
		update.Disabled = boolValue(value)
	}
	if value, ok := fields["script"]; ok {
		update.SetScript = true
		update.Script = stringValue(value)
	}
	if value, ok := fields["archive"]; ok {
		update.SetArchive = true
		update.Archive = archiveFromValue(value)
	}
	return update
}

// archiveFromValue 把前端提交的归档配置转成 jenkins.ArtifactArchive；
// defaultExcludes 未传时保持 Jenkins 默认的 true，避免把已有配置改成 false。
func archiveFromValue(value any) jenkins.ArtifactArchive {
	archive := jenkins.ArtifactArchive{DefaultExcludes: true}
	raw, ok := value.(map[string]any)
	if !ok {
		return archive
	}
	archive.Enabled = boolValue(raw["enabled"])
	archive.Files = stringValue(raw["files"])
	archive.Excludes = stringValue(raw["excludes"])
	archive.Fingerprint = boolValue(raw["fingerprint"])
	archive.OnlyIfSuccessful = boolValue(raw["onlyIfSuccessful"])
	archive.AllowEmptyArchive = boolValue(raw["allowEmptyArchive"])
	if _, present := raw["defaultExcludes"]; present {
		archive.DefaultExcludes = boolValue(raw["defaultExcludes"])
	}
	archive.CaseSensitive = boolValue(raw["caseSensitive"])
	return archive
}

// viewUpdate 把前端提交的部分字段转成 jenkins.ViewUpdate。
func viewUpdate(fields map[string]any) jenkins.ViewUpdate {
	var update jenkins.ViewUpdate
	if value, ok := fields["name"]; ok {
		update.SetName = true
		update.Name = stringValue(value)
	}
	if value, ok := fields["description"]; ok {
		update.SetDescription = true
		update.Description = stringValue(value)
	}
	if value, ok := fields["jobNames"]; ok {
		update.SetJobNames = true
		update.JobNames = stringList(value)
	}
	if value, ok := fields["includeRegex"]; ok {
		update.SetIncludeRegex = true
		update.IncludeRegex = stringValue(value)
	}
	if value, ok := fields["recurse"]; ok {
		update.SetRecurse = true
		update.Recurse = boolValue(value)
	}
	return update
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		return err == nil && parsed
	case float64:
		return typed != 0
	}
	return false
}

func stringList(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			result = append(result, stringValue(item))
		}
		return result
	}
	return nil
}

func mapError(err error) *Error {
	var apiErr *jenkins.APIError
	if errors.As(err, &apiErr) {
		code := -32000
		if apiErr.Kind == jenkins.ErrInvalidConfig {
			code = -32602
		}
		return &Error{
			Code:    code,
			Kind:    string(apiErr.Kind),
			Message: string(apiErr.Kind) + ": " + apiErr.Message,
		}
	}
	return &Error{Code: -32000, Kind: "internal", Message: "internal: " + err.Error()}
}

func invalidParams(message string) *Error {
	return &Error{Code: -32602, Kind: "invalid_params", Message: "invalid_params: " + message}
}

// validateArtifactRequest 校验制品下载的前置参数；路径安全由 jenkins.ArtifactPath 二次把关。
func validateArtifactRequest(request LifecycleRequest) *Error {
	if strings.TrimSpace(request.Job) == "" {
		return invalidParams("missing job name")
	}
	if request.Number <= 0 {
		return invalidParams("build number must be positive")
	}
	if strings.TrimSpace(request.Path) == "" {
		return invalidParams("missing artifact path")
	}
	return nil
}

func invalidConfig(message string) *Error {
	return &Error{
		Code:    -32602,
		Kind:    string(jenkins.ErrInvalidConfig),
		Message: string(jenkins.ErrInvalidConfig) + ": " + message,
	}
}
