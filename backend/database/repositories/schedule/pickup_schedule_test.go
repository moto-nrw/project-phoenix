package schedule_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// strPtr returns a pointer to the given string
func strPtr(s string) *string {
	return &s
}

// =============================================================================
// StudentPickupScheduleRepository Tests
// =============================================================================

func TestStudentPickupScheduleRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	ctx := context.Background()

	t.Run("creates schedule successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		schedule := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayMonday,
			PickupTime: time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC),
			CreatedBy:  1,
		}

		err := repo.Create(ctx, schedule)

		require.NoError(t, err)
		assert.Greater(t, schedule.ID, int64(0))
	})

	t.Run("fails validation on nil schedule", func(t *testing.T) {
		err := repo.Create(ctx, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("fails validation on invalid schedule", func(t *testing.T) {
		schedule := &scheduleModels.StudentPickupSchedule{
			StudentID:  1,
			Weekday:    10,
			PickupTime: time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC),
			CreatedBy:  1,
		}

		err := repo.Create(ctx, schedule)

		require.Error(t, err)
	})
}

func TestStudentPickupScheduleRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	ctx := context.Background()

	t.Run("finds schedule by ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		schedule := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayTuesday,
			PickupTime: time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
			CreatedBy:  1,
		}
		err := repo.Create(ctx, schedule)
		require.NoError(t, err)

		result, err := repo.FindByID(ctx, schedule.ID)

		require.NoError(t, err)
		assert.Equal(t, schedule.ID, result.ID)
		assert.Equal(t, schedule.StudentID, result.StudentID)
		assert.Equal(t, schedule.Weekday, result.Weekday)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		result, err := repo.FindByID(ctx, int64(99999999))

		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestStudentPickupScheduleRepository_FindByStudentID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	ctx := context.Background()

	t.Run("finds all schedules for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		for _, weekday := range []int{scheduleModels.WeekdayMonday, scheduleModels.WeekdayWednesday, scheduleModels.WeekdayFriday} {
			schedule := &scheduleModels.StudentPickupSchedule{
				StudentID:  student.ID,
				Weekday:    weekday,
				PickupTime: time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC),
				CreatedBy:  1,
			}
			err := repo.Create(ctx, schedule)
			require.NoError(t, err)
		}

		results, err := repo.FindByStudentID(ctx, student.ID)

		require.NoError(t, err)
		assert.Len(t, results, 3)
		assert.Equal(t, scheduleModels.WeekdayMonday, results[0].Weekday)
		assert.Equal(t, scheduleModels.WeekdayWednesday, results[1].Weekday)
		assert.Equal(t, scheduleModels.WeekdayFriday, results[2].Weekday)
	})

	t.Run("returns empty slice when no schedules found", func(t *testing.T) {
		results, err := repo.FindByStudentID(ctx, int64(99999999))

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestStudentPickupScheduleRepository_FindByStudentIDAndWeekday(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	ctx := context.Background()

	t.Run("finds schedule for specific weekday", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		schedule := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayThursday,
			PickupTime: time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC),
			CreatedBy:  1,
		}
		err := repo.Create(ctx, schedule)
		require.NoError(t, err)

		result, err := repo.FindByStudentIDAndWeekday(ctx, student.ID, scheduleModels.WeekdayThursday)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, schedule.ID, result.ID)
		assert.Equal(t, scheduleModels.WeekdayThursday, result.Weekday)
	})

	t.Run("returns nil when not found", func(t *testing.T) {
		result, err := repo.FindByStudentIDAndWeekday(ctx, int64(99999999), scheduleModels.WeekdayMonday)

		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestStudentPickupScheduleRepository_FindByStudentIDsAndWeekday(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	ctx := context.Background()

	t.Run("finds schedules for multiple students", func(t *testing.T) {
		student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "1b")
		defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID)

		for _, studentID := range []int64{student1.ID, student2.ID} {
			schedule := &scheduleModels.StudentPickupSchedule{
				StudentID:  studentID,
				Weekday:    scheduleModels.WeekdayFriday,
				PickupTime: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
				CreatedBy:  1,
			}
			err := repo.Create(ctx, schedule)
			require.NoError(t, err)
		}

		results, err := repo.FindByStudentIDsAndWeekday(ctx, []int64{student1.ID, student2.ID}, scheduleModels.WeekdayFriday)

		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("returns empty slice for empty student IDs", func(t *testing.T) {
		results, err := repo.FindByStudentIDsAndWeekday(ctx, []int64{}, scheduleModels.WeekdayMonday)

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestStudentPickupScheduleRepository_UpsertSchedule(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	ctx := context.Background()

	t.Run("creates new schedule when doesn't exist", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		schedule := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayMonday,
			PickupTime: time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC),
			CreatedBy:  1,
		}

		err := repo.UpsertSchedule(ctx, schedule)

		require.NoError(t, err)
		assert.Greater(t, schedule.ID, int64(0))
	})

	t.Run("updates existing schedule", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		schedule := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayTuesday,
			PickupTime: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			CreatedBy:  1,
		}
		err := repo.UpsertSchedule(ctx, schedule)
		require.NoError(t, err)

		schedule.PickupTime = time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC)
		notes := "Updated notes"
		schedule.Notes = &notes

		err = repo.UpsertSchedule(ctx, schedule)

		require.NoError(t, err)

		result, err := repo.FindByStudentIDAndWeekday(ctx, student.ID, scheduleModels.WeekdayTuesday)
		require.NoError(t, err)
		assert.Equal(t, 15, result.PickupTime.Hour())
		assert.Equal(t, "Updated notes", *result.Notes)
	})

	t.Run("fails validation on nil schedule", func(t *testing.T) {
		err := repo.UpsertSchedule(ctx, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestStudentPickupScheduleRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	ctx := context.Background()

	t.Run("updates schedule successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		schedule := &scheduleModels.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    scheduleModels.WeekdayMonday,
			PickupTime: time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC),
			CreatedBy:  1,
		}
		err := repo.Create(ctx, schedule)
		require.NoError(t, err)

		schedule.PickupTime = time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC)
		notes := "Updated notes"
		schedule.Notes = &notes

		err = repo.Update(ctx, schedule)

		require.NoError(t, err)

		result, err := repo.FindByID(ctx, schedule.ID)
		require.NoError(t, err)
		assert.Equal(t, 16, result.PickupTime.Hour())
		assert.Equal(t, "Updated notes", *result.Notes)
	})

	t.Run("fails validation on nil schedule", func(t *testing.T) {
		err := repo.Update(ctx, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("fails validation on invalid schedule", func(t *testing.T) {
		schedule := &scheduleModels.StudentPickupSchedule{
			StudentID:  0, // Invalid
			Weekday:    scheduleModels.WeekdayMonday,
			PickupTime: time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC),
			CreatedBy:  1,
		}

		err := repo.Update(ctx, schedule)

		require.Error(t, err)
	})
}

func TestStudentPickupScheduleRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	ctx := context.Background()

	t.Run("lists all schedules", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		for _, weekday := range []int{scheduleModels.WeekdayMonday, scheduleModels.WeekdayTuesday} {
			schedule := &scheduleModels.StudentPickupSchedule{
				StudentID:  student.ID,
				Weekday:    weekday,
				PickupTime: time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC),
				CreatedBy:  1,
			}
			err := repo.Create(ctx, schedule)
			require.NoError(t, err)
		}

		results, err := repo.List(ctx, nil)

		require.NoError(t, err)
		// At least our 2 schedules should be present
		assert.GreaterOrEqual(t, len(results), 2)
	})

	t.Run("lists with nil options", func(t *testing.T) {
		results, err := repo.List(ctx, nil)

		require.NoError(t, err)
		assert.NotNil(t, results)
	})
}

