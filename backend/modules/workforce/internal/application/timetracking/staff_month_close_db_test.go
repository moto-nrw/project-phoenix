package timetracking_test

import (
	"fmt"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMonthClose_ReadsClosedSnapshotsOnce pins #5821: a school-wide close
// checks the existing month snapshots of all staff with one read, after every
// staff balance lock is held, instead of two lookups per staff member. Each
// staff member still gets a fresh snapshot for the month.
func TestMonthClose_ReadsClosedSnapshotsOnce(t *testing.T) {
	t.Parallel()

	for _, staffCount := range []int{3, 8} {
		t.Run(fmt.Sprintf("staff_%d", staffCount), func(t *testing.T) {
			f := newSnapshotFixture(t)
			// The fixture brings two staff members; add the rest.
			for i := 2; i < staffCount; i++ {
				testpkg.CreateTestStaffForTenant(t, f.db, f.tenantID, "Abschluss", fmt.Sprintf("Weitere-%d", i))
			}

			counter := testpkg.CaptureQueriesForContext(t, f.db)
			result, err := f.closeSvc.CloseMonth(counter.Context(f.ctx), f.admin, snapshotYear, snapshotMonth, "Abschluss")
			counter.Stop()
			require.NoError(t, err)
			assert.Equal(t, staffCount, result.ClosedStaff)
			assert.Zero(t, result.SkippedStaff)
			require.Len(t, result.Snapshots, staffCount)

			snapshotReads := counter.Matching(func(sql string) bool {
				return strings.HasPrefix(strings.TrimSpace(sql), "select") && strings.Contains(sql, "staff_month_balance_snapshots")
			})
			locks := counter.Matching(func(sql string) bool { return strings.Contains(sql, "pg_advisory_xact_lock") })
			t.Logf("#5821 month close staff_%d: %d statements, %d snapshot reads, %d balance locks",
				staffCount, counter.Total(), len(snapshotReads), len(locks))
			assert.Len(t, locks, staffCount, "one balance lock per staff member")
			batchReads := counter.Matching(func(sql string) bool {
				return strings.Contains(sql, "staff_month_balance_snapshots") && strings.Contains(sql, ".staff_id in (")
			})
			assert.Len(t, batchReads, 1, "the close checks read the snapshots once for all staff")
			// The month summary keeps its own carry-in lookup per staff member.
			assert.Len(t, snapshotReads, staffCount+1, "no per-staff close lookups remain")
		})
	}
}
