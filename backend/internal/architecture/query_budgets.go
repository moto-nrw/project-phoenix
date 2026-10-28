package architecture

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/architecture/querybudget"
)

// CompareQueryBudgets preserves each SQL contract against an immutable Git
// base, including the historical Go-map representation. New scenarios establish
// a budget; an existing scenario cannot disappear, grow or lose exactness.
func CompareQueryBudgets(project, ref string) error {
	root, err := gitOutput(project, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	root = strings.TrimSpace(root)
	sha, err := resolveBaseCommit(root, ref)
	if err != nil {
		return err
	}
	base, err := baseQueryBudgets(root, project, sha)
	if err != nil {
		return fmt.Errorf("read query budgets at base %s: %w", sha, err)
	}
	candidate, err := candidateQueryBudgets(project)
	if err != nil {
		return fmt.Errorf("read candidate query budgets: %w", err)
	}
	var violations []string
	for scenario, before := range base {
		after, exists := candidate[scenario]
		switch {
		case !exists:
			violations = append(violations, fmt.Sprintf("%s disappeared", scenario))
		case after.Max > before.Max:
			violations = append(violations, fmt.Sprintf("%s max: %d -> %d", scenario, before.Max, after.Max))
		case before.Exact && !after.Exact:
			violations = append(violations, fmt.Sprintf("%s exact: true -> false", scenario))
		}
	}
	if len(violations) != 0 {
		sort.Strings(violations)
		return fmt.Errorf("query budgets weakened against base %s:\n  %s", sha, strings.Join(violations, "\n  "))
	}
	return nil
}

func candidateQueryBudgets(project string) (map[string]querybudget.Budget, error) {
	_, err := os.Stat(filepath.Join(project, "test", "query-budgets"))
	if err == nil {
		return querybudget.ReadQueryBudgets(os.DirFS(project), "test/query-budgets")
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	source, err := os.ReadFile(filepath.Join(project, "test", "query_budgets.go"))
	if err != nil {
		return nil, err
	}
	return querybudget.ReadHistorical(source)
}

func baseQueryBudgets(root, project, sha string) (map[string]querybudget.Budget, error) {
	prefix, err := repositoryRelativePath(root, project, "test/query-budgets")
	if err != nil {
		return nil, err
	}
	listing, err := gitOutput(root, "ls-tree", "-r", "-z", sha, "--", prefix)
	if err != nil {
		return nil, err
	}
	if listing == "" {
		source, err := readGitBlob(root, project, sha, "test/query_budgets.go")
		if err != nil {
			return nil, err
		}
		return querybudget.ReadHistorical(source)
	}
	budgets := map[string]querybudget.Budget{}
	for _, record := range strings.Split(listing, "\x00") {
		if record == "" {
			continue
		}
		budget, include, err := baseQueryBudgetRecord(root, sha, record)
		if err != nil {
			return nil, err
		}
		if !include {
			continue
		}

		if _, exists := budgets[budget.Scenario]; exists {
			return nil, fmt.Errorf("duplicate base query budget %q", budget.Scenario)
		}
		budgets[budget.Scenario] = budget
	}
	if len(budgets) == 0 {
		return nil, fmt.Errorf("empty base query budget register")
	}
	return budgets, nil
}

func baseQueryBudgetRecord(root, sha, record string) (querybudget.Budget, bool, error) {
	metadata, name, ok := strings.Cut(record, "\t")
	fields := strings.Fields(metadata)
	if !ok || len(fields) != 3 || fields[0] != "100644" && fields[0] != "100755" {
		return querybudget.Budget{}, false, fmt.Errorf("invalid query budget tree entry %q", record)
	}
	if !strings.HasSuffix(name, ".json") {
		return querybudget.Budget{}, false, nil
	}
	source, err := gitOutput(root, "cat-file", "blob", sha+":"+name)
	if err != nil {
		return querybudget.Budget{}, false, err
	}
	budget, err := querybudget.Parse([]byte(source))
	if err != nil {
		return querybudget.Budget{}, false, fmt.Errorf("%s: %w", name, err)
	}
	return budget, true, nil
}
