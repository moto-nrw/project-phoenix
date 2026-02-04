package schedule_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// strPtr returns a pointer to the given string
func strPtr(s string) *string {
	return &s
}

// setupPickupScheduleService creates a PickupScheduleService with real database connection
func setupPickupScheduleService(t *testing.T, db *bun.DB) schedule.PickupScheduleService {
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.PickupSchedule
}

// =============================================================================
// Schedule Operations Tests
// =============================================================================

func TestPickupScheduleService_GetStudentPickupSchedules(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := context.Background()

	t.Run("returns all schedules for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		for _, weekday := range []int{scheduleModels.WeekdayMonday, scheduleModels.WeekdayWednesday} {
			sched := &scheduleModels.StudentPickupSchedule{
				StudentID:  student.ID,
				Weekday:    weekday,
				PickupTime: time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC),
				CreatedBy:  1,
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
	ctx := context.Background()

	t.Run("returns schedule for specific weekday", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayTuesday,
			PickupTime: time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
			CreatedBy:  1,
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
	ctx := context.Background()

	t.Run("creates new schedule", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayFriday,
			PickupTime: time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
			CreatedBy:  1,
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
			CreatedBy:  1,
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
			CreatedBy:  1,
		}

		err := service.UpsertStudentPickupSchedule(ctx, sched)

		require.Error(t, err)
	})
}

