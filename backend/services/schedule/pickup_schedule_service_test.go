package schedule_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupPickupScheduleService creates a PickupScheduleService with real database connection
func setupPickupScheduleService(t *testing.T, db *bun.DB) schedule.PickupScheduleService {
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.PickupSchedule
}

func createPickupServiceTestStaffID(t *testing.T, db *bun.DB) int64 {
	t.Helper()

	staff := testpkg.CreateTestStaff(t, db, "Pickup", fmt.Sprintf("Creator-%d", time.Now().UnixNano()))
	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(t, db, staff.ID)
	})

	return staff.ID
}

// =============================================================================
// Schedule Operations Tests
// =============================================================================

func TestPickupScheduleService_GetStudentPickupSchedules(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns all schedules for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		for _, weekday := range []int{scheduleModels.WeekdayMonday, scheduleModels.WeekdayWednesday} {
			sched := &scheduleModels.StudentPickupSchedule{
				StudentID:  student.ID,
				Weekday:    weekday,
				PickupTime: time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC),
				CreatedBy:  createPickupServiceTestStaffID(t, db),
			}
			err := service.UpsertStudentPickupSchedule(ctx, sched)
			require.NoError(t, err)
		}

		results, err := service.GetStudentPickupSchedules(ctx, student.ID)

		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("returns empty slice when no schedules", func(t *testing.T) {
		results, err := service.GetStudentPickupSchedules(ctx, int64(99999999))

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestPickupScheduleService_GetStudentPickupScheduleForWeekday(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns schedule for specific weekday", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayTuesday,
			PickupTime: time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
			CreatedBy:  createPickupServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentPickupSchedule(ctx, sched)
		require.NoError(t, err)

		result, err := service.GetStudentPickupScheduleForWeekday(ctx, student.ID, scheduleModels.WeekdayTuesday)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, scheduleModels.WeekdayTuesday, result.Weekday)
	})

	t.Run("returns error for invalid weekday", func(t *testing.T) {
		result, err := service.GetStudentPickupScheduleForWeekday(ctx, int64(1), 10)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid weekday")
	})
}

func TestPickupScheduleService_UpsertStudentPickupSchedule(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("creates new schedule", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayFriday,
			PickupTime: time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
			CreatedBy:  createPickupServiceTestStaffID(t, db),
		}

		err := service.UpsertStudentPickupSchedule(ctx, sched)

		require.NoError(t, err)
		assert.Greater(t, sched.ID, int64(0))
	})

	t.Run("updates existing schedule", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayMonday,
			PickupTime: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			CreatedBy:  createPickupServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentPickupSchedule(ctx, sched)
		require.NoError(t, err)

		sched.PickupTime = time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC)

		err = service.UpsertStudentPickupSchedule(ctx, sched)

		require.NoError(t, err)

		result, err := service.GetStudentPickupScheduleForWeekday(ctx, student.ID, scheduleModels.WeekdayMonday)
		require.NoError(t, err)
		assert.Equal(t, 15, result.PickupTime.Hour())
	})

	t.Run("fails validation for invalid schedule", func(t *testing.T) {
		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  0,
			Weekday:    scheduleModels.WeekdayMonday,
			PickupTime: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			CreatedBy:  createPickupServiceTestStaffID(t, db),
		}

		err := service.UpsertStudentPickupSchedule(ctx, sched)

		require.Error(t, err)
	})
}

