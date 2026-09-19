package test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestModulePassthroughRatchet enforces .claude/rules/backend-conventions.md
// Rule 8: "A service method that does nothing but call one repository method
// is NOT a service method." Rule 8 has been written down since the legacy
// conventions were first recorded, but it never had a check — the handler,
// service-query, model-ceremony and query-budget ratchets all stop at the
// api/, services/ and models/ directory boundaries, so nothing looked at the
// application layer the #2580 migration created under modules/ and
// workflows/. This ratchet is the missing gate: it does not repair anything
// today, it freezes today's number per module and lets it run down only.
//
// What counts as a pass-through (the four AST conditions, verbatim — keep
// them intact, the seed below is only reproducible against exactly these):
//
//  1. *ast.FuncDecl with a receiver (a method, not a free function).
//  2. Exported name (first letter upper-case).
//  3. Body line span (Rbrace.Line - Lbrace.Line + 1) at most 14.
//  4. The body contains EXACTLY ONE *ast.CallExpr whose Fun is a selector
//     chain of the form <receiver>.<field>.<method>(...), that is, a call
//     through a FIELD of the receiver. Direct calls to sister methods
//     (<receiver>.<method>(...)) do NOT count, because they reach neither a
//     repository nor a port. Free function calls (domain.X, fmt.Errorf, len)
//     count neither for nor against. The one field call may sit at any
//     nesting depth, including inside a function literal handed to a wrapper.
//     Chain length: the selector needs at least two steps off the receiver,
//     and LONGER chains count too — s.deps.Devices.Find(...) is one field
//     call, not none. Reading condition 4 as exactly three links gives 692
//     hits in 20 modules instead of the 704 in 21 seeded below; the 12
//     difference is almost all modules/devicefleet device.go.
//
// Condition 4 is what "call one repository method" means at the AST level:
// the receiver's own fields are its collaborators — the store, the ports, the
// clock — so a call through a field is the only call that can leave the
// module. Everything else in the body is either ceremony (error wrapping,
// length measurement, a domain function) or a call to a sibling method of the
// same type, which is the module talking to itself.
//
// Two earlier drafts were measured over the same tree and rejected, both for
// documented reasons:
//
//   - "exactly ONE *ast.CallExpr in the whole body" found 61 hits in 10
//     modules. Every method that also calls fmt.Errorf or len dropped out.
//   - "exactly ONE receiver-rooted call" (any chain bottoming out in the
//     receiver, s.helper(...) included) found 162 hits in 14 modules, and it
//     measured the wrapper style rather than the depth. 45 of those 162 (28%)
//     called no field at all, only a sister method, so they forwarded to
//     nothing — modules/devicefleet IsDeviceOnlineAt, whose body IS the
//     staleness rule (#586, Rule 12), was counted as debt. At the same time
//     modules/timetable scored 0 although it has the widest application layer,
//     because its store calls sit inside s.run("…", func(stats) { …
//     s.store.X(…) … }) — two receiver-rooted calls, hence "orchestration".
//     modules/careplan writes the same wrapper as the free function
//     runValue(s, "…", func() { … s.store.X(…) … }) and scored 50. Same
//     domain, same design, two spellings, 50 against 0.
//
// Counting field calls removes both errors with one rule: the 45 sister-only
// methods have zero field calls and are out, and timetable's wrapped store
// calls are one field call each and are in (127 of them).
//
// Scanned tree: non-test .go files under modules/*/internal/application/**
// and workflows/*/internal/application/**, including nested module layouts
// such as modules/careplan/requestfeed/internal/application. legacy/ is NOT
// scanned here — it has its own migration track, and the application layer is
// the surface the module boundary is supposed to make deep.
//
// Form: a budget per module, like test/query_budgets.go, not an allowlist per
// method. The key is the module directory relative to the backend root
// ("modules/identityaccess", "workflows/reminderdelivery"); the value is the
// number of pass-throughs measured in it.
//
//   - Over budget  → new delegation was added. Rule 8: a service method
//     encapsulates (validation, orchestration, audit, tenant transaction,
//     error mapping) or it does not exist. Never raise a budget.
//   - Under budget → the win is real; lower the entry so it cannot regress.
//   - Module with hits but no budget → a new application package; add its
//     entry with today's count and shrink from there.
//   - Budget with no hits → the entry is stale; remove it.
//
// Seed: measured 2026-09-18 at commit 19feca2822 by the scan below, over 203
// non-test files: 704 pass-throughs in 21 modules. Re-measure with
//
//	cd backend && ../scripts/run-go-toolchain.sh go test ./test/ \
//	    -run TestModulePassthroughRatchet -v
//
// A red run prints the measured count for every module, so the register can
// always be rebuilt from the failure output. Numbers only ever go down.
//
// Known limitations — conditions 1-4 describe a shape, not an intent. Five
// gaps survive this version and are recorded rather than patched, because a
// literal shape is what keeps the seed reproducible:
//
//  1. A method that hands its work to a pure domain function makes no field
//     call at all and is invisible here. modules/schoolcalendar
//     ValidHolidayRegion (service.go:195, `return
//     domain.ValidHolidayRegion(region)`) is the plainest forwarder in the
//     tree and is not counted; the same holds for its ListHolidays,
//     HolidayDates, RenderCalendar and RenderCalendarObject neighbours, whose
//     only work is a domain call inside s.run.
//  2. The 14-line limit cuts through a continuous population. 93 exported
//     methods have a body of 15 to 18 lines and exactly one field call, i.e.
//     they are the same shape that counts below and miss only on length.
//     Measured spans of the four examples: modules/organizationtenancy
//     school.go:107 FindSchoolByID 17, modules/peopledirectory service.go:59
//     FindByID 18, modules/schoolmembership class_list_entry.go:14
//     FindClassListEntry 18, modules/schoolmembership service.go:36 FindStaff
//  18. modules/timetable timeframes.go:9 FindTimeframe sits in the same
//     band at 15. Raising the limit would be raising a budget; it stays.
//  3. A module with an application layer but no budget entry turns the
//     ratchet red the moment its first hit appears, and the failure reads
//     like a regression rather than "this package was empty before". Three
//     such packages exist today: workflows/openroommove,
//     workflows/reminderdelivery and workflows/sessionend.
//  4. A field call can be hidden behind a local alias: rewriting
//     s.store.X(ctx) as x := s.store.X; x(ctx) lowers a budget without
//     removing a delegation. Three alias sites exist in the scanned tree
//     today (modules/peopledirectory student_directory.go:54,
//     student_enrollment.go:10, student_photo.go:246) and none of them
//     exploits the gap, so nothing is measured away right now. A budget that
//     drops without a matching deletion is worth a look at the diff.
//  5. A field is not necessarily a repository. The receiver's collaborators
//     also include the clock, the logger, the transaction runner and, on
//     error types, the wrapped cause — a call through any of them satisfies
//     condition 4. Measured over the 704: 600 go through `store`, 9 through
//     `records`, 95 through some other field, of which roughly 9 to 15 are
//     plainly not a repository call. The worst single case is
//     modules/identityaccess operator_mfa_flow.go:75 HasEnrollment, a
//     fail-closed MFA gate whose only field call is f.logger.Warn(...) —
//     deleting the log line would lower the identityaccess budget by one
//     without removing any delegation. Separating ports from infrastructure
//     would mean naming or typing them, which turns a reproducible AST shape
//     into a convention; that trade was declined.
const modulePassthroughMaxBodyLines = 14

