package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// classArrivalBaseline builds the projection with the booking mode off, which
// is the default and what every school without a Halbjahresanmeldung sees.
func classArrivalBaseline(t *testing.T, repos *repositories.Factory) scheduleService.ArrivalBaselineReader {
	t.Helper()
	return scheduleService.NewArrivalBaselineService(
		repos.StudentArrivalSchedule,
		repos.Student,
		repos.ClassArrivalTime,
		repos.ClassArrivalException,
		approvedOfferingProjection(t),
		repos.CareOffering,
		nil,
	)
}

func approvedOfferingProjection(t *testing.T) scheduleService.ApprovedBookingReader {
	t.Helper()
	projection, err := services.NewOwnerApprovedOfferingTestProjection(testpkg.SetupTestDB(t))
	require.NoError(t, err)
	return projection
}

func setClassArrivalTimes(t *testing.T, repos *repositories.Factory, class string, times map[string]string) {
	t.Helper()
	row := &educationModel.ClassArrivalTime{SchoolClass: class, ArrivalTimes: times}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, row.Validate())
	require.NoError(t, repos.ClassArrivalTime.Upsert(testpkg.Ctx(t), row))
}

// mondayOnOrAfter keeps the assertions off weekends without pinning a date.
func mondayOnOrAfter(from timezone.Date) timezone.Date {
	date := from
	for date.Weekday() != time.Monday {
		date = date.AddDays(1)
	}
	return date
}

func TestArrivalBaselineTakesTimeFromTheClass(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	ctx := testpkg.Ctx(t)
	baseline := classArrivalBaseline(t, repos)

	student := testpkg.CreateTestStudent(t, db, "Klara", "Klasse", "3b")
	staff := testpkg.CreateTestStaff(t, db, "Betreuung", "Person")
	setClassArrivalTimes(t, repos, "3b", map[string]string{"mon": "11:45", "wed": "12:45"})

	monday := mondayOnOrAfter(timezone.TodayDate())
	tuesday := monday.AddDays(1)
	wednesday := monday.AddDays(2)

	// The child is in care on Monday and Wednesday. Neither row carries a
	// time: both take it from the class.
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, scheduleModel.WeekdayMonday, staff.ID, "")
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, scheduleModel.WeekdayWednesday, staff.ID, "")

	projection, err := baseline.Project(ctx, []int64{student.ID}, monday, wednesday)
	require.NoError(t, err)

	t.Run("a care day without its own time inherits the class time", func(t *testing.T) {
		row := projection.ForDate(student.ID, monday)
		require.NotNil(t, row)
		assert.Equal(t, "11:45", row.ExpectedArrival.Format("15:04"))
		assert.Equal(t, scheduleModel.ArrivalScheduleSourceClassSchedule, row.Source)
		assert.Equal(t, "3b", row.SourceClass)
	})

	t.Run("each weekday inherits its own class time", func(t *testing.T) {
		row := projection.ForDate(student.ID, wednesday)
		require.NotNil(t, row)
		assert.Equal(t, "12:45", row.ExpectedArrival.Format("15:04"))
	})

	t.Run("a weekday without care carries no arrival time", func(t *testing.T) {
		assert.Nil(t, projection.ForDate(student.ID, tuesday),
			"an arrival time on a day without care is exactly what must not exist")
	})

	t.Run("the inherited time is never written back", func(t *testing.T) {
		stored, storeErr := repos.StudentArrivalSchedule.FindByStudentID(ctx, student.ID)
		require.NoError(t, storeErr)
		require.Len(t, stored, 2)
		for _, row := range stored {
			assert.True(t, row.InheritsClassTime(), "a derived time must stay a read-time projection")
		}
	})
}

func TestArrivalBaselineHandlesStudentWithoutClassTimetable(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	ctx := testpkg.Ctx(t)
	baseline := classArrivalBaseline(t, repos)
	student := testpkg.CreateTestStudent(t, db, "Ohne", "Klasse", "3c")
	staff := testpkg.CreateTestStaff(t, db, "Betreuung", "Person")
	_, err := db.ExecContext(context.Background(), `UPDATE users.students SET school_class = '' WHERE id = ?`, student.ID)
	require.NoError(t, err)
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, scheduleModel.WeekdayMonday, staff.ID, "")
	monday := mondayOnOrAfter(timezone.TodayDate())

	projection, err := baseline.Project(ctx, []int64{student.ID}, monday, monday)
	require.NoError(t, err)
	row := projection.ForDate(student.ID, monday)
	require.NotNil(t, row)
	assert.True(t, row.ExpectedArrival.IsZero())
}