func TestPickupScheduleService_UpsertBulkStudentPickupSchedules(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := context.Background()

	t.Run("creates multiple schedules in transaction", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		schedules := []*scheduleModels.StudentPickupSchedule{
			{
				Weekday:    scheduleModels.WeekdayMonday,
				PickupTime: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
				CreatedBy:  1,
			},
			{
				Weekday:    scheduleModels.WeekdayWednesday,
				PickupTime: time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
				CreatedBy:  1,
			},
			{
				Weekday:    scheduleModels.WeekdayFriday,
				PickupTime: time.Date(2024, 1, 1, 13, 30, 0, 0, time.UTC),
				CreatedBy:  1,
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
				CreatedBy:  1,
			},
			{
				Weekday:    10,
				PickupTime: time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
				CreatedBy:  1,
			},
		}

		err := service.UpsertBulkStudentPickupSchedules(ctx, student.ID, schedules)

		require.Error(t, err)

		results, err := service.GetStudentPickupSchedules(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestPickupScheduleService_DeleteStudentPickupSchedule(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := context.Background()

	t.Run("deletes schedule by ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayThursday,
			PickupTime: time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC),
			CreatedBy:  1,
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
	ctx := context.Background()

	t.Run("deletes all schedules for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		for _, weekday := range []int{scheduleModels.WeekdayMonday, scheduleModels.WeekdayWednesday, scheduleModels.WeekdayFriday} {
			sched := &scheduleModels.StudentPickupSchedule{
				StudentID:  student.ID,
				Weekday:    weekday,
				PickupTime: time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC),
				CreatedBy:  1,
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
	ctx := context.Background()

	t.Run("creates exception successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		exception := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: time.Date(2024, 3, 15, 12, 0, 0, 0, timezone.Berlin),
			Reason:        strPtr("Doctor appointment"),
			CreatedBy:     1,
		}

		err := service.CreateStudentPickupException(ctx, exception)

		require.NoError(t, err)
		assert.Greater(t, exception.ID, int64(0))
	})

	t.Run("fails when exception already exists for date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Use Berlin timezone for consistent date handling
		exceptionDate := time.Date(2024, 3, 20, 12, 0, 0, 0, timezone.Berlin)
		exception1 := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: exceptionDate,
			Reason:        strPtr("First exception"),
			CreatedBy:     1,
		}
		err := service.CreateStudentPickupException(ctx, exception1)
		require.NoError(t, err)

		exception2 := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: exceptionDate,
			Reason:        strPtr("Second exception"),
			CreatedBy:     1,
		}

		err = service.CreateStudentPickupException(ctx, exception2)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("fails validation for invalid exception", func(t *testing.T) {
		exception := &scheduleModels.StudentPickupException{
			StudentID:     0,
			ExceptionDate: time.Date(2024, 3, 15, 12, 0, 0, 0, timezone.Berlin),
			Reason:        strPtr("Test"),
			CreatedBy:     1,
		}

		err := service.CreateStudentPickupException(ctx, exception)

		require.Error(t, err)
	})
}

func TestPickupScheduleService_GetStudentPickupExceptions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := context.Background()

	t.Run("returns all exceptions for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Use consistent base date to avoid any timezone edge cases
		baseDate := timezone.Today()
		for i := -2; i <= 2; i++ {
			exception := &scheduleModels.StudentPickupException{
				StudentID:     student.ID,
				ExceptionDate: baseDate.AddDate(0, 0, i),
				Reason:        strPtr("Exception"),
				CreatedBy:     1,
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
	ctx := context.Background()

	t.Run("returns only upcoming exceptions", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Use consistent base date to avoid timezone edge cases
		baseDate := timezone.Today()

		for i := -5; i < 0; i++ {
			exception := &scheduleModels.StudentPickupException{
				StudentID:     student.ID,
				ExceptionDate: baseDate.AddDate(0, 0, i),
				Reason:        strPtr("Past"),
				CreatedBy:     1,
			}
			err := service.CreateStudentPickupException(ctx, exception)
			require.NoError(t, err)
		}

		for i := 1; i <= 3; i++ {
			exception := &scheduleModels.StudentPickupException{
				StudentID:     student.ID,
				ExceptionDate: baseDate.AddDate(0, 0, i),
				Reason:        strPtr("Future"),
				CreatedBy:     1,
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
	ctx := context.Background()

	t.Run("updates exception successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		exception := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC),
			Reason:        strPtr("Original reason"),
			CreatedBy:     1,
		}
		err := service.CreateStudentPickupException(ctx, exception)
		require.NoError(t, err)

		exception.Reason = strPtr("Updated reason")

		err = service.UpdateStudentPickupException(ctx, exception)

		require.NoError(t, err)

		exceptions, err := service.GetStudentPickupExceptions(ctx, student.ID)
		require.NoError(t, err)
		assert.Len(t, exceptions, 1)
		assert.Equal(t, "Updated reason", *exceptions[0].Reason)
	})
}

func TestPickupScheduleService_DeleteStudentPickupException(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := context.Background()

	t.Run("deletes exception by ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		exception := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC),
			Reason:        strPtr("Test"),
			CreatedBy:     1,
		}
		err := service.CreateStudentPickupException(ctx, exception)
		require.NoError(t, err)

		err = service.DeleteStudentPickupException(ctx, exception.ID)

		require.NoError(t, err)

		results, err := service.GetStudentPickupExceptions(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestPickupScheduleService_DeleteAllStudentPickupExceptions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := context.Background()

	t.Run("deletes all exceptions for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Use consistent base date to avoid timezone edge cases
		baseDate := timezone.Today()
		for i := 1; i <= 5; i++ {
			exception := &scheduleModels.StudentPickupException{
				StudentID:     student.ID,
				ExceptionDate: baseDate.AddDate(0, 0, i),
				Reason:        strPtr("Exception"),
				CreatedBy:     1,
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
}

// =============================================================================
// Computed Operations Tests
// =============================================================================

func TestPickupScheduleService_GetStudentPickupData(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := context.Background()

	t.Run("returns combined schedule and exception data", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayMonday,
			PickupTime: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			CreatedBy:  1,
		}
		err := service.UpsertStudentPickupSchedule(ctx, sched)
		require.NoError(t, err)

		exception := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: timezone.Today().AddDate(0, 0, 5),
			Reason:        strPtr("Future exception"),
			CreatedBy:     1,
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
	ctx := context.Background()

	t.Run("returns exception when present", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayMonday,
			PickupTime: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			CreatedBy:  1,
		}
		err := service.UpsertStudentPickupSchedule(ctx, sched)
		require.NoError(t, err)

		// Use a fixed Monday date at noon to avoid timezone boundary issues
		// January 8, 2024 is a Monday, and noon UTC is still Monday in Berlin
		testDate := time.Date(2024, 1, 8, 12, 0, 0, 0, time.UTC)

		earlyTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		exception := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: testDate,
			PickupTime:    &earlyTime,
			Reason:        strPtr("Early pickup"),
			CreatedBy:     1,
		}
		err = service.CreateStudentPickupException(ctx, exception)
		require.NoError(t, err)

		result, err := service.GetEffectivePickupTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.True(t, result.IsException)
		assert.NotNil(t, result.PickupTime)
		assert.Equal(t, 12, result.PickupTime.Hour())
	})

	t.Run("returns schedule when no exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayTuesday,
			PickupTime: time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC),
			CreatedBy:  1,
		}
		err := service.UpsertStudentPickupSchedule(ctx, sched)
		require.NoError(t, err)

		// Use a fixed Tuesday date at noon to avoid timezone boundary issues
		// January 9, 2024 is a Tuesday, and noon UTC is still Tuesday in Berlin
		testDate := time.Date(2024, 1, 9, 12, 0, 0, 0, time.UTC)

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
		testDate := time.Date(2024, 1, 13, 12, 0, 0, 0, time.UTC)

		result, err := service.GetEffectivePickupTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.Nil(t, result.PickupTime)
	})

	t.Run("returns nil when no schedule and no exception", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Use a fixed Wednesday date at noon to avoid timezone boundary issues
		// January 10, 2024 is a Wednesday
		testDate := time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)

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
			CreatedBy:  1,
		}
		err := service.UpsertStudentPickupSchedule(ctx, sched)
		require.NoError(t, err)

		// Use a fixed Friday date at noon to avoid timezone boundary issues
		// January 12, 2024 is a Friday
		testDate := time.Date(2024, 1, 12, 12, 0, 0, 0, time.UTC)

		result, err := service.GetEffectivePickupTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.False(t, result.IsException)
		assert.NotNil(t, result.PickupTime)
		assert.Equal(t, "Pick up with grandma", result.Notes)
	})

	t.Run("handles Sunday correctly", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Use a fixed Sunday date at noon to avoid timezone boundary issues
		// January 14, 2024 is a Sunday
		testDate := time.Date(2024, 1, 14, 12, 0, 0, 0, time.UTC)

		result, err := service.GetEffectivePickupTimeForDate(ctx, student.ID, testDate)

		require.NoError(t, err)
		assert.Nil(t, result.PickupTime)
	})
}

