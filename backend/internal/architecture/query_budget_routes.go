package architecture

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/moto-nrw/project-phoenix/architecture/querybudget"
)

const queryBudgetNonHTTPPath = "architecture/query-budget-non-http.json"

// Explicit scenario/reason pairs are immutable once introduced. Comparing
// membership, not size, also rejects exchanging one exception for another.
func validateQueryBudgetRoutes(root, project, sha string, budgets map[string]querybudget.Budget, routes map[string]bool) ([]string, error) {
	source, err := os.ReadFile(filepath.Join(project, queryBudgetNonHTTPPath))
	missing := os.IsNotExist(err)
	if missing {
		source = []byte(`{}`)
	} else if err != nil {
		return nil, err
	}
	exceptions, err := parseNonHTTPBudgets(source)
	if err != nil {
		return nil, fmt.Errorf("read non-HTTP query budgets: %w", err)
	}
	baseSource, exists, err := baseProjectFile(root, project, sha, queryBudgetNonHTTPPath)
	if err != nil {
		return nil, err
	}
	var violations []string
	if exists {
		if missing {
			return nil, fmt.Errorf("non-HTTP query budget register must remain present")
		}
		before, err := parseNonHTTPBudgets(baseSource)
		if err != nil {
			return nil, err
		}
		for scenario, reason := range exceptions {
			if before[scenario] != reason {
				violations = append(violations, scenario+" non-HTTP exception added or changed")
			}
		}
	}
	for scenario, reason := range exceptions {
		budget, registered := budgets[scenario]
		if !registered || len(budget.Route) != 0 || strings.TrimSpace(reason) == "" {
			violations = append(violations, scenario+" invalid non-HTTP exception: needs a registered routeless budget and reason")
		}
	}
	for scenario, budget := range budgets {
		if len(budget.Route) != 0 {
			for _, route := range budget.Route {
				if !routes[route] {
					violations = append(violations, fmt.Sprintf("%s route %q is absent from router golden", scenario, route))
				}
			}
		} else if strings.HasPrefix(scenario, "api.") && exceptions[scenario] == "" {
			violations = append(violations, scenario+" missing route and non-HTTP exception")
		}
	}
	return violations, nil
}

func parseNonHTTPBudgets(source []byte) (map[string]string, error) {
	decoder := json.NewDecoder(bytes.NewReader(source))
	// Tokenize to reject duplicate keys, rather than silently keeping the last.
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("non-HTTP query budgets must be an object")
	}
	records := map[string]string{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		scenario, ok := token.(string)
		if !ok || strings.TrimSpace(scenario) == "" {
			return nil, fmt.Errorf("empty non-HTTP scenario")
		}
		if _, duplicate := records[scenario]; duplicate {
			return nil, fmt.Errorf("duplicate non-HTTP scenario %q", scenario)
		}
		var reason string
		if err := decoder.Decode(&reason); err != nil {
			return nil, err
		}
		if strings.TrimSpace(reason) == "" {
			return nil, fmt.Errorf("%s needs a non-HTTP reason", scenario)
		}
		records[scenario] = reason
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("trailing non-HTTP query budget content")
	}
	return records, nil
}
