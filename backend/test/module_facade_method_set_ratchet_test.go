package test

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestModuleFacadeMethodSetRatchet closes the blind spot of
// TestModuleFacadeWidthRatchet (ADR 0178, #6178 point 3). That ratchet counts
// only the methods an interface declares itself, so a composite interface
// that embeds other interfaces scores only its own few methods:
// identityaccess.Owner embeds 47 capabilities and passed with 3.
//
// Measurement (go/types, standard library only): every exported interface in
// the same root packages TestModuleFacadeWidthRatchet scans that embeds at least one other interface
// is measured by its complete method set (types.Interface.NumMethods). An
// interface without embeddings keeps its declared count in the other
// register, where its method set is the same number. No embedded facade is
// charged here: each composite is measured once, as the facade a consumer of
// that composite depends on, and the embedded facades keep their own entries.
//
// Allowlist semantics are those of TestModuleFacadeWidthRatchet: a composite
// above the threshold must be in moduleFacadeMethodSetAllowlist, an entry may
// never be exceeded, and a lower measurement fails until the entry is lowered
// or removed. Raises and additions need a new-numbered ADR (ADR 0057 point 5).
// To re-derive a number, run the ratchet; every drift reports the width
// measured today:
//
//	cd backend && ../scripts/run-go-toolchain.sh go test -tags=testhooks ./test/ -run TestModuleFacadeMethodSetRatchet
//
// Keys are "relative/file.go:InterfaceName" like the declared register.