func TestPickupScheduleService_GetBulkEffectivePickupTimesForDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := context.Background()

	t.Run("returns effective times for multiple students", func(t *testing.T) {
		student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "1b")
		student3 := testpkg.CreateTestStudent(t, db, "Student", "Three", "1c")
		defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID, student3.ID)

		// Use a fixed Thursday date at noon to avoid timezone boundary issues
		// January 11, 2024 is a Thursday
		testDate := time.Date(2024, 1, 11, 12, 0, 0, 0, time.UTC)

		sched1 := &scheduleModels.StudentPickupSchedule{
			StudentID:  student1.ID,
			Weekday:    scheduleModels.WeekdayThursday,
			PickupTime: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			CreatedBy:  1,
		}
		err := service.UpsertStudentPickupSchedule(ctx, sched1)
		require.NoError(t, err)

		earlyTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		exception2 := &scheduleModels.StudentPickupException{
			StudentID:     student2.ID,
			ExceptionDate: testDate,
			PickupTime:    &earlyTime,
			Reason:        strPtr("Doctor appointment"),
			CreatedBy:     1,
		}
		err = service.CreateStudentPickupException(ctx, exception2)
		require.NoError(t, err)

		exception3 := &scheduleModels.StudentPickupException{
			StudentID:     student3.ID,
			ExceptionDate: testDate,
			PickupTime:    nil,
			Reason:        strPtr("Sick"),
			CreatedBy:     1,
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

		assert.True(t, results[student3.ID].IsException)
		assert.Nil(t, results[student3.ID].PickupTime)
	})

	t.Run("returns empty map for empty student IDs", func(t *testing.T) {
		results, err := service.GetBulkEffectivePickupTimesForDate(ctx, []int64{}, time.Now())

		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("handles weekend correctly for all students", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Use a fixed Sunday date at noon to avoid timezone boundary issues
		// January 14, 2024 is a Sunday
		testDate := time.Date(2024, 1, 14, 12, 0, 0, 0, time.UTC)

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
		testDate := time.Date(2024, 1, 8, 12, 0, 0, 0, time.UTC)

		notes := "Picked up by aunt"
		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayMonday,
			PickupTime: time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
			Notes:      &notes,
			CreatedBy:  1,
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
}

// =============================================================================
// Note Operations Tests
// =============================================================================

func TestPickupScheduleService_CreateStudentPickupNote(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := context.Background()

	t.Run("creates note successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		note := &scheduleModels.StudentPickupNote{
			StudentID: student.ID,
			NoteDate:  time.Date(2024, 3, 15, 12, 0, 0, 0, timezone.Berlin),
			Content:   "Please call before pickup",
			CreatedBy: 1,
		}

		err := service.CreateStudentPickupNote(ctx, note)

		require.NoError(t, err)
		assert.Greater(t, note.ID, int64(0))
	})

	t.Run("fails validation for invalid note", func(t *testing.T) {
		note := &scheduleModels.StudentPickupNote{
			StudentID: 0, // Invalid
			NoteDate:  time.Date(2024, 3, 15, 12, 0, 0, 0, timezone.Berlin),
			Content:   "Test",
			CreatedBy: 1,
		}

		err := service.CreateStudentPickupNote(ctx, note)

		require.Error(t, err)
	})

	t.Run("fails validation for empty content", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		note := &scheduleModels.StudentPickupNote{
			StudentID: student.ID,
			NoteDate:  time.Date(2024, 3, 15, 12, 0, 0, 0, timezone.Berlin),
			Content:   "",
			CreatedBy: 1,
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
	ctx := context.Background()

	t.Run("returns note by ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		note := &scheduleModels.StudentPickupNote{
			StudentID: student.ID,
			NoteDate:  time.Date(2024, 3, 16, 12, 0, 0, 0, timezone.Berlin),
			Content:   "Test note",
			CreatedBy: 1,
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
	ctx := context.Background()

	t.Run("returns all notes for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		baseDate := timezone.Today()
		for i := 0; i < 3; i++ {
			note := &scheduleModels.StudentPickupNote{
				StudentID: student.ID,
				NoteDate:  baseDate.AddDate(0, 0, i),
				Content:   "Note content",
				CreatedBy: 1,
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
	ctx := context.Background()

	t.Run("returns notes for specific date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		targetDate := time.Date(2024, 3, 20, 12, 0, 0, 0, timezone.Berlin)

		// Create notes for target date
		for i := 0; i < 2; i++ {
			note := &scheduleModels.StudentPickupNote{
				StudentID: student.ID,
				NoteDate:  targetDate,
				Content:   fmt.Sprintf("Note %d", i),
				CreatedBy: 1,
			}
			err := service.CreateStudentPickupNote(ctx, note)
			require.NoError(t, err)
		}

		// Create note for different date
		differentDate := targetDate.AddDate(0, 0, 1)
		note := &scheduleModels.StudentPickupNote{
			StudentID: student.ID,
			NoteDate:  differentDate,
			Content:   "Different date note",
			CreatedBy: 1,
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
	ctx := context.Background()

	t.Run("updates note successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		note := &scheduleModels.StudentPickupNote{
			StudentID: student.ID,
			NoteDate:  time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC),
			Content:   "Original content",
			CreatedBy: 1,
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
			NoteDate:  time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC),
			Content:   "Test",
			CreatedBy: 1,
		}

		err := service.UpdateStudentPickupNote(ctx, note)

		require.Error(t, err)
	})
}

func TestPickupScheduleService_DeleteStudentPickupNote(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupPickupScheduleService(t, db)
	ctx := context.Background()

	t.Run("deletes note by ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		note := &scheduleModels.StudentPickupNote{
			StudentID: student.ID,
			NoteDate:  time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC),
			Content:   "Test",
			CreatedBy: 1,
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
	ctx := context.Background()

	t.Run("deletes all notes for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		baseDate := timezone.Today()
		for i := 1; i <= 5; i++ {
			note := &scheduleModels.StudentPickupNote{
				StudentID: student.ID,
				NoteDate:  baseDate.AddDate(0, 0, i),
				Content:   "Note",
				CreatedBy: 1,
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
