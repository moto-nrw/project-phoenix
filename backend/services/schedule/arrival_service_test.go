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
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupArrivalScheduleService creates an ArrivalScheduleService with real database connection
func setupArrivalScheduleService(t *testing.T, db *bun.DB) schedule.ArrivalScheduleService {
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.ArrivalSchedule
}

func createArrivalServiceTestStaffID(t *testing.T, db *bun.DB) int64 {
	t.Helper()

	staff := testpkg.CreateTestStaff(t, db, "Arrival", fmt.Sprintf("Creator-%d", time.Now().UnixNano()))

	return staff.ID
}

// =============================================================================
// Schedule Operations Tests
// =============================================================================

func TestArrivalScheduleService_GetStudentArrivalSchedules(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns all schedules for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		for _, weekday := range []int{scheduleModels.WeekdayMonday, scheduleModels.WeekdayWednesday} {
			sched := &scheduleModels.StudentArrivalSchedule{
				StudentID:       student.ID,
				Weekday:         weekday,
				ExpectedArrival: time.Date(2024, 1, 1, 7, 50, 0, 0, time.UTC),
				CreatedBy:       createArrivalServiceTestStaffID(t, db),
			}
			err := service.UpsertStudentArrivalSchedule(ctx, sched)
			require.NoError(t, err)
		}

		results, err := service.GetStudentArrivalSchedules(ctx, student.ID)

		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("returns empty slice when no schedules", func(t *testing.T) {
		results, err := service.GetStudentArrivalSchedules(ctx, int64(99999999))

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestArrivalScheduleService_GetStudentArrivalScheduleForWeekday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns schedule for specific weekday", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		sched := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         scheduleModels.WeekdayTuesday,
			ExpectedArrival: time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC),
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentArrivalSchedule(ctx, sched)
		require.NoError(t, err)

		result, err := service.GetStudentArrivalScheduleForWeekday(ctx, student.ID, scheduleModels.WeekdayTuesday)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, scheduleModels.WeekdayTuesday, result.Weekday)
	})

	t.Run("returns error for invalid weekday", func(t *testing.T) {
		result, err := service.GetStudentArrivalScheduleForWeekday(ctx, int64(1), 10)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid weekday")
	})
}

func TestArrivalScheduleService_UpsertStudentArrivalSchedule(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates new schedule", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		sched := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         scheduleModels.WeekdayFriday,
			ExpectedArrival: time.Date(2024, 1, 1, 7, 30, 0, 0, time.UTC),
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}

		err := service.UpsertStudentArrivalSchedule(ctx, sched)

		require.NoError(t, err)
		assert.Greater(t, sched.ID, int64(0))
	})

	t.Run("updates existing schedule", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		sched := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         scheduleModels.WeekdayMonday,
			ExpectedArrival: time.Date(2024, 1, 1, 7, 30, 0, 0, time.UTC),
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentArrivalSchedule(ctx, sched)
		require.NoError(t, err)

		sched.ExpectedArrival = time.Date(2024, 1, 1, 8, 15, 0, 0, time.UTC)

		err = service.UpsertStudentArrivalSchedule(ctx, sched)

		require.NoError(t, err)

		result, err := service.GetStudentArrivalScheduleForWeekday(ctx, student.ID, scheduleModels.WeekdayMonday)
		require.NoError(t, err)
		assert.Equal(t, 8, result.ExpectedArrival.Hour())
	})

	t.Run("fails validation for invalid schedule", func(t *testing.T) {
		sched := &scheduleModels.StudentArrivalSchedule{
			StudentID:       0,
			Weekday:         scheduleModels.WeekdayMonday,
			ExpectedArrival: time.Date(2024, 1, 1, 7, 30, 0, 0, time.UTC),
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}

		err := service.UpsertStudentArrivalSchedule(ctx, sched)

		require.Error(t, err)
	})
}

func TestArrivalScheduleService_UpsertBulkStudentArrivalSchedules(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates multiple schedules in transaction", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		schedules := []*scheduleModels.StudentArrivalSchedule{
			{
				Weekday:         scheduleModels.WeekdayMonday,
				ExpectedArrival: time.Date(2024, 1, 1, 7, 30, 0, 0, time.UTC),
				CreatedBy:       createArrivalServiceTestStaffID(t, db),
			},
			{
				Weekday:         scheduleModels.WeekdayWednesday,
				ExpectedArrival: time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC),
				CreatedBy:       createArrivalServiceTestStaffID(t, db),
			},
			{
				Weekday:         scheduleModels.WeekdayFriday,
				ExpectedArrival: time.Date(2024, 1, 1, 7, 45, 0, 0, time.UTC),
				CreatedBy:       createArrivalServiceTestStaffID(t, db),
			},
		}

		err := service.UpsertBulkStudentArrivalSchedules(ctx, student.ID, schedules)

		require.NoError(t, err)

		results, err := service.GetStudentArrivalSchedules(ctx, student.ID)
		require.NoError(t, err)
		assert.Len(t, results, 3)
	})

	t.Run("rolls back on validation error", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		schedules := []*scheduleModels.StudentArrivalSchedule{
			{
				Weekday:         scheduleModels.WeekdayMonday,
				ExpectedArrival: time.Date(2024, 1, 1, 7, 30, 0, 0, time.UTC),
				CreatedBy:       createArrivalServiceTestStaffID(t, db),
			},
			{
				Weekday:         10,
				ExpectedArrival: time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC),
				CreatedBy:       createArrivalServiceTestStaffID(t, db),
			},
		}

		// Wrap in transaction so partial writes are rolled back on error
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		txCtx := base.ContextWithTx(ctx, &tx)

		err = service.UpsertBulkStudentArrivalSchedules(txCtx, student.ID, schedules)

		require.Error(t, err)
		require.NoError(t, tx.Rollback())

		results, err := service.GetStudentArrivalSchedules(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestArrivalScheduleService_UpsertBulkWaitsForStudentLock(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "ArrivalLock", "Student", "1a")
	staffID := createArrivalServiceTestStaffID(t, db)

	locked := make(chan struct{})
	release := make(chan struct{})
	lockDone := make(chan error, 1)
	go func() {
		lockDone <- tenant.WithTenantTx(ctx, db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
			if _, err := repos.Student.FindByIDForUpdate(txCtx, student.ID); err != nil {
				return err
			}
			close(locked)
			<-release
			return nil
		})
	}()

	select {
	case <-locked:
	case err := <-lockDone:
		require.NoError(t, err)
		t.Fatal("student transaction ended before holding the lock")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out while acquiring the student lock")
	}

	released := false
	releaseLock := func() {
		if !released {
			close(release)
			released = true
		}
	}
	defer releaseLock()

	writeDone := make(chan error, 1)
	go func() {
		writeDone <- service.UpsertBulkStudentArrivalSchedules(ctx, student.ID, []*scheduleModels.StudentArrivalSchedule{
			{
				Weekday:         scheduleModels.WeekdayMonday,
				ExpectedArrival: time.Date(2024, 1, 1, 8, 15, 0, 0, time.UTC),
				CreatedBy:       staffID,
			},
		})
	}()

	select {
	case err := <-writeDone:
		require.NoError(t, err)
		t.Fatal("arrival write bypassed the held student lock")
	case <-time.After(150 * time.Millisecond):
	}

	releaseLock()
	require.NoError(t, <-lockDone)
	select {
	case err := <-writeDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("arrival write did not resume after the student lock was released")
	}
}