// modulePassthroughApplicationSegment selects the scanned tree: every path
// under a module that reaches into its application layer, at any nesting
// depth.
const modulePassthroughApplicationSegment = "/internal/application/"

// modulePassthroughBudgets is the register: module directory → pass-throughs
// measured there. Sorted alphabetically so later diffs stay readable.
// Shrink-only — see the header.
var modulePassthroughBudgets = map[string]int{
	// Appointment, recipient, recurrence-rule and occurrence-override CRUD,
	// all of it one s.store call inside s.run; service.go alone.
	"modules/appointments": 29,
	// The widest register: student_schedules.go (45) and records.go (29)
	// forward one store call per schedule and record operation, plus
	// withdrawals (12), the care-exit/companion service surface (15) and the
	// student-deletion and excused-request entry points.
	"modules/careplan": 106,
	// The caller, report and arrival-exception ports reached straight
	// through, most of them behind a nil-port guard and a date parse.
	"modules/classday": 9,
	// Announcement and feed writes, each one store call behind the shared
	// mutateExisting/run helpers.
	"modules/communication": 6,
	// Email outbox and backlog reads/writes forwarded to the store, and the
	// worker's Backlog.
	"modules/delivery": 7,
	// device.go carries 12 (registration, last-seen, status, listings); the
	// display, online-window, service and unregistered-tag-scan files one each.
	"modules/devicefleet": 16,
	// Session lifecycle (6) plus the school-name lookup and the service
	// facade's device read.
	"modules/devicescan": 8,
	// The target-state read, which only maps the port error to ErrUnavailable.
	"modules/exporttransfer": 1,
	// Room and toilet reads/writes in service.go.
	"modules/facilities": 4,
	// Feedback submission and listing, one store call each.
	"modules/feedback": 6,
	// CountAttachments, the only attachment method short enough to qualify.
	"modules/filestorage": 1,
	// Spread over 32 files, led by operator_tokens.go (18), account_session.go
	// (12), operator.go (10) and operator_mfa.go (9): session/operator
	// lookups and their ForUpdate twins, MFA and passkey credential and
	// challenge stores, permission and role administration reads.
	"modules/identityaccess": 120,
	// Menu, participation and week reads forwarded to the store.
	"modules/mealplan": 8,
	// The provisioning listings (organisations, accounts, devices, dashboard)
	// and the school read/soft-delete/restore surface.
	"modules/organizationtenancy": 21,
	// student.go (15), service.go (13) and student_records.go (12) lead;
	// guardian links, enrollment, directory, deletion and photo reads follow,
	// each one observeRun around a single s.store call.
	"modules/peopledirectory": 62,
	// Calendar-period, closing-day and dateframe CRUD in service.go. The
	// holiday and iCal methods are NOT in this number — they call a domain
	// function, see known limitation 1.
	"modules/schoolcalendar": 15,
	// Membership service reads (17), teaching assignments (11) and the
	// class-list-entry surface (5).
	"modules/schoolmembership": 33,
	// School-year transition (9) plus the transition history and service reads.
	"modules/schoolstructure": 13,
	// Attendance, visit, supervision, group-mapping and room reads/writes
	// across 18 files, each one store call inside the operation wrapper.
	"modules/studentpresence": 64,
	// The largest block, and the one the previous version could not see at
	// all: service.go (23), instance_students.go (21), activity_instances.go
	// (14) and 19 more files, every one of them s.run around a single
	// s.store call.
	"modules/timetable": 127,
	// Work sessions (15), the service facade (7), absences, shifts, staff
	// records, month snapshots, offboarding and substitutions.
	"modules/workforce": 47,
}

func TestModulePassthroughRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	counts, err := modulePassthroughScan(backendRoot)
	if err != nil {
		t.Fatalf("module-passthrough scan failed: %v", err)
	}

	violations := modulePassthroughViolations(counts, modulePassthroughBudgets)
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Module-passthrough ratchet check failed (%d issue(s)):\n\n%s\n\n"+
			"These counts measure a shape (four AST conditions), not the worth of any single method —\n"+
			"a counted method may well be the right place for its logic. See \"Known limitations\" in\n"+
			"the header of test/module_passthrough_ratchet_test.go before acting on an individual hit.",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// modulePassthroughViolations compares the measured per-module counts against
// the register. It does not reuse ratchetViolations: entries here are module
// budgets, not per-file hit allowances, and the shared helper's wording ("the
// file is not in the allowlist") would point reviewers at a file that does not
// exist.
func modulePassthroughViolations(counts, budgets map[string]int) []string {
	var violations []string
	for module, got := range counts {
		budget, ok := budgets[module]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"[module passthrough] %s: %d single-call delegation(s), but the module has no budget.\n"+
					"  New application package — add the entry with today's count and shrink it from there (Rule 8).",
				module, got))
		case got > budget:
			violations = append(violations, fmt.Sprintf(
				"[module passthrough] %s: %d single-call delegation(s), budget %d.\n"+
					"  Delegations were added. A service method encapsulates (validation, orchestration, audit,\n"+
					"  tenant transaction, error mapping) or it does not exist (Rule 8). Never raise a budget.",
				module, got, budget))
		case got < budget:
			violations = append(violations, fmt.Sprintf(
				"[module passthrough] %s: %d single-call delegation(s), budget says %d.\n"+
					"  Nice — lower the budget to %d so the improvement cannot regress.",
				module, got, budget, got))
		}
	}
	for module, budget := range budgets {
		if _, ok := counts[module]; !ok {
			violations = append(violations, fmt.Sprintf(
				"[module passthrough] %s: budget %d, but not one delegation is left (cleaned up, moved or renamed?).\n"+
					"  Remove the entry.", module, budget))
		}
	}
	return violations
}

