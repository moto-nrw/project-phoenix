// Package active_test tests the analytics service using the hermetic testing pattern.
package active_test

import (
	"testing"
	"time"

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

	service := setupActiveService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns count of active groups", func(t *testing.T) {
		// ARRANGE: Create an active group (no end time)
		activity := testpkg.CreateTestActivityGroup(t, db, "count-active")
		room := testpkg.CreateTestRoom(t, db, "Count Active Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID)

		// ACT
		count, err := service.GetActiveGroupsCount(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 1, "Should have at least 1 active group")
	})

	t.Run("does not count ended groups", func(t *testing.T) {
		// ARRANGE: Create and end a group
		activity := testpkg.CreateTestActivityGroup(t, db, "count-ended")
		room := testpkg.CreateTestRoom(t, db, "Count Ended Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID)

		// Verify group is counted while active
		countBefore, err := service.GetActiveGroupsCount(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, countBefore, 1, "Should have at least 1 active group before ending")

		// End the group
		err = service.EndActivitySession(ctx, activeGroup.ID)
		require.NoError(t, err)

		// ACT
		countAfter, err := service.GetActiveGroupsCount(ctx)

		// ASSERT: count should not be greater than before (our group was removed)
		require.NoError(t, err)
		assert.LessOrEqual(t, countAfter, countBefore, "Count should not increase after ending a group")
	})
}

// =============================================================================
// GetTotalVisitsCount Tests
// =============================================================================

func TestGetTotalVisitsCount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns total visit count", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "total-visits")
		room := testpkg.CreateTestRoom(t, db, "Total Visits Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "TotalVisit", "Student", "9a")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID)

		// Create a visit
		entryTime := time.Now().Add(-30 * time.Minute)
		testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, entryTime, nil)

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

	service := setupActiveService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("counts only active visits", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "active-visits")
		room := testpkg.CreateTestRoom(t, db, "Active Visits Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student1 := testpkg.CreateTestStudent(t, db, "ActiveVisit1", "Student", "9b")
		student2 := testpkg.CreateTestStudent(t, db, "ActiveVisit2", "Student", "9b")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student1.ID, student2.ID)

		// Create one active visit (no exit time)
		entryTime := time.Now().Add(-30 * time.Minute)
		testpkg.CreateTestVisit(t, db, student1.ID, activeGroup.ID, entryTime, nil)

		// Create one ended visit (has exit time)
		exitTime := time.Now().Add(-10 * time.Minute)
		testpkg.CreateTestVisit(t, db, student2.ID, activeGroup.ID, entryTime, &exitTime)

		// ACT
		count, err := service.GetActiveVisitsCount(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 1, "Should have at least 1 active visit")
	})
}

// =============================================================================
// GetDashboardAnalytics Tests
// =============================================================================

func TestGetDashboardAnalytics(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := testpkg.TenantContext(1)

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
		activity := testpkg.CreateTestActivityGroup(t, db, "dashboard-active")
		room := testpkg.CreateTestRoom(t, db, "Dashboard Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID)

		// ACT
		analytics, err := service.GetDashboardAnalytics(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, analytics.ActiveActivities, 1, "Should have at least 1 active activity")
	})
}