func TestArrivalScheduleService_DeleteStudentArrivalSchedule(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes schedule by ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		sched := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         scheduleModels.WeekdayThursday,
			ExpectedArrival: time.Date(2024, 1, 1, 7, 50, 0, 0, time.UTC),
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentArrivalSchedule(ctx, sched)
		require.NoError(t, err)

		err = service.DeleteStudentArrivalSchedule(ctx, sched.ID)

		require.NoError(t, err)

		results, err := service.GetStudentArrivalSchedules(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestArrivalScheduleService_DeleteAllStudentArrivalSchedules(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes all schedules for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		for _, weekday := range []int{scheduleModels.WeekdayMonday, scheduleModels.WeekdayWednesday, scheduleModels.WeekdayFriday} {
			sched := &scheduleModels.StudentArrivalSchedule{
				StudentID:       student.ID,
				Weekday:         weekday,
				ExpectedArrival: time.Date(2024, 1, 1, 7, 50, 0, 0, time.UTC),
				CreatedBy:       createArrivalServiceTestStaffID(t, db),
			}
			err := service.UpsertStudentArrivalSchedule(ctx, sched)
			require.NoError(t, err)
		}

		err := service.DeleteAllStudentArrivalSchedules(ctx, student.ID)

		require.NoError(t, err)

		results, err := service.GetStudentArrivalSchedules(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

// =============================================================================
// Exception Operations Tests
// =============================================================================

func TestArrivalScheduleService_CreateStudentArrivalException(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates exception successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		exception := &scheduleModels.StudentArrivalException{
			StudentID:     student.ID,
			ExceptionDate: timezone.NewDate(2024, 3, 15),
			Reason:        testpkg.StrPtr("Doctor appointment"),
			CreatedBy:     createArrivalServiceTestStaffID(t, db),
		}

		err := service.CreateStudentArrivalException(ctx, exception)

		require.NoError(t, err)
		assert.Greater(t, exception.ID, int64(0))
	})

	t.Run("returns error when exception already exists for date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		exceptionDate := timezone.NewDate(2024, 3, 20)
		exception1 := &scheduleModels.StudentArrivalException{
			StudentID:     student.ID,
			ExceptionDate: exceptionDate,
			Reason:        testpkg.StrPtr("First exception"),
			CreatedBy:     createArrivalServiceTestStaffID(t, db),
		}
		err := service.CreateStudentArrivalException(ctx, exception1)
		require.NoError(t, err)

		exception2 := &scheduleModels.StudentArrivalException{
			StudentID:     student.ID,
			ExceptionDate: exceptionDate,
			Reason:        testpkg.StrPtr("Second exception"),
			CreatedBy:     createArrivalServiceTestStaffID(t, db),
		}
		err = service.CreateStudentArrivalException(ctx, exception2)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "exception already exists for this date")
	})

	t.Run("fails validation for invalid exception", func(t *testing.T) {
		exception := &scheduleModels.StudentArrivalException{
			StudentID:     0,
			ExceptionDate: timezone.NewDate(2024, 3, 15),
			Reason:        testpkg.StrPtr("Test"),
			CreatedBy:     createArrivalServiceTestStaffID(t, db),
		}

		err := service.CreateStudentArrivalException(ctx, exception)

		require.Error(t, err)
	})
}

func TestArrivalScheduleService_GetStudentArrivalExceptions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns all exceptions for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		baseDate := timezone.TodayDate()
		for i := -2; i <= 2; i++ {
			exception := &scheduleModels.StudentArrivalException{
				StudentID:     student.ID,
				ExceptionDate: baseDate.AddDays(i),
				Reason:        testpkg.StrPtr("Exception"),
				CreatedBy:     createArrivalServiceTestStaffID(t, db),
			}
			err := service.CreateStudentArrivalException(ctx, exception)
			require.NoError(t, err)
		}

		results, err := service.GetStudentArrivalExceptions(ctx, student.ID)

		require.NoError(t, err)
		assert.Len(t, results, 5)
	})
}

func TestArrivalScheduleService_GetUpcomingStudentArrivalExceptions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns only upcoming exceptions", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		baseDate := timezone.TodayDate()

		for i := -5; i < 0; i++ {
			exception := &scheduleModels.StudentArrivalException{
				StudentID:     student.ID,
				ExceptionDate: baseDate.AddDays(i),
				Reason:        testpkg.StrPtr("Past"),
				CreatedBy:     createArrivalServiceTestStaffID(t, db),
			}
			err := service.CreateStudentArrivalException(ctx, exception)
			require.NoError(t, err)
		}

		for i := 1; i <= 3; i++ {
			exception := &scheduleModels.StudentArrivalException{
				StudentID:     student.ID,
				ExceptionDate: baseDate.AddDays(i),
				Reason:        testpkg.StrPtr("Future"),
				CreatedBy:     createArrivalServiceTestStaffID(t, db),
			}
			err := service.CreateStudentArrivalException(ctx, exception)
			require.NoError(t, err)
		}

		results, err := service.GetUpcomingStudentArrivalExceptions(ctx, student.ID)

		require.NoError(t, err)
		assert.Len(t, results, 3)
		for _, result := range results {
			assert.Equal(t, "Future", *result.Reason)
		}
	})
}