// modulePassthroughScan counts pass-through methods per module in the
// application layers of modules/ and workflows/. Modules without a single hit
// do not appear in the result, which is what makes a stale budget detectable.
func modulePassthroughScan(backendRoot string) (map[string]int, error) {
	counts := make(map[string]int)
	for _, tree := range []string{"modules", "workflows"} {
		root := filepath.Join(backendRoot, tree)
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if strings.HasPrefix(d.Name(), ".") && path != root {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, relErr := filepath.Rel(backendRoot, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			if !strings.Contains(rel, modulePassthroughApplicationSegment) {
				return nil
			}
			if strings.Contains(rel, "/legacy/") {
				return nil
			}
			module := modulePassthroughModuleKey(rel)
			if module == "" {
				return nil
			}

			fset := token.NewFileSet()
			file, parseErr := parser.ParseFile(fset, path, nil, 0)
			if parseErr != nil {
				return fmt.Errorf("parse %s: %w", rel, parseErr)
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				if modulePassthroughIsDelegation(fn, fset) {
					counts[module]++
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return counts, nil
}

// modulePassthroughModuleKey maps "modules/careplan/requestfeed/internal/
// application/x.go" to its owning module directory, "modules/careplan".
func modulePassthroughModuleKey(rel string) string {
	parts := strings.Split(rel, "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "/" + parts[1]
}

// modulePassthroughIsDelegation applies the four conditions from the header,
// in that order.
func modulePassthroughIsDelegation(fn *ast.FuncDecl, fset *token.FileSet) bool {
	// (1) a method, not a free function.
	if fn.Recv == nil || len(fn.Recv.List) == 0 || fn.Body == nil {
		return false
	}
	// (2) exported.
	if fn.Name == nil || !fn.Name.IsExported() {
		return false
	}
	// The receiver must be named; an unnamed or blank receiver cannot be the
	// root of the forwarded call.
	receiver := modulePassthroughReceiverName(fn)
	if receiver == "" {
		return false
	}
	// (3) body line span.
	span := fset.Position(fn.Body.Rbrace).Line - fset.Position(fn.Body.Lbrace).Line + 1
	if span > modulePassthroughMaxBodyLines {
		return false
	}
	// (4) exactly one call through a field of the receiver. Sister-method
	// calls (s.helper(...)) and free functions (fmt.Errorf, domain.X, len)
	// are ignored: neither leaves the module. A second field call means the
	// method combines two collaborators, which is orchestration.
	return modulePassthroughFieldCalls(fn.Body, receiver) == 1
}

// modulePassthroughFieldCalls counts the calls in body whose Fun is a selector
// chain of at least two steps bottoming out in the receiver identifier —
// s.store.Find(...), s.ports.Y(...), s.deps.Clock.Now(...). A one-step chain
// (s.Helper(...)) is a sister method and is not counted; so is every call that
// does not start at the receiver at all. Function literals are inspected as
// part of the body, so a store call wrapped in s.run("…", func(){…}) counts.
func modulePassthroughFieldCalls(body *ast.BlockStmt, receiver string) int {
	count := 0
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		// The method name is one step; at least one field must sit between it
		// and the receiver.
		field, ok := selector.X.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if root := modulePassthroughSelectorRoot(field); root != nil && root.Name == receiver {
			count++
		}
		return true
	})
	return count
}

// modulePassthroughReceiverName returns the receiver's identifier, or "" when
// the method has none (`func (*Service) X()`) or discards it (`func (_ *S) X()`).
func modulePassthroughReceiverName(fn *ast.FuncDecl) string {
	names := fn.Recv.List[0].Names
	if len(names) == 0 || names[0] == nil || names[0].Name == "_" {
		return ""
	}
	return names[0].Name
}

// modulePassthroughSelectorRoot peels a selector chain down to its innermost
// expression: r.store.Find → r, s.Helper → s. It returns nil when the chain
// does not bottom out in a plain identifier (a call, an index or a literal).
func modulePassthroughSelectorRoot(expr ast.Expr) *ast.Ident {
	for {
		selector, ok := expr.(*ast.SelectorExpr)
		if !ok {
			break
		}
		expr = selector.X
	}
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return nil
	}
	return ident
}
