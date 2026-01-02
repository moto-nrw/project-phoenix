package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/services"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupTestDB creates a test database connection
func setupTestDB(t *testing.T) *bun.DB {
	// Use test database DSN from environment or fallback
	testDSN := viper.GetString("test_db_dsn")
	if testDSN == "" {
		testDSN = viper.GetString("db_dsn")
		if testDSN == "" {
			t.Skip("No test database configured (set TEST_DB_DSN or DB_DSN)")
		}
	}

	// Enable debug mode for tests
	viper.Set("db_debug", true)

	db, err := database.DBConn()
	require.NoError(t, err, "Failed to connect to test database")

	return db
}

// setupActiveService creates an active service with real database connection
func setupActiveService(t *testing.T, db *bun.DB) activeSvc.Service {
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db) // Pass db as second parameter
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.Active
}

// cleanupTestData removes test data from database
func cleanupTestData(t *testing.T, db *bun.DB, groupIDs ...int64) {
	ctx := context.Background()

	// Clean up active groups
	for _, groupID := range groupIDs {
		_, err := db.NewDelete().
			Model((*active.Group)(nil)).
			Where("group_id = ?", groupID).
			Exec(ctx)
		if err != nil {
			t.Logf("Warning: Failed to cleanup test group %d: %v", groupID, err)
		}
	}
}

// TestActivitySessionConflictDetection tests the core conflict detection functionality
func TestActivitySessionConflictDetection(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	}()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("no conflict when activity not active", func(t *testing.T) {
		activityID := int64(1001)
		deviceID := int64(1)
		staffID := int64(1)

		defer cleanupTestData(t, db, activityID)

		// Check for conflicts - should be none
		conflict, err := service.CheckActivityConflict(ctx, deviceID)
		require.NoError(t, err)
		assert.False(t, conflict.HasConflict, "Expected no conflict for inactive activity")

		// Start session - should succeed
		session, err := service.StartActivitySession(ctx, activityID, deviceID, staffID, nil)
		require.NoError(t, err)
		assert.NotNil(t, session)
		assert.Equal(t, activityID, session.GroupID)
		assert.Equal(t, &deviceID, session.DeviceID)
	})

	t.Run("conflict when activity already active on different device", func(t *testing.T) {
		activityID := int64(1002)
		device1ID := int64(1)
		device2ID := int64(2)
		staffID := int64(1)

		defer cleanupTestData(t, db, activityID)

		// Start session on device 1
		session1, err := service.StartActivitySession(ctx, activityID, device1ID, staffID, nil)
		require.NoError(t, err)
		assert.NotNil(t, session1)

		// Check for conflicts on device 2 - should detect conflict
		conflict, err := service.CheckActivityConflict(ctx, device2ID)
		require.NoError(t, err)
		assert.True(t, conflict.HasConflict, "Expected conflict when activity already active")
		assert.Contains(t, conflict.ConflictMessage, "already active")

		// Try to start session on device 2 - should fail
		_, err = service.StartActivitySession(ctx, activityID, device2ID, staffID, nil)
		assert.Error(t, err, "Expected error when starting session on conflicting device")
		assert.Contains(t, err.Error(), "conflict")
	})

	t.Run("conflict when device already running another activity", func(t *testing.T) {
		activity1ID := int64(1003)
		activity2ID := int64(1004)
		deviceID := int64(1)
		staffID := int64(1)

		defer cleanupTestData(t, db, activity1ID, activity2ID)

		// Start session for activity 1 on device
		session1, err := service.StartActivitySession(ctx, activity1ID, deviceID, staffID, nil)
		require.NoError(t, err)
		assert.NotNil(t, session1)

		// Try to start activity 2 on same device - should fail
		_, err = service.StartActivitySession(ctx, activity2ID, deviceID, staffID, nil)
		assert.Error(t, err, "Expected error when device already running another activity")
	})

	t.Run("force override ends existing sessions", func(t *testing.T) {
		activityID := int64(1005)
		device1ID := int64(1)
		device2ID := int64(2)
		staffID := int64(1)

		defer cleanupTestData(t, db, activityID)

		// Start session on device 1
		session1, err := service.StartActivitySession(ctx, activityID, device1ID, staffID, nil)
		require.NoError(t, err)
		assert.NotNil(t, session1)

		// Force start on device 2 - should succeed and end previous session
		session2, err := service.ForceStartActivitySession(ctx, activityID, device2ID, staffID, nil)
		require.NoError(t, err)
		assert.NotNil(t, session2)
		assert.Equal(t, activityID, session2.GroupID)
		assert.Equal(t, &device2ID, session2.DeviceID)

		// Verify first session was ended
		updatedSession1, err := service.GetActiveGroup(ctx, session1.ID)
		require.NoError(t, err)
		assert.NotNil(t, updatedSession1.EndTime, "Expected first session to be ended")
	})

	t.Run("get current session for device", func(t *testing.T) {
		activityID := int64(1006)
		deviceID := int64(1)
		staffID := int64(1)

		defer cleanupTestData(t, db, activityID)

		// Start session
		session, err := service.StartActivitySession(ctx, activityID, deviceID, staffID, nil)
		require.NoError(t, err)

		// Get current session
		currentSession, err := service.GetDeviceCurrentSession(ctx, deviceID)
		require.NoError(t, err)
		assert.NotNil(t, currentSession)
		assert.Equal(t, session.ID, currentSession.ID)
		assert.Equal(t, activityID, currentSession.GroupID)

		// End session
		err = service.EndActivitySession(ctx, session.ID)
		require.NoError(t, err)

		// Verify no current session
		currentSession, err = service.GetDeviceCurrentSession(ctx, deviceID)
		assert.Error(t, err, "Expected error when no active session")
		assert.Nil(t, currentSession)
	})
}

