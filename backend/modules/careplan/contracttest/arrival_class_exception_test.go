package contracttest_test

import (
	"errors"
	"testing"
	"time"

	timetable "github.com/moto-nrw/project-phoenix/modules/timetable"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Class-wide arrival day exceptions (#2962): the class time of one date is
// replaced for every child of the class, a per-child weekly deviation is
// replaced too, a per-child day exception still wins, and the write side
// refuses past dates and empty classes.

func arrivalServiceWithClassExceptions(t *testing.T, repos *api.RepositoryTestFactory) careplan.ArrivalScheduleService {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	classes, err := arrivalTimetable.NewClassArrivals(db, func(arrivalTimetable.Observation) {})
	require.NoError(t, err)
	classExceptions, err := api.NewClassArrivalExceptions(classes, api.MustNewPeopleDirectory(testpkg.MustDatabase(db)))
	require.NoError(t, err)
	return nativeArrivalSchedules(t, repos.CarePlan(), classArrivalBaseline(t, repos), api.ClassArrivalPlans(classes), classExceptions)
}

func setClassArrivalException(t *testing.T, _ *api.RepositoryTestFactory, class string, date calendar.Date, hhmm, reason string) {
	t.Helper()
	// Class arrival exceptions belong to Timetable (#2962, #2743).
	db := testpkg.SetupTestDB(t)
	parsed, err := time.Parse("15:04", hhmm)
	require.NoError(t, err)
	row := &careplan.ClassArrivalException{
		SchoolClass: class,
		Date:        calendar.Date(date),
		ArrivalTime: calendar.NormalizeWallClock(parsed),
	}
	if reason != "" {
		row.Reason = &reason
	}
	row.TenantID = testpkg.Tenant(t)
	require.NoError(t, row.Validate())
	require.NoError(t, api.UpsertClassArrivalTestRecord(testpkg.Ctx(t), testpkg.NewDatabase(t, db), row))
}

func TestClassArrivalExceptionReplacesTheClassTimeOnThatDateOnly(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	ctx := testpkg.Ctx(t)
	baseline := classArrivalBaseline(t, repos)

	student := testpkg.CreateTestStudent(t, db, "Ausfall", "Kind", "4a")
	staff := testpkg.CreateTestStaff(t, db, "Betreuung", "Person")
	setClassArrivalTimes(t, "4a", map[string]string{"mon": "13:30", "tue": "13:30"})
	// Care days come from the stored rows with the booking mode off.
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, timetable.WeekdayMonday, staff.ID, "")
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, timetable.WeekdayTuesday, staff.ID, "")

	monday := mondayOnOrAfter(calendar.TodayDate())
	tuesday := monday.AddDays(1)
	nextMonday := monday.AddDays(7)
	setClassArrivalException(t, repos, "4a", monday, "12:45", "Unterricht fällt aus")

	projection, err := baseline.Project(ctx, []int64{student.ID}, monday, nextMonday)
	require.NoError(t, err)

	changed := projection.ForDate(student.ID, monday)
	require.NotNil(t, changed)
	assert.Equal(t, "12:45", changed.ExpectedArrival.Format("15:04"))
	assert.Equal(t, careplan.ScheduleSourceClassException, changed.Source)
	assert.Equal(t, "4a", changed.SourceClass)
	assert.Equal(t, "Klasse 4a: Unterricht fällt aus", changed.SourceLabel)
	require.NotNil(t, changed.Notes)
	assert.Equal(t, "Klasse 4a: Unterricht fällt aus", *changed.Notes)

	// The next day and the same weekday a week later keep the class time.
	untouched := projection.ForDate(student.ID, tuesday)
	require.NotNil(t, untouched)
	assert.Equal(t, "13:30", untouched.ExpectedArrival.Format("15:04"))
	assert.Equal(t, careplan.ScheduleSourceClassSchedule, untouched.Source)
	weekLater := projection.ForDate(student.ID, nextMonday)
	require.NotNil(t, weekLater)
	assert.Equal(t, "13:30", weekLater.ExpectedArrival.Format("15:04"))
}

