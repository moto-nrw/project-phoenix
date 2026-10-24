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
		{"register moved out of the ratchet files", "test/example_test.go", strings.Replace(ratchetBudgetBase, `"b": 3`, `"b": 4`, 1), "", "register Budgets (sum 5 at base) disappeared"},
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

// Review of #4095, C1: a measurement is a Go constant expression, not only a
// literal, and a base measurement the evaluation cannot read fails.
func TestCompareRatchetBudgetsEvaluatesConstantExpressions(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, old, new, want string }{
		{"threshold arithmetic", "Threshold = 15", "Threshold = 15 + 1", "constant Threshold: 15 -> 16"},
		{"map entry arithmetic", `"b": 3`, `"b": 3 + 1`, `Budgets["b"]: 3 -> 4`},
		{"parentheses", "Threshold = 15", "Threshold = (16)", "constant Threshold: 15 -> 16"},
		{"constant reference", "const Threshold = 15", "const Limit = 16\n\nconst Threshold = Limit", "constant Threshold: 15 -> 16"},
		{"constant reference in a register", `"b": 3`, `"b": Threshold - 11`, `Budgets["b"]: 3 -> 4`},
		{"iota", "const Threshold = 15", "const (\n\tZero = iota\n\tThreshold = iota * 16\n)", "constant Threshold: 15 -> 16"},
		{"conversion", "Threshold = 15", "Threshold = int(16)", "constant Threshold: 15 -> 16"},
		{"named map type", "var Budgets = map[string]int{", "type registry map[string]int\n\nvar Budgets = registry{\n\t\"c\": 1,", "Budgets: entries sum 5 -> 6"},
		{"equal value as an expression", "Threshold = 15", "Threshold = 14 + 1", ""},
		{"builtin constant function", "Threshold = 15", `Threshold = len("sixteen letters!")`, "constant Threshold: 15 -> 16"},
		{"unreadable constant", "const Threshold = 15", "var limit = 16\n\nconst Threshold = limit", "constant Threshold cannot be measured any more"},
		{"register built by a function", "var Budgets = map[string]int{", "var Budgets = func() map[string]int { return map[string]int{\"x\": 9} }()\n\nvar unused = map[string]int{", "register Budgets cannot be measured any more"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			base, candidate := newRatchetBudgets(), newRatchetBudgets()
			source := strings.Replace(ratchetBudgetBase, tt.old, tt.new, 1)
			if source == ratchetBudgetBase {
				t.Fatalf("replacement %q did not apply", tt.old)
			}
			if err := base.add("test/example_ratchet_test.go", []byte(ratchetBudgetBase)); err != nil {
				t.Fatal(err)
			}
			if err := candidate.add("test/example_ratchet_test.go", []byte(source)); err != nil {
				t.Fatal(err)
			}
			raises := strings.Join(compareRatchetBudgets(base, candidate, nil), "\n")
			if tt.want == "" && raises != "" || !strings.Contains(raises, tt.want) {
				t.Fatalf("want %q, got %q\n%s", tt.want, raises, source)
			}
		})
	}
}

// Review of #4095, C2: renaming or moving a measurement keeps its history.
func TestCompareRatchetBudgetsFollowsRenamedMeasurements(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		edit *strings.Replacer
		adrs map[string]bool
		want string
	}{
		{"renamed and raised without a marker", strings.NewReplacer("Budgets", "OtherBudgets", `"b": 3`, `"b": 4`, "const Total = 5", "const OtherTotal = 6"), nil, "register Budgets (sum 5 at base) disappeared"},
		{"renamed total without a marker", strings.NewReplacer("const Total = 5", "const OtherTotal = 5"), nil, "constant Total (5 at base) disappeared"},
		{"renamed with a marker", strings.NewReplacer("// Budgets is a register.\nvar Budgets", "// OtherBudgets is a register, renamed from Budgets.\nvar OtherBudgets", "// Total is the sum of Budgets.\nconst Total", "// OtherTotal, renamed from Total.\nconst OtherTotal"), nil, ""},
		{"renamed with a marker and raised", strings.NewReplacer("// Budgets is a register.\nvar Budgets", "// Renamed from Budgets.\nvar OtherBudgets", `"b": 3`, `"b": 4`, "// Total is the sum of Budgets.\nconst Total = 5", "// Renamed from Total.\nconst OtherTotal = 6"), nil, `Budgets["b"]: 3 -> 4`},
		{"marker beside the kept original", strings.NewReplacer("var Selectors", "// Renamed from Budgets.\nvar Copy = map[string]int{\"z\": 9}\n\nvar Selectors"), nil, ""},
		{"removed exception register", strings.NewReplacer("var Exceptions = map[string]string{\n\t\"f\": \"reason\",\n}\n", ""), nil, "exception register Exceptions (1 entries at base) disappeared"},
		{"removal under an added ADR", strings.NewReplacer("var Exceptions = map[string]string{\n\t\"f\": \"reason\",\n}\n", "// Exceptions retired with ADR 0999.\n"), map[string]bool{"0999": true}, ""},
		{"turned into a variable", strings.NewReplacer("const Threshold = 15", "var Threshold = 16"), nil, "constant Threshold cannot be measured any more"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			base, candidate := newRatchetBudgets(), newRatchetBudgets()
			source := tt.edit.Replace(ratchetBudgetBase)
			if source == ratchetBudgetBase {
				t.Fatal("edit did not apply")
			}
			if err := base.add("test/example_ratchet_test.go", []byte(ratchetBudgetBase)); err != nil {
				t.Fatal(err)
			}
			if err := candidate.add("test/example_ratchet_test.go", []byte(source)); err != nil {
				t.Fatal(err)
			}
			raises := strings.Join(compareRatchetBudgets(base, candidate, tt.adrs), "\n")
			if tt.want == "" && raises != "" || !strings.Contains(raises, tt.want) {
				t.Fatalf("want %q, got %q\n%s", tt.want, raises, source)
			}
		})
	}
	empty := newRatchetBudgets()
	retired := newRatchetBudgets()
	if err := empty.add("test/empty_ratchet_test.go", []byte("package test\n\nvar Allowlist = map[string]int{}\n")); err != nil {
		t.Fatal(err)
	}
	if err := retired.add("test/other_ratchet_test.go", []byte("package test\n")); err != nil {
		t.Fatal(err)
	}
	if raises := compareRatchetBudgets(empty, retired, nil); len(raises) != 0 {
		t.Fatalf("an empty register may go: %v", raises)
	}
}

