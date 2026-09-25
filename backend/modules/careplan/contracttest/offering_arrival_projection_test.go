package contracttest_test

import (
	"context"
	"testing"
	"time"

	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"

	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bookingModeArrivalBaseline mirrors projectedPickupReader: the arrival
// projection with enrollment.bookings_authoritative switched on, so the
// approved booking links decide which weekdays a child is in care (#2414).
func bookingModeArrivalBaseline(t *testing.T, env *decisionTestEnv, authoritative bool) careplan.ArrivalBaselineReader {
	t.Helper()
	classArrivalQueries, err := arrivalTimetable.NewClassArrivalQueries(env.db, func(arrivalTimetable.Observation) {})
	if err != nil {
		panic(err)
	}
	baselines, err := newArrivalBaselinesFixture(env.repos.CarePlan(), repositories.MustNewPeopleDirectory(env.db), classArrivalQueries, approvedOfferingTestProjection(env.repos), func(context.Context) (bool, error) { return authoritative, nil })
	if err != nil {
		panic(err)
	}
	return baselines
}

func bookingModeCareDays(t *testing.T, env *decisionTestEnv, authoritative bool) careplan.CareDayQuery {
	t.Helper()
	participation := newTestCareLifecycle(env.db, repositories.CareLifecycleTestConfig{
		BookingsAuthoritative: func(context.Context) (bool, error) { return authoritative, nil },
	})
	return careplanCompose.NewCareDays(careplanCompose.CareDayDependencies{
		ArrivalBaselines: bookingModeArrivalBaseline(t, env, authoritative),
		Records:          env.repos.CarePlan(),

		PickupBaselines: newPickupBaselineService(env.repos.CarePlan(), approvedOfferingTestProjection(env.repos)),

		CareParticipation: participation,
	})
}

func bookingModePickupBaseline(env *decisionTestEnv, authoritative bool) careplan.PickupBaselineReader {
	baselines, err := careplanCompose.NewPickupBaselines(env.repos.CarePlan(), approvedOfferingTestProjection(env.repos),
		func(context.Context) (bool, error) { return authoritative, nil })
	if err != nil {
		panic(err)
	}
	return baselines
}

func bookingModePickupService(env *decisionTestEnv, authoritative bool) careplan.PickupScheduleService {
	result, err := careplanCompose.NewPickupSchedules(env.db, env.repos.CarePlan(), bookingModePickupBaseline(env, authoritative), nil, nil, nil)
	if err != nil {
		panic(err)
	}
	return result
}

func assertLegacyPickupRestored(t *testing.T, env *decisionTestEnv, studentID int64, date timezone.Date) {
	t.Helper()
	restored, err := bookingModePickupService(env, false).GetEffectivePickupTimeForDate(testpkg.Ctx(t), studentID, date)
	require.NoError(t, err)
	require.NotNil(t, restored)
	require.NotNil(t, restored.PickupTime)
	assert.Equal(t, "16:00", restored.PickupTime.Format("15:04"),
		"turning booking authority off must restore the preserved weekly plan")
}

func createArrivalOffering(t *testing.T, env *decisionTestEnv, name string, days []string) *enrollmentModels.CareOffering {
	t.Helper()
	offering := &enrollmentModels.CareOffering{
		PhaseID:        env.sourcePhase.ID,
		Name:           uniqueSchemaName(name + "-" + t.Name()),
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  days,
		IsActive:       true,
		CountsAsCare:   true,
	}
	offering.TenantID = testpkg.Tenant(t)
	require.NoError(t, enrollmentSvc.NewCareOfferingRepository(env.repos.CarePlan()).Create(testpkg.Ctx(t), offering))
	return offering
}

func setArrivalClassTimes(t *testing.T, env *decisionTestEnv, class string, times map[string]string) {
	t.Helper()
	row := &educationModel.ClassArrivalTime{SchoolClass: class, ArrivalTimes: times}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, row.Validate())
	require.NoError(t, env.repos.ClassArrivalTime.Upsert(testpkg.Ctx(t), row))
}

func setStudentClass(t *testing.T, env *decisionTestEnv, studentID int64, class string) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	student, err := env.repos.Student.FindByID(ctx, studentID)
	require.NoError(t, err)
	student.SchoolClass = class
	require.NoError(t, env.repos.Student.Update(ctx, student))
}