func TestPickupScheduleService_UpsertBulkStudentPickupSchedules(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("creates multiple schedules in transaction", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		schedules := []*scheduleModels.StudentPickupSchedule{
			{
				Weekday:    scheduleModels.WeekdayMonday,
				PickupTime: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
				CreatedBy:  createPickupServiceTestStaffID(t, db),
			},
			{
				Weekday:    scheduleModels.WeekdayWednesday,
				PickupTime: time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
				CreatedBy:  createPickupServiceTestStaffID(t, db),
			},
			{
				Weekday:    scheduleModels.WeekdayFriday,
				PickupTime: time.Date(2024, 1, 1, 13, 30, 0, 0, time.UTC),
				CreatedBy:  createPickupServiceTestStaffID(t, db),
			},
		}

		err := service.UpsertBulkStudentPickupSchedules(ctx, student.ID, schedules)

		require.NoError(t, err)

		results, err := service.GetStudentPickupSchedules(ctx, student.ID)
		require.NoError(t, err)
		assert.Len(t, results, 3)
	})

	t.Run("rolls back on validation error", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		schedules := []*scheduleModels.StudentPickupSchedule{
			{
				Weekday:    scheduleModels.WeekdayMonday,
				PickupTime: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
				CreatedBy:  createPickupServiceTestStaffID(t, db),
			},
			{
				Weekday:    10,
				PickupTime: time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
				CreatedBy:  createPickupServiceTestStaffID(t, db),
			},
		}

		// Wrap in transaction so partial writes are rolled back on error
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		txCtx := base.ContextWithTx(ctx, &tx)

		err = service.UpsertBulkStudentPickupSchedules(txCtx, student.ID, schedules)

		require.Error(t, err)
		require.NoError(t, tx.Rollback())

		results, err := service.GetStudentPickupSchedules(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestPickupScheduleService_DeleteStudentPickupSchedule(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("deletes schedule by ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayThursday,
			PickupTime: time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC),
			CreatedBy:  createPickupServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentPickupSchedule(ctx, sched)
		require.NoError(t, err)

		err = service.DeleteStudentPickupSchedule(ctx, sched.ID)

		require.NoError(t, err)

		results, err := service.GetStudentPickupSchedules(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestPickupScheduleService_DeleteAllStudentPickupSchedules(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("deletes all schedules for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		for _, weekday := range []int{scheduleModels.WeekdayMonday, scheduleModels.WeekdayWednesday, scheduleModels.WeekdayFriday} {
			sched := &scheduleModels.StudentPickupSchedule{
				StudentID:  student.ID,
				Weekday:    weekday,
				PickupTime: time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC),
				CreatedBy:  createPickupServiceTestStaffID(t, db),
			}
			err := service.UpsertStudentPickupSchedule(ctx, sched)
			require.NoError(t, err)
		}

		err := service.DeleteAllStudentPickupSchedules(ctx, student.ID)

		require.NoError(t, err)

		results, err := service.GetStudentPickupSchedules(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

// =============================================================================
// Exception Operations Tests
// =============================================================================

func TestPickupScheduleService_CreateStudentPickupException(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("creates exception successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		exception := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: timezone.NewDate(2024, 3, 15),
			Reason:        testpkg.StrPtr("Doctor appointment"),
			CreatedBy:     createPickupServiceTestStaffID(t, db),
		}

		err := service.CreateStudentPickupException(ctx, exception)

		require.NoError(t, err)
		assert.Greater(t, exception.ID, int64(0))
	})

	t.Run("upserts when exception already exists for date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Use Berlin timezone for consistent date handling
		exceptionDate := timezone.NewDate(2024, 3, 20)
		firstPickupTime := time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC)
		exception1 := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: exceptionDate,
			PickupTime:    &firstPickupTime,
			Reason:        testpkg.StrPtr("First exception"),
			CreatedBy:     createPickupServiceTestStaffID(t, db),
		}
		err := service.CreateStudentPickupException(ctx, exception1)
		require.NoError(t, err)
		originalID := exception1.ID

		secondPickupTime := time.Date(2000, 1, 1, 13, 0, 0, 0, time.UTC)
		exception2 := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: exceptionDate,
			PickupTime:    &secondPickupTime,
			Reason:        testpkg.StrPtr("Changed pickup"),
			CreatedBy:     createPickupServiceTestStaffID(t, db),
		}

		err = service.CreateStudentPickupException(ctx, exception2)

		require.NoError(t, err)
		assert.Equal(t, originalID, exception2.ID, "should update existing exception, not create new")
		assert.Equal(t, 13, exception2.PickupTime.Hour(), "should have updated pickup time")
		assert.Equal(t, "Changed pickup", *exception2.Reason, "should have updated reason")
	})

	t.Run("fails validation for invalid exception", func(t *testing.T) {
		exception := &scheduleModels.StudentPickupException{
			StudentID:     0,
			ExceptionDate: timezone.NewDate(2024, 3, 15),
			Reason:        testpkg.StrPtr("Test"),
			CreatedBy:     createPickupServiceTestStaffID(t, db),
		}

		err := service.CreateStudentPickupException(ctx, exception)

		require.Error(t, err)
	})
}

func TestPickupScheduleService_ReclaimGuardianPickupRejectsSharedPartialAbsence(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err)
	ctx := testpkg.TenantContext(1)
	student := testpkg.CreateTestStudent(t, db, "Guardian", fmt.Sprintf("Partial-%d", time.Now().UnixNano()), "1a")
	staff := testpkg.CreateTestStaff(t, db, "Partial", fmt.Sprintf("Owner-%d", time.Now().UnixNano()))
	guardian := testpkg.CreateTestAccount(t, db, fmt.Sprintf("guardian-partial-%d@test.local", time.Now().UnixNano()))
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)
	defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
	defer testpkg.CleanupAuthFixtures(t, db, guardian.ID)

	date := timezone.TodayDate().AddDays(4)
	pickupTime := timezone.WallClock(time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC))
	guardianID := guardian.ID
	guardianPickup := &scheduleModels.StudentPickupException{
		StudentID:         student.ID,
		ExceptionDate:     date,
		PickupTime:        &pickupTime,
		Source:            scheduleModels.ExceptionSourceGuardian,
		CreatedByGuardian: &guardianID,
	}
	require.NoError(t, repoFactory.StudentPickupException.Create(ctx, guardianPickup))
	defer testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_exceptions", guardianPickup.ID)

	partial, err := serviceFactory.PartialAbsence.Create(ctx, schedule.PartialAbsenceInput{
		StudentID: student.ID,
		Date:      date,
		FromTime:  timezone.WallClock(time.Date(2000, 1, 1, 13, 30, 0, 0, time.UTC)),
		Reason:    "Termin",
		StaffID:   staff.ID,
	})
	require.NoError(t, err)
	require.Equal(t, guardianPickup.ID, partial.ID, "the partial must reuse the guardian pickup row")

	staffPickup := timezone.WallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	_, err = serviceFactory.PickupSchedule.CreateOrReclaimException(
		ctx,
		student.ID,
		date,
		&staffPickup,
		testpkg.StrPtr("staff override"),
		staff.ID,
		func() (int64, error) { return staff.ID, nil },
	)
	require.ErrorIs(t, err, schedule.ErrCareExceptionContainsPartialAbsence)

	fresh, err := repoFactory.StudentPickupException.FindByID(ctx, guardianPickup.ID)
	require.NoError(t, err)
	require.NotNil(t, fresh)
	require.NotNil(t, fresh.ExcusedFrom, "failed reclaim must preserve partial metadata")
	assert.Equal(t, "13:30", timezone.WallClock(*fresh.ExcusedFrom).Format("15:04"))
	assert.Equal(t, scheduleModels.ExceptionSourceGuardian, fresh.Source)
}

func TestPickupScheduleService_GetStudentPickupExceptions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns all exceptions for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Use consistent base date to avoid any timezone edge cases
		baseDate := timezone.TodayDate()
		for i := -2; i <= 2; i++ {
			exception := &scheduleModels.StudentPickupException{
				StudentID:     student.ID,
				ExceptionDate: baseDate.AddDays(i),
				Reason:        testpkg.StrPtr("Exception"),
				CreatedBy:     createPickupServiceTestStaffID(t, db),
			}
			err := service.CreateStudentPickupException(ctx, exception)
			require.NoError(t, err)
		}

		results, err := service.GetStudentPickupExceptions(ctx, student.ID)

		require.NoError(t, err)
		assert.Len(t, results, 5)
	})
}

