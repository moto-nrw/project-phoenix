package httpintegration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/tenant"

	fixtures "github.com/moto-nrw/project-phoenix/modules/timetable/fixturetest"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The retained arrival and pickup rows share one contract: weekly schedules,
// dated exceptions and dated notes over Care Plan. The suites below run that
// contract once per direction; a kind only says how to build and read its
// rows, because arrival rows carry their clock as ExpectedArrival and pickup
// rows as PickupTime.

type scheduleRows[S any] interface {
	Create(context.Context, *S) error
	FindByID(context.Context, int64) (*S, error)
	Update(context.Context, *S) error
	FindByStudentID(context.Context, int64) ([]*S, error)
	FindByStudentIDAndWeekday(context.Context, int64, int) (*S, error)
	FindByStudentIDsAndWeekday(context.Context, []int64, int) ([]*S, error)
	UpsertSchedule(context.Context, *S) error
	DeleteByStudentID(context.Context, int64) error
	queryListRepository[*S]
}

type exceptionRows[E any] interface {
	Create(context.Context, *E) error
	FindByID(context.Context, int64) (*E, error)
	Update(context.Context, *E) error
	FindByStudentID(context.Context, int64) ([]*E, error)
	FindUpcomingByStudentID(context.Context, int64) ([]*E, error)
	FindByStudentIDAndDate(context.Context, int64, calendar.Date) (*E, error)
	FindByStudentIDsAndDate(context.Context, []int64, calendar.Date) ([]*E, error)
	DeleteByStudentID(context.Context, int64) error
	queryListRepository[*E]
}

type noteRows[N any] interface {
	Create(context.Context, *N) error
	FindByID(context.Context, int64) (*N, error)
	Update(context.Context, *N) error
	FindByStudentID(context.Context, int64) ([]*N, error)
	FindByStudentIDAndDate(context.Context, int64, calendar.Date) ([]*N, error)
	FindByStudentIDsAndDate(context.Context, []int64, calendar.Date) ([]*N, error)
	DeleteByStudentID(context.Context, int64) error
	queryListRepository[*N]
}

type scheduleView struct {
	ID        int64
	StudentID int64
	Weekday   int
	Clock     time.Time
	Notes     *string
}

type exceptionView struct {
	ID            int64
	ExceptionDate calendar.Date
	Reason        *string
	Clock         *time.Time
}

type noteView struct {
	ID       int64
	NoteDate calendar.Date
	Content  string
}

// scheduleKind builds and reads one direction's weekly schedules. hourOffset
// moves every clock of the suite into the direction's part of the day.
type scheduleKind[S any] struct {
	hourOffset int
	repo       func(*bun.DB) scheduleRows[S]
	newRow     func(studentID int64, weekday int, clock time.Time, createdBy int64) *S
	view       func(*S) scheduleView
	edit       func(row *S, clock time.Time, notes *string)
}

type exceptionKind[E any] struct {
	hourOffset int
	repo       func(*bun.DB) exceptionRows[E]
	newRow     func(studentID int64, date calendar.Date, reason string, createdBy int64) *E
	view       func(*E) exceptionView
	edit       func(row *E, reason string, clock *time.Time)
}

type noteKind[N any] struct {
	repo   func(*bun.DB) noteRows[N]
	newRow func(studentID int64, date calendar.Date, content string, createdBy int64) *N
	view   func(*N) noteView
	edit   func(row *N, content string)
}

// careClock is a wall-clock value of the suites, shifted by the kind's offset.
func careClock(hourOffset, hour, minute int) time.Time {
	return time.Date(2024, 1, 1, hour+hourOffset, minute, 0, 0, time.UTC)
}

func careRows(db *bun.DB) *api.CareScheduleSuiteRepositories {
	return api.NewCareScheduleSuiteRepositories(testpkg.MustDatabase(db))
}