func TestClassArrivalExceptionOutranksAPerChildWeeklyDeviation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	ctx := testpkg.Ctx(t)
	baseline := classArrivalBaseline(t, repos)

	student := testpkg.CreateTestStudent(t, db, "Sechs", "Stunden", "4b")
	staff := testpkg.CreateTestStaff(t, db, "Betreuung", "Person")
	setClassArrivalTimes(t, "4b", map[string]string{"mon": "12:45"})
	// This child normally has six lessons and arrives later than the class.
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, timetable.WeekdayMonday, staff.ID, "13:30")

	monday := mondayOnOrAfter(calendar.TodayDate())
	setClassArrivalException(t, repos, "4b", monday, "11:45", "")

	projection, err := baseline.Project(ctx, []int64{student.ID}, monday, monday)
	require.NoError(t, err)

	row := projection.ForDate(student.ID, monday)
	require.NotNil(t, row)
	assert.Equal(t, "11:45", row.ExpectedArrival.Format("15:04"))
	assert.Equal(t, "Klasse 4b: andere Ankunftszeit", row.SourceLabel)
}

func TestClassArrivalExceptionStaysOutOfWeeklySchedules(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	ctx := testpkg.Ctx(t)
	svc := arrivalServiceWithClassExceptions(t, repos)

	student := testpkg.CreateTestStudent(t, db, "Wochenplan", "Kind", "4b")
	staff := testpkg.CreateTestStaff(t, db, "Betreuung", "Person")
	setClassArrivalTimes(t, "4b", map[string]string{"mon": "13:30"})
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, timetable.WeekdayMonday, staff.ID, "")
	monday := mondayOnOrAfter(calendar.TodayDate())
	setClassArrivalException(t, repos, "4b", monday, "11:45", "Unterricht fällt aus")

	weekly, err := svc.GetWeeklySchedulesByStudentIDsForDate(ctx, []int64{student.ID}, monday)
	require.NoError(t, err)
	require.Len(t, weekly, 1)
	assert.Equal(t, "13:30", weekly[0].ExpectedArrival.Format("15:04"))
	assert.Equal(t, careplan.ScheduleSourceClassSchedule, weekly[0].Source)

	data, err := svc.GetStudentArrivalDataForDateRange(ctx, student.ID, monday, monday)
	require.NoError(t, err)
	require.Len(t, data.Schedules, 1)
	assert.Equal(t, "13:30", data.Schedules[0].ExpectedArrival.Format("15:04"))
	assert.Equal(t, careplan.ScheduleSourceClassSchedule, data.Schedules[0].Source)

	effective, err := svc.GetEffectiveArrivalTimeForDate(ctx, student.ID, monday)
	require.NoError(t, err)
	require.NotNil(t, effective.ArrivalTime)
	assert.Equal(t, "11:45", effective.ArrivalTime.Format("15:04"))
	require.NotNil(t, effective.ClassException)
}

func TestClassArrivalExceptionDoesNotAddACareDay(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	ctx := testpkg.Ctx(t)
	baseline := classArrivalBaseline(t, repos)

	student := testpkg.CreateTestStudent(t, db, "Kein", "Montag", "4c")
	staff := testpkg.CreateTestStaff(t, db, "Betreuung", "Person")
	setClassArrivalTimes(t, "4c", map[string]string{"mon": "12:45", "tue": "12:45"})
	// Only Tuesday is a care day.
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, timetable.WeekdayTuesday, staff.ID, "")

	monday := mondayOnOrAfter(calendar.TodayDate())
	setClassArrivalException(t, repos, "4c", monday, "11:45", "Unterricht fällt aus")

	projection, err := baseline.Project(ctx, []int64{student.ID}, monday, monday)
	require.NoError(t, err)
	assert.Nil(t, projection.ForDate(student.ID, monday), "an exception changes when the class arrives, never whether a child is in care")
}

