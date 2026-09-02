package schedule_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Class-wide arrival day exceptions (#2962): the class time of one date is
// replaced for every child of the class, a per-child weekly deviation is
// replaced too, a per-child day exception still wins, and the write side
// refuses past dates and empty classes.

func arrivalServiceWithClassExceptions(t *testing.T, repos *repositories.Factory) scheduleService.ArrivalScheduleService {
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
		scheduleService.WithClassArrivalExceptions(repos.ClassArrivalException),
	)
}

func setClassArrivalException(t *testing.T, repos *repositories.Factory, class string, date timezone.Date, hhmm, reason string) {
	t.Helper()
	parsed, err := time.Parse("15:04", hhmm)
	require.NoError(t, err)
	row := &scheduleModel.ClassArrivalException{
		SchoolClass: class,
		Date:        date,
		ArrivalTime: timezone.NormalizeWallClock(parsed),
	}
	if reason != "" {
		row.Reason = &reason
	}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, row.Validate())
	require.NoError(t, repos.ClassArrivalException.Upsert(testpkg.Ctx(t), row))
}

func TestClassArrivalExceptionReplacesTheClassTimeOnThatDateOnly(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	baseline := classArrivalBaseline(t, repos)

	student := testpkg.CreateTestStudent(t, db, "Ausfall", "Kind", "4a")
	staff := testpkg.CreateTestStaff(t, db, "Betreuung", "Person")
	setClassArrivalTimes(t, repos, "4a", map[string]string{"mon": "13:30", "tue": "13:30"})
	// Care days come from the stored rows with the booking mode off.
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, scheduleModel.WeekdayMonday, staff.ID, "")
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, scheduleModel.WeekdayTuesday, staff.ID, "")

	monday := mondayOnOrAfter(timezone.TodayDate())
	tuesday := monday.AddDays(1)
	nextMonday := monday.AddDays(7)
	setClassArrivalException(t, repos, "4a", monday, "12:45", "Unterricht fällt aus")

	projection, err := baseline.Project(ctx, []int64{student.ID}, monday, nextMonday)
	require.NoError(t, err)

	changed := projection.ForDate(student.ID, monday)
	require.NotNil(t, changed)
	assert.Equal(t, "12:45", changed.ExpectedArrival.Format("15:04"))
	assert.Equal(t, scheduleModel.ArrivalScheduleSourceClassException, changed.Source)
	assert.Equal(t, "4a", changed.SourceClass)
	assert.Equal(t, "Klasse 4a: Unterricht fällt aus", changed.SourceLabel)
	require.NotNil(t, changed.Notes)
	assert.Equal(t, "Klasse 4a: Unterricht fällt aus", *changed.Notes)

	// The next day and the same weekday a week later keep the class time.
	untouched := projection.ForDate(student.ID, tuesday)
	require.NotNil(t, untouched)
	assert.Equal(t, "13:30", untouched.ExpectedArrival.Format("15:04"))
	assert.Equal(t, scheduleModel.ArrivalScheduleSourceClassSchedule, untouched.Source)
	weekLater := projection.ForDate(student.ID, nextMonday)
	require.NotNil(t, weekLater)
	assert.Equal(t, "13:30", weekLater.ExpectedArrival.Format("15:04"))
}

func TestClassArrivalExceptionOutranksAPerChildWeeklyDeviation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	baseline := classArrivalBaseline(t, repos)

	student := testpkg.CreateTestStudent(t, db, "Sechs", "Stunden", "4b")
	staff := testpkg.CreateTestStaff(t, db, "Betreuung", "Person")
	setClassArrivalTimes(t, repos, "4b", map[string]string{"mon": "12:45"})
	// This child normally has six lessons and arrives later than the class.
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, scheduleModel.WeekdayMonday, staff.ID, "13:30")

	monday := mondayOnOrAfter(timezone.TodayDate())
	setClassArrivalException(t, repos, "4b", monday, "11:45", "")

	projection, err := baseline.Project(ctx, []int64{student.ID}, monday, monday)
	require.NoError(t, err)

	row := projection.ForDate(student.ID, monday)
	require.NotNil(t, row)
	assert.Equal(t, "11:45", row.ExpectedArrival.Format("15:04"))
	assert.Equal(t, "Klasse 4b: andere Ankunftszeit", row.SourceLabel)
}

