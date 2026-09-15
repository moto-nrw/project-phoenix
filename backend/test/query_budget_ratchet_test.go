package test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestQueryBudgetRatchet is the source-level half of the query-budget gate
// (#2940); it runs in the no-database CI ratchet step. The database half is
// every test that calls AssertQueryBudget, which runs with the full suite.
//
// Two checks, both with empty allowlists:
//
//   - Register ↔ tests: every entry in queryBudgets is referenced by an
//     AssertQueryBudget call with its scenario name, and every referenced
//     scenario is registered. A stale entry or a typo fails here instead of
//     at runtime.
//   - One counter: no _test.go file defines its own bun.QueryHook
//     (`BeforeQuery` method). Count with testpkg.CaptureQueries /
//     testpkg.NewQueryCounter so the enable/disable and bucket semantics stay
//     in one place.
func TestQueryBudgetRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	refs, hooks, err := scanQueryBudgetSources(backendRoot)
	if err != nil {
		t.Fatalf("query budget scan failed: %v", err)
	}

	var violations []string
	for _, scenario := range QueryBudgetScenarios() {
		if len(refs[scenario]) == 0 {
			violations = append(violations, fmt.Sprintf(
				"[register] %q is registered in test/query_budgets.go but no test calls AssertQueryBudget with it.\n  Remove the entry or add the test.", scenario))
		}
	}
	for scenario, files := range refs {
		if _, ok := queryBudgets[scenario]; !ok {
			violations = append(violations, fmt.Sprintf(
				"[register] %q is used in %s but not registered.\n  Add it to queryBudgets in test/query_budgets.go with the measured count.",
				scenario, strings.Join(files, ", ")))
		}
	}
	for _, file := range hooks {
		violations = append(violations, fmt.Sprintf(
			"[one-counter] %s defines its own bun.QueryHook (BeforeQuery method).\n  Use testpkg.CaptureQueries(t, db) or db.WithQueryHook(testpkg.NewQueryCounter()) instead.", file))
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Query-budget ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

var (
	assertQueryBudgetPattern = regexp.MustCompile(`AssertQueryBudget\(\s*\w+\s*,\s*"([^"]+)"`)
	queryHookMethodPattern   = regexp.MustCompile(`^func \([^)]*\) BeforeQuery\(`)
)

// scanQueryBudgetSources walks every _test.go file under backendRoot and
// returns scenario → referencing files, plus the files defining a query hook.
// test/query_counter.go is the sanctioned hook and is not a test file, so the
// walk never sees it.
func scanQueryBudgetSources(backendRoot string) (map[string][]string, []string, error) {
	refs := make(map[string][]string)
	var hooks []string
	err := filepath.WalkDir(backendRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != backendRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(backendRoot, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		for _, m := range assertQueryBudgetPattern.FindAllStringSubmatch(string(content), -1) {
			refs[m[1]] = append(refs[m[1]], rel)
		}
		for _, line := range strings.Split(string(content), "\n") {
			if queryHookMethodPattern.MatchString(line) {
				hooks = append(hooks, rel)
				break
			}
		}
		return nil
	})
	return refs, hooks, err
}
