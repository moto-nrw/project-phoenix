// Package active_test tests the analytics service using the hermetic testing pattern.
package active_test

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