func TestArrivalScheduleService_UpdateStudentArrivalException(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates exception successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		exception := &scheduleModels.StudentArrivalException{
			StudentID:     student.ID,
			ExceptionDate: timezone.NewDate(2024, 4, 1),
			Reason:        testpkg.StrPtr("Original reason"),
			CreatedBy:     createArrivalServiceTestStaffID(t, db),
		}
		err := service.CreateStudentArrivalException(ctx, exception)
		require.NoError(t, err)

		exception.Reason = testpkg.StrPtr("Updated reason")

		err = service.UpdateStudentArrivalException(ctx, exception)

		require.NoError(t, err)

		exceptions, err := service.GetStudentArrivalExceptions(ctx, student.ID)
		require.NoError(t, err)
		assert.Len(t, exceptions, 1)
		assert.Equal(t, "Updated reason", *exceptions[0].Reason)
	})
}

func TestArrivalScheduleService_UpdateExceptionPreservesOmittedArrivalTime(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	student := testpkg.CreateTestStudent(t, db, "Test", "ArrivalPatch", "1a")

	ctx := testpkg.TenantContext(student.TenantID)
	staffID := createArrivalServiceTestStaffID(t, db)
	exceptionDate := timezone.NewDate(2024, 4, 2)
	expectedArrival := time.Date(2000, 1, 1, 8, 15, 0, 0, time.UTC)
	exception := &scheduleModels.StudentArrivalException{
		StudentID:       student.ID,
		ExceptionDate:   exceptionDate,
		ExpectedArrival: &expectedArrival,
		Reason:          testpkg.StrPtr("Original reason"),
		CreatedBy:       staffID,
	}
	require.NoError(t, service.CreateStudentArrivalException(ctx, exception))

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
	require.NotNil(t, updated.ExpectedArrival)
	assert.Equal(t, expectedArrival.Format("15:04"), updated.ExpectedArrival.Format("15:04"))

	fresh, err := service.GetStudentArrivalExceptionByID(ctx, exception.ID)
	require.NoError(t, err)
	require.NotNil(t, fresh.ExpectedArrival)
	assert.Equal(t, expectedArrival.Format("15:04"), fresh.ExpectedArrival.Format("15:04"))
}

func TestArrivalScheduleService_UpdateExceptionClearsArrivalTime(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	student := testpkg.CreateTestStudent(t, db, "Test", "ArrivalClear", "1a")

	ctx := testpkg.TenantContext(student.TenantID)
	staffID := createArrivalServiceTestStaffID(t, db)
	exceptionDate := timezone.NewDate(2024, 4, 2)
	expectedArrival := time.Date(2000, 1, 1, 8, 15, 0, 0, time.UTC)
	exception := &scheduleModels.StudentArrivalException{
		StudentID:       student.ID,
		ExceptionDate:   exceptionDate,
		ExpectedArrival: &expectedArrival,
		Reason:          testpkg.StrPtr("Original reason"),
		CreatedBy:       staffID,
	}
	require.NoError(t, service.CreateStudentArrivalException(ctx, exception))
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
	assert.Nil(t, updated.ExpectedArrival)
	assert.Nil(t, updated.Reason)
}

func TestArrivalScheduleService_DeleteStudentArrivalException(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes exception by ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		exception := &scheduleModels.StudentArrivalException{
			StudentID:     student.ID,
			ExceptionDate: timezone.NewDate(2024, 5, 1),
			Reason:        testpkg.StrPtr("Test"),
			CreatedBy:     createArrivalServiceTestStaffID(t, db),
		}
		err := service.CreateStudentArrivalException(ctx, exception)
		require.NoError(t, err)

		err = service.DeleteStudentArrivalException(ctx, exception.ID)

		require.NoError(t, err)

		results, err := service.GetStudentArrivalExceptions(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestArrivalScheduleService_DeleteAllStudentArrivalExceptions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes all exceptions for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		baseDate := timezone.TodayDate()
		for i := 1; i <= 5; i++ {
			exception := &scheduleModels.StudentArrivalException{
				StudentID:     student.ID,
				ExceptionDate: baseDate.AddDays(i),
				Reason:        testpkg.StrPtr("Exception"),
				CreatedBy:     createArrivalServiceTestStaffID(t, db),
			}
			err := service.CreateStudentArrivalException(ctx, exception)
			require.NoError(t, err)
		}

		err := service.DeleteAllStudentArrivalExceptions(ctx, student.ID)

		require.NoError(t, err)

		results, err := service.GetStudentArrivalExceptions(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

// =============================================================================
// Note Operations Tests
// =============================================================================

func TestArrivalScheduleService_CreateStudentArrivalNote(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates note successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		note := &scheduleModels.StudentArrivalNote{
			StudentID: student.ID,
			NoteDate:  timezone.NewDate(2024, 3, 15),
			Content:   "Arrives by bus today",
			CreatedBy: createArrivalServiceTestStaffID(t, db),
		}

		err := service.CreateStudentArrivalNote(ctx, note)

		require.NoError(t, err)
		assert.Greater(t, note.ID, int64(0))
	})

	t.Run("fails validation for invalid note", func(t *testing.T) {
		note := &scheduleModels.StudentArrivalNote{
			StudentID: 0, // Invalid
			NoteDate:  timezone.NewDate(2024, 3, 15),
			Content:   "Test",
			CreatedBy: createArrivalServiceTestStaffID(t, db),
		}

		err := service.CreateStudentArrivalNote(ctx, note)

		require.Error(t, err)
	})

	t.Run("fails validation for empty content", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		note := &scheduleModels.StudentArrivalNote{
			StudentID: student.ID,
			NoteDate:  timezone.NewDate(2024, 3, 15),
			Content:   "",
			CreatedBy: createArrivalServiceTestStaffID(t, db),
		}

		err := service.CreateStudentArrivalNote(ctx, note)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "content is required")
	})
}

func TestArrivalScheduleService_GetStudentArrivalNoteByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns note by ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		note := &scheduleModels.StudentArrivalNote{
			StudentID: student.ID,
			NoteDate:  timezone.NewDate(2024, 3, 16),
			Content:   "Test note",
			CreatedBy: createArrivalServiceTestStaffID(t, db),
		}
		err := service.CreateStudentArrivalNote(ctx, note)
		require.NoError(t, err)

		result, err := service.GetStudentArrivalNoteByID(ctx, note.ID)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, note.ID, result.ID)
		assert.Equal(t, "Test note", result.Content)
	})
}