var arrivalSchedules = scheduleKind[fixtures.StudentArrivalSchedule]{
	repo: func(db *bun.DB) scheduleRows[fixtures.StudentArrivalSchedule] {
		return fixtures.FromStudentArrivalScheduleRecords(careRows(db).StudentArrivalSchedule).(scheduleRows[fixtures.StudentArrivalSchedule])
	},
	newRow: func(studentID int64, weekday int, clock time.Time, createdBy int64) *fixtures.StudentArrivalSchedule {
		return fixtures.NewStudentArrivalSchedule(func(row *fixtures.StudentArrivalSchedule) {
			row.StudentID, row.Weekday, row.ExpectedArrival, row.CreatedBy = studentID, weekday, clock, createdBy
		})
	},
	view: func(row *fixtures.StudentArrivalSchedule) scheduleView {
		return scheduleView{ID: row.ID, StudentID: row.StudentID, Weekday: row.Weekday, Clock: row.ExpectedArrival, Notes: row.Notes}
	},
	edit: func(row *fixtures.StudentArrivalSchedule, clock time.Time, notes *string) {
		row.ExpectedArrival, row.Notes = clock, notes
	},
}

var pickupSchedules = scheduleKind[fixtures.StudentPickupSchedule]{
	hourOffset: 7,
	repo: func(db *bun.DB) scheduleRows[fixtures.StudentPickupSchedule] {
		return fixtures.FromStudentPickupScheduleRecords(careRows(db).StudentPickupSchedule).(scheduleRows[fixtures.StudentPickupSchedule])
	},
	newRow: func(studentID int64, weekday int, clock time.Time, createdBy int64) *fixtures.StudentPickupSchedule {
		return fixtures.NewStudentPickupSchedule(func(row *fixtures.StudentPickupSchedule) {
			row.StudentID, row.Weekday, row.PickupTime, row.CreatedBy = studentID, weekday, clock, createdBy
		})
	},
	view: func(row *fixtures.StudentPickupSchedule) scheduleView {
		return scheduleView{ID: row.ID, StudentID: row.StudentID, Weekday: row.Weekday, Clock: row.PickupTime, Notes: row.Notes}
	},
	edit: func(row *fixtures.StudentPickupSchedule, clock time.Time, notes *string) {
		row.PickupTime, row.Notes = clock, notes
	},
}

var arrivalExceptions = exceptionKind[fixtures.StudentArrivalException]{
	repo: func(db *bun.DB) exceptionRows[fixtures.StudentArrivalException] {
		return fixtures.FromStudentArrivalExceptionRecords(careRows(db).StudentArrivalException).(exceptionRows[fixtures.StudentArrivalException])
	},
	newRow: func(studentID int64, date calendar.Date, reason string, createdBy int64) *fixtures.StudentArrivalException {
		return fixtures.NewStudentArrivalException(func(row *fixtures.StudentArrivalException) {
			row.StudentID, row.ExceptionDate, row.Reason, row.CreatedBy = studentID, date, testpkg.StrPtr(reason), createdBy
		})
	},
	view: func(row *fixtures.StudentArrivalException) exceptionView {
		return exceptionView{ID: row.ID, ExceptionDate: row.ExceptionDate, Reason: row.Reason, Clock: row.ExpectedArrival}
	},
	edit: func(row *fixtures.StudentArrivalException, reason string, clock *time.Time) {
		row.Reason, row.ExpectedArrival = testpkg.StrPtr(reason), clock
	},
}

var pickupExceptions = exceptionKind[fixtures.StudentPickupException]{
	hourOffset: 7,
	repo: func(db *bun.DB) exceptionRows[fixtures.StudentPickupException] {
		return fixtures.FromStudentPickupExceptionRecords(careRows(db).StudentPickupException).(exceptionRows[fixtures.StudentPickupException])
	},
	newRow: func(studentID int64, date calendar.Date, reason string, createdBy int64) *fixtures.StudentPickupException {
		return fixtures.NewStudentPickupException(func(row *fixtures.StudentPickupException) {
			row.StudentID, row.ExceptionDate, row.Reason, row.CreatedBy = studentID, date, testpkg.StrPtr(reason), createdBy
		})
	},
	view: func(row *fixtures.StudentPickupException) exceptionView {
		return exceptionView{ID: row.ID, ExceptionDate: row.ExceptionDate, Reason: row.Reason, Clock: row.PickupTime}
	},
	edit: func(row *fixtures.StudentPickupException, reason string, clock *time.Time) {
		row.Reason, row.PickupTime = testpkg.StrPtr(reason), clock
	},
}

