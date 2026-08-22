package schedule_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func arrivalServiceWithClassTimes(t *testing.T, repos *repositories.Factory) scheduleService.ArrivalScheduleService {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	return scheduleService.NewArrivalScheduleServiceWithBaselines(
		repos.StudentArrivalSchedule,
		repos.StudentArrivalException,
		repos.StudentArrivalNote,
		repos.Student,
		repos.Person,
		classArrivalBaseline(t, repos),
		repos.ClassArrivalTime,
		db,
		nil,
	)
}

// TestBulkUpsertBySchoolClassWritesTheClassTimetable pins the pflege win of
// #2414: setting the time for a class writes one class row, not one row per
// child. A child's own time remains the higher-priority deviation (ADR 0005).
func TestBulkUpsertBySchoolClassWritesTheClassTimetable(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	svc := arrivalServiceWithClassTimes(t, repos)

	staff := testpkg.CreateTestStaff(t, db, "Sammel", "Betreuung")
	first := testpkg.CreateTestStudent(t, db, "Eins", "Kind", "7c")
	second := testpkg.CreateTestStudent(t, db, "Zwei", "Kind", "7c")
	testpkg.CreateTestArrivalSchedule(t, db, first.ID, scheduleModel.WeekdayMonday, staff.ID, "12:15")
	testpkg.CreateTestArrivalSchedule(t, db, second.ID, scheduleModel.WeekdayMonday, staff.ID, "")

	result, err := svc.BulkUpsertArrivalSchedules(ctx,
		scheduleService.ArrivalScheduleBulkFilter{SchoolClass: "7c"},
		[]scheduleService.ArrivalScheduleInput{{Weekday: scheduleModel.WeekdayMonday, ArrivalTime: "11:45"}},
		staff.ID,
	)
	require.NoError(t, err)
	assert.Equal(t, 2, result.StudentsAffected)

	t.Run("the class carries the time exactly once", func(t *testing.T) {
		rows, findErr := repos.ClassArrivalTime.FindByClasses(ctx, []string{"7c"})
		require.NoError(t, findErr)
		require.Len(t, rows, 1)
		assert.Equal(t, "11:45", rows[0].ArrivalTimes["mon"])
	})

	t.Run("an own time remains while an inherited row follows the class", func(t *testing.T) {
		firstRows, storeErr := repos.StudentArrivalSchedule.FindByStudentID(ctx, first.ID)
		require.NoError(t, storeErr)
		require.Len(t, firstRows, 1)
		assert.Equal(t, "12:15", firstRows[0].ExpectedArrival.Format("15:04"))

		secondRows, storeErr := repos.StudentArrivalSchedule.FindByStudentID(ctx, second.ID)
		require.NoError(t, storeErr)
		require.Len(t, secondRows, 1)
		assert.True(t, secondRows[0].InheritsClassTime())
	})

	t.Run("no child is reported as overwritten", func(t *testing.T) {
		assert.Empty(t, result.OverwrittenStudents)
	})

	t.Run("the effective times respect the priority order", func(t *testing.T) {
		firstWeek, readErr := svc.GetStudentArrivalSchedules(ctx, first.ID)
		require.NoError(t, readErr)
		require.Len(t, firstWeek, 1)
		assert.Equal(t, "12:15", firstWeek[0].ExpectedArrival.Format("15:04"))
		assert.Equal(t, scheduleModel.ArrivalScheduleSourceStaff, firstWeek[0].Source)

		rows, readErr := svc.GetStudentArrivalSchedules(ctx, second.ID)
		require.NoError(t, readErr)
		require.Len(t, rows, 1)
		assert.Equal(t, "11:45", rows[0].ExpectedArrival.Format("15:04"))
		assert.Equal(t, scheduleModel.ArrivalScheduleSourceClassSchedule, rows[0].Source)
		assert.Equal(t, "7c", rows[0].SourceClass)
	})
}