func TestArrivalScheduleService_GetStudentArrivalNotes(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns all notes for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		baseDate := timezone.TodayDate()
		for i := 0; i < 3; i++ {
			note := &scheduleModels.StudentArrivalNote{
				StudentID: student.ID,
				NoteDate:  baseDate.AddDays(i),
				Content:   "Note content",
				CreatedBy: createArrivalServiceTestStaffID(t, db),
			}
			err := service.CreateStudentArrivalNote(ctx, note)
			require.NoError(t, err)
		}

		results, err := service.GetStudentArrivalNotes(ctx, student.ID)

		require.NoError(t, err)
		assert.Len(t, results, 3)
	})

	t.Run("returns empty slice when no notes", func(t *testing.T) {
		results, err := service.GetStudentArrivalNotes(ctx, int64(99999999))

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestArrivalScheduleService_GetStudentArrivalNotesForDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns notes for specific date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		targetDate := timezone.NewDate(2024, 3, 20)

		// Create notes for target date
		for i := 0; i < 2; i++ {
			note := &scheduleModels.StudentArrivalNote{
				StudentID: student.ID,
				NoteDate:  targetDate,
				Content:   fmt.Sprintf("Note %d", i),
				CreatedBy: createArrivalServiceTestStaffID(t, db),
			}
			err := service.CreateStudentArrivalNote(ctx, note)
			require.NoError(t, err)
		}

		// Create note for different date
		differentDate := targetDate.AddDays(1)
		note := &scheduleModels.StudentArrivalNote{
			StudentID: student.ID,
			NoteDate:  differentDate,
			Content:   "Different date note",
			CreatedBy: createArrivalServiceTestStaffID(t, db),
		}
		err := service.CreateStudentArrivalNote(ctx, note)
		require.NoError(t, err)

		results, err := service.GetStudentArrivalNotesForDate(ctx, student.ID, targetDate)

		require.NoError(t, err)
		assert.Len(t, results, 2)
	})
}

func TestArrivalScheduleService_UpdateStudentArrivalNote(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates note successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		note := &scheduleModels.StudentArrivalNote{
			StudentID: student.ID,
			NoteDate:  timezone.NewDate(2024, 4, 1),
			Content:   "Original content",
			CreatedBy: createArrivalServiceTestStaffID(t, db),
		}
		err := service.CreateStudentArrivalNote(ctx, note)
		require.NoError(t, err)

		note.Content = "Updated content"

		err = service.UpdateStudentArrivalNote(ctx, note)

		require.NoError(t, err)

		notes, err := service.GetStudentArrivalNotes(ctx, student.ID)
		require.NoError(t, err)
		assert.Len(t, notes, 1)
		assert.Equal(t, "Updated content", notes[0].Content)
	})

	t.Run("fails validation on invalid note", func(t *testing.T) {
		note := &scheduleModels.StudentArrivalNote{
			StudentID: 0, // Invalid
			NoteDate:  timezone.NewDate(2024, 4, 1),
			Content:   "Test",
			CreatedBy: createArrivalServiceTestStaffID(t, db),
		}

		err := service.UpdateStudentArrivalNote(ctx, note)

		require.Error(t, err)
	})
}