var arrivalNotes = noteKind[fixtures.StudentArrivalNote]{
	repo: func(db *bun.DB) noteRows[fixtures.StudentArrivalNote] {
		return fixtures.FromStudentArrivalNoteRecords(careRows(db).StudentArrivalNote).(noteRows[fixtures.StudentArrivalNote])
	},
	newRow: func(studentID int64, date calendar.Date, content string, createdBy int64) *fixtures.StudentArrivalNote {
		return fixtures.NewStudentArrivalNote(func(row *fixtures.StudentArrivalNote) {
			row.StudentID, row.NoteDate, row.Content, row.CreatedBy = studentID, date, content, createdBy
		})
	},
	view: func(row *fixtures.StudentArrivalNote) noteView {
		return noteView{ID: row.ID, NoteDate: row.NoteDate, Content: row.Content}
	},
	edit: func(row *fixtures.StudentArrivalNote, content string) { row.Content = content },
}

var pickupNotes = noteKind[fixtures.StudentPickupNote]{
	repo: func(db *bun.DB) noteRows[fixtures.StudentPickupNote] {
		return fixtures.FromStudentPickupNoteRecords(careRows(db).StudentPickupNote).(noteRows[fixtures.StudentPickupNote])
	},
	newRow: func(studentID int64, date calendar.Date, content string, createdBy int64) *fixtures.StudentPickupNote {
		return fixtures.NewStudentPickupNote(func(row *fixtures.StudentPickupNote) {
			row.StudentID, row.NoteDate, row.Content, row.CreatedBy = studentID, date, content, createdBy
		})
	},
	view: func(row *fixtures.StudentPickupNote) noteView {
		return noteView{ID: row.ID, NoteDate: row.NoteDate, Content: row.Content}
	},
	edit: func(row *fixtures.StudentPickupNote, content string) { row.Content = content },
}

func TestCareScheduleRows(t *testing.T) {
	t.Parallel()

	t.Run("arrival", func(t *testing.T) {
		t.Parallel()
		runScheduleRowsSuite(t, arrivalSchedules)
	})
	t.Run("pickup", func(t *testing.T) {
		t.Parallel()
		runScheduleRowsSuite(t, pickupSchedules)
	})
}

func TestCareExceptionRows(t *testing.T) {
	t.Parallel()

	t.Run("arrival", func(t *testing.T) {
		t.Parallel()
		runExceptionRowsSuite(t, arrivalExceptions)
	})
	t.Run("pickup", func(t *testing.T) {
		t.Parallel()
		runExceptionRowsSuite(t, pickupExceptions)
	})
}

func TestCareNoteRows(t *testing.T) {
	t.Parallel()

	t.Run("arrival", func(t *testing.T) {
		t.Parallel()
		runNoteRowsSuite(t, arrivalNotes)
	})
	t.Run("pickup", func(t *testing.T) {
		t.Parallel()
		runNoteRowsSuite(t, pickupNotes)
	})
}

