package test

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestModuleLegacyBudgetRatchet caps the production LOC of every legacy tree
// under backend/modules/ and backend/workflows/ at today's measured size.
//
// Why this ratchet exists: 92,928 production LOC sit in directories named
// "legacy" below backend/modules/ and backend/workflows/. They were moved
// there out of the old top-level packages, file by file, at 77-99% rename
// similarity. backend/architecture/policy.json classifies those 25 packages
// with ordinary roles (adapter 16, postgres 3, domain 2, compose 2,
// test-support 2) — "adapter" is not a debt marker. Only 56 entries in
// backend/architecture/legacy.jsonl have a legacy target at all, and they
// reach five packages holding 7,939 LOC: 84,989 of these LOC carry no exact
// tuple, so the architecture evaluator reports green and cannot point at that
// surface. No nest has shrunk since it was created, only grown:
// modules/workforce/legacy went from 15,402 to 21,468 LOC, and the merge that
// produced this seed added modules/careplan/legacy/carelifecycle (4,902 LOC)
// and modules/careplan/legacy/careexitview (382 LOC) in one step. This ratchet
// is the missing per-module completion metric.
//
// Scope: every directory named "legacy" below modules/ or workflows/,
// aggregated per TOPMOST legacy directory — a nested legacy directory inside
// a legacy tree belongs to its parent's entry and gets no entry of its own.
// Counted are the lines of every .go file in the tree except *_test.go. Keys
// are repo-relative slash paths starting at "modules/" or "workflows/".
//
// Budget semantics (the shrink-only shape the other ratchets in this package
// use, counting LOC per tree instead of hits per file):
//
//   - A tree over its budget fails. Moving code into a legacy directory is
//     not progress; never raise a number.
//   - A tree under its budget fails until the entry is lowered, so the
//     reduction cannot silently regress.
//   - A legacy tree without an entry fails: a new legacy tree is almost
//     always the wrong direction.
//   - An entry without a tree fails so the entry gets deleted — that is the
//     success case.
//   - moduleLegacyBudgetTotal caps the sum on top of the per-tree budgets, so
//     shifting LOC from one legacy tree into another shows up as well.
//
// Seed: measured 2026-09-18 at merge commit ecf0003369 with a throwaway
// program (walk modules/ and workflows/ for directories named "legacy", sum
// the lines of all non-test .go files per topmost tree), run as
// `cd backend && ../scripts/run-go-toolchain.sh go run ./tmp/moduleLegacyBudget`.
// The first seed was taken at 19feca2822 and re-measured here after the merge
// of origin/development (a47f77b2c3): PR #3408 (issue #3350) moved the
// care-exit, companion and care-document code out of services/users into the
// new subtree modules/careplan/legacy/carelifecycle, so the numbers below are
// the merged state rather than a raised budget.
// #3427 dissolved modules/careplan/legacy entirely (5,278 LOC, the
// carelifecycle subtree and the careexitview projection) into native Care
// Plan, Timetable and the student directory projection; its entry is gone.
// #3422 dissolved the Student Presence legacy nest: first its statistics
// package (1,106 LOC) into the native application, then it handed the Care Plan
// status-day and excused-request rows (208 LOC net) to modules/careplan, and
// moved legacy/services/active (10,445 LOC) into the native presence
// application behind the public facades; the consumer switches shrank the
// grouplive, supervisiondashboard and timetable trees. The last slice folded
// legacy/repositories/active and the remaining session rows into the module's
// ports and Postgres adapter, so the tree is gone and its entry with it.
// Slice S5 of #3424 (#3551) moved the timetable reads, the operational day,
// the cleanup and the planning-track administration out of the timetable nest
// to the Timetable owner, shrinking the timetable and supervisiondashboard
// trees. Slice S2 (#3552) moved the template writes, the materialization and
// the roster maintenance after them, roughly halving the timetable tree.
// Slice S3 (#3553) moved the deviation, substitution and sick-report writes,
// the attendance correction and the attendance mirror. #3554 deleted the
// legacy SQL test providers (timetablesqltest). #3556 moved the group
// substitution relations out of the workforce tree. #3413 dissolved
// modules/workforce/legacy/timetracking (13,327 LOC) into the Workforce
// application layer and moved the work-session repository shim (1,158 LOC)
// into the Workforce composition. #3996 moved the staff absence adapters
// into the Workforce composition and trimmed the helpers the substitution
// adapter keeps (1,181 -> 763 LOC). #3487 moved the session token adapter
// (1,358 LOC) unchanged out of modules/identityaccess/legacy to
// modules/identityaccess/sessiontoken (ADR 0054); the tree and its entry are
// gone.
// Re-measure the same way when a number needs to move — downwards.
//
// Two more measurements live here since #4071 (ADR 0057 point 5), because a
// tree that is renamed instead of dissolved falls out of the per-tree budget:
//
//   - moduleLegacyFileBudgets: every non-test file named legacy_*.go anywhere
//     in the backend outside a legacy tree, keyed by its path, with its own
//     total. #3554 moved the last timetable nest into
//     modules/timetable/compose, and its legacy_*.go files kept the name.
//   - moduleComposeBudgets: the production LOC of every owner composition
//     package (a directory below modules/ or workflows/ whose path has a
//     compose or composition segment, legacy trees excluded), with its own
//     total. A composition constructs its owner (ADR 0053); everything else in
//     it is code that still has to move (#4084). The timetable composition
//     alone holds about 36,000 LOC.
//
// Both follow the same four outcomes and are shrink-only. A raise of any entry
// or total against the pull request's base also fails
// `scripts/backend-architecture.sh check` (ratchet budgets); it needs an ADR
// added in the same change.
const moduleLegacyBudgetCheck = "legacy LOC budget"

