package active_test

import (
	"context"
	"testing"
	"time"

	active "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// CountActiveVisitsByRoomID Integration Tests
// =============================================================================

func TestActiveService_CountActiveVisitsByRoomID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns zero for empty room", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "CountEmptyRoom")
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)

		count, err := service.CountActiveVisitsByRoomID(ctx, room.ID)

		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("counts active visits correctly", func(t *testing.T) {
		activity := testpkg.CreateTestActivityGroup(t, db, "count-room-visits")
		room := testpkg.CreateTestRoom(t, db, "CountRoomVisits")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student1 := testpkg.CreateTestStudent(t, db, "CountRoom1", "Student", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "CountRoom2", "Student", "1a")
		visit1 := testpkg.CreateTestVisit(t, db, student1.ID, activeGroup.ID, time.Now(), nil)
		visit2 := testpkg.CreateTestVisit(t, db, student2.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student1.ID, student2.ID, visit1.ID, visit2.ID)

		count, err := service.CountActiveVisitsByRoomID(ctx, room.ID)

		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("excludes exited visits", func(t *testing.T) {
		activity := testpkg.CreateTestActivityGroup(t, db, "count-room-exited")
		room := testpkg.CreateTestRoom(t, db, "CountRoomExited")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student1 := testpkg.CreateTestStudent(t, db, "StillIn", "Student", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Left", "Student", "1a")
		entryTime := time.Now().Add(-10 * time.Minute)
		exitTime := time.Now()
		visit1 := testpkg.CreateTestVisit(t, db, student1.ID, activeGroup.ID, time.Now(), nil)
		visit2 := testpkg.CreateTestVisit(t, db, student2.ID, activeGroup.ID, entryTime, &exitTime)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student1.ID, student2.ID, visit1.ID, visit2.ID)

		count, err := service.CountActiveVisitsByRoomID(ctx, room.ID)

		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("counts across multiple active groups in same room", func(t *testing.T) {
		activity1 := testpkg.CreateTestActivityGroup(t, db, "count-multi-1")
		activity2 := testpkg.CreateTestActivityGroup(t, db, "count-multi-2")
		room := testpkg.CreateTestRoom(t, db, "CountMultiGroupRoom")
		activeGroup1 := testpkg.CreateTestActiveGroup(t, db, activity1.ID, room.ID)
		activeGroup2 := testpkg.CreateTestActiveGroup(t, db, activity2.ID, room.ID)
		student1 := testpkg.CreateTestStudent(t, db, "Multi1", "Student", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Multi2", "Student", "1a")
		visit1 := testpkg.CreateTestVisit(t, db, student1.ID, activeGroup1.ID, time.Now(), nil)
		visit2 := testpkg.CreateTestVisit(t, db, student2.ID, activeGroup2.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity1.ID, activity2.ID, room.ID, activeGroup1.ID, activeGroup2.ID, student1.ID, student2.ID, visit1.ID, visit2.ID)

		count, err := service.CountActiveVisitsByRoomID(ctx, room.ID)

		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("returns zero for non-existent room", func(t *testing.T) {
		count, err := service.CountActiveVisitsByRoomID(ctx, 999999)

		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

// =============================================================================
// CountActiveVisitsByActiveGroupID Integration Tests
// =============================================================================

func TestActiveService_CountActiveVisitsByActiveGroupID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns zero for empty group", func(t *testing.T) {
		activity := testpkg.CreateTestActivityGroup(t, db, "count-empty-group")
		room := testpkg.CreateTestRoom(t, db, "CountEmptyGroupRoom")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID)

		count, err := service.CountActiveVisitsByActiveGroupID(ctx, activeGroup.ID)

		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("counts active visits", func(t *testing.T) {
		activity := testpkg.CreateTestActivityGroup(t, db, "count-group-active")
		room := testpkg.CreateTestRoom(t, db, "CountGroupActiveRoom")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student1 := testpkg.CreateTestStudent(t, db, "GroupCount1", "Student", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "GroupCount2", "Student", "1a")
		student3 := testpkg.CreateTestStudent(t, db, "GroupCount3", "Student", "1a")
		visit1 := testpkg.CreateTestVisit(t, db, student1.ID, activeGroup.ID, time.Now(), nil)
		visit2 := testpkg.CreateTestVisit(t, db, student2.ID, activeGroup.ID, time.Now(), nil)
		visit3 := testpkg.CreateTestVisit(t, db, student3.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student1.ID, student2.ID, student3.ID, visit1.ID, visit2.ID, visit3.ID)

		count, err := service.CountActiveVisitsByActiveGroupID(ctx, activeGroup.ID)

		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("excludes exited visits", func(t *testing.T) {
		activity := testpkg.CreateTestActivityGroup(t, db, "count-group-exited")
		room := testpkg.CreateTestRoom(t, db, "CountGroupExitedRoom")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student1 := testpkg.CreateTestStudent(t, db, "GroupStill", "Student", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "GroupLeft", "Student", "1a")
		entryTime := time.Now().Add(-10 * time.Minute)
		exitTime := time.Now()
		visit1 := testpkg.CreateTestVisit(t, db, student1.ID, activeGroup.ID, time.Now(), nil)
		visit2 := testpkg.CreateTestVisit(t, db, student2.ID, activeGroup.ID, entryTime, &exitTime)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student1.ID, student2.ID, visit1.ID, visit2.ID)

		count, err := service.CountActiveVisitsByActiveGroupID(ctx, activeGroup.ID)

		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}

// =============================================================================
// GetStudentCurrentVisitWithRoom Integration Tests
// =============================================================================

func TestActiveService_GetStudentCurrentVisitWithRoom(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns visit with room loaded", func(t *testing.T) {
		activity := testpkg.CreateTestActivityGroup(t, db, "visit-with-room")
		room := testpkg.CreateTestRoom(t, db, "VisitWithRoomRoom")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "WithRoom", "Student", "1a")
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID)

		result, err := service.GetStudentCurrentVisitWithRoom(ctx, student.ID)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, visit.ID, result.ID)
		require.NotNil(t, result.ActiveGroup, "ActiveGroup should be loaded")
		assert.Equal(t, activeGroup.ID, result.ActiveGroup.ID)
		require.NotNil(t, result.ActiveGroup.Room, "Room should be loaded")
		assert.Contains(t, result.ActiveGroup.Room.Name, "VisitWithRoomRoom")
	})

	t.Run("returns error for student with no visit", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "NoVisit", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		result, err := service.GetStudentCurrentVisitWithRoom(ctx, student.ID)

		require.Error(t, err)
		assert.Nil(t, result)

		var activeErr *active.ActiveError
		require.ErrorAs(t, err, &activeErr)
		assert.Equal(t, "GetStudentCurrentVisitWithRoom", activeErr.Op)
	})

	t.Run("returns error for non-existent student", func(t *testing.T) {
		result, err := service.GetStudentCurrentVisitWithRoom(ctx, 999999)

		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// GetStudentCurrentVisit Integration Tests (refactored implementation)
// =============================================================================

func TestActiveService_GetStudentCurrentVisit_Refactored(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns current visit", func(t *testing.T) {
		activity := testpkg.CreateTestActivityGroup(t, db, "current-visit-ref")
		room := testpkg.CreateTestRoom(t, db, "CurrentVisitRefRoom")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "Current", "Visit", "1a")
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID)

		result, err := service.GetStudentCurrentVisit(ctx, student.ID)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, visit.ID, result.ID)
		assert.Equal(t, student.ID, result.StudentID)
	})

	t.Run("returns error for student with no active visit", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "NoCurrent", "Visit", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		result, err := service.GetStudentCurrentVisit(ctx, student.ID)

		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("ignores exited visits", func(t *testing.T) {
		activity := testpkg.CreateTestActivityGroup(t, db, "exited-visit-ref")
		room := testpkg.CreateTestRoom(t, db, "ExitedVisitRefRoom")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "Exited", "Visit", "1a")
		entryTime := time.Now().Add(-10 * time.Minute)
		exitTime := time.Now()
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, entryTime, &exitTime)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID)

		result, err := service.GetStudentCurrentVisit(ctx, student.ID)

		require.Error(t, err, "Should not return an exited visit as current")
		assert.Nil(t, result)
	})
}
