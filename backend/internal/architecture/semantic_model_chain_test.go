package architecture

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestModelTableOverridesFollowReceiverChainsWithoutHidingTableViolations(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	module := "example.test/model-chain"
	writeFile(t, filepath.Join(project, "go.mod"), "module "+module+"\n\ngo 1.25\nrequire github.com/uptrace/bun v0.0.0\nreplace github.com/uptrace/bun => "+strconv.Quote(fixturePath(t, "semantic", "bun"))+"\n")
	writeFile(t, filepath.Join(project, "value.go"), readFile(t, fixturePath(t, "semantic", "model-chain", "value.go")))
	policy := &Policy{
		ModulePath: module,
		Build:      Build{GOOS: "linux", GOARCH: "amd64"},
		Packages:   []Package{{Path: ".", Owner: "alpha", Role: "postgres"}},
		DataObjects: []DataObject{
			{Name: "alpha.records", WriteOwner: "alpha"},
			{Name: "beta.records", WriteOwner: "beta"},
		},
	}
	violations, err := analyzeSemantics(project, policy)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, violation := range violations {
		for _, location := range violation.Locations {
			got = append(got, violation.Rule+"|"+location.Declaration)
		}
	}
	slices.Sort(got)
	want := []string{
		"tables.foreign-write|ForeignAfter",
		"tables.foreign-write|ForeignModel",
		"tables.unresolved|NoTable",
		"tables.unresolved|SiblingArgument",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("model/table chain findings = %v, want %v", got, want)
	}
}

func TestSemanticCallLocationFailureUnwindsNestedAncestors(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	module := "example.test/model-unwind"
	writeFile(t, filepath.Join(project, "go.mod"), "module "+module+"\n\ngo 1.25\nrequire github.com/uptrace/bun v0.0.0\nreplace github.com/uptrace/bun => "+strconv.Quote(fixturePath(t, "semantic", "bun"))+"\n")
	writeFile(t, filepath.Join(project, "value.go"), readFile(t, fixturePath(t, "semantic", "model-unwind", "value.go")))
	classification := Package{Path: ".", Owner: "alpha", Role: "postgres"}
	policy := &Policy{
		ModulePath: module,
		Build:      Build{GOOS: "linux", GOARCH: "amd64"},
		Packages:   []Package{classification},
	}
	loaded, err := loadTypedPackages(project, policy.Build)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || len(loaded[0].Syntax) != 1 {
		t.Fatalf("unexpected own-package syntax: %v", loaded)
	}
	// Keep the typed file outside the analyzer root to fail nodeLocation inside
	// a nested call. Its sibling calls exercise Inspect's pruned-node unwind.
	analyzer := newSemanticAnalyzer(policy, t.TempDir())
	violations, err := analyzer.analyzeFile(loaded[0], classification, loaded[0].Syntax[0])
	if err == nil || !strings.Contains(err.Error(), "source path is outside project") {
		t.Fatalf("nested location failure = %v, want an outside-project error", err)
	}
	if violations != nil {
		t.Fatalf("location failure returned partial violations: %v", violations)
	}
}
