// Package active_test tests the active group service layer with hermetic testing pattern.
package active_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// createActiveService creates an Active Service with real database connection
func createActiveService(t *testing.T, db *bun.DB) active.Service {
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db)
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.Active
}

// =============================================================================
// GetActiveGroup Tests
// =============================================================================

func TestActiveService_GetActiveGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns active group when found", func(t *testing.T) {
		// ARRANGE - create prerequisites
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "get-active-group")
		room := testpkg.CreateTestRoom(t, db, "Test Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, room.ID, activeGroup.ID)

		// ACT
		result, err := service.GetActiveGroup(ctx, activeGroup.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, activeGroup.ID, result.ID)
		assert.Equal(t, activityGroup.ID, result.GroupID)
		assert.Equal(t, room.ID, result.RoomID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetActiveGroup(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns error for invalid ID", func(t *testing.T) {
		// ACT
		result, err := service.GetActiveGroup(ctx, 0)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// GetActiveGroupsByIDs Tests
// =============================================================================

func TestActiveService_GetActiveGroupsByIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns multiple groups by IDs", func(t *testing.T) {
		// ARRANGE
		activity1 := testpkg.CreateTestActivityGroup(t, db, "multi-1")
		activity2 := testpkg.CreateTestActivityGroup(t, db, "multi-2")
		room := testpkg.CreateTestRoom(t, db, "Multi Room")
		group1 := testpkg.CreateTestActiveGroup(t, db, activity1.ID, room.ID)
		group2 := testpkg.CreateTestActiveGroup(t, db, activity2.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity1.ID, activity2.ID, room.ID, group1.ID, group2.ID)

		// ACT
		result, err := service.GetActiveGroupsByIDs(ctx, []int64{group1.ID, group2.ID})

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Contains(t, result, group1.ID)
		assert.Contains(t, result, group2.ID)
	})

	t.Run("returns empty map for empty input", func(t *testing.T) {
		// ACT
		result, err := service.GetActiveGroupsByIDs(ctx, []int64{})

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("returns partial results for mixed valid/invalid IDs", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "partial")
		room := testpkg.CreateTestRoom(t, db, "Partial Room")
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, group.ID)

		// ACT
		result, err := service.GetActiveGroupsByIDs(ctx, []int64{group.ID, 99999999})

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Contains(t, result, group.ID)
	})
}

// =============================================================================
// CreateActiveGroup Tests
// =============================================================================

func TestActiveService_CreateActiveGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("creates active group successfully", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "create-active")
		room := testpkg.CreateTestRoom(t, db, "Create Room")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID)

		now := time.Now()
		group := &activeModels.Group{
			GroupID:        activity.ID,
			RoomID:         room.ID,
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
		}

		// ACT
		err := service.CreateActiveGroup(ctx, group)

		// ASSERT
		require.NoError(t, err)
		assert.Greater(t, group.ID, int64(0))
		defer testpkg.CleanupActivityFixtures(t, db, group.ID)
	})

	t.Run("returns error for nil group", func(t *testing.T) {
		// ACT
		err := service.CreateActiveGroup(ctx, nil)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// UpdateActiveGroup Tests
// =============================================================================

func TestActiveService_UpdateActiveGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("updates active group successfully", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "update-active")
		room := testpkg.CreateTestRoom(t, db, "Update Room")
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, group.ID)

		// Modify group
		group.TimeoutMinutes = 60

		// ACT
		err := service.UpdateActiveGroup(ctx, group)

		// ASSERT
		require.NoError(t, err)

		// Verify update persisted
		updated, err := service.GetActiveGroup(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, 60, updated.TimeoutMinutes)
	})

	t.Run("returns error for nil group", func(t *testing.T) {
		// ACT
		err := service.UpdateActiveGroup(ctx, nil)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for group with zero ID", func(t *testing.T) {
		// ARRANGE
		group := &activeModels.Group{}
		group.ID = 0 // Set ID via embedded base.Model

		// ACT
		err := service.UpdateActiveGroup(ctx, group)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// DeleteActiveGroup Tests
// =============================================================================

func TestActiveService_DeleteActiveGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("deletes active group successfully", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "delete-active")
		room := testpkg.CreateTestRoom(t, db, "Delete Room")
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID)

		// ACT
		err := service.DeleteActiveGroup(ctx, group.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify deletion
		_, err = service.GetActiveGroup(ctx, group.ID)
		require.Error(t, err)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		err := service.DeleteActiveGroup(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for invalid ID", func(t *testing.T) {
		// ACT
		err := service.DeleteActiveGroup(ctx, 0)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// ListActiveGroups Tests
// =============================================================================

func TestActiveService_ListActiveGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns active groups with no options", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "list-active")
		room := testpkg.CreateTestRoom(t, db, "List Room")
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, group.ID)

		// ACT
		result, err := service.ListActiveGroups(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		// Should have at least our group
		assert.GreaterOrEqual(t, len(result), 1)
	})

	t.Run("returns active groups with pagination", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "list-paginated")
		room := testpkg.CreateTestRoom(t, db, "List Paginated Room")
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, group.ID)

		options := base.NewQueryOptions()
		options.WithPagination(1, 10)

		// ACT
		result, err := service.ListActiveGroups(ctx, options)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.LessOrEqual(t, len(result), 10)
	})
}