func TestPickupScheduleService_GetUpcomingStudentPickupExceptions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns only upcoming exceptions", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Use consistent base date to avoid timezone edge cases
		baseDate := timezone.TodayDate()

		for i := -5; i < 0; i++ {
			exception := &scheduleModels.StudentPickupException{
				StudentID:     student.ID,
				ExceptionDate: baseDate.AddDays(i),
				Reason:        testpkg.StrPtr("Past"),
				CreatedBy:     createPickupServiceTestStaffID(t, db),
			}
			err := service.CreateStudentPickupException(ctx, exception)
			require.NoError(t, err)
		}

		for i := 1; i <= 3; i++ {
			exception := &scheduleModels.StudentPickupException{
				StudentID:     student.ID,
				ExceptionDate: baseDate.AddDays(i),
				Reason:        testpkg.StrPtr("Future"),
				CreatedBy:     createPickupServiceTestStaffID(t, db),
			}
			err := service.CreateStudentPickupException(ctx, exception)
			require.NoError(t, err)
		}

		results, err := service.GetUpcomingStudentPickupExceptions(ctx, student.ID)

		require.NoError(t, err)
		assert.Len(t, results, 3)
		for _, result := range results {
			assert.Equal(t, "Future", *result.Reason)
		}
	})
}

func TestPickupScheduleService_UpdateStudentPickupException(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("updates exception successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		exception := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: timezone.NewDate(2024, 4, 1),
			Reason:        testpkg.StrPtr("Original reason"),
			CreatedBy:     createPickupServiceTestStaffID(t, db),
		}
		err := service.CreateStudentPickupException(ctx, exception)
		require.NoError(t, err)

		exception.Reason = testpkg.StrPtr("Updated reason")

		err = service.UpdateStudentPickupException(ctx, exception)

		require.NoError(t, err)

		exceptions, err := service.GetStudentPickupExceptions(ctx, student.ID)
		require.NoError(t, err)
		assert.Len(t, exceptions, 1)
		assert.Equal(t, "Updated reason", *exceptions[0].Reason)
	})
}

func TestPickupScheduleService_UpdateExceptionPreservesOmittedPickupTime(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupPickupScheduleService(t, db)
	student := testpkg.CreateTestStudent(t, db, "Test", "PickupPatch", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	ctx := testpkg.TenantContext(student.TenantID)
	staffID := createPickupServiceTestStaffID(t, db)
	exceptionDate := timezone.NewDate(2024, 4, 2)
	pickupTime := time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC)
	exception := &scheduleModels.StudentPickupException{
		StudentID:     student.ID,
		ExceptionDate: exceptionDate,
		PickupTime:    &pickupTime,
		Reason:        testpkg.StrPtr("Original reason"),
		CreatedBy:     staffID,
	}
	require.NoError(t, service.CreateStudentPickupException(ctx, exception))

	updatedReason := "Updated reason"
	updated, err := service.UpdateException(
		ctx,
		exception.ID,
		student.ID,
		exceptionDate,
		&updatedReason,
		nil,
		false,
		func() (int64, error) { return staffID, nil },
	)
	require.NoError(t, err)
	require.NotNil(t, updated.PickupTime)
	assert.Equal(t, pickupTime.Format("15:04"), updated.PickupTime.Format("15:04"))

	fresh, err := service.GetStudentPickupExceptionByID(ctx, exception.ID)
	require.NoError(t, err)
	require.NotNil(t, fresh.PickupTime)
	assert.Equal(t, pickupTime.Format("15:04"), fresh.PickupTime.Format("15:04"))
}

func TestPickupScheduleService_UpdateExceptionClearsPickupTime(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupPickupScheduleService(t, db)
	student := testpkg.CreateTestStudent(t, db, "Test", "PickupClear", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	ctx := testpkg.TenantContext(student.TenantID)
	staffID := createPickupServiceTestStaffID(t, db)
	exceptionDate := timezone.NewDate(2024, 4, 2)
	pickupTime := time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC)
	exception := &scheduleModels.StudentPickupException{
		StudentID:     student.ID,
		ExceptionDate: exceptionDate,
		PickupTime:    &pickupTime,
		Reason:        testpkg.StrPtr("Original reason"),
		CreatedBy:     staffID,
	}
	require.NoError(t, service.CreateStudentPickupException(ctx, exception))
	clearReason := ""

	updated, err := service.UpdateException(
		ctx,
		exception.ID,
		student.ID,
		exceptionDate,
		&clearReason,
		nil,
		true,
		func() (int64, error) { return staffID, nil },
	)
	require.NoError(t, err)
	assert.Nil(t, updated.PickupTime)
	assert.Nil(t, updated.Reason)
}

