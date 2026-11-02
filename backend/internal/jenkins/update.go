package jenkins

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// JobFields 是任务编辑表单需要的可读字段。
type JobFields struct {
	Title       string `json:"title"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Disabled    bool   `json:"disabled"`
	Script      string `json:"script"`
	// Archive 是「归档产物」配置；未配置或 Pipeline 任务为 nil。
	Archive *ArtifactArchive `json:"archive,omitempty"`
}

// JobUpdate 描述一次任务编辑；Set* 用于区分「未修改」与「改成空值」。
type JobUpdate struct {
	Title          string
	SetTitle       bool
	Description    string
	SetDescription bool
	Disabled       bool
	SetDisabled    bool
	Script         string
	SetScript      bool
	Archive        ArtifactArchive
	SetArchive     bool
}

// ViewUpdate 描述一次视图编辑。
type ViewUpdate struct {
	Name            string
	SetName         bool
	Description     string
	SetDescription  bool
	JobNames        []string
	SetJobNames     bool
	IncludeRegex    string
	SetIncludeRegex bool
	Recurse         bool
	SetRecurse      bool
}

type jobConfigDocument struct {
	XMLName     xml.Name
	Description string `xml:"description"`
	Disabled    bool   `xml:"disabled"`
	Definition  *struct {
		Class  string `xml:"class,attr"`
		Script string `xml:"script"`
	} `xml:"definition"`
	Publishers struct {
		Archivers []artifactArchiverXML `xml:"hudson.tasks.ArtifactArchiver"`
	} `xml:"publishers"`
}

type listViewDocument struct {
	XMLName     xml.Name
	Name        string `xml:"name"`
	Description string `xml:"description"`
	JobNames    struct {
		Strings []string `xml:"string"`
	} `xml:"jobNames"`
	IncludeRegex    string `xml:"includeRegex"`
	Recurse         bool   `xml:"recurse"`
	FilterExecutors bool   `xml:"filterExecutors"`
	FilterQueue     bool   `xml:"filterQueue"`
	Columns         struct {
		Items []struct {
			XMLName xml.Name
		} `xml:",any"`
	} `xml:"columns"`
}

// ParseJobFields 从 config.xml 读取编辑表单需要的字段。
func ParseJobFields(xmlText, fullName string) (*JobFields, error) {
	var document jobConfigDocument
	if err := xml.Unmarshal([]byte(normalizeXMLVersion(xmlText)), &document); err != nil {
		return nil, invalidXMLError(err.Error())
	}

	fields := &JobFields{
		Title:       lastSegment(fullName),
		Kind:        string(JobKindFreeStyle),
		Description: document.Description,
		Disabled:    document.Disabled,
	}
	if document.XMLName.Local == "flow-definition" {
		fields.Kind = string(JobKindPipeline)
	}
	if document.Definition != nil {
		fields.Script = document.Definition.Script
	}
	// Pipeline 的归档写在脚本里，publishers 里的同名动作不参与表单。
	if fields.Kind != string(JobKindPipeline) && len(document.Publishers.Archivers) > 0 {
		fields.Archive = document.Publishers.Archivers[0].toArchive()
	}
	return fields, nil
}

// parseListViewDocument 解析视图 XML，并确认它是 ListView。
func parseListViewDocument(xmlText string) (listViewDocument, error) {
	var document listViewDocument
	if err := xml.Unmarshal([]byte(normalizeXMLVersion(xmlText)), &document); err != nil {
		return listViewDocument{}, invalidXMLError(err.Error())
	}
	if document.XMLName.Local != listViewClass && document.XMLName.Local != "listView" {
		return listViewDocument{}, &APIError{
			Kind:    ErrInvalidConfig,
			Message: "view is not a ListView",
		}
	}
	return document, nil
}

// ParseListViewFields 从 config.xml 读取 ListView 的可编辑字段。
func ParseListViewFields(xmlText string) (ListViewConfig, error) {
	document, err := parseListViewDocument(xmlText)
	if err != nil {
		return ListViewConfig{}, err
	}
	return ListViewConfig{
		Name:         document.Name,
		Description:  document.Description,
		JobNames:     document.JobNames.Strings,
		IncludeRegex: document.IncludeRegex,
		Recurse:      document.Recurse,
	}, nil
}

// UpdateJob 按字段增量更新任务：先改名，再读原文、局部改写后写回。
func (c *Client) UpdateJob(ctx context.Context, fullName string, update JobUpdate) (string, error) {
	if c.readOnly {
		return "", &APIError{Kind: ErrReadOnly, Message: "connection is in read-only mode"}
	}

	current := strings.Trim(strings.TrimSpace(fullName), "/")
	if update.SetTitle {
		title := strings.TrimSpace(update.Title)
		if title != "" && title != lastSegment(current) {
			renamed, err := c.RenameJob(ctx, current, title)
			if err != nil {
				return "", err
			}
			current = renamed
		}
	}

	if !update.SetDescription && !update.SetDisabled && !update.SetScript && !update.SetArchive {
		return current, nil
	}

	xmlText, err := c.GetJobConfig(ctx, current)
	if err != nil {
		return "", err
	}
	if update.SetDescription {
		if xmlText, err = PatchDescription(xmlText, update.Description); err != nil {
			return "", err
		}
	}
	if update.SetDisabled {
		if xmlText, err = PatchDisabled(xmlText, update.Disabled); err != nil {
			return "", err
		}
	}
	if update.SetScript {
		if xmlText, err = PatchPipelineScript(xmlText, update.Script); err != nil {
			return "", err
		}
	}
	if update.SetArchive {
		if xmlText, err = PatchArtifactArchive(xmlText, update.Archive); err != nil {
			return "", err
		}
	}
	if err := c.SaveJobConfig(ctx, current, xmlText); err != nil {
		return "", err
	}
	return current, nil
}

// UpdateView 按字段增量更新 ListView：读原文、合并未修改字段、局部改写后写回。
// 返回最终名称（改名时为新名）。
func (c *Client) UpdateView(ctx context.Context, name string, update ViewUpdate) (string, error) {
	if c.readOnly {
		return "", &APIError{Kind: ErrReadOnly, Message: "connection is in read-only mode"}
	}
	if !update.SetName && !update.SetDescription && !update.SetJobNames &&
		!update.SetIncludeRegex && !update.SetRecurse {
		return name, nil
	}

	currentName := strings.TrimSpace(name)
	if update.SetName {
		target := strings.TrimSpace(update.Name)
		// 空名等同于「不修改」；同名由 Jenkins 视为 noop，这里直接跳过请求。
		if target != "" && target != currentName {
			if err := c.RenameView(ctx, currentName, target); err != nil {
				return "", err
			}
			currentName = target
		}
	}

	// 只改名时 RenameView 已经用原 XML 回写过一次，不必再写。
	if !update.SetDescription && !update.SetJobNames &&
		!update.SetIncludeRegex && !update.SetRecurse {
		return currentName, nil
	}

	xmlText, err := c.GetViewConfig(ctx, currentName)
	if err != nil {
		return "", err
	}
	current, err := ParseListViewFields(xmlText)
	if err != nil {
		return "", err
	}
	if update.SetDescription {
		current.Description = update.Description
	}
	if update.SetJobNames {
		current.JobNames = update.JobNames
	}
	if update.SetIncludeRegex {
		current.IncludeRegex = update.IncludeRegex
	}
	if update.SetRecurse {
		current.Recurse = update.Recurse
	}

	patched, err := PatchListView(xmlText, current)
	if err != nil {
		return "", err
	}
	if err := c.SaveViewConfig(ctx, currentName, patched); err != nil {
		return "", err
	}
	return currentName, nil
}

// RenameView 原地重命名视图。
//
// Jenkins 只有一处可以改名：POST /view/<name>/configSubmit 会调用 View.rename()；
// 而 POST config.xml（updateByXml）明确拒绝覆盖名称。configSubmit 是全量提交，
// 表单不完整会把任务与列清空，所以这里补齐 json 与 item_ 参数，并在改名后
// 用原 XML 回写一次，恢复 properties / jobFilters 等表单覆盖不到的字段。
func (c *Client) RenameView(ctx context.Context, name, newName string) error {
	if c.readOnly {
		return &APIError{Kind: ErrReadOnly, Message: "connection is in read-only mode"}
	}

	currentName := strings.TrimSpace(name)
	target := strings.TrimSpace(newName)
	if err := ValidateName(target); err != nil {
		return err
	}
	if target == currentName {
		return nil
	}
	if err := c.ensureViewNameFree(ctx, target); err != nil {
		return err
	}

	xmlText, err := c.GetViewConfig(ctx, currentName)
	if err != nil {
		return err
	}
	document, err := parseListViewDocument(xmlText)
	if err != nil {
		return err
	}

	encoded, err := json.Marshal(viewFormPayload(document))
	if err != nil {
		return &APIError{Kind: ErrRequest, Message: "cannot encode view configuration"}
	}

	form := url.Values{}
	form.Set("name", target)
	form.Set("json", string(encoded))
	for _, job := range document.JobNames.Strings {
		form.Set("item_"+job, "on")
	}
	// ListView 只在 useincluderegex 存在时才保留正则，否则会把原正则清空。
	if document.IncludeRegex != "" {
		form.Set("useincluderegex", "on")
		form.Set("includeRegex", document.IncludeRegex)
	}

	path := "/view/" + url.PathEscape(currentName) + "/configSubmit"
	if _, err := c.PostForm(ctx, path, form); err != nil {
		return err
	}

	// configSubmit 会清空 columns / properties / jobFilters，用原 XML 恢复。
	if err := c.SaveViewConfig(ctx, target, xmlText); err != nil {
		// 改名已经生效，这一步失败会留下「名字已换、配置被简化」的中间态，
		// 必须显式告诉用户，否则会被当成整体失败。
		return &APIError{
			Kind: errorKindOf(err),
			Message: fmt.Sprintf(
				"view was renamed to %q but its original configuration could not be restored: %v",
				target, err,
			),
		}
	}
	return nil
}

// errorKindOf 保留底层错误的分类，便于前端继续本地化。
func errorKindOf(err error) ErrorKind {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Kind
	}
	return ErrRequest
}

// ensureViewNameFree 按 Jenkins 的规则（区分大小写）判断视图名是否已被占用。
func (c *Client) ensureViewNameFree(ctx context.Context, name string) error {
	views, err := c.GetViews(ctx)
	if err != nil {
		return err
	}
	for _, view := range views {
		if view.Name == name {
			return &APIError{
				Kind:    ErrInvalidConfig,
				Message: fmt.Sprintf("view %q already exists", name),
			}
		}
	}
	return nil
}

type viewFormPayloadFields struct {
	Description     string              `json:"description"`
	Recurse         bool                `json:"recurse"`
	FilterExecutors bool                `json:"filterExecutors"`
	FilterQueue     bool                `json:"filterQueue"`
	Columns         []map[string]string `json:"columns"`
}

// viewFormPayload 复刻 Jenkins 视图配置页提交的结构化表单。
func viewFormPayload(document listViewDocument) viewFormPayloadFields {
	columns := make([]map[string]string, 0, len(document.Columns.Items))
	for _, item := range document.Columns.Items {
		class := item.XMLName.Local
		columns = append(columns, map[string]string{
			"stapler-class": class,
			"kind":          class,
		})
	}
	return viewFormPayloadFields{
		Description:     document.Description,
		Recurse:         document.Recurse,
		FilterExecutors: document.FilterExecutors,
		FilterQueue:     document.FilterQueue,
		Columns:         columns,
	}
}

func lastSegment(fullName string) string {
	trimmed := strings.Trim(strings.TrimSpace(fullName), "/")
	if index := strings.LastIndex(trimmed, "/"); index >= 0 {
		return trimmed[index+1:]
	}
	return trimmed
}