// moduleLegacyFileBudgetCheck and moduleComposeBudgetCheck name the two
// measurements #4071 added in failure output.
const (
	moduleLegacyFileBudgetCheck = "legacy_*.go LOC budget"
	moduleComposeBudgetCheck    = "compose LOC budget"
)

// moduleLegacyBudgetTotal is the sum of every entry below, measured with the
// same run. It catches LOC moved between two legacy trees, which leaves the
// individual budgets looking fine. Shrink-only, like every entry.
const moduleLegacyBudgetTotal = 3017

// moduleLegacyFileBudgetTotal is the sum of moduleLegacyFileBudgets.
// Shrink-only, like every entry.
const moduleLegacyFileBudgetTotal = 3816

// moduleLegacyFileBudgets maps every legacy_*.go file outside a legacy tree
// to its LOC, first measured on 2026-10-15 on top of development c49b81c3a9:
// 22 files, 3862 LOC. Carriers: the files in owner compositions #4084 (the
// timetable ones came out of the dissolved modules/timetable/legacy nest with
// #3554), api/ #4080, database/repositories/users #3990,
// modules/presenceprojection #4077 and modules/workforce/internal/planning
// #4076. No file gets a new entry.
var moduleLegacyFileBudgets = map[string]int{
	"api/legacy_builders_test_helpers.go":                                     23,
	"api/legacy_token_adapter.go":                                             73,
	"database/repositories/users/legacy_composition_support.go":               35,
	"database/repositories/users/legacy_types.go":                             14,
	"modules/peopledirectory/compose/legacy_repository_runtime.go":            98,
	"modules/presenceprojection/legacy_rows.go":                               357,
	"modules/schoolstructure/compose/legacy_group_catalog.go":                 71,
	"modules/schoolstructure/compose/legacy_group_contract.go":                67,
	"modules/schoolstructure/compose/legacy_group_management_test_helpers.go": 169,
	"modules/schoolstructure/compose/legacy_repository_runtime.go":            53,
	"modules/timetable/compose/legacy_activity_service.go":                    538,
	"modules/timetable/compose/legacy_category_operations.go":                 138,
	"modules/timetable/compose/legacy_composition_support.go":                 63,
	"modules/timetable/compose/legacy_composition_support_test_helpers.go":    70,
	"modules/timetable/compose/legacy_enrollment_operations.go":               570,
	"modules/timetable/compose/legacy_errors.go":                              121,
	"modules/timetable/compose/legacy_group_update.go":                        47,
	"modules/timetable/compose/legacy_interface.go":                           90,
	"modules/timetable/compose/legacy_repository_options.go":                  531,
	"modules/timetable/compose/legacy_schedule_operations.go":                 118,
	"modules/timetable/compose/legacy_supervisor_operations.go":               523,
	"modules/workforce/internal/planning/legacy_helpers.go":                   47,
}