func TestPickupScheduleService_CreateExceptionUpsertDropsPartialOwnershipOnTimeChange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupPickupScheduleService(t, db)
	student := testpkg.CreateTestStudent(t, db, "Test", "UpsertOwns", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	ctx := testpkg.TenantContext(student.TenantID)
	staffID := createPickupServiceTestStaffID(t, db)
	exceptionDate := timezone.NewDate(2024, 4, 4)
	from := timezone.WallClock(time.Date(2000, 1, 1, 13, 30, 0, 0, time.UTC))
	exception := &scheduleModels.StudentPickupException{
		StudentID:             student.ID,
		ExceptionDate:         exceptionDate,
		PickupTime:            &from,
		ExcusedFrom:           &from,
		ExcusedCreatedBy:      &staffID,
		ExcusedOwnsPickupTime: true,
		Source:                scheduleModels.ExceptionSourceStaff,
		CreatedBy:             staffID,
	}
	require.NoError(t, service.CreateStudentPickupException(ctx, exception))

	// Collision-update path: a second create for the same date with a different
	// wall-clock must drop partial ownership so delete cannot wipe the override.
	newPickup := timezone.WallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))
	override := &scheduleModels.StudentPickupException{
		StudentID:     student.ID,
		ExceptionDate: exceptionDate,
		PickupTime:    &newPickup,
		Source:        scheduleModels.ExceptionSourceStaff,
		CreatedBy:     staffID,
	}
	require.NoError(t, service.CreateStudentPickupException(ctx, override))

	fresh, err := service.GetStudentPickupExceptionForDate(ctx, student.ID, exceptionDate)
	require.NoError(t, err)
	require.NotNil(t, fresh)
	require.NotNil(t, fresh.PickupTime)
	assert.Equal(t, "15:00", timezone.WallClock(*fresh.PickupTime).Format("15:04"))
	assert.False(t, fresh.ExcusedOwnsPickupTime,
		"upsert with a new wall-clock must drop partial ownership of the pickup time")
	require.NotNil(t, fresh.ExcusedFrom, "partial metadata itself is preserved until explicit delete")
}

func TestPickupScheduleService_UpdateExceptionSamePickupTimeKeepsPartialOwnership(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service := setupPickupScheduleService(t, db)
	student := testpkg.CreateTestStudent(t, db, "Test", "PartialOwns", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	ctx := testpkg.TenantContext(student.TenantID)
	staffID := createPickupServiceTestStaffID(t, db)
	exceptionDate := timezone.NewDate(2024, 4, 3)
	pickupTime := timezone.WallClock(time.Date(2000, 1, 1, 13, 30, 0, 0, time.UTC))
	exception := &scheduleModels.StudentPickupException{
		StudentID:             student.ID,
		ExceptionDate:         exceptionDate,
		PickupTime:            &pickupTime,
		ExcusedFrom:           &pickupTime,
		ExcusedCreatedBy:      &staffID,
		ExcusedOwnsPickupTime: true,
		Source:                scheduleModels.ExceptionSourceStaff,
		CreatedBy:             staffID,
	}
	require.NoError(t, service.CreateStudentPickupException(ctx, exception))

	// Same wall-clock with a different date anchor must not look like a time edit.
	sameClockDifferentAnchor := time.Date(2024, 4, 3, 13, 30, 0, 0, time.UTC)
	updatedReason := "Arzttermin"
	updated, err := service.UpdateException(
		ctx,
		exception.ID,
		student.ID,
		exceptionDate,
		&updatedReason,
		&sameClockDifferentAnchor,
		false,
		func() (int64, error) { return staffID, nil },
	)
	require.NoError(t, err)
	require.NotNil(t, updated.ExcusedFrom)
	assert.True(t, updated.ExcusedOwnsPickupTime,
		"identical wall-clock must keep partial ownership so delete can reclaim the pickup row")
	require.NotNil(t, updated.PickupTime)
	assert.Equal(t, "13:30", timezone.WallClock(*updated.PickupTime).Format("15:04"))
}

func TestPickupScheduleService_DeleteStudentPickupException(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("deletes exception by ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		exception := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: timezone.NewDate(2024, 5, 1),
			Reason:        testpkg.StrPtr("Test"),
			CreatedBy:     createPickupServiceTestStaffID(t, db),
		}
		err := service.CreateStudentPickupException(ctx, exception)
		require.NoError(t, err)

		err = service.DeleteStudentPickupException(ctx, exception.ID)

		require.NoError(t, err)

		results, err := service.GetStudentPickupExceptions(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)

		err = service.DeleteStudentPickupException(ctx, exception.ID)
		require.ErrorIs(t, err, schedule.ErrCareExceptionNotFound,
			"a repeated or concurrent losing delete must be classified as not found")
	})

	t.Run("rejects a pickup exception shared with a partial absence", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Partial", fmt.Sprintf("Delete-%d", time.Now().UnixNano()), "1a")
		staffID := createPickupServiceTestStaffID(t, db)
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)
		date := timezone.TodayDate().AddDays(5)
		from := timezone.WallClock(time.Date(2000, 1, 1, 13, 30, 0, 0, time.UTC))
		exception := &scheduleModels.StudentPickupException{
			StudentID:             student.ID,
			ExceptionDate:         date,
			PickupTime:            &from,
			ExcusedFrom:           &from,
			ExcusedCreatedBy:      &staffID,
			ExcusedOwnsPickupTime: true,
			CreatedBy:             staffID,
		}
		require.NoError(t, service.CreateStudentPickupException(ctx, exception))

		err := service.DeleteStudentPickupException(ctx, exception.ID)
		require.ErrorIs(t, err, schedule.ErrCareExceptionContainsPartialAbsence)
		fresh, findErr := service.GetStudentPickupExceptionByID(ctx, exception.ID)
		require.NoError(t, findErr)
		require.NotNil(t, fresh)
		require.NotNil(t, fresh.ExcusedFrom)
	})
}

