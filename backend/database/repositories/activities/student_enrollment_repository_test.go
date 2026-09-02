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

	ctx := testpkg.Ctx(t)
	enrollment := &activities.StudentEnrollment{
		StudentID:        studentID,
		ActivityGroupID:  groupID,
		ValidFrom:        timezone.DateFromTime(enrollmentDate),
		AttendanceStatus: status,
	}
	enrollment.SetTenantID(testpkg.Tenant(t))

	err := db.NewInsert().
		Model(enrollment).
		ModelTableExpr(`activities.student_enrollments AS "enrollment"`).
		Scan(ctx)
	require.NoError(t, err, "Failed to create test enrollment")

	return enrollment
}

func createEnrollmentRequestChildForStudentEnrollmentTest(t *testing.T, db *bun.DB, studentID int64) int64 {
	t.Helper()
	testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))

	ctx := context.Background()
	token := fmt.Sprintf("student-enrollment-source-%d", time.Now().UnixNano())
	phaseName := fmt.Sprintf("Student enrollment source %d", time.Now().UnixNano())
	var phaseID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.phases
			(tenant_id, name, kind, service_start_date, service_end_date)
		VALUES (?, ?, 'custom', '2026-09-01', '2027-07-31')
		RETURNING id
	`, testpkg.Tenant(t), phaseName).Scan(ctx, &phaseID))

	var requestID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.requests
			(tenant_id, phase_id, guardian_first_name, guardian_last_name,
			 guardian_email, consent_flags, custom_data, status_token, submitted_at)
		VALUES (?, ?, 'Anna', 'Beispiel', ?, '{}'::jsonb, '{}'::jsonb, ?, NOW())
		RETURNING id
	`, testpkg.Tenant(t), phaseID, "student-enrollment-source@example.test", token).Scan(ctx, &requestID))

	var childID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.request_children
			(tenant_id, request_id, first_name, last_name, date_of_birth,
			 status, activation_mode, created_student_id, sort_order, custom_data)
		VALUES (?, ?, 'Lina', 'Quelle', '2018-04-15',
			'approved', 'scheduled', ?, 0, '{}'::jsonb)
		RETURNING id
	`, testpkg.Tenant(t), requestID, studentID).Scan(ctx, &childID))

	tenantID := testpkg.Tenant(t)
	t.Cleanup(func() {
		_, _ = db.NewDelete().
			TableExpr("enrollment.request_children").
			Where("tenant_id = ? AND id = ?", tenantID, childID).
			Exec(context.Background())
		_, _ = db.NewDelete().
			TableExpr("enrollment.requests").
			Where("tenant_id = ? AND id = ?", tenantID, requestID).
			Exec(context.Background())
		_, _ = db.NewDelete().
			TableExpr("enrollment.phases").
			Where("tenant_id = ? AND id = ?", tenantID, phaseID).
			Exec(context.Background())
	})

	return childID
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestStudentEnrollmentRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.Ctx(t)

	t.Run("creates enrollment with valid data", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "TestGroup")

		enrollment := &activities.StudentEnrollment{
			StudentID:       student.ID,
			ActivityGroupID: group.ID,
			ValidFrom:       timezone.TodayDate(),
		}

		err := repo.Create(ctx, enrollment)
		require.NoError(t, err)
		assert.NotZero(t, enrollment.ID)

	})

	t.Run("creates enrollment with attendance status", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Status", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "StatusGroup")

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

	})

	t.Run("creates enrollment with tenant from context and selected weekdays", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Weekday", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "WeekdayGroup")

		enrollment := &activities.StudentEnrollment{
			StudentID:        student.ID,
			ActivityGroupID:  group.ID,
			ValidFrom:        timezone.TodayDate(),
			SelectedWeekdays: []int{1, 3, 5},
		}

		err := repo.Create(ctx, enrollment)
		require.NoError(t, err)
		assert.NotZero(t, enrollment.ID)
		assert.EqualValues(t, testpkg.Tenant(t), enrollment.GetTenantID())

		found, err := repo.FindByID(ctx, enrollment.ID)
		require.NoError(t, err)
		assert.Equal(t, []int{1, 3, 5}, found.SelectedWeekdays)

	})

	t.Run("creates enrollment with explicit tenant outside tenant context", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "ExplicitTenant", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "ExplicitTenantGroup")

		enrollment := &activities.StudentEnrollment{
			StudentID:       student.ID,
			ActivityGroupID: group.ID,
			ValidFrom:       timezone.TodayDate(),
		}
		enrollment.SetTenantID(testpkg.Tenant(t))

		err := repo.Create(context.Background(), enrollment)
		require.NoError(t, err)
		assert.NotZero(t, enrollment.ID)

	})

	t.Run("rejects invalid selected weekdays before insert", func(t *testing.T) {
		enrollment := &activities.StudentEnrollment{
			StudentID:        106,
			ActivityGroupID:  206,
			ValidFrom:        timezone.TodayDate(),
			SelectedWeekdays: []int{9},
		}
		enrollment.SetTenantID(testpkg.Tenant(t))

		err := repo.Create(ctx, enrollment)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "between 1 and 7")
	})
}

func TestStudentEnrollmentRepository_Create_WithNil(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.Ctx(t)

	t.Run("returns error when enrollment is nil", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestStudentEnrollmentRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.Ctx(t)

	t.Run("finds existing enrollment", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Find", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "FindGroup")

		enrollment := createEnrollment(t, db, student.ID, group.ID, time.Now(), nil)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.Ctx(t)

	t.Run("updates enrollment attendance status", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Update", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "UpdateGroup")

		enrollment := createEnrollment(t, db, student.ID, group.ID, time.Now(), nil)

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

		enrollment := createEnrollment(t, db, student.ID, group.ID, time.Now(), nil)

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
		enrollment.SetTenantID(testpkg.Tenant(t))

		err := repo.Update(ctx, enrollment)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update student_enrollment")
	})
}

func TestStudentEnrollmentRepository_Update_WithNil(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.Ctx(t)

	t.Run("returns error when enrollment is nil", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestStudentEnrollmentRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing enrollment", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Delete", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "DeleteGroup")

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.Ctx(t)

	t.Run("lists all enrollments", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "List", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "ListGroup")

		createEnrollment(t, db, student.ID, group.ID, time.Now(), nil)

		enrollments, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, enrollments)
	})

	t.Run("lists enrollments with pagination", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Page", "Student", "1a")
		group := testpkg.CreateTestActivityGroup(t, db, "PageGroup")

		createEnrollment(t, db, student.ID, group.ID, time.Now(), nil)

		options := modelBase.NewQueryOptions()
		options.WithPagination(1, 10)

		enrollments, err := repo.List(ctx, options)
		require.NoError(t, err)
		assert.NotNil(t, enrollments)
	})
}

func TestStudentEnrollmentRepository_FindByStudentID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.Ctx(t)

	t.Run("finds enrollments for a student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Student", "Enrollments", "1a")
		group1 := testpkg.CreateTestActivityGroup(t, db, "Group1")
		group2 := testpkg.CreateTestActivityGroup(t, db, "Group2")

		enrollment1 := createEnrollment(t, db, student.ID, group1.ID, time.Now(), nil)
		enrollment2 := createEnrollment(t, db, student.ID, group2.ID, time.Now().Add(-24*time.Hour), nil)

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

		enrollments, err := repo.FindByStudentID(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, enrollments)
	})
}

func TestStudentEnrollmentRepository_FindActiveByStudentIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.Ctx(t)
	onDate := timezone.NewDate(2026, time.September, 15)
	validFrom := timezone.NewDate(2026, time.September, 1)
	futureFrom := timezone.NewDate(2026, time.October, 1)
	endsOnDate := onDate
	endsAfterDate := timezone.NewDate(2026, time.September, 16)
	expiredUntil := timezone.NewDate(2026, time.September, 10)

	studentA := testpkg.CreateTestStudent(t, db, "FindActive", "Alpha", "1a")
	studentB := testpkg.CreateTestStudent(t, db, "FindActive", "Beta", "1a")
	studentOther := testpkg.CreateTestStudent(t, db, "FindActive", "Other", "1a")
	groupAlpha := testpkg.CreateTestActivityGroup(t, db, "FindActiveAlpha")
	groupBeta := testpkg.CreateTestActivityGroup(t, db, "FindActiveBeta")
	groupFuture := testpkg.CreateTestActivityGroup(t, db, "FindActiveFuture")
	groupExpired := testpkg.CreateTestActivityGroup(t, db, "FindActiveExpired")
	groupBoundary := testpkg.CreateTestActivityGroup(t, db, "FindActiveBoundary")
	groupWeekdayMatch := testpkg.CreateTestActivityGroup(t, db, "FindActiveWeekdayMatch")
	groupWeekdayMismatch := testpkg.CreateTestActivityGroup(t, db, "FindActiveWeekdayMismatch")
	matchingWeekday := int(onDate.Weekday())
	if matchingWeekday == 0 {
		matchingWeekday = activities.WeekdaySunday
	}
	mismatchingWeekday := matchingWeekday%activities.WeekdaySunday + 1

	activeOpen := &activities.StudentEnrollment{StudentID: studentA.ID, ActivityGroupID: groupBeta.ID, ValidFrom: validFrom}
	activeBounded := &activities.StudentEnrollment{StudentID: studentA.ID, ActivityGroupID: groupAlpha.ID, ValidFrom: validFrom, ValidUntil: &endsAfterDate}
	activeSecondStudent := &activities.StudentEnrollment{StudentID: studentB.ID, ActivityGroupID: groupAlpha.ID, ValidFrom: validFrom}
	future := &activities.StudentEnrollment{StudentID: studentA.ID, ActivityGroupID: groupFuture.ID, ValidFrom: futureFrom}
	expired := &activities.StudentEnrollment{StudentID: studentA.ID, ActivityGroupID: groupExpired.ID, ValidFrom: validFrom, ValidUntil: &expiredUntil}
	boundary := &activities.StudentEnrollment{StudentID: studentA.ID, ActivityGroupID: groupBoundary.ID, ValidFrom: validFrom, ValidUntil: &endsOnDate}
	otherStudent := &activities.StudentEnrollment{StudentID: studentOther.ID, ActivityGroupID: groupAlpha.ID, ValidFrom: validFrom}
	weekdayMatch := &activities.StudentEnrollment{StudentID: studentA.ID, ActivityGroupID: groupWeekdayMatch.ID, ValidFrom: validFrom, Weekday: &matchingWeekday}
	weekdayMismatch := &activities.StudentEnrollment{StudentID: studentA.ID, ActivityGroupID: groupWeekdayMismatch.ID, ValidFrom: validFrom, Weekday: &mismatchingWeekday}
	for _, enrollment := range []*activities.StudentEnrollment{
		activeOpen,
		activeBounded,
		activeSecondStudent,
		future,
		expired,
		boundary,
		otherStudent,
		weekdayMatch,
		weekdayMismatch,
	} {
		require.NoError(t, repo.Create(ctx, enrollment))
	}

	empty, err := repo.FindActiveByStudentIDs(ctx, nil, onDate)
	require.NoError(t, err)
	assert.Empty(t, empty)

	got, err := repo.FindActiveByStudentIDs(ctx, []int64{studentA.ID, studentB.ID}, onDate)
	require.NoError(t, err)
	require.Len(t, got, 4)

	assert.Equal(t, studentA.ID, got[0].StudentID)
	require.NotNil(t, got[0].ActivityGroup)
	assert.Equal(t, groupAlpha.ID, got[0].ActivityGroup.ID)
	assert.Equal(t, groupAlpha.Name, got[0].ActivityGroup.Name)

	assert.Equal(t, studentA.ID, got[1].StudentID)
	require.NotNil(t, got[1].ActivityGroup)
	assert.Equal(t, groupBeta.ID, got[1].ActivityGroup.ID)

	assert.Equal(t, studentA.ID, got[2].StudentID)
	require.NotNil(t, got[2].ActivityGroup)
	assert.Equal(t, groupWeekdayMatch.ID, got[2].ActivityGroup.ID)

	assert.Equal(t, studentB.ID, got[3].StudentID)
	require.NotNil(t, got[3].ActivityGroup)
	assert.Equal(t, groupAlpha.ID, got[3].ActivityGroup.ID)
}

func TestStudentEnrollmentRepository_FindByGroupID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	// Person relations come from the People Directory composition (#2661),
	// so this test drives the composed repository the service graph uses.
	factory, err := repositories.NewFactoryWithPeopleDirectory(db)
	require.NoError(t, err)
	repo := factory.StudentEnrollment
	ctx := testpkg.Ctx(t)

	t.Run("finds enrollments for a group", func(t *testing.T) {
		student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "1b")
		group := testpkg.CreateTestActivityGroup(t, db, "GroupEnrollments")

		enrollment1 := createEnrollment(t, db, student1.ID, group.ID, time.Now(), nil)
		enrollment2 := createEnrollment(t, db, student2.ID, group.ID, time.Now(), nil)

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

		enrollments, err := repo.FindByGroupID(ctx, group.ID)
		require.NoError(t, err)
		assert.Empty(t, enrollments)
	})
}

func TestStudentEnrollmentRepository_CapActiveByGroup(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.Ctx(t)

	activeStudent := testpkg.CreateTestStudent(t, db, "CapActive", "Open", "1a")
	closedStudent := testpkg.CreateTestStudent(t, db, "CapActive", "Closed", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "CapActiveGroup")

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

func TestStudentEnrollmentRepository_BackfillEnrollmentRequestChildSource(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "BackfillSource", "Student", "1a")
	groupLinked := testpkg.CreateTestActivityGroup(t, db, "BackfillSourceLinked")
	groupManual := testpkg.CreateTestActivityGroup(t, db, "BackfillSourceManual")
	requestChildID := createEnrollmentRequestChildForStudentEnrollmentTest(t, db, student.ID)

	reviewedAt := time.Now().UTC()
	_, err := db.NewRaw(`
		UPDATE enrollment.request_children
		SET reviewed_at = ?
		WHERE tenant_id = ? AND id = ?
	`, reviewedAt, testpkg.Tenant(t), requestChildID).Exec(context.Background())
	require.NoError(t, err)

	validFrom := timezone.NewDate(2026, time.September, 1)
	validUntil := timezone.NewDate(2027, time.July, 31)
	manualUntil := timezone.NewDate(2028, time.July, 31)
	linkedEnrollment := &activities.StudentEnrollment{
		StudentID:       student.ID,
		ActivityGroupID: groupLinked.ID,
		ValidFrom:       validFrom,
		ValidUntil:      &validUntil,
	}
	manualEnrollment := &activities.StudentEnrollment{
		StudentID:       student.ID,
		ActivityGroupID: groupManual.ID,
		ValidFrom:       validFrom,
		ValidUntil:      &manualUntil,
	}
	require.NoError(t, repo.Create(ctx, linkedEnrollment))
	require.NoError(t, repo.Create(ctx, manualEnrollment))

	rows, err := repo.BackfillEnrollmentRequestChildSource(ctx, student.ID, requestChildID, nil)
	require.NoError(t, err)
	assert.Zero(t, rows)

	rows, err = repo.BackfillEnrollmentRequestChildSource(ctx, student.ID, requestChildID, []int64{groupLinked.ID})
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)

	gotLinked, err := repo.FindByID(ctx, linkedEnrollment.ID)
	require.NoError(t, err)
	require.NotNil(t, gotLinked.EnrollmentRequestChildID)
	assert.Equal(t, requestChildID, *gotLinked.EnrollmentRequestChildID)

	gotManual, err := repo.FindByID(ctx, manualEnrollment.ID)
	require.NoError(t, err)
	assert.Nil(t, gotManual.EnrollmentRequestChildID)

	_, err = repo.BackfillEnrollmentRequestChildSource(ctx, 0, requestChildID, []int64{groupLinked.ID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "student_id is required")
	_, err = repo.BackfillEnrollmentRequestChildSource(ctx, student.ID, 0, []int64{groupLinked.ID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enrollment_request_child_id is required")
}

func TestStudentEnrollmentRepository_DeleteByEnrollmentRequestChild(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "DeleteSource", "Student", "1a")
	groupFromEnrollment := testpkg.CreateTestActivityGroup(t, db, "DeleteSourceEnrollment")
	groupManual := testpkg.CreateTestActivityGroup(t, db, "DeleteSourceManual")
	requestChildID := createEnrollmentRequestChildForStudentEnrollmentTest(t, db, student.ID)

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

func TestStudentEnrollmentRepository_CloseOpenByGroupAndPeriod(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repoFactory := repositories.NewFactory(db)
	repo := repoFactory.StudentEnrollment
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StudentEnrollment
	ctx := testpkg.Ctx(t)

	t.Run("does not error when deleting non-existent enrollment", func(t *testing.T) {
		err := repo.Delete(ctx, int64(999999))
		require.NoError(t, err)
	})
}
