package test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every lock a Timetable, Workforce or School Membership transaction takes,
// and every lock of the workflows that compose them, is admitted through the
// unit of work's lock order (tenant.OrderLock) before it can wait (#6702).
// The ledger then refuses a gate, day or staff row after a later lock, and a
// staff row below one the transaction already holds, so a writer fails
// instead of closing a 40P01 cycle with offboarding, a sick report or
// another writer. The lock classes and their order are documented on
// tenant.LockClass.
//
// The scan covers every non-test Go file under rowLockAdmissionRoots and the
// listed files of other owners whose locks take part in the same protocol
// (offboarding's person and supervision rows, the staff person merge). A lock
// site is a BUN For(...) call or a string literal or package constant with
// FOR UPDATE / FOR NO KEY UPDATE / FOR SHARE / FOR KEY SHARE, LOCK TABLE or a
// pg_advisory lock. Each site needs a tenant.OrderLock call before it in the
// same or an enclosing function whose error is used.
//
// rowLockPrimitives lists the lock statements whose key is a parameter: they
// cannot know the class, so the callers admit. Every file that may reference
// a primitive is listed with it and must itself call tenant.OrderLock.
var rowLockAdmissionRoots = []string{
	"modules/timetable",
	"modules/workforce",
	"modules/schoolmembership",
	"workflows/shiftplansync",
	"workflows/staffoffboarding",
	"workflows/staffpersonmerge",
}

var rowLockAdmissionFiles = []string{
	"modules/identityaccess/internal/adapters/postgres/account_login_batch.go",
	"api/timetable_advisory_locks.go",
	"modules/studentpresence/internal/application/presence/presence_guards.go",
	"modules/peopledirectory/internal/adapters/peoplepostgres/person.go",
	"modules/peopledirectory/internal/adapters/postgres/staff_person_merge.go",
	"modules/peopledirectory/internal/application/caregiver_capability.go",
	"modules/studentpresence/internal/adapters/postgres/offboarding.go",
}

type rowLockPrimitive struct {
	// site is "file#Function" of the lock statement.
	site string
	// refs are the source texts that reference the primitive.
	refs   []string
	reason string
	// callers hand it a key; each calls tenant.OrderLock first.
	callers []string
	// wiring binds the primitive to an admitting acquirer and takes no lock
	// of its own.
	wiring []string
}

var rowLockPrimitives = []rowLockPrimitive{
	{
		site:   "modules/timetable/internal/adapters/postgres/store.go#Store.AcquireTransactionLock",
		refs:   []string{".AcquireTransactionLock"},
		reason: "the recurrence and grade-transition gates and the day locks share one advisory statement",
		callers: []string{
			"modules/timetable/internal/adapters/postgres/native_compose_bindings_recurrence_lock.go",
		},
		wiring: []string{
			"modules/timetable/ports/tenant_transactions.go",
			"modules/timetable/internal/adapters/postgres/native_compose_bindings_instance_lifecycle_ports.go",
			// Both hand the statement to lockScheduleDays (day_locks.go),
			// which admits every day before it calls it.
			"modules/timetable/internal/application/staff_deviations.go",
			"modules/timetable/internal/application/instance_lifecycle.go",
		},
	},
	{
		site:   "modules/workforce/internal/adapters/postgres/absence_store.go#Store.AcquireXactLock",
		refs:   []string{"store.AcquireXactLock", "New(DatabaseRuntime(db)).AcquireXactLock"},
		reason: "Workforce's per-staff gates share one advisory statement; the gate functions admit (TestOrderedLocksHaveOneAcquirer)",
		callers: []string{
			"modules/workforce/internal/adapters/postgres/native_compose_bindings_staff_shift_lock.go",
		},
		wiring: []string{
			"modules/workforce/compose/new.go",
			"modules/workforce/compose/documentcleanup.go",
			"modules/workforce/compose/staffadmin.go",
		},
	},
	{
		site:   "modules/workforce/internal/adapters/postgres/native_compose_bindings_staff_shift_lock.go#AcquireXactLock",
		refs:   []string{"postgres.AcquireXactLock(", "AcquireXactLock(ctx, key, New("},
		reason: "Workforce's per-staff gates share one advisory statement; the gate functions admit (TestOrderedLocksHaveOneAcquirer)",
		callers: []string{
			"modules/workforce/internal/adapters/postgres/native_compose_bindings_staff_shift_lock.go",
		},
		wiring: []string{
			// transaction.acquireXactLock hands it to the gate functions.
			"modules/workforce/compose/new.go",
		},
	},
	{
		site:   "api/timetable_advisory_locks.go#timetableAdvisoryLocks.AcquireXactLock",
		refs:   []string{"Advisory.AcquireXactLock"},
		reason: "the spontaneous start's advisory locks run on the root's tenant runtime",
		callers: []string{
			"modules/timetable/internal/application/timetable_data_spontaneous.go",
		},
	},
}