func TestStoredArrivalScheduleStatusCountsOnlyOwnTimes(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	svc := arrivalServiceWithClassTimes(t, repos)

	staff := testpkg.CreateTestStaff(t, db, "Status", "Betreuung")
	withOwnTime := testpkg.CreateTestStudent(t, db, "Eigene", "Zeit", "7d")
	withInheritedMarker := testpkg.CreateTestStudent(t, db, "Klassen", "Zeit", "7d")
	testpkg.CreateTestArrivalSchedule(t, db, withOwnTime.ID, scheduleModel.WeekdayMonday, staff.ID, "12:15")
	testpkg.CreateTestArrivalSchedule(t, db, withInheritedMarker.ID, scheduleModel.WeekdayMonday, staff.ID, "")

	hasSchedules, err := svc.GetStudentsWithStoredArrivalSchedules(ctx, []int64{withOwnTime.ID, withInheritedMarker.ID})
	require.NoError(t, err)
	assert.True(t, hasSchedules[withOwnTime.ID])
	assert.False(t, hasSchedules[withInheritedMarker.ID])
}

func TestArrivalSchedulesForDateUseTheRequestedProjectionDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	svc := arrivalServiceWithClassTimes(t, repos)

	student := testpkg.CreateTestStudent(t, db, "Export", "Datum", "8d")
	staff := testpkg.CreateTestStaff(t, db, "Betreuung", "Export")
	setClassArrivalTimes(t, repos, "8d", map[string]string{"mon": "11:45"})
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, scheduleModel.WeekdayMonday, staff.ID, "")
	monday := mondayOnOrAfter(timezone.TodayDate())

	rows, err := svc.GetWeeklySchedulesByStudentIDsForDate(ctx, []int64{student.ID}, monday)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "11:45", rows[0].ExpectedArrival.Format("15:04"))
}

type tuesdayOnlyArrivalBaseline struct{}

func (tuesdayOnlyArrivalBaseline) Project(
	_ context.Context,
	studentIDs []int64,
	from, to timezone.Date,
) (*scheduleService.ArrivalBaselineProjection, error) {
	projection := &scheduleService.ArrivalBaselineProjection{
		WeeklyByStudentDate:   make(scheduleService.ArrivalPlansByStudent, len(studentIDs)),
		DerivedByStudentDate:  make(scheduleService.ArrivalPlansByStudent, len(studentIDs)),
		BookingsAuthoritative: true,
	}
	for _, studentID := range studentIDs {
		weekly := scheduleService.ArrivalPlanByDate{}
		for date := from; !date.After(to); date = date.AddDays(1) {
			week := scheduleService.ArrivalWeek{}
			if date.Weekday() == time.Tuesday {
				week[scheduleModel.WeekdayTuesday] = &scheduleModel.StudentArrivalSchedule{
					StudentID: studentID,
					Weekday:   scheduleModel.WeekdayTuesday,
				}
			}
			weekly[date] = week
		}
		projection.WeeklyByStudentDate[studentID] = weekly
		projection.DerivedByStudentDate[studentID] = scheduleService.ArrivalPlanByDate{}
	}
	return projection, nil
}

func TestArrivalDataForDateRangeKeepsMidweekBookingStart(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	svc := scheduleService.NewArrivalScheduleServiceWithBaselines(
		repos.StudentArrivalSchedule,
		repos.StudentArrivalException,
		repos.StudentArrivalNote,
		repos.Student,
		repos.Person,
		tuesdayOnlyArrivalBaseline{},
		repos.ClassArrivalTime,
		db,
		nil,
	)
	student := testpkg.CreateTestStudent(t, db, "Mitte", "Woche", "8f")
	monday := mondayOnOrAfter(timezone.TodayDate())

	data, err := svc.GetStudentArrivalDataForDateRange(ctx, student.ID, monday, monday.AddDays(4))
	require.NoError(t, err)
	require.Len(t, data.Schedules, 1)
	assert.Equal(t, scheduleModel.WeekdayTuesday, data.Schedules[0].Weekday)
}

func TestArrivalDataUsesWholeCurrentWeek(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	svc := scheduleService.NewArrivalScheduleServiceWithBaselines(
		repos.StudentArrivalSchedule,
		repos.StudentArrivalException,
		repos.StudentArrivalNote,
		repos.Student,
		repos.Person,
		tuesdayOnlyArrivalBaseline{},
		repos.ClassArrivalTime,
		db,
		nil,
	)
	student := testpkg.CreateTestStudent(t, db, "Ganz", "Woche", "8j")

	data, err := svc.GetStudentArrivalData(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, data.Schedules, 1)
	assert.Equal(t, scheduleModel.WeekdayTuesday, data.Schedules[0].Weekday)
}

