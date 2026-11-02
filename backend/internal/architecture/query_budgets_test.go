package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const queryBudgetFragment = `{"scenario":"fixture.list","max":1,"exact":true,"description":"Fixture bulk read."}`

func TestCompareQueryBudgetsImmutableRepresentations(t *testing.T) {
	t.Parallel()
	for _, representation := range []string{"historical", "fragments"} {
		for _, test := range []struct{ name, fragment, want string }{
			{"unchanged", queryBudgetFragment, ""},
			{"tightening", strings.Replace(queryBudgetFragment, `"max":1`, `"max":0`, 1), ""},
			{"raise", strings.Replace(queryBudgetFragment, `"max":1`, `"max":2`, 1), "max: 1 -> 2"},
			{"weaken exact", strings.Replace(queryBudgetFragment, `"exact":true`, `"exact":false`, 1), "exact: true -> false"},
			{"rename evasion", strings.Replace(queryBudgetFragment, `fixture.list`, `fixture.other`, 1), "fixture.list disappeared"},
			{"malformed candidate", `{`, "read candidate query budgets"},
		} {
			t.Run(representation+"/"+test.name, func(t *testing.T) {
				t.Parallel()
				repo, _ := ratchetRepository(t, legacyRecord(2583))
				fragment := filepath.Join(repo, "test", "query-budgets", "owner", "list.json")
				if representation == "historical" {
					historicalQueryFixture(t, repo)
				}
				base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
				writeFile(t, fragment, test.fragment)
				err := CompareQueryBudgets(repo, base)
				if test.want == "" && err != nil || test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
					t.Fatalf("want %q, got %v", test.want, err)
				}
			})
		}
	}
}

func TestCompareQueryBudgetsMissingOrMalformedBaseFailsClosed(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, source string
		remove       bool
	}{
		{name: "missing", remove: true}, {name: "malformed Go", source: `package test`},
		{name: "malformed fragments", source: `{`}, {name: "duplicate fragments", source: queryBudgetFragment},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo, _ := ratchetRepository(t, legacyRecord(2583))
			historicalQueryFixture(t, repo)
			historical := filepath.Join(repo, "test", "query_budgets.go")
			switch test.name {
			case "missing":
				if err := os.Remove(historical); err != nil {
					t.Fatal(err)
				}
			case "malformed Go":
				writeFile(t, historical, test.source)
			default:
				writeFile(t, filepath.Join(repo, "test", "query-budgets", "a.json"), test.source)
				if test.name == "duplicate fragments" {
					writeFile(t, filepath.Join(repo, "test", "query-budgets", "b.json"), test.source)
				}
			}
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-qm", "broken base")
			base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
			// Repair the candidate: base discovery must still read its pinned tree.
			if err := os.RemoveAll(filepath.Join(repo, "test", "query-budgets")); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(repo, "test", "query-budgets", "fixed.json"), queryBudgetFragment)
			if err := CompareQueryBudgets(repo, base); err == nil || !strings.Contains(err.Error(), "read query budgets at base") {
				t.Fatalf("accepted broken base: %v", err)
			}
			if err := CompareQueryBudgets(repo, "missing-base"); err == nil {
				t.Fatal("accepted unresolved base")
			}
		})
	}
}

func TestCompareRatchetBudgetsIncludesQueryContracts(t *testing.T) {
	t.Parallel()
	repo, base := ratchetRepository(t, legacyRecord(2583))
	writeFile(t, filepath.Join(repo, "test", "query-budgets", "owner", "list.json"), strings.Replace(queryBudgetFragment, `"max":1`, `"max":2`, 1))
	if err := CompareRatchetBudgets(repo, base); err == nil || !strings.Contains(err.Error(), "max: 1 -> 2") {
		t.Fatalf("integration accepted raise: %v", err)
	}
}

func historicalQueryFixture(t *testing.T, repo string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(repo, "test", "query-budgets")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "test", "query_budgets.go"), `package test
 type queryBudget struct { max int; exact bool }
 var queryBudgets = map[string]queryBudget{"fixture.list": {max: 1, exact: true}}
 `)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "historical query base")
}
