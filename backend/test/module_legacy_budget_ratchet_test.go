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
// legacy SQL test providers (timetablesqltest).
// Re-measure the same way when a number needs to move — downwards.
const moduleLegacyBudgetCheck = "legacy LOC budget"

// moduleLegacyBudgetTotal is the sum of every entry below, measured with the
// same run. It catches LOC moved between two legacy trees, which leaves the
// individual budgets looking fine. Shrink-only, like every entry.
const moduleLegacyBudgetTotal = 24828

// moduleLegacyBudgets maps a legacy tree to its production LOC on 2026-09-18.
// The comment on each entry names the ticket that is supposed to dissolve the
// tree; "no ticket today" is the measurement, not an omission — those trees
// have no owner and no plan, which is precisely what this ratchet exposes.
var moduleLegacyBudgets = map[string]int{
	// No ticket today.
	"modules/devicefleet/compose/legacy": 231,
	// No ticket today.
	"modules/emergencysnapshot/legacy": 319,
	// No ticket today.
	"modules/facilities/compose/legacy": 518,
	// No ticket today. Two lines under the first seed: #3427 removed the
	// carelifecycle import PR #3408 (#3350) had added to legacy.go, and #3501
	// replaced the user-context service with a local CallerContext port.
	"modules/grouplive/legacy": 581,
	// #3226 (auth/jwt + repositories move). The usercontext read side (#2725)
	// dissolved into the Identity & Access caller context with #3501; its
	// request memo slot stayed in the session adapter (legacy/jwt).
	"modules/identityaccess/legacy": 1363,
	// No ticket today.
	"modules/planexport/legacy": 337,
	// No ticket today.
	"modules/supervisiondashboard/legacy": 721,
	// No ticket today — and the largest tree of the twelve.
	"modules/timetable/legacy": 4701,
	// No ticket today — grew from 15,402 LOC at creation to this.
	"modules/workforce/legacy": 16057,
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