// =============================================================================
// FindActiveGroupsByRoomID Tests
// =============================================================================

func TestActiveService_FindActiveGroupsByRoomID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns groups for room", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "room-find")
		room := testpkg.CreateTestRoom(t, db, "Find Room")
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, group.ID)

		// ACT
		result, err := service.FindActiveGroupsByRoomID(ctx, room.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		// Verify at least one has our room ID
		found := false
		for _, g := range result {
			if g.RoomID == room.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected to find group with room ID")
	})

	t.Run("returns empty list for non-existent room", func(t *testing.T) {
		// ACT
		result, err := service.FindActiveGroupsByRoomID(ctx, 99999999)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// FindActiveGroupsByGroupID Tests
// =============================================================================

func TestActiveService_FindActiveGroupsByGroupID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns active groups for activity group ID", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "group-find")
		room := testpkg.CreateTestRoom(t, db, "Group Find Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID)

		// ACT
		result, err := service.FindActiveGroupsByGroupID(ctx, activity.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		// Verify at least one has our activity group ID
		found := false
		for _, g := range result {
			if g.GroupID == activity.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected to find active group with activity group ID")
	})

	t.Run("returns empty list for non-existent group", func(t *testing.T) {
		// ACT
		result, err := service.FindActiveGroupsByGroupID(ctx, 99999999)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// FindActiveGroupsByTimeRange Tests
// =============================================================================

func TestActiveService_FindActiveGroupsByTimeRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns groups in time range", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "time-range")
		room := testpkg.CreateTestRoom(t, db, "Time Range Room")
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, group.ID)

		// Use a time range that includes the group's start time
		start := time.Now().Add(-1 * time.Hour)
		end := time.Now().Add(1 * time.Hour)

		// ACT
		result, err := service.FindActiveGroupsByTimeRange(ctx, start, end)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})

	t.Run("returns empty list for past time range", func(t *testing.T) {
		// ARRANGE - use a time range so far in the past that no real data could match.
		// This ensures hermeticity: the test passes regardless of what data exists,
		// since no active groups can have started before 1900.
		start := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(1900, 1, 2, 0, 0, 0, 0, time.UTC)

		// ACT
		result, err := service.FindActiveGroupsByTimeRange(ctx, start, end)

		// ASSERT
		require.NoError(t, err)
		// Result may be nil or empty slice - both are valid for "no results"
		assert.Empty(t, result)
	})
}

// =============================================================================
// EndActiveGroupSession Tests
// =============================================================================

func TestActiveService_EndActiveGroupSession(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("ends session successfully", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "end-session")
		room := testpkg.CreateTestRoom(t, db, "End Session Room")
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, group.ID)

		// ACT
		err := service.EndActiveGroupSession(ctx, group.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify session ended (end_time set)
		ended, err := service.GetActiveGroup(ctx, group.ID)
		require.NoError(t, err)
		assert.NotNil(t, ended.EndTime)
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		// ACT
		err := service.EndActiveGroupSession(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// GetActiveGroupWithVisits Tests
// =============================================================================

func TestActiveService_GetActiveGroupWithVisits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns group with visits", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "with-visits")
		room := testpkg.CreateTestRoom(t, db, "Visits Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "Visit", "Student", "1a")
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID)

		// ACT
		result, err := service.GetActiveGroupWithVisits(ctx, activeGroup.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, activeGroup.ID, result.ID)
		// Visits relation should be loaded
		assert.NotNil(t, result.Visits)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetActiveGroupWithVisits(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// GetActiveGroupWithSupervisors Tests
// =============================================================================

func TestActiveService_GetActiveGroupWithSupervisors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns group with supervisors", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "with-supervisors")
		room := testpkg.CreateTestRoom(t, db, "Supervisors Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, db, "Super", "Visor")
		supervisor := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, activeGroup.ID, "supervisor")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, staff.ID, supervisor.ID)

		// ACT
		result, err := service.GetActiveGroupWithSupervisors(ctx, activeGroup.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, activeGroup.ID, result.ID)
		// Supervisors relation should be loaded with the supervisor we created
		require.NotNil(t, result.Supervisors)
		assert.Len(t, result.Supervisors, 1)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetActiveGroupWithSupervisors(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// Analytics and Statistics Tests
// =============================================================================

func TestActiveService_GetActiveGroupsCount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns count of active groups", func(t *testing.T) {
		// ARRANGE - create an active group
		activity := testpkg.CreateTestActivityGroup(t, db, "count-active")
		room := testpkg.CreateTestRoom(t, db, "Count Room")
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, group.ID)

		// ACT
		count, err := service.GetActiveGroupsCount(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 1)
	})
}

