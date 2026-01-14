// Package active_test tests the active repositories using hermetic testing pattern.
//
// These tests use real database connections instead of sqlmock, ensuring that:
// - SQL queries actually work against PostgreSQL
// - BUN ORM mappings are correct (schema-qualified tables, column aliases)
// - Foreign key relationships are properly established
// - Edge cases like de-duplication are tested with real data
package active_test

import (
	"context"
	"testing"
	"time"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAttendanceRepository_GetTodayByStudentIDs tests fetching today's attendance
// records for multiple students with hermetic fixtures.
func TestAttendanceRepository_GetTodayByStudentIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	repo := activeRepo.NewAttendanceRepository(db)

	t.Run("returns latest attendance per student with deduplication", func(t *testing.T) {
		// ARRANGE: Create fixtures
		student1 := testpkg.CreateTestStudent(t, db, "Alice", "Test", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Bob", "Test", "1a")
		staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Test")
		device := testpkg.CreateTestDevice(t, db, "attendance-device-001")

		defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID, staff.ID, device.ID)

		// Create multiple attendance records for student1 (different check-in times)
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// Earlier check-in for student1
		_ = testpkg.CreateTestAttendance(t, db, student1.ID, staff.ID, device.ID,
			today.Add(7*time.Hour), nil) // 07:00

		// Later check-in for student1 (this should be returned)
		attendance1 := testpkg.CreateTestAttendance(t, db, student1.ID, staff.ID, device.ID,
			today.Add(8*time.Hour), nil) // 08:00

		// Check-in for student2 (with checkout)
		checkoutTime := today.Add(15 * time.Hour)
		attendance2 := testpkg.CreateTestAttendance(t, db, student2.ID, staff.ID, device.ID,
			today.Add(8*time.Hour+30*time.Minute), &checkoutTime)

		// ACT: Query with duplicate student IDs to test de-duplication
		studentIDs := []int64{student1.ID, student2.ID, student1.ID}
		result, err := repo.GetTodayByStudentIDs(ctx, studentIDs)

		// ASSERT
		require.NoError(t, err)
		require.Len(t, result, 2, "Should return 2 students after de-duplication")

		// Student1 should have the latest attendance (08:00, not 07:00)
		if att, ok := result[student1.ID]; assert.True(t, ok, "Should have attendance for student1") {
			assert.Equal(t, attendance1.ID, att.ID, "Should return latest attendance record")
			assert.Nil(t, att.CheckOutTime, "Student1 should not be checked out")
		}

		// Student2 should have checkout time
		if att, ok := result[student2.ID]; assert.True(t, ok, "Should have attendance for student2") {
			assert.Equal(t, attendance2.ID, att.ID)
			assert.NotNil(t, att.CheckOutTime, "Student2 should be checked out")
		}
	})

	t.Run("returns empty map for empty input", func(t *testing.T) {
		result, err := repo.GetTodayByStudentIDs(ctx, []int64{})

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("returns empty map when no attendance records exist", func(t *testing.T) {
		// ARRANGE: Create student with no attendance
		student := testpkg.CreateTestStudent(t, db, "NoAttendance", "Student", "2a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// ACT
		result, err := repo.GetTodayByStudentIDs(ctx, []int64{student.ID})

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result, "Should return empty map for student without attendance")
	})
}

// TestVisitRepository_GetCurrentByStudentIDs tests fetching current (active) visits
// for students using hermetic fixtures.
func TestVisitRepository_GetCurrentByStudentIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	repo := activeRepo.NewVisitRepository(db)

	t.Run("returns current visits for students", func(t *testing.T) {
		// ARRANGE: Create activity infrastructure
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Chess Club")
		room := testpkg.CreateTestRoom(t, db, "Activity Room 1")

		// Create active session
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)

		// Create students
		student1 := testpkg.CreateTestStudent(t, db, "Carol", "Visitor", "2a")
		student2 := testpkg.CreateTestStudent(t, db, "Dave", "Visitor", "2b")

		defer testpkg.CleanupActivityFixtures(t, db,
			activityGroup.ID, room.ID, activeGroup.ID, student1.ID, student2.ID)

		now := time.Now()

		// Create active visit for student1 (no exit time)
		visit1 := testpkg.CreateTestVisit(t, db, student1.ID, activeGroup.ID,
			now.Add(-15*time.Minute), nil)

		// Create active visit for student2 (no exit time)
		visit2 := testpkg.CreateTestVisit(t, db, student2.ID, activeGroup.ID,
			now.Add(-10*time.Minute), nil)

		// Also clean up visits
		defer testpkg.CleanupActivityFixtures(t, db, visit1.ID, visit2.ID)

		// ACT
		studentIDs := []int64{student1.ID, student2.ID}
		result, err := repo.GetCurrentByStudentIDs(ctx, studentIDs)

		// ASSERT
		require.NoError(t, err)
		require.Len(t, result, 2)

		assert.Equal(t, activeGroup.ID, result[student1.ID].ActiveGroupID)
		assert.Equal(t, activeGroup.ID, result[student2.ID].ActiveGroupID)
	})

	t.Run("excludes visits with exit time", func(t *testing.T) {
		// ARRANGE
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Art Class")
		room := testpkg.CreateTestRoom(t, db, "Art Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "Eve", "Former", "3a")

		defer testpkg.CleanupActivityFixtures(t, db,
			activityGroup.ID, room.ID, activeGroup.ID, student.ID)

		// Create visit that has ended
		exitTime := time.Now().Add(-5 * time.Minute)
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID,
			time.Now().Add(-30*time.Minute), &exitTime)
		defer testpkg.CleanupActivityFixtures(t, db, visit.ID)

		// ACT
		result, err := repo.GetCurrentByStudentIDs(ctx, []int64{student.ID})

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result, "Should not return visits with exit time")
	})

	t.Run("returns empty map for empty input", func(t *testing.T) {
		result, err := repo.GetCurrentByStudentIDs(ctx, []int64{})

		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// TestGroupRepository_FindByIDs tests fetching active groups with their Room relations
// using hermetic fixtures.
func TestGroupRepository_FindByIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	repo := activeRepo.NewGroupRepository(db)

	t.Run("returns groups with room relations", func(t *testing.T) {
		// ARRANGE: Create two complete group setups
		activity1 := testpkg.CreateTestActivityGroup(t, db, "Science Club")
		room1 := testpkg.CreateTestRoom(t, db, "Lab Room")
		activeGroup1 := testpkg.CreateTestActiveGroup(t, db, activity1.ID, room1.ID)

		activity2 := testpkg.CreateTestActivityGroup(t, db, "Music Club")
		room2 := testpkg.CreateTestRoom(t, db, "Music Room")
		activeGroup2 := testpkg.CreateTestActiveGroup(t, db, activity2.ID, room2.ID)

		defer testpkg.CleanupActivityFixtures(t, db,
			activity1.ID, room1.ID, activeGroup1.ID,
			activity2.ID, room2.ID, activeGroup2.ID)

		// ACT
		groupIDs := []int64{activeGroup1.ID, activeGroup2.ID}
		result, err := repo.FindByIDs(ctx, groupIDs)

		// ASSERT
		require.NoError(t, err)
		require.Len(t, result, 2)

		// Check group 1
		if group, ok := result[activeGroup1.ID]; assert.True(t, ok) {
			assert.Equal(t, room1.ID, group.RoomID)
			assert.NotNil(t, group.Room, "Should load Room relation")
			assert.Contains(t, group.Room.Name, "Lab Room")
		}

		// Check group 2
		if group, ok := result[activeGroup2.ID]; assert.True(t, ok) {
			assert.Equal(t, room2.ID, group.RoomID)
			assert.NotNil(t, group.Room, "Should load Room relation")
			assert.Contains(t, group.Room.Name, "Music Room")
		}
	})

	t.Run("returns empty map for empty input", func(t *testing.T) {
		result, err := repo.FindByIDs(ctx, []int64{})

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("returns empty map for non-existent IDs", func(t *testing.T) {
		result, err := repo.FindByIDs(ctx, []int64{999999, 999998})

		require.NoError(t, err)
		assert.Empty(t, result)
	})
}
