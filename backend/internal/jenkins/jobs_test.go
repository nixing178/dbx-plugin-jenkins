package jenkins

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetJobsRoot(t *testing.T) {
	var tree string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/json" {
			http.NotFound(w, r)
			return
		}
		tree = r.URL.Query().Get("tree")
		fmt.Fprint(w, `{"jobs":[
			{"_class":"hudson.model.FreeStyleProject","name":"build","fullName":"build","color":"red_anime","url":"http://ci/job/build/","lastBuild":{"number":7,"result":"FAILURE","timestamp":1700000000000,"duration":1234,"building":true}},
			{"_class":"com.cloudbees.hudson.plugins.folder.Folder","name":"team","fullName":"team","color":"blue","url":"http://ci/job/team/","lastBuild":null}
		]}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	jobs, err := client.GetJobs(context.Background(), "")
	if err != nil {
		t.Fatalf("GetJobs() error = %v", err)
	}
	if !strings.Contains(tree, "lastBuild[number,result") {
		t.Errorf("tree = %q, want lastBuild fields", tree)
	}
	if len(jobs) != 2 {
		t.Fatalf("jobs = %d, want 2", len(jobs))
	}
	if jobs[0].Name != "team" || !jobs[0].IsFolder {
		t.Errorf("first job = %+v, want folder team first", jobs[0])
	}
	if jobs[1].Name != "build" || jobs[1].Status != StatusBuilding || !jobs[1].Building {
		t.Errorf("second job = %+v, want building build", jobs[1])
	}
}

func TestGetJobsFolder(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		fmt.Fprint(w, `{"jobs":[
			{"_class":"org.jenkinsci.plugins.workflow.job.WorkflowJob","name":"demo","color":"blue","url":"http://ci/job/team/job/demo/","lastBuild":{"number":1,"result":"SUCCESS","building":false}}
		]}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	jobs, err := client.GetJobs(context.Background(), "team")
	if err != nil {
		t.Fatalf("GetJobs() error = %v", err)
	}
	if path != "/job/team/api/json" {
		t.Errorf("path = %q, want /job/team/api/json", path)
	}
	if len(jobs) != 1 || jobs[0].FullName != "team/demo" {
		t.Errorf("jobs = %+v, want fullName team/demo", jobs)
	}
	if jobs[0].Status != StatusSuccess {
		t.Errorf("status = %q, want success", jobs[0].Status)
	}
}