// Review of #4095, C6: an ADR number the base already has authorizes nothing,
// whether its file was renamed or copied.
func TestAddedADRsIgnoreNumbersTheBaseHas(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		prepare func(t *testing.T, repo string)
		want    bool
	}{
		{"renamed", func(t *testing.T, repo string) {
			runGit(t, repo, "mv", "docs/adr/0001-existing.md", "docs/adr/0001-renamed.md")
		}, false},
		{"copied", func(t *testing.T, repo string) {
			writeFile(t, filepath.Join(repo, "docs", "adr", "0001-copy.md"), "# Existing\n")
		}, false},
		{"new number", func(t *testing.T, repo string) {
			writeFile(t, filepath.Join(repo, "docs", "adr", "0002-new.md"), "# New\n")
		}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, _ := ratchetRepository(t, legacyRecord(2583))
			writeFile(t, filepath.Join(repo, "docs", "adr", "0001-existing.md"), "# Existing\n")
			writeFile(t, filepath.Join(repo, "test", "example_ratchet_test.go"), ratchetBudgetBase)
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-qm", "ADR base")
			base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
			tt.prepare(t, repo)
			number := "0001"
			if tt.want {
				number = "0002"
			}
			writeFile(t, filepath.Join(repo, "test", "example_ratchet_test.go"), strings.Replace(ratchetBudgetBase, "const Threshold = 15", "// Raised by ADR "+number+".\nconst Threshold = 16", 1))
			err := CompareRatchetBudgets(repo, base)
			if tt.want && err != nil {
				t.Fatalf("a new ADR number must authorize the raise: %v", err)
			}
			if !tt.want && (err == nil || !strings.Contains(err.Error(), "constant Threshold: 15 -> 16")) {
				t.Fatalf("ADR %s exists at the base and must not authorize the raise: %v", number, err)
			}
		})
	}
}

// Review of #4095, round 2, C1: a typed constant keeps its Go value, so
// ^uint8(0) is 255 and a raise, not -1.
func TestCompareRatchetBudgetsUsesTheCompilerValue(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, old, new, want string }{
		{"typed complement", "Threshold = 15", "Threshold = ^uint8(0)", "constant Threshold: 15 -> 255"},
		{"typed complement in a register", `"b": 3`, `"b": int(^uint8(0))`, `Budgets["b"]: 3 -> 255`},
		{"typed constant reference", "const Threshold = 15", "type limit uint8\n\nconst ceiling limit = ^limit(0)\n\nconst Threshold = int(ceiling) - 239", "constant Threshold: 15 -> 16"},
		{"overflowing conversion", "Threshold = 15", "Threshold = uint8(256)", "constant Threshold cannot be measured any more"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			base, candidate := newRatchetBudgets(), newRatchetBudgets()
			source := strings.Replace(ratchetBudgetBase, tt.old, tt.new, 1)
			if err := base.add("test/example_ratchet_test.go", []byte(ratchetBudgetBase)); err != nil {
				t.Fatal(err)
			}
			if err := candidate.add("test/example_ratchet_test.go", []byte(source)); err != nil {
				t.Fatal(err)
			}
			if raises := strings.Join(compareRatchetBudgets(base, candidate, nil), "\n"); !strings.Contains(raises, tt.want) {
				t.Fatalf("want %q, got %q", tt.want, raises)
			}
		})
	}
	repo, _ := ratchetRepository(t, legacyRecord(2583))
	path := filepath.Join(repo, "test", "example_ratchet_test.go")
	writeFile(t, path, ratchetBudgetBase)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "budget base")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	writeFile(t, path, strings.Replace(ratchetBudgetBase, "Threshold = 15", "Threshold = ^uint8(0)", 1))
	if err := CompareRatchetBudgets(repo, base); err == nil || !strings.Contains(err.Error(), "constant Threshold: 15 -> 255") {
		t.Fatalf("public comparison accepted 15 -> 255: %v", err)
	}
}

