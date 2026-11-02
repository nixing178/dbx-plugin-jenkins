package jenkins

import (
	"encoding/xml"
	"io"
	"strconv"
	"strings"
)

// ArtifactArchive 是「归档产物」（Archive the artifacts）post-build action 的可编辑字段。
type ArtifactArchive struct {
	Enabled           bool   `json:"enabled"`
	Files             string `json:"files"`
	Excludes          string `json:"excludes"`
	Fingerprint       bool   `json:"fingerprint"`
	OnlyIfSuccessful  bool   `json:"onlyIfSuccessful"`
	AllowEmptyArchive bool   `json:"allowEmptyArchive"`
	DefaultExcludes   bool   `json:"defaultExcludes"`
	CaseSensitive     bool   `json:"caseSensitive"`
}

// artifactArchiverXML 是 config.xml 里 hudson.tasks.ArtifactArchiver 的结构。
type artifactArchiverXML struct {
	Artifacts         string `xml:"artifacts"`
	Excludes          string `xml:"excludes"`
	Fingerprint       *bool  `xml:"fingerprint"`
	AllowEmptyArchive *bool  `xml:"allowEmptyArchive"`
	OnlyIfSuccessful  *bool  `xml:"onlyIfSuccessful"`
	DefaultExcludes   *bool  `xml:"defaultExcludes"`
	CaseSensitive     *bool  `xml:"caseSensitive"`
}

func (raw artifactArchiverXML) toArchive() *ArtifactArchive {
	return &ArtifactArchive{
		Enabled:           true,
		Files:             raw.Artifacts,
		Excludes:          raw.Excludes,
		Fingerprint:       boolOr(raw.Fingerprint, false),
		AllowEmptyArchive: boolOr(raw.AllowEmptyArchive, false),
		OnlyIfSuccessful:  boolOr(raw.OnlyIfSuccessful, false),
		// Jenkins 的默认值：默认排除开启、区分大小写关闭。
		DefaultExcludes: boolOr(raw.DefaultExcludes, true),
		CaseSensitive:   boolOr(raw.CaseSensitive, false),
	}
}

func boolOr(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

// PatchArtifactArchive 在任务配置里增删改 hudson.tasks.ArtifactArchiver，
// 其它 post-build action 原样保留。
func PatchArtifactArchive(xmlText string, cfg ArtifactArchive) (string, error) {
	root, elements, err := scanElements(xmlText)
	if err != nil {
		return "", err
	}
	if root == "flow-definition" {
		return "", &APIError{
			Kind:    ErrInvalidConfig,
			Message: "Pipeline jobs archive artifacts in their script; edit the pipeline instead",
		}
	}
	if cfg.Enabled && strings.TrimSpace(cfg.Files) == "" {
		return "", &APIError{Kind: ErrInvalidConfig, Message: "artifacts pattern is required"}
	}

	archiver := artifactArchiverXMLString(cfg)

	publishers := -1
	for index := range elements {
		if elements[index].name == "publishers" {
			publishers = index
			break
		}
	}
	if publishers < 0 {
		if !cfg.Enabled {
			return xmlText, nil
		}
		return insertBeforeRootEnd(
			xmlText,
			"<publishers>\n    "+archiver+"\n  </publishers>",
		)
	}

	element := elements[publishers]
	tag := xmlText[element.start:element.end]
	if strings.HasSuffix(strings.TrimSpace(tag), "/>") {
		if !cfg.Enabled {
			return xmlText, nil
		}
		expanded := "<publishers>\n    " + archiver + "\n  </publishers>"
		return xmlText[:element.start] + expanded + xmlText[element.end:], nil
	}

	openEnd := element.start + strings.Index(tag, ">") + 1
	closeOffset := strings.LastIndex(tag, "</publishers>")
	if closeOffset < 0 {
		return "", invalidXMLError("publishers element is not closed")
	}
	closeStart := element.start + closeOffset
	inner := xmlText[openEnd:closeStart]

	start, end, found, err := findChildSpan(inner, "hudson.tasks.ArtifactArchiver")
	if err != nil {
		return "", err
	}

	switch {
	case found && cfg.Enabled:
		inner = inner[:start] + archiver + inner[end:]
	case found && !cfg.Enabled:
		inner = inner[:start] + inner[end:]
	case !found && cfg.Enabled:
		if strings.TrimSpace(inner) == "" {
			inner = "\n    " + archiver + "\n  "
		} else {
			inner = strings.TrimRight(inner, " \t\r\n") + "\n    " + archiver + "\n  "
		}
	default:
		return xmlText, nil
	}

	return xmlText[:openEnd] + inner + xmlText[closeStart:], nil
}

func artifactArchiverXMLString(cfg ArtifactArchive) string {
	var builder strings.Builder
	builder.WriteString("<hudson.tasks.ArtifactArchiver>")
	builder.WriteString("<artifacts>" + escapeXMLText(strings.TrimSpace(cfg.Files)) + "</artifacts>")
	builder.WriteString("<excludes>" + escapeXMLText(strings.TrimSpace(cfg.Excludes)) + "</excludes>")
	builder.WriteString("<fingerprint>" + strconv.FormatBool(cfg.Fingerprint) + "</fingerprint>")
	builder.WriteString("<allowEmptyArchive>" + strconv.FormatBool(cfg.AllowEmptyArchive) + "</allowEmptyArchive>")
	builder.WriteString("<onlyIfSuccessful>" + strconv.FormatBool(cfg.OnlyIfSuccessful) + "</onlyIfSuccessful>")
	builder.WriteString("<defaultExcludes>" + strconv.FormatBool(cfg.DefaultExcludes) + "</defaultExcludes>")
	builder.WriteString("<caseSensitive>" + strconv.FormatBool(cfg.CaseSensitive) + "</caseSensitive>")
	builder.WriteString("</hudson.tasks.ArtifactArchiver>")
	return builder.String()
}

// findChildSpan 在 XML 片段里定位第一个指定直接子元素，返回 [start,end) 偏移。
func findChildSpan(fragment, name string) (int, int, bool, error) {
	const prefix = "<root>"
	wrapped := prefix + fragment + "</root>"

	decoder := xml.NewDecoder(strings.NewReader(normalizeXMLVersion(wrapped)))
	depth := 0
	start := -1
	for {
		tokenStart := int(decoder.InputOffset())
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, false, invalidXMLError(err.Error())
		}
		switch typed := token.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 && typed.Name.Local == name && start < 0 {
				start = tokenStart - len(prefix)
			}
		case xml.EndElement:
			if depth == 2 && start >= 0 {
				return start, int(decoder.InputOffset()) - len(prefix), true, nil
			}
			depth--
		}
	}
	return 0, 0, false, nil
}