// TestSessionLifecycle tests the basic session lifecycle
func TestSessionLifecycle(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	}()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("complete session lifecycle", func(t *testing.T) {
		activityID := int64(2001)
		deviceID := int64(1)
		staffID := int64(1)

		defer cleanupTestData(t, db, activityID)

		// Start session
		session, err := service.StartActivitySession(ctx, activityID, deviceID, staffID, nil)
		require.NoError(t, err)
		assert.NotNil(t, session)
		assert.Nil(t, session.EndTime, "New session should not have end time")
		assert.Equal(t, &deviceID, session.DeviceID)

		// Verify session is active
		currentSession, err := service.GetDeviceCurrentSession(ctx, deviceID)
		require.NoError(t, err)
		assert.Equal(t, session.ID, currentSession.ID)

		// End session
		err = service.EndActivitySession(ctx, session.ID)
		require.NoError(t, err)

		// Verify session is ended
		endedSession, err := service.GetActiveGroup(ctx, session.ID)
		require.NoError(t, err)
		assert.NotNil(t, endedSession.EndTime, "Ended session should have end time")

		// Verify no current session for device
		_, err = service.GetDeviceCurrentSession(ctx, deviceID)
		assert.Error(t, err, "Should not have current session after ending")
	})

	t.Run("end non-existent session returns error", func(t *testing.T) {
		nonExistentID := int64(99999)

		err := service.EndActivitySession(ctx, nonExistentID)
		assert.Error(t, err, "Expected error when ending non-existent session")
	})
}

// TestConflictInfoStructure tests the conflict information structure
func TestConflictInfoStructure(t *testing.T) {
	// Test that ActivityConflictInfo struct has expected fields
	conflictInfo := &activeSvc.ActivityConflictInfo{
		HasConflict:      true,
		ConflictingGroup: &active.Group{},
		ConflictMessage:  "Test conflict",
		CanOverride:      true,
	}

	assert.True(t, conflictInfo.HasConflict)
	assert.NotEmpty(t, conflictInfo.ConflictMessage)
	assert.True(t, conflictInfo.CanOverride)
	assert.NotNil(t, conflictInfo.ConflictingGroup)
}

