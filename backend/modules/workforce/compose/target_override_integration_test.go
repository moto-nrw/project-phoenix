package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The capability expands a stored Sonderarbeitszeit into the days it sets a
// target on: weekends and the School Calendar's statutory holidays are left
// out, and the holiday lookup covers only the span the ranges touch.
func TestStaffTargetOverrideDays_SkipsStatutoryHolidays(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	runtime := testpkg.ConfigRuntime(db)
	var asked [][2]string
	capability, err := New(Dependencies{LockStaffAssignment: runtime.LockStaffAssignment,
		DB: db, LiveStaffIDs: runtime.LiveStaffIDs, Observe: func(Observation) {},
		StatutoryHolidays: func(_ context.Context, from, to string) (map[string]bool, error) {
			asked = append(asked, [2]string{from, to})
			return map[string]bool{"2026-12-25": true}, nil
		},
	})
	require.NoError(t, err)

	staff := testpkg.CreateTestStaff(t, db, "Sonder", "Holiday")
	without := testpkg.CreateTestStaff(t, db, "Sonder", "Without")
	_, err = capability.CreateStaffTargetOverride(ctx, staff.ID, workforce.StaffTargetOverrideFields{
		StartDate: "2026-12-21", EndDate: "2026-12-31", DailyMinutes: 300,
	}, nil)
	require.NoError(t, err)

	days, err := capability.StaffTargetOverrideDays(ctx, []int64{staff.ID, without.ID}, "2026-12-01", "2026-12-31")
	require.NoError(t, err)
	assert.Equal(t, 300, days[staff.ID]["2026-12-24"])
	assert.NotContains(t, days[staff.ID], "2026-12-25", "a statutory holiday keeps Soll 0")
	assert.NotContains(t, days[staff.ID], "2026-12-26", "Saturday is no override day")
	assert.Len(t, days[staff.ID], 8)
	assert.Empty(t, days[without.ID])
	assert.Equal(t, [][2]string{{"2026-12-21", "2026-12-31"}}, asked)

	asked = nil
	days, err = capability.StaffTargetOverrideDays(ctx, []int64{without.ID}, "2026-12-01", "2026-12-31")
	require.NoError(t, err)
	assert.Empty(t, days)
	assert.Empty(t, asked, "no Sonderarbeitszeit, no calendar lookup")
}

// Tenant isolation at the capability: another school's staff member and row
// stay invisible under row-level security.
func TestStaffTargetOverrides_TenantIsolation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	capability := buildWorkforce(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Sonder", "TenantA")
	created, err := capability.CreateStaffTargetOverride(testpkg.Ctx(t), staff.ID, workforce.StaffTargetOverrideFields{
		StartDate: "2026-10-19", EndDate: "2026-10-23", DailyMinutes: 510,
	}, nil)
	require.NoError(t, err)

	t.Run("other tenant", func(t *testing.T) {
		otherCtx := testpkg.OwnCtx(t)
		rows, err := capability.ListStaffTargetOverrides(otherCtx, staff.ID)
		require.NoError(t, err)
		assert.Empty(t, rows)
		days, err := capability.StaffTargetOverrideDays(otherCtx, []int64{staff.ID}, "2026-10-01", "2026-10-31")
		require.NoError(t, err)
		assert.Empty(t, days[staff.ID])
		err = capability.DeleteStaffTargetOverride(otherCtx, staff.ID, created.ID)
		assert.ErrorIs(t, err, workforce.ErrStaffTargetOverrideNotFound)
	})

	rows, err := capability.ListStaffTargetOverrides(testpkg.Ctx(t), staff.ID)
	require.NoError(t, err)
	assert.Len(t, rows, 1, "the owning tenant keeps its row")
}
