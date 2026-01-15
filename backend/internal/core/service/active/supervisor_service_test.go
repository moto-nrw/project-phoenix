// Package active_test tests the supervisor operations in active service layer.
package active_test

import (
	"context"
	"testing"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/service/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// GetGroupSupervisor Tests
// =============================================================================

func TestActiveService_GetGroupSupervisor(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns supervisor when found", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "get-supervisor")
		room := testpkg.CreateTestRoom(t, db, "Supervisor Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, db, "Get", "Supervisor")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, staff.ID)

		// Create supervisor
		now := time.Now()
		supervisor := &activeModels.GroupSupervisor{
			GroupID:   activeGroup.ID,
			StaffID:   staff.ID,
			Role:      "supervisor",
			StartDate: now,
		}
		err := service.CreateGroupSupervisor(ctx, supervisor)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, supervisor.ID)

		// ACT
		result, err := service.GetGroupSupervisor(ctx, supervisor.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, supervisor.ID, result.ID)
		assert.Equal(t, staff.ID, result.StaffID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetGroupSupervisor(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns error for invalid ID", func(t *testing.T) {
		// ACT
		result, err := service.GetGroupSupervisor(ctx, 0)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// CreateGroupSupervisor Tests
// =============================================================================

func TestActiveService_CreateGroupSupervisor(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("creates supervisor successfully", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "create-supervisor")
		room := testpkg.CreateTestRoom(t, db, "Create Supervisor Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, db, "Create", "Supervisor")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, staff.ID)

		now := time.Now()
		supervisor := &activeModels.GroupSupervisor{
			GroupID:   activeGroup.ID,
			StaffID:   staff.ID,
			Role:      "supervisor",
			StartDate: now,
		}

		// ACT
		err := service.CreateGroupSupervisor(ctx, supervisor)

		// ASSERT
		require.NoError(t, err)
		assert.Greater(t, supervisor.ID, int64(0))
		defer testpkg.CleanupActivityFixtures(t, db, supervisor.ID)
	})

	t.Run("returns error for nil supervisor", func(t *testing.T) {
		// ACT
		err := service.CreateGroupSupervisor(ctx, nil)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for invalid group ID", func(t *testing.T) {
		// ARRANGE
		staff := testpkg.CreateTestStaff(t, db, "Invalid", "Group")
		defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

		now := time.Now()
		supervisor := &activeModels.GroupSupervisor{
			GroupID:   99999999, // invalid
			StaffID:   staff.ID,
			Role:      "supervisor",
			StartDate: now,
		}

		// ACT
		err := service.CreateGroupSupervisor(ctx, supervisor)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// UpdateGroupSupervisor Tests
// =============================================================================

func TestActiveService_UpdateGroupSupervisor(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("updates supervisor successfully", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "update-supervisor")
		room := testpkg.CreateTestRoom(t, db, "Update Supervisor Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, db, "Update", "Supervisor")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, staff.ID)

		now := time.Now()
		supervisor := &activeModels.GroupSupervisor{
			GroupID:   activeGroup.ID,
			StaffID:   staff.ID,
			Role:      "supervisor",
			StartDate: now,
		}
		err := service.CreateGroupSupervisor(ctx, supervisor)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, supervisor.ID)

		// Update role
		supervisor.Role = "primary_supervisor"

		// ACT
		err = service.UpdateGroupSupervisor(ctx, supervisor)

		// ASSERT
		require.NoError(t, err)

		// Verify update persisted
		updated, err := service.GetGroupSupervisor(ctx, supervisor.ID)
		require.NoError(t, err)
		assert.Equal(t, "primary_supervisor", updated.Role)
	})

	t.Run("returns error for nil supervisor", func(t *testing.T) {
		// ACT
		err := service.UpdateGroupSupervisor(ctx, nil)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for supervisor with zero ID", func(t *testing.T) {
		// ARRANGE
		supervisor := &activeModels.GroupSupervisor{}
		supervisor.ID = 0 // Set ID via embedded base.Model

		// ACT
		err := service.UpdateGroupSupervisor(ctx, supervisor)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// DeleteGroupSupervisor Tests
// =============================================================================

func TestActiveService_DeleteGroupSupervisor(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("deletes supervisor successfully", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "delete-supervisor")
		room := testpkg.CreateTestRoom(t, db, "Delete Supervisor Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, db, "Delete", "Supervisor")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, staff.ID)

		now := time.Now()
		supervisor := &activeModels.GroupSupervisor{
			GroupID:   activeGroup.ID,
			StaffID:   staff.ID,
			Role:      "supervisor",
			StartDate: now,
		}
		err := service.CreateGroupSupervisor(ctx, supervisor)
		require.NoError(t, err)

		// ACT
		err = service.DeleteGroupSupervisor(ctx, supervisor.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify deletion
		_, err = service.GetGroupSupervisor(ctx, supervisor.ID)
		require.Error(t, err)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		err := service.DeleteGroupSupervisor(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for invalid ID", func(t *testing.T) {
		// ACT
		err := service.DeleteGroupSupervisor(ctx, 0)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// ListGroupSupervisors Tests
// =============================================================================

func TestActiveService_ListGroupSupervisors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns supervisors with no options", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "list-supervisors")
		room := testpkg.CreateTestRoom(t, db, "List Supervisors Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, db, "List", "Supervisor")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, staff.ID)

		now := time.Now()
		supervisor := &activeModels.GroupSupervisor{
			GroupID:   activeGroup.ID,
			StaffID:   staff.ID,
			Role:      "supervisor",
			StartDate: now,
		}
		err := service.CreateGroupSupervisor(ctx, supervisor)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, supervisor.ID)

		// ACT
		result, err := service.ListGroupSupervisors(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.GreaterOrEqual(t, len(result), 1)
	})

	t.Run("returns supervisors with pagination", func(t *testing.T) {
		// ARRANGE
		options := base.NewQueryOptions()
		options.WithPagination(1, 5)

		// ACT
		result, err := service.ListGroupSupervisors(ctx, options)

		// ASSERT
		require.NoError(t, err)
		assert.LessOrEqual(t, len(result), 5)
	})
}

// =============================================================================
// FindSupervisorsByStaffID Tests
// =============================================================================

func TestActiveService_FindSupervisorsByStaffID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns supervisors for staff", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "staff-supervisors")
		room := testpkg.CreateTestRoom(t, db, "Staff Supervisors Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, db, "Find", "ByStaff")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, staff.ID)

		now := time.Now()
		supervisor := &activeModels.GroupSupervisor{
			GroupID:   activeGroup.ID,
			StaffID:   staff.ID,
			Role:      "supervisor",
			StartDate: now,
		}
		err := service.CreateGroupSupervisor(ctx, supervisor)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, supervisor.ID)

		// ACT
		result, err := service.FindSupervisorsByStaffID(ctx, staff.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		// All supervisors should be for this staff
		for _, s := range result {
			assert.Equal(t, staff.ID, s.StaffID)
		}
	})

	t.Run("returns empty list for staff with no supervisions", func(t *testing.T) {
		// ARRANGE
		staff := testpkg.CreateTestStaff(t, db, "No", "Supervisions")
		defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

		// ACT
		result, err := service.FindSupervisorsByStaffID(ctx, staff.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// FindSupervisorsByActiveGroupID Tests
// =============================================================================

func TestActiveService_FindSupervisorsByActiveGroupID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns supervisors for active group", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "group-supervisors")
		room := testpkg.CreateTestRoom(t, db, "Group Supervisors Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, db, "Find", "ByGroup")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, staff.ID)

		now := time.Now()
		supervisor := &activeModels.GroupSupervisor{
			GroupID:   activeGroup.ID,
			StaffID:   staff.ID,
			Role:      "supervisor",
			StartDate: now,
		}
		err := service.CreateGroupSupervisor(ctx, supervisor)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, supervisor.ID)

		// ACT
		result, err := service.FindSupervisorsByActiveGroupID(ctx, activeGroup.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		// All supervisors should be for this group
		for _, s := range result {
			assert.Equal(t, activeGroup.ID, s.GroupID)
		}
	})

	t.Run("returns empty list for group with no supervisors", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "no-supervisors")
		room := testpkg.CreateTestRoom(t, db, "No Supervisors Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID)

		// ACT
		result, err := service.FindSupervisorsByActiveGroupID(ctx, activeGroup.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// FindSupervisorsByActiveGroupIDs Tests
// =============================================================================

func TestActiveService_FindSupervisorsByActiveGroupIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns supervisors for multiple groups", func(t *testing.T) {
		// ARRANGE
		activity1 := testpkg.CreateTestActivityGroup(t, db, "multi-group-1")
		activity2 := testpkg.CreateTestActivityGroup(t, db, "multi-group-2")
		room := testpkg.CreateTestRoom(t, db, "Multi Groups Room")
		group1 := testpkg.CreateTestActiveGroup(t, db, activity1.ID, room.ID)
		group2 := testpkg.CreateTestActiveGroup(t, db, activity2.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, db, "Multi", "Supervisor")
		defer testpkg.CleanupActivityFixtures(t, db, activity1.ID, activity2.ID, room.ID, group1.ID, group2.ID, staff.ID)

		now := time.Now()
		sup1 := &activeModels.GroupSupervisor{GroupID: group1.ID, StaffID: staff.ID, Role: "supervisor", StartDate: now}
		sup2 := &activeModels.GroupSupervisor{GroupID: group2.ID, StaffID: staff.ID, Role: "supervisor", StartDate: now}
		err := service.CreateGroupSupervisor(ctx, sup1)
		require.NoError(t, err)
		err = service.CreateGroupSupervisor(ctx, sup2)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, sup1.ID, sup2.ID)

		// ACT
		result, err := service.FindSupervisorsByActiveGroupIDs(ctx, []int64{group1.ID, group2.ID})

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(result), 2)
	})

	t.Run("returns empty list for empty input", func(t *testing.T) {
		// ACT
		result, err := service.FindSupervisorsByActiveGroupIDs(ctx, []int64{})

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// EndSupervision Tests
// =============================================================================

func TestActiveService_EndSupervision(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("ends supervision successfully", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "end-supervision")
		room := testpkg.CreateTestRoom(t, db, "End Supervision Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, db, "End", "Supervision")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, staff.ID)

		now := time.Now()
		supervisor := &activeModels.GroupSupervisor{
			GroupID:   activeGroup.ID,
			StaffID:   staff.ID,
			Role:      "supervisor",
			StartDate: now,
		}
		err := service.CreateGroupSupervisor(ctx, supervisor)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, supervisor.ID)

		// ACT
		err = service.EndSupervision(ctx, supervisor.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify end time set
		ended, err := service.GetGroupSupervisor(ctx, supervisor.ID)
		require.NoError(t, err)
		assert.NotNil(t, ended.EndDate)
	})

	t.Run("returns error for non-existent supervision", func(t *testing.T) {
		// ACT
		err := service.EndSupervision(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error on database failure", func(t *testing.T) {
		// ARRANGE - use canceled context to trigger DB error
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		// ACT
		err := service.EndSupervision(canceledCtx, 1)

		// ASSERT
		require.Error(t, err)
		var activeErr *active.ActiveError
		require.ErrorAs(t, err, &activeErr)
	})
}

// =============================================================================
// GetStaffActiveSupervisions Tests
// =============================================================================

func TestActiveService_GetStaffActiveSupervisions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns active supervisions for staff", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "active-supervisions")
		room := testpkg.CreateTestRoom(t, db, "Active Supervisions Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, db, "Active", "Supervisions")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, staff.ID)

		now := time.Now()
		supervisor := &activeModels.GroupSupervisor{
			GroupID:   activeGroup.ID,
			StaffID:   staff.ID,
			Role:      "supervisor",
			StartDate: now,
		}
		err := service.CreateGroupSupervisor(ctx, supervisor)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, supervisor.ID)

		// ACT
		result, err := service.GetStaffActiveSupervisions(ctx, staff.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		// All should be active (no end time)
		for _, s := range result {
			assert.Nil(t, s.EndDate)
		}
	})

	t.Run("returns empty list for staff with no active supervisions", func(t *testing.T) {
		// ARRANGE
		staff := testpkg.CreateTestStaff(t, db, "No", "Active")
		defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

		// ACT
		result, err := service.GetStaffActiveSupervisions(ctx, staff.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// UpdateActiveGroupSupervisors Tests
// =============================================================================

func TestActiveService_UpdateActiveGroupSupervisors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("updates supervisors for group", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "update-supervisors")
		room := testpkg.CreateTestRoom(t, db, "Update Supervisors Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		staff1 := testpkg.CreateTestStaff(t, db, "First", "Supervisor")
		staff2 := testpkg.CreateTestStaff(t, db, "Second", "Supervisor")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, staff1.ID, staff2.ID)

		// ACT - set new supervisors
		result, err := service.UpdateActiveGroupSupervisors(ctx, activeGroup.ID, []int64{staff1.ID, staff2.ID})

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		// ARRANGE
		staff := testpkg.CreateTestStaff(t, db, "Update", "NonExistent")
		defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

		// ACT
		result, err := service.UpdateActiveGroupSupervisors(ctx, 99999999, []int64{staff.ID})

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns error for empty supervisor list", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "empty-supervisors")
		room := testpkg.CreateTestRoom(t, db, "Empty Supervisors Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID)

		// ACT - service requires at least one supervisor
		result, err := service.UpdateActiveGroupSupervisors(ctx, activeGroup.ID, []int64{})

		// ASSERT - should fail because at least one supervisor is required
		require.Error(t, err)
		assert.Nil(t, result)
	})
}
