package test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// testSkipReasons lists every function that may skip a test, keyed by
// "path:Function" (the enclosing named function). A skipped test proves
// nothing, so a skip is either an environment opt-in that CI runs elsewhere
// ("environment: ...") or a quarantined flake with its tracking issue
// ("quarantine #1234: ..."). Flaky tests are fixed, not skipped: a guard that
// skips on the weekday or wall-clock minute hides its case on some CI runs, and
// a ratchet that skips when it cannot find its input guards nothing.
var testSkipReasons = map[string]string{
	"internal/devtools/seedapi/coverage_ratchet_test.go:TestSeedCoverageRatchet": "environment: needs a seeded stack via SEED_COVERAGE_DSN; the seed-smoke CI job provides it",
	"internal/testdb/testdb_test.go:integrationConfig":                           "environment: -short and missing TEST_DB_DSN skip DB integration tests; CI sets TEST_DB_DSN",
	"internal/testdb/testdb_test.go:TestMigrationsCompleteAgainstRealTemplate":   "environment: -short and missing TEST_DB_DSN skip DB integration tests; CI sets TEST_DB_DSN",
	"test/helpers.go:SetupTestDB":                                                "environment: -short skips DB integration tests; CI runs without -short",
	"test/runtime_checkpoint.go:RequireRuntimeEvidence":                          "environment: runtime evidence measurement runs only on request (docs/runtime-checkpoints/README.md)",
}

var testSkipReasonFormat = regexp.MustCompile(`^(environment: |quarantine #[0-9]+: )\S`)

func TestTestSkipRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Fatalf("Could not find backend root: %v", err)
	}
	sites, err := scanTestSkips(backendRoot)
	if err != nil {
		t.Fatal(err)
	}
	var problems []string
	for key, lines := range sites {
		if _, ok := testSkipReasons[key]; !ok {
			problems = append(problems, "unregistered skip: "+key+" (line "+strings.Join(lines, ", ")+")")
		}
	}
	for key, reason := range testSkipReasons {
		if _, ok := sites[key]; !ok {
			problems = append(problems, "stale entry, no skip left: "+key)
		}
		if !testSkipReasonFormat.MatchString(reason) {
			problems = append(problems, "reason must start with \"environment: \" or \"quarantine #<issue>: \": "+key)
		}
	}
	if len(problems) == 0 {
		return
	}
	sort.Strings(problems)
	t.Errorf("Test skip ratchet failed:\n  %s\n\n"+
		"Fix the test instead of skipping it: fix its clock with synctest or a fixed date, "+
		"and let a broken fixture or a missing input fail. Only an environment opt-in or a "+
		"quarantine with a tracking issue goes into testSkipReasons.",
		strings.Join(problems, "\n  "))
}

func TestTestSkipRatchetFindsSkipsOnTestHandlesOnly(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write := func(path, source string) {
		t.Helper()
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("a/a_test.go", `package a

import "testing"

func TestWeekend(t *testing.T) {
	t.Parallel()
	t.Run("sub", func(t *testing.T) { t.Skip("weekend") })
}

func helper(tb testing.TB) { tb.SkipNow() }
`)
	write("b/b_test.go", `package b

import "testing"

type suite struct{ t *testing.T }

func (s suite) T() *testing.T { return s.t }

func (s suite) TestX() { s.T().Skipf("later") }
`)
	write("c/days.go", `package c

type days struct{}

func (days) Skip(string, bool) int { return 0 }

func Use(d days) int { return d.Skip("2026-10-03", false) }
`)

	sites, err := scanTestSkips(root)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for key := range sites {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	want := []string{"a/a_test.go:TestWeekend", "a/a_test.go:helper", "b/b_test.go:TestX"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("skip sites = %v, want %v", keys, want)
	}
}

// scanTestSkips returns the line numbers of t.Skip, t.Skipf and t.SkipNow
// calls on testing handles (t, tb, b, or a suite's T()) per "path:Function".
func scanTestSkips(root string) (map[string][]string, error) {
	sites := map[string][]string{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" || entry.Name() == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(source), ".Skip") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, source, 0)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || !isTestHandleSkip(call) {
					return true
				}
				key := filepath.ToSlash(relative) + ":" + function.Name.Name
				sites[key] = append(sites[key], strings.TrimPrefix(fset.Position(call.Pos()).String(), path+":"))
				return true
			})
		}
		return nil
	})
	return sites, err
}

func isTestHandleSkip(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (selector.Sel.Name != "Skip" && selector.Sel.Name != "Skipf" && selector.Sel.Name != "SkipNow") {
		return false
	}
	switch receiver := selector.X.(type) {
	case *ast.Ident:
		return receiver.Name == "t" || receiver.Name == "tb" || receiver.Name == "b"
	case *ast.CallExpr:
		inner, ok := receiver.Fun.(*ast.SelectorExpr)
		return ok && inner.Sel.Name == "T" && len(receiver.Args) == 0
	}
	return false
}
