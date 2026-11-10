package jenkins

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	// maxChangeMessageRunes 是变更信息截断上限，避免矩阵响应体过大。
	maxChangeMessageRunes = 200
	// maxChangeBodyRunes 是提交正文截断上限：正文只在构建详情返回，防止超长说明撑大响应。
	maxChangeBodyRunes = 2000
)

// changeItemsTree 是 changelog 条目的公共字段。
// comment（完整正文）体积大，只在构建详情里请求，别加进列表/矩阵。
const changeItemsTree = "items[commitId,msg,timestamp,date,affectedPaths,author[fullName]]"

// changeDetailItemsTree 在公共字段上多取 comment，供构建详情展示提交正文。
const changeDetailItemsTree = "items[commitId,msg,comment,timestamp,date,affectedPaths,author[fullName]]"

// rawChange 是 Jenkins changelog 里的一条提交。
type rawChange struct {
	CommitID string `json:"commitId"`
	// Message 只有首行（subject），完整多行说明在 Comment。
	Message       string   `json:"msg"`
	Comment       string   `json:"comment"`
	Timestamp     int64    `json:"timestamp"`
	Date          string   `json:"date"`
	AffectedPaths []string `json:"affectedPaths"`
	Author        struct {
		FullName string `json:"fullName"`
	} `json:"author"`
}

type rawChangeSet struct {
	Items []rawChange `json:"items"`
}

// rawParameter 是构建参数值；Value 可能是字符串、布尔或数字，统一按字符串归一化。
type rawParameter struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

// rawAction 是构建 actions 里本插件关心的两类数据：触发原因与实际参数。
type rawAction struct {
	Causes     []Cause        `json:"causes"`
	Parameters []rawParameter `json:"parameters"`
}

type rawBuild struct {
	Number            int         `json:"number"`
	Result            string      `json:"result"`
	Building          bool        `json:"building"`
	Timestamp         int64       `json:"timestamp"`
	Duration          int64       `json:"duration"`
	EstimatedDuration int64       `json:"estimatedDuration"`
	DisplayName       string      `json:"displayName"`
	Actions           []rawAction `json:"actions"`
	// Freestyle 用 changeSet，Pipeline（WorkflowRun）的 changelog 只出现在 changeSets。
	ChangeSet  rawChangeSet   `json:"changeSet"`
	ChangeSets []rawChangeSet `json:"changeSets"`
}

func (raw rawBuild) causes() []Cause {
	var causes []Cause
	for _, action := range raw.Actions {
		causes = append(causes, action.Causes...)
	}
	return causes
}

// buildParameters 汇总所有 action 里的参数：名称去空白、丢弃空名称，
// 同名参数保留首次出现的位置、值以最后一次为准，避免详情页出现重复行。
func buildParameters(actions []rawAction) []BuildParameter {
	parameters := make([]BuildParameter, 0)
	indexByName := make(map[string]int)
	for _, action := range actions {
		for _, raw := range action.Parameters {
			name := strings.TrimSpace(raw.Name)
			if name == "" {
				continue
			}
			value := anyToString(raw.Value)
			if index, ok := indexByName[name]; ok {
				parameters[index].Value = value
				continue
			}
			indexByName[name] = len(parameters)
			parameters = append(parameters, BuildParameter{Name: name, Value: value})
		}
	}
	return parameters
}

// changes 把构建的变更集归一化为阶段矩阵模型；没有变更时返回空切片而不是 nil。
func (raw rawBuild) changes() []ChangeSetItem {
	sets := make([][]rawChange, 0, 1+len(raw.ChangeSets))
	sets = append(sets, raw.ChangeSet.Items)
	for _, set := range raw.ChangeSets {
		sets = append(sets, set.Items)
	}

	changes := mergeChanges(sets...)
	for index := range changes {
		changes[index].Message = truncateChangeMessage(changes[index].Message)
		// 矩阵一次要装 10 次构建 × N 条提交，正文只属于构建详情，这里一律清掉。
		changes[index].Body = ""
	}
	return changes
}

// mergeChanges 合并单数/复数 changelog，按 commitId 去重后归一化（保留完整信息）。
func mergeChanges(sets ...[]rawChange) []ChangeSetItem {
	total := 0
	for _, items := range sets {
		total += len(items)
	}

	changes := make([]ChangeSetItem, 0, total)
	seen := make(map[string]bool, total)
	for _, items := range sets {
		for _, item := range items {
			commitID := strings.TrimSpace(item.CommitID)
			if commitID != "" {
				if seen[commitID] {
					continue
				}
				seen[commitID] = true
			}
			changes = append(changes, ChangeSetItem{
				CommitID:  commitID,
				Message:   strings.TrimSpace(item.Message),
				Author:    strings.TrimSpace(item.Author.FullName),
				Timestamp: changeTimestamp(item),
				FileCount: len(item.AffectedPaths),
				Body:      changeBody(item.Comment, item.Message),
			})
		}
	}
	return changes
}

// truncateChangeMessage 去掉首尾空白；超长时按 rune 截断并补省略号，避免截断多字节字符。
func truncateChangeMessage(message string) string {
	message = strings.TrimSpace(message)
	runes := []rune(message)
	if len(runes) <= maxChangeMessageRunes {
		return message
	}
	return string(runes[:maxChangeMessageRunes]) + "…"
}