// TestErrorTypes verifies the custom error types are properly defined
func TestErrorTypes(t *testing.T) {
	// Test that error constants are defined
	errors := []error{
		activeSvc.ErrDeviceAlreadyActive,
		activeSvc.ErrNoActiveSession,
		activeSvc.ErrSessionConflict,
		activeSvc.ErrInvalidActivitySession,
	}

	for _, err := range errors {
		assert.NotNil(t, err, "Expected error to be defined")
		assert.NotEmpty(t, err.Error(), "Expected error to have message")
	}
}

// TestConcurrentSessionAttempts tests race condition handling
func TestConcurrentSessionAttempts(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	}()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("concurrent start attempts on same activity", func(t *testing.T) {
		activityID := int64(3001)
		device1ID := int64(1)
		device2ID := int64(2)
		staffID := int64(1)

		defer cleanupTestData(t, db, activityID)

		// Start two goroutines trying to start the same activity simultaneously
		results := make(chan error, 2)

		go func() {
			_, err := service.StartActivitySession(ctx, activityID, device1ID, staffID, nil)
			results <- err
		}()

		go func() {
			time.Sleep(10 * time.Millisecond) // Small delay to test race condition
			_, err := service.StartActivitySession(ctx, activityID, device2ID, staffID, nil)
			results <- err
		}()

		// Collect results
		err1 := <-results
		err2 := <-results

		// One should succeed, one should fail with conflict
		if err1 == nil {
			assert.Error(t, err2, "Second concurrent attempt should fail")
			assert.Contains(t, err2.Error(), "conflict")
		} else {
			assert.NoError(t, err2, "If first failed, second should succeed")
			assert.Contains(t, err1.Error(), "conflict")
		}
	})
}

// setupTestDBBench creates a test database connection for benchmarks
func setupTestDBBench(b *testing.B) *bun.DB {
	// Use test database DSN from environment or fallback
	testDSN := viper.GetString("test_db_dsn")
	if testDSN == "" {
		testDSN = viper.GetString("db_dsn")
		if testDSN == "" {
			b.Skip("No test database configured (set TEST_DB_DSN or DB_DSN)")
		}
	}

	// Enable debug mode for tests
	viper.Set("db_debug", true)

	db, err := database.DBConn()
	require.NoError(b, err, "Failed to connect to test database")

	return db
}

// setupActiveServiceBench creates an active service for benchmarks
func setupActiveServiceBench(b *testing.B, db *bun.DB) activeSvc.Service {
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db) // Pass db as second parameter
	require.NoError(b, err, "Failed to create service factory")
	return serviceFactory.Active
}

// cleanupTestDataBench removes test data from database for benchmarks
func cleanupTestDataBench(b *testing.B, db *bun.DB, groupIDs ...int64) {
	ctx := context.Background()

	// Clean up active groups
	for _, groupID := range groupIDs {
		_, err := db.NewDelete().
			Model((*active.Group)(nil)).
			Where("group_id = ?", groupID).
			Exec(ctx)
		if err != nil {
			b.Logf("Warning: Failed to cleanup test group %d: %v", groupID, err)
		}
	}
}

// BenchmarkConflictDetection benchmarks conflict detection performance
func BenchmarkConflictDetection(b *testing.B) {
	db := setupTestDBBench(b)
	defer func() {
		if err := db.Close(); err != nil {
			b.Logf("Failed to close database: %v", err)
		}
	}()

	service := setupActiveServiceBench(b, db)
	ctx := context.Background()

	// Setup test data
	activityID := int64(4001)
	deviceID := int64(1)
	staffID := int64(1)

	// Start a session to create conflict scenario
	_, err := service.StartActivitySession(ctx, activityID, deviceID, staffID, nil)
	require.NoError(b, err)

	defer cleanupTestDataBench(b, db, activityID)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := service.CheckActivityConflict(ctx, deviceID+1)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
