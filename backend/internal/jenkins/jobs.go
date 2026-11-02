package jenkins

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const jobsTree = "jobs[name,fullName,color,url,_class,lastBuild[number,result,timestamp,duration,building]]"

const jobTree = "name,fullName,description,url,color,buildable,inQueue," +
	"lastBuild[number,result,timestamp,duration,building]," +
	"lastSuccessfulBuild[number],lastFailedBuild[number]," +
	"property[parameterDefinitions[name,type,_class,description,defaultParameterValue[value],choices]],_class"

// GetJobs 列出某个文件夹下的直接子任务；folder 为空表示根目录。
func (c *Client) GetJobs(ctx context.Context, folder string) ([]JobSummary, error) {
	path := "/api/json"
	trimmedFolder := strings.Trim(strings.TrimSpace(folder), "/")
	if trimmedFolder != "" {
		jobPath, err := JobPath(trimmedFolder)
		if err != nil {
			return nil, &APIError{Kind: ErrInvalidConfig, Message: err.Error()}
		}
		path = jobPath + "/api/json"
	}

	query := url.Values{"tree": {jobsTree}}
	var payload struct {
		Jobs []JobSummary `json:"jobs"`
	}
	if err := c.Get(ctx, path+"?"+query.Encode(), &payload); err != nil {
		return nil, err
	}
	return normalizeJobs(payload.Jobs, trimmedFolder), nil
}

// normalizeJobs 补齐 fullName、归一化状态、识别文件夹并按「文件夹优先、名称升序」排序。
func normalizeJobs(jobs []JobSummary, folder string) []JobSummary {
	for index := range jobs {
		job := &jobs[index]
		if job.FullName == "" {
			if folder != "" {
				job.FullName = folder + "/" + job.Name
			} else {
				job.FullName = job.Name
			}
		}
		job.Status = NormalizeColor(job.Color)
		job.Building = strings.HasSuffix(strings.ToLower(job.Color), "_anime") ||
			(job.LastBuild != nil && job.LastBuild.Building)
		if job.Status == StatusUnknown && job.LastBuild != nil {
			job.Status = StatusFromResult(job.LastBuild.Result, job.LastBuild.Building)
		}
		job.IsFolder = isFolderClass(job.Class)
	}

	sort.SliceStable(jobs, func(left, right int) bool {
		if jobs[left].IsFolder != jobs[right].IsFolder {
			return jobs[left].IsFolder
		}
		return strings.ToLower(jobs[left].Name) < strings.ToLower(jobs[right].Name)
	})
	return jobs
}

// GetJob 读取任务详情与参数定义。
func (c *Client) GetJob(ctx context.Context, fullName string) (*JobDetail, error) {
	jobPath, err := JobPath(fullName)
	if err != nil {
		return nil, &APIError{Kind: ErrInvalidConfig, Message: err.Error()}
	}

	var payload struct {
		Name                string    `json:"name"`
		FullName            string    `json:"fullName"`
		Description         string    `json:"description"`
		URL                 string    `json:"url"`
		Color               string    `json:"color"`
		Class               string    `json:"_class"`
		Buildable           bool      `json:"buildable"`
		InQueue             bool      `json:"inQueue"`
		LastBuild           *BuildRef `json:"lastBuild"`
		LastSuccessfulBuild *BuildRef `json:"lastSuccessfulBuild"`
		LastFailedBuild     *BuildRef `json:"lastFailedBuild"`
		Property            []struct {
			ParameterDefinitions []struct {
				Name                  string   `json:"name"`
				Type                  string   `json:"type"`
				Class                 string   `json:"_class"`
				Description           string   `json:"description"`
				Choices               []string `json:"choices"`
				DefaultParameterValue *struct {
					Value any `json:"value"`
				} `json:"defaultParameterValue"`
			} `json:"parameterDefinitions"`
		} `json:"property"`
	}

	query := url.Values{"tree": {jobTree}}
	if err := c.Get(ctx, jobPath+"/api/json?"+query.Encode(), &payload); err != nil {
		return nil, err
	}

	job := &JobDetail{
		Name:                payload.Name,
		FullName:            payload.FullName,
		Description:         payload.Description,
		URL:                 payload.URL,
		Color:               payload.Color,
		Class:               payload.Class,
		Buildable:           payload.Buildable,
		InQueue:             payload.InQueue,
		LastBuild:           payload.LastBuild,
		LastSuccessfulBuild: payload.LastSuccessfulBuild,
		LastFailedBuild:     payload.LastFailedBuild,
	}
	if job.FullName == "" {
		job.FullName = strings.Trim(strings.TrimSpace(fullName), "/")
	}
	job.Status = NormalizeColor(job.Color)
	if job.Status == StatusUnknown && job.LastBuild != nil {
		job.Status = StatusFromResult(job.LastBuild.Result, job.LastBuild.Building)
	}

	activeChoice := map[string]bool{}
	for _, property := range payload.Property {
		for _, definition := range property.ParameterDefinitions {
			parameter := ParameterDefinition{
				Name:        definition.Name,
				Type:        definition.Type,
				Description: definition.Description,
				Choices:     definition.Choices,
			}
			if definition.DefaultParameterValue != nil {
				parameter.DefaultValue = anyToString(definition.DefaultParameterValue.Value)
			}
			if len(definition.Choices) == 0 && isActiveChoiceClass(definition.Class) {
				activeChoice[definition.Name] = true
			}
			job.Parameters = append(job.Parameters, parameter)
		}
	}
	// Active Choices（uno-choice）参数不会在 JSON API 里返回 choices，
	// 只能从 config.xml 的脚本里取静态字面量列表，取不到就退回文本框。
	if len(activeChoice) > 0 {
		if configText, err := c.GetJobConfig(ctx, job.FullName); err == nil {
			options := activeChoiceOptions(configText, activeChoice)
			for index := range job.Parameters {
				if info, ok := options[job.Parameters[index].Name]; ok {
					job.Parameters[index].Choices = info.Choices
					job.Parameters[index].MultiSelect = info.MultiSelect
				}
			}
		}
	}
	return job, nil
}

