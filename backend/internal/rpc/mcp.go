package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nixing178/dbx-plugin-jenkins/internal/jenkins"
)

// MCPTool 是暴露给 DBX 内置 AI / 外部 MCP 的工具定义。
type MCPTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations,omitempty"`
}

// MCPToolList 是 mcp/tools 的返回。
type MCPToolList struct {
	Tools []MCPTool `json:"tools"`
}

// MCPContent 是 CallToolResult 的内容块。
type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCPCallResult 是 mcp/call 的返回。
type MCPCallResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError"`
}

var writeTools = map[string]bool{
	"trigger_build":     true,
	"stop_build":        true,
	"cancel_queue_item": true,
}

func (s *Service) handleMCPTools(raw json.RawMessage) (any, *Error) {
	var request struct {
		Lifecycle    json.RawMessage `json:"lifecycle"`
		ConnectionID string          `json:"connectionId"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, invalidParams("invalid mcp/tools parameters")
		}
	}
	hasLifecycle := len(request.Lifecycle) > 0 && string(request.Lifecycle) != "null"
	// 宿主 MCP 能力页做静态发现时既没有打开连接、也不下发 lifecycle；
	// 此时返回完整工具面，读写能力由 manifest 的 access 声明和运行期只读拦截决定。
	if strings.TrimSpace(request.ConnectionID) == "" && !hasLifecycle {
		return MCPToolList{Tools: mcpTools(false)}, nil
	}
	client, rpcErr := s.mcpClient(request.ConnectionID, request.Lifecycle)
	if rpcErr != nil {
		return nil, rpcErr
	}
	tools := mcpTools(client.ReadOnly())
	// lifecycle 客户端只用于发现（外部 MCP 调用时连接未必在 DBX 里打开），不保留连接。
	if hasLifecycle {
		client.CloseIdleConnections()
	}
	return MCPToolList{Tools: tools}, nil
}

func (s *Service) handleMCPCall(ctx context.Context, raw json.RawMessage) (any, *Error) {
	var request struct {
		Tool         string          `json:"tool"`
		Arguments    map[string]any  `json:"arguments"`
		Lifecycle    json.RawMessage `json:"lifecycle"`
		ConnectionID string          `json:"connectionId"`
	}
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, invalidParams("invalid mcp/call parameters")
	}
	if strings.TrimSpace(request.Tool) == "" {
		return nil, invalidParams("missing tool name")
	}

	client, rpcErr := s.mcpClient(request.ConnectionID, request.Lifecycle)
	if rpcErr != nil {
		return nil, rpcErr
	}
	return callTool(ctx, client, request.Tool, request.Arguments), nil
}

func (s *Service) mcpClient(connectionID string, lifecycle json.RawMessage) (*jenkins.Client, *Error) {
	if len(lifecycle) > 0 && string(lifecycle) != "null" {
		var request LifecycleRequest
		if err := json.Unmarshal(lifecycle, &request); err != nil {
			return nil, invalidParams("invalid lifecycle payload")
		}
		return buildClient(request)
	}
	return s.session(connectionID)
}

func callTool(
	ctx context.Context,
	client *jenkins.Client,
	tool string,
	arguments map[string]any,
) *MCPCallResult {
	if writeTools[tool] && client.ReadOnly() {
		return mcpError("read_only: connection is in read-only mode")
	}

	switch tool {
	case "list_jobs":
		folder := argString(arguments, "folder")
		view := argString(arguments, "view")
		var (
			jobs []jenkins.JobSummary
			err  error
		)
		switch {
		case folder != "":
			jobs, err = client.GetJobs(ctx, folder)
		case view != "":
			jobs, err = client.GetViewJobs(ctx, view)
		default:
			jobs, err = client.GetJobs(ctx, "")
		}
		if err != nil {
			return mcpErrorFrom(err)
		}
		filter := strings.ToLower(argString(arguments, "filter"))
		limit := argInt(arguments, "limit", 50)
		if limit <= 0 {
			limit = 50
		}
		if limit > 200 {
			limit = 200
		}

		result := make([]map[string]any, 0, len(jobs))
		for _, job := range jobs {
			if filter != "" &&
				!strings.Contains(strings.ToLower(job.Name), filter) &&
				!strings.Contains(strings.ToLower(job.FullName), filter) {
				continue
			}
			result = append(result, map[string]any{
				"name":      job.Name,
				"fullName":  job.FullName,
				"status":    job.Status,
				"isFolder":  job.IsFolder,
				"building":  job.Building,
				"lastBuild": job.LastBuild,
			})
			if len(result) >= limit {
				break
			}
		}
		return mcpJSON(map[string]any{"jobs": result})

	case "get_job":
		job, rpcErr := requiredJob(arguments)
		if rpcErr != nil {
			return mcpError(rpcErr.Message)
		}
		detail, err := client.GetJob(ctx, job)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(detail)

	case "list_builds":
		job, rpcErr := requiredJob(arguments)
		if rpcErr != nil {
			return mcpError(rpcErr.Message)
		}
		builds, err := client.GetBuilds(ctx, job, argInt(arguments, "limit", 20))
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(map[string]any{"builds": builds})

	case "get_build":
		job, rpcErr := requiredJob(arguments)
		if rpcErr != nil {
			return mcpError(rpcErr.Message)
		}
		number := argInt(arguments, "number", 0)
		if number <= 0 {
			return mcpError("invalid_params: missing build number")
		}
		build, err := client.GetBuild(ctx, job, number)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(build)

	case "get_console_log":
		job, rpcErr := requiredJob(arguments)
		if rpcErr != nil {
			return mcpError(rpcErr.Message)
		}
		number := argInt(arguments, "number", 0)
		if number <= 0 {
			return mcpError("invalid_params: missing build number")
		}
		chunk, err := client.GetConsole(ctx, job, number, int64(argInt(arguments, "start", 0)))
		if err != nil {
			return mcpErrorFrom(err)
		}
		meta, _ := json.Marshal(map[string]any{
			"nextStart": chunk.NextStart,
			"moreData":  chunk.MoreData,
		})
		return &MCPCallResult{
			Content: []MCPContent{
				{Type: "text", Text: chunk.Text},
				{Type: "text", Text: string(meta)},
			},
		}

	case "list_queue":
		items, err := client.GetQueue(ctx)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(map[string]any{"items": items})

	case "list_nodes":
		nodes, err := client.GetNodes(ctx)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(map[string]any{"nodes": nodes})

	case "trigger_build":
		job, rpcErr := requiredJob(arguments)
		if rpcErr != nil {
			return mcpError(rpcErr.Message)
		}
		params, _ := arguments["params"].(map[string]any)
		queueID, err := client.TriggerBuild(ctx, job, toForm(params))
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(map[string]any{"success": true, "queueId": queueID})

	case "stop_build":
		job, rpcErr := requiredJob(arguments)
		if rpcErr != nil {
			return mcpError(rpcErr.Message)
		}
		number := argInt(arguments, "number", 0)
		if number <= 0 {
			return mcpError("invalid_params: missing build number")
		}
		if err := client.StopBuild(ctx, job, number); err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(map[string]any{"success": true})

	case "cancel_queue_item":
		id := argInt(arguments, "id", 0)
		if id <= 0 {
			return mcpError("invalid_params: missing queue item id")
		}
		if err := client.CancelQueueItem(ctx, id); err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(map[string]any{"success": true})

	default:
		return mcpError("unknown tool: " + tool)
	}
}

func requiredJob(arguments map[string]any) (string, *Error) {
	job := argString(arguments, "job")
	if strings.TrimSpace(job) == "" {
		return "", invalidParams("missing job name")
	}
	return job, nil
}

func argString(arguments map[string]any, key string) string {
	value, ok := arguments[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func argInt(arguments map[string]any, key string, fallback int) int {
	value, ok := arguments[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%d", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func mcpJSON(value any) *MCPCallResult {
	encoded, err := json.Marshal(value)
	if err != nil {
		return mcpError("internal: cannot encode tool result")
	}
	return &MCPCallResult{
		Content: []MCPContent{{Type: "text", Text: string(encoded)}},
	}
}

func mcpError(message string) *MCPCallResult {
	return &MCPCallResult{
		Content: []MCPContent{{Type: "text", Text: message}},
		IsError: true,
	}
}

func mcpErrorFrom(err error) *MCPCallResult {
	var apiErr *jenkins.APIError
	if errors.As(err, &apiErr) {
		return mcpError(string(apiErr.Kind) + ": " + apiErr.Message)
	}
	return mcpError("internal: " + err.Error())
}

func mcpTools(readOnly bool) []MCPTool {
	read := map[string]any{"readOnlyHint": true}

	tools := []MCPTool{
		{
			Name:        "list_jobs",
			Description: "List Jenkins jobs in a view or folder. Folders have isFolder=true. Folder takes precedence over view.",
			InputSchema: objectSchema(map[string]any{
				"folder": stringSchema("Folder full name, e.g. team/api. Omit for the root."),
				"view":   stringSchema("Jenkins view name, e.g. Builds. Ignored when folder is set."),
				"filter": stringSchema("Case-insensitive substring filter on job name."),
				"limit":  numberSchema("Maximum number of jobs (default 50, max 200)."),
			}),
			Annotations: read,
		},
		{
			Name:        "get_job",
			Description: "Get one Jenkins job, including its parameter definitions and last build.",
			InputSchema: objectSchema(map[string]any{
				"job": stringSchema("Job full name, e.g. team/demo."),
			}, "job"),
			Annotations: read,
		},
		{
			Name:        "list_builds",
			Description: "List recent builds of a Jenkins job, newest first.",
			InputSchema: objectSchema(map[string]any{
				"job":   stringSchema("Job full name, e.g. team/demo."),
				"limit": numberSchema("Maximum number of builds (default 20, max 100)."),
			}, "job"),
			Annotations: read,
		},
		{
			Name:        "get_build",
			Description: "Get one build: result, duration, causes, parameters, change set and artifacts.",
			InputSchema: objectSchema(map[string]any{
				"job":    stringSchema("Job full name, e.g. team/demo."),
				"number": numberSchema("Build number."),
			}, "job", "number"),
			Annotations: read,
		},
		{
			Name:        "get_console_log",
			Description: "Read one chunk of a build console log. Use nextStart from the metadata block to continue.",
			InputSchema: objectSchema(map[string]any{
				"job":    stringSchema("Job full name, e.g. team/demo."),
				"number": numberSchema("Build number."),
				"start":  numberSchema("Byte offset to start from (default 0)."),
			}, "job", "number"),
			Annotations: read,
		},
		{
			Name:        "list_queue",
			Description: "List queued builds with their blocking reasons.",
			InputSchema: objectSchema(map[string]any{}),
			Annotations: read,
		},
		{
			Name:        "list_nodes",
			Description: "List Jenkins nodes with online status, executors and labels.",
			InputSchema: objectSchema(map[string]any{}),
			Annotations: read,
		},
	}

	if readOnly {
		return tools
	}

	return append(tools,
		MCPTool{
			Name:        "trigger_build",
			Description: "Trigger a Jenkins build, optionally with parameters. Requires user confirmation in the host.",
			InputSchema: objectSchema(map[string]any{
				"job":    stringSchema("Job full name, e.g. team/demo."),
				"params": objectSchema(map[string]any{}),
			}, "job"),
			Annotations: map[string]any{
				"readOnlyHint":    false,
				"destructiveHint": false,
				"idempotentHint":  false,
			},
		},
		MCPTool{
			Name:        "stop_build",
			Description: "Stop a running Jenkins build. Requires user confirmation in the host.",
			InputSchema: objectSchema(map[string]any{
				"job":    stringSchema("Job full name, e.g. team/demo."),
				"number": numberSchema("Build number."),
			}, "job", "number"),
			Annotations: map[string]any{
				"readOnlyHint":    false,
				"destructiveHint": true,
				"idempotentHint":  true,
			},
		},
		MCPTool{
			Name:        "cancel_queue_item",
			Description: "Cancel a queued Jenkins build. Requires user confirmation in the host.",
			InputSchema: objectSchema(map[string]any{
				"id": numberSchema("Queue item id."),
			}, "id"),
			Annotations: map[string]any{
				"readOnlyHint":    false,
				"destructiveHint": true,
				"idempotentHint":  true,
			},
		},
	)
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func numberSchema(description string) map[string]any {
	return map[string]any{"type": "number", "description": description}
}
