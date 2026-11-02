package jenkins

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const freestyleWithMailer = `<?xml version='1.1' encoding='UTF-8'?>
<project>
  <description>demo</description>
  <disabled>false</disabled>
  <publishers>
    <hudson.tasks.Mailer>
      <recipients>ci@example.com</recipients>
    </hudson.tasks.Mailer>
  </publishers>
</project>`

const freestyleWithArchiver = `<?xml version='1.1' encoding='UTF-8'?>
<project>
  <description>demo</description>
  <publishers>
    <hudson.tasks.ArtifactArchiver>
      <artifacts>dist/**/*.jar</artifacts>
      <excludes>**/*.tmp</excludes>
      <fingerprint>true</fingerprint>
      <allowEmptyArchive>false</allowEmptyArchive>
      <onlyIfSuccessful>true</onlyIfSuccessful>
      <defaultExcludes>false</defaultExcludes>
      <caseSensitive>true</caseSensitive>
    </hudson.tasks.ArtifactArchiver>
    <hudson.tasks.Mailer>
      <recipients>ci@example.com</recipients>
    </hudson.tasks.Mailer>
  </publishers>
</project>`

func parseJobFields(t *testing.T, xmlText string) *JobFields {
	t.Helper()
	fields, err := ParseJobFields(xmlText, "demo")
	if err != nil {
		t.Fatalf("ParseJobFields() error = %v", err)
	}
	return fields
}

func TestParseArtifactArchiveReadsAllFields(t *testing.T) {
	fields := parseJobFields(t, freestyleWithArchiver)
	if fields.Archive == nil {
		t.Fatal("archive = nil, want parsed archiver")
	}
	archive := fields.Archive
	if !archive.Enabled {
		t.Error("enabled = false, want true")
	}
	if archive.Files != "dist/**/*.jar" || archive.Excludes != "**/*.tmp" {
		t.Errorf("files/excludes = %q/%q", archive.Files, archive.Excludes)
	}
	if !archive.Fingerprint || !archive.OnlyIfSuccessful || !archive.CaseSensitive {
		t.Errorf("booleans = %+v, want fingerprint/onlyIfSuccessful/caseSensitive true", archive)
	}
	if archive.DefaultExcludes {
		t.Error("defaultExcludes = true, want false")
	}
}

func TestParseArtifactArchiveMissing(t *testing.T) {
	fields := parseJobFields(t, freestyleWithMailer)
	if fields.Archive != nil {
		t.Errorf("archive = %+v, want nil when no ArtifactArchiver", fields.Archive)
	}
}

// Jenkins 不写 defaultExcludes / caseSensitive 时按 Jenkins 默认值（true / false）补齐。
func TestParseArtifactArchiveDefaults(t *testing.T) {
	fields := parseJobFields(t, `<?xml version='1.1' encoding='UTF-8'?>
<project>
  <publishers>
    <hudson.tasks.ArtifactArchiver>
      <artifacts>out/*.zip</artifacts>
    </hudson.tasks.ArtifactArchiver>
  </publishers>
</project>`)
	if fields.Archive == nil {
		t.Fatal("archive = nil, want parsed archiver")
	}
	if !fields.Archive.DefaultExcludes {
		t.Error("defaultExcludes = false, want Jenkins default true")
	}
	if fields.Archive.CaseSensitive {
		t.Error("caseSensitive = true, want default false")
	}
	if fields.Archive.Fingerprint || fields.Archive.OnlyIfSuccessful || fields.Archive.AllowEmptyArchive {
		t.Errorf("archive = %+v, want boolean defaults false", fields.Archive)
	}
}

func TestParseArtifactArchiveSkipsPipeline(t *testing.T) {
	fields := parseJobFields(t, `<?xml version='1.1' encoding='UTF-8'?>
<flow-definition>
  <publishers>
    <hudson.tasks.ArtifactArchiver>
      <artifacts>dist/**</artifacts>
    </hudson.tasks.ArtifactArchiver>
  </publishers>
</flow-definition>`)
	if fields.Archive != nil {
		t.Errorf("archive = %+v, want nil for Pipeline job", fields.Archive)
	}
}

func TestPatchArtifactArchiveIntoEmptyPublishers(t *testing.T) {
	patched, err := PatchArtifactArchive(
		`<project><publishers/></project>`,
		ArtifactArchive{Enabled: true, Files: "dist/**/*.jar", DefaultExcludes: true},
	)
	if err != nil {
		t.Fatalf("PatchArtifactArchive() error = %v", err)
	}
	if !strings.Contains(patched, "<hudson.tasks.ArtifactArchiver>") {
		t.Errorf("patched = %s, want archiver", patched)
	}
	if strings.Contains(patched, "<publishers/>") {
		t.Errorf("patched = %s, want expanded publishers", patched)
	}

	var decoded struct {
		Archivers []struct {
			Artifacts string `xml:"artifacts"`
		} `xml:"publishers>hudson.tasks.ArtifactArchiver"`
	}
	if err := xml.Unmarshal([]byte(normalizeXMLVersion(patched)), &decoded); err != nil {
		t.Fatalf("patched XML is invalid: %v", err)
	}
	if len(decoded.Archivers) != 1 || decoded.Archivers[0].Artifacts != "dist/**/*.jar" {
		t.Errorf("decoded archivers = %+v", decoded.Archivers)
	}
}

