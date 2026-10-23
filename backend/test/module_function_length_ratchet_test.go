package test

import (
	"fmt"
	"go/ast"
	"sort"
	"strings"
	"testing"
)

// TestModuleFunctionLengthRatchet caps function body length in the modules
// that migration #2580 carved out and in every root (ADR 0082).
//
// Why a length gate: #2580 moved 228,306 of 638,717 production LOC into
// backend/modules/ and backend/workflows/, where no gate in this package
// measured function length, so the code that grew fastest was exactly the
// code nothing measured. A 300-line service
// method is the shape .claude/rules/backend-conventions.md keeps legislating
// against from three directions — Rule 4 (thin handlers), Rule 8 (services
// encapsulate, not delegate) and Rule 12 (models hold data, not decisions) —
// without any of them counting lines. This ratchet counts lines, which is the
// crude proxy that catches the cases cognitive complexity misses: long, flat,
// branch-free orchestration that should be three named steps.
//
// The threshold is 60 body lines, measured from the opening brace line to the
// closing brace line inclusive, on named declarations only. Function literals
// are not counted: a long literal is a property of its enclosing declaration,
// which the ratchet already measures.
//
// Allowlist semantics (per function, not per file — four cases):
//
//   - A function NOT in the allowlist must have a body of ≤ 60 lines.
//   - An allowlisted function may never exceed its recorded length.
//   - When a split shortens a function, the test fails until the entry is
//     lowered — the ratchet only turns one way. Never raise a number.
//   - When a function drops to ≤ 60 lines (or is deleted or renamed) it
//     disappears from the scan and its entry must be removed.
//
// Keys are "relative/file.go:FuncName" with methods rendered the way the
// gocognit CLI renders them ("(*service).ListOptions"), so entries here line
// up with the neighbouring complexity ratchet's keys.
//
// Scope: walkModuleRatchetSources covers modules/, workflows/, roots/, api/,
// cmd/ recursively and direct backend-root .go files, including the
// legacy/ subtrees. Those are counted deliberately — the retained legacy code
// is the larger half of the problem, and exempting it would let a migration
// move a 200-line method into legacy/ to silence the gate.
//
// Seed: measured 2026-09-18 at merge commit ecf0003369 with a throwaway
// go/ast walk over modules/ and workflows/ (named FuncDecls with a body,
// non-test files), recording fn.Body.Rbrace.Line - fn.Body.Lbrace.Line + 1
// per function:
//
//	cd backend && ../scripts/run-go-toolchain.sh go run ./tmp/moduleFuncLen
//
// The first seed was taken at 19feca2822 and re-measured here after the merge
// of origin/development (a47f77b2c3): PR #3408 (issue #3350) moved seven
// over-threshold functions into modules/, which the ratchet cannot absorb
// without a fresh measurement.
//
// 299 of 14,623 functions were over the threshold at seed time (483 over 50
// lines, 83 over 100, 5 over 200). The seed freezes that state; it is not a
// statement that these 299 functions are acceptable.
const moduleFuncLenThreshold = 60

// Seeded 2026-09-18 from merge commit ecf0003369; module entries drained. Shrink-only:
// never add an entry.
// ADR 0082: newly covered sources, measured at 722c5aa147; no headroom.
var moduleFuncLenAllowlist = map[string]int{
	"api/active_test_helpers.go:buildActiveTestModule":                                       185,
	"api/auth_test_helpers.go:NewAuthTestModule":                                             169,
	"api/calendar_test_helpers.go:NewCalendarTestModule":                                     63,
	"api/guardian_invitation_test_helpers.go:lifecycleTestModule":                            61,
	"api/guardians_composition.go:newGuardiansResource":                                      108,
	"api/import_test_helpers.go:NewImportTestModuleWithOwners":                               82,
	"api/iot/compose/compose.go:(*Resource).Router":                                          110,
	"api/reminders_test_helpers.go:NewRemindersTestModule":                                   64,
	"api/root_serve.go:composeServeRoot":                                                     511,
	"api/root_test_root.go:composeTestRootWith":                                              283,
	"api/serve_mounts_portals.go:newOperatorRouter":                                          65,
	"api/serve_mounts_students.go:mountStudentRoutes":                                        120,
	"api/serve_mounts_students.go:requestReviewDependencies":                                 100,
	"api/settings_composition.go:newSettingsResource":                                        61,
	"api/student_owner_http_test_helpers.go:NewStudentHTTPTestResources":                     176,
	"api/student_owner_http_test_helpers.go:newStudentHTTPTestReviews":                       103,
	"api/student_test_helpers.go:newStudentTestModule":                                       315,
	"api/timetable_http_test_helpers.go:NewTimetableHTTPTestModule":                          85,
	"api/timetable_http_test_helpers.go:newTimetableHTTPTestReads":                           78,
	"api/timetable_test_helpers.go:NewTimetableTestModule":                                   169,
	"api/workforce_test_helpers.go:NewWorkforceTestModule":                                   102,
	"cmd/devtools/command_adapter.go:(*Adapter).LoginOperator":                               65,
	"cmd/gendoc.go:getSettingsSchemas":                                                       79,
	"roots/owners/parent_messaging_announcement_access_port.go:BindParentAnnouncementAccess": 125,
	"roots/owners/root_identity.go:BuildIdentityAccess":                                      106,
	"roots/owners/root_worker.go:ComposeWorker":                                              358,
	"roots/owners/worker.go:NewWorker":                                                       91,
}

func TestModuleFunctionLengthRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	lengths, err := moduleFuncLenScan(backendRoot)
	if err != nil {
		t.Fatalf("function-length scan failed: %v", err)
	}

	violations := moduleFuncLenViolations(lengths, moduleFuncLenAllowlist)
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Module function-length ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// moduleFuncLenViolations compares per-function body lengths over the
// threshold against the allowlist. Like the complexity ratchet it cannot reuse
// ratchetViolations: entries are capped lengths rather than hit counts, and an
// allowlisted function falling to or below the threshold leaves the scan
// entirely, which is the ratchet-down signal rather than a deleted file.
func moduleFuncLenViolations(lengths, allowlist map[string]int) []string {
	var violations []string
	for fn, got := range lengths {
		allowed, ok := allowlist[fn]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"[func-length] %s has a %d-line body (limit %d), and is not in the allowlist.\n  Split the function into named steps — do not add new entries.",
				fn, got, moduleFuncLenThreshold))
		case got > allowed:
			violations = append(violations, fmt.Sprintf(
				"[func-length] %s has a %d-line body, allowed %d.\n  The function grew. Extract a step — never raise the allowlist.", fn, got, allowed))
		case got < allowed:
			violations = append(violations, fmt.Sprintf(
				"[func-length] %s has a %d-line body, allowlist says %d.\n  Nice — ratchet the entry down to %d so the improvement cannot regress.", fn, got, allowed, got))
		}
	}
	for fn, allowed := range allowlist {
		if _, ok := lengths[fn]; !ok {
			violations = append(violations, fmt.Sprintf(
				"[func-length] %s is allowlisted at %d lines but is now ≤ %d (or was deleted/renamed).\n  Remove the entry.",
				fn, allowed, moduleFuncLenThreshold))
		}
	}
	return violations
}

// All roots retain coverage and thresholds after a move (ADRs 0064, 0082).
// moduleFuncLenScan returns the body length of every named function over the
// threshold in the shared module/root scope, keyed
// "relpath:FuncName" with methods rendered like the gocognit CLI. Declarations
// without a body (assembly stubs, cgo) are skipped; function literals are not
// counted.
func moduleFuncLenScan(backendRoot string) (map[string]int, error) {
	lengths := make(map[string]int)
	err := walkModuleRatchetSources(backendRoot, func(path, rel string) error {
		fset, file, parseErr := parseGoSourceCached(path, nil)
		if parseErr != nil {
			return fmt.Errorf("parse %s: %w", rel, parseErr)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			length := fset.Position(fn.Body.Rbrace).Line - fset.Position(fn.Body.Lbrace).Line + 1
			if length > moduleFuncLenThreshold {
				lengths[rel+":"+moduleFuncLenDisplayName(fn)] = length
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return lengths, nil
}

// moduleFuncLenDisplayName renders a function name the way the gocognit CLI
// does: plain functions as "name", methods as "(recv).name" / "(*recv).name".
func moduleFuncLenDisplayName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return "(" + moduleFuncLenTypeExpr(fn.Recv.List[0].Type) + ")." + fn.Name.Name
}

func moduleFuncLenTypeExpr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + moduleFuncLenTypeExpr(e.X)
	case *ast.IndexExpr: // generic receiver
		return moduleFuncLenTypeExpr(e.X)
	case *ast.IndexListExpr:
		return moduleFuncLenTypeExpr(e.X)
	default:
		return fmt.Sprintf("%T", expr)
	}
}
