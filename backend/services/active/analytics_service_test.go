// Package active_test tests the analytics service using the hermetic testing pattern.
package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// GetActiveGroupsCount Tests
// =============================================================================

func TestGetActiveGroupsCount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := setupActiveService(t, db)

	// Create context with tenant info
	tc := &tenant.TenantContext{
		OrgID:   ogsID,
		OrgName: "Test OGS",
	}
	ctx := tenant.SetTenantContext(context.Background(), tc)

	t.Run("returns count of active groups", func(t *testing.T) {
		// ARRANGE: Create an active group (no end time)
		activity := testpkg.CreateTestActivityGroup(t, db, "count-active", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Count Active Room", ogsID)
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID, ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID)

		// ACT
		count, err := service.GetActiveGroupsCount(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 1, "Should have at least 1 active group")
	})

	t.Run("does not count ended groups", func(t *testing.T) {
		// ARRANGE: Create and end a group
		activity := testpkg.CreateTestActivityGroup(t, db, "count-ended", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Count Ended Room", ogsID)
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID, ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID)

		// Get count before ending
		countBefore, err := service.GetActiveGroupsCount(ctx)
		require.NoError(t, err)

		// End the group
		err = service.EndActivitySession(ctx, activeGroup.ID)
		require.NoError(t, err)

		// ACT
		countAfter, err := service.GetActiveGroupsCount(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, countBefore-1, countAfter, "Count should decrease by 1 after ending group")
	})
}

// =============================================================================
// GetTotalVisitsCount Tests
// =============================================================================

func TestGetTotalVisitsCount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := setupActiveService(t, db)

	// Create context with tenant info
	tc := &tenant.TenantContext{
		OrgID:   ogsID,
		OrgName: "Test OGS",
	}
	ctx := tenant.SetTenantContext(context.Background(), tc)

	t.Run("returns total visit count", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "total-visits", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Total Visits Room", ogsID)
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID, ogsID)
		student := testpkg.CreateTestStudent(t, db, "TotalVisit", "Student", "9a", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID)

		// Create a visit
		entryTime := time.Now().Add(-30 * time.Minute)
		testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, entryTime, nil, ogsID)

		// ACT
		count, err := service.GetTotalVisitsCount(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 1, "Should have at least 1 visit")
	})
}

// =============================================================================
// GetActiveVisitsCount Tests
// =============================================================================

func TestGetActiveVisitsCount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := setupActiveService(t, db)

	// Create context with tenant info
	tc := &tenant.TenantContext{
		OrgID:   ogsID,
		OrgName: "Test OGS",
	}
	ctx := tenant.SetTenantContext(context.Background(), tc)

	t.Run("counts only active visits", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "active-visits", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Active Visits Room", ogsID)
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID, ogsID)
		student1 := testpkg.CreateTestStudent(t, db, "ActiveVisit1", "Student", "9b", ogsID)
		student2 := testpkg.CreateTestStudent(t, db, "ActiveVisit2", "Student", "9b", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student1.ID, student2.ID)

		// Create one active visit (no exit time)
		entryTime := time.Now().Add(-30 * time.Minute)
		testpkg.CreateTestVisit(t, db, student1.ID, activeGroup.ID, entryTime, nil, ogsID)

		// Create one ended visit (has exit time)
		exitTime := time.Now().Add(-10 * time.Minute)
		testpkg.CreateTestVisit(t, db, student2.ID, activeGroup.ID, entryTime, &exitTime, ogsID)

		// ACT
		count, err := service.GetActiveVisitsCount(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 1, "Should have at least 1 active visit")
	})
}

// =============================================================================
// GetRoomUtilization Tests
// =============================================================================

