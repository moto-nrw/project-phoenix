package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStaffPlanningProfilesRequireSchoolAndIsolateAllJoinedRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	capability := buildWorkforce(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Planning", "Own")
	foreign := testpkg.NewTenantScope(t, db)
	foreignStaff := testpkg.CreateTestStaffForTenant(t, db, foreign.TenantID, "Planning", "Foreign")
	employment := buildStaffEmployment(t, db)
	for _, entry := range []struct {
		ctx        context.Context
		id, school int64
		anchor     string
	}{
		{testpkg.Ctx(t), staff.ID, testpkg.Tenant(t), "2026-01-05"}, {foreign.Context(), foreignStaff.ID, foreign.TenantID, "2026-01-12"},
	} {
		require.NoError(t, employment.SaveStaffEmployment(entry.ctx, workforce.StaffEmployment{MembershipID: entry.id, RotationAnchorDate: entry.anchor}))
		_, err := db.NewRaw(`INSERT INTO config.staff_work_schedules
   (tenant_id, staff_id, week_index, rotation_length, day_of_week, target_minutes, valid_from)
   VALUES (?, ?, 0, 1, 0, 300, '2026-01-01')`, entry.school, entry.id).Exec(context.Background())
		require.NoError(t, err)
	}
	ids := []int64{staff.ID, foreignStaff.ID}
	for _, entry := range []struct {
		ctx                     context.Context
		school, visible, hidden int64
		anchor                  string
	}{
		{testpkg.Ctx(t), testpkg.Tenant(t), staff.ID, foreignStaff.ID, "2026-01-05"},
		{foreign.Context(), foreign.TenantID, foreignStaff.ID, staff.ID, "2026-01-12"},
	} {
		require.NoError(t, testpkg.WithTenantTx(t, entry.ctx, db, entry.school, func(ctx context.Context, tx bun.Tx) error {
			var bypass bool
			require.NoError(t, tx.NewRaw("SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(ctx, &bypass))
			require.False(t, bypass)
			rows, err := capability.StaffPlanningProfiles(ctx, ids, "2026-01-01", "2026-01-31")
			require.NoError(t, err)
			profiles := make(map[int64]workforce.StaffPlanningProfile, len(rows))
			for _, row := range rows {
				profiles[row.MembershipID] = row
			}
			require.Equal(t, entry.anchor, profiles[entry.visible].RotationAnchorDate)
			require.Len(t, profiles[entry.visible].Schedules, 1)
			require.Equal(t, entry.school, profiles[entry.visible].Schedules[0].TenantID)
			require.Empty(t, profiles[entry.hidden].RotationAnchorDate)
			require.Empty(t, profiles[entry.hidden].Schedules)
			return nil
		}))
	}
	_, err := capability.StaffPlanningProfiles(context.Background(), ids, "2026-01-01", "2026-01-31")
	require.Error(t, err)
}
