package schedule_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func newArrivalScheduleRepository(db *bun.DB) scheduleModels.StudentArrivalScheduleRepository {
	return repositories.NewFactory(db).StudentArrivalSchedule
}

func newArrivalExceptionRepository(db *bun.DB) scheduleModels.StudentArrivalExceptionRepository {
	return repositories.NewFactory(db).StudentArrivalException
}

func newArrivalNoteRepository(db *bun.DB) scheduleModels.StudentArrivalNoteRepository {
	return repositories.NewFactory(db).StudentArrivalNote
}

// =============================================================================
// StudentArrivalScheduleRepository Tests
// =============================================================================

func TestStudentArrivalScheduleRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalScheduleRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("creates schedule successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		schedule := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         scheduleModels.WeekdayMonday,
			ExpectedArrival: time.Date(2024, 1, 1, 7, 50, 0, 0, time.UTC),
			CreatedBy:       createRepositoryTestStaffID(t, db),
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
		schedule := &scheduleModels.StudentArrivalSchedule{
			StudentID:       1,
			Weekday:         10,
			ExpectedArrival: time.Date(2024, 1, 1, 7, 50, 0, 0, time.UTC),
			CreatedBy:       createRepositoryTestStaffID(t, db),
		}

		err := repo.Create(ctx, schedule)

		require.Error(t, err)
	})
}

func TestStudentArrivalScheduleRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalScheduleRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("finds schedule by ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		schedule := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         scheduleModels.WeekdayTuesday,
			ExpectedArrival: time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC),
			CreatedBy:       createRepositoryTestStaffID(t, db),
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

