package architecture

import (
	"path/filepath"
	"strings"
	"testing"
)

const ratchetBudgetBase = `package test

// Total is the sum of Budgets.
const Total = 5

const Threshold = 15

// Budgets is a register.
var Budgets = map[string]int{
	"a": 2,
	"b": 3,
}

var Exceptions = map[string]string{
	"f": "reason",
}

var Selectors = map[string]bool{"x": true}
`

func TestCompareRatchetBudgetsRejectsRaisesAgainstTheBase(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, file, candidate, adr, want string
	}{
		{"unchanged", "test/example_ratchet_test.go", ratchetBudgetBase, "", ""},
		{"lowered entry and total", "test/example_ratchet_test.go", strings.NewReplacer(`"b": 3`, `"b": 1`, "Total = 5", "Total = 3").Replace(ratchetBudgetBase), "", ""},
		{"removed entry", "test/example_ratchet_test.go", strings.Replace(ratchetBudgetBase, "\t\"b\": 3,\n", "", 1), "", ""},
		{"moved entry", "test/example_ratchet_test.go", strings.Replace(ratchetBudgetBase, `"b": 3`, `"c": 3`, 1), "", ""},
		{"raised entry", "test/example_ratchet_test.go", strings.Replace(ratchetBudgetBase, `"b": 3`, `"b": 4`, 1), "", `Budgets["b"]: 3 -> 4`},
		{"new entry", "test/example_ratchet_test.go", strings.Replace(ratchetBudgetBase, `"b": 3,`, `"b": 3, "c": 1,`, 1), "", "Budgets: entries sum 5 -> 6"},
		{"raised total", "test/example_ratchet_test.go", strings.Replace(ratchetBudgetBase, "Total = 5", "Total = 6", 1), "", "constant Total: 5 -> 6"},
		{"raised threshold", "test/example_ratchet_test.go", strings.Replace(ratchetBudgetBase, "Threshold = 15", "Threshold = 0x10", 1), "", "constant Threshold: 15 -> 16"},
		{"new exception", "test/example_ratchet_test.go", strings.Replace(ratchetBudgetBase, `"f": "reason",`, `"f": "reason", "g": "reason",`, 1), "", "Exceptions: 1 -> 2 entries"},
		{"configuration is not compared", "test/example_ratchet_test.go", strings.Replace(ratchetBudgetBase, `{"x": true}`, `{"x": true, "y": true}`, 1), "", ""},
		{"register moved to another ratchet file", "test/other_ratchet_test.go", strings.Replace(ratchetBudgetBase, `"b": 3`, `"b": 4`, 1), "", `Budgets["b"]: 3 -> 4`},
		{"files outside the ratchets", "test/example_test.go", strings.Replace(ratchetBudgetBase, `"b": 3`, `"b": 4`, 1), "", ""},
		{"raise with an added ADR", "test/example_ratchet_test.go", strings.Replace(ratchetBudgetBase, `"b": 3,`, "// Raised by ADR 0999.\n\t\"b\": 4,", 1), "docs/adr/0999-raise.md", ""},
		{"raise citing an existing ADR", "test/example_ratchet_test.go", strings.Replace(ratchetBudgetBase, `"b": 3,`, "// Raised by ADR 0001.\n\t\"b\": 4,", 1), "docs/adr/0999-raise.md", `Budgets["b"]: 3 -> 4`},
		{"constant raise with an added ADR", "test/example_ratchet_test.go", strings.Replace(ratchetBudgetBase, "// Total is the sum of Budgets.", "// Total is the sum of Budgets; raised by ADR 0999.", 1), "docs/adr/0999-raise.md", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, _ := ratchetRepository(t, legacyRecord(2583))
			writeFile(t, filepath.Join(repo, "docs", "adr", "0001-existing.md"), "# Existing\n")
			writeFile(t, filepath.Join(repo, "test", "example_ratchet_test.go"), ratchetBudgetBase)
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-qm", "ratchet base")
			base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
			if tt.file != "test/example_ratchet_test.go" {
				runGit(t, repo, "rm", "-q", "test/example_ratchet_test.go")
			}
			writeFile(t, filepath.Join(repo, filepath.FromSlash(tt.file)), tt.candidate)
			if tt.adr != "" {
				writeFile(t, filepath.Join(repo, filepath.FromSlash(tt.adr)), "# Raise\n")
			}
			err := CompareRatchetBudgets(repo, base)
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want %q, got %v", tt.want, err)
			}
		})
	}
}

func TestCheckRejectsRatchetBudgetRaise(t *testing.T) {
	t.Parallel()
	repo, _ := ratchetRepository(t, legacyRecord(2583))
	path := filepath.Join(repo, "test", "example_ratchet_test.go")
	writeFile(t, path, ratchetBudgetBase)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "ratchet base")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	writeFile(t, path, strings.Replace(ratchetBudgetBase, `"a": 2`, `"a": 3`, 1))
	output, err := runArchitecture(t, "check", "--project", repo, "--baseline", "architecture/legacy.jsonl", "--base-ref", base)
	if err == nil || !strings.Contains(output, `Budgets["a"]: 2 -> 3`) {
		t.Fatalf("CLI accepted a raised ratchet budget: %v\n%s", err, output)
	}
}