func TestPatchArtifactArchiveKeepsOtherPublishers(t *testing.T) {
	patched, err := PatchArtifactArchive(
		freestyleWithMailer,
		ArtifactArchive{Enabled: true, Files: "target/*.jar"},
	)
	if err != nil {
		t.Fatalf("PatchArtifactArchive() error = %v", err)
	}
	if !strings.Contains(patched, "hudson.tasks.Mailer") || !strings.Contains(patched, "ci@example.com") {
		t.Errorf("patched dropped existing publisher: %s", patched)
	}
	if strings.Count(patched, "<hudson.tasks.ArtifactArchiver>") != 1 {
		t.Errorf("patched = %s, want exactly one archiver", patched)
	}
}

func TestPatchArtifactArchiveReplacesExisting(t *testing.T) {
	patched, err := PatchArtifactArchive(
		freestyleWithArchiver,
		ArtifactArchive{Enabled: true, Files: "target/*.war", Fingerprint: true},
	)
	if err != nil {
		t.Fatalf("PatchArtifactArchive() error = %v", err)
	}
	if strings.Contains(patched, "dist/**/*.jar") {
		t.Errorf("patched kept stale artifacts: %s", patched)
	}
	if strings.Count(patched, "<hudson.tasks.ArtifactArchiver>") != 1 {
		t.Errorf("patched = %s, want exactly one archiver", patched)
	}
	fields := parseJobFields(t, patched)
	if fields.Archive == nil || fields.Archive.Files != "target/*.war" {
		t.Errorf("reparsed archive = %+v, want target/*.war", fields.Archive)
	}
}

func TestPatchArtifactArchiveDisables(t *testing.T) {
	patched, err := PatchArtifactArchive(freestyleWithArchiver, ArtifactArchive{Enabled: false})
	if err != nil {
		t.Fatalf("PatchArtifactArchive() error = %v", err)
	}
	if strings.Contains(patched, "ArtifactArchiver") {
		t.Errorf("patched = %s, want archiver removed", patched)
	}
	if !strings.Contains(patched, "hudson.tasks.Mailer") {
		t.Errorf("patched dropped other publisher: %s", patched)
	}
}

func TestPatchArtifactArchiveRejectsPipeline(t *testing.T) {
	_, err := PatchArtifactArchive(
		`<flow-definition><publishers/></flow-definition>`,
		ArtifactArchive{Enabled: true, Files: "dist/**"},
	)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Kind != ErrInvalidConfig {
		t.Fatalf("error = %v, want invalid_config", err)
	}
	if !strings.Contains(apiErr.Message, "Pipeline") {
		t.Errorf("message = %q, want Pipeline hint", apiErr.Message)
	}
}

func TestPatchArtifactArchiveRequiresFiles(t *testing.T) {
	_, err := PatchArtifactArchive(`<project><publishers/></project>`, ArtifactArchive{Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "pattern") {
		t.Fatalf("error = %v, want missing pattern error", err)
	}
}

func TestUpdateJobAppliesArchive(t *testing.T) {
	var posted string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/job/demo/config.xml":
			if r.Method == http.MethodGet {
				fmt.Fprint(w, freestyleWithMailer)
				return
			}
			body, _ := io.ReadAll(r.Body)
			posted = string(body)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, false)
	_, err := client.UpdateJob(context.Background(), "demo", JobUpdate{
		SetArchive: true,
		Archive: ArtifactArchive{
			Enabled:         true,
			Files:           "dist/**/*.jar",
			Excludes:        "**/*.tmp",
			OnlyIfSuccessful: true,
			DefaultExcludes: true,
		},
	})
	if err != nil {
		t.Fatalf("UpdateJob() error = %v", err)
	}
	if !strings.Contains(posted, "<hudson.tasks.ArtifactArchiver>") {
		t.Errorf("posted config has no archiver: %s", posted)
	}
	if !strings.Contains(posted, "dist/**/*.jar") {
		t.Errorf("posted config has no artifacts pattern: %s", posted)
	}
	if !strings.Contains(posted, "hudson.tasks.Mailer") {
		t.Errorf("posted config dropped other publisher: %s", posted)
	}
}