func TestClassArrivalExceptionDoesNotAddACareDay(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	baseline := classArrivalBaseline(t, repos)

	student := testpkg.CreateTestStudent(t, db, "Kein", "Montag", "4c")
	staff := testpkg.CreateTestStaff(t, db, "Betreuung", "Person")
	setClassArrivalTimes(t, repos, "4c", map[string]string{"mon": "12:45", "tue": "12:45"})
	// Only Tuesday is a care day.
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, scheduleModel.WeekdayTuesday, staff.ID, "")

	monday := mondayOnOrAfter(timezone.TodayDate())
	setClassArrivalException(t, repos, "4c", monday, "11:45", "Unterricht fällt aus")

	projection, err := baseline.Project(ctx, []int64{student.ID}, monday, monday)
	require.NoError(t, err)
	assert.Nil(t, projection.ForDate(student.ID, monday), "an exception changes when the class arrives, never whether a child is in care")
}

func TestPerChildDayExceptionWinsOverTheClassException(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	svc := arrivalServiceWithClassExceptions(t, repos)

	first := testpkg.CreateTestStudent(t, db, "Arzt", "Termin", "3a")
	second := testpkg.CreateTestStudent(t, db, "Ohne", "Termin", "3a")
	staff := testpkg.CreateTestStaff(t, db, "Betreuung", "Person")
	setClassArrivalTimes(t, repos, "3a", map[string]string{"mon": "13:30"})
	testpkg.CreateTestArrivalSchedule(t, db, first.ID, scheduleModel.WeekdayMonday, staff.ID, "")
	testpkg.CreateTestArrivalSchedule(t, db, second.ID, scheduleModel.WeekdayMonday, staff.ID, "")

	monday := mondayOnOrAfter(timezone.TodayDate())
	setClassArrivalException(t, repos, "3a", monday, "12:45", "Unterricht fällt aus")
	testpkg.CreateTestArrivalException(t, db, first.ID, monday, staff.ID, "14:00", "Arzttermin")

	times, err := svc.GetBulkEffectiveArrivalTimesForDate(ctx, []int64{first.ID, second.ID}, monday)
	require.NoError(t, err)

	withOwn := times[first.ID]
	require.NotNil(t, withOwn)
	require.NotNil(t, withOwn.ArrivalTime)
	assert.Equal(t, "14:00", withOwn.ArrivalTime.Format("15:04"))
	assert.True(t, withOwn.IsException)
	assert.Nil(t, withOwn.ClassException, "a per-child day exception hides the class one")

	withClass := times[second.ID]
	require.NotNil(t, withClass)
	require.NotNil(t, withClass.ArrivalTime)
	assert.Equal(t, "12:45", withClass.ArrivalTime.Format("15:04"))
	assert.False(t, withClass.IsException)
	require.NotNil(t, withClass.ClassException)
	assert.Equal(t, "3a", withClass.ClassException.SchoolClass)
	assert.Equal(t, "12:45", withClass.ClassException.ArrivalTime)
	assert.Equal(t, "Klasse 3a: Unterricht fällt aus", withClass.ClassException.Label)
	assert.Equal(t, "Klasse 3a: Unterricht fällt aus", withClass.Notes)

	single, err := svc.GetEffectiveArrivalTimeForDate(ctx, second.ID, monday)
	require.NoError(t, err)
	require.NotNil(t, single.ClassException)
	assert.Equal(t, "Klasse 3a: Unterricht fällt aus", single.ClassException.Label)
}