func TestStudentPickupScheduleRepository_DeleteByStudentID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupScheduleRepository(db)
	ctx := context.Background()

	t.Run("deletes all schedules for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		for _, weekday := range []int{scheduleModels.WeekdayMonday, scheduleModels.WeekdayWednesday} {
			schedule := &scheduleModels.StudentPickupSchedule{
				StudentID:  student.ID,
				Weekday:    weekday,
				PickupTime: time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC),
				CreatedBy:  1,
			}
			err := repo.Create(ctx, schedule)
			require.NoError(t, err)
		}

		err := repo.DeleteByStudentID(ctx, student.ID)

		require.NoError(t, err)

		results, err := repo.FindByStudentID(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("succeeds when no schedules exist", func(t *testing.T) {
		err := repo.DeleteByStudentID(ctx, int64(99999999))

		require.NoError(t, err)
	})
}

// =============================================================================
// StudentPickupExceptionRepository Tests
// =============================================================================

func TestStudentPickupExceptionRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupExceptionRepository(db)
	ctx := context.Background()

	t.Run("creates exception successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		exception := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: time.Date(2024, 2, 14, 12, 0, 0, 0, timezone.Berlin),
			Reason:        strPtr("Doctor appointment"),
			CreatedBy:     1,
		}

		err := repo.Create(ctx, exception)

		require.NoError(t, err)
		assert.Greater(t, exception.ID, int64(0))
	})

	t.Run("fails validation on nil exception", func(t *testing.T) {
		err := repo.Create(ctx, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestStudentPickupExceptionRepository_FindByStudentID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupExceptionRepository(db)
	ctx := context.Background()

	t.Run("finds all exceptions for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		dates := []time.Time{
			time.Date(2024, 2, 14, 12, 0, 0, 0, timezone.Berlin),
			time.Date(2024, 2, 15, 12, 0, 0, 0, timezone.Berlin),
		}

		for _, date := range dates {
			exception := &scheduleModels.StudentPickupException{
				StudentID:     student.ID,
				ExceptionDate: date,
				Reason:        strPtr("Test reason"),
				CreatedBy:     1,
			}
			err := repo.Create(ctx, exception)
			require.NoError(t, err)
		}

		results, err := repo.FindByStudentID(ctx, student.ID)

		require.NoError(t, err)
		assert.Len(t, results, 2)
		assert.True(t, results[0].ExceptionDate.Before(results[1].ExceptionDate))
	})
}