func TestActiveService_GetTotalVisitsCount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns count of total visits", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "visit-count")
		room := testpkg.CreateTestRoom(t, db, "Visit Count Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "Visit", "Count", "1a")
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID)

		// ACT
		count, err := service.GetTotalVisitsCount(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 1)
	})
}

func TestActiveService_GetActiveVisitsCount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns count of active visits", func(t *testing.T) {
		// ARRANGE - create visit without exit time (active)
		activity := testpkg.CreateTestActivityGroup(t, db, "active-visit")
		room := testpkg.CreateTestRoom(t, db, "Active Visit Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "Active", "Visit", "1a")
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID)

		// ACT
		count, err := service.GetActiveVisitsCount(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 1)
	})
}

func TestActiveService_GetRoomUtilization(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns room utilization", func(t *testing.T) {
		// ARRANGE
		room := testpkg.CreateTestRoom(t, db, "Utilization Room")
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)

		// ACT
		utilization, err := service.GetRoomUtilization(ctx, room.ID)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, utilization, 0.0)
		assert.LessOrEqual(t, utilization, 1.0) // 0-100% as decimal
	})
}

func TestActiveService_GetDashboardAnalytics(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns dashboard analytics", func(t *testing.T) {
		// ACT
		result, err := service.GetDashboardAnalytics(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		// Verify structure has required fields
		assert.GreaterOrEqual(t, result.StudentsPresent, 0)
		assert.GreaterOrEqual(t, result.ActiveActivities, 0)
		assert.NotZero(t, result.LastUpdated)
	})
}

// =============================================================================
// Unclaimed Groups Tests
// =============================================================================