func TestArrivalExceptionsDoNotCreateUnbookedCareDays(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	svc := scheduleService.NewArrivalScheduleServiceWithBaselines(
		repos.StudentArrivalSchedule,
		repos.StudentArrivalException,
		repos.StudentArrivalNote,
		repos.Student,
		repos.Person,
		tuesdayOnlyArrivalBaseline{},
		repos.ClassArrivalTime,
		db,
		nil,
	)
	student := testpkg.CreateTestStudent(t, db, "Ohne", "Buchung", "8k")
	staff := testpkg.CreateTestStaff(t, db, "Betreuung", "Ausnahme")
	monday := mondayOnOrAfter(timezone.TodayDate())
	arrivalTime := time.Date(2000, time.January, 1, 11, 45, 0, 0, time.UTC)
	// The booking-authoritative baseline only returns Tuesday. Monday must stay
	// unplanned even when the stored exception has an arrival time.
	require.NoError(t, svc.CreateStudentArrivalException(ctx, &scheduleModel.StudentArrivalException{
		StudentID:       student.ID,
		ExceptionDate:   monday,
		ExpectedArrival: &arrivalTime,
		CreatedBy:       staff.ID,
	}))

	single, err := svc.GetEffectiveArrivalTimeForDate(ctx, student.ID, monday)
	require.NoError(t, err)
	assert.Nil(t, single.ArrivalTime)
	assert.False(t, single.IsException)

	bulk, err := svc.GetBulkEffectiveArrivalTimesForDate(ctx, []int64{student.ID}, monday)
	require.NoError(t, err)
	assert.Nil(t, bulk[student.ID].ArrivalTime)
	assert.False(t, bulk[student.ID].IsException)
}

func TestWeeklyArrivalReadersUseEachWeekdayDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	svc := scheduleService.NewArrivalScheduleServiceWithBaselines(
		repos.StudentArrivalSchedule,
		repos.StudentArrivalException,
		repos.StudentArrivalNote,
		repos.Student,
		repos.Person,
		tuesdayOnlyArrivalBaseline{},
		repos.ClassArrivalTime,
		db,
		nil,
	)
	student := testpkg.CreateTestStudent(t, db, "Woche", "Dienstag", "8h")

	week, err := svc.GetStudentArrivalSchedules(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, week, 1)
	assert.Equal(t, scheduleModel.WeekdayTuesday, week[0].Weekday)

	byWeekday, err := svc.GetWeeklySchedulesByStudentIDsAndWeekday(ctx, []int64{student.ID}, scheduleModel.WeekdayTuesday)
	require.NoError(t, err)
	require.Len(t, byWeekday, 1)
	assert.Equal(t, scheduleModel.WeekdayTuesday, byWeekday[0].Weekday)
}

func TestArrivalScheduleForWeekdayProjectsBookedDayWithoutStoredRow(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	svc := scheduleService.NewArrivalScheduleServiceWithBaselines(
		repos.StudentArrivalSchedule,
		repos.StudentArrivalException,
		repos.StudentArrivalNote,
		repos.Student,
		repos.Person,
		tuesdayOnlyArrivalBaseline{},
		repos.ClassArrivalTime,
		db,
		nil,
	)
	student := testpkg.CreateTestStudent(t, db, "Ohne", "Zeile", "8i")

	row, err := svc.GetStudentArrivalScheduleForWeekday(ctx, student.ID, scheduleModel.WeekdayTuesday)
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.Equal(t, scheduleModel.WeekdayTuesday, row.Weekday)
}

func TestArrivalDataForDateRangeKeepsOneRowPerWeekday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	svc := scheduleService.NewArrivalScheduleServiceWithBaselines(
		repos.StudentArrivalSchedule,
		repos.StudentArrivalException,
		repos.StudentArrivalNote,
		repos.Student,
		repos.Person,
		tuesdayOnlyArrivalBaseline{},
		repos.ClassArrivalTime,
		db,
		nil,
	)
	student := testpkg.CreateTestStudent(t, db, "Bereich", "Woche", "8g")
	monday := mondayOnOrAfter(timezone.TodayDate())

	data, err := svc.GetStudentArrivalDataForDateRange(ctx, student.ID, monday, monday.AddDays(8))
	require.NoError(t, err)
	require.Len(t, data.Schedules, 1)
	assert.Equal(t, scheduleModel.WeekdayTuesday, data.Schedules[0].Weekday)
}