// Review of #4095, round 2, C2: a history continues in exactly one
// successor, and the result does not depend on map iteration order.
func TestCompareRatchetBudgetsRejectsAmbiguousRenames(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, base, candidate, want string }{
		{"two successors", "package test\nvar Budgets = map[string]int{\"a\": 5}\n",
			"package test\n// renamed from Budgets\nvar Kept = map[string]int{\"a\": 5}\n// renamed from Budgets\nvar Raised = map[string]int{\"a\": 6}\n",
			"Budgets is claimed by several successors (Kept, Raised)"},
		{"two predecessors", "package test\nconst A = 1\nconst B = 2\n",
			"package test\n// Renamed from A, moved from B.\nconst C = 3\n",
			"C names several predecessors (A, B)"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			base, candidate := newRatchetBudgets(), newRatchetBudgets()
			if err := base.add("test/example_ratchet_test.go", []byte(tt.base)); err != nil {
				t.Fatal(err)
			}
			if err := candidate.add("test/example_ratchet_test.go", []byte(tt.candidate)); err != nil {
				t.Fatal(err)
			}
			first := strings.Join(compareRatchetBudgets(base, candidate, nil), "\n")
			if !strings.Contains(first, tt.want) {
				t.Fatalf("want %q, got %q", tt.want, first)
			}
			for range 256 {
				if again := strings.Join(compareRatchetBudgets(base, candidate, nil), "\n"); again != first {
					t.Fatalf("result depends on iteration order:\n%s\n---\n%s", first, again)
				}
			}
		})
	}
}

// A measurement that disappears needs an ADR the change adds, even when a
// comment cites an existing one. The one-time ADR 0057 exception for the
// temporary-rule registers is gone, so their file has no exception either.
func TestCompareRatchetBudgetsRetiresOnlyUnderAnAddedADR(t *testing.T) {
	t.Parallel()
	const temporaryRules = "package test\n\nconst (\n\tpolicyTempRulesTotal = 2\n\tpolicyTempRulesCompatTotal = 49\n)\n\nvar policyTempRulesFamilies = map[string]int{\"a\": 1, \"b\": 1}\n"
	for _, tt := range []struct {
		name, file, base, candidate, want string
		adrs                              map[string]bool
	}{
		{"former exception file", "test/policy_temporary_rules_ratchet_test.go", temporaryRules, "package test\n", "constant policyTempRulesTotal (2 at base) disappeared", nil},
		{"register with an existing ADR", "test/example_ratchet_test.go", ratchetBudgetBase, strings.Replace(ratchetBudgetBase, "// Budgets is a register.\nvar Budgets = map[string]int{\n\t\"a\": 2,\n\t\"b\": 3,\n}\n", "// Budgets is retired under ADR 0057.\n", 1), "register Budgets (sum 5 at base) disappeared", nil},
		{"register with an added ADR", "test/example_ratchet_test.go", ratchetBudgetBase, strings.Replace(ratchetBudgetBase, "// Budgets is a register.\nvar Budgets = map[string]int{\n\t\"a\": 2,\n\t\"b\": 3,\n}\n", "// Budgets is retired under ADR 0999.\n", 1), "", map[string]bool{"0999": true}},
		{"raise under an existing ADR", "test/policy_temporary_rules_ratchet_test.go", temporaryRules, strings.Replace(temporaryRules, "policyTempRulesTotal = 2", "// Raised under ADR 0057.\n\tpolicyTempRulesTotal = 3", 1), "constant policyTempRulesTotal: 2 -> 3", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			base, candidate := newRatchetBudgets(), newRatchetBudgets()
			if err := base.add(tt.file, []byte(tt.base)); err != nil {
				t.Fatal(err)
			}
			if err := candidate.add(tt.file, []byte(tt.candidate)); err != nil {
				t.Fatal(err)
			}
			raises := strings.Join(compareRatchetBudgets(base, candidate, tt.adrs), "\n")
			if tt.want == "" && raises != "" || !strings.Contains(raises, tt.want) {
				t.Fatalf("want %q, got %q", tt.want, raises)
			}
		})
	}
}