func TestGetJobParameters(t *testing.T) {
	var path, tree string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		tree = r.URL.Query().Get("tree")
		fmt.Fprint(w, `{
			"name":"demo","fullName":"team/demo","description":"Build demo","url":"http://ci/job/team/job/demo/",
			"color":"blue","buildable":true,"inQueue":false,
			"lastBuild":{"number":12,"result":"SUCCESS","timestamp":1700000000000,"duration":1000,"building":false},
			"lastSuccessfulBuild":{"number":12},"lastFailedBuild":{"number":9},
			"property":[{"parameterDefinitions":[
				{"name":"BRANCH","type":"StringParameterDefinition","description":"branch","defaultParameterValue":{"value":"main"}},
				{"name":"ENV","type":"ChoiceParameterDefinition","description":"env","choices":["dev","prod"]},
				{"name":"CLEAN","type":"BooleanParameterDefinition","defaultParameterValue":{"value":true}}
			]}]
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	job, err := client.GetJob(context.Background(), "team/demo")
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if path != "/job/team/job/demo/api/json" {
		t.Errorf("path = %q, want /job/team/job/demo/api/json", path)
	}
	if !strings.Contains(tree, "parameterDefinitions") {
		t.Errorf("tree = %q, want parameterDefinitions", tree)
	}
	if job.Status != StatusSuccess || job.LastBuild == nil || job.LastBuild.Number != 12 {
		t.Errorf("job = %+v, want success with build 12", job)
	}
	if len(job.Parameters) != 3 {
		t.Fatalf("parameters = %d, want 3", len(job.Parameters))
	}
	if job.Parameters[0].Name != "BRANCH" || job.Parameters[0].DefaultValue != "main" {
		t.Errorf("BRANCH = %+v, want default main", job.Parameters[0])
	}
	if len(job.Parameters[1].Choices) != 2 || job.Parameters[1].Choices[1] != "prod" {
		t.Errorf("ENV choices = %+v, want dev/prod", job.Parameters[1].Choices)
	}
	if job.Parameters[2].DefaultValue != "true" {
		t.Errorf("CLEAN default = %q, want true", job.Parameters[2].DefaultValue)
	}
}

func TestGetJobNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	_, err := client.GetJob(context.Background(), "missing")
	if err == nil {
		t.Fatal("GetJob() error = nil, want not found")
	}
}

func TestActiveChoiceOptionsStaticLiteral(t *testing.T) {
	configXML := `<project>
  <properties>
    <hudson.model.ParametersDefinitionProperty>
      <parameterDefinitions>
        <hudson.model.ChoiceParameterDefinition>
          <name>env</name>
          <choices class="java.util.Arrays$ArrayList"><a class="string-array"><string>dev</string></a></choices>
        </hudson.model.ChoiceParameterDefinition>
        <org.biouno.unochoice.ChoiceParameter plugin="uno-choice@2.8.9">
          <name>projectName</name>
          <script class="org.biouno.unochoice.model.GroovyScript">
            <secureScript plugin="script-security@1402">
              <script>return [&apos;t6-auth&apos;,&apos;t6-gateway&apos;,&apos;t6-base&apos;]</script>
              <sandbox>true</sandbox>
            </secureScript>
          </script>
          <choiceType>PT_CHECKBOX</choiceType>
        </org.biouno.unochoice.ChoiceParameter>
        <org.biouno.unochoice.ChoiceParameter>
          <name>single</name>
          <script class="org.biouno.unochoice.model.GroovyScript">
            <secureScript><script>return ["only"]</script></secureScript>
          </script>
          <choiceType>PT_SINGLE_SELECT</choiceType>
        </org.biouno.unochoice.ChoiceParameter>
        <org.biouno.unochoice.ChoiceParameter>
          <name>dynamic</name>
          <script class="org.biouno.unochoice.model.GroovyScript">
            <secureScript><script>return jenkins.model.Jenkins.instance.items.collect { it.name }</script></secureScript>
          </script>
        </org.biouno.unochoice.ChoiceParameter>
      </parameterDefinitions>
    </hudson.model.ParametersDefinitionProperty>
  </properties>
</project>`

	options := activeChoiceOptions(configXML, map[string]bool{"projectName": true, "single": true, "dynamic": true})
	got := options["projectName"].Choices
	want := []string{"t6-auth", "t6-gateway", "t6-base"}
	if len(got) != len(want) {
		t.Fatalf("projectName options = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("projectName options = %v, want %v", got, want)
		}
	}
	if !options["projectName"].MultiSelect {
		t.Errorf("PT_CHECKBOX should be multi-select")
	}
	if options["single"].MultiSelect {
		t.Errorf("PT_SINGLE_SELECT must not be multi-select")
	}
	if _, ok := options["dynamic"]; ok {
		t.Errorf("dynamic script should not produce literal options, got %v", options["dynamic"].Choices)
	}
	if _, ok := options["env"]; ok {
		t.Errorf("standard choice parameter must not be handled here")
	}
}

func TestGetJobFillsActiveChoiceOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/job/t7-api/api/json":
			fmt.Fprint(w, `{"_class":"hudson.maven.MavenModuleSet","name":"t7-api","fullName":"t7-api","color":"blue","buildable":true,
				"property":[{"parameterDefinitions":[
					{"_class":"org.biouno.unochoice.ChoiceParameter","name":"projectName","type":"ChoiceParameter","description":"模块","defaultParameterValue":{"value":"t6-auth"}}
				]}]}`)
		case "/job/t7-api/config.xml":
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<maven2-moduleset><properties><hudson.model.ParametersDefinitionProperty><parameterDefinitions>
				<org.biouno.unochoice.ChoiceParameter plugin="uno-choice@2.8.9">
					<name>projectName</name>
					<script class="org.biouno.unochoice.model.GroovyScript"><secureScript>
						<script>return [&apos;t6-auth&apos;,&apos;t6-gateway&apos;]</script>
					</secureScript></script>
					<choiceType>PT_CHECKBOX</choiceType>
				</org.biouno.unochoice.ChoiceParameter>
			</parameterDefinitions></hudson.model.ParametersDefinitionProperty></properties></maven2-moduleset>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, true)
	job, err := client.GetJob(context.Background(), "t7-api")
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if len(job.Parameters) != 1 {
		t.Fatalf("parameters = %d, want 1", len(job.Parameters))
	}
	parameter := job.Parameters[0]
	if len(parameter.Choices) != 2 || parameter.Choices[1] != "t6-gateway" {
		t.Fatalf("choices = %v, want [t6-auth t6-gateway]", parameter.Choices)
	}
	if parameter.DefaultValue != "t6-auth" {
		t.Errorf("default = %q, want t6-auth", parameter.DefaultValue)
	}
	if !parameter.MultiSelect {
		t.Errorf("PT_CHECKBOX should surface multiSelect=true")
	}
}
