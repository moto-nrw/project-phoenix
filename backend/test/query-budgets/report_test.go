package querybudgets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestNormalizeSQLLiteralsAndIdentifiers(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, first, second, want string }{
		{"boolean and null literals", `SELECT TRUE, false, NULL, "true"`, `select false, TRUE, null, "true"`, `select ? , ? , ? , "true"`},
		{"numeric forms", `SELECT 1, .2, 3., 4e-5, 0xFF, 0o77, 0b10, 1_000`, `select 9, .8, 7., 6E+5, 0xAB, 0o10, 0b11, 2_000`, "select ? , ? , ? , ? , ? , ? , ? , ?"},
		{"quoted strings", `SELECT 'a''b', E'a\'b', U&'d\0061t', B'0101', X'ff', N'abc'`, `select 'c', E'c\\d', U&'other', B'1111', X'aa', N'other'`, "select ? , ? , ? , ? , ? , ?"},
		{"dollar strings", `SELECT $$a'b -- /*$foo$'$$, $tag$contents $$ 123 $tag$`, `select $$other$$, $else$more$else$`, "select ? , ?"},
		{"comments", "SELECT/* outer /*nested*/end*/1--comment\n,2", "select 3,/*comment*/4 -- tail", "select ? , ?"},
		{"identifiers", `SELECT "A1", "a""b", item2, thing$2 FROM "Table" WHERE id=1`, `select "A1", "a""b", ITEM2, thing$2 from "Table" where id=9`, `select "A1" , "a""b" , item2 , thing$2 from "Table" where id = ?`},
		{"standard backslash", `SELECT 'a\', 1`, `SELECT 'b', 2`, "select ? , ?"},
		{"comment-like strings", `select '--not comment /* also not */', $t$/*x*/$t$`, `select 'other', $$anything$$`, "select ? , ?"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, input := range []string{tc.first, tc.second} {
				if got := normalizeSQL(input); got != tc.want {
					t.Errorf("normalizeSQL(%q) = %q, want %q", input, got, tc.want)
				}
			}
		})
	}
	if normalizeSQL(`SELECT "TenantA" FROM x`) == normalizeSQL(`SELECT "TenantB" FROM x`) {
		t.Fatal("different quoted identifiers must not collapse")
	}
}

func TestDuplicateStatementsCountOnlyRepeatedForms(t *testing.T) {
	t.Parallel()
	queries := []string{`SELECT "id" FROM students WHERE id=123`, `SELECT "id" FROM students WHERE id=456`, `SELECT "id" FROM rooms WHERE id=123`}
	want := map[string]int{`select "id" from students where id = ?`: 2}
	if got := duplicateStatements(queries); !reflect.DeepEqual(got, want) {
		t.Fatalf("duplicates = %#v, want %#v", got, want)
	}
	if got := duplicateStatements(nil); got == nil || len(got) != 0 {
		t.Fatalf("empty measurements must encode an empty object, got %#v", got)
	}
}

type feedbackRecorder struct{ logs, errors []string }

func (*feedbackRecorder) Name() string { return "TestList/N=9" }
func (r *feedbackRecorder) Logf(format string, args ...any) {
	r.logs = append(r.logs, fmt.Sprintf(format, args...))
}
func (r *feedbackRecorder) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func TestQueryFeedbackOptIn(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "report.jsonl")
	queries := []string{"SELECT 1", "SELECT 2"}
	budget := queryBudget{max: 2, exact: true}
	var recorder feedbackRecorder
	emitQueryFeedback(&recorder, "fixture", queries, budget, func(string) string { return "" })
	if len(recorder.logs) != 0 || len(recorder.errors) != 0 {
		t.Fatalf("feedback without environment must be silent: %+v", recorder)
	}
	emitQueryFeedback(&recorder, "fixture", queries, budget, func(key string) string {
		if key == "PHX_QUERY_DUMP" {
			return "1"
		}
		return path
	})
	if len(recorder.logs) != 1 || !strings.Contains(recorder.logs[0], indentQueries(queries)) {
		t.Fatalf("dump must include existing indentQueries output: %+v", recorder)
	}
	if len(recorder.errors) != 0 {
		t.Fatal(recorder.errors)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var report budgetReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.Scenario != "fixture" || report.Test != recorder.Name() || report.Actual != 2 || report.Max != 2 || !report.Exact || report.DuplicateStatements["select ?"] != 2 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestQueryFeedbackWriteFailureFailsAssertion(t *testing.T) {
	t.Parallel()
	var recorder feedbackRecorder
	emitQueryFeedback(&recorder, "fixture", nil, queryBudget{}, func(key string) string {
		if key == "PHX_QUERY_DUMP" {
			return "true"
		}
		return t.TempDir()
	})
	if len(recorder.logs) != 0 {
		t.Fatal("only PHX_QUERY_DUMP=1 enables the dump")
	}
	if len(recorder.errors) != 1 || !strings.Contains(recorder.errors[0], "write query budget report") {
		t.Fatalf("write failure did not fail assertion: %+v", recorder)
	}
}

// Every writer opens its own descriptor, exercising the same flock contract
// used by concurrent test binaries without adding process execution to this owner.
func TestBudgetReportConcurrentWriters(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "shared.jsonl")
	report := budgetReport{Scenario: "concurrent", Test: t.Name(), Actual: 2, Max: 2,
		DuplicateStatements: map[string]int{strings.Repeat("select x ", 10000): 2}}
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			for range 10 {
				if err := appendBudgetReport(path, report); err != nil {
					t.Errorf("writer: %v", err)
				}
			}
		})
	}
	wg.Wait()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 30 {
		t.Fatalf("got %d lines, want 30", len(lines))
	}
	for _, line := range lines {
		var decoded budgetReport
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("interleaved JSONL: %v", err)
		}
		if !reflect.DeepEqual(decoded, report) {
			t.Fatal("truncated or corrupted report")
		}
	}
}