func TestArrivalScheduleService_DeleteStudentArrivalNote(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes note by ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		note := &scheduleModels.StudentArrivalNote{
			StudentID: student.ID,
			NoteDate:  timezone.NewDate(2024, 5, 1),
			Content:   "Test",
			CreatedBy: createArrivalServiceTestStaffID(t, db),
		}
		err := service.CreateStudentArrivalNote(ctx, note)
		require.NoError(t, err)

		err = service.DeleteStudentArrivalNote(ctx, note.ID)

		require.NoError(t, err)

		results, err := service.GetStudentArrivalNotes(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestArrivalScheduleService_DeleteAllStudentArrivalNotes(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes all notes for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		baseDate := timezone.TodayDate()
		for i := 1; i <= 5; i++ {
			note := &scheduleModels.StudentArrivalNote{
				StudentID: student.ID,
				NoteDate:  baseDate.AddDays(i),
				Content:   "Note",
				CreatedBy: createArrivalServiceTestStaffID(t, db),
			}
			err := service.CreateStudentArrivalNote(ctx, note)
			require.NoError(t, err)
		}

		err := service.DeleteAllStudentArrivalNotes(ctx, student.ID)

		require.NoError(t, err)

		results, err := service.GetStudentArrivalNotes(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

// =============================================================================
// Computed Operations Tests
// =============================================================================

func TestArrivalScheduleService_GetStudentArrivalData(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns combined schedule, exception, and note data", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		sched := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         scheduleModels.WeekdayMonday,
			ExpectedArrival: time.Date(2024, 1, 1, 7, 50, 0, 0, time.UTC),
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentArrivalSchedule(ctx, sched)
		require.NoError(t, err)

		exception := &scheduleModels.StudentArrivalException{
			StudentID:     student.ID,
			ExceptionDate: timezone.TodayDate().AddDays(5),
			Reason:        testpkg.StrPtr("Future exception"),
			CreatedBy:     createArrivalServiceTestStaffID(t, db),
		}
		err = service.CreateStudentArrivalException(ctx, exception)
		require.NoError(t, err)

		note := &scheduleModels.StudentArrivalNote{
			StudentID: student.ID,
			NoteDate:  timezone.TodayDate(),
			Content:   "Test note",
			CreatedBy: createArrivalServiceTestStaffID(t, db),
		}
		err = service.CreateStudentArrivalNote(ctx, note)
		require.NoError(t, err)

		result, err := service.GetStudentArrivalData(ctx, student.ID)

		require.NoError(t, err)
		assert.Len(t, result.Schedules, 1)
		assert.Len(t, result.Exceptions, 1)
		assert.Len(t, result.Notes, 1)
	})
}

func TestArrivalScheduleService_GetEffectiveArrivalTimeForDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns exception when present", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		recurringNote := "Arrives at side gate"
		sched := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         scheduleModels.WeekdayMonday,
			ExpectedArrival: time.Date(2024, 1, 1, 7, 50, 0, 0, time.UTC),
			Notes:           &recurringNote,
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentArrivalSchedule(ctx, sched)
		require.NoError(t, err)

		// January 8, 2024 is a Monday, noon UTC stays Monday in Berlin
		testDate := timezone.NewDate(2024, 1, 8)

		earlyTime := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
		exception := &scheduleModels.StudentArrivalException{
			StudentID:       student.ID,
			ExceptionDate:   testDate,
			ExpectedArrival: &earlyTime,
			Reason:          testpkg.StrPtr("Late arrival"),
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}
		err = service.CreateStudentArrivalException(ctx, exception)
		require.NoError(t, err)

		result, err := service.GetEffectiveArrivalTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.True(t, result.IsException)
		assert.NotNil(t, result.ArrivalTime)
		assert.Equal(t, 9, result.ArrivalTime.Hour())
		assert.Equal(t, "Late arrival", result.Notes)
	})

	t.Run("returns schedule when no exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		sched := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         scheduleModels.WeekdayTuesday,
			ExpectedArrival: time.Date(2024, 1, 1, 8, 15, 0, 0, time.UTC),
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentArrivalSchedule(ctx, sched)
		require.NoError(t, err)

		// January 9, 2024 is a Tuesday, noon UTC stays Tuesday in Berlin
		testDate := timezone.NewDate(2024, 1, 9)

		result, err := service.GetEffectiveArrivalTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.False(t, result.IsException)
		assert.NotNil(t, result.ArrivalTime)
		assert.Equal(t, 8, result.ArrivalTime.Hour())
		assert.Equal(t, 15, result.ArrivalTime.Minute())
	})

	t.Run("returns nil arrival time for weekend", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		// January 13, 2024 is a Saturday, noon UTC stays Saturday in Berlin
		testDate := timezone.NewDate(2024, 1, 13)

		result, err := service.GetEffectiveArrivalTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.Nil(t, result.ArrivalTime)
	})

	t.Run("returns nil when no schedule and no exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		// January 10, 2024 is a Wednesday
		testDate := timezone.NewDate(2024, 1, 10)

		result, err := service.GetEffectiveArrivalTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.Nil(t, result.ArrivalTime)
		assert.False(t, result.IsException)
	})

	t.Run("returns schedule with notes", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		notes := "Walks with sibling"
		sched := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         scheduleModels.WeekdayFriday,
			ExpectedArrival: time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC),
			Notes:           &notes,
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentArrivalSchedule(ctx, sched)
		require.NoError(t, err)

		// January 12, 2024 is a Friday
		testDate := timezone.NewDate(2024, 1, 12)

		result, err := service.GetEffectiveArrivalTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.False(t, result.IsException)
		assert.NotNil(t, result.ArrivalTime)
		assert.Equal(t, "Walks with sibling", result.Notes)
	})

	t.Run("falls back to recurring schedule notes when exception reason is blank", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		recurringNote := "Wait at the side entrance"
		sched := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         scheduleModels.WeekdayMonday,
			ExpectedArrival: time.Date(2024, 1, 1, 7, 50, 0, 0, time.UTC),
			Notes:           &recurringNote,
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentArrivalSchedule(ctx, sched)
		require.NoError(t, err)

		testDate := timezone.NewDate(2024, 1, 8)
		updatedTime := time.Date(2024, 1, 1, 9, 30, 0, 0, time.UTC)
		blankReason := "   "
		exception := &scheduleModels.StudentArrivalException{
			StudentID:       student.ID,
			ExceptionDate:   testDate,
			ExpectedArrival: &updatedTime,
			Reason:          &blankReason,
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}
		err = service.CreateStudentArrivalException(ctx, exception)
		require.NoError(t, err)

		result, err := service.GetEffectiveArrivalTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.True(t, result.IsException)
		assert.Equal(t, "Wait at the side entrance", result.Notes)
	})

	t.Run("handles Sunday correctly", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		// January 14, 2024 is a Sunday
		testDate := timezone.NewDate(2024, 1, 14)

		result, err := service.GetEffectiveArrivalTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.Nil(t, result.ArrivalTime)
	})

	t.Run("absent exception returns nil arrival time", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		// January 8, 2024 is a Monday
		testDate := timezone.NewDate(2024, 1, 8)

		exception := &scheduleModels.StudentArrivalException{
			StudentID:       student.ID,
			ExceptionDate:   testDate,
			ExpectedArrival: nil, // absent
			Reason:          testpkg.StrPtr("Sick"),
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}
		err := service.CreateStudentArrivalException(ctx, exception)
		require.NoError(t, err)

		result, err := service.GetEffectiveArrivalTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.True(t, result.IsException)
		assert.Nil(t, result.ArrivalTime)
		assert.Equal(t, "Sick", result.Notes)
	})
}