// moduleComposeBudgetTotal is the sum of moduleComposeBudgets. Shrink-only,
// like every entry.
const moduleComposeBudgetTotal = 90386

// moduleComposeBudgets maps every owner composition package to its production
// LOC, first measured on 2026-10-15 on top of development c49b81c3a9: 61
// packages, 90627 LOC. #4084 carries the reduction to construction only;
// modules/timetable/compose is the largest by far. Shrink-only — a new owner
// composition starts small and is no reason to raise the total; split code
// keeps its LOC counted.
var moduleComposeBudgets = map[string]int{
	"modules/appointments/compose":                    528,
	"modules/auditlog/compose":                        275,
	"modules/auditlog/compose/vocabulary":             132,
	"modules/careplan/compose":                        4510,
	"modules/careplan/requestfeed/compose":            101,
	"modules/classday/compose":                        535,
	"modules/communication/composition":               1525,
	"modules/dataimport/compose":                      299,
	"modules/dataimport/compose/retained":             34,
	"modules/dataimport/inbound/compose":              131,
	"modules/delivery/compose":                        818,
	"modules/delivery/compose/email":                  36,
	"modules/delivery/compose/realtime":               30,
	"modules/devicefleet/compose":                     712,
	"modules/devicefleet/compose/httpadapter":         88,
	"modules/devicefleet/compose/repositoryadapter":   298,
	"modules/devicescan/compose":                      1513,
	"modules/emergencysnapshot/compose":               13,
	"modules/enrollment/compose":                      848,
	"modules/exporttransfer/compose":                  323,
	"modules/facilities/compose":                      394,
	"modules/facilities/compose/httpadapter":          491,
	"modules/facilities/compose/repositoryadapter":    242,
	"modules/feedback/compose":                        219,
	"modules/filestorage/compose":                     642,
	"modules/grouplive/compose":                       16,
	"modules/identityaccess/compose":                  7339,
	"modules/mealplan/compose":                        274,
	"modules/organizationtenancy/compose":             746,
	"modules/organizationtenancy/compose/vocabulary":  48,
	"modules/peopledirectory/compose":                 8962,
	"modules/peopledirectory/compose/vocabulary":      136,
	"modules/requestreview/compose":                   1264,
	"modules/schedulerruntime/compose":                85,
	"modules/schoolcalendar/compose":                  572,
	"modules/schoolcalendar/portal/compose":           160,
	"modules/schoolmembership/compose":                869,
	"modules/schoolsetup/compose":                     107,
	"modules/schoolstructure/compose":                 1573,
	"modules/settings/compose":                        587,
	"modules/settings/compose/vocabulary":             126,
	"modules/studentpresence/compose":                 1805,
	"modules/studentpresence/compose/presenceservice": 873,
	"modules/supervisiondashboard/compose":            27,
	"modules/timetable/compose":                       36342,
	"modules/timetable/compose/httpadapter":           3964,
	"modules/timetableblockdisplay/compose":           9,
	"modules/transactionruntime/compose":              173,
	"modules/workforce/compose":                       7454,
	"modules/workforce/compose/httpadapter":           81,
	"modules/workforce/compose/repositoryadapter":     274,
	"workflows/gradetransition/compose":               180,
	"workflows/openroommove/compose":                  81,
	"workflows/parentportal/compose":                  509,
	"workflows/reminderdelivery/compose":              71,
	"workflows/sessionend/compose":                    221,
	"workflows/shiftplansync/compose":                 225,
	"workflows/staffclock/compose":                    36,
	"workflows/staffoffboarding/compose":              224,
	"workflows/studentdeletion/compose":               236,
}

