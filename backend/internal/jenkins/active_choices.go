package jenkins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

const (
	parameterSessionTTL  = time.Minute
	maxParameterSessions = 8
)

type parameterSession struct {
	parameters []ParameterDefinition
	forms      map[string]activeChoiceForm
	formURL    string
	options    map[string]ParameterDefinition
	expires    time.Time
}

// GetParameterOptions 在 Jenkins 服务端求值 Active Choices 脚本，不在本地执行 Groovy。
// 同一连接内短期复用表单代理和已计算选项；不会触发 Jenkins 构建。
func (c *Client) GetParameterOptions(ctx context.Context, fullName string, values map[string]string) ([]ParameterDefinition, error) {
	lock := c.parameterJobLock(fullName)
	lock.Lock()
	defer lock.Unlock()
	return c.getParameterOptions(ctx, fullName, values, true)
}

func (c *Client) getParameterOptions(ctx context.Context, fullName string, values map[string]string, retry bool) ([]ParameterDefinition, error) {
	session := c.cachedParameterSession(fullName)
	if session == nil {
		var err error
		session, err = c.loadParameterSession(ctx, fullName)
		if err != nil {
			return nil, err
		}
		c.storeParameterSession(fullName, session)
	}

	parameters := cloneParameterDefinitions(session.parameters)
	if values == nil {
		values = map[string]string{}
	}
	normalizeParameterValues(parameters, values, false)
	for index := range parameters {
		base := parameters[index]
		form, ok := session.forms[base.Name]
		if !base.Dynamic || !ok {
			continue
		}
		refs := make([]string, 0, len(form.ReferencedParameters))
		for _, name := range form.ReferencedParameters {
			refs = append(refs, name+"="+values[name])
		}
		payload, err := json.Marshal([]string{strings.Join(refs, "__LESEP__")})
		if err != nil {
			return nil, err
		}
		key := base.Name + "\x00" + string(payload)
		if cached, ok := session.options[key]; ok {
			parameters[index] = cloneParameterDefinition(cached)
			normalizeParameterValues(parameters[:index+1], values, true)
			continue
		}

		parameter, err := c.evaluateParameter(ctx, session, form, base, payload)
		if err != nil {
			if retry && isStaleParameterProxy(err) {
				c.invalidateParameterSession(fullName, session)
				return c.getParameterOptions(ctx, fullName, values, false)
			}
			return nil, fmt.Errorf("read parameter %s choices: %w", base.Name, err)
		}
		session.options[key] = cloneParameterDefinition(parameter)
		parameters[index] = parameter
		// 上游选项刚计算出有效默认值时，必须立即用于后续级联参数。
		normalizeParameterValues(parameters[:index+1], values, true)
	}
	return parameters, nil
}

// normalizeParameterValues 在求值每个级联参数前统一上游的当前值。
// 仅修正不在候选集中的空值或占位值；用户已经选择的有效值、合法空选项保持不变。
func normalizeParameterValues(parameters []ParameterDefinition, values map[string]string, evaluated bool) {
	for _, parameter := range parameters {
		if !parameter.Dynamic && len(parameter.Choices) == 0 {
			continue
		}
		if parameter.Dynamic && len(parameter.Choices) == 0 {
			if evaluated {
				values[parameter.Name] = ""
			}
			continue
		}
		if parameter.MultiSelect {
			values[parameter.Name] = validParameterSelection(values[parameter.Name], parameter.Choices, parameter.DisabledChoices)
			if values[parameter.Name] == "" && len(parameter.SelectedValues) > 0 {
				values[parameter.Name] = validParameterSelection(strings.Join(parameter.SelectedValues, ","), parameter.Choices, parameter.DisabledChoices)
			}
			continue
		}
		current, present := values[parameter.Name]
		if present && containsString(parameter.Choices, current) && !containsString(parameter.DisabledChoices, current) {
			continue
		}
		values[parameter.Name] = ""
		if selected := validParameterSelection(strings.Join(parameter.SelectedValues, ","), parameter.Choices, parameter.DisabledChoices); selected != "" {
			values[parameter.Name] = selected
			continue
		}
		if containsString(parameter.Choices, parameter.DefaultValue) && !containsString(parameter.DisabledChoices, parameter.DefaultValue) {
			values[parameter.Name] = parameter.DefaultValue
			continue
		}
		for _, choice := range parameter.Choices {
			if !containsString(parameter.DisabledChoices, choice) {
				values[parameter.Name] = choice
				break
			}
		}
	}
}

