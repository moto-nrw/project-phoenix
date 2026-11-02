package querybudgets

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
)

// Test names retain fixture-size subtests when several calls share a scenario.
type budgetReport struct {
	Scenario            string         `json:"scenario"`
	Test                string         `json:"test"`
	Actual              int            `json:"actual"`
	Max                 int            `json:"max"`
	Exact               bool           `json:"exact"`
	DuplicateStatements map[string]int `json:"duplicate_statements"`
}

type feedbackTB interface {
	Name() string
	Logf(string, ...any)
	Errorf(string, ...any)
}

func emitQueryFeedback(tb feedbackTB, scenario string, queries []string, budget queryBudget, getenv func(string) string) {
	if getenv("PHX_QUERY_DUMP") == "1" {
		tb.Logf("query budget %q: %d statements\n%s", scenario, len(queries), indentQueries(queries))
	}
	if path := getenv("PHX_QUERY_BUDGET_REPORT"); path != "" {
		report := budgetReport{
			Scenario: scenario, Test: tb.Name(), Actual: len(queries),
			Max: budget.max, Exact: budget.exact, DuplicateStatements: duplicateStatements(queries),
		}
		if err := appendBudgetReport(path, report); err != nil {
			tb.Errorf("write query budget report %q: %v", path, err)
		}
	}
}

func duplicateStatements(queries []string) map[string]int {
	counts := make(map[string]int)
	for _, query := range queries {
		counts[normalizeSQL(query)]++
	}
	for form, count := range counts {
		if count < 2 {
			delete(counts, form)
		}
	}
	return counts
}

// Each binary opens independently. flock also protects arbitrarily long SQL
// forms across binaries, rather than relying on a pipe-size atomicity bound.
// Closing the descriptor releases its lock, including all error paths.
func appendBudgetReport(path string, report budgetReport) (err error) {
	line, err := json.Marshal(report)
	if err != nil {
		return err
	}
	access := os.O_WRONLY
	if runtime.GOOS == "windows" {
		// LockFileEx needs a generic access right in addition to append data.
		access = os.O_RDWR
	}
	file, err := os.OpenFile(path, access|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()
	if err := lockBudgetReport(file); err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	line = append(line, '\n')
	n, err := file.Write(line)
	if err == nil && n != len(line) {
		return io.ErrShortWrite
	}
	return err
}