// rowLockAdmitter names the function that admits a lock statement of a
// persistence package the architecture policy keeps away from the tenant
// runtime: it calls tenant.OrderLock (checked) before its last call of the
// statement's function (calls; the statement's method name when empty).
type rowLockAdmitter struct {
	file     string
	function string
	calls    string
}

const (
	membershipApplication = "modules/schoolmembership/internal/application/"
	membershipStore       = "modules/schoolmembership/internal/adapters/postgres/"
)

// rowLockAdmitted maps "file#Function" of a lock statement to its admitters.
// An admitter with function "*" stands for every function of that directory
// that calls the statement with a lock: each must admit before the call. A
// call that passes false or "" takes no ordered lock (a shared class-write
// gate, an unlocked read). A nil entry is a documented reader of a gate.
var rowLockAdmitted = map[string][]rowLockAdmitter{
	"modules/identityaccess/internal/adapters/postgres/account_login_batch.go#Store.LockLoginAccounts": {{"modules/identityaccess/internal/application/staff_offboarding_account_locks.go", "LockStaffOffboardingAccounts", ""}},

	membershipStore + "store.go#Store.ListStaff":                           {{membershipApplication + "staff_row_lock.go", "listStaffForMutation", ""}},
	membershipStore + "store.go#Store.FindStaff":                           {{"modules/schoolmembership/internal/application/staff_row_lock.go", "findStaffRow", ""}},
	membershipStore + "store.go#Store.CreateStaff":                         {{membershipApplication, "*", "CreateStaff"}},
	membershipStore + "store.go#Store.FindTeacher":                         {{membershipApplication, "*", "FindTeacher"}},
	membershipStore + "store.go#Store.FindExternalCaregiverRecord":         {{membershipApplication, "*", "FindExternalCaregiverRecord"}},
	membershipStore + "class_list_entry.go#Store.FindClassListEntry":       {{membershipApplication, "*", "FindClassListEntry"}},
	membershipStore + "list_filters.go#groupAssignmentListQuery":           {{membershipApplication, "*", "ListGroupAssignments"}},
	membershipStore + "teaching_assignment.go#Store.ListClassAssignments":  {{membershipApplication, "*", "ListClassAssignments"}},
	membershipStore + "teaching_assignment.go#Store.LockGroupAssignments":  {{membershipApplication, "*", "LockGroupAssignments"}},
	membershipStore + "retired_staff.go#Store.MarkNextRetiredStaffErased":  {{membershipApplication, "*", "MarkNextRetiredStaffErased"}},
	membershipStore + "student_counts.go#Store.LockChildQuota":             {{membershipApplication, "*", "LockChildQuota"}},
	membershipStore + "student_enrollment.go#Store.LockStudentClassWrites": {{membershipApplication, "*", "LockStudentClassWrites"}},
	// The shared class-write gate a student insert embeds, here and in People
	// Directory's student statements, is a reader of the gate; only the
	// exclusive holder is ordered (tenant.LockStudentClassWrites).
	membershipStore + "student_enrollment.go#StudentClassWriteGateQuery":  nil,
	membershipStore + "student_membership.go#Store.LockStudentWindow":     {{membershipApplication, "*", "LockStudentWindow"}},
	membershipStore + "staff_person_merge.go#Store.StaffMergeMemberships": {{"modules/schoolmembership/internal/application/staff_person_merge.go", "StaffMergeMemberships", ""}},
	// The person lock of a staff creation is admitted. Two root bindings
	// under an approved exception (ADR 0067) lock the person unadmitted:
	// staff admin's LockStaff takes it after the staff row, Care Plan's master
	// data update takes no gate, day or staff row; both follow the order.
	"modules/peopledirectory/internal/adapters/peoplepostgres/person.go#personRepository.FindByIDForUpdate": {
		{"modules/peopledirectory/internal/application/staff_directory.go", "requireLivePersonLocked", ""},
	},
	"modules/studentpresence/internal/adapters/postgres/offboarding.go#Store.LockStaffSupervision": {
		{"modules/studentpresence/internal/application/offboarding.go", "LockStaffSupervision", ""},
	},
}