// changeTimestamp 归一化提交时间：优先 timestamp（epoch 毫秒），
// 其次解析 date 字符串（部分 SCM 只给 "2006-01-02 15:04:05 -0700"）。
// 前端 new Date() 对后一种格式并不通用，所以统一在后端转成毫秒。
func changeTimestamp(item rawChange) int64 {
	if item.Timestamp > 0 {
		return item.Timestamp
	}
	parsed, err := time.Parse("2006-01-02 15:04:05 -0700", strings.TrimSpace(item.Date))
	if err != nil {
		return 0
	}
	return parsed.UnixMilli()
}

// changeBody 返回提交正文。Jenkins 的 msg 只有首行，完整多行说明在 comment；
// 首行与 msg 相同时剥掉，剩余部分按 rune 截断，避免超长说明撑大详情响应。
// 注意先按原始文本切行：先 TrimSpace 会把单行 comment 的换行吃掉，误判成有正文。
func changeBody(comment, subject string) string {
	normalized := strings.ReplaceAll(comment, "\r\n", "\n")
	if strings.TrimSpace(normalized) == "" {
		return ""
	}

	firstLine, rest, hasMore := strings.Cut(normalized, "\n")
	if !hasMore {
		return ""
	}

	text := strings.TrimSpace(normalized)
	if strings.TrimSpace(firstLine) == strings.TrimSpace(subject) {
		text = strings.TrimSpace(rest)
	}
	if text == "" {
		return ""
	}

	runes := []rune(text)
	if len(runes) > maxChangeBodyRunes {
		return string(runes[:maxChangeBodyRunes]) + "…"
	}
	return text
}

// GetBuilds 读取任务最近的构建列表。
func (c *Client) GetBuilds(ctx context.Context, fullName string, limit int) ([]BuildSummary, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	jobPath, err := JobPath(fullName)
	if err != nil {
		return nil, &APIError{Kind: ErrInvalidConfig, Message: err.Error()}
	}

	tree := fmt.Sprintf(
		"builds[number,result,building,timestamp,duration,estimatedDuration,displayName,actions[causes[shortDescription,userId,userName]],changeSet[%s],changeSets[%s]]{0,%d}",
		changeItemsTree, changeItemsTree, limit,
	)
	var payload struct {
		Builds []rawBuild `json:"builds"`
	}
	query := url.Values{"tree": {tree}}
	if err := c.Get(ctx, jobPath+"/api/json?"+query.Encode(), &payload); err != nil {
		return nil, err
	}

	builds := make([]BuildSummary, 0, len(payload.Builds))
	for _, raw := range payload.Builds {
		changes := raw.changes()
		builds = append(builds, BuildSummary{
			Number:            raw.Number,
			Result:            raw.Result,
			Building:          raw.Building,
			Timestamp:         raw.Timestamp,
			Duration:          raw.Duration,
			EstimatedDuration: raw.EstimatedDuration,
			DisplayName:       raw.DisplayName,
			Causes:            raw.causes(),
			Status:            StatusFromResult(raw.Result, raw.Building),
			CommitCount:       len(changes),
			Changes:           changes,
		})
	}
	return builds, nil
}

// GetBuild 读取单次构建详情。
func (c *Client) GetBuild(ctx context.Context, fullName string, number int) (*BuildDetail, error) {
	jobPath, err := JobPath(fullName)
	if err != nil {
		return nil, &APIError{Kind: ErrInvalidConfig, Message: err.Error()}
	}
	if number <= 0 {
		return nil, &APIError{Kind: ErrInvalidConfig, Message: "build number must be positive"}
	}

	tree := "number,result,building,timestamp,duration,estimatedDuration,displayName,fullDisplayName,url," +
		"actions[causes[shortDescription,userId,userName],parameters[name,value]]," +
		"changeSet[" + changeDetailItemsTree + "]," +
		"changeSets[" + changeDetailItemsTree + "]," +
		"artifacts[fileName,relativePath]"
	var payload struct {
		Number            int            `json:"number"`
		Result            string         `json:"result"`
		Building          bool           `json:"building"`
		Timestamp         int64          `json:"timestamp"`
		Duration          int64          `json:"duration"`
		EstimatedDuration int64          `json:"estimatedDuration"`
		DisplayName       string         `json:"displayName"`
		FullDisplayName   string         `json:"fullDisplayName"`
		URL               string         `json:"url"`
		Actions           []rawAction    `json:"actions"`
		ChangeSet         rawChangeSet   `json:"changeSet"`
		ChangeSets        []rawChangeSet `json:"changeSets"`
		Artifacts         []Artifact     `json:"artifacts"`
	}
	query := url.Values{"tree": {tree}}
	path := fmt.Sprintf("%s/%d/api/json?%s", jobPath, number, query.Encode())
	if err := c.Get(ctx, path, &payload); err != nil {
		return nil, err
	}

	build := &BuildDetail{
		Number:            payload.Number,
		Result:            payload.Result,
		Building:          payload.Building,
		Timestamp:         payload.Timestamp,
		Duration:          payload.Duration,
		EstimatedDuration: payload.EstimatedDuration,
		DisplayName:       payload.DisplayName,
		FullDisplayName:   payload.FullDisplayName,
		URL:               payload.URL,
		Parameters:        buildParameters(payload.Actions),
		Artifacts:         payload.Artifacts,
		Status:            StatusFromResult(payload.Result, payload.Building),
	}
	for _, action := range payload.Actions {
		build.Causes = append(build.Causes, action.Causes...)
	}
	sets := make([][]rawChange, 0, 1+len(payload.ChangeSets))
	sets = append(sets, payload.ChangeSet.Items)
	for _, set := range payload.ChangeSets {
		sets = append(sets, set.Items)
	}
	build.ChangeSet = mergeChanges(sets...)
	return build, nil
}