type firstMondayOnlyArrivalBaseline struct{}

func (firstMondayOnlyArrivalBaseline) Project(
	_ context.Context,
	studentIDs []int64,
	from, to timezone.Date,
) (*scheduleService.ArrivalBaselineProjection, error) {
	projection := &scheduleService.ArrivalBaselineProjection{
		WeeklyByStudentDate:   make(scheduleService.ArrivalPlansByStudent, len(studentIDs)),
		DerivedByStudentDate:  make(scheduleService.ArrivalPlansByStudent, len(studentIDs)),
		BookingsAuthoritative: true,
	}
	for _, studentID := range studentIDs {
		weekly := scheduleService.ArrivalPlanByDate{}
		for date := from; !date.After(to); date = date.AddDays(1) {
			week := scheduleService.ArrivalWeek{}
			if date == from {
				week[scheduleModel.WeekdayMonday] = &scheduleModel.StudentArrivalSchedule{
					StudentID: studentID,
					Weekday:   scheduleModel.WeekdayMonday,
				}
			}
			weekly[date] = week
		}
		projection.WeeklyByStudentDate[studentID] = weekly
		projection.DerivedByStudentDate[studentID] = scheduleService.ArrivalPlanByDate{}
	}
	return projection, nil
}

func TestArrivalDataForDateRangeDropsCareDayAfterBookingEnds(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	svc := scheduleService.NewArrivalScheduleServiceWithBaselines(
		repos.StudentArrivalSchedule,
		repos.StudentArrivalException,
		repos.StudentArrivalNote,
		repos.Student,
		repos.Person,
		firstMondayOnlyArrivalBaseline{},
		repos.ClassArrivalTime,
		db,
		nil,
	)
	student := testpkg.CreateTestStudent(t, db, "Ende", "Buchung", "8l")
	monday := mondayOnOrAfter(timezone.TodayDate())

	data, err := svc.GetStudentArrivalDataForDateRange(ctx, student.ID, monday, monday.AddDays(7))

	require.NoError(t, err)
	assert.Empty(t, data.Schedules)
}

type failingClassArrivalTimeRepository struct {
	educationModel.ClassArrivalTimeRepository
	err error
}

type bookingMondayBaseline struct{}

func (bookingMondayBaseline) Project(
	_ context.Context,
	studentIDs []int64,
	from timezone.Date,
	_ timezone.Date,
) (*scheduleService.ArrivalBaselineProjection, error) {
	projection := &scheduleService.ArrivalBaselineProjection{
		BookingsAuthoritative: true,
		WeeklyByStudentDate:   make(scheduleService.ArrivalPlansByStudent, len(studentIDs)),
		DerivedByStudentDate:  make(scheduleService.ArrivalPlansByStudent, len(studentIDs)),
	}
	for _, studentID := range studentIDs {
		projection.WeeklyByStudentDate[studentID] = scheduleService.ArrivalPlanByDate{
			from: scheduleService.ArrivalWeek{
				scheduleModel.WeekdayMonday: {
					StudentID: studentID,
					Weekday:   scheduleModel.WeekdayMonday,
				},
			},
		}
	}
	return projection, nil
}

func (r failingClassArrivalTimeRepository) FindByClasses(context.Context, []string) ([]*educationModel.ClassArrivalTime, error) {
	return nil, r.err
}

