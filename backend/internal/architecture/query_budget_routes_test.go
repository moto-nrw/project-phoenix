package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQueryBudgetExplicitRoutesAndNonHTTPExceptions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, route, baseExceptions, exceptions, golden, want string
		missing                                               bool
	}{
		{name: "table-driven description without route", route: "GET /api/students/"},
		{name: "renamed students route", route: "GET /api/students/", golden: "GET /api/pupils/\n", want: "absent from router golden"},
		{name: "missing route", want: "missing route"},
		{name: "non-HTTP use case", exceptions: `{"api.students.list":"Calls the directory use case directly; no HTTP request."}`},
		{name: "exception may shrink", route: "GET /api/students/", baseExceptions: `{"api.students.list":"Use case only."}`},
		{name: "register cannot disappear", baseExceptions: `{}`, missing: true, want: "must remain present"},
		{name: "new exception", baseExceptions: `{}`, exceptions: `{"api.students.list":"Use case only."}`, want: "exception added or changed"},
		{name: "same-size exchange", baseExceptions: `{"api.other":"Use case only."}`, exceptions: `{"api.students.list":"Use case only."}`, want: "exception added or changed"},
		{name: "changed reason", baseExceptions: `{"api.students.list":"Use case only."}`, exceptions: `{"api.students.list":"Different reason."}`, want: "exception added or changed"},
		{name: "HTTP cannot claim exception", route: "GET /api/students/", exceptions: `{"api.students.list":"Use case only."}`, want: "invalid non-HTTP exception"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, _ := ratchetRepository(t, legacyRecord(2583))
			writeFile(t, filepath.Join(repo, "test/query-budgets/owner/list.json"), strings.Replace(strings.Replace(queryBudgetFragment, "fixture.list", "api.students.list", 1), `"route":"GET /fixture/list",`, "", 1))
			if tc.baseExceptions != "" {
				writeFile(t, filepath.Join(repo, queryBudgetNonHTTPPath), tc.baseExceptions)
			}
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "--allow-empty", "-qm", "exception base")
			base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
			exceptions := tc.exceptions
			if exceptions == "" {
				exceptions = `{}`
			}
			if !tc.missing {
				writeFile(t, filepath.Join(repo, queryBudgetNonHTTPPath), exceptions)
			} else {
				if err := os.Remove(filepath.Join(repo, queryBudgetNonHTTPPath)); err != nil {
					t.Fatal(err)
				}
			}
			fragment := `{"scenario":"api.students.list","max":1,"exact":true,"description":"three children"`
			if tc.route != "" {
				fragment += `,"route":"` + tc.route + `"`
			}
			fragment += `}`
			writeFile(t, filepath.Join(repo, "test/query-budgets/owner/list.json"), fragment)
			golden := tc.golden
			if golden == "" {
				golden = "GET /api/students/\n"
			}
			writeFile(t, filepath.Join(repo, routeGoldenDirectory, "fixture.golden"), golden)
			err := CompareQueryBudgets(repo, base)
			if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

func TestQueryBudgetRouteSetRejectsOneOfThreeRenamedRoutes(t *testing.T) {
	t.Parallel()
	repo, _ := ratchetRepository(t, legacyRecord(2583))
	fragment := `{"scenario":"api.students.list","max":0,"exact":true,"description":"three requests","route":["GET /api/students/","GET /api/students/{id}","PUT /api/students/{id}"]}`
	writeFile(t, filepath.Join(repo, "test/query-budgets/owner/list.json"), fragment)
	golden := filepath.Join(repo, routeGoldenDirectory, "fixture.golden")
	writeFile(t, golden, "GET /api/students/\nGET /api/students/{id}\nPUT /api/students/{id}\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "three-route base")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	writeFile(t, golden, "GET /api/students/\nGET /api/pupils/{id}\nPUT /api/students/{id}\n")
	err := CompareQueryBudgets(repo, base)
	if err == nil || !strings.Contains(err.Error(), `GET /api/students/{id}`) {
		t.Fatalf("one missing route must fail: %v", err)
	}
}