func TestPickupScheduleService_DeleteAllStudentPickupExceptions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("deletes all exceptions for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Use consistent base date to avoid timezone edge cases
		baseDate := timezone.TodayDate()
		for i := 1; i <= 5; i++ {
			exception := &scheduleModels.StudentPickupException{
				StudentID:     student.ID,
				ExceptionDate: baseDate.AddDays(i),
				Reason:        testpkg.StrPtr("Exception"),
				CreatedBy:     createPickupServiceTestStaffID(t, db),
			}
			err := service.CreateStudentPickupException(ctx, exception)
			require.NoError(t, err)
		}

		err := service.DeleteAllStudentPickupExceptions(ctx, student.ID)

		require.NoError(t, err)

		results, err := service.GetStudentPickupExceptions(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("rejects the batch when one exception owns a partial absence", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Partial", fmt.Sprintf("Bulk-%d", time.Now().UnixNano()), "1a")
		staffID := createPickupServiceTestStaffID(t, db)
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)
		date := timezone.TodayDate().AddDays(6)
		ordinary := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: date.AddDays(-1),
			PickupTime:    testpkg.TimePtr(timezone.WallClock(time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC))),
			CreatedBy:     staffID,
		}
		require.NoError(t, service.CreateStudentPickupException(ctx, ordinary))
		from := timezone.WallClock(time.Date(2000, 1, 1, 13, 30, 0, 0, time.UTC))
		partial := &scheduleModels.StudentPickupException{
			StudentID:             student.ID,
			ExceptionDate:         date,
			PickupTime:            &from,
			ExcusedFrom:           &from,
			ExcusedCreatedBy:      &staffID,
			ExcusedOwnsPickupTime: true,
			CreatedBy:             staffID,
		}
		require.NoError(t, service.CreateStudentPickupException(ctx, partial))

		err := service.DeleteAllStudentPickupExceptions(ctx, student.ID)
		require.ErrorIs(t, err, schedule.ErrCareExceptionContainsPartialAbsence)
		rows, findErr := service.GetStudentPickupExceptions(ctx, student.ID)
		require.NoError(t, findErr)
		require.Len(t, rows, 2, "the transaction must roll back the ordinary deletion")
		assert.Equal(t, ordinary.ID, rows[0].ID)
		require.NotNil(t, rows[1].ExcusedFrom)
	})
}

// =============================================================================
// Computed Operations Tests
// =============================================================================

func TestPickupScheduleService_GetStudentPickupData(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns combined schedule and exception data", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayMonday,
			PickupTime: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			CreatedBy:  createPickupServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentPickupSchedule(ctx, sched)
		require.NoError(t, err)

		exception := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: timezone.TodayDate().AddDays(5),
			Reason:        testpkg.StrPtr("Future exception"),
			CreatedBy:     createPickupServiceTestStaffID(t, db),
		}
		err = service.CreateStudentPickupException(ctx, exception)
		require.NoError(t, err)

		result, err := service.GetStudentPickupData(ctx, student.ID)

		require.NoError(t, err)
		assert.Len(t, result.Schedules, 1)
		assert.Len(t, result.Exceptions, 1)
	})
}

func TestPickupScheduleService_GetEffectivePickupTimeForDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns exception when present", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		recurringNote := "Pick up at the side entrance"
		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayMonday,
			PickupTime: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			Notes:      &recurringNote,
			CreatedBy:  createPickupServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentPickupSchedule(ctx, sched)
		require.NoError(t, err)

		// Use a fixed Monday date at noon to avoid timezone boundary issues
		// January 8, 2024 is a Monday, and noon UTC is still Monday in Berlin
		testDate := timezone.NewDate(2024, 1, 8)

		earlyTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		exception := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: testDate,
			PickupTime:    &earlyTime,
			Reason:        testpkg.StrPtr("Early pickup"),
			CreatedBy:     createPickupServiceTestStaffID(t, db),
		}
		err = service.CreateStudentPickupException(ctx, exception)
		require.NoError(t, err)

		result, err := service.GetEffectivePickupTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.True(t, result.IsException)
		assert.NotNil(t, result.PickupTime)
		assert.Equal(t, 12, result.PickupTime.Hour())
		assert.Equal(t, "Early pickup", result.Notes)
	})

	t.Run("returns schedule when no exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayTuesday,
			PickupTime: time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC),
			CreatedBy:  createPickupServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentPickupSchedule(ctx, sched)
		require.NoError(t, err)

		// Use a fixed Tuesday date at noon to avoid timezone boundary issues
		// January 9, 2024 is a Tuesday, and noon UTC is still Tuesday in Berlin
		testDate := timezone.NewDate(2024, 1, 9)

		result, err := service.GetEffectivePickupTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.False(t, result.IsException)
		assert.NotNil(t, result.PickupTime)
		assert.Equal(t, 15, result.PickupTime.Hour())
		assert.Equal(t, 30, result.PickupTime.Minute())
	})

	t.Run("returns nil pickup time for weekend", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Use a fixed Saturday date at noon to avoid timezone boundary issues
		// January 13, 2024 is a Saturday, and noon UTC is still Saturday in Berlin
		testDate := timezone.NewDate(2024, 1, 13)

		result, err := service.GetEffectivePickupTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.Nil(t, result.PickupTime)
	})

	t.Run("returns nil when no schedule and no exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Use a fixed Wednesday date at noon to avoid timezone boundary issues
		// January 10, 2024 is a Wednesday
		testDate := timezone.NewDate(2024, 1, 10)

		result, err := service.GetEffectivePickupTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.Nil(t, result.PickupTime)
		assert.False(t, result.IsException)
	})

	t.Run("returns schedule with notes", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		notes := "Pick up with grandma"
		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayFriday,
			PickupTime: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			Notes:      &notes,
			CreatedBy:  createPickupServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentPickupSchedule(ctx, sched)
		require.NoError(t, err)

		// Use a fixed Friday date at noon to avoid timezone boundary issues
		// January 12, 2024 is a Friday
		testDate := timezone.NewDate(2024, 1, 12)

		result, err := service.GetEffectivePickupTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.False(t, result.IsException)
		assert.NotNil(t, result.PickupTime)
		assert.Equal(t, "Pick up with grandma", result.Notes)
	})

	t.Run("falls back to recurring schedule notes when exception reason is blank", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		recurringNote := "Wait at the side entrance"
		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayMonday,
			PickupTime: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			Notes:      &recurringNote,
			CreatedBy:  createPickupServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentPickupSchedule(ctx, sched)
		require.NoError(t, err)

		testDate := timezone.NewDate(2024, 1, 8)
		updatedTime := time.Date(2024, 1, 1, 11, 30, 0, 0, time.UTC)
		blankReason := "   "
		exception := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: testDate,
			PickupTime:    &updatedTime,
			Reason:        &blankReason,
			CreatedBy:     createPickupServiceTestStaffID(t, db),
		}
		err = service.CreateStudentPickupException(ctx, exception)
		require.NoError(t, err)

		result, err := service.GetEffectivePickupTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.True(t, result.IsException)
		assert.Equal(t, "Wait at the side entrance", result.Notes)
	})

	t.Run("handles Sunday correctly", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Use a fixed Sunday date at noon to avoid timezone boundary issues
		// January 14, 2024 is a Sunday
		testDate := timezone.NewDate(2024, 1, 14)

		result, err := service.GetEffectivePickupTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.Nil(t, result.PickupTime)
	})
}

