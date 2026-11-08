package architecture

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/architecture/querybudget"
)

// The retirement register and the route table golden of
// TestFullProductionRouterGolden, both relative to the backend project.
const (
	queryBudgetRetirementsPath = "architecture/query-budget-retirements.json"
	routeGoldenDirectory       = "api/testdata/routes"
)

var (
	routeGoldenLine = regexp.MustCompile(`^[A-Z]+ /\S*$`)
)

// queryBudgetRetirement lets a base scenario disappear together with its own
// route (ADR 0160). The record names the route; the checker, not the review,
// proves that it is the budget's route and that it left the production router
// golden in the same change.
type queryBudgetRetirement struct {
	Scenario string `json:"scenario"`
	Route    string `json:"route"`
	Issue    int    `json:"issue"`
}

// retiredQueryBudgets returns the base scenarios this change retires and the
// violations of the route binding (ADR 0160). Both router goldens are always
// read and fail closed. Only a record the base does not hold yet retires
// anything: an unchanged record is history, never a standing permission.
func retiredQueryBudgets(root, project, sha string, base, candidate map[string]querybudget.Budget) (map[string]bool, []string, error) {
	baseRoutes, err := baseRouteGolden(root, project, sha)
	if err != nil {
		return nil, nil, fmt.Errorf("read route golden at base %s: %w", sha, err)
	}
	candidateRoutes, err := candidateRouteGolden(project, baseRoutes.files)
	if err != nil {
		return nil, nil, fmt.Errorf("read candidate route golden: %w", err)
	}
	added, err := addedQueryBudgetRetirements(root, project, sha)
	if err != nil {
		return nil, nil, err
	}
	retired := map[string]bool{}
	var violations []string
	for _, record := range added {
		if reason := retirementViolation(record, base, candidate, baseRoutes.routes, candidateRoutes.routes); reason != "" {
			violations = append(violations, fmt.Sprintf("%s retirement (%s) %s", record.Scenario, record.Route, reason))
			continue
		}
		retired[record.Scenario] = true
	}
	routeViolations, err := validateQueryBudgetRoutes(root, project, sha, candidate, candidateRoutes.routes)
	if err != nil {
		return nil, nil, err
	}
	violations = append(violations, routeViolations...)
	violations = append(violations, orphanedQueryBudgets(base, candidate, baseRoutes.routes, candidateRoutes.routes)...)
	return retired, violations, nil
}

// retirementViolation explains why a new record may not retire its scenario,
// or returns "" when the scenario's own route left the router golden.
func retirementViolation(record queryBudgetRetirement, base, candidate map[string]querybudget.Budget, baseRoutes, candidateRoutes map[string]bool) string {
	before, registered := base[record.Scenario]
	_, active := candidate[record.Scenario]
	own := budgetRoutes(before, baseRoutes)
	switch {
	case !registered:
		return "retires no budget of the base register"
	case active:
		return "names a budget that is still registered"
	case !own[record.Route]:
		return fmt.Sprintf("names a route that is not the budget's own; its base route binds %s", describeRoutes(own))
	case anyRoute(own, candidateRoutes):
		return "names a route the router golden still lists"
	}
	return ""
}

// orphanedQueryBudgets retains the base-route retirement guard. Candidate
// bindings are additionally checked member-by-member by queryBudgetRouteViolations.
func orphanedQueryBudgets(base, candidate map[string]querybudget.Budget, baseRoutes, candidateRoutes map[string]bool) []string {
	var violations []string
	for scenario, after := range candidate {
		before, existed := base[scenario]
		own := budgetRoutes(before, baseRoutes)
		if !existed || len(own) == 0 || anyRoute(own, candidateRoutes) || len(budgetRoutes(after, candidateRoutes)) != 0 {
			continue
		}
		violations = append(violations, fmt.Sprintf("%s orphaned: its route %s left the router golden; record the new route or retire it", scenario, describeRoutes(own)))
	}
	return violations
}

// budgetRoutes selects explicit measured bindings present in this golden.
func budgetRoutes(budget querybudget.Budget, routes map[string]bool) map[string]bool {
	own := map[string]bool{}
	for _, route := range budget.Route {
		if routes[route] {
			own[route] = true
		}
	}
	return own
}

func anyRoute(routes, golden map[string]bool) bool {
	for route := range routes {
		if golden[route] {
			return true
		}
	}
	return false
}