func TestGetRoomUtilization(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := setupActiveService(t, db)

	// Create context with tenant info
	tc := &tenant.TenantContext{
		OrgID:   ogsID,
		OrgName: "Test OGS",
	}
	ctx := tenant.SetTenantContext(context.Background(), tc)

	t.Run("returns utilization ratio for room with capacity", func(t *testing.T) {
		// ARRANGE: Create room with capacity
		activity := testpkg.CreateTestActivityGroup(t, db, "util-activity", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Utilization Room", ogsID)
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID, ogsID)
		student := testpkg.CreateTestStudent(t, db, "Util", "Student", "10a", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID)

		// Create active visit in this room
		entryTime := time.Now().Add(-30 * time.Minute)
		testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, entryTime, nil, ogsID)

		// ACT
		utilization, err := service.GetRoomUtilization(ctx, room.ID)

		// ASSERT
		require.NoError(t, err)
		// Room has default capacity of 30, with 1 student = 1/30 ~= 0.033
		assert.GreaterOrEqual(t, utilization, 0.0)
		assert.LessOrEqual(t, utilization, 1.0)
	})

	t.Run("returns 0 for room without capacity", func(t *testing.T) {
		// ARRANGE: Create room and update capacity to nil
		room := testpkg.CreateTestRoom(t, db, "No Capacity Room", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)

		// Update room to have no capacity
		_, err := db.NewUpdate().
			Model(room).
			ModelTableExpr(`facilities.rooms`).
			Set("capacity = NULL").
			Where("id = ?", room.ID).
			Exec(ctx)
		require.NoError(t, err)

		// ACT
		utilization, err := service.GetRoomUtilization(ctx, room.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, 0.0, utilization, "Should return 0 for room without capacity")
	})

	t.Run("returns 0 for room with zero capacity", func(t *testing.T) {
		// ARRANGE: Create room and set capacity to 0
		room := testpkg.CreateTestRoom(t, db, "Zero Capacity Room", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)

		// Update room to have zero capacity
		_, err := db.NewUpdate().
			Model(room).
			ModelTableExpr(`facilities.rooms`).
			Set("capacity = 0").
			Where("id = ?", room.ID).
			Exec(ctx)
		require.NoError(t, err)

		// ACT
		utilization, err := service.GetRoomUtilization(ctx, room.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, 0.0, utilization, "Should return 0 for room with zero capacity")
	})

	t.Run("returns error for non-existent room", func(t *testing.T) {
		// ACT
		_, err := service.GetRoomUtilization(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// GetStudentAttendanceRate Tests
// =============================================================================

func TestGetStudentAttendanceRate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := setupActiveService(t, db)

	// Create context with tenant info
	tc := &tenant.TenantContext{
		OrgID:   ogsID,
		OrgName: "Test OGS",
	}
	ctx := tenant.SetTenantContext(context.Background(), tc)

	t.Run("returns 1.0 for student with active visit", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "rate-active", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Rate Active Room", ogsID)
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID, ogsID)
		student := testpkg.CreateTestStudent(t, db, "RateActive", "Student", "11a", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID)

		// Create active visit
		entryTime := time.Now().Add(-30 * time.Minute)
		testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, entryTime, nil, ogsID)

		// ACT
		rate, err := service.GetStudentAttendanceRate(ctx, student.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, 1.0, rate, "Should return 1.0 for student with active visit")
	})

	t.Run("returns 0.0 for student without active visit", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "RateInactive", "Student", "11b", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// ACT
		rate, err := service.GetStudentAttendanceRate(ctx, student.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, 0.0, rate, "Should return 0.0 for student without active visit")
	})

	t.Run("returns 0.0 for student with ended visit", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "rate-ended", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Rate Ended Room", ogsID)
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID, ogsID)
		student := testpkg.CreateTestStudent(t, db, "RateEnded", "Student", "11c", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID)

		// Create ended visit
		entryTime := time.Now().Add(-2 * time.Hour)
		exitTime := time.Now().Add(-1 * time.Hour)
		testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, entryTime, &exitTime, ogsID)

		// ACT
		rate, err := service.GetStudentAttendanceRate(ctx, student.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, 0.0, rate, "Should return 0.0 for student with ended visit")
	})
}

// =============================================================================
// GetDashboardAnalytics Tests
// =============================================================================

func TestGetDashboardAnalytics(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := setupActiveService(t, db)

	// Create context with tenant info
	tc := &tenant.TenantContext{
		OrgID:   ogsID,
		OrgName: "Test OGS",
	}
	ctx := tenant.SetTenantContext(context.Background(), tc)

	t.Run("returns dashboard analytics without error", func(t *testing.T) {
		// ACT
		analytics, err := service.GetDashboardAnalytics(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, analytics)
		assert.False(t, analytics.LastUpdated.IsZero(), "LastUpdated should be set")
		assert.GreaterOrEqual(t, analytics.TotalRooms, 0)
	})

	t.Run("includes active groups in analytics", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "dashboard-active", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Dashboard Room", ogsID)
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID, ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID)

		// ACT
		analytics, err := service.GetDashboardAnalytics(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, analytics.ActiveActivities, 1, "Should have at least 1 active activity")
	})
}
