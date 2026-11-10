package migrations

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffShiftSeriesOverridePreservesLegacyDetachments(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Series", "Override")
	day := calendar.NewDate(2026, 11, 23)
	plain := testpkg.CreateTestStaffShift(t, db, staff.ID, day, testpkg.StaffShiftOpts{})
	detached := testpkg.CreateTestStaffShift(t, db, staff.ID, day.AddDays(1), testpkg.StaffShiftOpts{})
	_, err := db.ExecContext(ctx, `UPDATE schedule.staff_shifts SET detached = TRUE WHERE id = ?`, detached.ID)
	require.NoError(t, err)

	// This test owns a database clone, so the down/up cycle cannot affect
	// other tests. Detached legacy rows have no trustworthy edit provenance.
	require.NoError(t, staffShiftSeriesOverrideDown(ctx, db))
	require.NoError(t, staffShiftSeriesOverrideUp(ctx, db))
	var override bool
	require.NoError(t, db.NewRaw(`SELECT series_override FROM schedule.staff_shifts WHERE id = ?`, plain.ID).Scan(ctx, &override))
	assert.False(t, override, "plain occurrences remain subject to series validity")
	require.NoError(t, db.NewRaw(`SELECT series_override FROM schedule.staff_shifts WHERE id = ?`, detached.ID).Scan(ctx, &override))
	assert.True(t, override, "an unknown legacy detachment is preserved conservatively")
}