func TestActiveService_GetUnclaimedActiveGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns unclaimed groups", func(t *testing.T) {
		// ARRANGE - create group without supervisors (unclaimed)
		activity := testpkg.CreateTestActivityGroup(t, db, "unclaimed")
		room := testpkg.CreateTestRoom(t, db, "Unclaimed Room")
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, group.ID)

		// ACT
		_, err := service.GetUnclaimedActiveGroups(ctx)

		// ASSERT
		require.NoError(t, err)
		// Result may be nil or empty slice if no unclaimed groups exist
		// Just verify no error - the repository correctly handles this case
	})
}

func TestActiveService_ClaimActiveGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("claims group successfully", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "claim-group")
		room := testpkg.CreateTestRoom(t, db, "Claim Room")
		group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, db, "Claim", "Staff")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, group.ID, staff.ID)

		// ACT
		supervisor, err := service.ClaimActiveGroup(ctx, group.ID, staff.ID, "supervisor")

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, supervisor)
		assert.Equal(t, staff.ID, supervisor.StaffID)
		assert.Equal(t, group.ID, supervisor.GroupID)
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		// ARRANGE
		staff := testpkg.CreateTestStaff(t, db, "Claim", "NoGroup")
		defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

		// ACT
		result, err := service.ClaimActiveGroup(ctx, 99999999, staff.ID, "supervisor")

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// CleanupAbandonedSessions Tests
// =============================================================================

func TestActiveService_CleanupAbandonedSessions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("cleans up old sessions", func(t *testing.T) {
		// ACT - cleanup sessions older than 24 hours
		count, err := service.CleanupAbandonedSessions(ctx, 24*time.Hour)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})

	t.Run("handles no abandoned sessions", func(t *testing.T) {
		// ACT - use very short threshold (nothing should match)
		count, err := service.CleanupAbandonedSessions(ctx, 1*time.Nanosecond)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})
}

// =============================================================================
// Transaction Support Tests
// =============================================================================

func TestActiveService_WithTx(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns service instance with transaction", func(t *testing.T) {
		// ARRANGE
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		// ACT
		txService := service.WithTx(tx)

		// ASSERT - should return a service that implements the interface
		require.NotNil(t, txService)
		_, ok := txService.(active.Service)
		assert.True(t, ok, "WithTx should return a Service interface")
	})
}

// Helper for unique test names
func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// =============================================================================
// Student Attendance Rate Tests
// =============================================================================

func TestActiveService_GetStudentAttendanceRate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns 0 when student has no active visit", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "Rate", "NoVisit", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// ACT
		rate, err := service.GetStudentAttendanceRate(ctx, student.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, 0.0, rate)
	})

	t.Run("returns 1.0 when student has active visit", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "Rate", "Active", "1a")
		staff := testpkg.CreateTestStaff(t, db, "Rate", "Staff")
		iotDevice := testpkg.CreateTestDevice(t, db, fmt.Sprintf("rate-device-%d", time.Now().UnixNano()))
		activity := testpkg.CreateTestActivityGroup(t, db, "rate-activity")
		room := testpkg.CreateTestRoom(t, db, "Rate Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID, iotDevice.ID, activity.ID, room.ID, activeGroup.ID)

		// Create visit using context with device/staff
		visit := &activeModels.Visit{
			StudentID:     student.ID,
			ActiveGroupID: activeGroup.ID,
			EntryTime:     time.Now(),
		}
		staffCtx := context.WithValue(ctx, device.CtxStaff, staff)
		deviceCtx := context.WithValue(staffCtx, device.CtxDevice, iotDevice)
		err := service.CreateVisit(deviceCtx, visit)
		require.NoError(t, err)

		// ACT
		rate, err := service.GetStudentAttendanceRate(ctx, student.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, 1.0, rate)
	})

	t.Run("returns 0 for non-existent student", func(t *testing.T) {
		// ACT
		rate, err := service.GetStudentAttendanceRate(ctx, 99999999)

		// ASSERT - should not error, just return 0
		require.NoError(t, err)
		assert.Equal(t, 0.0, rate)
	})
}

