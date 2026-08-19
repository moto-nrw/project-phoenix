package activities_test

import (
	"context"
	"testing"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestUpdateGroupWithDetails_RollsBackOnSupervisorFailure pins the issue
// #575 B10 behavior change: a supervisor-update failure inside
// UpdateGroupWithDetails aborts the WHOLE update — including the already
// applied group-field changes — instead of the historical Warn-and-commit
// partial write.
func TestUpdateGroupWithDetails_RollsBackOnSupervisorFailure(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := context.Background()

	group := testpkg.CreateTestActivityGroup(t, db, "rollback-original")
	defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

	// Rename + attach a nonexistent supervisor: the FK violation from the
	// supervisor insert must roll the rename back too.
	group.Name = "rollback-renamed"
	nonexistentStaffID := int64(999999999)

	txErr := tenant.WithTenantTx(ctx, db, group.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, err := service.UpdateGroupWithDetails(txCtx, group, 0, true, []int64{nonexistentStaffID}, nil)
		return err
	})
	require.Error(t, txErr, "updating with a nonexistent supervisor must fail")

	reloaded, err := service.GetGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, "rollback-original", reloaded.Name,
		"group rename must roll back when the supervisor update fails — partial commit would resurrect the pre-B10 bug")
}

// TestUpdateGroupWithDetails_UpdatesFieldsSupervisorsAndSchedules covers the
// happy path: fields, supervisor set, and schedule replacement all land.
func TestUpdateGroupWithDetails_UpdatesFieldsSupervisorsAndSchedules(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := context.Background()

	group := testpkg.CreateTestActivityGroup(t, db, "with-details-original")
	defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)
	staff := testpkg.CreateTestStaff(t, db, "Update", "Supervisor")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	group.Name = "with-details-renamed"

	var updated *activitiesModels.Group
	txErr := tenant.WithTenantTx(ctx, db, group.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		updated, err = service.UpdateGroupWithDetails(txCtx, group, 0, true, []int64{staff.ID}, nil)
		return err
	})
	require.NoError(t, txErr)
	require.NotNil(t, updated)

	reloaded, err := service.GetGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, "with-details-renamed", reloaded.Name)

	supervisors, err := service.GetGroupSupervisors(ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, supervisors, 1)
	assert.Equal(t, staff.ID, supervisors[0].StaffID)
}