func (c *Client) loadParameterSession(ctx context.Context, fullName string) (*parameterSession, error) {
	job, err := c.GetJob(ctx, fullName)
	if err != nil {
		return nil, err
	}
	session := &parameterSession{
		parameters: cloneParameterDefinitions(job.Parameters),
		forms:      map[string]activeChoiceForm{},
		options:    map[string]ParameterDefinition{},
		expires:    time.Now().Add(parameterSessionTTL),
	}
	dynamic := false
	for _, parameter := range job.Parameters {
		dynamic = dynamic || parameter.Dynamic
	}
	if !dynamic {
		return session, nil
	}
	jobPath, err := JobPath(fullName)
	if err != nil {
		return nil, &APIError{Kind: ErrInvalidConfig, Message: err.Error()}
	}
	body, err := c.getRaw(ctx, jobPath+"/build?delay=0sec", "text/html", http.StatusMethodNotAllowed)
	if err != nil {
		return nil, err
	}
	forms, err := parseActiveChoiceForm(body)
	if err != nil {
		return nil, err
	}
	for _, form := range forms {
		session.forms[form.Name] = form
	}
	session.formURL = c.resolve(jobPath + "/build?delay=0sec")
	return session, nil
}

func (c *Client) evaluateParameter(ctx context.Context, session *parameterSession, form activeChoiceForm, base ParameterDefinition, payload []byte) (ParameterDefinition, error) {
	parameter := cloneParameterDefinition(base)
	parameter.ReferencedParameters = append([]string(nil), form.ReferencedParameters...)
	parameter.MultiSelect = form.MultiSelect
	if len(form.ReferencedParameters) == 0 && form.initialReady {
		applyActiveChoiceValues(&parameter, form.initial.Choices, form.initial.SelectedValues, form.initial.DisabledChoices)
		return parameter, nil
	}
	if _, err := c.invokeParameterMethod(ctx, session.formURL, form.proxyURL+"/doUpdate", payload); err != nil {
		return ParameterDefinition{}, fmt.Errorf("update parameter: %w", err)
	}
	result, err := c.invokeParameterMethod(ctx, session.formURL, form.proxyURL+"/getChoicesForUI", []byte("[]"))
	if err != nil {
		return ParameterDefinition{}, err
	}
	if err := decodeActiveChoices(&parameter, result); err != nil {
		return ParameterDefinition{}, err
	}
	return parameter, nil
}

func (c *Client) parameterJobLock(fullName string) *sync.Mutex {
	c.parameterMu.Lock()
	defer c.parameterMu.Unlock()
	lock := c.parameterLocks[fullName]
	if lock == nil {
		lock = &sync.Mutex{}
		c.parameterLocks[fullName] = lock
	}
	return lock
}

func (c *Client) cachedParameterSession(fullName string) *parameterSession {
	c.parameterMu.Lock()
	defer c.parameterMu.Unlock()
	session := c.parameterSessions[fullName]
	if session == nil {
		return nil
	}
	if !time.Now().Before(session.expires) {
		delete(c.parameterSessions, fullName)
		return nil
	}
	return session
}

func (c *Client) storeParameterSession(fullName string, session *parameterSession) {
	c.parameterMu.Lock()
	defer c.parameterMu.Unlock()
	if len(c.parameterSessions) >= maxParameterSessions {
		oldestName := ""
		for name, candidate := range c.parameterSessions {
			if oldestName == "" || candidate.expires.Before(c.parameterSessions[oldestName].expires) {
				oldestName = name
			}
		}
		if oldestName != "" && oldestName != fullName {
			delete(c.parameterSessions, oldestName)
		}
	}
	c.parameterSessions[fullName] = session
}