// moduleFacadeMethodSetAllowlist freezes the composite facades whose method
// set was above moduleFacadeThreshold when ADR 0178 introduced the
// measurement. Seeded 2026-11-08 on development e20e8013b3: 108 composites,
// 4491 methods in their method sets together. Shrink-only.
var moduleFacadeMethodSetAllowlist = map[string]int{
	"modules/appointments/appointments.go:Capability":                                                 36,
	"modules/appointments/appointments.go:Command":                                                    16,
	"modules/appointments/appointments.go:Query":                                                      20,
	"modules/appointments/retention.go:Owner":                                                         38,
	"modules/careplan/arrival_schedules.go:ArrivalScheduleService":                                    38,
	"modules/careplan/booking_materialization.go:BookingMaterializationCapability":                    16,
	"modules/careplan/booking_materialization.go:CareOfferingCapability":                              55,
	"modules/careplan/care_documents.go:StudentDocuments":                                             14,
	"modules/careplan/care_lifecycle.go:CareLifecycle":                                                18,
	"modules/careplan/care_offering_catalog.go:CareOfferingCatalogCapability":                         15,
	"modules/careplan/careplan.go:Capability":                                                         158,
	"modules/careplan/careplan.go:Command":                                                            90,
	"modules/careplan/careplan.go:Owner":                                                              159,
	"modules/careplan/careplan.go:Query":                                                              62,
	"modules/careplan/excused_requests.go:ExcusedAbsenceRequests":                                     18,
	"modules/careplan/excusedrequests/contract.go:Service":                                            18,
	"modules/careplan/offering_changes.go:OfferingChangeCapability":                                   21,
	"modules/careplan/pickup_schedules.go:PickupScheduleService":                                      38,
	"modules/careplan/records.go:CareRecordsCommand":                                                  15,
	"modules/careplan/records.go:CareRecordsQuery":                                                    15,
	"modules/careplan/requests.go:CareRequestsCommand":                                                16,
	"modules/careplan/schedules.go:StudentSchedulesCommand":                                           28,
	"modules/careplan/schedules.go:StudentSchedulesQuery":                                             13,
	"modules/classday/classday.go:ClassDay":                                                           17,
	"modules/communication/communication.go:Capability":                                               13,
	"modules/communication/messages.go:ParentMessagingCapability":                                     13,
	"modules/communication/parent_announcements.go:ParentAnnouncementCapability":                      23,
	"modules/communication/portal_announcement_access.go:PortalAnnouncementAccess":                    23,
	"modules/communication/staff_announcement_records.go:StaffAnnouncementRecords":                    37,
	"modules/communication/staff_records.go:StaffConversationRecords":                                 13,
	"modules/communication/surveys.go:SurveyCapability":                                               17,
	"modules/communication/surveys.go:SurveyRecords":                                                  16,
	"modules/devicefleet/administration.go:Administration":                                            23,
	"modules/devicefleet/devicefleet.go:Capability":                                                   35,
	"modules/devicefleet/devicefleet.go:Command":                                                      16,
	"modules/devicefleet/devicefleet.go:DeviceQuery":                                                  14,
	"modules/devicefleet/devicefleet.go:Query":                                                        18,
	"modules/enrollment/decisions.go:Decisions":                                                       18,
	"modules/enrollment/enrollment.go:Owner":                                                          140,
	"modules/enrollment/enrollmenttest/careplan_contract.go:PublicDecisions":                          18,
	"modules/enrollment/holiday_registration_review.go:HolidayRegistrationReview":                     14,
	"modules/filestorage/filestorage.go:Capability":                                                   17,
	"modules/identityaccess/account_authentication.go:AccountAuthentication":                          18,
	"modules/identityaccess/account_lifecycle.go:AccountLifecycle":                                    22,
	"modules/identityaccess/account_mfa.go:AccountMFA":                                                29,
	"modules/identityaccess/identityaccess.go:Owner":                                                  262,
	"modules/identityaccess/role_administration.go:RoleAdministration":                                25,
	"modules/mealplan/mealplan.go:Capability":                                                         20,
	"modules/organizationtenancy/organizationtenancy.go:Capability":                                   40,
	"modules/organizationtenancy/organizationtenancy.go:Query":                                        26,
	"modules/organizationtenancy/provisioning.go:Provisioning":                                        47,
	"modules/parentbilling/care_months.go:CareBilling":                                                14,
	"modules/peopledirectory/guardian.go:GuardianCommand":                                             16,
	"modules/peopledirectory/guardian.go:GuardianProvider":                                            31,
	"modules/peopledirectory/guardian.go:GuardianQuery":                                               20,
	"modules/peopledirectory/peopledirectory.go:Capability":                                           143,
	"modules/peopledirectory/peopledirectory.go:Command":                                              78,
	"modules/peopledirectory/peopledirectory.go:Owner":                                                146,
	"modules/peopledirectory/peopledirectory.go:Query":                                                64,
	"modules/peopledirectory/student_directory.go:StudentDirectoryQuery":                              16,
	"modules/schoolcalendar/period_administration.go:Calendar":                                        32,
	"modules/schoolcalendar/portal/contract.go:FullService":                                           40,
	"modules/schoolcalendar/portal/contract.go:Service":                                               21,
	"modules/schoolcalendar/schoolcalendar.go:Capability":                                             21,
	"modules/schoolcalendar/schoolcalendar.go:Days":                                                   27,
	"modules/schoolmembership/schoolmembership.go:Capability":                                         59,
	"modules/schoolmembership/schoolmembership.go:Command":                                            28,
	"modules/schoolmembership/schoolmembership.go:Owner":                                              60,
	"modules/schoolmembership/schoolmembership.go:Query":                                              18,
	"modules/schoolstructure/contract/group_management.go:GroupManagement":                            16,
	"modules/schoolstructure/contracts.go:GroupManagement":                                            16,
	"modules/schoolstructure/schoolstructure.go:Owner":                                                22,
	"modules/schoolstructure/transition.go:TransitionCapability":                                      16,
	"modules/settings/records/settings_interface.go:BatchSettingsService":                             33,
	"modules/settings/records/settings_interface.go:SettingsService":                                  27,
	"modules/studentpresence/presence.go:Presence":                                                    52,
	"modules/studentpresence/studentpresence.go:Capability":                                           128,
	"modules/studentpresence/studentpresence.go:Command":                                              79,
	"modules/studentpresence/studentpresence.go:Owner":                                                138,
	"modules/studentpresence/studentpresence.go:Query":                                                47,
	"modules/studentpresence/visit.go:VisitQuery":                                                     13,
	"modules/timetable/activity_instances.go:ActivityInstanceCapability":                              15,
	"modules/timetable/activitycontract/capability.go:ActivityService":                                40,
	"modules/timetable/fixturetest/activity_instance_records_test_helpers.go:ActivityInstanceRecords": 20,
	"modules/timetable/fixturetest/instance_lifecycle_test_helpers.go:InstanceLifecycle":              21,
	"modules/timetable/fixturetest/instance_student_records_test_helpers.go:InstanceStudentRecords":   39,
	"modules/timetable/instance_lifecycle.go:InstanceLifecycleCapability":                             17,
	"modules/timetable/instance_students.go:InstanceStudentCapability":                                16,
	"modules/timetable/operations.go:OperationCapability":                                             14,
	"modules/timetable/timetable.go:Capability":                                                       164,
	"modules/timetable/timetable.go:Command":                                                          97,
	"modules/timetable/timetable.go:Owner":                                                            166,
	"modules/timetable/timetable.go:Query":                                                            60,
	"modules/timetable/timetable_data.go:TimetableDataCapability":                                     24,
	"modules/workforce/absence.go:AbsenceQuery":                                                       15,
	"modules/workforce/shift.go:ShiftCommand":                                                         19,
	"modules/workforce/staffadmin.go:StaffDirectory":                                                  13,
	"modules/workforce/staffadmin.go:StaffDocuments":                                                  16,
	"modules/workforce/staffrecord.go:StaffRecordCommand":                                             13,
	"modules/workforce/timetracking.go:StaffAbsences":                                                 22,
	"modules/workforce/timetracking.go:WorkSessions":                                                  15,
	"modules/workforce/workforce.go:Capability":                                                       146,
	"modules/workforce/workforce.go:Command":                                                          82,
	"modules/workforce/workforce.go:Query":                                                            64,
	"modules/workforce/worksession.go:WorkSessionCommand":                                             24,
	"modules/workforce/worksession.go:WorkSessionQuery":                                               18,
	"workflows/gradetransition/transition.go:Structure":                                               16,
	"workflows/parentportal/messaging_announcement_access_port.go:AnnouncementAccess":                 23,
}

func TestModuleFacadeMethodSetRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Fatalf("Could not find backend root: %v", err)
		return
	}

	widths, err := moduleFacadeMethodSetScan(backendRoot)
	if err != nil {
		t.Fatalf("module-facade method-set scan failed: %v", err)
	}

	violations := moduleFacadeViolations(widths, moduleFacadeMethodSetAllowlist, moduleFacadeMethodSetMeasure)
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Module-facade method-set ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// moduleFacadeMethodSetMeasure names the method-set measurement in a violation.
var moduleFacadeMethodSetMeasure = moduleFacadeMeasure{
	register: "facade method set",
	verb:     "has a method set of",
}

// moduleFacadeMethodSetScan type-checks the root packages and returns the
// method-set width of every exported composite interface above the threshold.
func moduleFacadeMethodSetScan(backendRoot string) (map[string]int, error) {
	dirs, err := moduleFacadeRootDirs(backendRoot)
	if err != nil {
		return nil, err
	}
	imp, err := newModuleFacadeImporter(backendRoot)
	if err != nil {
		return nil, err
	}
	widths := make(map[string]int)
	for _, dir := range dirs {
		pkg, err := imp.Import(moduleFacadeModulePath + "/" + dir)
		if err != nil {
			return nil, fmt.Errorf("type-check %s: %w", dir, err)
		}
		measured, err := moduleFacadeMethodSetWidths(pkg, imp.fset, dir)
		if err != nil {
			return nil, fmt.Errorf("%w (first type errors: %v)", err, imp.errors[pkg.Path()])
		}
		for key, width := range measured {
			widths[key] = width
		}
	}
	return widths, nil
}