// =============================================================================
// Activity Session with Supervisors Tests
// =============================================================================

func TestActiveService_StartActivitySessionWithSupervisors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("starts session with multiple supervisors", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, uniqueName("session-supervisors"))
		room := testpkg.CreateTestRoom(t, db, uniqueName("Session Room"))
		device := testpkg.CreateTestDevice(t, db, uniqueName("session-device"))
		staff1 := testpkg.CreateTestStaff(t, db, "Session", "Supervisor1")
		staff2 := testpkg.CreateTestStaff(t, db, "Session", "Supervisor2")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, device.ID, staff1.ID, staff2.ID)

		roomID := room.ID

		// ACT
		result, err := service.StartActivitySessionWithSupervisors(ctx, activity.ID, device.ID, []int64{staff1.ID, staff2.ID}, &roomID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Greater(t, result.ID, int64(0))
	})

	t.Run("returns error for empty supervisor list", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, uniqueName("empty-supervisors"))
		device := testpkg.CreateTestDevice(t, db, uniqueName("empty-device"))
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, device.ID)

		// ACT
		result, err := service.StartActivitySessionWithSupervisors(ctx, activity.ID, device.ID, []int64{}, nil)

		// ASSERT - at least one supervisor required
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns error for non-existent supervisor", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, uniqueName("bad-supervisor"))
		device := testpkg.CreateTestDevice(t, db, uniqueName("bad-device"))
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, device.ID)

		// ACT
		result, err := service.StartActivitySessionWithSupervisors(ctx, activity.ID, device.ID, []int64{99999999}, nil)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// Session Timeout Tests
// =============================================================================

func TestActiveService_ProcessSessionTimeout(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns error when device has no active session", func(t *testing.T) {
		// ARRANGE
		device := testpkg.CreateTestDevice(t, db, uniqueName("timeout-no-session"))
		defer testpkg.CleanupActivityFixtures(t, db, device.ID)

		// ACT
		result, err := service.ProcessSessionTimeout(ctx, device.ID)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("processes timeout for active session", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, uniqueName("timeout-activity"))
		room := testpkg.CreateTestRoom(t, db, uniqueName("Timeout Room"))
		device := testpkg.CreateTestDevice(t, db, uniqueName("timeout-device"))
		staff := testpkg.CreateTestStaff(t, db, "Timeout", "Supervisor")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, device.ID, staff.ID)

		roomID := room.ID

		// Start a session
		session, err := service.StartActivitySessionWithSupervisors(ctx, activity.ID, device.ID, []int64{staff.ID}, &roomID)
		require.NoError(t, err)
		require.NotNil(t, session)

		// ACT
		result, err := service.ProcessSessionTimeout(ctx, device.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
	})
}

// =============================================================================
// Session Timeout Validation Tests
// =============================================================================

func TestActiveService_ValidateSessionTimeout(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns error when device has no active session", func(t *testing.T) {
		// ARRANGE
		iotDevice := testpkg.CreateTestDevice(t, db, uniqueName("validate-no-session"))
		defer testpkg.CleanupActivityFixtures(t, db, iotDevice.ID)

		// ACT
		err := service.ValidateSessionTimeout(ctx, iotDevice.ID, 30)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error when session has not timed out yet", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, uniqueName("validate-timeout"))
		room := testpkg.CreateTestRoom(t, db, uniqueName("Validate Room"))
		iotDevice := testpkg.CreateTestDevice(t, db, uniqueName("validate-device"))
		staff := testpkg.CreateTestStaff(t, db, "Validate", "Supervisor")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, iotDevice.ID, staff.ID)

		roomID := room.ID

		// Start a session (fresh session - not timed out)
		session, err := service.StartActivitySessionWithSupervisors(ctx, activity.ID, iotDevice.ID, []int64{staff.ID}, &roomID)
		require.NoError(t, err)
		require.NotNil(t, session)

		// ACT - validate with a 30 minute timeout (fresh session won't be timed out)
		err = service.ValidateSessionTimeout(ctx, iotDevice.ID, 30)

		// ASSERT - should error because session is active (not timed out)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not yet timed out")
	})
}