func TestPickupScheduleService_GetBulkEffectivePickupTimesForDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns effective times for multiple students", func(t *testing.T) {
		student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "1b")
		student3 := testpkg.CreateTestStudent(t, db, "Student", "Three", "1c")
		defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID, student3.ID)

		// Use a fixed Thursday date at noon to avoid timezone boundary issues
		// January 11, 2024 is a Thursday
		testDate := timezone.NewDate(2024, 1, 11)

		sched1 := &scheduleModels.StudentPickupSchedule{
			StudentID:  student1.ID,
			Weekday:    scheduleModels.WeekdayThursday,
			PickupTime: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			CreatedBy:  createPickupServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentPickupSchedule(ctx, sched1)
		require.NoError(t, err)

		earlyTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		exception2 := &scheduleModels.StudentPickupException{
			StudentID:     student2.ID,
			ExceptionDate: testDate,
			PickupTime:    &earlyTime,
			Reason:        testpkg.StrPtr("Doctor appointment"),
			CreatedBy:     createPickupServiceTestStaffID(t, db),
		}
		err = service.CreateStudentPickupException(ctx, exception2)
		require.NoError(t, err)

		exception3 := &scheduleModels.StudentPickupException{
			StudentID:     student3.ID,
			ExceptionDate: testDate,
			PickupTime:    nil,
			Reason:        testpkg.StrPtr("Sick"),
			CreatedBy:     createPickupServiceTestStaffID(t, db),
		}
		err = service.CreateStudentPickupException(ctx, exception3)
		require.NoError(t, err)

		results, err := service.GetBulkEffectivePickupTimesForDate(ctx, []int64{student1.ID, student2.ID, student3.ID}, testDate)

		require.NoError(t, err)
		assert.Len(t, results, 3)

		assert.False(t, results[student1.ID].IsException)
		assert.NotNil(t, results[student1.ID].PickupTime)
		assert.Equal(t, 14, results[student1.ID].PickupTime.Hour())

		assert.True(t, results[student2.ID].IsException)
		assert.NotNil(t, results[student2.ID].PickupTime)
		assert.Equal(t, 12, results[student2.ID].PickupTime.Hour())
		assert.Equal(t, "Doctor appointment", results[student2.ID].Notes)

		assert.True(t, results[student3.ID].IsException)
		assert.Nil(t, results[student3.ID].PickupTime)
		assert.Equal(t, "Sick", results[student3.ID].Notes)
	})

	t.Run("returns empty map for empty student IDs", func(t *testing.T) {
		results, err := service.GetBulkEffectivePickupTimesForDate(ctx, []int64{}, timezone.TodayDate())

		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("handles weekend correctly for all students", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Use a fixed Sunday date at noon to avoid timezone boundary issues
		// January 14, 2024 is a Sunday
		testDate := timezone.NewDate(2024, 1, 14)

		results, err := service.GetBulkEffectivePickupTimesForDate(ctx, []int64{student.ID}, testDate)

		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Nil(t, results[student.ID].PickupTime)
	})

	t.Run("returns schedule notes in bulk lookup", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Use a fixed Monday date at noon to avoid timezone boundary issues
		// January 8, 2024 is a Monday
		testDate := timezone.NewDate(2024, 1, 8)

		notes := "Picked up by aunt"
		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayMonday,
			PickupTime: time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
			Notes:      &notes,
			CreatedBy:  createPickupServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentPickupSchedule(ctx, sched)
		require.NoError(t, err)

		results, err := service.GetBulkEffectivePickupTimesForDate(ctx, []int64{student.ID}, testDate)

		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.NotNil(t, results[student.ID].PickupTime)
		assert.Equal(t, "Picked up by aunt", results[student.ID].Notes)
		assert.False(t, results[student.ID].IsException)
	})

	t.Run("falls back to recurring schedule notes in bulk lookup when exception reason is blank", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		testDate := timezone.NewDate(2024, 1, 8)

		recurringNote := "Ring the side entrance bell"
		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayMonday,
			PickupTime: time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
			Notes:      &recurringNote,
			CreatedBy:  createPickupServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentPickupSchedule(ctx, sched)
		require.NoError(t, err)

		updatedTime := time.Date(2024, 1, 1, 13, 15, 0, 0, time.UTC)
		blankReason := " "
		exception := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: testDate,
			PickupTime:    &updatedTime,
			Reason:        &blankReason,
			CreatedBy:     createPickupServiceTestStaffID(t, db),
		}
		err = service.CreateStudentPickupException(ctx, exception)
		require.NoError(t, err)

		results, err := service.GetBulkEffectivePickupTimesForDate(ctx, []int64{student.ID}, testDate)

		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.True(t, results[student.ID].IsException)
		assert.Equal(t, "Ring the side entrance bell", results[student.ID].Notes)
	})
}

