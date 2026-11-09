// Package querybudgets holds the query-budget register (#2940) and its assertion.
//
// It lives next to the JSON fragments it embeds, outside package test, so that
// editing a budget only rebuilds and reselects the packages that assert budgets
// instead of every package importing the shared test fixtures.
package querybudgets

import (
	"embed"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/architecture/querybudget"
)

// queryBudget is one register entry: the number of SQL statements a scenario
// may issue. `max` entries are ceilings (less is fine, more fails); `exact`
// entries pin a dedup contract (one bulk load, one settings snapshot) where a
// count of zero would mean the code path silently stopped running.
type queryBudget struct {
	max   int
	exact bool
	route querybudget.Routes
}

// queryBudgets is the query-budget register (#2940): scenario → statements.
//
// Shrink-only, like every other ratchet in this package:
//
//   - Never raise a number. A scenario that needs more statements is an N+1
//     regression until proven otherwise; the fix is a batch load keyed by an
//     ID set (modules/timetable/compose/exception_conflicts.go, the
//     FindByStudentIDsAndDate calls, is the reference shape).
//   - Lower a number when a fix removes statements, so the win cannot regress.
//   - Every new list endpoint gets an entry plus a test calling
//     AssertQueryBudget with it (`.claude/rules/backend-conventions.md`,
//     Rule 15). TestQueryBudgetRatchet fails on entries no test references
//     and on scenario names no entry defines.
//
// The counts are what the fixture-sized scenario in the referenced test
// issues; small fixtures are enough because N+1 shows up at N=3 already.
//
//go:embed *
var queryBudgetSources embed.FS

var queryBudgets = loadQueryBudgets()

func loadQueryBudgets() map[string]queryBudget {
	entries, err := querybudget.ReadQueryBudgets(queryBudgetSources, ".")
	if err != nil {
		panic(fmt.Sprintf("load query budgets: %v", err))
	}
	budgets := make(map[string]queryBudget, len(entries))
	for scenario, entry := range entries {
		budgets[scenario] = queryBudget{max: entry.Max, exact: entry.Exact, route: entry.Route}
	}
	return budgets
}

// AssertQueryBudget checks the statements a scenario issued against the
// register. Pass counter.Measurement with the matched SQL bucket, including
// for non-HTTP use cases. Bare SQL slices would discard HTTP route provenance.
func AssertQueryBudget(tb testing.TB, scenario string, observed Measurement) {
	tb.Helper()
	budget, ok := queryBudgets[scenario]
	if !ok {
		tb.Fatalf("query budget %q is not registered in backend/test/query-budgets/", scenario)
	}
	assertQueryBudget(tb, scenario, budget, observed, os.Getenv)
}

func assertQueryBudget(tb testing.TB, scenario string, budget queryBudget, observed Measurement, getenv func(string) string) {
	tb.Helper()
	queries, route := observed.Statements, observed.Route
	if observed.HTTPRequest && len(route) == 0 {
		tb.Errorf("query budget %q: measured HTTP request has no resolved route", scenario)
	}
	if err := measuredRouteError(budget.route, route); err != nil && (len(budget.route) != 0 || getenv("PHX_QUERY_BUDGET_CAPTURE_ROUTES") == "") {
		tb.Errorf("query budget %q: %v", scenario, err)
	}

	if path := getenv("PHX_QUERY_BUDGET_CAPTURE_ROUTES"); path != "" {
		if err := appendBudgetReport(path, budgetReport{Scenario: scenario, Route: route, Test: tb.Name(), Actual: len(queries), Max: budget.max, Exact: budget.exact}); err != nil {
			tb.Errorf("capture budget route: %v", err)
		}
	}
	emitQueryFeedback(tb, scenario, queries, budget, getenv)
	got := len(queries)
	switch {
	case got > budget.max:
		tb.Errorf("query budget exceeded: scenario %q issued %d statements, budget %d.\n"+
			"  Likely an N+1: a query inside a loop over rows. Batch-load by ID set instead\n"+
			"  (reference: modules/timetable/compose/exception_conflicts.go, FindByStudentIDsAndDate).\n"+
			"  Never raise the register entry.\n%s",
			scenario, got, budget.max, indentQueries(queries, 200))
	case budget.exact && got < budget.max:
		tb.Errorf("query budget %q pins exactly %d statement(s) but %d ran.\n"+
			"  The dedup contract it guards no longer executes; check the scenario before touching the register.\n%s",
			scenario, budget.max, got, indentQueries(queries, 200))
	case got < budget.max:
		tb.Logf("query budget %q: %d of %d statements used. Lower the register entry to lock the headroom in.",
			scenario, got, budget.max)
	}
}

func indentQueries(queries []string, maxBytes int) string {
	if len(queries) == 0 {
		return ""
	}
	lines := make([]string, 0, len(queries)+1)
	lines = append(lines, "  Statements:")
	for i, q := range queries {
		q = strings.Join(strings.Fields(q), " ")
		if maxBytes > 0 && len(q) > maxBytes {
			q = q[:maxBytes] + "..."
		}
		lines = append(lines, fmt.Sprintf("  %3d. %s", i+1, q))
	}
	return strings.Join(lines, "\n")
}

// QueryBudgetScenarios lists the registered scenario names, sorted.
func QueryBudgetScenarios() []string {
	names := make([]string, 0, len(queryBudgets))
	for name := range queryBudgets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Measurement is the request-aware SQL snapshot returned by QueryCounter.Measurement.
type Measurement = querybudget.Measurement

func measuredRouteError(stored, measured querybudget.Routes) error {
	if !slices.Equal(stored, measured) {
		return fmt.Errorf("route: stored %q, measured %q", stored, measured)
	}
	return nil
}