func TestPerChildDayExceptionWinsOverTheClassException(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	ctx := testpkg.Ctx(t)
	svc := arrivalServiceWithClassExceptions(t, repos)

	first := testpkg.CreateTestStudent(t, db, "Arzt", "Termin", "3a")
	second := testpkg.CreateTestStudent(t, db, "Ohne", "Termin", "3a")
	staff := testpkg.CreateTestStaff(t, db, "Betreuung", "Person")
	setClassArrivalTimes(t, "3a", map[string]string{"mon": "13:30"})
	testpkg.CreateTestArrivalSchedule(t, db, first.ID, timetable.WeekdayMonday, staff.ID, "")
	testpkg.CreateTestArrivalSchedule(t, db, second.ID, timetable.WeekdayMonday, staff.ID, "")

	monday := mondayOnOrAfter(calendar.TodayDate())
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
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	ctx := testpkg.Ctx(t)
	svc := arrivalServiceWithClassExceptions(t, repos)

	testpkg.CreateTestStudent(t, db, "Klassen", "Kind", "2b")
	staff := testpkg.CreateTestStaff(t, db, "Koordination", "Person")
	monday := mondayOnOrAfter(calendar.TodayDate())
	reason := "  Wandertag  "

	first, err := svc.UpsertClassArrivalException(ctx, careplan.ClassArrivalExceptionInput{
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
	_, err = svc.UpsertClassArrivalException(ctx, careplan.ClassArrivalExceptionInput{
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
	assert.True(t, errors.Is(err, careplan.ErrClassArrivalExceptionNotFound))
}

func TestUpsertClassArrivalExceptionReturnsPersistedMetadata(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	ctx := testpkg.Ctx(t)
	svc := arrivalServiceWithClassExceptions(t, repos)

	testpkg.CreateTestStudent(t, db, "Klassen", "Kind", "2c")
	staff := testpkg.CreateTestStaff(t, db, "Koordination", "Person")
	monday := mondayOnOrAfter(calendar.TodayDate())

	created, err := svc.UpsertClassArrivalException(ctx, careplan.ClassArrivalExceptionInput{
		SchoolClass: "2c",
		Date:        monday,
		ArrivalTime: time.Date(2000, 1, 1, 14, 15, 0, 0, time.UTC),
	}, staff.ID)
	require.NoError(t, err)
	assert.Positive(t, created.ID)
	assert.False(t, created.CreatedAt.IsZero())
	assert.False(t, created.UpdatedAt.IsZero())

	updated, err := svc.UpsertClassArrivalException(ctx, careplan.ClassArrivalExceptionInput{
		SchoolClass: "2c",
		Date:        monday,
		ArrivalTime: time.Date(2000, 1, 1, 11, 45, 0, 0, time.UTC),
	}, staff.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, created.CreatedAt, updated.CreatedAt)
	assert.Equal(t, "11:45", updated.ArrivalTime.Format("15:04"))
}

func TestUpsertClassArrivalExceptionRefusesPastDatesAndEmptyClasses(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	ctx := testpkg.Ctx(t)
	svc := arrivalServiceWithClassExceptions(t, repos)

	testpkg.CreateTestStudent(t, db, "Klassen", "Kind", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Koordination", "Person")
	noon := time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)

	_, err := svc.UpsertClassArrivalException(ctx, careplan.ClassArrivalExceptionInput{
		SchoolClass: "1a",
		Date:        calendar.TodayDate().AddDays(-1),
		ArrivalTime: noon,
	}, staff.ID)
	assert.True(t, errors.Is(err, careplan.ErrClassArrivalExceptionPastDate), "got %v", err)

	weekend := calendar.NewDate(2099, time.March, 7)
	_, err = svc.UpsertClassArrivalException(ctx, careplan.ClassArrivalExceptionInput{
		SchoolClass: "9z",
		Date:        weekend.AddDays(2),
		ArrivalTime: noon,
	}, staff.ID)
	assert.True(t, errors.Is(err, careplan.ErrClassArrivalExceptionClassNotFound), "got %v", err)

	_, err = svc.UpsertClassArrivalException(ctx, careplan.ClassArrivalExceptionInput{
		SchoolClass: "1a",
		Date:        weekend,
		ArrivalTime: noon,
	}, staff.ID)
	assert.True(t, errors.Is(err, careplan.ErrClassArrivalExceptionWeekend), "got %v", err)

	err = svc.DeleteClassArrivalException(ctx, "1a", calendar.TodayDate().AddDays(-1))
	assert.True(t, errors.Is(err, careplan.ErrClassArrivalExceptionPastDate), "got %v", err)
}