func TestArrivalProjection_BookingModeLimitsCareDaysToBookedWeekdays(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, decisionTestToday.AddDays(-30))
	ctx := testpkg.Ctx(t)

	offering := createArrivalOffering(t, env, "ankunft-montags", []string{"mon"})
	studentID, _ := submitAndApproveOfferingChild(
		t, env, offering.ID, "ankunft-montags@example.com", "Mona", 2,
	)
	setStudentClass(t, env, studentID, "3b")
	setArrivalClassTimes(t, env, "3b", map[string]string{"mon": "11:45", "tue": "11:45"})

	monday := nextWeekday(decisionTestToday, time.Monday)
	baseline := bookingModeArrivalBaseline(t, env, true)
	projection, err := baseline.Project(ctx, []int64{studentID}, monday, monday.AddDays(1))
	require.NoError(t, err)

	t.Run("the booked weekday carries the class time", func(t *testing.T) {
		row := projection.ForDate(studentID, monday)
		require.NotNil(t, row)
		assert.Equal(t, "11:45", row.ExpectedArrival.Format("15:04"))
	})

	t.Run("an unbooked weekday carries nothing even though the class plans it", func(t *testing.T) {
		assert.Nil(t, projection.ForDate(studentID, monday.AddDays(1)))
	})

	t.Run("with the booking mode off the stored care days decide instead", func(t *testing.T) {
		classOnly := bookingModeArrivalBaseline(t, env, false)
		fallback, fallbackErr := classOnly.Project(ctx, []int64{studentID}, monday, monday.AddDays(1))
		require.NoError(t, fallbackErr)
		assert.Nil(t, fallback.ForDate(studentID, monday),
			"without stored care days and without the booking mode nothing is planned")

		staff := testpkg.CreateTestStaff(t, env.db, "Betreuung", "Fallback")
		testpkg.CreateTestArrivalSchedule(t, env.db, studentID, scheduleModels.WeekdayTuesday, staff.ID, "")
		withRow, rowErr := classOnly.Project(ctx, []int64{studentID}, monday, monday.AddDays(1))
		require.NoError(t, rowErr)
		require.NotNil(t, withRow.ForDate(studentID, monday.AddDays(1)))
		assert.Equal(t, "11:45", withRow.ForDate(studentID, monday.AddDays(1)).ExpectedArrival.Format("15:04"))
	})
}

// TestArrivalProjection_StaleRowOnUnbookedDayIsIgnored reproduces the OGS am
// Berg incident of 19.08.: two deregistered children kept arrival rows Mo-Fr
// and stayed "expected". With the booking mode on those rows no longer plan
// anything, and nothing has to be deleted from the database by hand.
func TestArrivalProjection_StaleRowOnUnbookedDayIsIgnored(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, decisionTestToday.AddDays(-30))
	ctx := testpkg.Ctx(t)

	offering := createArrivalOffering(t, env, "ankunft-altzeile", []string{"mon"})
	studentID, _ := submitAndApproveOfferingChild(
		t, env, offering.ID, "ankunft-altzeile@example.com", "Alt", 2,
	)
	setStudentClass(t, env, studentID, "3b")
	setArrivalClassTimes(t, env, "3b", map[string]string{"mon": "11:45", "thu": "11:45"})

	staff := testpkg.CreateTestStaff(t, env.db, "Betreuung", "Altzeile")
	testpkg.CreateTestArrivalSchedule(t, env.db, studentID, scheduleModels.WeekdayThursday, staff.ID, "11:45")

	monday := nextWeekday(decisionTestToday, time.Monday)
	thursday := monday.AddDays(3)

	baseline := bookingModeArrivalBaseline(t, env, true)
	projection, err := baseline.Project(ctx, []int64{studentID}, monday, thursday)
	require.NoError(t, err)

	assert.NotNil(t, projection.ForDate(studentID, monday), "the booked day still plans")
	assert.Nil(t, projection.ForDate(studentID, thursday),
		"a stored row on an unbooked weekday must not make the child expected")

	stored, err := env.repos.StudentArrivalSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	assert.Len(t, stored, 1, "the stale row is ignored on read, never deleted behind the school's back")
}

