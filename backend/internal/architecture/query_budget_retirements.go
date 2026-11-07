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
	"strings"

	"github.com/moto-nrw/project-phoenix/architecture/querybudget"
)

// The retirement register and the route table golden of
// TestFullProductionRouterGolden, both relative to the backend project.
const (
	queryBudgetRetirementsPath = "architecture/query-budget-retirements.json"
	routeGoldenDirectory       = "api/testdata/routes"
)

var routeGoldenLine = regexp.MustCompile(`^[A-Z]+ /\S*$`)

// queryBudgetRetirement lets a base scenario disappear together with its
// route (ADR 0160). The record names the route; the checker, not the review,
// proves that the route left the production router golden in the same change.
type queryBudgetRetirement struct {
	Scenario string `json:"scenario"`
	Route    string `json:"route"`
	Issue    int    `json:"issue"`
}

// retiredQueryBudgets returns the base scenarios this change retires and the
// violations of retirement records it adds. Only a record the base does not
// hold yet retires anything: an unchanged record is history, never a standing
// permission. A new record must retire a scenario of the base register whose
// route the base router golden lists and the candidate golden no longer does.
func retiredQueryBudgets(root, project, sha string, base, candidate map[string]querybudget.Budget) (map[string]bool, []string, error) {
	added, err := addedQueryBudgetRetirements(root, project, sha)
	if err != nil || len(added) == 0 {
		return nil, nil, err
	}
	baseRoutes, err := baseRouteGolden(root, project, sha)
	if err != nil {
		return nil, nil, fmt.Errorf("read route golden at base %s: %w", sha, err)
	}
	candidateRoutes, err := candidateRouteGolden(project)
	if err != nil {
		return nil, nil, fmt.Errorf("read candidate route golden: %w", err)
	}
	retired := map[string]bool{}
	var violations []string
	for _, record := range added {
		reject := func(reason string) {
			violations = append(violations, fmt.Sprintf("%s retirement (%s) %s", record.Scenario, record.Route, reason))
		}
		_, registered := base[record.Scenario]
		_, active := candidate[record.Scenario]
		switch {
		case !registered:
			reject("retires no budget of the base register")
		case active:
			reject("names a budget that is still registered")
		case !baseRoutes[record.Route]:
			reject("names a route the base router golden does not list")
		case candidateRoutes[record.Route]:
			reject("names a route the router golden still lists")
		default:
			retired[record.Scenario] = true
		}
	}
	return retired, violations, nil
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

func baseRouteGolden(root, project, sha string) (map[string]bool, error) {
	prefix, err := repositoryRelativePath(root, project, routeGoldenDirectory)
	if err != nil {
		return nil, err
	}
	listing, err := gitOutput(root, "ls-tree", "-r", "-z", "--name-only", sha, "--", filepath.ToSlash(prefix))
	if err != nil {
		return nil, err
	}
	routes := map[string]bool{}
	for _, name := range strings.Split(listing, "\x00") {
		if !strings.HasSuffix(name, ".golden") {
			continue
		}
		source, err := gitOutput(root, "cat-file", "blob", sha+":"+name)
		if err != nil {
			return nil, err
		}
		if err := addGoldenRoutes(routes, name, source); err != nil {
			return nil, err
		}
	}
	return routes, nil
}

func candidateRouteGolden(project string) (map[string]bool, error) {
	routes := map[string]bool{}
	directory := filepath.Join(project, routeGoldenDirectory)
	err := filepath.WalkDir(directory, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(name, ".golden") {
			return err
		}
		source, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		return addGoldenRoutes(routes, name, string(source))
	})
	if errors.Is(err, fs.ErrNotExist) {
		return routes, nil
	}
	return routes, err
}

// addGoldenRoutes fails closed on a line that is no route: a route the
// checker cannot read must not count as removed.
func addGoldenRoutes(routes map[string]bool, name, source string) error {
	scanner := bufio.NewScanner(strings.NewReader(source))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !routeGoldenLine.MatchString(line) {
			return fmt.Errorf("%s: unreadable route golden line %q", name, line)
		}
		routes[line] = true
	}
	return scanner.Err()
}
