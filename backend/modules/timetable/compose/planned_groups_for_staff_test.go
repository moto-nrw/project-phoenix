package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleListsGroupsAStaffMemberIsPlannedToSupervise(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Planned", "Supervisor")
	colleague := testpkg.CreateTestStaff(t, db, "Planned", "Colleague")
	weekly := testpkg.CreateTestActivityGroup(t, db, "Planned weekly group")
	expired := testpkg.CreateTestActivityGroup(t, db, "Planned expired group")
	colleagues := testpkg.CreateTestActivityGroup(t, db, "Planned colleague group")

	createOwnedSupervisor(t, module, ctx, staff.ID, weekly.ID, true)
	weekday := 3
	_, err := module.CreatePlannedSupervisor(ctx, timetable.PlannedSupervisorInput{
		StaffID: staff.ID, GroupID: weekly.ID, ValidFrom: "2026-09-01", Weekday: &weekday,
	})
	require.NoError(t, err)
	// Legacy parity: every planned row counts, whatever its validity window.
	validUntil := "2026-09-02"
	_, err = module.CreatePlannedSupervisor(ctx, timetable.PlannedSupervisorInput{
		StaffID: staff.ID, GroupID: expired.ID, ValidFrom: "2026-09-01", ValidUntil: &validUntil,
	})
	require.NoError(t, err)
	createOwnedSupervisor(t, module, ctx, colleague.ID, colleagues.ID, true)

	groups, err := module.ListPlannedGroupsForStaff(ctx, staff.ID)
	require.NoError(t, err)
	ids := make([]int64, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
		assert.Equal(t, testpkg.Tenant(t), group.TenantID)
	}
	assert.ElementsMatch(t, []int64{weekly.ID, expired.ID}, ids)

	none, err := module.ListPlannedGroupsForStaff(ctx, testpkg.CreateTestStaff(t, db, "Unplanned", "Staff").ID)
	require.NoError(t, err)
	assert.NotNil(t, none)
	assert.Empty(t, none)

	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)
	foreign, err := module.ListPlannedGroupsForStaff(foreignCtx, staff.ID)
	require.NoError(t, err)
	assert.Empty(t, foreign)

	_, err = module.ListPlannedGroupsForStaff(ctx, 0)
	require.ErrorIs(t, err, timetable.ErrInvalidPlannedSupervisorQuery)
}