func runScheduleRowsSuite[S any](t *testing.T, kind scheduleKind[S]) {
	// Each direction owns a tenant, so its rows never meet the other's.
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)

	repo := kind.repo(db)
	ctx := testpkg.Ctx(t)
	clock := func(hour, minute int) time.Time { return careClock(kind.hourOffset, hour, minute) }
	create := func(t *testing.T, studentID int64, weekday int, at time.Time) *S {
		t.Helper()
		schedule := kind.newRow(studentID, weekday, at, createRepositoryTestStaffID(t, db))
		require.NoError(t, repo.Create(ctx, schedule))
		return schedule
	}

	t.Run("Create", func(t *testing.T) {
		t.Run("creates schedule successfully", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")

			schedule := kind.newRow(student.ID, fixtures.WeekdayMonday(), clock(7, 50), createRepositoryTestStaffID(t, db))

			err := repo.Create(ctx, schedule)

			require.NoError(t, err)
			assert.Greater(t, kind.view(schedule).ID, int64(0))
		})

		t.Run("fails validation on nil schedule", func(t *testing.T) {
			err := repo.Create(ctx, nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot be nil")
		})

		t.Run("fails validation on invalid schedule", func(t *testing.T) {
			schedule := kind.newRow(1, 10, clock(7, 50), createRepositoryTestStaffID(t, db))

			err := repo.Create(ctx, schedule)

			require.Error(t, err)
		})
	})

	t.Run("FindByID", func(t *testing.T) {
		t.Run("finds schedule by ID", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			schedule := kind.view(create(t, student.ID, fixtures.WeekdayTuesday(), clock(8, 0)))

			result, err := repo.FindByID(ctx, schedule.ID)

			require.NoError(t, err)
			found := kind.view(result)
			assert.Equal(t, schedule.ID, found.ID)
			assert.Equal(t, schedule.StudentID, found.StudentID)
			assert.Equal(t, schedule.Weekday, found.Weekday)
		})

		t.Run("returns error when not found", func(t *testing.T) {
			result, err := repo.FindByID(ctx, int64(99999999))

			require.Error(t, err)
			assert.Nil(t, result)
		})
	})

	t.Run("FindByStudentID", func(t *testing.T) {
		t.Run("finds all schedules for student", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			for _, weekday := range []int{fixtures.WeekdayMonday(), fixtures.WeekdayWednesday(), fixtures.WeekdayFriday()} {
				create(t, student.ID, weekday, clock(7, 50))
			}

			results, err := repo.FindByStudentID(ctx, student.ID)

			require.NoError(t, err)
			assert.Len(t, results, 3)
			assert.Equal(t, fixtures.WeekdayMonday(), kind.view(results[0]).Weekday)
			assert.Equal(t, fixtures.WeekdayWednesday(), kind.view(results[1]).Weekday)
			assert.Equal(t, fixtures.WeekdayFriday(), kind.view(results[2]).Weekday)
		})

		t.Run("returns empty slice when no schedules found", func(t *testing.T) {
			results, err := repo.FindByStudentID(ctx, int64(99999999))

			require.NoError(t, err)
			assert.Empty(t, results)
		})
	})

	t.Run("FindByStudentIDAndWeekday", func(t *testing.T) {
		t.Run("finds schedule for specific weekday", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			schedule := kind.view(create(t, student.ID, fixtures.WeekdayThursday(), clock(8, 15)))

			result, err := repo.FindByStudentIDAndWeekday(ctx, student.ID, fixtures.WeekdayThursday())

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, schedule.ID, kind.view(result).ID)
			assert.Equal(t, fixtures.WeekdayThursday(), kind.view(result).Weekday)
		})

		t.Run("returns nil when not found", func(t *testing.T) {
			result, err := repo.FindByStudentIDAndWeekday(ctx, int64(99999999), fixtures.WeekdayMonday())

			require.NoError(t, err)
			assert.Nil(t, result)
		})
	})

	t.Run("FindByStudentIDsAndWeekday", func(t *testing.T) {
		t.Run("finds schedules for multiple students", func(t *testing.T) {
			student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "1a")
			student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "1b")
			for _, studentID := range []int64{student1.ID, student2.ID} {
				create(t, studentID, fixtures.WeekdayFriday(), clock(8, 0))
			}

			results, err := repo.FindByStudentIDsAndWeekday(ctx, []int64{student1.ID, student2.ID}, fixtures.WeekdayFriday())

			require.NoError(t, err)
			assert.Len(t, results, 2)
		})

		t.Run("returns empty slice for empty student IDs", func(t *testing.T) {
			results, err := repo.FindByStudentIDsAndWeekday(ctx, []int64{}, fixtures.WeekdayMonday())

			require.NoError(t, err)
			assert.Empty(t, results)
		})
	})

	t.Run("UpsertSchedule", func(t *testing.T) {
		t.Run("creates new schedule when doesn't exist", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			schedule := kind.newRow(student.ID, fixtures.WeekdayMonday(), clock(7, 50), createRepositoryTestStaffID(t, db))

			err := repo.UpsertSchedule(ctx, schedule)

			require.NoError(t, err)
			assert.Greater(t, kind.view(schedule).ID, int64(0))
		})

		t.Run("updates existing schedule", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			schedule := kind.newRow(student.ID, fixtures.WeekdayTuesday(), clock(7, 30), createRepositoryTestStaffID(t, db))
			require.NoError(t, repo.UpsertSchedule(ctx, schedule))

			updated := clock(8, 15)
			kind.edit(schedule, updated, testpkg.StrPtr("Updated notes"))

			err := repo.UpsertSchedule(ctx, schedule)

			require.NoError(t, err)

			result, err := repo.FindByStudentIDAndWeekday(ctx, student.ID, fixtures.WeekdayTuesday())
			require.NoError(t, err)
			assert.Equal(t, updated.Hour(), kind.view(result).Clock.Hour())
			assert.Equal(t, "Updated notes", *kind.view(result).Notes)
		})

		t.Run("fails validation on nil schedule", func(t *testing.T) {
			err := repo.UpsertSchedule(ctx, nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot be nil")
		})
	})

	t.Run("Update", func(t *testing.T) {
		t.Run("updates schedule successfully", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			schedule := create(t, student.ID, fixtures.WeekdayMonday(), clock(7, 50))

			updated := clock(9, 0)
			kind.edit(schedule, updated, testpkg.StrPtr("Updated notes"))

			err := repo.Update(ctx, schedule)

			require.NoError(t, err)

			result, err := repo.FindByID(ctx, kind.view(schedule).ID)
			require.NoError(t, err)
			assert.Equal(t, updated.Hour(), kind.view(result).Clock.Hour())
			assert.Equal(t, "Updated notes", *kind.view(result).Notes)
		})

		t.Run("fails validation on nil schedule", func(t *testing.T) {
			err := repo.Update(ctx, nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot be nil")
		})

		t.Run("fails validation on invalid schedule", func(t *testing.T) {
			schedule := kind.newRow(0, fixtures.WeekdayMonday(), clock(7, 50), createRepositoryTestStaffID(t, db))

			err := repo.Update(ctx, schedule)

			require.Error(t, err)
		})
	})

	t.Run("List", func(t *testing.T) {
		t.Run("lists all schedules", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			for _, weekday := range []int{fixtures.WeekdayMonday(), fixtures.WeekdayTuesday()} {
				create(t, student.ID, weekday, clock(7, 50))
			}

			results, err := repo.List(ctx, nil)

			require.NoError(t, err)
			// At least our 2 schedules should be present
			assert.GreaterOrEqual(t, len(results), 2)

			options := newListOptions().WithPagination(1, 1)
			options.Filter.Equal("student_id", student.ID)
			sortedBy(options, "weekday", true)
			results, err = repo.List(ctx, options)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Equal(t, fixtures.WeekdayTuesday(), kind.view(results[0]).Weekday)
		})

		t.Run("lists with nil options", func(t *testing.T) {
			results, err := repo.List(ctx, nil)

			require.NoError(t, err)
			assert.NotNil(t, results)
		})
	})

	t.Run("DeleteByStudentID", func(t *testing.T) {
		t.Run("deletes all schedules for student", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			for _, weekday := range []int{fixtures.WeekdayMonday(), fixtures.WeekdayWednesday()} {
				create(t, student.ID, weekday, clock(7, 50))
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
	})
}

func runExceptionRowsSuite[E any](t *testing.T, kind exceptionKind[E]) {
	// Each direction owns a tenant, so its rows never meet the other's.
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)

	repo := kind.repo(db)
	ctx := testpkg.Ctx(t)
	clock := func(hour, minute int) time.Time { return careClock(kind.hourOffset, hour, minute) }
	create := func(t *testing.T, studentID int64, date calendar.Date, reason string) *E {
		t.Helper()
		exception := kind.newRow(studentID, date, reason, createRepositoryTestStaffID(t, db))
		require.NoError(t, repo.Create(ctx, exception))
		return exception
	}

	t.Run("Create", func(t *testing.T) {
		t.Run("creates exception successfully", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			exception := kind.newRow(student.ID, calendar.NewDate(2024, 2, 14), "Doctor appointment", createRepositoryTestStaffID(t, db))

			err := repo.Create(ctx, exception)

			require.NoError(t, err)
			assert.Greater(t, kind.view(exception).ID, int64(0))
		})

		t.Run("fails validation on nil exception", func(t *testing.T) {
			err := repo.Create(ctx, nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot be nil")
		})
	})

	t.Run("FindByStudentID", func(t *testing.T) {
		t.Run("finds all exceptions for student", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			for _, date := range []calendar.Date{calendar.NewDate(2024, 2, 14), calendar.NewDate(2024, 2, 15)} {
				create(t, student.ID, date, "Test reason")
			}

			results, err := repo.FindByStudentID(ctx, student.ID)

			require.NoError(t, err)
			assert.Len(t, results, 2)
			assert.True(t, kind.view(results[0]).ExceptionDate.Before(kind.view(results[1]).ExceptionDate))
		})
	})

	t.Run("FindUpcomingByStudentID", func(t *testing.T) {
		t.Run("finds only upcoming exceptions", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			create(t, student.ID, calendar.Date(calendar.TodayDate()).AddDays(-7), "Past exception")
			create(t, student.ID, calendar.Date(calendar.TodayDate()).AddDays(7), "Future exception")

			results, err := repo.FindUpcomingByStudentID(ctx, student.ID)

			require.NoError(t, err)
			assert.Len(t, results, 1)
			assert.Equal(t, "Future exception", *kind.view(results[0]).Reason)
		})
	})

	t.Run("FindByStudentIDAndDate", func(t *testing.T) {
		t.Run("finds exception for specific date", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			exceptionDate := calendar.NewDate(2024, 3, 20)
			exception := create(t, student.ID, exceptionDate, "Specific date exception")

			result, err := repo.FindByStudentIDAndDate(ctx, student.ID, exceptionDate)

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, kind.view(exception).ID, kind.view(result).ID)
		})

		t.Run("returns nil when not found", func(t *testing.T) {
			result, err := repo.FindByStudentIDAndDate(ctx, int64(99999999), calendar.Date(calendar.TodayDate()))

			require.NoError(t, err)
			assert.Nil(t, result)
		})
	})

	t.Run("FindByStudentIDsAndDate", func(t *testing.T) {
		t.Run("finds exceptions for multiple students on same date", func(t *testing.T) {
			student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "1a")
			student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "1b")
			staff := testpkg.CreateTestStaff(t, db, "CareStaff", "BulkLookup")
			exceptionDate := calendar.NewDate(2024, 4, 10)

			for _, studentID := range []int64{student1.ID, student2.ID} {
				require.NoError(t, repo.Create(ctx, kind.newRow(studentID, exceptionDate, "Group exception", staff.ID)))
			}

			results, err := repo.FindByStudentIDsAndDate(ctx, []int64{student1.ID, student2.ID}, exceptionDate)

			require.NoError(t, err)
			assert.Len(t, results, 2)
		})

		t.Run("returns empty slice for empty student IDs", func(t *testing.T) {
			results, err := repo.FindByStudentIDsAndDate(ctx, []int64{}, calendar.Date(calendar.TodayDate()))

			require.NoError(t, err)
			assert.Empty(t, results)
		})

		t.Run("matches date in Berlin session", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "CareStudent", "BerlinTZ", "1a")
			staff := testpkg.CreateTestStaff(t, db, "CareStaff", "BerlinTZ")

			// scheduleModels.Date binds as a 'YYYY-MM-DD' literal, so the DB session
			// timezone can no longer shift the stored or queried day. The SET LOCAL
			// stays to pin exactly that: the roundtrip is session-TZ-independent.
			err := db.RunInTx(ctx, nil, func(_ context.Context, tx bun.Tx) error {
				txCtx := tenant.WithTransactionForTest(ctx, &tx)
				_, err := tx.NewRaw(`SET LOCAL timezone = 'Europe/Berlin'`).Exec(txCtx)
				if err != nil {
					return err
				}

				day := calendar.NewDate(2026, 4, 24)
				exception := kind.newRow(student.ID, day, "Berlin session regression", staff.ID)

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
				if got, want := kind.view(results[0]).ID, kind.view(exception).ID; got != want {
					return fmt.Errorf("expected exception ID %d, got %d", want, got)
				}
				if got := kind.view(results[0]).ExceptionDate; got != day {
					return fmt.Errorf("expected exception date %s to roundtrip, got %s", day, got)
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
		})
	})

	t.Run("FindByID", func(t *testing.T) {
		t.Run("finds exception by ID", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			exception := create(t, student.ID, calendar.NewDate(2024, 5, 20), "Test reason")

			result, err := repo.FindByID(ctx, kind.view(exception).ID)

			require.NoError(t, err)
			assert.Equal(t, kind.view(exception).ID, kind.view(result).ID)
			assert.Equal(t, "Test reason", *kind.view(result).Reason)
		})

		t.Run("returns error when not found", func(t *testing.T) {
			result, err := repo.FindByID(ctx, int64(99999999))

			require.Error(t, err)
			assert.Nil(t, result)
		})
	})

	t.Run("Update", func(t *testing.T) {
		t.Run("updates exception successfully", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			original := clock(8, 0)
			exception := kind.newRow(student.ID, calendar.NewDate(2024, 6, 15), "Original reason", createRepositoryTestStaffID(t, db))
			kind.edit(exception, "Original reason", &original)
			require.NoError(t, repo.Create(ctx, exception))

			updated := clock(9, 30)
			kind.edit(exception, "Updated reason", &updated)

			err := repo.Update(ctx, exception)

			require.NoError(t, err)

			result, err := repo.FindByID(ctx, kind.view(exception).ID)
			require.NoError(t, err)
			assert.Equal(t, "Updated reason", *kind.view(result).Reason)
			assert.Equal(t, updated.Hour(), kind.view(result).Clock.Hour())
		})

		t.Run("fails validation on nil exception", func(t *testing.T) {
			err := repo.Update(ctx, nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot be nil")
		})

		t.Run("fails validation on invalid exception", func(t *testing.T) {
			exception := kind.newRow(0, calendar.NewDate(2024, 6, 15), "Test", createRepositoryTestStaffID(t, db))

			err := repo.Update(ctx, exception)

			require.Error(t, err)
		})
	})

	t.Run("List", func(t *testing.T) {
		t.Run("lists all exceptions", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			for i := 1; i <= 3; i++ {
				create(t, student.ID, calendar.NewDate(2099, time.January, i), "Test exception")
			}

			results, err := repo.List(ctx, nil)

			require.NoError(t, err)
			// At least our 3 exceptions should be present
			assert.GreaterOrEqual(t, len(results), 3)

			options := newListOptions().WithPagination(1, 1)
			options.Filter.Equal("student_id", student.ID)
			sortedBy(options, "exception_date", true)
			results, err = repo.List(ctx, options)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Equal(t, calendar.NewDate(2099, time.January, 3), kind.view(results[0]).ExceptionDate)
		})

		t.Run("lists with nil options", func(t *testing.T) {
			results, err := repo.List(ctx, nil)

			require.NoError(t, err)
			assert.NotNil(t, results)
		})
	})

	t.Run("DeleteByStudentID", func(t *testing.T) {
		t.Run("deletes all exceptions for student", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			for i := 0; i < 3; i++ {
				create(t, student.ID, calendar.Date(calendar.TodayDate()).AddDays(i), "Exception")
			}

			err := repo.DeleteByStudentID(ctx, student.ID)

			require.NoError(t, err)

			results, err := repo.FindByStudentID(ctx, student.ID)
			require.NoError(t, err)
			assert.Empty(t, results)
		})
	})
}

