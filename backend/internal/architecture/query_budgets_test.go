package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const queryBudgetFragment = `{"scenario":"fixture.list","max":1,"exact":true,"route":"GET /fixture/list","description":"GET /fixture/list bulk read."}`

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

// writeQueryBudgetFixture writes the fixture budget and the route golden that
// binds it: the checker reads both and fails closed without routes (ADR 0160).
func writeQueryBudgetFixture(t *testing.T, project string) {
	t.Helper()
	writeFile(t, filepath.Join(project, "test", "query-budgets", "owner", "list.json"), queryBudgetFragment)
	writeFile(t, filepath.Join(project, "api", "testdata", "routes", "fixture.golden"), "GET /fixture/\nGET /fixture/list\n")
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

const fixtureRetirement = `[{"scenario":"fixture.list","route":"GET /fixture/list","issue":5972}]`

// queryBudgetRouteCase is one candidate change against a base with the
// fixture budget, bound by its description to GET /fixture/list, and a route
// golden that lists GET /fixture/ and GET /fixture/list.
type queryBudgetRouteCase struct {
	name       string
	baseRecord bool
	// baseGolden replaces the base golden file; "-" removes the directory.
	baseGolden string
	// golden replaces the candidate golden file; "-" removes the directory.
	golden       string
	extraGolden  bool
	removeBudget bool
	description  string
	record, want string
}

// TestCompareQueryBudgetsRetiresABudgetOnlyWithItsRoute pins ADR 0160: a base
// scenario may disappear only under a retirement record this change adds, and
// only when its own route leaves the router golden in the same change.
func TestCompareQueryBudgetsRetiresABudgetOnlyWithItsRoute(t *testing.T) {
	t.Parallel()
	for _, test := range []queryBudgetRouteCase{
		{name: "route and budget leave", removeBudget: true, golden: "GET /fixture/\n", record: fixtureRetirement},
		{name: "route stays", removeBudget: true, record: fixtureRetirement, want: "names a route the router golden still lists"},
		{name: "no record", removeBudget: true, golden: "GET /fixture/\n", want: "fixture.list disappeared"},
		{name: "idle record for an active budget", record: fixtureRetirement, want: "names a budget that is still registered"},
		{name: "idle record for an unknown budget", record: strings.Replace(fixtureRetirement, "fixture.list", "fixture.other", 1), want: "retires no budget of the base register"},
		{name: "idle record for an unknown route", removeBudget: true, golden: "GET /fixture/\n", record: strings.Replace(fixtureRetirement, "/fixture/list", "/fixture/other", 1), want: "is not the budget's own"},
		{name: "foreign route leaves", removeBudget: true, golden: "GET /fixture/list\n", record: strings.Replace(fixtureRetirement, "/fixture/list", "/fixture/", 1), want: "is not the budget's own; its base route binds GET /fixture/list"},
		{name: "historical record grants nothing", baseRecord: true, removeBudget: true, golden: "GET /fixture/\n", record: fixtureRetirement, want: "fixture.list disappeared"},
		{name: "malformed record", removeBudget: true, golden: "GET /fixture/\n", record: `[{"scenario":"fixture.list","route":"fixture/list","issue":5972}]`, want: "read candidate query budget retirements"},
		{name: "unknown field", removeBudget: true, golden: "GET /fixture/\n", record: `[{"scenario":"fixture.list","route":"GET /fixture/list","issue":5972,"force":true}]`, want: "unknown field"},
		{name: "empty candidate golden", removeBudget: true, golden: "\n", record: fixtureRetirement, want: "fixture.golden: empty route golden"},
		{name: "missing candidate golden file", removeBudget: true, extraGolden: true, golden: "-", record: fixtureRetirement, want: "route golden fixture.golden is missing"},
		{name: "missing candidate golden directory", removeBudget: true, golden: "-", record: fixtureRetirement, want: "read candidate route golden"},
		{name: "empty base golden", baseGolden: "\n", want: "read route golden at base"},
		{name: "no routes read at base", baseGolden: "-", want: "no routes read from api/testdata/routes"},
		{name: "renamed route orphans the budget", golden: "GET /fixture/\nGET /fixture/items\n", want: "fixture.list orphaned: its route GET /fixture/list left the router golden"},
		{name: "renamed route with renamed budget route", golden: "GET /fixture/\nGET /fixture/items\n", description: "GET /fixture/items bulk read."},
		{name: "renamed route with retired budget", removeBudget: true, golden: "GET /fixture/\nGET /fixture/items\n", record: fixtureRetirement},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := compareQueryBudgetRouteCase(t, test)
			if test.want == "" && err != nil || test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("want %q, got %v", test.want, err)
			}
		})
	}
}

func compareQueryBudgetRouteCase(t *testing.T, test queryBudgetRouteCase) error {
	t.Helper()
	repo, _ := ratchetRepository(t, legacyRecord(2583))
	routes := filepath.Join(repo, "api", "testdata", "routes")
	golden := filepath.Join(routes, "fixture.golden")
	record := filepath.Join(repo, "architecture", "query-budget-retirements.json")
	if test.baseRecord {
		writeFile(t, record, fixtureRetirement)
	}
	if test.extraGolden {
		writeFile(t, filepath.Join(routes, "other.golden"), "GET /other\n")
	}
	writeGoldenCase(t, routes, golden, test.baseGolden)
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "--allow-empty", "-qm", "route golden base")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	fragment := filepath.Join(repo, "test", "query-budgets", "owner", "list.json")
	if test.removeBudget {
		writeFile(t, filepath.Join(repo, "test", "query-budgets", "owner", "other.json"), strings.Replace(strings.Replace(queryBudgetFragment, "fixture.list", "fixture.kept", 1), `"route":"GET /fixture/list"`, `"route":"GET /fixture/"`, 1))
		if err := os.Remove(fragment); err != nil {
			t.Fatal(err)
		}
	}
	if test.description != "" {
		writeFile(t, fragment, strings.Replace(strings.Replace(queryBudgetFragment, "GET /fixture/list bulk read.", test.description, 1), `"route":"GET /fixture/list"`, `"route":"GET /fixture/items"`, 1))
	}
	if test.golden == "-" && test.extraGolden {
		if err := os.Remove(golden); err != nil {
			t.Fatal(err)
		}
	} else {
		writeGoldenCase(t, routes, golden, test.golden)
	}
	if test.record != "" {
		writeFile(t, record, test.record)
	}
	return CompareQueryBudgets(repo, base)
}

// writeGoldenCase replaces the fixture golden: "" keeps it, "-" removes the
// whole golden directory.
func writeGoldenCase(t *testing.T, routes, golden, content string) {
	t.Helper()
	switch content {
	case "":
	case "-":
		if err := os.RemoveAll(routes); err != nil {
			t.Fatal(err)
		}
	default:
		writeFile(t, golden, content)
	}
}