func TestArrivalScheduleService_GetBulkEffectiveArrivalTimesForDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns effective times for multiple students", func(t *testing.T) {
		student1 := testpkg.CreateTestStudent(t, db, "ArrStudent", "One", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "ArrStudent", "Two", "1b")
		student3 := testpkg.CreateTestStudent(t, db, "ArrStudent", "Three", "1c")

		// January 11, 2024 is a Thursday
		testDate := timezone.NewDate(2024, 1, 11)

		sched1 := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student1.ID,
			Weekday:         scheduleModels.WeekdayThursday,
			ExpectedArrival: time.Date(2024, 1, 1, 7, 50, 0, 0, time.UTC),
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentArrivalSchedule(ctx, sched1)
		require.NoError(t, err)

		earlyTime := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
		exception2 := &scheduleModels.StudentArrivalException{
			StudentID:       student2.ID,
			ExceptionDate:   testDate,
			ExpectedArrival: &earlyTime,
			Reason:          testpkg.StrPtr("Doctor appointment"),
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}
		err = service.CreateStudentArrivalException(ctx, exception2)
		require.NoError(t, err)

		exception3 := &scheduleModels.StudentArrivalException{
			StudentID:       student3.ID,
			ExceptionDate:   testDate,
			ExpectedArrival: nil,
			Reason:          testpkg.StrPtr("Sick"),
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}
		err = service.CreateStudentArrivalException(ctx, exception3)
		require.NoError(t, err)

		results, err := service.GetBulkEffectiveArrivalTimesForDate(ctx, []int64{student1.ID, student2.ID, student3.ID}, testDate)

		require.NoError(t, err)
		assert.Len(t, results, 3)

		assert.False(t, results[student1.ID].IsException)
		assert.NotNil(t, results[student1.ID].ArrivalTime)
		assert.Equal(t, 7, results[student1.ID].ArrivalTime.Hour())

		assert.True(t, results[student2.ID].IsException)
		assert.NotNil(t, results[student2.ID].ArrivalTime)
		assert.Equal(t, 9, results[student2.ID].ArrivalTime.Hour())
		assert.Equal(t, "Doctor appointment", results[student2.ID].Notes)

		assert.True(t, results[student3.ID].IsException)
		assert.Nil(t, results[student3.ID].ArrivalTime)
		assert.Equal(t, "Sick", results[student3.ID].Notes)
	})

	t.Run("returns empty map for empty student IDs", func(t *testing.T) {
		results, err := service.GetBulkEffectiveArrivalTimesForDate(ctx, []int64{}, timezone.TodayDate())

		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("handles weekend correctly for all students", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		// January 14, 2024 is a Sunday
		testDate := timezone.NewDate(2024, 1, 14)

		results, err := service.GetBulkEffectiveArrivalTimesForDate(ctx, []int64{student.ID}, testDate)

		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Nil(t, results[student.ID].ArrivalTime)
	})

	t.Run("returns schedule notes in bulk lookup", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		// January 8, 2024 is a Monday
		testDate := timezone.NewDate(2024, 1, 8)

		notes := "Walked to school by aunt"
		sched := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         scheduleModels.WeekdayMonday,
			ExpectedArrival: time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC),
			Notes:           &notes,
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentArrivalSchedule(ctx, sched)
		require.NoError(t, err)

		results, err := service.GetBulkEffectiveArrivalTimesForDate(ctx, []int64{student.ID}, testDate)

		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.NotNil(t, results[student.ID].ArrivalTime)
		assert.Equal(t, "Walked to school by aunt", results[student.ID].Notes)
		assert.False(t, results[student.ID].IsException)
	})

	t.Run("falls back to recurring schedule notes in bulk lookup when exception reason is blank", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		testDate := timezone.NewDate(2024, 1, 8)

		recurringNote := "Ring the side entrance bell"
		sched := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         scheduleModels.WeekdayMonday,
			ExpectedArrival: time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC),
			Notes:           &recurringNote,
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentArrivalSchedule(ctx, sched)
		require.NoError(t, err)

		updatedTime := time.Date(2024, 1, 1, 9, 15, 0, 0, time.UTC)
		blankReason := " "
		exception := &scheduleModels.StudentArrivalException{
			StudentID:       student.ID,
			ExceptionDate:   testDate,
			ExpectedArrival: &updatedTime,
			Reason:          &blankReason,
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}
		err = service.CreateStudentArrivalException(ctx, exception)
		require.NoError(t, err)

		results, err := service.GetBulkEffectiveArrivalTimesForDate(ctx, []int64{student.ID}, testDate)

		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.True(t, results[student.ID].IsException)
		assert.Equal(t, "Ring the side entrance bell", results[student.ID].Notes)
	})
}

// =============================================================================
// Bulk Filtered Upsert Tests
// =============================================================================