func TestWeeklyWriteFailsWhenClassTimeCannotBeChecked(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	wantErr := errors.New("class-time lookup failed")
	svc := scheduleService.NewArrivalScheduleServiceWithBaselines(
		repos.StudentArrivalSchedule,
		repos.StudentArrivalException,
		repos.StudentArrivalNote,
		repos.Student,
		repos.Person,
		classArrivalBaseline(t, repos),
		failingClassArrivalTimeRepository{ClassArrivalTimeRepository: repos.ClassArrivalTime, err: wantErr},
		db,
		nil,
	)

	staff := testpkg.CreateTestStaff(t, db, "Fehler", "Pruefung")
	student := testpkg.CreateTestStudent(t, db, "Fehler", "Kind", "8e")
	err := svc.UpsertBulkStudentArrivalSchedules(ctx, student.ID, []*scheduleModel.StudentArrivalSchedule{
		{
			StudentID:       student.ID,
			Weekday:         scheduleModel.WeekdayMonday,
			ExpectedArrival: timezone.WallClock(mustParseHHMM(t, "11:45")),
			CreatedBy:       staff.ID,
		},
	})

	require.ErrorIs(t, err, wantErr)
	stored, findErr := repos.StudentArrivalSchedule.FindByStudentID(ctx, student.ID)
	require.NoError(t, findErr)
	assert.Empty(t, stored, "a failed class-time check must not persist a false deviation")
}

func TestBookingModeWeeklyWritePreservesInactiveRows(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	svc := scheduleService.NewArrivalScheduleServiceWithBaselines(
		repos.StudentArrivalSchedule,
		repos.StudentArrivalException,
		repos.StudentArrivalNote,
		repos.Student,
		repos.Person,
		bookingMondayBaseline{},
		repos.ClassArrivalTime,
		db,
		nil,
	)

	staff := testpkg.CreateTestStaff(t, db, "Buchung", "Bewahren")
	student := testpkg.CreateTestStudent(t, db, "Buchung", "Kind", "8f")
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, scheduleModel.WeekdayMonday, staff.ID, "12:15")
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, scheduleModel.WeekdayTuesday, staff.ID, "13:00")

	require.NoError(t, svc.UpsertBulkStudentArrivalSchedules(ctx, student.ID, nil))

	stored, err := repos.StudentArrivalSchedule.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, stored, 2)
	assert.Equal(t, scheduleModel.WeekdayMonday, stored[0].Weekday)
	assert.Equal(t, "12:15", stored[0].ExpectedArrival.Format("15:04"))
	assert.Equal(t, scheduleModel.WeekdayTuesday, stored[1].Weekday)
	assert.Equal(t, "13:00", stored[1].ExpectedArrival.Format("15:04"))
}

// TestWeeklyWriteCollapsesIntoTheClassTime pins the ADR 0005 promise: a value
// identical to the class time is not stored as a deviation, so the child keeps
// following the class when the Unterrichtsschluss moves.
func TestWeeklyWriteCollapsesIntoTheClassTime(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	svc := arrivalServiceWithClassTimes(t, repos)

	staff := testpkg.CreateTestStaff(t, db, "Deckungs", "Gleich")
	student := testpkg.CreateTestStudent(t, db, "Deckungs", "Kind", "8d")
	setClassArrivalTimes(t, repos, "8d", map[string]string{"mon": "11:45", "tue": "11:45"})

	sameAsClass := timezone.WallClock(mustParseHHMM(t, "11:45"))
	deviating := timezone.WallClock(mustParseHHMM(t, "12:15"))
	require.NoError(t, svc.UpsertBulkStudentArrivalSchedules(ctx, student.ID,
		[]*scheduleModel.StudentArrivalSchedule{
			{StudentID: student.ID, Weekday: scheduleModel.WeekdayMonday, ExpectedArrival: sameAsClass, CreatedBy: staff.ID},
			{StudentID: student.ID, Weekday: scheduleModel.WeekdayTuesday, ExpectedArrival: deviating, CreatedBy: staff.ID},
		}))

	stored, err := repos.StudentArrivalSchedule.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, stored, 2)
	byWeekday := map[int]*scheduleModel.StudentArrivalSchedule{}
	for _, row := range stored {
		byWeekday[row.Weekday] = row
	}
	assert.True(t, byWeekday[scheduleModel.WeekdayMonday].InheritsClassTime(),
		"a value identical to the class time is no deviation")
	assert.False(t, byWeekday[scheduleModel.WeekdayTuesday].InheritsClassTime())
	assert.Equal(t, "12:15", byWeekday[scheduleModel.WeekdayTuesday].ExpectedArrival.Format("15:04"))
}

func mustParseHHMM(t *testing.T, hhmm string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", hhmm)
	require.NoError(t, err)
	return parsed
}