func TestStudentPickupExceptionRepository_FindUpcomingByStudentID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupExceptionRepository(db)
	ctx := context.Background()

	t.Run("finds only upcoming exceptions", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		pastException := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: time.Now().AddDate(0, 0, -7),
			Reason:        strPtr("Past exception"),
			CreatedBy:     1,
		}
		err := repo.Create(ctx, pastException)
		require.NoError(t, err)

		futureException := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: time.Now().AddDate(0, 0, 7),
			Reason:        strPtr("Future exception"),
			CreatedBy:     1,
		}
		err = repo.Create(ctx, futureException)
		require.NoError(t, err)

		results, err := repo.FindUpcomingByStudentID(ctx, student.ID)

		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "Future exception", *results[0].Reason)
	})
}

func TestStudentPickupExceptionRepository_FindByStudentIDAndDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupExceptionRepository(db)
	ctx := context.Background()

	t.Run("finds exception for specific date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Use Berlin timezone for consistent date handling
		exceptionDate := time.Date(2024, 3, 20, 12, 0, 0, 0, timezone.Berlin)
		exception := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: exceptionDate,
			Reason:        strPtr("Specific date exception"),
			CreatedBy:     1,
		}
		err := repo.Create(ctx, exception)
		require.NoError(t, err)

		result, err := repo.FindByStudentIDAndDate(ctx, student.ID, exceptionDate)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, exception.ID, result.ID)
	})

	t.Run("returns nil when not found", func(t *testing.T) {
		result, err := repo.FindByStudentIDAndDate(ctx, int64(99999999), time.Now())

		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestStudentPickupExceptionRepository_FindByStudentIDsAndDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupExceptionRepository(db)
	ctx := context.Background()

	t.Run("finds exceptions for multiple students on same date", func(t *testing.T) {
		student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "1b")
		defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID)

		// Use Berlin timezone for consistent date handling
		exceptionDate := time.Date(2024, 4, 10, 12, 0, 0, 0, timezone.Berlin)

		for _, studentID := range []int64{student1.ID, student2.ID} {
			exception := &scheduleModels.StudentPickupException{
				StudentID:     studentID,
				ExceptionDate: exceptionDate,
				Reason:        strPtr("Group exception"),
				CreatedBy:     1,
			}
			err := repo.Create(ctx, exception)
			require.NoError(t, err)
		}

		results, err := repo.FindByStudentIDsAndDate(ctx, []int64{student1.ID, student2.ID}, exceptionDate)

		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("returns empty slice for empty student IDs", func(t *testing.T) {
		results, err := repo.FindByStudentIDsAndDate(ctx, []int64{}, time.Now())

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestStudentPickupExceptionRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupExceptionRepository(db)
	ctx := context.Background()

	t.Run("finds exception by ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		exception := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: time.Date(2024, 5, 20, 12, 0, 0, 0, timezone.Berlin),
			Reason:        strPtr("Test reason"),
			CreatedBy:     1,
		}
		err := repo.Create(ctx, exception)
		require.NoError(t, err)

		result, err := repo.FindByID(ctx, exception.ID)

		require.NoError(t, err)
		assert.Equal(t, exception.ID, result.ID)
		assert.Equal(t, "Test reason", *result.Reason)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		result, err := repo.FindByID(ctx, int64(99999999))

		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestStudentPickupExceptionRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupExceptionRepository(db)
	ctx := context.Background()

	t.Run("updates exception successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		pickupTime := time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC)
		exception := &scheduleModels.StudentPickupException{
			StudentID:     student.ID,
			ExceptionDate: time.Date(2024, 6, 15, 12, 0, 0, 0, timezone.Berlin),
			PickupTime:    &pickupTime,
			Reason:        strPtr("Original reason"),
			CreatedBy:     1,
		}
		err := repo.Create(ctx, exception)
		require.NoError(t, err)

		exception.Reason = strPtr("Updated reason")
		newPickupTime := time.Date(2024, 1, 1, 15, 30, 0, 0, time.UTC)
		exception.PickupTime = &newPickupTime

		err = repo.Update(ctx, exception)

		require.NoError(t, err)

		result, err := repo.FindByID(ctx, exception.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated reason", *result.Reason)
		assert.Equal(t, 15, result.PickupTime.Hour())
	})

	t.Run("fails validation on nil exception", func(t *testing.T) {
		err := repo.Update(ctx, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("fails validation on invalid exception", func(t *testing.T) {
		exception := &scheduleModels.StudentPickupException{
			StudentID:     0, // Invalid
			ExceptionDate: time.Date(2024, 6, 15, 12, 0, 0, 0, timezone.Berlin),
			Reason:        strPtr("Test"),
			CreatedBy:     1,
		}

		err := repo.Update(ctx, exception)

		require.Error(t, err)
	})
}

func TestStudentPickupExceptionRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupExceptionRepository(db)
	ctx := context.Background()

	t.Run("lists all exceptions", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		for i := 1; i <= 3; i++ {
			exception := &scheduleModels.StudentPickupException{
				StudentID:     student.ID,
				ExceptionDate: time.Now().AddDate(0, 0, i+100), // Far future to avoid conflicts
				Reason:        strPtr("Test exception"),
				CreatedBy:     1,
			}
			err := repo.Create(ctx, exception)
			require.NoError(t, err)
		}

		results, err := repo.List(ctx, nil)

		require.NoError(t, err)
		// At least our 3 exceptions should be present
		assert.GreaterOrEqual(t, len(results), 3)
	})

	t.Run("lists with nil options", func(t *testing.T) {
		results, err := repo.List(ctx, nil)

		require.NoError(t, err)
		assert.NotNil(t, results)
	})
}

func TestStudentPickupExceptionRepository_DeleteByStudentID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupExceptionRepository(db)
	ctx := context.Background()

	t.Run("deletes all exceptions for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		for i := 0; i < 3; i++ {
			exception := &scheduleModels.StudentPickupException{
				StudentID:     student.ID,
				ExceptionDate: time.Now().AddDate(0, 0, i),
				Reason:        strPtr("Exception"),
				CreatedBy:     1,
			}
			err := repo.Create(ctx, exception)
			require.NoError(t, err)
		}

		err := repo.DeleteByStudentID(ctx, student.ID)

		require.NoError(t, err)

		results, err := repo.FindByStudentID(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestStudentPickupExceptionRepository_DeletePastExceptions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupExceptionRepository(db)
	ctx := context.Background()

	t.Run("deletes only past exceptions", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Create past exceptions (will be deleted)
		pastExceptionCount := 0
		for i := -10; i < -5; i++ {
			exception := &scheduleModels.StudentPickupException{
				StudentID:     student.ID,
				ExceptionDate: time.Now().AddDate(0, 0, i),
				Reason:        strPtr("Past exception"),
				CreatedBy:     1,
			}
			err := repo.Create(ctx, exception)
			require.NoError(t, err)
			pastExceptionCount++
		}

		// Create future exceptions (should remain)
		futureExceptionCount := 0
		for i := 1; i <= 5; i++ {
			exception := &scheduleModels.StudentPickupException{
				StudentID:     student.ID,
				ExceptionDate: time.Now().AddDate(0, 0, i),
				Reason:        strPtr("Future exception"),
				CreatedBy:     1,
			}
			err := repo.Create(ctx, exception)
			require.NoError(t, err)
			futureExceptionCount++
		}

		cutoffDate := timezone.DateOfUTC(time.Now())
		rowsAffected, err := repo.DeletePastExceptions(ctx, cutoffDate)

		require.NoError(t, err)
		// At minimum, our past exceptions should be deleted (may be more from other tests)
		assert.GreaterOrEqual(t, rowsAffected, int64(pastExceptionCount))

		// Verify THIS student's future exceptions remain intact
		results, err := repo.FindByStudentID(ctx, student.ID)
		require.NoError(t, err)
		assert.Len(t, results, futureExceptionCount)
		for _, result := range results {
			assert.True(t, result.ExceptionDate.After(cutoffDate) || result.ExceptionDate.Equal(cutoffDate))
		}
	})
}

// =============================================================================
// StudentPickupNoteRepository Tests
// =============================================================================

func TestStudentPickupNoteRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupNoteRepository(db)
	ctx := context.Background()

	t.Run("creates note successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		note := &scheduleModels.StudentPickupNote{
			StudentID: student.ID,
			NoteDate:  time.Date(2024, 2, 14, 12, 0, 0, 0, timezone.Berlin),
			Content:   "Please call before pickup",
			CreatedBy: 1,
		}

		err := repo.Create(ctx, note)

		require.NoError(t, err)
		assert.Greater(t, note.ID, int64(0))
	})

	t.Run("fails validation on nil note", func(t *testing.T) {
		err := repo.Create(ctx, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("fails validation on invalid note", func(t *testing.T) {
		note := &scheduleModels.StudentPickupNote{
			StudentID: 0, // Invalid
			NoteDate:  time.Date(2024, 2, 14, 12, 0, 0, 0, timezone.Berlin),
			Content:   "Test",
			CreatedBy: 1,
		}

		err := repo.Create(ctx, note)

		require.Error(t, err)
	})
}

func TestStudentPickupNoteRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupNoteRepository(db)
	ctx := context.Background()

	t.Run("finds note by ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		note := &scheduleModels.StudentPickupNote{
			StudentID: student.ID,
			NoteDate:  time.Date(2024, 5, 20, 12, 0, 0, 0, timezone.Berlin),
			Content:   "Test note",
			CreatedBy: 1,
		}
		err := repo.Create(ctx, note)
		require.NoError(t, err)

		result, err := repo.FindByID(ctx, note.ID)

		require.NoError(t, err)
		assert.Equal(t, note.ID, result.ID)
		assert.Equal(t, "Test note", result.Content)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		result, err := repo.FindByID(ctx, int64(99999999))

		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestStudentPickupNoteRepository_FindByStudentID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupNoteRepository(db)
	ctx := context.Background()

	t.Run("finds all notes for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		dates := []time.Time{
			time.Date(2024, 2, 14, 12, 0, 0, 0, timezone.Berlin),
			time.Date(2024, 2, 15, 12, 0, 0, 0, timezone.Berlin),
			time.Date(2024, 2, 16, 12, 0, 0, 0, timezone.Berlin),
		}

		for _, date := range dates {
			note := &scheduleModels.StudentPickupNote{
				StudentID: student.ID,
				NoteDate:  date,
				Content:   "Test note",
				CreatedBy: 1,
			}
			err := repo.Create(ctx, note)
			require.NoError(t, err)
		}

		results, err := repo.FindByStudentID(ctx, student.ID)

		require.NoError(t, err)
		assert.Len(t, results, 3)
		// Results should be ordered by note_date ASC, then created_at ASC
		assert.True(t, results[0].NoteDate.Before(results[1].NoteDate) || results[0].NoteDate.Equal(results[1].NoteDate))
	})

	t.Run("returns empty slice when no notes found", func(t *testing.T) {
		results, err := repo.FindByStudentID(ctx, int64(99999999))

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestStudentPickupNoteRepository_FindByStudentIDAndDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupNoteRepository(db)
	ctx := context.Background()

	t.Run("finds notes for specific date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		targetDate := time.Date(2024, 3, 20, 12, 0, 0, 0, timezone.Berlin)

		// Create multiple notes for target date
		for i := 0; i < 2; i++ {
			note := &scheduleModels.StudentPickupNote{
				StudentID: student.ID,
				NoteDate:  targetDate,
				Content:   fmt.Sprintf("Note %d", i),
				CreatedBy: 1,
			}
			err := repo.Create(ctx, note)
			require.NoError(t, err)
		}

		// Create note for different date
		differentDate := targetDate.AddDate(0, 0, 1)
		note := &scheduleModels.StudentPickupNote{
			StudentID: student.ID,
			NoteDate:  differentDate,
			Content:   "Different date",
			CreatedBy: 1,
		}
		err := repo.Create(ctx, note)
		require.NoError(t, err)

		results, err := repo.FindByStudentIDAndDate(ctx, student.ID, targetDate)

		require.NoError(t, err)
		assert.Len(t, results, 2)
		for _, result := range results {
			assert.Equal(t, targetDate.Format("2006-01-02"), result.NoteDate.UTC().Format("2006-01-02"))
		}
	})

	t.Run("returns empty slice when no notes found", func(t *testing.T) {
		result, err := repo.FindByStudentIDAndDate(ctx, int64(99999999), time.Now())

		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestStudentPickupNoteRepository_FindByStudentIDsAndDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupNoteRepository(db)
	ctx := context.Background()

	t.Run("finds notes for multiple students on same date", func(t *testing.T) {
		student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "1b")
		defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID)

		noteDate := time.Date(2024, 4, 10, 12, 0, 0, 0, timezone.Berlin)

		for _, studentID := range []int64{student1.ID, student2.ID} {
			note := &scheduleModels.StudentPickupNote{
				StudentID: studentID,
				NoteDate:  noteDate,
				Content:   "Group note",
				CreatedBy: 1,
			}
			err := repo.Create(ctx, note)
			require.NoError(t, err)
		}

		results, err := repo.FindByStudentIDsAndDate(ctx, []int64{student1.ID, student2.ID}, noteDate)

		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("returns empty slice for empty student IDs", func(t *testing.T) {
		results, err := repo.FindByStudentIDsAndDate(ctx, []int64{}, time.Now())

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestStudentPickupNoteRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupNoteRepository(db)
	ctx := context.Background()

	t.Run("updates note successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		note := &scheduleModels.StudentPickupNote{
			StudentID: student.ID,
			NoteDate:  time.Date(2024, 6, 15, 12, 0, 0, 0, timezone.Berlin),
			Content:   "Original content",
			CreatedBy: 1,
		}
		err := repo.Create(ctx, note)
		require.NoError(t, err)

		note.Content = "Updated content"

		err = repo.Update(ctx, note)

		require.NoError(t, err)

		result, err := repo.FindByID(ctx, note.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated content", result.Content)
	})

	t.Run("fails validation on nil note", func(t *testing.T) {
		err := repo.Update(ctx, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("fails validation on invalid note", func(t *testing.T) {
		note := &scheduleModels.StudentPickupNote{
			StudentID: 0, // Invalid
			NoteDate:  time.Date(2024, 6, 15, 12, 0, 0, 0, timezone.Berlin),
			Content:   "Test",
			CreatedBy: 1,
		}

		err := repo.Update(ctx, note)

		require.Error(t, err)
	})
}

func TestStudentPickupNoteRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupNoteRepository(db)
	ctx := context.Background()

	t.Run("lists all notes", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		for i := 1; i <= 3; i++ {
			note := &scheduleModels.StudentPickupNote{
				StudentID: student.ID,
				NoteDate:  time.Now().AddDate(0, 0, i+200), // Far future to avoid conflicts
				Content:   "Test note",
				CreatedBy: 1,
			}
			err := repo.Create(ctx, note)
			require.NoError(t, err)
		}

		results, err := repo.List(ctx, nil)

		require.NoError(t, err)
		// At least our 3 notes should be present
		assert.GreaterOrEqual(t, len(results), 3)
	})

	t.Run("lists with nil options", func(t *testing.T) {
		results, err := repo.List(ctx, nil)

		require.NoError(t, err)
		assert.NotNil(t, results)
	})
}

func TestStudentPickupNoteRepository_DeleteByStudentID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupNoteRepository(db)
	ctx := context.Background()

	t.Run("deletes all notes for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		for i := 0; i < 3; i++ {
			note := &scheduleModels.StudentPickupNote{
				StudentID: student.ID,
				NoteDate:  time.Now().AddDate(0, 0, i),
				Content:   "Note",
				CreatedBy: 1,
			}
			err := repo.Create(ctx, note)
			require.NoError(t, err)
		}

		err := repo.DeleteByStudentID(ctx, student.ID)

		require.NoError(t, err)

		results, err := repo.FindByStudentID(ctx, student.ID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("succeeds when no notes exist", func(t *testing.T) {
		err := repo.DeleteByStudentID(ctx, int64(99999999))

		require.NoError(t, err)
	})
}

func TestStudentPickupNoteRepository_DeletePastNotes(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewStudentPickupNoteRepository(db)
	ctx := context.Background()

	t.Run("deletes only past notes", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Create past notes (will be deleted)
		pastNoteCount := 0
		for i := -10; i < -5; i++ {
			note := &scheduleModels.StudentPickupNote{
				StudentID: student.ID,
				NoteDate:  time.Now().AddDate(0, 0, i),
				Content:   "Past note",
				CreatedBy: 1,
			}
			err := repo.Create(ctx, note)
			require.NoError(t, err)
			pastNoteCount++
		}

		// Create future notes (should remain)
		futureNoteCount := 0
		for i := 1; i <= 5; i++ {
			note := &scheduleModels.StudentPickupNote{
				StudentID: student.ID,
				NoteDate:  time.Now().AddDate(0, 0, i),
				Content:   "Future note",
				CreatedBy: 1,
			}
			err := repo.Create(ctx, note)
			require.NoError(t, err)
			futureNoteCount++
		}

		cutoffDate := timezone.DateOfUTC(time.Now())
		rowsAffected, err := repo.DeletePastNotes(ctx, cutoffDate)

		require.NoError(t, err)
		// At minimum, our past notes should be deleted (may be more from other tests)
		assert.GreaterOrEqual(t, rowsAffected, int64(pastNoteCount))

		// Verify THIS student's future notes remain intact
		results, err := repo.FindByStudentID(ctx, student.ID)
		require.NoError(t, err)
		assert.Len(t, results, futureNoteCount)
		for _, result := range results {
			assert.True(t, result.NoteDate.After(cutoffDate) || result.NoteDate.Equal(cutoffDate))
		}
	})
}