// =============================================================================
// Note Operations Tests
// =============================================================================

func TestPickupScheduleService_CreateStudentPickupNote(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("creates note successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		note := &scheduleModels.StudentPickupNote{
			StudentID: student.ID,
			NoteDate:  timezone.NewDate(2024, 3, 15),
			Content:   "Please call before pickup",
			CreatedBy: createPickupServiceTestStaffID(t, db),
		}

		err := service.CreateStudentPickupNote(ctx, note)

		require.NoError(t, err)
		assert.Greater(t, note.ID, int64(0))
	})

	t.Run("fails validation for invalid note", func(t *testing.T) {
		note := &scheduleModels.StudentPickupNote{
			StudentID: 0, // Invalid
			NoteDate:  timezone.NewDate(2024, 3, 15),
			Content:   "Test",
			CreatedBy: createPickupServiceTestStaffID(t, db),
		}

		err := service.CreateStudentPickupNote(ctx, note)

		require.Error(t, err)
	})

	t.Run("fails validation for empty content", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		note := &scheduleModels.StudentPickupNote{
			StudentID: student.ID,
			NoteDate:  timezone.NewDate(2024, 3, 15),
			Content:   "",
			CreatedBy: createPickupServiceTestStaffID(t, db),
		}

		err := service.CreateStudentPickupNote(ctx, note)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "content is required")
	})
}

func TestPickupScheduleService_GetStudentPickupNoteByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns note by ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		note := &scheduleModels.StudentPickupNote{
			StudentID: student.ID,
			NoteDate:  timezone.NewDate(2024, 3, 16),
			Content:   "Test note",
			CreatedBy: createPickupServiceTestStaffID(t, db),
		}
		err := service.CreateStudentPickupNote(ctx, note)
		require.NoError(t, err)

		result, err := service.GetStudentPickupNoteByID(ctx, note.ID)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, note.ID, result.ID)
		assert.Equal(t, "Test note", result.Content)
	})
}

func TestPickupScheduleService_GetStudentPickupNotes(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns all notes for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		baseDate := timezone.TodayDate()
		for i := 0; i < 3; i++ {
			note := &scheduleModels.StudentPickupNote{
				StudentID: student.ID,
				NoteDate:  baseDate.AddDays(i),
				Content:   "Note content",
				CreatedBy: createPickupServiceTestStaffID(t, db),
			}
			err := service.CreateStudentPickupNote(ctx, note)
			require.NoError(t, err)
		}

		results, err := service.GetStudentPickupNotes(ctx, student.ID)

		require.NoError(t, err)
		assert.Len(t, results, 3)
	})

	t.Run("returns empty slice when no notes", func(t *testing.T) {
		results, err := service.GetStudentPickupNotes(ctx, int64(99999999))

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestPickupScheduleService_GetStudentPickupNotesForDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns notes for specific date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		targetDate := timezone.NewDate(2024, 3, 20)

		// Create notes for target date
		for i := 0; i < 2; i++ {
			note := &scheduleModels.StudentPickupNote{
				StudentID: student.ID,
				NoteDate:  targetDate,
				Content:   fmt.Sprintf("Note %d", i),
				CreatedBy: createPickupServiceTestStaffID(t, db),
			}
			err := service.CreateStudentPickupNote(ctx, note)
			require.NoError(t, err)
		}

		// Create note for different date
		differentDate := targetDate.AddDays(1)
		note := &scheduleModels.StudentPickupNote{
			StudentID: student.ID,
			NoteDate:  differentDate,
			Content:   "Different date note",
			CreatedBy: createPickupServiceTestStaffID(t, db),
		}
		err := service.CreateStudentPickupNote(ctx, note)
		require.NoError(t, err)

		results, err := service.GetStudentPickupNotesForDate(ctx, student.ID, targetDate)

		require.NoError(t, err)
		assert.Len(t, results, 2)
	})
}

func TestPickupScheduleService_UpdateStudentPickupNote(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("updates note successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		note := &scheduleModels.StudentPickupNote{
			StudentID: student.ID,
			NoteDate:  timezone.NewDate(2024, 4, 1),
			Content:   "Original content",
			CreatedBy: createPickupServiceTestStaffID(t, db),
		}
		err := service.CreateStudentPickupNote(ctx, note)
		require.NoError(t, err)

		note.Content = "Updated content"

		err = service.UpdateStudentPickupNote(ctx, note)

		require.NoError(t, err)

		notes, err := service.GetStudentPickupNotes(ctx, student.ID)
		require.NoError(t, err)
		assert.Len(t, notes, 1)
		assert.Equal(t, "Updated content", notes[0].Content)
	})

	t.Run("fails validation on invalid note", func(t *testing.T) {
		note := &scheduleModels.StudentPickupNote{
			StudentID: 0, // Invalid
			NoteDate:  timezone.NewDate(2024, 4, 1),
			Content:   "Test",
			CreatedBy: createPickupServiceTestStaffID(t, db),
		}

		err := service.UpdateStudentPickupNote(ctx, note)

		require.Error(t, err)
	})
}

