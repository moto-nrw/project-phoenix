package activities_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// createEnrollment is a helper to create an enrollment without validation
func createEnrollment(t *testing.T, db *bun.DB, studentID, groupID int64, enrollmentDate time.Time, status *string) *activities.StudentEnrollment {
	t.Helper()

	ctx := context.Background()
	enrollment := &activities.StudentEnrollment{
		StudentID:        studentID,
		ActivityGroupID:  groupID,
		EnrollmentDate:   enrollmentDate,
		AttendanceStatus: status,
	}

	err := db.NewInsert().
		Model(enrollment).
		ModelTableExpr(`activities.student_enrollments AS "enrollment"`).
		Scan(ctx)
	require.NoError(t, err, "Failed to create test enrollment")

	return enrollment
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestStudentEnrollmentRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := context.Background()

	t.Run("creates enrollment with valid data", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "TestGroup")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		enrollment := &activities.StudentEnrollment{
			StudentID:       student.ID,
			ActivityGroupID: group.ID,
			EnrollmentDate:  time.Now(),
		}

		err := repo.Create(ctx, enrollment)
		require.NoError(t, err)
		assert.NotZero(t, enrollment.ID)

		testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment.ID)
	})

	t.Run("creates enrollment with attendance status", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Status", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "StatusGroup")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		status := activities.AttendancePresent
		enrollment := &activities.StudentEnrollment{
			StudentID:        student.ID,
			ActivityGroupID:  group.ID,
			EnrollmentDate:   time.Now(),
			AttendanceStatus: &status,
		}

		err := repo.Create(ctx, enrollment)
		require.NoError(t, err)
		assert.Equal(t, activities.AttendancePresent, *enrollment.AttendanceStatus)

		testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment.ID)
	})
}

func TestStudentEnrollmentRepository_Create_WithNil(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := context.Background()

	t.Run("returns error when enrollment is nil", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestStudentEnrollmentRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := context.Background()

	t.Run("finds existing enrollment", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Find", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "FindGroup")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		enrollment := createEnrollment(t, db, student.ID, group.ID, time.Now(), nil)
		defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment.ID)

		found, err := repo.FindByID(ctx, enrollment.ID)
		require.NoError(t, err)
		assert.Equal(t, enrollment.ID, found.ID)
		assert.Equal(t, student.ID, found.StudentID)
	})

	t.Run("returns error for non-existent enrollment", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestStudentEnrollmentRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := context.Background()

	t.Run("updates enrollment attendance status", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Update", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "UpdateGroup")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		enrollment := createEnrollment(t, db, student.ID, group.ID, time.Now(), nil)
		defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment.ID)

		status := activities.AttendanceAbsent
		enrollment.AttendanceStatus = &status
		err := repo.Update(ctx, enrollment)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, enrollment.ID)
		require.NoError(t, err)
		require.NotNil(t, found.AttendanceStatus)
		assert.Equal(t, activities.AttendanceAbsent, *found.AttendanceStatus)
	})
}

func TestStudentEnrollmentRepository_Update_WithNil(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := context.Background()

	t.Run("returns error when enrollment is nil", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestStudentEnrollmentRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := context.Background()

	t.Run("deletes existing enrollment", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Delete", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "DeleteGroup")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		enrollment := createEnrollment(t, db, student.ID, group.ID, time.Now(), nil)

		err := repo.Delete(ctx, enrollment.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, enrollment.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestStudentEnrollmentRepository_List(t *testing.T) {

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := context.Background()

	t.Run("lists all enrollments", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "List", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "ListGroup")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		enrollment := createEnrollment(t, db, student.ID, group.ID, time.Now(), nil)
		defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment.ID)

		enrollments, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, enrollments)
	})

	t.Run("lists enrollments with pagination", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Page", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "PageGroup")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		enrollment := createEnrollment(t, db, student.ID, group.ID, time.Now(), nil)
		defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment.ID)

		options := modelBase.NewQueryOptions()
		options.WithPagination(1, 10)

		enrollments, err := repo.List(ctx, options)
		require.NoError(t, err)
		assert.NotNil(t, enrollments)
	})
}