// moduleLegacyBudgets maps a legacy tree to its production LOC on 2026-09-18.
// The comment on each entry names the ticket that is supposed to dissolve the
// tree; "no ticket today" is the measurement, not an omission — those trees
// have no owner and no plan, which is precisely what this ratchet exposes.
var moduleLegacyBudgets = map[string]int{
	// No ticket today.
	"modules/devicefleet/compose/legacy": 231,
	// No ticket today.
	"modules/emergencysnapshot/legacy": 298,
	// No ticket today.
	"modules/facilities/compose/legacy": 518,
	// No ticket today. Two lines under the first seed: #3427 removed the
	// carelifecycle import PR #3408 (#3350) had added to legacy.go, and #3501
	// replaced the user-context service with a local CallerContext port.
	"modules/grouplive/legacy": 539,
	// No ticket today.
	"modules/supervisiondashboard/legacy": 701,
	// No ticket today — and the largest tree of the twelve.
	// No ticket today — grew from 15,402 LOC at creation to 15,666. #3556
	// moved the group substitution relations to the legacy root's binding,
	// #3413 the time-tracking services and the work-session shim out, #3996
	// the staff absence adapters.
	"modules/workforce/legacy": 730,
	// No ticket today.
}

func TestModuleLegacyBudgetRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	counts, err := moduleLegacyBudgetScan(backendRoot)
	if err != nil {
		t.Fatalf("legacy LOC scan failed: %v", err)
	}

	violations := moduleLegacyBudgetViolations(counts, moduleLegacyBudgets)
	violations = append(violations, moduleLegacyBudgetTotalViolations(counts)...)

	files, err := moduleLegacyFileScan(backendRoot)
	if err != nil {
		t.Fatalf("legacy_*.go LOC scan failed: %v", err)
	}
	violations = append(violations, moduleLOCBudgetViolations(moduleLegacyFileBudgetCheck, files, moduleLegacyFileBudgets)...)
	violations = append(violations, moduleLOCBudgetTotalViolations(moduleLegacyFileBudgetCheck, "moduleLegacyFileBudgetTotal", files, moduleLegacyFileBudgetTotal)...)

	compositions, err := moduleComposeScan(backendRoot)
	if err != nil {
		t.Fatalf("compose LOC scan failed: %v", err)
	}
	violations = append(violations, moduleLOCBudgetViolations(moduleComposeBudgetCheck, compositions, moduleComposeBudgets)...)
	violations = append(violations, moduleLOCBudgetTotalViolations(moduleComposeBudgetCheck, "moduleComposeBudgetTotal", compositions, moduleComposeBudgetTotal)...)
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Module legacy LOC budget ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// moduleLegacyBudgetViolations compares the measured LOC per legacy tree
// against the budgets. It does not reuse ratchetViolations because the entries
// here are LOC ceilings per directory tree rather than pattern hits per file,
// and each of the four outcomes needs its own instruction.
func moduleLegacyBudgetViolations(counts, budgets map[string]int) []string {
	var violations []string
	for tree, got := range counts {
		budget, ok := budgets[tree]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"[%s] %s: %d LOC, but the tree has no budget entry.\n"+
					"  A new legacy tree. That is almost always the wrong direction; if a move is genuinely necessary, add the entry with a justification.",
				moduleLegacyBudgetCheck, tree, got))
		case got > budget:
			violations = append(violations, fmt.Sprintf(
				"[%s] %s: %d LOC, budget %d.\n"+
					"  The legacy tree grew. Moving code into a legacy directory is not progress; never raise a number.",
				moduleLegacyBudgetCheck, tree, got, budget))
		case got < budget:
			violations = append(violations, fmt.Sprintf(
				"[%s] %s: %d LOC, budget %d.\n"+
					"  Nice — lower the budget to %d so the reduction cannot regress.",
				moduleLegacyBudgetCheck, tree, got, budget, got))
		}
	}
	for tree, budget := range budgets {
		if _, ok := counts[tree]; !ok {
			violations = append(violations, fmt.Sprintf(
				"[%s] %s: budget %d LOC, but the tree holds no production .go file any more.\n"+
					"  Tree dissolved — remove the entry.",
				moduleLegacyBudgetCheck, tree, budget))
		}
	}
	return violations
}