func TestArrivalProjection_BookingEndStopsTheArrival(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, decisionTestToday.AddDays(-30))
	ctx := testpkg.Ctx(t)

	offering := createArrivalOffering(t, env, "ankunft-abmeldung", []string{"mon"})
	studentID, childID := submitAndApproveOfferingChild(
		t, env, offering.ID, "ankunft-abmeldung@example.com", "Enno", 2,
	)
	setStudentClass(t, env, studentID, "4a")
	setArrivalClassTimes(t, env, "4a", map[string]string{"mon": "12:45"})

	firstMonday := nextWeekday(decisionTestToday.AddDays(1), time.Monday)
	secondMonday := firstMonday.AddDays(7)

	// Abmeldung: the booking stops at the second Monday (half-open window).
	err := repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant).ScheduleRequestChildOfferings(ctx, childID, capability.Date(secondMonday), nil)
	require.NoError(t, err)

	baseline := bookingModeArrivalBaseline(t, env, true)
	projection, projectErr := baseline.Project(ctx, []int64{studentID}, firstMonday, secondMonday)
	require.NoError(t, projectErr)

	t.Run("before the end date the arrival still stands", func(t *testing.T) {
		require.NotNil(t, projection.ForDate(studentID, firstMonday))
	})

	t.Run("from the end date on the arrival is gone without any cleanup job", func(t *testing.T) {
		assert.Nil(t, projection.ForDate(studentID, secondMonday))
	})
}

// TestArrivalProjection_StaleRowNoLongerMarksTheChildExpected closes the loop
// on the 19.08. incident through the surface that actually caused it: the
// care-day derivation behind the Erwartet-Status, not just the projection.
func TestArrivalProjection_StaleRowNoLongerMarksTheChildExpected(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, decisionTestToday.AddDays(-30))
	ctx := testpkg.Ctx(t)

	offering := createArrivalOffering(t, env, "erwartet-status", []string{"mon"})
	studentID, _ := submitAndApproveOfferingChild(
		t, env, offering.ID, "erwartet-status@example.com", "Erwin", 2,
	)
	setStudentClass(t, env, studentID, "3b")
	setArrivalClassTimes(t, env, "3b", map[string]string{"mon": "11:45", "thu": "11:45"})

	// The child is booked for Monday only, but a row from before the
	// Abmeldung still sits on Thursday.
	staff := testpkg.CreateTestStaff(t, env.db, "Betreuung", "Erwartet")
	testpkg.CreateTestArrivalSchedule(t, env.db, studentID, scheduleModels.WeekdayThursday, staff.ID, "11:45")

	careDays := bookingModeCareDays(t, env, true)

	monday := nextWeekday(decisionTestToday, time.Monday)
	thursday := monday.AddDays(3)

	booked, err := careDays.ResolveForDate(ctx, []int64{studentID}, monday)
	require.NoError(t, err)
	assert.True(t, booked[studentID].Expected(), "the booked day still expects the child")

	stale, err := careDays.ResolveForDate(ctx, []int64{studentID}, thursday)
	require.NoError(t, err)
	assert.False(t, stale[studentID].Expected(),
		"a stale arrival row on an unbooked weekday must not keep a deregistered child expected")
}

func TestPickupProjection_StaleRowCannotAddAuthoritativeCareDay(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	setSourcePhaseServiceStartDate(t, env, decisionTestToday.AddDays(-30))
	ctx := testpkg.Ctx(t)

	offering := createArrivalOffering(t, env, "pickup-boundary", []string{"mon"})
	studentID, _ := submitAndApproveOfferingChild(
		t, env, offering.ID, "pickup-boundary@example.com", "Paula", 2,
	)
	staff := testpkg.CreateTestStaff(t, env.db, "Abholung", "Grenze")
	testpkg.CreateTestPickupSchedule(t, env.db, studentID, scheduleModels.WeekdayMonday, staff.ID, "15:30")
	testpkg.CreateTestPickupSchedule(t, env.db, studentID, scheduleModels.WeekdayTuesday, staff.ID, "16:00")

	pickups := bookingModePickupService(env, true)

	tuesday := nextWeekday(decisionTestToday, time.Tuesday)
	effective, err := pickups.GetEffectivePickupTimeForDate(ctx, studentID, tuesday)
	require.NoError(t, err)
	assert.Nil(t, effective, "an unbooked pickup row must not escape as an effective time")

	week, err := pickups.GetStudentPickupSchedules(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, week, 1, "the weekly views must keep the booked day and hide the stale row")
	assert.Equal(t, scheduleModels.WeekdayMonday, week[0].Weekday)

	require.NoError(t, pickups.UpsertBulkStudentPickupSchedules(ctx, studentID, week))
	stored, err := env.repos.StudentPickupSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, stored, 2, "saving the visible booked week must preserve ignored legacy rows")

	assertLegacyPickupRestored(t, env, studentID, tuesday)
}
