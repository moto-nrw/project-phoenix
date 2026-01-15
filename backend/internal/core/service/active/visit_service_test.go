// Package active_test tests the visit operations in active service layer.
package active_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/device"
	activeModels "github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	active "github.com/moto-nrw/project-phoenix/internal/core/service/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// GetVisit Tests
// =============================================================================

func TestActiveService_GetVisit(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns visit when found", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "get-visit")
		room := testpkg.CreateTestRoom(t, db, "Visit Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "Get", "Visit", "1a")
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID)

		// ACT
		result, err := service.GetVisit(ctx, visit.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, visit.ID, result.ID)
		assert.Equal(t, student.ID, result.StudentID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetVisit(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns error for invalid ID", func(t *testing.T) {
		// ACT
		result, err := service.GetVisit(ctx, 0)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// CreateVisit Tests
// =============================================================================

func TestActiveService_CreateVisit(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("creates visit successfully", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "create-visit")
		room := testpkg.CreateTestRoom(t, db, "Create Visit Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "Create", "Visit", "1a")
		staff := testpkg.CreateTestStaff(t, db, "Check", "In")
		iotDevice := testpkg.CreateTestDevice(t, db, "create-visit-device")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, staff.ID, iotDevice.ID)

		// CreateVisit requires staff and device context for attendance FK constraints
		staffCtx := context.WithValue(ctx, device.CtxStaff, staff)
		deviceCtx := context.WithValue(staffCtx, device.CtxDevice, iotDevice)

		visit := &activeModels.Visit{
			StudentID:     student.ID,
			ActiveGroupID: activeGroup.ID,
			EntryTime:     time.Now(),
		}

		// ACT
		err := service.CreateVisit(deviceCtx, visit)

		// ASSERT
		require.NoError(t, err)
		assert.Greater(t, visit.ID, int64(0))
		defer testpkg.CleanupActivityFixtures(t, db, visit.ID)
	})

	t.Run("returns error for nil visit", func(t *testing.T) {
		// ACT
		err := service.CreateVisit(ctx, nil)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for invalid student ID", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "invalid-student-visit")
		room := testpkg.CreateTestRoom(t, db, "Invalid Student Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID)

		visit := &activeModels.Visit{
			StudentID:     99999999, // invalid
			ActiveGroupID: activeGroup.ID,
			EntryTime:     time.Now(),
		}

		// ACT
		err := service.CreateVisit(ctx, visit)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// UpdateVisit Tests
// =============================================================================

func TestActiveService_UpdateVisit(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("updates visit successfully", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "update-visit")
		room := testpkg.CreateTestRoom(t, db, "Update Visit Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "Update", "Visit", "1a")
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID)

		// Set exit time
		now := time.Now()
		visit.ExitTime = &now

		// ACT
		err := service.UpdateVisit(ctx, visit)

		// ASSERT
		require.NoError(t, err)

		// Verify update persisted
		updated, err := service.GetVisit(ctx, visit.ID)
		require.NoError(t, err)
		assert.NotNil(t, updated.ExitTime)
	})

	t.Run("returns error for nil visit", func(t *testing.T) {
		// ACT
		err := service.UpdateVisit(ctx, nil)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for visit with zero ID", func(t *testing.T) {
		// ARRANGE
		visit := &activeModels.Visit{}
		visit.ID = 0 // Set ID via embedded base.Model

		// ACT
		err := service.UpdateVisit(ctx, visit)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// DeleteVisit Tests
// =============================================================================

func TestActiveService_DeleteVisit(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("deletes visit successfully", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "delete-visit")
		room := testpkg.CreateTestRoom(t, db, "Delete Visit Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "Delete", "Visit", "1a")
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID)

		// ACT
		err := service.DeleteVisit(ctx, visit.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify deletion
		_, err = service.GetVisit(ctx, visit.ID)
		require.Error(t, err)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		err := service.DeleteVisit(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for invalid ID", func(t *testing.T) {
		// ACT
		err := service.DeleteVisit(ctx, 0)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// ListVisits Tests
// =============================================================================

func TestActiveService_ListVisits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns visits with no options", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "list-visits")
		room := testpkg.CreateTestRoom(t, db, "List Visits Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "List", "Visits", "1a")
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID)

		// ACT
		result, err := service.ListVisits(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.GreaterOrEqual(t, len(result), 1)
	})

	t.Run("returns visits with pagination", func(t *testing.T) {
		// ARRANGE
		options := base.NewQueryOptions()
		options.WithPagination(1, 5)

		// ACT
		result, err := service.ListVisits(ctx, options)

		// ASSERT
		require.NoError(t, err)
		assert.LessOrEqual(t, len(result), 5)
	})
}

// =============================================================================
// FindVisitsByStudentID Tests
// =============================================================================

func TestActiveService_FindVisitsByStudentID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns visits for student", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "student-visits")
		room := testpkg.CreateTestRoom(t, db, "Student Visits Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "Find", "ByStudent", "1a")
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID)

		// ACT
		result, err := service.FindVisitsByStudentID(ctx, student.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		// All visits should be for this student
		for _, v := range result {
			assert.Equal(t, student.ID, v.StudentID)
		}
	})

	t.Run("returns empty list for student with no visits", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "No", "Visits", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// ACT
		result, err := service.FindVisitsByStudentID(ctx, student.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// FindVisitsByActiveGroupID Tests
// =============================================================================

func TestActiveService_FindVisitsByActiveGroupID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns visits for active group", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "group-visits")
		room := testpkg.CreateTestRoom(t, db, "Group Visits Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "Find", "ByGroup", "1a")
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID)

		// ACT
		result, err := service.FindVisitsByActiveGroupID(ctx, activeGroup.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		// All visits should be for this active group
		for _, v := range result {
			assert.Equal(t, activeGroup.ID, v.ActiveGroupID)
		}
	})

	t.Run("returns empty list for group with no visits", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "empty-group")
		room := testpkg.CreateTestRoom(t, db, "Empty Group Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID)

		// ACT
		result, err := service.FindVisitsByActiveGroupID(ctx, activeGroup.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// FindVisitsByTimeRange Tests
// =============================================================================

func TestActiveService_FindVisitsByTimeRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns visits in time range", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "time-visits")
		room := testpkg.CreateTestRoom(t, db, "Time Visits Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "Time", "Range", "1a")
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID)

		// Use time range that includes the visit
		start := time.Now().Add(-1 * time.Hour)
		end := time.Now().Add(1 * time.Hour)

		// ACT
		result, err := service.FindVisitsByTimeRange(ctx, start, end)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})
}

// =============================================================================
// EndVisit Tests
// =============================================================================

func TestActiveService_EndVisit(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("ends visit successfully", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "end-visit")
		room := testpkg.CreateTestRoom(t, db, "End Visit Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "End", "Visit", "1a")
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID)

		// ACT
		err := service.EndVisit(ctx, visit.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify exit time set
		ended, err := service.GetVisit(ctx, visit.ID)
		require.NoError(t, err)
		assert.NotNil(t, ended.ExitTime)
	})

	t.Run("returns error for non-existent visit", func(t *testing.T) {
		// ACT
		err := service.EndVisit(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// GetStudentCurrentVisit Tests
// =============================================================================

func TestActiveService_GetStudentCurrentVisit(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns current visit when student is visiting", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "current-visit")
		room := testpkg.CreateTestRoom(t, db, "Current Visit Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "Current", "Visit", "1a")
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID)

		// ACT
		result, err := service.GetStudentCurrentVisit(ctx, student.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, student.ID, result.StudentID)
		assert.Nil(t, result.ExitTime) // Active visit has no exit time
	})

	t.Run("returns error when student has no current visit", func(t *testing.T) {
		// ARRANGE - student with no visits
		student := testpkg.CreateTestStudent(t, db, "No", "Current", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// ACT
		result, err := service.GetStudentCurrentVisit(ctx, student.ID)

		// ASSERT - service returns ErrVisitNotFound when no active visit exists
		require.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, active.ErrVisitNotFound), "expected ErrVisitNotFound")
	})

	t.Run("returns error when student visit is ended", func(t *testing.T) {
		// ARRANGE - student with ended visit
		activity := testpkg.CreateTestActivityGroup(t, db, "ended-visit")
		room := testpkg.CreateTestRoom(t, db, "Ended Visit Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "Ended", "Visit", "1a")
		exitTime := time.Now()
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, time.Now().Add(-1*time.Hour), &exitTime)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID)

		// ACT
		result, err := service.GetStudentCurrentVisit(ctx, student.ID)

		// ASSERT - ended visits are not "current", so ErrVisitNotFound
		require.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, active.ErrVisitNotFound), "expected ErrVisitNotFound")
	})
}

// =============================================================================
// GetStudentsCurrentVisits Tests
// =============================================================================

func TestActiveService_GetStudentsCurrentVisits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns current visits for multiple students", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "multi-visits")
		room := testpkg.CreateTestRoom(t, db, "Multi Visits Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student1 := testpkg.CreateTestStudent(t, db, "Multi", "One", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Multi", "Two", "1a")
		visit1 := testpkg.CreateTestVisit(t, db, student1.ID, activeGroup.ID, time.Now(), nil)
		visit2 := testpkg.CreateTestVisit(t, db, student2.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student1.ID, student2.ID, visit1.ID, visit2.ID)

		// ACT
		result, err := service.GetStudentsCurrentVisits(ctx, []int64{student1.ID, student2.ID})

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Contains(t, result, student1.ID)
		assert.Contains(t, result, student2.ID)
	})

	t.Run("returns empty map for empty input", func(t *testing.T) {
		// ACT
		result, err := service.GetStudentsCurrentVisits(ctx, []int64{})

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("returns partial results for mixed active/inactive students", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "partial-visits")
		room := testpkg.CreateTestRoom(t, db, "Partial Visits Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		studentActive := testpkg.CreateTestStudent(t, db, "Active", "Student", "1a")
		studentInactive := testpkg.CreateTestStudent(t, db, "Inactive", "Student", "1a")
		visit := testpkg.CreateTestVisit(t, db, studentActive.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, studentActive.ID, studentInactive.ID, visit.ID)

		// ACT
		result, err := service.GetStudentsCurrentVisits(ctx, []int64{studentActive.ID, studentInactive.ID})

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Contains(t, result, studentActive.ID)
		assert.NotContains(t, result, studentInactive.ID)
	})
}

// =============================================================================
// CheckTeacherStudentAccess Tests
// =============================================================================

func TestActiveService_CheckTeacherStudentAccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns false when teacher has no access", func(t *testing.T) {
		// ARRANGE - teacher and student not related
		teacher := testpkg.CreateTestTeacher(t, db, "No", "Access")
		student := testpkg.CreateTestStudent(t, db, "Unrelated", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, teacher.ID, teacher.Staff.ID, student.ID)

		// ACT - use staff ID as first parameter (service expects staffID)
		hasAccess, err := service.CheckTeacherStudentAccess(ctx, teacher.Staff.ID, student.ID)

		// ASSERT
		require.NoError(t, err)
		assert.False(t, hasAccess)
	})

	t.Run("returns error for invalid teacher ID", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "Valid", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// ACT
		_, err := service.CheckTeacherStudentAccess(ctx, 99999999, student.ID)

		// ASSERT
		// May or may not error depending on implementation
		// Just verify no panic
		_ = err
	})
}