func describeRoutes(routes map[string]bool) string {
	if len(routes) == 0 {
		return "no route"
	}
	names := make([]string, 0, len(routes))
	for route := range routes {
		names = append(names, route)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func addedQueryBudgetRetirements(root, project, sha string) ([]queryBudgetRetirement, error) {
	source, err := os.ReadFile(filepath.Join(project, queryBudgetRetirementsPath))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	candidate, err := parseQueryBudgetRetirements(source)
	if err != nil {
		return nil, fmt.Errorf("read candidate query budget retirements: %w", err)
	}
	historical := map[queryBudgetRetirement]bool{}
	baseSource, exists, err := baseProjectFile(root, project, sha, queryBudgetRetirementsPath)
	if err != nil {
		return nil, err
	}
	if exists {
		records, err := parseQueryBudgetRetirements(baseSource)
		if err != nil {
			return nil, fmt.Errorf("read query budget retirements at base %s: %w", sha, err)
		}
		for _, record := range records {
			historical[record] = true
		}
	}
	var added []queryBudgetRetirement
	for _, record := range candidate {
		if !historical[record] {
			added = append(added, record)
		}
	}
	return added, nil
}

func parseQueryBudgetRetirements(source []byte) ([]queryBudgetRetirement, error) {
	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.DisallowUnknownFields()
	var records []queryBudgetRetirement
	if err := decoder.Decode(&records); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("trailing data after retirement register")
	}
	seen := map[string]bool{}
	for _, record := range records {
		if record.Scenario == "" || record.Issue <= 0 || !routeGoldenLine.MatchString(record.Route) {
			return nil, fmt.Errorf("retirement %+v needs a scenario, a positive issue and a route \"METHOD /path\"", record)
		}
		if seen[record.Scenario] {
			return nil, fmt.Errorf("duplicate retirement of %q", record.Scenario)
		}
		seen[record.Scenario] = true
	}
	return records, nil
}

// baseProjectFile reads one project file at the base commit and reports a
// missing file separately from a failed read.
func baseProjectFile(root, project, sha, name string) ([]byte, bool, error) {
	relative, err := repositoryRelativePath(root, project, name)
	if err != nil {
		return nil, false, err
	}
	listing, err := gitOutput(root, "ls-tree", sha, "--", filepath.ToSlash(relative))
	if err != nil || strings.TrimSpace(listing) == "" {
		return nil, false, err
	}
	source, err := readGitBlob(root, project, sha, name)
	return source, err == nil, err
}

// routeGolden is the route table golden of TestFullProductionRouterGolden:
// its routes and the files that list them, relative to routeGoldenDirectory.
type routeGolden struct {
	routes map[string]bool
	files  map[string]bool
}

func newRouteGolden() routeGolden {
	return routeGolden{routes: map[string]bool{}, files: map[string]bool{}}
}

// add reads one golden file. An empty file or a line that is no route fails
// closed: a route the checker cannot read must not count as removed.
func (g routeGolden) add(name, source string) error {
	count := 0
	scanner := bufio.NewScanner(strings.NewReader(source))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !routeGoldenLine.MatchString(line) {
			return fmt.Errorf("%s: unreadable route golden line %q", name, line)
		}
		g.routes[querybudget.CanonicalRoute(line)] = true
		count++
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("%s: empty route golden", name)
	}
	g.files[name] = true
	return nil
}

func (g routeGolden) complete() error {
	if len(g.routes) == 0 {
		return fmt.Errorf("no routes read from %s", routeGoldenDirectory)
	}
	return nil
}

func baseRouteGolden(root, project, sha string) (routeGolden, error) {
	golden := newRouteGolden()
	prefix, err := repositoryRelativePath(root, project, routeGoldenDirectory)
	if err != nil {
		return golden, err
	}
	prefix = filepath.ToSlash(prefix)
	listing, err := gitOutput(root, "ls-tree", "-r", "-z", "--name-only", sha, "--", prefix)
	if err != nil {
		return golden, err
	}
	for _, name := range strings.Split(listing, "\x00") {
		if !strings.HasSuffix(name, ".golden") {
			continue
		}
		source, err := gitOutput(root, "cat-file", "blob", sha+":"+name)
		if err != nil {
			return golden, err
		}
		if err := golden.add(strings.TrimPrefix(name, prefix+"/"), source); err != nil {
			return golden, err
		}
	}
	return golden, golden.complete()
}

// candidateRouteGolden reads the candidate golden. Every file the base lists
// must still exist: a deleted golden file is no proof that its routes left.
func candidateRouteGolden(project string, baseFiles map[string]bool) (routeGolden, error) {
	golden := newRouteGolden()
	directory := filepath.Join(project, routeGoldenDirectory)
	err := filepath.WalkDir(directory, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(name, ".golden") {
			return err
		}
		source, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(directory, name)
		if err != nil {
			return err
		}
		return golden.add(filepath.ToSlash(relative), string(source))
	})
	if err != nil {
		return golden, err
	}
	for name := range baseFiles {
		if !golden.files[name] {
			return golden, fmt.Errorf("route golden %s is missing", name)
		}
	}
	return golden, golden.complete()
}