// moduleLegacyBudgetTotalViolations checks the measured sum against
// moduleLegacyBudgetTotal. Per-tree budgets alone accept LOC shifted from one
// legacy tree into another as long as both entries are updated; the total does
// not.
func moduleLegacyBudgetTotalViolations(counts map[string]int) []string {
	total := 0
	for _, got := range counts {
		total += got
	}
	switch {
	case total > moduleLegacyBudgetTotal:
		return []string{fmt.Sprintf(
			"[%s] total: %d LOC across %d tree(s), moduleLegacyBudgetTotal %d.\n"+
				"  The sum of all legacy trees grew. Shifting LOC from one tree into another is not a reduction; never raise it.",
			moduleLegacyBudgetCheck, total, len(counts), moduleLegacyBudgetTotal)}
	case total < moduleLegacyBudgetTotal:
		return []string{fmt.Sprintf(
			"[%s] total: %d LOC across %d tree(s), moduleLegacyBudgetTotal %d.\n"+
				"  Nice — lower moduleLegacyBudgetTotal to %d so the reduction cannot regress.",
			moduleLegacyBudgetCheck, total, len(counts), moduleLegacyBudgetTotal, total)}
	}
	return nil
}

// moduleLegacyBudgetScan returns the production LOC of every legacy tree,
// keyed by its repo-relative slash path. Trees holding only *_test.go files
// are omitted, so their budget entry surfaces as "dissolved, remove it".
func moduleLegacyBudgetScan(backendRoot string) (map[string]int, error) {
	roots, err := moduleLegacyBudgetRoots(backendRoot)
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(roots))
	for _, tree := range roots {
		loc, locErr := moduleLegacyBudgetTreeLOC(filepath.Join(backendRoot, filepath.FromSlash(tree)))
		if locErr != nil {
			return nil, locErr
		}
		if loc > 0 {
			counts[tree] = loc
		}
	}
	return counts, nil
}

// moduleLegacyBudgetRoots lists the topmost directories named "legacy" below
// modules/ and workflows/. Descent stops at a hit, so a legacy directory
// nested inside a legacy tree is counted with its parent instead of forming a
// second entry.
func moduleLegacyBudgetRoots(backendRoot string) ([]string, error) {
	var roots []string
	for _, top := range []string{"modules", "workflows"} {
		err := filepath.WalkDir(filepath.Join(backendRoot, top), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				return nil
			}
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			if d.Name() != "legacy" {
				return nil
			}
			rel, relErr := filepath.Rel(backendRoot, path)
			if relErr != nil {
				return relErr
			}
			roots = append(roots, filepath.ToSlash(rel))
			return filepath.SkipDir
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(roots)
	return roots, nil
}

// moduleLegacyBudgetTreeLOC sums the lines of every non-test .go file below
// root. Lines, not statements: the budget measures how much code still has to
// move out, and a comment-heavy file is no cheaper to migrate.
func moduleLegacyBudgetTreeLOC(root string) (int, error) {
	total := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		lines, countErr := moduleLegacyBudgetCountLines(path)
		if countErr != nil {
			return countErr
		}
		total += lines
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}

func moduleLegacyBudgetCountLines(path string) (int, error) {
	f, err := os.Open(path) // #nosec G304 -- test scans repo-local source files
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	lines := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		lines++
	}
	return lines, scanner.Err()
}