func TestPickupScheduleService_DeleteStudentPickupNote(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("deletes note by ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		note := &scheduleModels.StudentPickupNote{
			StudentID: student.ID,
			NoteDate:  timezone.NewDate(2024, 5, 1),
			Content:   "Test",
			CreatedBy: createPickupServiceTestStaffID(t, db),
		}
		err := service.CreateStudentPickupNote(ctx, note)
		require.NoError(t, err)

		err = service.DeleteStudentPickupNote(ctx, note.ID)

		require.NoError(t, err)

		results, err := service.GetStudentPickupNotes(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestPickupScheduleService_DeleteAllStudentPickupNotes(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("deletes all notes for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		baseDate := timezone.TodayDate()
		for i := 1; i <= 5; i++ {
			note := &scheduleModels.StudentPickupNote{
				StudentID: student.ID,
				NoteDate:  baseDate.AddDays(i),
				Content:   "Note",
				CreatedBy: createPickupServiceTestStaffID(t, db),
			}
			err := service.CreateStudentPickupNote(ctx, note)
			require.NoError(t, err)
		}

		err := service.DeleteAllStudentPickupNotes(ctx, student.ID)

		require.NoError(t, err)

		results, err := service.GetStudentPickupNotes(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestPickupScheduleService_BulkUpsertPickupSchedules(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)
	studentA := testpkg.CreateTestStudent(t, db, "BulkPickup", "StudentA", "BP-A")
	studentB := testpkg.CreateTestStudent(t, db, "BulkPickup", "StudentB", "BP-B")
	defer testpkg.CleanupActivityFixtures(t, db, studentA.ID, studentB.ID)

	note := "Oma holt ab"
	require.NoError(t, service.UpsertStudentPickupSchedule(ctx, &scheduleModels.StudentPickupSchedule{
		StudentID:  studentA.ID,
		Weekday:    scheduleModels.WeekdayMonday,
		PickupTime: time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC),
		Notes:      &note,
		CreatedBy:  createPickupServiceTestStaffID(t, db),
	}))
	require.NoError(t, service.UpsertStudentPickupSchedule(ctx, &scheduleModels.StudentPickupSchedule{
		StudentID:  studentA.ID,
		Weekday:    scheduleModels.WeekdayTuesday,
		PickupTime: time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		CreatedBy:  createPickupServiceTestStaffID(t, db),
	}))

	result, err := service.BulkUpsertPickupSchedules(
		ctx,
		schedule.ArrivalScheduleBulkFilter{StudentIDs: []int64{studentA.ID, studentB.ID}},
		[]schedule.PickupScheduleInput{{Weekday: scheduleModels.WeekdayMonday, PickupTime: "16:10"}},
		createPickupServiceTestStaffID(t, db),
	)

	require.NoError(t, err)
	assert.Equal(t, 2, result.StudentsAffected)
	rowsA, err := service.GetStudentPickupSchedules(ctx, studentA.ID)
	require.NoError(t, err)
	require.Len(t, rowsA, 2, "unspecified weekdays must remain unchanged")
	byWeekday := make(map[int]*scheduleModels.StudentPickupSchedule, len(rowsA))
	for _, row := range rowsA {
		byWeekday[row.Weekday] = row
	}
	assert.Equal(t, "16:10", byWeekday[scheduleModels.WeekdayMonday].PickupTime.Format("15:04"))
	require.NotNil(t, byWeekday[scheduleModels.WeekdayMonday].Notes)
	assert.Equal(t, note, *byWeekday[scheduleModels.WeekdayMonday].Notes, "bulk time patches must preserve notes")
	assert.Equal(t, "15:00", byWeekday[scheduleModels.WeekdayTuesday].PickupTime.Format("15:04"))
}

func TestPickupScheduleService_BulkUpsertPickupSchedules_RollsBackUnauthorizedSelection(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)
	allowed := testpkg.CreateTestStudent(t, db, "BulkPickupAllowed", "Student", "BP-A")
	denied := testpkg.CreateTestStudent(t, db, "BulkPickupDenied", "Student", "BP-B")
	defer testpkg.CleanupActivityFixtures(t, db, allowed.ID, denied.ID)

	result, err := service.BulkUpsertPickupSchedules(ctx, schedule.ArrivalScheduleBulkFilter{
		StudentIDs: []int64{allowed.ID, denied.ID},
		Authorize: func(_ context.Context, student *usersModels.Student) (bool, error) {
			return student.ID == allowed.ID, nil
		},
	}, []schedule.PickupScheduleInput{{Weekday: 1, PickupTime: "16:10"}}, createPickupServiceTestStaffID(t, db))

	require.ErrorIs(t, err, schedule.ErrBulkStudentUnauthorized)
	assert.Nil(t, result)
	rows, findErr := service.GetStudentPickupSchedules(ctx, allowed.ID)
	require.NoError(t, findErr)
	assert.Empty(t, rows, "authorization failure must roll back every selected student")
}

// Production canUpdateStudent returns (false, err) on deny; that must map to
// ErrBulkStudentUnauthorized (HTTP 403), not a bare authorize error (HTTP 500).
func TestPickupScheduleService_BulkUpsertPickupSchedules_MapsAuthorizeErrorToUnauthorized(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	service := setupPickupScheduleService(t, db)
	ctx := testpkg.TenantContext(1)
	allowed := testpkg.CreateTestStudent(t, db, "BulkPickupAuthErrA", "Student", "BP-E1")
	denied := testpkg.CreateTestStudent(t, db, "BulkPickupAuthErrB", "Student", "BP-E2")
	defer testpkg.CleanupActivityFixtures(t, db, allowed.ID, denied.ID)

	result, err := service.BulkUpsertPickupSchedules(ctx, schedule.ArrivalScheduleBulkFilter{
		StudentIDs: []int64{allowed.ID, denied.ID},
		Authorize: func(_ context.Context, student *usersModels.Student) (bool, error) {
			if student.ID == allowed.ID {
				return true, nil
			}
			return false, errors.New("you can only update students in groups you supervise")
		},
	}, []schedule.PickupScheduleInput{{Weekday: 1, PickupTime: "16:10"}}, createPickupServiceTestStaffID(t, db))

	require.ErrorIs(t, err, schedule.ErrBulkStudentUnauthorized)
	assert.Nil(t, result)
	rows, findErr := service.GetStudentPickupSchedules(ctx, allowed.ID)
	require.NoError(t, findErr)
	assert.Empty(t, rows, "authorize error denial must not leave partial writes")
}
