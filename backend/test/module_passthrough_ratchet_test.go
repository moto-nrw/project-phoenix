package test

import (
	"fmt"
	"go/ast"
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
// The owner compositions are scanned too (#4071, ADR 0057 point 5): every
// non-test .go file below a directory named compose or composition under
// modules/ or workflows/, legacy/ excluded as above. A composition is where
// forwarding moved when the application layer shrank — identityaccess.Module
// alone forwards hundreds of methods — so a register that stopped at the
// application layer measured where code left, not where it went.
//
// Code #4084 moved into other owner packages stays scanned (ADR 0064):
// modulePassthroughMovedTrees lists them, keyed to their module like the two
// trees above.
//
// Since ADR 0061 (#4084) one register counts both trees per module:
// modulePassthroughOwnerBudgets replaces the application register
// modulePassthroughBudgets and the composition register
// modulePassthroughComposeBudgets, and modulePassthroughOwnerBudgetTotal
// replaces their totals modulePassthroughBudgetTotal and
// modulePassthroughComposeBudgetTotal. #4084 moves code from the compositions
// into the application layers; moving a method within one module must not
// count as growth, so both trees share one entry. A method moved to another
// module still raises that module's entry.
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
// The register has a total (modulePassthroughOwnerBudgetTotal) that must
// equal the sum of its entries. Raising an entry therefore means raising the
// total as well, and a raise of either against the pull request's base fails
// `scripts/backend-architecture.sh check` (ratchet budgets, #4071); a raise
// needs an ADR added in the same change (ADR 0057 point 5). The application
// register's total was 718 when the total was introduced; it had grown from
// the 704 seeded below through the four raises recorded in the entries.
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

// modulePassthroughMovedTrees are owner packages outside the application
// layers and the compositions that #4084 moved composition code into (ADR
// 0064). A method that leaves a counted tree keeps being counted where it
// lands, so a move cannot lower a budget: the Workforce Dienstplan rows went
// into internal/planning (#4240), and the root-facing seam and record types
// into the owners' ports packages (#4174, #4182, #4210, #4227, #4236, #4238).
var modulePassthroughMovedTrees = []struct{ path, owner string }{
	{"modules/schoolstructure/internal/adapters/postgres/runtime/", "modules/schoolstructure"},
	{"modules/peopledirectory/internal/adapters/postgres/runtime/", "modules/peopledirectory"},
	// ADR 0064: consumer glue moved to the root, but its delegations stay
	// attributed to the consumer. These entries expand the scan, not allowances.
	{"api/device_scan_catalog.go", "modules/devicescan"},
	{"api/kiosk_timetable.go", "modules/devicescan"},
	{"modules/careplan/ports/", "modules/careplan"},
	{"modules/identityaccess/ports/", "modules/identityaccess"},
	{"modules/peopledirectory/ports/", "modules/peopledirectory"},
	{"modules/timetable/ports/", "modules/timetable"},
	{"modules/workforce/internal/planning/", "modules/workforce"},
}

// modulePassthroughApplicationSegment selects the scanned tree: every path
// under a module that reaches into its application layer, at any nesting
// depth.
const modulePassthroughApplicationSegment = "/internal/application/"

// modulePassthroughOwnerBudgetTotal is the sum of
// modulePassthroughOwnerBudgets: 3109 under ADR 0061, the 717 of
// modulePassthroughBudgetTotal and the 2392 of
// modulePassthroughComposeBudgetTotal. Shrink-only, like every entry; raised
// by ADR 0063 and by ADR 0064 (3,011 to 3,091).
const modulePassthroughOwnerBudgetTotal = 3017

// modulePassthroughOwnerBudgets is the register: module directory →
// pass-throughs measured in its application layer and its compositions
// together (ADR 0061). Each entry started as the sum of the module's entries
// in modulePassthroughBudgets and modulePassthroughComposeBudgets; the
// composition counts were first measured on 2026-10-15 on top of development
// c49b81c3a9 (2394 in 34 compositions). #4084 carries the compositions that
// still hold more than construction. Sorted alphabetically so later diffs stay
// readable. Shrink-only — an entry may fall or go, never rise, and no module
// gets a new entry.
var modulePassthroughOwnerBudgets = map[string]int{
	"modules/appointments":        65,
	"modules/auditlog":            1,
	"modules/careplan":            293,
	"modules/classday":            18,
	"modules/communication":       71,
	"modules/dataimport":          1,
	"modules/delivery":            15,
	"modules/devicefleet":         58,
	"modules/devicescan":          48,
	"modules/enrollment":          35,
	"modules/exporttransfer":      4,
	"modules/facilities":          18,
	"modules/feedback":            14,
	"modules/filestorage":         27,
	"modules/identityaccess":      587,
	"modules/mealplan":            15,
	"modules/organizationtenancy": 60,
	"modules/peopledirectory":     307,
	"modules/requestreview":       17,
	"modules/schedulerruntime":    4,
	"modules/schoolcalendar":      44,
	"modules/schoolmembership":    81,
	"modules/schoolsetup":         2,
	"modules/schoolstructure":     70,
	"modules/settings":            19,
	"modules/studentpresence":     240,
	"modules/timetable":           488,
	"modules/workforce":           392,
	"workflows/openroommove":      1,
	"workflows/parentportal":      19,
	"workflows/shiftplansync":     3,
}

func TestModulePassthroughRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	counts, err := modulePassthroughScan(backendRoot, modulePassthroughOwnerKey)
	if err != nil {
		t.Fatalf("module-passthrough scan failed: %v", err)
	}

	violations := modulePassthroughViolations(counts, modulePassthroughOwnerBudgets)
	violations = append(violations, modulePassthroughTotalViolations("modulePassthroughOwnerBudgetTotal", modulePassthroughOwnerBudgets, modulePassthroughOwnerBudgetTotal)...)
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
					"  New application package or composition — a new application package gets an entry with today's count\n"+
					"  and shrinks from there (Rule 8); code moved out of a counted tree keeps its delegations counted.",
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

// modulePassthroughTotalViolations pins a register's total to the sum of its
// entries, so that raising an entry shows up as a raised constant as well.
func modulePassthroughTotalViolations(name string, budgets map[string]int, total int) []string {
	sum := 0
	for _, budget := range budgets {
		sum += budget
	}
	switch {
	case sum > total:
		return []string{fmt.Sprintf(
			"[module passthrough] %s: the entries sum to %d, the total is %d.\n"+
				"  An entry was raised. Never raise a budget; a raise needs an ADR (ADR 0057 point 5).",
			name, sum, total)}
	case sum < total:
		return []string{fmt.Sprintf(
			"[module passthrough] %s: the entries sum to %d, the total is %d.\n"+
				"  Nice — lower %s to %d so the reduction cannot regress.",
			name, sum, total, name, sum)}
	}
	return nil
}

// modulePassthroughScan counts pass-through methods per register key in
// modules/ and workflows/. key maps a backend-relative file path to its
// register entry, or "" when the file is outside the register's tree. Keys
// without a single hit do not appear in the result, which is what makes a
// stale budget detectable.
func modulePassthroughScan(backendRoot string, key func(string) string) (map[string]int, error) {
	counts := make(map[string]int)
	for _, tree := range []string{"modules", "workflows", "api"} {
		root := filepath.Join(backendRoot, tree)
		// Synthetic module/workflow fixtures need not contain a root package.
		if tree == "api" {
			if _, err := os.Stat(root); os.IsNotExist(err) {
				continue
			}
		}
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
			if strings.Contains(rel, "/legacy/") {
				return nil
			}
			module := key(rel)
			if module == "" {
				return nil
			}

			fset, file, parseErr := parseGoSourceCached(path, nil)
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

// modulePassthroughOwnerKey maps a file in an application layer or below an
// owner composition to its module directory, and every other file to "":
// "modules/careplan/requestfeed/internal/application/x.go" and
// "modules/careplan/requestfeed/compose/x.go" to "modules/careplan",
// "modules/communication/composition/x.go" to "modules/communication". A
// composition is a directory named compose or composition below the module
// directory (ADR 0061).
func modulePassthroughOwnerKey(rel string) string {
	for _, tree := range modulePassthroughMovedTrees {
		if strings.HasPrefix(rel, tree.path) {
			return tree.owner
		}
	}
	parts := strings.Split(rel, "/")
	if len(parts) < 3 {
		return ""
	}
	if strings.Contains(rel, modulePassthroughApplicationSegment) {
		return parts[0] + "/" + parts[1]
	}
	for index, part := range parts[:len(parts)-1] {
		if index >= 2 && (part == "compose" || part == "composition") {
			return parts[0] + "/" + parts[1]
		}
	}
	return ""
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