func TestStudentEnrollmentRepository_FindByStudentID(t *testing.T) {

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := context.Background()

	t.Run("finds enrollments for a student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Student", "Enrollments", "1a")
		group1 := testpkg.CreateTestActivityGroup(t, db, "Group1")
		group2 := testpkg.CreateTestActivityGroup(t, db, "Group2")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, 0, 0, group1.CategoryID, 0)
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group2.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group1.ID, group2.ID)

		enrollment1 := createEnrollment(t, db, student.ID, group1.ID, time.Now(), nil)
		enrollment2 := createEnrollment(t, db, student.ID, group2.ID, time.Now().Add(-24*time.Hour), nil)
		defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment1.ID, enrollment2.ID)

		enrollments, err := repo.FindByStudentID(ctx, student.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, enrollments)

		// Should have at least our 2 enrollments
		var count int
		for _, e := range enrollments {
			if e.ID == enrollment1.ID || e.ID == enrollment2.ID {
				count++
			}
		}
		assert.GreaterOrEqual(t, count, 2)
	})

	t.Run("returns empty for student with no enrollments", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "NoEnrollments", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, 0, 0, 0, 0)

		enrollments, err := repo.FindByStudentID(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, enrollments)
	})
}

func TestStudentEnrollmentRepository_FindByGroupID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := context.Background()

	t.Run("finds enrollments for a group", func(t *testing.T) {
		student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "1b")
		group := testpkg.CreateTestActivityGroup(t, db, "GroupEnrollments")
		defer testpkg.CleanupActivityFixtures(t, db, student1.ID, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupActivityFixtures(t, db, student2.ID, 0, 0, 0, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		enrollment1 := createEnrollment(t, db, student1.ID, group.ID, time.Now(), nil)
		enrollment2 := createEnrollment(t, db, student2.ID, group.ID, time.Now(), nil)
		defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment1.ID, enrollment2.ID)

		enrollments, err := repo.FindByGroupID(ctx, group.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, enrollments)

		// Check that we have our enrollments
		var count int
		for _, e := range enrollments {
			if e.ID == enrollment1.ID || e.ID == enrollment2.ID {
				count++
				// Check that student and person are loaded
				assert.NotNil(t, e.Student)
				if e.Student != nil {
					assert.NotNil(t, e.Student.Person)
				}
			}
		}
		assert.GreaterOrEqual(t, count, 2)
	})

	t.Run("returns empty for group with no enrollments", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "EmptyGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		enrollments, err := repo.FindByGroupID(ctx, group.ID)
		require.NoError(t, err)
		assert.Empty(t, enrollments)
	})
}