// moduleFacadeModulePath is the backend's module path; first-party imports
// below it are type-checked from source.
const moduleFacadeModulePath = "github.com/moto-nrw/project-phoenix"

// moduleFacadeImporter type-checks first-party packages from source with the
// standard library alone (the test-support role may not import
// compiler-tooling such as golang.org/x/tools). Function bodies are skipped:
// only declarations shape a method set. The standard library is type-checked
// the same way from GOROOT source. Files are selected for linux/amd64 without
// cgo and with the testhooks tag, the fixed build of the architecture check;
// go/build and os/exec are no dependency class of test support either. Every
// other import resolves to an empty package, and
// the errors that causes are ignored; an embedded
// interface that therefore stays unresolved fails the scan in
// moduleFacadeMethodSetWidths instead of shrinking a method set silently.
type moduleFacadeImporter struct {
	backendRoot string
	fset        *token.FileSet
	goroot      string
	cache       map[string]*types.Package
	errors      map[string][]error // first type errors per package, for diagnostics
}

func newModuleFacadeImporter(backendRoot string) (*moduleFacadeImporter, error) {
	goroot, err := moduleFacadeGOROOT()
	if err != nil {
		return nil, err
	}
	return &moduleFacadeImporter{
		backendRoot: backendRoot,
		fset:        token.NewFileSet(),
		goroot:      goroot,
		cache:       make(map[string]*types.Package),
		errors:      make(map[string][]error),
	}, nil
}

// moduleFacadeGOROOT finds the toolchain's GOROOT. A -trimpath test binary
// does not know its own, but go test puts $GOROOT/bin first on PATH.
func moduleFacadeGOROOT() (string, error) {
	candidates := []string{os.Getenv("GOROOT")}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		candidates = append(candidates, filepath.Dir(dir))
	}
	for _, root := range candidates {
		if root == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, "src", "runtime", "runtime.go")); err == nil {
			return root, nil
		}
	}
	return "", fmt.Errorf("no GOROOT with src/runtime found via $GOROOT or $PATH")
}

func (imp *moduleFacadeImporter) Import(importPath string) (*types.Package, error) {
	if pkg, ok := imp.cache[importPath]; ok {
		return pkg, nil
	}
	if importPath == "unsafe" {
		return types.Unsafe, nil
	}
	var dir string
	if rel, firstParty := strings.CutPrefix(importPath, moduleFacadeModulePath+"/"); firstParty {
		dir = filepath.Join(imp.backendRoot, filepath.FromSlash(rel))
	} else if !strings.Contains(strings.SplitN(importPath, "/", 2)[0], ".") {
		dir = filepath.Join(imp.goroot, "src", filepath.FromSlash(importPath))
	} else {
		pkg := types.NewPackage(importPath, path.Base(importPath))
		pkg.MarkComplete()
		imp.cache[importPath] = pkg
		return pkg, nil
	}
	files, err := imp.parseBuildFiles(dir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", importPath, err)
	}
	config := types.Config{Importer: imp, IgnoreFuncBodies: true, Error: func(err error) {
		if len(imp.errors[importPath]) < 3 {
			imp.errors[importPath] = append(imp.errors[importPath], err)
		}
	}}
	pkg, _ := config.Check(importPath, imp.fset, files, nil)
	imp.cache[importPath] = pkg
	return pkg, nil
}

// moduleFacadeMethodSetWidths measures one type-checked package, keyed
// "relDir/file.go:Name".
func moduleFacadeMethodSetWidths(pkg *types.Package, fset *token.FileSet, relDir string) (map[string]int, error) {
	widths := make(map[string]int)
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		obj, ok := scope.Lookup(name).(*types.TypeName)
		if !ok || !obj.Exported() {
			continue
		}
		iface, ok := types.Unalias(obj.Type()).Underlying().(*types.Interface)
		if !ok || iface.NumEmbeddeds() == 0 || !iface.IsMethodSet() {
			continue
		}
		key := relDir + "/" + path.Base(filepath.ToSlash(fset.Position(obj.Pos()).Filename)) + ":" + name
		if err := moduleFacadeResolvedEmbeddings(iface, map[*types.Interface]bool{}); err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		if width := iface.NumMethods(); width > moduleFacadeThreshold {
			widths[key] = width
		}
	}
	return widths, nil
}

