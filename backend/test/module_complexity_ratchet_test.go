package test

import (
	"fmt"
	"go/ast"
	"sort"
	"strings"
	"testing"

	"github.com/uudashr/gocognit"
)

// TestModuleComplexityRatchet extends .claude/rules/backend-conventions.md
// Rule 4 — gocognit cognitive complexity ≤ 15 — from api/ to the capability
// modules, application workflows and complete roots (ADR 0082).
//
// Why a second ratchet instead of a wider api/ one: migration #2580 moved
// 228,306 of 638,717 production LOC into modules/ and workflows/, and the
// api/ ratchet stopped at its directory boundary. Moved code left the
// ratchet's reach without a single line changing. This
// test closes that gap by freezing today's state under the new roots.
//
// TestHandlerComplexityRatchet keeps the default production api/ build at
// zero. This ratchet includes test hooks too and scores the module handlers
// (modules/*/http, modules/*/inbound, workflows/*/http) against the same
// threshold; they have no register of their own.
//
// Allowlist semantics (per function, not per file), identical to the api/
// ratchet:
//
//   - A function NOT in the allowlist must score ≤ 15.
//   - An allowlisted function may never exceed its recorded score.
//   - When refactoring lowers a score (or drops it to ≤ 15), the test fails
//     until the entry is lowered/removed — the ratchet only turns one way.
//     Additions or raises require a new-numbered ADR cited by the entry
//     (ADR 0070).
//
// Keys are "relative/file.go:FuncName" with methods rendered exactly like the
// gocognit CLI ("(*service).ListOptions"), so CLI output maps 1:1 onto
// entries here. walkModuleRatchetSources scans modules/, workflows/, roots/,
// api/, cmd/ recursively and direct backend-root non-test .go files. Legacy
// subdirectories stay counted so a move cannot silence the gate.
const moduleComplexityThreshold = 15

// Seeded 2026-09-18 at merge commit ecf0003369, the state of modules/ and
// workflows/ right after the #2580 move, measured with:
//
//	cd backend && ../scripts/run-go-toolchain.sh \
//	    go run github.com/uudashr/gocognit/cmd/gocognit -over 15 ./modules ./workflows
//
// (_test.go hits filtered out). 444 functions. The first seed was taken at
// 19feca2822 and re-measured here after the merge of origin/development
// (a47f77b2c3): PR #3408 (issue #3350) moved eleven functions over the limit
// into modules/, which the ratchet cannot absorb without a fresh measurement.
// The entries record the debt as it was on that day; every one of them may
// only shrink or disappear.
//
// Re-measured on 2026-10-19 at 148e5aeb86 for the gocognit 1.2.1 → 1.3.0
// upgrade (ADR 0072). The new version scores else nesting, parenthesized
// logical operators and recursion differently; no code changed. Thirteen
// entries rose, three functions entered at 16, and two entries fell: the
// register's sum went from 5,460 to 5,527 over 228 entries. This is a tool
// rebaseline, not new debt; ADR 0072 records why the functions were not
// refactored instead. The module register drained; ADR 0082 adds the root
// baseline without headroom, measured at 722c5aa147.
var moduleComplexityAllowlist = map[string]int{
	"api/active_test_helpers.go:buildActiveTestModule":                                                        31,
	"api/auth_test_helpers.go:NewAuthTestModule":                                                              25,
	"api/enrollment_booking_repository_test_helpers.go:(EnrollmentBookingFixture).InsertRequestChildOffering": 18,
	"api/enrollment_booking_repository_test_helpers.go:(EnrollmentBookingFixture).bookingCommandValues":       22,
	"api/student_owner_http_test_helpers.go:NewStudentHTTPTestResources":                                      19,
	"api/student_test_helpers.go:newStudentTestModule":                                                        32,
	"api/timetable_test_helpers.go:NewTimetableTestModule":                                                    27,
	"cmd/devtools/demo.go:provisionDemoSchool":                                                                18,
	"cmd/devtools/demo_runtime.go:NewDemoRuntime":                                                             16,
	"cmd/devtools/demo_scheduler.go:(*demoScheduler).drain":                                                   20,
	"cmd/devtools/demo_scheduler.go:runDemoSchools":                                                           16,
	"cmd/serve.go:scrubSentryEvent":                                                                           20,
	"cmd/serve.go:validateServeConfig":                                                                        18,
}

func TestModuleComplexityRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	scores, err := moduleComplexityScan(backendRoot)
	if err != nil {
		t.Fatalf("complexity scan failed: %v", err)
	}

	violations := moduleComplexityViolations(scores, moduleComplexityAllowlist)
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Module-complexity ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// moduleComplexityViolations compares per-function scores over the threshold
// against the allowlist. Like its api/ counterpart it cannot reuse
// ratchetViolations: entries here are capped scores, not hit counts, and an
// allowlisted function dropping to or below the threshold disappears from the
// scan entirely (which is the "ratchet down" signal, not a stale-entry error
// in the per-file sense).
func moduleComplexityViolations(scores, allowlist map[string]int) []string {
	var violations []string
	for fn, got := range scores {
		allowed, ok := allowlist[fn]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s scores %d (limit %d), and is not in the allowlist.\n  Move branching/orchestration behind a capability method (Rule 4) — do not add new entries.",
				fn, got, moduleComplexityThreshold))
		case got > allowed:
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s scores %d, allowed %d.\n  Complexity grew. Simplify the function — never raise the allowlist.", fn, got, allowed))
		case got < allowed:
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s scores %d, allowlist says %d.\n  Nice — ratchet the entry down to %d so the improvement cannot regress.", fn, got, allowed, got))
		}
	}
	for fn, allowed := range allowlist {
		if _, ok := scores[fn]; !ok {
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s is allowlisted at %d but now scores ≤ %d (or was deleted/renamed).\n  Remove the entry.",
				fn, allowed, moduleComplexityThreshold))
		}
	}
	return violations
}

// All roots retain coverage and thresholds after a move (ADRs 0064, 0082).
// moduleComplexityScan returns gocognit scores above the threshold for every
// function in the shared module/root scope, keyed
// "relpath:FuncName" with methods rendered like the gocognit CLI
// ("(*service).name"). It shares funcDisplayName with the api/ ratchet so
// both registers speak the same key language.
func moduleComplexityScan(backendRoot string) (map[string]int, error) {
	scores := make(map[string]int)
	err := walkModuleRatchetSources(backendRoot, func(path, rel string) error {
		_, file, parseErr := parseGoSourceCached(path, nil)
		if parseErr != nil {
			return fmt.Errorf("parse %s: %w", rel, parseErr)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if score := gocognit.Complexity(fn); score > moduleComplexityThreshold {
				scores[rel+":"+funcDisplayName(fn)] = score
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return scores, nil
}