func TestStudentArrivalScheduleRepository_FindByStudentID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalScheduleRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("finds all schedules for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		for _, weekday := range []int{scheduleModels.WeekdayMonday, scheduleModels.WeekdayWednesday, scheduleModels.WeekdayFriday} {
			schedule := &scheduleModels.StudentArrivalSchedule{
				StudentID:       student.ID,
				Weekday:         weekday,
				ExpectedArrival: time.Date(2024, 1, 1, 7, 50, 0, 0, time.UTC),
				CreatedBy:       createRepositoryTestStaffID(t, db),
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

func TestStudentArrivalScheduleRepository_FindByStudentIDAndWeekday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalScheduleRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("finds schedule for specific weekday", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		schedule := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         scheduleModels.WeekdayThursday,
			ExpectedArrival: time.Date(2024, 1, 1, 8, 15, 0, 0, time.UTC),
			CreatedBy:       createRepositoryTestStaffID(t, db),
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

func TestStudentArrivalScheduleRepository_FindByStudentIDsAndWeekday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalScheduleRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("finds schedules for multiple students", func(t *testing.T) {
		student1 := testpkg.CreateTestStudent(t, db, "ArrStudent", "One", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "ArrStudent", "Two", "1b")

		for _, studentID := range []int64{student1.ID, student2.ID} {
			schedule := &scheduleModels.StudentArrivalSchedule{
				StudentID:       studentID,
				Weekday:         scheduleModels.WeekdayFriday,
				ExpectedArrival: time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC),
				CreatedBy:       createRepositoryTestStaffID(t, db),
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

func TestStudentArrivalScheduleRepository_UpsertSchedule(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalScheduleRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("creates new schedule when doesn't exist", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		schedule := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         scheduleModels.WeekdayMonday,
			ExpectedArrival: time.Date(2024, 1, 1, 7, 50, 0, 0, time.UTC),
			CreatedBy:       createRepositoryTestStaffID(t, db),
		}

		err := repo.UpsertSchedule(ctx, schedule)

		require.NoError(t, err)
		assert.Greater(t, schedule.ID, int64(0))
	})

	t.Run("updates existing schedule", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		schedule := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         scheduleModels.WeekdayTuesday,
			ExpectedArrival: time.Date(2024, 1, 1, 7, 30, 0, 0, time.UTC),
			CreatedBy:       createRepositoryTestStaffID(t, db),
		}
		err := repo.UpsertSchedule(ctx, schedule)
		require.NoError(t, err)

		schedule.ExpectedArrival = time.Date(2024, 1, 1, 8, 15, 0, 0, time.UTC)
		notes := "Updated notes"
		schedule.Notes = &notes

		err = repo.UpsertSchedule(ctx, schedule)

		require.NoError(t, err)

		result, err := repo.FindByStudentIDAndWeekday(ctx, student.ID, scheduleModels.WeekdayTuesday)
		require.NoError(t, err)
		assert.Equal(t, 8, result.ExpectedArrival.Hour())
		assert.Equal(t, "Updated notes", *result.Notes)
	})

	t.Run("fails validation on nil schedule", func(t *testing.T) {
		err := repo.UpsertSchedule(ctx, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestStudentArrivalScheduleRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalScheduleRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("updates schedule successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		schedule := &scheduleModels.StudentArrivalSchedule{
			StudentID:       student.ID,
			Weekday:         scheduleModels.WeekdayMonday,
			ExpectedArrival: time.Date(2024, 1, 1, 7, 50, 0, 0, time.UTC),
			CreatedBy:       createRepositoryTestStaffID(t, db),
		}
		err := repo.Create(ctx, schedule)
		require.NoError(t, err)

		schedule.ExpectedArrival = time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
		notes := "Updated notes"
		schedule.Notes = &notes

		err = repo.Update(ctx, schedule)

		require.NoError(t, err)

		result, err := repo.FindByID(ctx, schedule.ID)
		require.NoError(t, err)
		assert.Equal(t, 9, result.ExpectedArrival.Hour())
		assert.Equal(t, "Updated notes", *result.Notes)
	})

	t.Run("fails validation on nil schedule", func(t *testing.T) {
		err := repo.Update(ctx, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("fails validation on invalid schedule", func(t *testing.T) {
		schedule := &scheduleModels.StudentArrivalSchedule{
			StudentID:       0, // Invalid
			Weekday:         scheduleModels.WeekdayMonday,
			ExpectedArrival: time.Date(2024, 1, 1, 7, 50, 0, 0, time.UTC),
			CreatedBy:       createRepositoryTestStaffID(t, db),
		}

		err := repo.Update(ctx, schedule)

		require.Error(t, err)
	})
}

func TestStudentArrivalScheduleRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalScheduleRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("lists all schedules", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		for _, weekday := range []int{scheduleModels.WeekdayMonday, scheduleModels.WeekdayTuesday} {
			schedule := &scheduleModels.StudentArrivalSchedule{
				StudentID:       student.ID,
				Weekday:         weekday,
				ExpectedArrival: time.Date(2024, 1, 1, 7, 50, 0, 0, time.UTC),
				CreatedBy:       createRepositoryTestStaffID(t, db),
			}
			err := repo.Create(ctx, schedule)
			require.NoError(t, err)
		}

		results, err := repo.List(ctx, nil)

		require.NoError(t, err)
		// At least our 2 schedules should be present
		assert.GreaterOrEqual(t, len(results), 2)

		options := modelBase.NewQueryOptions().WithPagination(1, 1)
		options.Filter.Equal("student_id", student.ID)
		options.Sorting = (&modelBase.Sorting{}).AddField("weekday", modelBase.SortDesc)
		results, err = repo.List(ctx, options)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, scheduleModels.WeekdayTuesday, results[0].Weekday)
	})

	t.Run("lists with nil options", func(t *testing.T) {
		results, err := repo.List(ctx, nil)

		require.NoError(t, err)
		assert.NotNil(t, results)
	})
}

func TestStudentArrivalScheduleRepository_DeleteByStudentID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalScheduleRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes all schedules for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		for _, weekday := range []int{scheduleModels.WeekdayMonday, scheduleModels.WeekdayWednesday} {
			schedule := &scheduleModels.StudentArrivalSchedule{
				StudentID:       student.ID,
				Weekday:         weekday,
				ExpectedArrival: time.Date(2024, 1, 1, 7, 50, 0, 0, time.UTC),
				CreatedBy:       createRepositoryTestStaffID(t, db),
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
// StudentArrivalExceptionRepository Tests
// =============================================================================

func TestStudentArrivalExceptionRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalExceptionRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("creates exception successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		exception := &scheduleModels.StudentArrivalException{
			StudentID:     student.ID,
			ExceptionDate: timezone.NewDate(2024, 2, 14),
			Reason:        testpkg.StrPtr("Doctor appointment"),
			CreatedBy:     createRepositoryTestStaffID(t, db),
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

func TestStudentArrivalExceptionRepository_FindByStudentID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalExceptionRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("finds all exceptions for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		dates := []timezone.Date{
			timezone.NewDate(2024, 2, 14),
			timezone.NewDate(2024, 2, 15),
		}

		for _, date := range dates {
			exception := &scheduleModels.StudentArrivalException{
				StudentID:     student.ID,
				ExceptionDate: date,
				Reason:        testpkg.StrPtr("Test reason"),
				CreatedBy:     createRepositoryTestStaffID(t, db),
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

func TestStudentArrivalExceptionRepository_FindUpcomingByStudentID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalExceptionRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("finds only upcoming exceptions", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		pastException := &scheduleModels.StudentArrivalException{
			StudentID:     student.ID,
			ExceptionDate: timezone.TodayDate().AddDays(-7),
			Reason:        testpkg.StrPtr("Past exception"),
			CreatedBy:     createRepositoryTestStaffID(t, db),
		}
		err := repo.Create(ctx, pastException)
		require.NoError(t, err)

		futureException := &scheduleModels.StudentArrivalException{
			StudentID:     student.ID,
			ExceptionDate: timezone.TodayDate().AddDays(7),
			Reason:        testpkg.StrPtr("Future exception"),
			CreatedBy:     createRepositoryTestStaffID(t, db),
		}
		err = repo.Create(ctx, futureException)
		require.NoError(t, err)

		results, err := repo.FindUpcomingByStudentID(ctx, student.ID)

		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "Future exception", *results[0].Reason)
	})
}

func TestStudentArrivalExceptionRepository_FindByStudentIDAndDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalExceptionRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("finds exception for specific date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		exceptionDate := timezone.NewDate(2024, 3, 20)
		exception := &scheduleModels.StudentArrivalException{
			StudentID:     student.ID,
			ExceptionDate: exceptionDate,
			Reason:        testpkg.StrPtr("Specific date exception"),
			CreatedBy:     createRepositoryTestStaffID(t, db),
		}
		err := repo.Create(ctx, exception)
		require.NoError(t, err)

		result, err := repo.FindByStudentIDAndDate(ctx, student.ID, exceptionDate)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, exception.ID, result.ID)
	})

	t.Run("returns nil when not found", func(t *testing.T) {
		result, err := repo.FindByStudentIDAndDate(ctx, int64(99999999), timezone.TodayDate())

		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestStudentArrivalExceptionRepository_FindByStudentIDsAndDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalExceptionRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("finds exceptions for multiple students on same date", func(t *testing.T) {
		student1 := testpkg.CreateTestStudent(t, db, "ArrStudent", "One", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "ArrStudent", "Two", "1b")
		staff := testpkg.CreateTestStaff(t, db, "ArrStaff", "BulkLookup")

		exceptionDate := timezone.NewDate(2024, 4, 10)

		for _, studentID := range []int64{student1.ID, student2.ID} {
			exception := &scheduleModels.StudentArrivalException{
				StudentID:     studentID,
				ExceptionDate: exceptionDate,
				Reason:        testpkg.StrPtr("Group exception"),
				CreatedBy:     staff.ID,
			}
			err := repo.Create(ctx, exception)
			require.NoError(t, err)
		}

		results, err := repo.FindByStudentIDsAndDate(ctx, []int64{student1.ID, student2.ID}, exceptionDate)

		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("returns empty slice for empty student IDs", func(t *testing.T) {
		results, err := repo.FindByStudentIDsAndDate(ctx, []int64{}, timezone.TodayDate())

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestStudentArrivalExceptionRepository_FindByStudentIDsAndDate_MatchesDateInBerlinSession(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalExceptionRepository(db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "ArrStudent", "BerlinTZ", "1a")
	staff := testpkg.CreateTestStaff(t, db, "ArrStaff", "BerlinTZ")

	// timezone.Date binds as a 'YYYY-MM-DD' literal, so the DB session
	// timezone can no longer shift the stored or queried day. The SET LOCAL
	// stays to pin exactly that: the roundtrip is session-TZ-independent.
	err := db.RunInTx(ctx, nil, func(_ context.Context, tx bun.Tx) error {
		txCtx := tenant.WithTransactionForTest(ctx, &tx)
		_, err := tx.NewRaw(`SET LOCAL timezone = 'Europe/Berlin'`).Exec(txCtx)
		if err != nil {
			return err
		}

		day := timezone.NewDate(2026, 4, 24)
		exception := &scheduleModels.StudentArrivalException{
			StudentID:     student.ID,
			ExceptionDate: day,
			Reason:        testpkg.StrPtr("Berlin session regression"),
			CreatedBy:     staff.ID,
		}
		if err := repo.Create(txCtx, exception); err != nil {
			return err
		}

		results, err := repo.FindByStudentIDsAndDate(txCtx, []int64{student.ID}, day)
		if err != nil {
			return err
		}
		if len(results) != 1 {
			return fmt.Errorf("expected one exception for query date, got %d", len(results))
		}
		if results[0].ID != exception.ID {
			return fmt.Errorf("expected exception ID %d, got %d", exception.ID, results[0].ID)
		}
		if results[0].ExceptionDate != day {
			return fmt.Errorf("expected exception date %s to roundtrip, got %s", day, results[0].ExceptionDate)
		}

		results, err = repo.FindByStudentIDsAndDate(txCtx, []int64{student.ID}, day.AddDays(-1))
		if err != nil {
			return err
		}
		if len(results) != 0 {
			return fmt.Errorf("expected no exception for previous day, got %d", len(results))
		}

		return nil
	})
	require.NoError(t, err)
}

func TestStudentArrivalExceptionRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalExceptionRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("finds exception by ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		exception := &scheduleModels.StudentArrivalException{
			StudentID:     student.ID,
			ExceptionDate: timezone.NewDate(2024, 5, 20),
			Reason:        testpkg.StrPtr("Test reason"),
			CreatedBy:     createRepositoryTestStaffID(t, db),
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

func TestStudentArrivalExceptionRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalExceptionRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("updates exception successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		arrivalTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		exception := &scheduleModels.StudentArrivalException{
			StudentID:       student.ID,
			ExceptionDate:   timezone.NewDate(2024, 6, 15),
			ExpectedArrival: &arrivalTime,
			Reason:          testpkg.StrPtr("Original reason"),
			CreatedBy:       createRepositoryTestStaffID(t, db),
		}
		err := repo.Create(ctx, exception)
		require.NoError(t, err)

		exception.Reason = testpkg.StrPtr("Updated reason")
		newArrivalTime := time.Date(2024, 1, 1, 9, 30, 0, 0, time.UTC)
		exception.ExpectedArrival = &newArrivalTime

		err = repo.Update(ctx, exception)

		require.NoError(t, err)

		result, err := repo.FindByID(ctx, exception.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated reason", *result.Reason)
		assert.Equal(t, 9, result.ExpectedArrival.Hour())
	})

	t.Run("fails validation on nil exception", func(t *testing.T) {
		err := repo.Update(ctx, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("fails validation on invalid exception", func(t *testing.T) {
		exception := &scheduleModels.StudentArrivalException{
			StudentID:     0, // Invalid
			ExceptionDate: timezone.NewDate(2024, 6, 15),
			Reason:        testpkg.StrPtr("Test"),
			CreatedBy:     createRepositoryTestStaffID(t, db),
		}

		err := repo.Update(ctx, exception)

		require.Error(t, err)
	})
}

func TestStudentArrivalExceptionRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalExceptionRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("lists all exceptions", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		for i := 1; i <= 3; i++ {
			exception := &scheduleModels.StudentArrivalException{
				StudentID:     student.ID,
				ExceptionDate: timezone.TodayDate().AddDays(i + 100), // Far future to avoid conflicts
				Reason:        testpkg.StrPtr("Test exception"),
				CreatedBy:     createRepositoryTestStaffID(t, db),
			}
			err := repo.Create(ctx, exception)
			require.NoError(t, err)
		}

		results, err := repo.List(ctx, nil)

		require.NoError(t, err)
		// At least our 3 exceptions should be present
		assert.GreaterOrEqual(t, len(results), 3)

		options := modelBase.NewQueryOptions().WithPagination(1, 1)
		options.Filter.Equal("student_id", student.ID)
		options.Sorting = (&modelBase.Sorting{}).AddField("exception_date", modelBase.SortDesc)
		results, err = repo.List(ctx, options)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, timezone.TodayDate().AddDays(103), results[0].ExceptionDate)
	})

	t.Run("lists with nil options", func(t *testing.T) {
		results, err := repo.List(ctx, nil)

		require.NoError(t, err)
		assert.NotNil(t, results)
	})
}

func TestStudentArrivalExceptionRepository_DeleteByStudentID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalExceptionRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes all exceptions for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		for i := 0; i < 3; i++ {
			exception := &scheduleModels.StudentArrivalException{
				StudentID:     student.ID,
				ExceptionDate: timezone.TodayDate().AddDays(i),
				Reason:        testpkg.StrPtr("Exception"),
				CreatedBy:     createRepositoryTestStaffID(t, db),
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

func TestStudentArrivalExceptionRepository_DeletePastExceptions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalExceptionRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes only past exceptions", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		// Create past exceptions (will be deleted)
		pastExceptionCount := 0
		for i := -10; i < -5; i++ {
			exception := &scheduleModels.StudentArrivalException{
				StudentID:     student.ID,
				ExceptionDate: timezone.NewDate(2026, 8, 24).AddDays(i),
				Reason:        testpkg.StrPtr("Past exception"),
				CreatedBy:     createRepositoryTestStaffID(t, db),
			}
			err := repo.Create(ctx, exception)
			require.NoError(t, err)
			pastExceptionCount++
		}

		// Create future exceptions (should remain)
		futureExceptionCount := 0
		for i := 1; i <= 5; i++ {
			exception := &scheduleModels.StudentArrivalException{
				StudentID:     student.ID,
				ExceptionDate: timezone.NewDate(2026, 8, 24).AddDays(i),
				Reason:        testpkg.StrPtr("Future exception"),
				CreatedBy:     createRepositoryTestStaffID(t, db),
			}
			err := repo.Create(ctx, exception)
			require.NoError(t, err)
			futureExceptionCount++
		}

		cutoffDate := timezone.NewDate(2026, 8, 24)
		rowsAffected, err := repo.DeletePastExceptions(ctx, cutoffDate)

		require.NoError(t, err)
		// At minimum, our past exceptions should be deleted (may be more from other tests)
		assert.GreaterOrEqual(t, rowsAffected, int64(pastExceptionCount))

		// Verify THIS student's future exceptions remain intact
		results, err := repo.FindByStudentID(ctx, student.ID)
		require.NoError(t, err)
		assert.Len(t, results, futureExceptionCount)
		for _, result := range results {
			assert.True(t, !result.ExceptionDate.Before(cutoffDate))
		}
	})
}

// =============================================================================
// StudentArrivalNoteRepository Tests
// =============================================================================

func TestStudentArrivalNoteRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalNoteRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("creates note successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		note := &scheduleModels.StudentArrivalNote{
			StudentID: student.ID,
			NoteDate:  timezone.NewDate(2024, 2, 14),
			Content:   "Arrives by school bus",
			CreatedBy: createRepositoryTestStaffID(t, db),
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
		note := &scheduleModels.StudentArrivalNote{
			StudentID: 0, // Invalid
			NoteDate:  timezone.NewDate(2024, 2, 14),
			Content:   "Test",
			CreatedBy: createRepositoryTestStaffID(t, db),
		}

		err := repo.Create(ctx, note)

		require.Error(t, err)
	})
}

func TestStudentArrivalNoteRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalNoteRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("finds note by ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		note := &scheduleModels.StudentArrivalNote{
			StudentID: student.ID,
			NoteDate:  timezone.NewDate(2024, 5, 20),
			Content:   "Test note",
			CreatedBy: createRepositoryTestStaffID(t, db),
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

func TestStudentArrivalNoteRepository_FindByStudentID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalNoteRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("finds all notes for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		dates := []timezone.Date{
			timezone.NewDate(2024, 2, 14),
			timezone.NewDate(2024, 2, 15),
			timezone.NewDate(2024, 2, 16),
		}

		for _, date := range dates {
			note := &scheduleModels.StudentArrivalNote{
				StudentID: student.ID,
				NoteDate:  date,
				Content:   "Test note",
				CreatedBy: createRepositoryTestStaffID(t, db),
			}
			err := repo.Create(ctx, note)
			require.NoError(t, err)
		}

		results, err := repo.FindByStudentID(ctx, student.ID)

		require.NoError(t, err)
		assert.Len(t, results, 3)
		// Results should be ordered by note_date ASC, then created_at ASC
		assert.True(t, !results[1].NoteDate.Before(results[0].NoteDate))
	})

	t.Run("returns empty slice when no notes found", func(t *testing.T) {
		results, err := repo.FindByStudentID(ctx, int64(99999999))

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestStudentArrivalNoteRepository_FindByStudentIDAndDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalNoteRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("finds notes for specific date", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		targetDate := timezone.NewDate(2024, 3, 20)

		// Create multiple notes for target date
		for i := 0; i < 2; i++ {
			note := &scheduleModels.StudentArrivalNote{
				StudentID: student.ID,
				NoteDate:  targetDate,
				Content:   fmt.Sprintf("Note %d", i),
				CreatedBy: createRepositoryTestStaffID(t, db),
			}
			err := repo.Create(ctx, note)
			require.NoError(t, err)
		}

		// Create note for different date
		differentDate := targetDate.AddDays(1)
		note := &scheduleModels.StudentArrivalNote{
			StudentID: student.ID,
			NoteDate:  differentDate,
			Content:   "Different date",
			CreatedBy: createRepositoryTestStaffID(t, db),
		}
		err := repo.Create(ctx, note)
		require.NoError(t, err)

		results, err := repo.FindByStudentIDAndDate(ctx, student.ID, targetDate)

		require.NoError(t, err)
		assert.Len(t, results, 2)
		for _, result := range results {
			assert.Equal(t, targetDate, result.NoteDate)
		}
	})

	t.Run("returns empty slice when no notes found", func(t *testing.T) {
		result, err := repo.FindByStudentIDAndDate(ctx, int64(99999999), timezone.TodayDate())

		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestStudentArrivalNoteRepository_FindByStudentIDsAndDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalNoteRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("finds notes for multiple students on same date", func(t *testing.T) {
		student1 := testpkg.CreateTestStudent(t, db, "ArrStudent", "One", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "ArrStudent", "Two", "1b")

		noteDate := timezone.NewDate(2024, 4, 10)

		for _, studentID := range []int64{student1.ID, student2.ID} {
			note := &scheduleModels.StudentArrivalNote{
				StudentID: studentID,
				NoteDate:  noteDate,
				Content:   "Group note",
				CreatedBy: createRepositoryTestStaffID(t, db),
			}
			err := repo.Create(ctx, note)
			require.NoError(t, err)
		}

		results, err := repo.FindByStudentIDsAndDate(ctx, []int64{student1.ID, student2.ID}, noteDate)

		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("returns empty slice for empty student IDs", func(t *testing.T) {
		results, err := repo.FindByStudentIDsAndDate(ctx, []int64{}, timezone.TodayDate())

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestStudentArrivalNoteRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalNoteRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("updates note successfully", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		note := &scheduleModels.StudentArrivalNote{
			StudentID: student.ID,
			NoteDate:  timezone.NewDate(2024, 6, 15),
			Content:   "Original content",
			CreatedBy: createRepositoryTestStaffID(t, db),
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
		note := &scheduleModels.StudentArrivalNote{
			StudentID: 0, // Invalid
			NoteDate:  timezone.NewDate(2024, 6, 15),
			Content:   "Test",
			CreatedBy: createRepositoryTestStaffID(t, db),
		}

		err := repo.Update(ctx, note)

		require.Error(t, err)
	})
}

func TestStudentArrivalNoteRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalNoteRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("lists all notes", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		for i := 1; i <= 3; i++ {
			note := &scheduleModels.StudentArrivalNote{
				StudentID: student.ID,
				NoteDate:  timezone.TodayDate().AddDays(i + 200), // Far future to avoid conflicts
				Content:   "Test note",
				CreatedBy: createRepositoryTestStaffID(t, db),
			}
			err := repo.Create(ctx, note)
			require.NoError(t, err)
		}

		results, err := repo.List(ctx, nil)

		require.NoError(t, err)
		// At least our 3 notes should be present
		assert.GreaterOrEqual(t, len(results), 3)

		options := modelBase.NewQueryOptions().WithPagination(1, 1)
		options.Filter.Equal("student_id", student.ID)
		options.Sorting = (&modelBase.Sorting{}).AddField("note_date", modelBase.SortDesc)
		results, err = repo.List(ctx, options)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, timezone.TodayDate().AddDays(203), results[0].NoteDate)
	})

	t.Run("lists with nil options", func(t *testing.T) {
		results, err := repo.List(ctx, nil)

		require.NoError(t, err)
		assert.NotNil(t, results)
	})
}

func TestStudentArrivalNoteRepository_DeleteByStudentID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalNoteRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes all notes for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		for i := 0; i < 3; i++ {
			note := &scheduleModels.StudentArrivalNote{
				StudentID: student.ID,
				NoteDate:  timezone.TodayDate().AddDays(i),
				Content:   "Note",
				CreatedBy: createRepositoryTestStaffID(t, db),
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

func TestStudentArrivalNoteRepository_DeletePastNotes(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newArrivalNoteRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes only past notes", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Test", "ArrStudent", "1a")

		// Create past notes (will be deleted)
		pastNoteCount := 0
		for i := -10; i < -5; i++ {
			note := &scheduleModels.StudentArrivalNote{
				StudentID: student.ID,
				NoteDate:  timezone.NewDate(2026, 8, 24).AddDays(i),
				Content:   "Past note",
				CreatedBy: createRepositoryTestStaffID(t, db),
			}
			err := repo.Create(ctx, note)
			require.NoError(t, err)
			pastNoteCount++
		}

		// Create future notes (should remain)
		futureNoteCount := 0
		for i := 1; i <= 5; i++ {
			note := &scheduleModels.StudentArrivalNote{
				StudentID: student.ID,
				NoteDate:  timezone.NewDate(2026, 8, 24).AddDays(i),
				Content:   "Future note",
				CreatedBy: createRepositoryTestStaffID(t, db),
			}
			err := repo.Create(ctx, note)
			require.NoError(t, err)
			futureNoteCount++
		}

		cutoffDate := timezone.NewDate(2026, 8, 24)
		rowsAffected, err := repo.DeletePastNotes(ctx, cutoffDate)

		require.NoError(t, err)
		// At minimum, our past notes should be deleted (may be more from other tests)
		assert.GreaterOrEqual(t, rowsAffected, int64(pastNoteCount))

		// Verify THIS student's future notes remain intact
		results, err := repo.FindByStudentID(ctx, student.ID)
		require.NoError(t, err)
		assert.Len(t, results, futureNoteCount)
		for _, result := range results {
			assert.True(t, !result.NoteDate.Before(cutoffDate))
		}
	})
}