// moduleFacadeResolvedEmbeddings fails when an embedded element, at any
// depth, did not resolve to an interface.
func moduleFacadeResolvedEmbeddings(iface *types.Interface, seen map[*types.Interface]bool) error {
	if seen[iface] {
		return nil
	}
	seen[iface] = true
	for i := 0; i < iface.NumEmbeddeds(); i++ {
		embedded := iface.EmbeddedType(i)
		inner, ok := types.Unalias(embedded).Underlying().(*types.Interface)
		if !ok {
			return fmt.Errorf("embedded %s did not resolve to an interface; the method set cannot be measured", embedded)
		}
		if err := moduleFacadeResolvedEmbeddings(inner, seen); err != nil {
			return err
		}
	}
	return nil
}

// parseBuildFiles parses the non-test files of dir that build for
// linux/amd64 with the testhooks tag and without cgo.
func (imp *moduleFacadeImporter) parseBuildFiles(dir string) ([]*ast.File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !moduleFacadeBuildsFileName(name) {
			continue
		}
		file, err := parser.ParseFile(imp.fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution|parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		if moduleFacadeBuildsFile(file) {
			files = append(files, file)
		}
	}
	return files, nil
}

// The GOOS and GOARCH values go/build recognizes as file-name suffixes.
var (
	moduleFacadeKnownOS   = moduleFacadeSet("aix android darwin dragonfly freebsd hurd illumos ios js linux nacl netbsd openbsd plan9 solaris wasip1 windows zos")
	moduleFacadeKnownArch = moduleFacadeSet("386 amd64 amd64p32 arm armbe arm64 arm64be loong64 mips mipsle mips64 mips64le mips64p32 mips64p32le ppc ppc64 ppc64le riscv riscv64 s390 s390x sparc sparc64 wasm")
)

func moduleFacadeSet(words string) map[string]bool {
	set := make(map[string]bool)
	for _, word := range strings.Fields(words) {
		set[word] = true
	}
	return set
}

// moduleFacadeBuildsFileName applies go/build's file-name rules for
// linux/amd64: no test files, no _ or . prefix, and a _GOOS, _GOARCH or
// _GOOS_GOARCH suffix must match.
func moduleFacadeBuildsFileName(name string) bool {
	if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") ||
		strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
		return false
	}
	parts := strings.Split(strings.TrimSuffix(name, ".go"), "_")
	n := len(parts)
	if n >= 3 && moduleFacadeKnownOS[parts[n-2]] && moduleFacadeKnownArch[parts[n-1]] {
		return parts[n-2] == "linux" && parts[n-1] == "amd64"
	}
	if n >= 2 && moduleFacadeKnownOS[parts[n-1]] {
		return parts[n-1] == "linux"
	}
	if n >= 2 && moduleFacadeKnownArch[parts[n-1]] {
		return parts[n-1] == "amd64"
	}
	return true
}

// moduleFacadeBuildsFile evaluates the //go:build line and drops cgo files.
func moduleFacadeBuildsFile(file *ast.File) bool {
	for _, spec := range file.Imports {
		if spec.Path.Value == `"C"` {
			return false
		}
	}
	for _, group := range file.Comments {
		if group.Pos() >= file.Package {
			break
		}
		for _, comment := range group.List {
			if !constraint.IsGoBuild(comment.Text) {
				continue
			}
			expr, err := constraint.Parse(comment.Text)
			if err != nil {
				return false
			}
			return expr.Eval(func(tag string) bool {
				switch tag {
				case "linux", "amd64", "unix", "gc", "testhooks":
					return true
				}
				return strings.HasPrefix(tag, "go1.")
			})
		}
	}
	return true
}