func isActiveChoiceClass(class string) bool {
	return strings.HasPrefix(class, "org.biouno.unochoice.")
}

// activeChoiceInfo 是从任务配置里能还原出来的 Active Choices 参数信息。
type activeChoiceInfo struct {
	Choices     []string
	MultiSelect bool
}

var (
	activeChoiceBlock = regexp.MustCompile(`(?s)<org\.biouno\.unochoice\.[A-Za-z]+Parameter\b[^>]*>(.*?)</org\.biouno\.unochoice\.[A-Za-z]+Parameter>`)
	activeChoiceName  = regexp.MustCompile(`(?s)<name>([^<]*)</name>`)
	activeChoiceCode  = regexp.MustCompile(`(?s)<script>([^<]*)</script>`)
	activeChoiceType  = regexp.MustCompile(`(?s)<choiceType>([^<]*)</choiceType>`)
	literalReturn     = regexp.MustCompile(`(?s)^\s*return\s*\[(.*)\]\s*;?\s*$`)
	literalString     = regexp.MustCompile(`'([^']*)'|"([^"]*)"`)
)

// activeChoiceOptions 从任务配置里提取 Active Choices 参数的静态选项。
// 只认 `return ['a', 'b']` 这种纯字面量脚本，动态脚本一律放弃。
func activeChoiceOptions(configXML string, targets map[string]bool) map[string]activeChoiceInfo {
	options := map[string]activeChoiceInfo{}
	for _, block := range activeChoiceBlock.FindAllStringSubmatch(configXML, -1) {
		name := activeChoiceName.FindStringSubmatch(block[1])
		if name == nil {
			continue
		}
		key := strings.TrimSpace(name[1])
		if !targets[key] {
			continue
		}
		code := activeChoiceCode.FindStringSubmatch(block[1])
		if code == nil {
			continue
		}
		values := literalStrings(html.UnescapeString(code[1]))
		if len(values) == 0 {
			continue
		}
		info := activeChoiceInfo{Choices: values}
		if choiceType := activeChoiceType.FindStringSubmatch(block[1]); choiceType != nil {
			info.MultiSelect = strings.EqualFold(strings.TrimSpace(choiceType[1]), "PT_CHECKBOX")
		}
		options[key] = info
	}
	return options
}

func literalStrings(script string) []string {
	match := literalReturn.FindStringSubmatch(script)
	if match == nil {
		return nil
	}
	var values []string
	for _, item := range literalString.FindAllStringSubmatch(match[1], -1) {
		value := item[1]
		if value == "" {
			value = item[2]
		}
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func isFolderClass(class string) bool {
	return strings.Contains(class, "Folder") || strings.Contains(class, "MultiBranch")
}

func anyToString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	default:
		return fmt.Sprint(typed)
	}
}
