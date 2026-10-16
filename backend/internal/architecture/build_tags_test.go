package architecture

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildContextIncludesTaggedHookDependencies(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	module := "example.test/tagged-hooks"
	writeFile(t, filepath.Join(project, "go.mod"), "module "+module+"\n\ngo 1.25\n")
	writeFile(t, filepath.Join(project, "source.go"), "package source\n")
	writeFile(t, filepath.Join(project, "source_test_helpers.go"), "//go:build testhooks\n\npackage source\nimport _ \"net/http\"\n")
	for _, tagged := range []bool{false, true} {
		build := Build{GOOS: "linux", GOARCH: "amd64"}
		if tagged {
			build.Tags = []string{"testhooks"}
		}
		output, err := runGoList(project, build)
		if err != nil {
			t.Fatal(err)
		}
		graph, err := decodeGraph(output, module, project)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, edge := range graph.Edges {
			if edge.Source == module && edge.Target == "net/http" && edge.Scope == ScopeProduction {
				found = true
			}
		}
		if found != tagged {
			t.Fatalf("hook dependency visible=%v with testhooks=%v", found, tagged)
		}
	}
}

func TestAnalysisBuildTagsCannotHideProductionSource(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "go.mod"), "module example.test/hidden-source\n\ngo 1.25\n")
	writeFile(t, filepath.Join(project, "source.go"), "package source\n")
	writeFile(t, filepath.Join(project, "hidden.go"), "//go:build !testhooks\n\npackage source\n")
	_, err := runGoList(project, Build{GOOS: "linux", GOARCH: "amd64", Tags: []string{"testhooks"}})
	if err == nil || !strings.Contains(err.Error(), "analysis build tags exclude production source") {
		t.Fatalf("analysis tag hid production source: %v", err)
	}
}

func TestGodaQueryCarriesAnalysisBuildTags(t *testing.T) {
	t.Parallel()
	policy, graph, _ := loadProjectionInputs(t)
	policy.Build.Tags = []string{"testhooks"}
	query, err := GodaQuery(policy, graph, "module:alpha")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, "testhooks=1(") {
		t.Fatalf("goda query lost the analysis build tag: %s", query)
	}
}
