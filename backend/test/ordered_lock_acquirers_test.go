package test

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The locks of the cross-owner lock order (#6232, #6702) are taken in one place
// each, and that place admits the lock through tenant.OrderLock before it
// waits: Timetable's day locks and recurrence gate, the grade-transition gate
// (taken by Timetable and by School Structure), Workforce's staff-shift gate
// and School Membership's staff row lock (#6583). A second place that takes
// one of these locks would wait outside the order check, and a sick report,
// a materialization, a series write or an offboarding could close a 40P01
// cycle again; review found such cycles three times in #6300. So the key
// builders may appear only in the files below, and every acquiring file calls
// tenant.OrderLock.
//
// The order itself is enforced at run time: tenant.OrderLock refuses a lock
// against the order before it waits (tenant/lock_order_test.go), and the
// concurrent tests in workflows/shiftplansync/compose/sick_report_races_test.go,
// modules/timetable/compose/pickup_extension_failure_points_test.go and
// modules/peopledirectory/contracttest/staff_offboarding_day_writer_race_test.go run
// the real writers against each other.
var orderedLockKeyBuilders = map[string]struct {
	definition string
	acquirers  []string
}{
	// School Structure's adapter builds the key; the application admits the
	// gate in lockTransitionGate, the only caller of the store method.
	"tenantTransitionsLockKey(": {
		definition: "modules/schoolstructure/internal/adapters/postgres/transition_store.go",
	},
	"store.LockTransitions(": {
		acquirers: []string{"modules/schoolstructure/internal/application/transition.go"},
	},
	"SubstituteDayLockKey(": {
		definition: "modules/timetable/lifecycle.go",
		acquirers:  []string{"modules/timetable/internal/application/day_locks.go"},
	},
	"TenantRecurrenceLockKey(": {
		definition: "modules/timetable/internal/domain/schedulerecords/recurrence_lock.go",
		acquirers:  []string{"modules/timetable/internal/adapters/postgres/native_compose_bindings_recurrence_lock.go"},
	},
	// School Membership's staff row lock (#6583): every read of a staff row
	// passes findStaffRow, which admits the lock before it waits.
	"store.FindStaff(": {
		acquirers: []string{"modules/schoolmembership/internal/application/staff_row_lock.go"},
	},
	// Workforce's work-time gates (#6702): the balance gate is also taken by
	// Student Presence before a supervision write and by the Settings
	// Platform, which builds the key in its application and admits it in
	// its runtime binding.
	`"staff-balance:`: {
		definition: "modules/settings/internal/application/composition_setting_transaction_present.go",
		acquirers: []string{
			"modules/workforce/internal/adapters/postgres/composition_lockstaffbalancegate.go",
			"modules/studentpresence/internal/application/presence/presence_guards.go",
			"test/tenant_runtime.go",
		},
	},
	"app.LockSettingStaffBalance(": {
		acquirers: []string{"modules/settings/compose/tenant_settings_runtime.go"},
	},
	`"staff-absence:`: {
		acquirers: []string{"modules/workforce/internal/adapters/postgres/composition_lockstaffabsencegate.go"},
	},
	`"staff-qualifications:`: {
		acquirers: []string{"modules/workforce/internal/adapters/postgres/composition_lockstaffqualificationsgate.go"},
	},
	// Timetable's spontaneous start gates (#6702).
	`"timetable:spontaneous-`: {
		acquirers: []string{"modules/timetable/internal/application/timetable_data_spontaneous.go"},
	},
	`"staff-shift:`: {
		acquirers: []string{
			"modules/workforce/internal/adapters/postgres/composition_lockstaffshiftsgate.go",
			"modules/workforce/internal/adapters/postgres/native_compose_bindings_staff_shift_lock.go",
		},
	},
}

func TestOrderedLocksHaveOneAcquirer(t *testing.T) {
	t.Parallel()
	root, err := findBackendRoot()
	if err != nil {
		t.Fatal(err)
	}
	found := make(map[string][]string, len(orderedLockKeyBuilders))
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
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
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		for builder := range orderedLockKeyBuilders {
			if strings.Contains(string(source), builder) {
				found[builder] = append(found[builder], rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for builder, want := range orderedLockKeyBuilders {
		allowed := append([]string{}, want.acquirers...)
		if want.definition != "" {
			allowed = append(allowed, want.definition)
		}
		sort.Strings(allowed)
		got := found[builder]
		sort.Strings(got)
		if strings.Join(got, "\n") != strings.Join(allowed, "\n") {
			t.Errorf("%s appears in %v, want only %v: take the lock through the ordered acquirer instead", builder, got, allowed)
		}
		for _, acquirer := range want.acquirers {
			source, err := os.ReadFile(filepath.Join(root, acquirer))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(source), "tenant.OrderLock(") && !strings.Contains(string(source), "tenant.AdmitBefore(") {
				t.Errorf("%s takes %s without tenant.OrderLock", acquirer, builder)
			}
		}
	}
}