func (c *Client) invalidateParameterSession(fullName string, session *parameterSession) {
	c.parameterMu.Lock()
	defer c.parameterMu.Unlock()
	if c.parameterSessions[fullName] == session {
		delete(c.parameterSessions, fullName)
	}
}

func isStaleParameterProxy(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && (apiErr.Kind == ErrNotFound || apiErr.Kind == ErrServer)
}

func cloneParameterDefinitions(parameters []ParameterDefinition) []ParameterDefinition {
	cloned := make([]ParameterDefinition, len(parameters))
	for index, parameter := range parameters {
		cloned[index] = cloneParameterDefinition(parameter)
	}
	return cloned
}

func cloneParameterDefinition(parameter ParameterDefinition) ParameterDefinition {
	parameter.Choices = append([]string(nil), parameter.Choices...)
	parameter.ReferencedParameters = append([]string(nil), parameter.ReferencedParameters...)
	parameter.SelectedValues = append([]string(nil), parameter.SelectedValues...)
	parameter.DisabledChoices = append([]string(nil), parameter.DisabledChoices...)
	return parameter
}

type activeChoiceForm struct {
	ParameterDefinition
	proxyURL     string
	proxyName    string
	initial      ParameterDefinition
	initialReady bool
}

func parseActiveChoiceForm(body []byte) ([]activeChoiceForm, error) {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, &APIError{Kind: ErrRequest, Message: "cannot parse Jenkins parameter form"}
	}
	parameters := []activeChoiceForm{}
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if element(node, "span") && hasClass(attr(node, "class"), "cascade-choice-parameter-data-holder") {
			name := attr(node, "data-name")
			if name != "" {
				parameters = append(parameters, activeChoiceForm{
					ParameterDefinition: ParameterDefinition{
						Name:                 name,
						Dynamic:              true,
						ReferencedParameters: splitParameterNames(attr(node, "data-referenced-parameters")),
					},
					proxyName: attr(node, "data-proxy-name"),
				})
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	for index := range parameters {
		parameter := &parameters[index]
		container := findNode(doc, func(node *html.Node) bool {
			return element(node, "div") && hasClass(attr(node, "class"), "active-choice") &&
				findInputValue(node, "name") == parameter.Name
		})
		if container == nil {
			return nil, &APIError{Kind: ErrRequest, Message: "Jenkins parameter form is incomplete"}
		}
		parameter.MultiSelect = hasMultiSelectControl(container)
		parameter.initialReady = readActiveChoiceControl(container, &parameter.initial)
		// data-proxy-name 精确绑定脚本，即使参数顺序或 ID 改变也不会串用代理。
		scripts := []*html.Node{}
		if parameter.proxyName != "" {
			scripts = findNodes(doc, func(node *html.Node) bool {
				return element(node, "script") && strings.Contains(attr(node, "src"), "var="+parameter.proxyName)
			})
		}
		if len(scripts) != 1 {
			return nil, &APIError{Kind: ErrRequest, Message: "Jenkins parameter proxy is missing"}
		}
		boundPath, err := parameterProxyPath(attr(scripts[0], "src"))
		if err != nil {
			return nil, err
		}
		parameter.proxyURL = boundPath
	}
	return parameters, nil
}

func parameterProxyPath(source string) (string, error) {
	const prefix = "/$stapler/bound/script"
	if !strings.HasPrefix(source, prefix) {
		return "", &APIError{Kind: ErrRequest, Message: "unsupported Jenkins parameter proxy"}
	}
	path := strings.TrimPrefix(source, prefix)
	if query := strings.IndexByte(path, '?'); query >= 0 {
		path = path[:query]
	}
	if !isBoundParameterPath(path) {
		return "", &APIError{Kind: ErrRequest, Message: "unsupported Jenkins parameter proxy"}
	}
	return path, nil
}

func isBoundParameterPath(path string) bool {
	id := strings.TrimPrefix(strings.Trim(path, "/"), "$stapler/bound/")
	if strings.Contains(id, "/") || len(id) != 36 {
		return false
	}
	for _, char := range id {
		if !strings.ContainsRune("0123456789abcdefABCDEF-", char) {
			return false
		}
	}
	return true
}

// invokeParameterMethod 只接受表单生成的同源 $stapler/bound 地址，不能访问任意 URL。
func (c *Client) invokeParameterMethod(ctx context.Context, formURL, path string, body []byte) ([]byte, error) {
	basePath := ""
	for _, method := range []string{"doUpdate", "getChoicesForUI"} {
		if candidate := strings.TrimSuffix(path, "/"+method); candidate != path {
			basePath = candidate
			break
		}
	}
	if basePath == "" || !isBoundParameterPath(basePath) {
		return nil, &APIError{Kind: ErrRequest, Message: "invalid Jenkins parameter proxy"}
	}
	for attempt := 0; attempt < 2; attempt++ {
		crumb, err := c.ensureCrumb(ctx)
		if err != nil {
			return nil, err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.resolve(path), strings.NewReader(string(body)))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Content-Type", "application/x-stapler-method-invocation;charset=UTF-8")
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Referer", formURL)
		if !crumb.disabled && crumb.field != "" {
			request.Header.Set(crumb.field, crumb.value)
			request.Header.Set("Crumb", crumb.value)
		}
		response, err := c.do(request)
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(io.LimitReader(response.Body, maxJSONBody))
		response.Body.Close()
		if readErr != nil {
			return nil, &APIError{Kind: ErrNetwork, Message: "cannot read Jenkins parameter options"}
		}
		if response.StatusCode == http.StatusForbidden && attempt == 0 && isStaleCrumb(data) {
			c.invalidateCrumb()
			continue
		}
		if response.StatusCode >= 400 {
			return nil, newAPIError(response.StatusCode, data)
		}
		return data, nil
	}
	return nil, &APIError{Kind: ErrForbidden, Message: "Jenkins rejected the CSRF crumb after refresh"}
}

func decodeActiveChoices(parameter *ParameterDefinition, data []byte) error {
	var response []json.RawMessage
	if err := json.Unmarshal(data, &response); err != nil || len(response) != 2 {
		return &APIError{Kind: ErrRequest, Message: "invalid Jenkins parameter options"}
	}
	var labels, keys []json.RawMessage
	if err := json.Unmarshal(response[0], &labels); err != nil {
		return &APIError{Kind: ErrRequest, Message: "invalid Jenkins parameter option labels"}
	}
	if err := json.Unmarshal(response[1], &keys); err != nil {
		return &APIError{Kind: ErrRequest, Message: "invalid Jenkins parameter option values"}
	}
	if len(labels) != len(keys) {
		return &APIError{Kind: ErrRequest, Message: "inconsistent Jenkins parameter options"}
	}
	choices := []string{}
	selectedValues := []string{}
	disabledChoices := []string{}
	for index := range labels {
		var label, key string
		if err := json.Unmarshal(labels[index], &label); err != nil {
			return &APIError{Kind: ErrRequest, Message: "invalid Jenkins parameter option label"}
		}
		if err := json.Unmarshal(keys[index], &key); err != nil {
			return &APIError{Kind: ErrRequest, Message: "invalid Jenkins parameter option value"}
		}
		value := stripChoiceMarker(key)
		selected, disabled := false, false
		for _, item := range []string{label, key} {
			selected = selected || hasChoiceMarker(item, "selected")
			disabled = disabled || hasChoiceMarker(item, "disabled")
		}
		choices = append(choices, value)
		if selected {
			selectedValues = append(selectedValues, value)
		}
		if disabled {
			disabledChoices = append(disabledChoices, value)
		}
	}
	applyActiveChoiceValues(parameter, choices, selectedValues, disabledChoices)
	return nil
}

func applyActiveChoiceValues(parameter *ParameterDefinition, choices, selected, disabled []string) {
	parameter.Choices = append([]string(nil), choices...)
	parameter.SelectedValues = append([]string(nil), selected...)
	parameter.DisabledChoices = append([]string(nil), disabled...)
	if len(parameter.SelectedValues) > 0 {
		parameter.DefaultValue = strings.Join(parameter.SelectedValues, ",")
	} else if parameter.MultiSelect {
		// 无显式选中项时保留仍有效的默认多选，失效的提示文字不能提交。
		parameter.DefaultValue = validParameterSelection(parameter.DefaultValue, parameter.Choices, parameter.DisabledChoices)
	} else if !containsString(parameter.Choices, parameter.DefaultValue) || containsString(parameter.DisabledChoices, parameter.DefaultValue) {
		parameter.DefaultValue = ""
		for _, choice := range parameter.Choices {
			if !containsString(parameter.DisabledChoices, choice) {
				parameter.DefaultValue = choice
				break
			}
		}
	}
}

func validParameterSelection(value string, choices, disabled []string) string {
	selected := []string{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if containsString(choices, item) && !containsString(disabled, item) {
			selected = append(selected, item)
		}
	}
	return strings.Join(selected, ",")
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func stripChoiceMarker(value string) string {
	for strings.HasSuffix(value, ":selected") || strings.HasSuffix(value, ":disabled") {
		value = strings.TrimSuffix(strings.TrimSuffix(value, ":selected"), ":disabled")
	}
	return value
}

func hasChoiceMarker(value, marker string) bool {
	parts := strings.Split(value, ":")
	for _, part := range parts[1:] {
		if part == marker {
			return true
		}
	}
	return false
}

func splitParameterNames(value string) []string {
	var names []string
	for _, name := range strings.Split(value, ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func isDynamicChoiceClass(class string) bool {
	return class == "org.biouno.unochoice.CascadeChoiceParameter"
}

func element(node *html.Node, name string) bool {
	return node.Type == html.ElementNode && strings.EqualFold(node.Data, name)
}

func attr(node *html.Node, name string) string {
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, name) {
			return attribute.Val
		}
	}
	return ""
}

func hasClass(class, target string) bool {
	for _, name := range strings.Fields(class) {
		if name == target {
			return true
		}
	}
	return false
}

func findNode(node *html.Node, predicate func(*html.Node) bool) *html.Node {
	if predicate(node) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findNode(child, predicate); found != nil {
			return found
		}
	}
	return nil
}

func findNodes(node *html.Node, predicate func(*html.Node) bool) []*html.Node {
	var nodes []*html.Node
	if predicate(node) {
		nodes = append(nodes, node)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		nodes = append(nodes, findNodes(child, predicate)...)
	}
	return nodes
}

func findInputValue(node *html.Node, name string) string {
	input := findNode(node, func(node *html.Node) bool {
		return element(node, "input") && attr(node, "name") == name
	})
	if input == nil {
		return ""
	}
	return attr(input, "value")
}

// readActiveChoiceControl 复用 Jenkins 表单已求值的初始选项，只有完整选项集才跳过代理调用。
func readActiveChoiceControl(container *html.Node, parameter *ParameterDefinition) bool {
	selectNode := findNode(container, func(node *html.Node) bool {
		return element(node, "select") && attr(node, "name") == "value"
	})
	if selectNode != nil {
		options := findNodes(selectNode, func(node *html.Node) bool { return element(node, "option") })
		for _, option := range options {
			parameter.Choices = append(parameter.Choices, attr(option, "value"))
			if _, ok := nodeAttr(option, "selected"); ok {
				parameter.SelectedValues = append(parameter.SelectedValues, attr(option, "value"))
			}
			if _, ok := nodeAttr(option, "disabled"); ok {
				parameter.DisabledChoices = append(parameter.DisabledChoices, attr(option, "value"))
			}
		}
		return len(options) > 0
	}
	// 模块 checkbox 在加载依赖前只有占位值，不能把它当成静态选项集。
	return false
}

func hasMultiSelectControl(node *html.Node) bool {
	return findNode(node, func(node *html.Node) bool {
		if element(node, "input") && attr(node, "name") == "value" {
			return strings.EqualFold(attr(node, "type"), "checkbox")
		}
		if element(node, "select") && attr(node, "name") == "value" {
			_, ok := nodeAttr(node, "multiple")
			return ok
		}
		return false
	}) != nil
}

func nodeAttr(node *html.Node, name string) (string, bool) {
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, name) {
			return attribute.Val, true
		}
	}
	return "", false
}