func TestRowLocksAreAdmittedThroughTheLockOrder(t *testing.T) {
	t.Parallel()
	root, err := findBackendRoot()
	if err != nil {
		t.Fatal(err)
	}
	primitives := map[string]rowLockPrimitive{}
	for _, primitive := range rowLockPrimitives {
		primitives[primitive.site] = primitive
	}
	files, err := rowLockAdmissionScope(root)
	if err != nil {
		t.Fatal(err)
	}
	seenPrimitives := map[string]bool{}
	seenAdmitted := map[string]bool{}
	for _, rel := range files {
		source, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		sites, err := scanRowLockSites(rel, source)
		if err != nil {
			t.Fatal(err)
		}
		for _, site := range sites {
			key := rel + "#" + site.function
			if _, ok := primitives[key]; ok {
				seenPrimitives[key] = true
				continue
			}
			if admitters, ok := rowLockAdmitted[key]; ok {
				seenAdmitted[key] = true
				checkRowLockAdmitters(t, root, site, admitters)
				continue
			}
			if !site.admitted {
				t.Errorf("%s takes a lock without tenant.OrderLock before it: admit the lock in its class (tenant.LockClass) first", site)
			}
		}
	}
	for key := range rowLockAdmitted {
		if !seenAdmitted[key] {
			t.Errorf("rowLockAdmitted lists %s, which holds no lock statement any more: remove the entry", key)
		}
	}
	for _, primitive := range rowLockPrimitives {
		if !seenPrimitives[primitive.site] {
			t.Errorf("rowLockPrimitives lists %s, which holds no lock statement any more: remove the entry", primitive.site)
		}
		checkRowLockPrimitiveCallers(t, root, primitive)
	}
}

func rowLockAdmissionScope(root string) ([]string, error) {
	var files []string
	for _, dir := range rowLockAdmissionRoots {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return append(files, rowLockAdmissionFiles...), nil
}

// checkRowLockPrimitiveCallers requires every production reference to the
// primitive to sit in a listed caller that admits, or in listed wiring.
func checkRowLockPrimitiveCallers(t *testing.T, root string, primitive rowLockPrimitive) {
	t.Helper()
	allowed := map[string]bool{strings.SplitN(primitive.site, "#", 2)[0]: true}
	for _, file := range primitive.wiring {
		allowed[file] = true
	}
	for _, file := range primitive.callers {
		allowed[file] = true
		source, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(source), "tenant.OrderLock(") {
			t.Errorf("%s hands a key to %s without tenant.OrderLock", file, primitive.site)
		}
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		referenced := false
		for _, ref := range primitive.refs {
			referenced = referenced || strings.Contains(string(source), ref)
		}
		if !referenced {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if !allowed[filepath.ToSlash(rel)] {
			t.Errorf("%s references the lock primitive %s (%s); take the lock through its admitting caller instead", filepath.ToSlash(rel), primitive.site, primitive.reason)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// checkRowLockAdmitters requires each admitter to admit before it calls the
// statement's function.
func checkRowLockAdmitters(t *testing.T, root string, site rowLockSite, admitters []rowLockAdmitter) {
	t.Helper()
	called := site.function
	if dot := strings.LastIndex(called, "."); dot >= 0 {
		called = called[dot+1:]
	}
	for _, admitter := range admitters {
		called := called
		if admitter.calls != "" {
			called = admitter.calls
		}
		if admitter.function == "*" {
			unadmitted, found, err := everyLockingCallAdmitted(filepath.Join(root, admitter.file), called)
			if err != nil {
				t.Fatal(err)
			}
			if !found {
				t.Errorf("%s: no function takes %s with a lock any more (%s): update rowLockAdmitted", admitter.file, called, site)
			}
			for _, at := range unadmitted {
				at = strings.TrimPrefix(filepath.ToSlash(at), filepath.ToSlash(root)+"/")
				t.Errorf("%s calls %s with a lock without tenant.OrderLock before it (%s)", at, called, site)
			}
			continue
		}
		source, err := os.ReadFile(filepath.Join(root, admitter.file))
		if err != nil {
			t.Fatal(err)
		}
		ok, err := admitsBeforeCall(admitter.file, source, admitter.function, called)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Errorf("%s: %s does not call tenant.OrderLock before it calls %s (%s)", admitter.file, admitter.function, called, site)
		}
	}
}
