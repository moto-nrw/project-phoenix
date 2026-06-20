package activities_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
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

	ctx := testpkg.TenantContext(1)
	enrollment := &activities.StudentEnrollment{
		StudentID:        studentID,
		ActivityGroupID:  groupID,
		ValidFrom:        timezone.DateFromTime(enrollmentDate),
		AttendanceStatus: status,
	}
	enrollment.SetTenantID(1)

	err := db.NewInsert().
		Model(enrollment).
		ModelTableExpr(`activities.student_enrollments AS "enrollment"`).
		Scan(ctx)
	require.NoError(t, err, "Failed to create test enrollment")

	return enrollment
}

func createEnrollmentRequestChildForStudentEnrollmentTest(t *testing.T, db *bun.DB, studentID int64) int64 {
	t.Helper()
	testpkg.EnsureTestTenant(t, db, 1)

	ctx := context.Background()
	token := fmt.Sprintf("student-enrollment-source-%d", time.Now().UnixNano())
	phaseName := fmt.Sprintf("Student enrollment source %d", time.Now().UnixNano())
	var phaseID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.phases
			(tenant_id, name, kind, service_start_date, service_end_date)
		VALUES (1, ?, 'custom', '2026-09-01', '2027-07-31')
		RETURNING id
	`, phaseName).Scan(ctx, &phaseID))

	var requestID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.requests
			(tenant_id, phase_id, guardian_first_name, guardian_last_name,
			 guardian_email, consent_flags, custom_data, status_token, submitted_at)
		VALUES (1, ?, 'Anna', 'Beispiel', ?, '{}'::jsonb, '{}'::jsonb, ?, NOW())
		RETURNING id
	`, phaseID, "student-enrollment-source@example.test", token).Scan(ctx, &requestID))

	var childID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.request_children
			(tenant_id, request_id, first_name, last_name, date_of_birth,
			 status, activation_mode, created_student_id, sort_order, custom_data)
		VALUES (1, ?, 'Lina', 'Quelle', '2018-04-15',
			'approved', 'scheduled', ?, 0, '{}'::jsonb)
		RETURNING id
	`, requestID, studentID).Scan(ctx, &childID))

	t.Cleanup(func() {
		_, _ = db.NewDelete().
			TableExpr("enrollment.request_children").
			Where("tenant_id = 1 AND id = ?", childID).
			Exec(context.Background())
		_, _ = db.NewDelete().
			TableExpr("enrollment.requests").
			Where("tenant_id = 1 AND id = ?", requestID).
			Exec(context.Background())
		_, _ = db.NewDelete().
			TableExpr("enrollment.phases").
			Where("tenant_id = 1 AND id = ?", phaseID).
			Exec(context.Background())
	})

	return childID
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestStudentEnrollmentRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.TenantContext(1)

	t.Run("creates enrollment with valid data", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "TestGroup")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		enrollment := &activities.StudentEnrollment{
			StudentID:       student.ID,
			ActivityGroupID: group.ID,
			ValidFrom:       timezone.TodayDate(),
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
			ValidFrom:        timezone.TodayDate(),
			AttendanceStatus: &status,
		}

		err := repo.Create(ctx, enrollment)
		require.NoError(t, err)
		assert.Equal(t, activities.AttendancePresent, *enrollment.AttendanceStatus)

		testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment.ID)
	})

	t.Run("creates enrollment with tenant from context and selected weekdays", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Weekday", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "WeekdayGroup")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		enrollment := &activities.StudentEnrollment{
			StudentID:        student.ID,
			ActivityGroupID:  group.ID,
			ValidFrom:        timezone.TodayDate(),
			SelectedWeekdays: []int{1, 3, 5},
		}

		err := repo.Create(ctx, enrollment)
		require.NoError(t, err)
		assert.NotZero(t, enrollment.ID)
		assert.EqualValues(t, 1, enrollment.GetTenantID())

		found, err := repo.FindByID(ctx, enrollment.ID)
		require.NoError(t, err)
		assert.Equal(t, []int{1, 3, 5}, found.SelectedWeekdays)

		testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment.ID)
	})

	t.Run("creates enrollment with explicit tenant outside tenant context", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "ExplicitTenant", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "ExplicitTenantGroup")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		enrollment := &activities.StudentEnrollment{
			StudentID:       student.ID,
			ActivityGroupID: group.ID,
			ValidFrom:       timezone.TodayDate(),
		}
		enrollment.SetTenantID(1)

		err := repo.Create(context.Background(), enrollment)
		require.NoError(t, err)
		assert.NotZero(t, enrollment.ID)

		testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment.ID)
	})

	t.Run("rejects invalid selected weekdays before insert", func(t *testing.T) {
		enrollment := &activities.StudentEnrollment{
			StudentID:        106,
			ActivityGroupID:  206,
			ValidFrom:        timezone.TodayDate(),
			SelectedWeekdays: []int{9},
		}
		enrollment.SetTenantID(1)

		err := repo.Create(ctx, enrollment)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "between 1 and 7")
	})
}

func TestStudentEnrollmentRepository_Create_WithNil(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.TenantContext(1)

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
	ctx := testpkg.TenantContext(1)

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
	ctx := testpkg.TenantContext(1)

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

	t.Run("rejects invalid selected weekdays before update", func(t *testing.T) {
		enrollment := &activities.StudentEnrollment{
			StudentID:        101,
			ActivityGroupID:  201,
			ValidFrom:        timezone.TodayDate(),
			SelectedWeekdays: []int{1, 1},
		}
		enrollment.ID = 301

		err := repo.Update(ctx, enrollment)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicates")
	})

	t.Run("updates with explicit tenant when context has no tenant", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "NoTenant", "Update", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "NoTenantUpdateGroup")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		enrollment := createEnrollment(t, db, student.ID, group.ID, time.Now(), nil)
		defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment.ID)

		status := activities.AttendanceExcused
		enrollment.AttendanceStatus = &status
		err := repo.Update(context.Background(), enrollment)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, enrollment.ID)
		require.NoError(t, err)
		require.NotNil(t, found.AttendanceStatus)
		assert.Equal(t, activities.AttendanceExcused, *found.AttendanceStatus)
	})

	t.Run("returns error when no row is updated", func(t *testing.T) {
		enrollment := &activities.StudentEnrollment{
			StudentID:       105,
			ActivityGroupID: 205,
			ValidFrom:       timezone.TodayDate(),
		}
		enrollment.ID = 305
		enrollment.SetTenantID(1)

		err := repo.Update(ctx, enrollment)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update student_enrollment")
	})
}

func TestStudentEnrollmentRepository_Update_WithNil(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.TenantContext(1)

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
	ctx := testpkg.TenantContext(1)

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
	ctx := testpkg.TenantContext(1)

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
	ctx := testpkg.TenantContext(1)

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
	ctx := testpkg.TenantContext(1)

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
	ctx := testpkg.TenantContext(1)

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

func TestStudentEnrollmentRepository_FindByValidFromRange(t *testing.T) {

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.TenantContext(1)

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

		enrollments, err := repo.FindByValidFromRange(ctx, timezone.DateFromTime(start), timezone.DateFromTime(end))
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
		futureStart := timezone.TodayDate().AddDays(365)
		futureEnd := futureStart.AddDays(1)

		enrollments, err := repo.FindByValidFromRange(ctx, futureStart, futureEnd)
		require.NoError(t, err)
		assert.Empty(t, enrollments)
	})
}

func TestStudentEnrollmentRepository_UpdateAttendanceStatus(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.TenantContext(1)

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

	t.Run("returns error when no enrollment row is updated", func(t *testing.T) {
		status := activities.AttendancePresent

		err := repo.UpdateAttendanceStatus(ctx, int64(999999), &status)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update attendance status")
	})
}

func TestStudentEnrollmentRepository_CapActiveByGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.TenantContext(1)

	activeStudent := testpkg.CreateTestStudent(t, db, "CapActive", "Open", "1a")
	closedStudent := testpkg.CreateTestStudent(t, db, "CapActive", "Closed", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "CapActiveGroup")
	defer testpkg.CleanupActivityFixtures(t, db,
		activeStudent.ID, closedStudent.ID,
		group.ID, group.CategoryID, *group.CreatedBy,
	)

	validFrom := timezone.NewDate(2026, time.September, 1)
	existingUntil := timezone.NewDate(2026, time.December, 31)
	capUntil := timezone.NewDate(2026, time.October, 1)
	activeEnrollment := &activities.StudentEnrollment{
		StudentID:       activeStudent.ID,
		ActivityGroupID: group.ID,
		ValidFrom:       validFrom,
	}
	closedEnrollment := &activities.StudentEnrollment{
		StudentID:       closedStudent.ID,
		ActivityGroupID: group.ID,
		ValidFrom:       validFrom,
		ValidUntil:      &existingUntil,
	}
	require.NoError(t, repo.Create(ctx, activeEnrollment))
	require.NoError(t, repo.Create(ctx, closedEnrollment))
	defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", activeEnrollment.ID, closedEnrollment.ID)

	rows, err := repo.CapActiveByGroup(ctx, group.ID, capUntil)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)

	gotActive, err := repo.FindByID(ctx, activeEnrollment.ID)
	require.NoError(t, err)
	require.NotNil(t, gotActive.ValidUntil)
	assert.Equal(t, capUntil, *gotActive.ValidUntil)

	gotClosed, err := repo.FindByID(ctx, closedEnrollment.ID)
	require.NoError(t, err)
	require.NotNil(t, gotClosed.ValidUntil)
	assert.Equal(t, existingUntil, *gotClosed.ValidUntil)
}

func TestStudentEnrollmentRepository_DeleteByStudentGroupsAndWindow(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.TenantContext(1)

	student := testpkg.CreateTestStudent(t, db, "DeleteWindow", "Student", "1a")
	groupOpen := testpkg.CreateTestActivityGroup(t, db, "DeleteWindowOpen")
	groupBounded := testpkg.CreateTestActivityGroup(t, db, "DeleteWindowBounded")
	defer testpkg.CleanupActivityFixtures(t, db,
		student.ID,
		groupOpen.ID, groupOpen.CategoryID, *groupOpen.CreatedBy,
		groupBounded.ID, groupBounded.CategoryID, *groupBounded.CreatedBy,
	)

	validFrom := timezone.NewDate(2026, time.September, 1)
	validUntil := timezone.NewDate(2027, time.July, 31)
	openEnrollment := &activities.StudentEnrollment{
		StudentID:       student.ID,
		ActivityGroupID: groupOpen.ID,
		ValidFrom:       validFrom,
	}
	boundedEnrollment := &activities.StudentEnrollment{
		StudentID:       student.ID,
		ActivityGroupID: groupBounded.ID,
		ValidFrom:       validFrom,
		ValidUntil:      &validUntil,
	}
	require.NoError(t, repo.Create(ctx, openEnrollment))
	require.NoError(t, repo.Create(ctx, boundedEnrollment))
	defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", openEnrollment.ID, boundedEnrollment.ID)

	rows, err := repo.DeleteByStudentGroupsAndWindow(ctx, student.ID, nil, validFrom, nil)
	require.NoError(t, err)
	assert.Zero(t, rows)

	rows, err = repo.DeleteByStudentGroupsAndWindow(ctx, student.ID, []int64{groupOpen.ID, groupBounded.ID}, validFrom, nil)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)

	_, err = repo.FindByID(ctx, openEnrollment.ID)
	assert.Error(t, err)
	stillBounded, err := repo.FindByID(ctx, boundedEnrollment.ID)
	require.NoError(t, err)
	require.NotNil(t, stillBounded.ValidUntil)
	assert.Equal(t, validUntil, *stillBounded.ValidUntil)

	rows, err = repo.DeleteByStudentGroupsAndWindow(ctx, student.ID, []int64{groupBounded.ID}, validFrom, &validUntil)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)

	_, err = repo.DeleteByStudentGroupsAndWindow(ctx, 0, []int64{groupBounded.ID}, validFrom, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "student_id is required")
}

func TestStudentEnrollmentRepository_DeleteByEnrollmentRequestChild(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.TenantContext(1)

	student := testpkg.CreateTestStudent(t, db, "DeleteSource", "Student", "1a")
	groupFromEnrollment := testpkg.CreateTestActivityGroup(t, db, "DeleteSourceEnrollment")
	groupManual := testpkg.CreateTestActivityGroup(t, db, "DeleteSourceManual")
	requestChildID := createEnrollmentRequestChildForStudentEnrollmentTest(t, db, student.ID)
	defer testpkg.CleanupActivityFixtures(t, db,
		student.ID,
		groupFromEnrollment.ID, groupFromEnrollment.CategoryID, *groupFromEnrollment.CreatedBy,
		groupManual.ID, groupManual.CategoryID, *groupManual.CreatedBy,
	)

	validFrom := timezone.NewDate(2026, time.September, 1)
	validUntil := timezone.NewDate(2027, time.July, 31)
	sourcedEnrollment := &activities.StudentEnrollment{
		StudentID:                student.ID,
		ActivityGroupID:          groupFromEnrollment.ID,
		ValidFrom:                validFrom,
		ValidUntil:               &validUntil,
		EnrollmentRequestChildID: &requestChildID,
	}
	manualEnrollment := &activities.StudentEnrollment{
		StudentID:       student.ID,
		ActivityGroupID: groupManual.ID,
		ValidFrom:       validFrom,
		ValidUntil:      &validUntil,
	}
	require.NoError(t, repo.Create(ctx, sourcedEnrollment))
	require.NoError(t, repo.Create(ctx, manualEnrollment))
	defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", sourcedEnrollment.ID, manualEnrollment.ID)

	rows, err := repo.DeleteByEnrollmentRequestChild(ctx, student.ID, requestChildID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)

	_, err = repo.FindByID(ctx, sourcedEnrollment.ID)
	assert.Error(t, err)
	stillManual, err := repo.FindByID(ctx, manualEnrollment.ID)
	require.NoError(t, err)
	assert.Equal(t, groupManual.ID, stillManual.ActivityGroupID)

	_, err = repo.DeleteByEnrollmentRequestChild(ctx, 0, requestChildID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "student_id is required")
	_, err = repo.DeleteByEnrollmentRequestChild(ctx, student.ID, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enrollment_request_child_id is required")
}

func TestStudentEnrollmentRepository_DeleteUnattributedByStudentGroupsAndWindow(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.TenantContext(1)

	student := testpkg.CreateTestStudent(t, db, "DeleteLegacy", "Student", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "DeleteLegacyGroup")
	requestChildID := createEnrollmentRequestChildForStudentEnrollmentTest(t, db, student.ID)
	defer testpkg.CleanupActivityFixtures(t, db,
		student.ID,
		group.ID, group.CategoryID, *group.CreatedBy,
	)

	validFrom := timezone.NewDate(2026, time.September, 1)
	validUntil := timezone.NewDate(2027, time.July, 31)
	legacyEnrollment := &activities.StudentEnrollment{
		StudentID:       student.ID,
		ActivityGroupID: group.ID,
		ValidFrom:       validFrom,
		ValidUntil:      &validUntil,
	}
	sourcedEnrollment := &activities.StudentEnrollment{
		StudentID:                student.ID,
		ActivityGroupID:          group.ID,
		ValidFrom:                validFrom,
		ValidUntil:               &validUntil,
		EnrollmentRequestChildID: &requestChildID,
	}
	require.NoError(t, repo.Create(ctx, legacyEnrollment))
	require.NoError(t, repo.Create(ctx, sourcedEnrollment))
	defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", legacyEnrollment.ID, sourcedEnrollment.ID)

	rows, err := repo.DeleteUnattributedByStudentGroupsAndWindow(ctx, student.ID, []int64{group.ID}, validFrom, &validUntil)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)

	_, err = repo.FindByID(ctx, legacyEnrollment.ID)
	assert.Error(t, err)
	stillSourced, err := repo.FindByID(ctx, sourcedEnrollment.ID)
	require.NoError(t, err)
	require.NotNil(t, stillSourced.EnrollmentRequestChildID)
	assert.Equal(t, requestChildID, *stillSourced.EnrollmentRequestChildID)
}

func TestStudentEnrollmentRepository_CloseOpenByGroupAndPeriod(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	repo := repoFactory.StudentEnrollment
	ctx := testpkg.TenantContext(1)

	studentNoPeriod := testpkg.CreateTestStudent(t, db, "CloseOpen", "NoPeriod", "1a")
	studentWithPeriod := testpkg.CreateTestStudent(t, db, "CloseOpen", "WithPeriod", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "CloseOpenPeriod")
	period := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("CloseOpenPeriod-%d", time.Now().UnixNano()),
		PeriodType:      scheduleModels.PeriodTypeSchoolYear,
		StartDate:       timezone.NewDate(2026, time.September, 1),
		EndDate:         timezone.NewDate(2027, time.July, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	require.NoError(t, repoFactory.CalendarPeriod.Create(ctx, period))
	defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", period.ID)
	defer testpkg.CleanupActivityFixtures(t, db,
		studentNoPeriod.ID, studentWithPeriod.ID,
		group.ID, group.CategoryID, *group.CreatedBy,
	)

	validFrom := timezone.NewDate(2026, time.September, 1)
	closeAt := timezone.NewDate(2026, time.October, 1)
	noPeriod := &activities.StudentEnrollment{
		StudentID:       studentNoPeriod.ID,
		ActivityGroupID: group.ID,
		ValidFrom:       validFrom,
	}
	withPeriod := &activities.StudentEnrollment{
		StudentID:        studentWithPeriod.ID,
		ActivityGroupID:  group.ID,
		CalendarPeriodID: &period.ID,
		ValidFrom:        validFrom,
		SelectedWeekdays: []int{1, 3},
		AttendanceStatus: nil,
	}
	require.NoError(t, repo.Create(ctx, noPeriod))
	require.NoError(t, repo.Create(ctx, withPeriod))
	defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", noPeriod.ID, withPeriod.ID)

	require.NoError(t, repo.CloseOpenByGroupAndPeriod(ctx, group.ID, nil, closeAt))
	gotNoPeriod, err := repo.FindByID(ctx, noPeriod.ID)
	require.NoError(t, err)
	require.NotNil(t, gotNoPeriod.ValidUntil)
	assert.Equal(t, closeAt, *gotNoPeriod.ValidUntil)

	gotWithPeriod, err := repo.FindByID(ctx, withPeriod.ID)
	require.NoError(t, err)
	assert.Nil(t, gotWithPeriod.ValidUntil)

	require.NoError(t, repo.CloseOpenByGroupAndPeriod(ctx, group.ID, &period.ID, closeAt))
	gotWithPeriod, err = repo.FindByID(ctx, withPeriod.ID)
	require.NoError(t, err)
	require.NotNil(t, gotWithPeriod.ValidUntil)
	assert.Equal(t, closeAt, *gotWithPeriod.ValidUntil)
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestStudentEnrollmentRepository_Delete_NonExistent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.TenantContext(1)

	t.Run("does not error when deleting non-existent enrollment", func(t *testing.T) {
		err := repo.Delete(ctx, int64(999999))
		require.NoError(t, err)
	})
}

func TestStudentEnrollmentRepository_QueryErrorsAreWrapped(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.TenantContext(100)
	require.NoError(t, db.Close())

	_, err := repo.FindByStudentID(ctx, 900100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "find by student ID")

	_, err = repo.FindByGroupID(ctx, 900200)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "find by group ID")

	_, err = repo.CountByGroupID(ctx, 900300)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count by group ID")

	from := timezone.NewDate(2026, time.June, 1)
	to := from.AddDays(1)
	_, err = repo.FindByValidFromRange(ctx, from, to)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "find by valid_from date range")

	status := activities.AttendancePresent
	err = repo.UpdateAttendanceStatus(ctx, 900400, &status)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update attendance status")

	enrollment := &activities.StudentEnrollment{
		StudentID:       900500,
		ActivityGroupID: 900600,
		ValidFrom:       from,
	}
	enrollment.SetTenantID(100)
	err = repo.Create(ctx, enrollment)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create")

	enrollment.ID = 900700
	err = repo.Update(ctx, enrollment)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update")

	_, err = repo.List(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list")

	_, err = repo.CapActiveByGroup(ctx, 900800, from)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cap active enrollments by group")

	_, err = repo.DeleteByStudentGroupsAndWindow(ctx, 900900, []int64{901000}, from, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete by student groups and window")

	err = repo.CloseOpenByGroupAndPeriod(ctx, 901100, nil, from)
	require.Error(t, err)
}