// moduleLOCBudgetViolations compares measured LOC per key with a register of
// the two #4071 measurements, with the four outcomes of the legacy trees.
func moduleLOCBudgetViolations(check string, counts, budgets map[string]int) []string {
	var violations []string
	for key, got := range counts {
		budget, ok := budgets[key]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"[%s] %s: %d LOC, but it has no budget entry.\n"+
					"  Code moved here from a counted place keeps counting; a new entry needs a justification and the total may not rise.",
				check, key, got))
		case got > budget:
			violations = append(violations, fmt.Sprintf(
				"[%s] %s: %d LOC, budget %d.\n"+
					"  It grew. Never raise a number; a raise needs an ADR (ADR 0057 point 5).",
				check, key, got, budget))
		case got < budget:
			violations = append(violations, fmt.Sprintf(
				"[%s] %s: %d LOC, budget %d.\n"+
					"  Nice — lower the budget to %d so the reduction cannot regress.",
				check, key, got, budget, got))
		}
	}
	for key, budget := range budgets {
		if _, ok := counts[key]; !ok {
			violations = append(violations, fmt.Sprintf(
				"[%s] %s: budget %d LOC, but it holds no production .go code any more.\n"+
					"  Remove the entry.",
				check, key, budget))
		}
	}
	return violations
}

// moduleLOCBudgetTotalViolations checks the measured sum against a register's
// total, so LOC shifted between two entries shows up as well.
func moduleLOCBudgetTotalViolations(check, name string, counts map[string]int, budgetTotal int) []string {
	total := 0
	for _, got := range counts {
		total += got
	}
	switch {
	case total > budgetTotal:
		return []string{fmt.Sprintf(
			"[%s] total: %d LOC across %d entries, %s %d.\n"+
				"  The sum grew. Shifting LOC from one entry into another is not a reduction; never raise it.",
			check, total, len(counts), name, budgetTotal)}
	case total < budgetTotal:
		return []string{fmt.Sprintf(
			"[%s] total: %d LOC across %d entries, %s %d.\n"+
				"  Nice — lower %s to %d so the reduction cannot regress.",
			check, total, len(counts), name, budgetTotal, name, total)}
	}
	return nil
}

// moduleLegacyFileScan returns the LOC of every non-test legacy_*.go file in
// the backend outside a directory named legacy, keyed by its backend-relative
// path. Test helpers (legacy_*_test_helpers.go) compile into production and
// count.
func moduleLegacyFileScan(backendRoot string) (map[string]int, error) {
	counts := map[string]int{}
	err := filepath.WalkDir(backendRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != backendRoot && (strings.HasPrefix(name, ".") || name == "legacy" || name == "testdata" || name == "vendor" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasPrefix(name, "legacy_") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(backendRoot, path)
		if relErr != nil {
			return relErr
		}
		lines, countErr := moduleLegacyBudgetCountLines(path)
		if countErr != nil {
			return countErr
		}
		counts[filepath.ToSlash(rel)] = lines
		return nil
	})
	return counts, err
}

// moduleComposeScan returns the production LOC of every owner composition
// package below modules/ and workflows/, keyed by its directory. A package is a
// composition when a path segment below the module directory is compose or
// composition; legacy trees are counted by moduleLegacyBudgets instead.
func moduleComposeScan(backendRoot string) (map[string]int, error) {
	counts := map[string]int{}
	for _, top := range []string{"modules", "workflows"} {
		err := filepath.WalkDir(filepath.Join(backendRoot, top), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if strings.HasPrefix(d.Name(), ".") || d.Name() == "legacy" || d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, relErr := filepath.Rel(backendRoot, filepath.Dir(path))
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			if !moduleComposePackage(rel) {
				return nil
			}
			lines, countErr := moduleLegacyBudgetCountLines(path)
			if countErr != nil {
				return countErr
			}
			counts[rel] += lines
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return counts, nil
}

func moduleComposePackage(dir string) bool {
	parts := strings.Split(dir, "/")
	for index, part := range parts {
		if index >= 2 && (part == "compose" || part == "composition") {
			return true
		}
	}
	return false
}