func TestActiveService_GetSessionTimeoutInfo(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("returns error when device has no active session", func(t *testing.T) {
		// ARRANGE
		iotDevice := testpkg.CreateTestDevice(t, db, uniqueName("info-no-session"))
		defer testpkg.CleanupActivityFixtures(t, db, iotDevice.ID)

		// ACT
		info, err := service.GetSessionTimeoutInfo(ctx, iotDevice.ID)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, info)
	})

	t.Run("returns timeout info for active session", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, uniqueName("info-timeout"))
		room := testpkg.CreateTestRoom(t, db, uniqueName("Info Room"))
		iotDevice := testpkg.CreateTestDevice(t, db, uniqueName("info-device"))
		staff := testpkg.CreateTestStaff(t, db, "Info", "Supervisor")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, iotDevice.ID, staff.ID)

		roomID := room.ID

		// Start a session
		session, err := service.StartActivitySessionWithSupervisors(ctx, activity.ID, iotDevice.ID, []int64{staff.ID}, &roomID)
		require.NoError(t, err)
		require.NotNil(t, session)

		// ACT
		info, err := service.GetSessionTimeoutInfo(ctx, iotDevice.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, info)
		assert.Equal(t, session.ID, info.SessionID)
	})
}

// =============================================================================
// Daily Session Cleanup Tests
// =============================================================================

func TestActiveService_EndDailySessions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("ends daily sessions successfully", func(t *testing.T) {
		// ACT
		result, err := service.EndDailySessions(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
	})
}

// =============================================================================
// Force Start Session Tests
// =============================================================================

func TestActiveService_ForceStartActivitySession(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := context.Background()

	t.Run("force starts session for activity", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, uniqueName("force-start"))
		room := testpkg.CreateTestRoom(t, db, uniqueName("Force Room"))
		iotDevice := testpkg.CreateTestDevice(t, db, uniqueName("force-device"))
		staff := testpkg.CreateTestStaff(t, db, "Force", "Supervisor")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, iotDevice.ID, staff.ID)

		roomID := room.ID

		// ACT
		result, err := service.ForceStartActivitySession(ctx, activity.ID, iotDevice.ID, staff.ID, &roomID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Greater(t, result.ID, int64(0))
	})

	t.Run("force starts session even when device has existing session", func(t *testing.T) {
		// ARRANGE
		activity1 := testpkg.CreateTestActivityGroup(t, db, uniqueName("force-existing1"))
		activity2 := testpkg.CreateTestActivityGroup(t, db, uniqueName("force-existing2"))
		room := testpkg.CreateTestRoom(t, db, uniqueName("Force Existing Room"))
		iotDevice := testpkg.CreateTestDevice(t, db, uniqueName("force-existing-device"))
		staff := testpkg.CreateTestStaff(t, db, "Force", "Existing")
		defer testpkg.CleanupActivityFixtures(t, db, activity1.ID, activity2.ID, room.ID, iotDevice.ID, staff.ID)

		roomID := room.ID

		// Start first session
		session1, err := service.StartActivitySessionWithSupervisors(ctx, activity1.ID, iotDevice.ID, []int64{staff.ID}, &roomID)
		require.NoError(t, err)
		require.NotNil(t, session1)

		// ACT - Force start a new session (should end the first one)
		result, err := service.ForceStartActivitySession(ctx, activity2.ID, iotDevice.ID, staff.ID, &roomID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEqual(t, session1.ID, result.ID)
	})
}