func TestStudentEnrollmentRepository_CountByGroupID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := context.Background()

	t.Run("counts enrollments in a group", func(t *testing.T) {
		student1 := testpkg.CreateTestStudent(t, db, "Count", "One", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Count", "Two", "1b")
		student3 := testpkg.CreateTestStudent(t, db, "Count", "Three", "1c")
		group := testpkg.CreateTestActivityGroup(t, db, "CountGroup")
		defer testpkg.CleanupActivityFixtures(t, db, student1.ID, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupActivityFixtures(t, db, student2.ID, 0, 0, 0, 0)
		defer testpkg.CleanupActivityFixtures(t, db, student3.ID, 0, 0, 0, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		enrollment1 := createEnrollment(t, db, student1.ID, group.ID, time.Now(), nil)
		enrollment2 := createEnrollment(t, db, student2.ID, group.ID, time.Now(), nil)
		enrollment3 := createEnrollment(t, db, student3.ID, group.ID, time.Now(), nil)
		defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment1.ID, enrollment2.ID, enrollment3.ID)

		count, err := repo.CountByGroupID(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("returns zero for group with no enrollments", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "ZeroCount")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		count, err := repo.CountByGroupID(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestStudentEnrollmentRepository_FindByEnrollmentDateRange(t *testing.T) {

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := context.Background()

	t.Run("finds enrollments within date range", func(t *testing.T) {
		// Create multiple students for multiple enrollments (unique constraint: student_id + group_id)
		student1 := testpkg.CreateTestStudent(t, db, "DateRange1", "Student", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "DateRange2", "Student", "1a")
		student3 := testpkg.CreateTestStudent(t, db, "DateRange3", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "DateGroup")
		defer testpkg.CleanupActivityFixtures(t, db, student1.ID, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupActivityFixtures(t, db, student2.ID, 0, 0, 0, 0)
		defer testpkg.CleanupActivityFixtures(t, db, student3.ID, 0, 0, 0, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		now := time.Now()
		yesterday := now.Add(-24 * time.Hour)
		twoDaysAgo := now.Add(-48 * time.Hour)
		threeDaysAgo := now.Add(-72 * time.Hour)

		enrollment1 := createEnrollment(t, db, student1.ID, group.ID, yesterday, nil)
		enrollment2 := createEnrollment(t, db, student2.ID, group.ID, twoDaysAgo, nil)
		enrollment3 := createEnrollment(t, db, student3.ID, group.ID, threeDaysAgo, nil)
		defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment1.ID, enrollment2.ID, enrollment3.ID)

		// Search for enrollments in the last 2.5 days
		start := now.Add(-60 * time.Hour)
		end := now

		enrollments, err := repo.FindByEnrollmentDateRange(ctx, start, end)
		require.NoError(t, err)
		assert.NotEmpty(t, enrollments)

		// Should find enrollment1 and enrollment2, but not enrollment3
		var foundIDs []int64
		for _, e := range enrollments {
			if e.ID == enrollment1.ID || e.ID == enrollment2.ID || e.ID == enrollment3.ID {
				foundIDs = append(foundIDs, e.ID)
			}
		}

		assert.Contains(t, foundIDs, enrollment1.ID)
		assert.Contains(t, foundIDs, enrollment2.ID)
		// enrollment3 is outside the range, so it should not be found
	})

	t.Run("returns empty for range with no enrollments", func(t *testing.T) {
		futureStart := time.Now().Add(365 * 24 * time.Hour)
		futureEnd := futureStart.Add(24 * time.Hour)

		enrollments, err := repo.FindByEnrollmentDateRange(ctx, futureStart, futureEnd)
		require.NoError(t, err)
		assert.Empty(t, enrollments)
	})
}

func TestStudentEnrollmentRepository_UpdateAttendanceStatus(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := context.Background()

	t.Run("updates attendance status to present", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Status", "Update", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "StatusGroup")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		enrollment := createEnrollment(t, db, student.ID, group.ID, time.Now(), nil)
		defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment.ID)

		status := activities.AttendancePresent
		err := repo.UpdateAttendanceStatus(ctx, enrollment.ID, &status)
		require.NoError(t, err)

		// Verify the update
		found, err := repo.FindByID(ctx, enrollment.ID)
		require.NoError(t, err)
		require.NotNil(t, found.AttendanceStatus)
		assert.Equal(t, activities.AttendancePresent, *found.AttendanceStatus)
	})

	t.Run("updates attendance status to absent", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Absent", "Update", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "AbsentGroup")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		enrollment := createEnrollment(t, db, student.ID, group.ID, time.Now(), nil)
		defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment.ID)

		status := activities.AttendanceAbsent
		err := repo.UpdateAttendanceStatus(ctx, enrollment.ID, &status)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, enrollment.ID)
		require.NoError(t, err)
		require.NotNil(t, found.AttendanceStatus)
		assert.Equal(t, activities.AttendanceAbsent, *found.AttendanceStatus)
	})

	t.Run("sets attendance status to nil", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Nil", "Status", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "NilGroup")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		status := activities.AttendancePresent
		enrollment := createEnrollment(t, db, student.ID, group.ID, time.Now(), &status)
		defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment.ID)

		// Update to nil
		err := repo.UpdateAttendanceStatus(ctx, enrollment.ID, nil)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, enrollment.ID)
		require.NoError(t, err)
		assert.Nil(t, found.AttendanceStatus)
	})

	t.Run("returns error for invalid status", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Invalid", "Status", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "InvalidGroup")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		enrollment := createEnrollment(t, db, student.ID, group.ID, time.Now(), nil)
		defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment.ID)

		invalidStatus := "invalid_status_value"
		err := repo.UpdateAttendanceStatus(ctx, enrollment.ID, &invalidStatus)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid attendance status")
	})
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestStudentEnrollmentRepository_Delete_NonExistent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := context.Background()

	t.Run("does not error when deleting non-existent enrollment", func(t *testing.T) {
		err := repo.Delete(ctx, int64(999999))
		require.NoError(t, err)
	})
}