func runNoteRowsSuite[N any](t *testing.T, kind noteKind[N]) {
	// Each direction owns a tenant, so its rows never meet the other's.
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)

	repo := kind.repo(db)
	ctx := testpkg.Ctx(t)
	create := func(t *testing.T, studentID int64, date calendar.Date, content string) *N {
		t.Helper()
		note := kind.newRow(studentID, date, content, createRepositoryTestStaffID(t, db))
		require.NoError(t, repo.Create(ctx, note))
		return note
	}

	t.Run("Create", func(t *testing.T) {
		t.Run("creates note successfully", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			note := kind.newRow(student.ID, calendar.NewDate(2024, 2, 14), "Collected by school bus", createRepositoryTestStaffID(t, db))

			err := repo.Create(ctx, note)

			require.NoError(t, err)
			assert.Greater(t, kind.view(note).ID, int64(0))
		})

		t.Run("fails validation on nil note", func(t *testing.T) {
			err := repo.Create(ctx, nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot be nil")
		})

		t.Run("fails validation on invalid note", func(t *testing.T) {
			note := kind.newRow(0, calendar.NewDate(2024, 2, 14), "Test", createRepositoryTestStaffID(t, db))

			err := repo.Create(ctx, note)

			require.Error(t, err)
		})
	})

	t.Run("FindByID", func(t *testing.T) {
		t.Run("finds note by ID", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			note := create(t, student.ID, calendar.NewDate(2024, 5, 20), "Test note")

			result, err := repo.FindByID(ctx, kind.view(note).ID)

			require.NoError(t, err)
			assert.Equal(t, kind.view(note).ID, kind.view(result).ID)
			assert.Equal(t, "Test note", kind.view(result).Content)
		})

		t.Run("returns error when not found", func(t *testing.T) {
			result, err := repo.FindByID(ctx, int64(99999999))

			require.Error(t, err)
			assert.Nil(t, result)
		})
	})

	t.Run("FindByStudentID", func(t *testing.T) {
		t.Run("finds all notes for student", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			for _, date := range []calendar.Date{calendar.NewDate(2024, 2, 14), calendar.NewDate(2024, 2, 15), calendar.NewDate(2024, 2, 16)} {
				create(t, student.ID, date, "Test note")
			}

			results, err := repo.FindByStudentID(ctx, student.ID)

			require.NoError(t, err)
			assert.Len(t, results, 3)
			// Results should be ordered by note_date ASC, then created_at ASC
			assert.True(t, !kind.view(results[1]).NoteDate.Before(kind.view(results[0]).NoteDate))
		})

		t.Run("returns empty slice when no notes found", func(t *testing.T) {
			results, err := repo.FindByStudentID(ctx, int64(99999999))

			require.NoError(t, err)
			assert.Empty(t, results)
		})
	})

	t.Run("FindByStudentIDAndDate", func(t *testing.T) {
		t.Run("finds notes for specific date", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			targetDate := calendar.NewDate(2024, 3, 20)
			for i := 0; i < 2; i++ {
				create(t, student.ID, targetDate, fmt.Sprintf("Note %d", i))
			}
			create(t, student.ID, targetDate.AddDays(1), "Different date")

			results, err := repo.FindByStudentIDAndDate(ctx, student.ID, targetDate)

			require.NoError(t, err)
			assert.Len(t, results, 2)
			for _, result := range results {
				assert.Equal(t, targetDate, kind.view(result).NoteDate)
			}
		})

		t.Run("returns empty slice when no notes found", func(t *testing.T) {
			result, err := repo.FindByStudentIDAndDate(ctx, int64(99999999), calendar.Date(calendar.TodayDate()))

			require.NoError(t, err)
			assert.Empty(t, result)
		})
	})

	t.Run("FindByStudentIDsAndDate", func(t *testing.T) {
		t.Run("finds notes for multiple students on same date", func(t *testing.T) {
			student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "1a")
			student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "1b")
			noteDate := calendar.NewDate(2024, 4, 10)
			for _, studentID := range []int64{student1.ID, student2.ID} {
				create(t, studentID, noteDate, "Group note")
			}

			results, err := repo.FindByStudentIDsAndDate(ctx, []int64{student1.ID, student2.ID}, noteDate)

			require.NoError(t, err)
			assert.Len(t, results, 2)
		})

		t.Run("returns empty slice for empty student IDs", func(t *testing.T) {
			results, err := repo.FindByStudentIDsAndDate(ctx, []int64{}, calendar.Date(calendar.TodayDate()))

			require.NoError(t, err)
			assert.Empty(t, results)
		})
	})

	t.Run("Update", func(t *testing.T) {
		t.Run("updates note successfully", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			note := create(t, student.ID, calendar.NewDate(2024, 6, 15), "Original content")

			kind.edit(note, "Updated content")

			err := repo.Update(ctx, note)

			require.NoError(t, err)

			result, err := repo.FindByID(ctx, kind.view(note).ID)
			require.NoError(t, err)
			assert.Equal(t, "Updated content", kind.view(result).Content)
		})

		t.Run("fails validation on nil note", func(t *testing.T) {
			err := repo.Update(ctx, nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot be nil")
		})

		t.Run("fails validation on invalid note", func(t *testing.T) {
			note := kind.newRow(0, calendar.NewDate(2024, 6, 15), "Test", createRepositoryTestStaffID(t, db))

			err := repo.Update(ctx, note)

			require.Error(t, err)
		})
	})

	t.Run("List", func(t *testing.T) {
		t.Run("lists all notes", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			for i := 1; i <= 3; i++ {
				create(t, student.ID, calendar.NewDate(2099, time.February, i), "Test note")
			}

			results, err := repo.List(ctx, nil)

			require.NoError(t, err)
			// At least our 3 notes should be present
			assert.GreaterOrEqual(t, len(results), 3)

			options := newListOptions().WithPagination(1, 1)
			options.Filter.Equal("student_id", student.ID)
			sortedBy(options, "note_date", true)
			results, err = repo.List(ctx, options)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Equal(t, calendar.NewDate(2099, time.February, 3), kind.view(results[0]).NoteDate)
		})

		t.Run("lists with nil options", func(t *testing.T) {
			results, err := repo.List(ctx, nil)

			require.NoError(t, err)
			assert.NotNil(t, results)
		})
	})

	t.Run("DeleteByStudentID", func(t *testing.T) {
		t.Run("deletes all notes for student", func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a")
			for i := 0; i < 3; i++ {
				create(t, student.ID, calendar.Date(calendar.TodayDate()).AddDays(i), "Note")
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
	})
}