func TestArrivalScheduleService_BulkUpsertArrivalSchedules(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupArrivalScheduleService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error without a filter", func(t *testing.T) {
		result, err := service.BulkUpsertArrivalSchedules(ctx, schedule.ArrivalScheduleBulkFilter{}, []schedule.ArrivalScheduleInput{
			{Weekday: 1, ArrivalTime: "08:00"},
		}, createArrivalServiceTestStaffID(t, db))

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "exactly one bulk filter is required")
	})

	t.Run("returns error with class and group filters", func(t *testing.T) {
		result, err := service.BulkUpsertArrivalSchedules(ctx, schedule.ArrivalScheduleBulkFilter{
			SchoolClass: "1a",
			GroupID:     42,
		}, []schedule.ArrivalScheduleInput{
			{Weekday: 1, ArrivalTime: "08:00"},
		}, createArrivalServiceTestStaffID(t, db))

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "exactly one bulk filter is required")
	})

	t.Run("upserts schedules only for explicitly selected students", func(t *testing.T) {
		selectedA := testpkg.CreateTestStudent(t, db, "BulkSelected", "StudentA", "SEL-A")
		selectedB := testpkg.CreateTestStudent(t, db, "BulkSelected", "StudentB", "SEL-B")
		unselected := testpkg.CreateTestStudent(t, db, "BulkSelected", "StudentC", "SEL-A")
		note := "Förderkurs"
		err := service.UpsertStudentArrivalSchedule(ctx, &scheduleModels.StudentArrivalSchedule{
			StudentID: selectedA.ID, Weekday: 4,
			ExpectedArrival: time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC),
			Notes:           &note, CreatedBy: createArrivalServiceTestStaffID(t, db),
		})
		require.NoError(t, err)

		result, err := service.BulkUpsertArrivalSchedules(
			ctx,
			schedule.ArrivalScheduleBulkFilter{StudentIDs: []int64{selectedA.ID, selectedB.ID}},
			[]schedule.ArrivalScheduleInput{{Weekday: 4, ArrivalTime: "10:05"}},
			createArrivalServiceTestStaffID(t, db),
		)

		require.NoError(t, err)
		assert.ElementsMatch(t, []int64{selectedA.ID, selectedB.ID}, result.AffectedStudentIDs)
		for _, studentID := range []int64{selectedA.ID, selectedB.ID} {
			rows, findErr := service.GetStudentArrivalSchedules(ctx, studentID)
			require.NoError(t, findErr)
			require.Len(t, rows, 1)
			assert.Equal(t, "10:05", rows[0].ExpectedArrival.Format("15:04"))
			if studentID == selectedA.ID {
				require.NotNil(t, rows[0].Notes)
				assert.Equal(t, note, *rows[0].Notes)
			}
		}
		rows, findErr := service.GetStudentArrivalSchedules(ctx, unselected.ID)
		require.NoError(t, findErr)
		assert.Empty(t, rows)
	})

	t.Run("rejects the whole explicit selection when one student is unauthorized", func(t *testing.T) {
		allowed := testpkg.CreateTestStudent(t, db, "BulkAuthorized", "Student", "AUTH-A")
		denied := testpkg.CreateTestStudent(t, db, "BulkDenied", "Student", "AUTH-B")

		result, err := service.BulkUpsertArrivalSchedules(
			ctx,
			schedule.ArrivalScheduleBulkFilter{
				StudentIDs: []int64{allowed.ID, denied.ID},
				Authorize: func(_ context.Context, student *usersModels.Student) (bool, error) {
					return student.ID == allowed.ID, nil
				},
			},
			[]schedule.ArrivalScheduleInput{{Weekday: 1, ArrivalTime: "08:40"}},
			createArrivalServiceTestStaffID(t, db),
		)

		require.ErrorIs(t, err, schedule.ErrBulkStudentUnauthorized)
		assert.Nil(t, result)
		rows, findErr := service.GetStudentArrivalSchedules(ctx, allowed.ID)
		require.NoError(t, findErr)
		assert.Empty(t, rows, "authorization failure must roll back every selected student")
	})

	t.Run("rejects a class timetable update when one matched student is unauthorized", func(t *testing.T) {
		className := fmt.Sprintf("BulkClassAuth-%d", time.Now().UnixNano())
		allowed := testpkg.CreateTestStudent(t, db, "BulkClassAuth", "Allowed", className)
		testpkg.CreateTestStudent(t, db, "BulkClassAuth", "Denied", className)

		result, err := service.BulkUpsertArrivalSchedules(
			ctx,
			schedule.ArrivalScheduleBulkFilter{
				SchoolClass: className,
				Authorize: func(_ context.Context, student *usersModels.Student) (bool, error) {
					return student.ID == allowed.ID, nil
				},
			},
			[]schedule.ArrivalScheduleInput{{Weekday: 1, ArrivalTime: "08:40"}},
			createArrivalServiceTestStaffID(t, db),
		)

		require.ErrorIs(t, err, schedule.ErrBulkStudentUnauthorized)
		assert.Nil(t, result)
		classTimes, findErr := service.GetClassArrivalTimes(ctx, className)
		require.NoError(t, findErr)
		assert.Empty(t, classTimes.Times, "authorization failure must not write the shared class timetable")
	})

	// Production canUpdateStudent returns (false, err) on deny; that must map to
	// ErrBulkStudentUnauthorized (HTTP 403), not a bare authorize error (HTTP 500).
	t.Run("maps production-style authorize denial to unauthorized", func(t *testing.T) {
		allowed := testpkg.CreateTestStudent(t, db, "BulkAuthErrAllowed", "Student", "AUTH-E1")
		denied := testpkg.CreateTestStudent(t, db, "BulkAuthErrDenied", "Student", "AUTH-E2")

		result, err := service.BulkUpsertArrivalSchedules(
			ctx,
			schedule.ArrivalScheduleBulkFilter{
				StudentIDs: []int64{allowed.ID, denied.ID},
				Authorize: func(_ context.Context, student *usersModels.Student) (bool, error) {
					if student.ID == allowed.ID {
						return true, nil
					}
					return false, errors.New("you can only update students in groups you supervise")
				},
			},
			[]schedule.ArrivalScheduleInput{{Weekday: 1, ArrivalTime: "08:40"}},
			createArrivalServiceTestStaffID(t, db),
		)

		require.ErrorIs(t, err, schedule.ErrBulkStudentUnauthorized)
		assert.Nil(t, result)
		rows, findErr := service.GetStudentArrivalSchedules(ctx, allowed.ID)
		require.NoError(t, findErr)
		assert.Empty(t, rows, "authorize error denial must not leave partial writes")
	})

	t.Run("returns error for empty schedules", func(t *testing.T) {
		result, err := service.BulkUpsertArrivalSchedules(ctx, schedule.ArrivalScheduleBulkFilter{SchoolClass: "1a"}, []schedule.ArrivalScheduleInput{}, createArrivalServiceTestStaffID(t, db))

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "schedules cannot be empty")
	})

	t.Run("returns zero students affected for empty class", func(t *testing.T) {
		result, err := service.BulkUpsertArrivalSchedules(ctx, schedule.ArrivalScheduleBulkFilter{SchoolClass: "NONEXISTENT_CLASS_XYZ"}, []schedule.ArrivalScheduleInput{
			{Weekday: 1, ArrivalTime: "08:00"},
		}, createArrivalServiceTestStaffID(t, db))

		require.NoError(t, err)
		assert.Equal(t, 0, result.StudentsAffected)
	})

	// Class names must be unique per run. BulkUpsertArrivalSchedules finds
	// students by school_class string match, and the cleanup path for
	// users.students is a known no-op (see backend/test/fixtures.go —
	// cleanupDelete errors silently on `.Model((*interface{})(nil))`),
	// so stale students with reused class names accumulate across local
	// re-runs and inflate StudentsAffected. CI isn't affected because it
	// starts each run from a fresh test DB. Hermetic tests should not
	// reuse the same school_class literal on a long-lived test DB.
	// Business rule changed with #2414 / ADR 0005: setting the time for a
	// school class writes the class timetable, it does not invent care days.
	// A child is in care on a weekday because it is booked or because the OGS
	// entered that day, never because its class has lessons then — otherwise
	// children get arrival times on days they do not attend, which is the
	// confusion this issue exists to remove.
	t.Run("sets the class timetable without inventing care days", func(t *testing.T) {
		className := fmt.Sprintf("BC1-%d", time.Now().UnixNano())
		staffID := createArrivalServiceTestStaffID(t, db)

		withCareDay := testpkg.CreateTestStudent(t, db, "BulkArr", "Student1", className)
		withoutCareDay := testpkg.CreateTestStudent(t, db, "BulkArr", "Student2", className)
		testpkg.CreateTestArrivalSchedule(t, db, withCareDay.ID, 1, staffID, "")

		schedules := []schedule.ArrivalScheduleInput{
			{Weekday: 1, ArrivalTime: "07:45"},
			{Weekday: 3, ArrivalTime: "08:15"},
		}

		result, err := service.BulkUpsertArrivalSchedules(ctx, schedule.ArrivalScheduleBulkFilter{SchoolClass: className}, schedules, staffID)

		require.NoError(t, err)
		assert.Equal(t, 2, result.StudentsAffected)

		inCare, err := service.GetStudentArrivalSchedules(ctx, withCareDay.ID)
		require.NoError(t, err)
		require.Len(t, inCare, 1, "the child keeps its one care day, Wednesday is not invented")
		assert.Equal(t, "07:45", inCare[0].ExpectedArrival.Format("15:04"))

		none, err := service.GetStudentArrivalSchedules(ctx, withoutCareDay.ID)
		require.NoError(t, err)
		assert.Empty(t, none, "a child without care days gets no arrival time from its class")
	})

	// Naming a class means "everyone in it", and a child whose care has ended
	// is not in it any more (#2487). Before this, the class-wide write aborted
	// at the locked re-check as soon as ONE child of the class had left —
	// with a message naming a child the office cannot even see any more.
	t.Run("skips children whose care has ended instead of failing the class", func(t *testing.T) {
		className := fmt.Sprintf("BCCARE-%d", time.Now().UnixNano())
		staffID := createArrivalServiceTestStaffID(t, db)
		staying := testpkg.CreateTestStudent(t, db, "BulkCare", "Bleibt", className)
		departed := testpkg.CreateTestStudent(t, db, "BulkCare", "Weg", className)
		testpkg.CreateTestArrivalSchedule(t, db, staying.ID, scheduleModels.WeekdayMonday, staffID, "")
		_, err := db.NewUpdate().
			Table("users.students").
			Set("enrolled_until = ?", timezone.TodayDate().AddDays(-1)).
			Where("id = ?", departed.ID).
			Exec(ctx)
		require.NoError(t, err)

		schedules := []schedule.ArrivalScheduleInput{{Weekday: 1, ArrivalTime: "07:45"}}
		result, err := service.BulkUpsertArrivalSchedules(
			ctx,
			schedule.ArrivalScheduleBulkFilter{SchoolClass: className},
			schedules,
			staffID,
		)

		require.NoError(t, err, "one departed child must not abort the whole class")
		assert.Equal(t, 1, result.StudentsAffected)

		kept, err := service.GetStudentArrivalSchedules(ctx, staying.ID)
		require.NoError(t, err)
		assert.Len(t, kept, 1, "the existing care day remains a class-time marker")

		classTimes, err := service.GetClassArrivalTimes(ctx, className)
		require.NoError(t, err)
		require.NotNil(t, classTimes)
		assert.Equal(t, "07:45", classTimes.Times["mon"])

		none, err := service.GetStudentArrivalSchedules(ctx, departed.ID)
		require.NoError(t, err)
		assert.Empty(t, none, "a departed child gets no new weekly plan")
	})

	t.Run("upserts schedules only for students in the selected group", func(t *testing.T) {
		targetGroup := testpkg.CreateTestEducationGroup(t, db, "BulkArrivalTarget")
		otherGroup := testpkg.CreateTestEducationGroup(t, db, "BulkArrivalOther")
		targetStudent1 := testpkg.CreateTestStudent(t, db, "BulkGroup", "Student1", "1a")
		targetStudent2 := testpkg.CreateTestStudent(t, db, "BulkGroup", "Student2", "2b")
		otherStudent := testpkg.CreateTestStudent(t, db, "BulkOther", "Student", "1a")

		testpkg.AssignStudentToGroup(t, db, targetStudent1.ID, targetGroup.ID)
		testpkg.AssignStudentToGroup(t, db, targetStudent2.ID, targetGroup.ID)
		testpkg.AssignStudentToGroup(t, db, otherStudent.ID, otherGroup.ID)

		result, err := service.BulkUpsertArrivalSchedules(
			ctx,
			schedule.ArrivalScheduleBulkFilter{GroupID: targetGroup.ID},
			[]schedule.ArrivalScheduleInput{{Weekday: 2, ArrivalTime: "09:15"}},
			createArrivalServiceTestStaffID(t, db),
		)

		require.NoError(t, err)
		assert.Equal(t, 2, result.StudentsAffected)
		assert.ElementsMatch(t, []int64{targetStudent1.ID, targetStudent2.ID}, result.AffectedStudentIDs)

		for _, studentID := range []int64{targetStudent1.ID, targetStudent2.ID} {
			rows, findErr := service.GetStudentArrivalSchedules(ctx, studentID)
			require.NoError(t, findErr)
			require.Len(t, rows, 1)
			assert.Equal(t, "09:15", rows[0].ExpectedArrival.Format("15:04"))
		}
		otherRows, findErr := service.GetStudentArrivalSchedules(ctx, otherStudent.ID)
		require.NoError(t, findErr)
		assert.Empty(t, otherRows)
	})

	t.Run("preserves an own time when the class time changes", func(t *testing.T) {
		className := fmt.Sprintf("BOW1-%d", time.Now().UnixNano())
		student := testpkg.CreateTestStudent(t, db, "BulkOverwrite", "Student", className)

		// Create existing schedule with a different time
		existingSched := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         scheduleModels.WeekdayMonday,
			ExpectedArrival: time.Date(2024, 1, 1, 7, 30, 0, 0, time.UTC),
			CreatedBy:       createArrivalServiceTestStaffID(t, db),
		}
		err := service.UpsertStudentArrivalSchedule(ctx, existingSched)
		require.NoError(t, err)

		schedules := []schedule.ArrivalScheduleInput{
			{Weekday: 1, ArrivalTime: "08:00"}, // Different time
		}

		result, err := service.BulkUpsertArrivalSchedules(ctx, schedule.ArrivalScheduleBulkFilter{SchoolClass: className}, schedules, createArrivalServiceTestStaffID(t, db))

		require.NoError(t, err)
		assert.Equal(t, 1, result.StudentsAffected)
		assert.Empty(t, result.OverwrittenStudents)

		stored, findErr := service.GetStudentArrivalSchedules(ctx, student.ID)
		require.NoError(t, findErr)
		require.Len(t, stored, 1)
		assert.Equal(t, "07:30", stored[0].ExpectedArrival.Format("15:04"))
	})

	t.Run("returns error for invalid arrival time format", func(t *testing.T) {
		className := fmt.Sprintf("BBT1-%d", time.Now().UnixNano())
		testpkg.CreateTestStudent(t, db, "BulkBadTime", "Student", className)

		schedules := []schedule.ArrivalScheduleInput{
			{Weekday: 1, ArrivalTime: "invalid"},
		}

		result, err := service.BulkUpsertArrivalSchedules(ctx, schedule.ArrivalScheduleBulkFilter{SchoolClass: className}, schedules, createArrivalServiceTestStaffID(t, db))

		require.Error(t, err)
		assert.Nil(t, result)
	})
}
