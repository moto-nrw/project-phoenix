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

// TestModuleFacadeWidthRatchet enforces the module contract of the #2580
// architecture migration: a module publishes small concrete facades with
// deliberately limited methods, never a generic CRUD API. An interface that
// keeps growing is the migration failure mode this ratchet exists to stop —
// the package boundary moved, but the god-service moved with it. The three
// widest facades at seed time were organizationtenancy.Provisioning with 33
// methods, careplan.StudentSchedulesCommand with 32, and
// peopledirectory.GuardianProvider with 31: a consumer that depends on one of
// them depends on everything the module can do.
//
// The rule: no exported interface in a module's or workflow's public package
// declares more than 12 methods.
//
// Scanned surface — the public face only. A package counts as a root package
// when it holds non-test .go files directly and neither it nor any directory
// above it (below modules/ or workflows/) is named internal, legacy, compose,
// http, inbound, adapters, domain, application or ports. Those are wiring,
// transport and implementation detail, not the facade, and the walk prunes
// them, so nested submodules with their own public package (careplan's
// requestfeed, schoolcalendar's portal) are found without being named here.
//
// Measurement (go/ast): every exported *ast.TypeSpec whose type is an
// *ast.InterfaceType; the width is the number of *ast.Field entries in
// Methods.List whose type is an *ast.FuncType. Embedded interfaces are
// deliberately NOT counted — they are *ast.Ident / *ast.SelectorExpr, not
// FuncType. The repository's Capability = Query + Command composition shape
// would otherwise be charged twice for methods the embedded facades already
// carry, and those embedded facades are the entries this ratchet holds. The
// consequence is known and accepted: a Capability that embeds two allowlisted
// halves scores 0 here, and the halves are where the width is ratcheted.
//
// Allowlist semantics (per interface, like handler_complexity_ratchet_test.go):
//
//   - An interface NOT in the allowlist must declare ≤ 12 methods.
//   - An allowlisted interface may never exceed its recorded width.
//   - When a split lowers a width (or drops it to ≤ 12), the test fails until
//     the entry is lowered/removed — the ratchet only turns one way. Never
//     raise a number, never add an entry.
//
// Keys are "relative/file.go:InterfaceName", paths starting at "modules/" or
// "workflows/".
const moduleFacadeThreshold = 12

// moduleFacadeNonRootDirs are directory names that are never a module's public
// face. The walk prunes them, so nothing below them is scanned either.
var moduleFacadeNonRootDirs = map[string]bool{
	"adapters":    true,
	"application": true,
	"compose":     true,
	"domain":      true,
	"http":        true,
	"inbound":     true,
	"internal":    true,
	"legacy":      true,
	"ports":       true,
}

// moduleFacadeAllowlist freezes the facades that were already too wide when
// the ratchet was introduced. Shrink-only.
//
// Seeded 2026-09-18 at commit 19feca2822 by walking the root packages under
// backend/modules/ and backend/workflows/ with go/ast, exactly as
// moduleFacadeScan below does. 36 entries out of 421 exported interfaces in
// those packages (mean width 4.37; 46 above 10 methods, 11 above 20).
// To re-derive a number, run the ratchet — every drift is reported with the
// width measured today:
//
//	cd backend && ../scripts/run-go-toolchain.sh go test ./test/ -run TestModuleFacadeWidthRatchet
var moduleFacadeAllowlist = map[string]int{
	"modules/appointments/appointments.go:Command":                               16,
	"modules/appointments/appointments.go:Query":                                 20,
	"modules/careplan/excusedrequests/contract.go:Service":                       18,
	"modules/careplan/records.go:CareRecordsCommand":                             14,
	"modules/careplan/records.go:CareRecordsQuery":                               15,
	"modules/careplan/requests.go:CareRequestsCommand":                           16,
	"modules/careplan/schedules.go:StudentSchedulesCommand":                      32,
	"modules/careplan/schedules.go:StudentSchedulesQuery":                        13,
	"modules/communication/parent_announcements.go:ParentAnnouncementCapability": 16,
	"modules/devicefleet/administration.go:Administration":                       22,
	"modules/devicefleet/devicefleet.go:DeviceQuery":                             13,
	"modules/identityaccess/account_authentication.go:AccountAuthentication":     14,
	"modules/identityaccess/operator_tokens.go:OperatorTokenCommand":             13,
	"modules/organizationtenancy/organizationtenancy.go:Query":                   20,
	"modules/organizationtenancy/provisioning.go:Provisioning":                   33,
	"modules/peopledirectory/guardian.go:GuardianCommand":                        16,
	"modules/peopledirectory/guardian.go:GuardianProvider":                       31,
	"modules/peopledirectory/guardian.go:GuardianQuery":                          19,
	"modules/peopledirectory/student_directory.go:StudentDirectoryQuery":         16,
	"modules/schoolcalendar/portal/contract.go:Service":                          21,
	"modules/schoolmembership/schoolmembership.go:Command":                       25,
	"modules/schoolmembership/schoolmembership.go:Query":                         17,
	"modules/studentpresence/studentpresence.go:Command":                         26,
	"modules/studentpresence/studentpresence.go:Query":                           17,
	"modules/timetable/instance_students.go:InstanceStudentCommand":              29,
	"modules/timetable/instance_students.go:InstanceStudentQuery":                13,
	"modules/timetable/timetable.go:Command":                                     17,
	"modules/timetable/timetable.go:Query":                                       22,
	"modules/workforce/shift.go:ShiftCommand":                                    19,
	"modules/workforce/staffadmin.go:StaffDirectory":                             13,
	"modules/workforce/staffadmin.go:StaffDocuments":                             16,
	"modules/workforce/staffrecord.go:StaffRecordCommand":                        13,
	"modules/workforce/timetracking.go:StaffAbsences":                            22,
	"modules/workforce/timetracking.go:WorkSessions":                             15,
	"modules/workforce/worksession.go:WorkSessionCommand":                        24,
	"modules/workforce/worksession.go:WorkSessionQuery":                          18,
}

func TestModuleFacadeWidthRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	widths, err := moduleFacadeScan(backendRoot)
	if err != nil {
		t.Fatalf("module-facade scan failed: %v", err)
	}

	violations := moduleFacadeViolations(widths, moduleFacadeAllowlist)
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Module-facade width ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// moduleFacadeViolations compares the measured widths above the threshold
// against the allowlist. Like complexityViolations, and unlike
// ratchetViolations, entries are capped values rather than hit counts: an
// allowlisted facade that shrinks to the threshold disappears from the scan
// entirely, which is the ratchet-down signal, not a deleted file.
func moduleFacadeViolations(widths, allowlist map[string]int) []string {
	var violations []string
	for iface, got := range widths {
		allowed, ok := allowlist[iface]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"[facade width] %s declares %d methods (limit %d), and is not in the allowlist.\n  Split the facade along a capability boundary — #2580 asks for small concrete facades, not generic CRUD APIs. Do not add new entries.",
				iface, got, moduleFacadeThreshold))
		case got > allowed:
			violations = append(violations, fmt.Sprintf(
				"[facade width] %s declares %d methods, allowed %d.\n  The facade grew. Put the new behaviour behind its own capability — never raise the allowlist.",
				iface, got, allowed))
		case got < allowed:
			violations = append(violations, fmt.Sprintf(
				"[facade width] %s declares %d methods, allowlist says %d.\n  Nice — ratchet the entry down to %d so the improvement cannot regress.",
				iface, got, allowed, got))
		}
	}
	for iface, allowed := range allowlist {
		if _, ok := widths[iface]; !ok {
			violations = append(violations, fmt.Sprintf(
				"[facade width] %s is allowlisted at %d but now declares ≤ %d methods (or was deleted/renamed).\n  Remove the entry.",
				iface, allowed, moduleFacadeThreshold))
		}
	}
	return violations
}

// moduleFacadeScan returns the width of every exported interface wider than
// the threshold in the root packages under modules/ and workflows/, keyed
// "relpath:InterfaceName".
func moduleFacadeScan(backendRoot string) (map[string]int, error) {
	widths := make(map[string]int)
	for _, tree := range []string{"modules", "workflows"} {
		treeRoot := filepath.Join(backendRoot, tree)
		err := filepath.WalkDir(treeRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if path == treeRoot {
					return nil
				}
				if moduleFacadeNonRootDirs[d.Name()] || strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if filepath.Dir(path) == treeRoot {
				// No package lives directly in modules/ or workflows/.
				return nil
			}
			rel, relErr := filepath.Rel(backendRoot, path)
			if relErr != nil {
				return relErr
			}
			return moduleFacadeMeasureFile(path, filepath.ToSlash(rel), widths)
		})
		if err != nil {
			return nil, err
		}
	}
	return widths, nil
}

// moduleFacadeMeasureFile records every exported interface in one file whose
// declared-method count exceeds the threshold.
func moduleFacadeMeasureFile(path, rel string, widths map[string]int) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0) // #nosec G304 -- test scans repo-local source files
	if err != nil {
		return fmt.Errorf("parse %s: %w", rel, err)
	}
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok || !spec.Name.IsExported() {
			return true
		}
		iface, ok := spec.Type.(*ast.InterfaceType)
		if !ok {
			return true
		}
		if methods := moduleFacadeMethodCount(iface); methods > moduleFacadeThreshold {
			widths[rel+":"+spec.Name.Name] = methods
		}
		return true
	})
	return nil
}

// moduleFacadeMethodCount counts declared methods. Embedded interfaces and
// type-set elements are not FuncType fields and are deliberately excluded —
// see the file header.
func moduleFacadeMethodCount(iface *ast.InterfaceType) int {
	if iface.Methods == nil {
		return 0
	}
	count := 0
	for _, field := range iface.Methods.List {
		if _, isMethod := field.Type.(*ast.FuncType); isMethod {
			count++
		}
	}
	return count
}
