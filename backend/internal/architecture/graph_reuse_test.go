package architecture

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCandidateExternalClassificationGraphReuseParity(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	module := "example.test/graph-reuse"
	writeFile(t, filepath.Join(project, "go.mod"), "module "+module+"\n\ngo 1.25\n")
	for path, source := range map[string]string{
		"new/production.go":         "package introduced\nimport (_ \"fmt\"; _ \"io\"; _ \"os\")\n",
		"new/internal_test.go":      "package introduced\nimport (_ \"strings\"; _ \"sort\"; _ \"encoding/json\")\n",
		"new/external_test.go":      "package introduced_test\nimport (_ \"bytes\"; _ \"time\")\n",
		"existing/production.go":    "package existing\nimport (_ \"io\"; _ \"sort\")\n",
		"existing/internal_test.go": "package existing\nimport _ \"encoding/json\"\n",
		"existing/external_test.go": "package existing_test\nimport _ \"time\"\n",
	} {
		writeFile(t, filepath.Join(project, path), source)
	}
	policy := &Policy{ModulePath: module, Build: Build{GOOS: "linux", GOARCH: "amd64"}}
	for _, path := range []string{"fmt", "strings", "bytes", "io", "sort", "encoding/json", "time"} {
		policy.ExternalPackages = append(policy.ExternalPackages, ExternalPackage{Path: path, Class: "stdlib"})
	}
	created := map[string]struct{}{module + "/new": {}}
	output, err := runGoList(project, policy.Build)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := decodeGraph(output, module, project)
	if err != nil {
		t.Fatal(err)
	}
	standalone, err := candidateOnlyExternalPackages(project, policy, created)
	if err != nil {
		t.Fatal(err)
	}
	// Each exclusive dependency comes from a different source scope. Dependencies
	// shared with any existing consumer must stay excluded, even test consumers.
	want := map[string]struct{}{"fmt": {}, "strings": {}, "bytes": {}}
	if !maps.Equal(standalone, want) {
		t.Fatalf("standalone classification: want %v, got %v", want, standalone)
	}
	for _, reverse := range []bool{false, true} {
		reused := *graph
		reused.Edges = slices.Clone(graph.Edges)
		if reverse {
			slices.Reverse(reused.Edges)
		}
		got := candidateOnlyExternalPackagesFromGraph(policy, created, &reused)
		if !maps.Equal(got, standalone) {
			t.Fatalf("reused graph (reverse=%v): want standalone %v, got %v", reverse, standalone, got)
		}
	}
}

func TestCandidateExternalClassificationStandaloneLoadError(t *testing.T) {
	t.Parallel()
	policy := &Policy{ModulePath: "example.test/graph-reuse", Build: Build{GOOS: "linux", GOARCH: "amd64"}}
	got, err := candidateOnlyExternalPackages(filepath.Join(t.TempDir(), "missing"), policy, nil)
	if err == nil || !strings.Contains(err.Error(), "load Go packages:") || got != nil {
		t.Fatalf("standalone load failure lost its diagnostic: result=%v, err=%v", got, err)
	}
}