// TestArrivalBaselineClassTimeAloneIsNoCareDay pins the invariant the care-day
// split exists for: the class timetable supplies times, never care days. Half
// the children at the schools without a Halbjahresanmeldung attend fewer days
// than their class has lessons.
func TestArrivalBaselineClassTimeAloneIsNoCareDay(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	ctx := testpkg.Ctx(t)
	baseline := classArrivalBaseline(t, repos)

	student := testpkg.CreateTestStudent(t, db, "Nur", "Montags", "3b")
	staff := testpkg.CreateTestStaff(t, db, "Betreuung", "Montag")
	setClassArrivalTimes(t, repos, "3b", map[string]string{
		"mon": "11:45", "tue": "11:45", "wed": "11:45", "thu": "11:45", "fri": "11:45",
	})
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, scheduleModel.WeekdayMonday, staff.ID, "")

	monday := mondayOnOrAfter(timezone.TodayDate())
	projection, err := baseline.Project(ctx, []int64{student.ID}, monday, monday.AddDays(4))
	require.NoError(t, err)

	require.NotNil(t, projection.ForDate(student.ID, monday))
	for offset := 1; offset <= 4; offset++ {
		date := monday.AddDays(offset)
		assert.Nil(t, projection.ForDate(student.ID, date),
			"the class plans every weekday, the child is in care on Monday only")
	}
}

func TestArrivalBaselineCareDayWithoutAnyClassTime(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	ctx := testpkg.Ctx(t)
	baseline := classArrivalBaseline(t, repos)

	student := testpkg.CreateTestStudent(t, db, "Ohne", "Klassenzeit", "9z")
	staff := testpkg.CreateTestStaff(t, db, "Betreuung", "Ohne")
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, scheduleModel.WeekdayMonday, staff.ID, "")

	monday := mondayOnOrAfter(timezone.NewDate(2026, 8, 24))
	projection, err := baseline.Project(ctx, []int64{student.ID}, monday, monday)
	require.NoError(t, err)

	row := projection.ForDate(student.ID, monday)
	require.NotNil(t, row, "the care day survives even when no class time is maintained yet")
	assert.True(t, row.InheritsClassTime(), "and it carries no arrival time rather than midnight")
	assert.True(t, projection.HasPlan(student.ID, monday))
}

func TestArrivalBaselineManualRowOverridesClassTime(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	ctx := testpkg.Ctx(t)
	baseline := classArrivalBaseline(t, repos)

	student := testpkg.CreateTestStudent(t, db, "Otto", "Override", "3b")
	staff := testpkg.CreateTestStaff(t, db, "Betreuung", "Person")
	setClassArrivalTimes(t, repos, "3b", map[string]string{"mon": "11:45", "tue": "11:45"})
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, scheduleModel.WeekdayMonday, staff.ID, "12:15")
	testpkg.CreateTestArrivalSchedule(t, db, student.ID, scheduleModel.WeekdayTuesday, staff.ID, "")

	monday := mondayOnOrAfter(timezone.TodayDate())
	projection, err := baseline.Project(ctx, []int64{student.ID}, monday, monday.AddDays(1))
	require.NoError(t, err)

	t.Run("the deviating time wins on its weekday", func(t *testing.T) {
		row := projection.ForDate(student.ID, monday)
		require.NotNil(t, row)
		assert.Equal(t, "12:15", row.ExpectedArrival.Format("15:04"))
		assert.Equal(t, scheduleModel.ArrivalScheduleSourceStaff, row.Source)
	})

	t.Run("the class time it hides stays visible underneath", func(t *testing.T) {
		derived := projection.DerivedForDate(student.ID, monday)
		require.NotNil(t, derived)
		assert.Equal(t, "11:45", derived.ExpectedArrival.Format("15:04"))
	})

	t.Run("the other care day keeps inheriting", func(t *testing.T) {
		row := projection.ForDate(student.ID, monday.AddDays(1))
		require.NotNil(t, row)
		assert.Equal(t, "11:45", row.ExpectedArrival.Format("15:04"))
		assert.Equal(t, scheduleModel.ArrivalScheduleSourceClassSchedule, row.Source)
	})
}
