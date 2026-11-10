package test

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The advisory locks of the cross-owner lock order (#6232) are taken in one
// place each, and that place admits the lock through tenant.OrderLock before
// it waits: Timetable's day locks, its recurrence and grade-transition gates
// and Workforce's staff-shift gate. A second place that hashes one of these
// keys would wait outside the order check, and a sick report, a
// materialization or a series write could close a 40P01 cycle again; review
// found such cycles three times in #6300. So the key builders may appear only
// in the files below, and every acquiring file calls tenant.OrderLock.
//
// The order itself is enforced at run time: tenant.OrderLock refuses a lock
// against the order before it waits (tenant/lock_order_test.go), and the
// concurrent tests in workflows/shiftplansync/compose/sick_report_races_test.go
// and modules/timetable/compose/pickup_extension_failure_points_test.go run
// the real writers against each other.
var orderedLockKeyBuilders = map[string]struct {
	definition string
	acquirers  []string
}{
	"SubstituteDayLockKey(": {
		definition: "modules/timetable/lifecycle.go",
		acquirers:  []string{"modules/timetable/internal/application/day_locks.go"},
	},
	"TenantRecurrenceLockKey(": {
		definition: "modules/timetable/internal/domain/schedulerecords/recurrence_lock.go",
		acquirers:  []string{"modules/timetable/internal/adapters/postgres/native_compose_bindings_recurrence_lock.go"},
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
			if !strings.Contains(string(source), "tenant.OrderLock(") {
				t.Errorf("%s takes %s without tenant.OrderLock", acquirer, builder)
			}
		}
	}
}