func TestUpsertClassArrivalExceptionWritesAndListsOneRowPerDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	svc := arrivalServiceWithClassExceptions(t, repos)

	testpkg.CreateTestStudent(t, db, "Klassen", "Kind", "2b")
	staff := testpkg.CreateTestStaff(t, db, "Koordination", "Person")
	monday := mondayOnOrAfter(timezone.TodayDate())
	reason := "  Wandertag  "

	first, err := svc.UpsertClassArrivalException(ctx, scheduleService.ClassArrivalExceptionInput{
		SchoolClass: " 2b ",
		Date:        monday,
		ArrivalTime: time.Date(2000, 1, 1, 14, 15, 0, 0, time.UTC),
		Reason:      &reason,
	}, staff.ID)
	require.NoError(t, err)
	assert.Equal(t, "2b", first.SchoolClass)
	require.NotNil(t, first.Reason)
	assert.Equal(t, "Wandertag", *first.Reason)
	require.NotNil(t, first.CreatedBy)
	assert.Equal(t, staff.ID, *first.CreatedBy)

	// Saving the same date again replaces instead of duplicating.
	_, err = svc.UpsertClassArrivalException(ctx, scheduleService.ClassArrivalExceptionInput{
		SchoolClass: "2b",
		Date:        monday,
		ArrivalTime: time.Date(2000, 1, 1, 11, 45, 0, 0, time.UTC),
	}, staff.ID)
	require.NoError(t, err)

	rows, err := svc.ListClassArrivalExceptions(ctx, "2b", monday, monday.AddDays(30))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "11:45", rows[0].ArrivalTime.Format("15:04"))
	assert.Nil(t, rows[0].Reason)

	require.NoError(t, svc.DeleteClassArrivalException(ctx, "2b", monday))
	rows, err = svc.ListClassArrivalExceptions(ctx, "2b", monday, monday.AddDays(30))
	require.NoError(t, err)
	assert.Empty(t, rows)

	err = svc.DeleteClassArrivalException(ctx, "2b", monday)
	assert.True(t, errors.Is(err, scheduleService.ErrClassArrivalExceptionNotFound))
}

func TestUpsertClassArrivalExceptionRefusesPastDatesAndEmptyClasses(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	svc := arrivalServiceWithClassExceptions(t, repos)

	testpkg.CreateTestStudent(t, db, "Klassen", "Kind", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Koordination", "Person")
	noon := time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)

	_, err := svc.UpsertClassArrivalException(ctx, scheduleService.ClassArrivalExceptionInput{
		SchoolClass: "1a",
		Date:        timezone.TodayDate().AddDays(-1),
		ArrivalTime: noon,
	}, staff.ID)
	assert.True(t, errors.Is(err, scheduleService.ErrClassArrivalExceptionPastDate), "got %v", err)

	_, err = svc.UpsertClassArrivalException(ctx, scheduleService.ClassArrivalExceptionInput{
		SchoolClass: "9z",
		Date:        timezone.TodayDate(),
		ArrivalTime: noon,
	}, staff.ID)
	assert.True(t, errors.Is(err, scheduleService.ErrClassArrivalExceptionClassNotFound), "got %v", err)

	weekend := timezone.NewDate(2099, time.March, 7)
	_, err = svc.UpsertClassArrivalException(ctx, scheduleService.ClassArrivalExceptionInput{
		SchoolClass: "1a",
		Date:        weekend,
		ArrivalTime: noon,
	}, staff.ID)
	assert.True(t, errors.Is(err, scheduleService.ErrClassArrivalExceptionWeekend), "got %v", err)

	err = svc.DeleteClassArrivalException(ctx, "1a", timezone.TodayDate().AddDays(-1))
	assert.True(t, errors.Is(err, scheduleService.ErrClassArrivalExceptionPastDate), "got %v", err)
}
