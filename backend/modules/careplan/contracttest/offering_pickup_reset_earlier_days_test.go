package contracttest_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	timetable "github.com/moto-nrw/project-phoenix/modules/timetable"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A staff pickup row has no validity period: resetting it on a future day
// removes it for every earlier day of the weekday too. While the old booking
// still sets another time on such a day, the reset is refused and names the
// last day the stored time is needed (#5643).
func TestOfferingPickupReset_FutureDayKeepsStoredTimeForEarlierDays(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	setSourcePhaseServiceStartDate(t, env, decisionTestToday.AddDays(-30))
	ctx := testpkg.Ctx(t)

	oldOffering := createPickupTimeOffering(t, env, "gehzeit-reset-old",
		[]string{"mon"}, map[string]string{"mon": "14:30"})
	newOffering := createPickupTimeOffering(t, env, "gehzeit-reset-new",
		[]string{"mon"}, map[string]string{"mon": "16:00"})
	studentID, childID := submitAndApproveOfferingChild(
		t, env, oldOffering.ID, "gehzeit-reset-earlier@example.com", "Stichtag", 2,
	)
	// decisionTestToday is a Monday; the change applies two weeks later.
	effectiveFrom := decisionTestToday.AddDays(14)
	require.Equal(t, time.Monday, effectiveFrom.Weekday())
	require.NoError(t, api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant).ScheduleRequestChildOfferings(
		ctx, childID, enrollmentTest.Date(effectiveFrom),
		[]*enrollmentTest.RequestChildOffering{{CareOfferingID: newOffering.ID, SelectedDays: []string{"mon"}}},
	))
	author := testpkg.CreateTestStaff(t, env.db, "Gehzeit", "Stichtag")
	testpkg.CreateTestPickupSchedule(t, env.db, studentID, timetable.WeekdayMonday, author.ID, "15:15")

	err := pickupTimeService(t, env).ResetStudentPickupDayToOffering(ctx, studentID, effectiveFrom)
	var stillNeeded *careplan.PickupResetStillNeededError
	require.ErrorAs(t, err, &stillNeeded)
	assert.Equal(t, effectiveFrom.AddDays(-7), stillNeeded.NeededUntil,
		"the last Monday before the change still needs the stored time")
	assert.Equal(t, effectiveFrom.AddDays(-6), stillNeeded.PossibleFrom)

	stored, err := careplantest.NewScheduleRecords(env.repos.CarePlan()).StudentPickupSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	assert.Equal(t, "15:15", stored[0].PickupTime.Format("15:04"),
		"a refused reset must keep the stored time for the days before the change")
}

// When the old booking already sets the stored time on every earlier day,
// removing the row changes none of them, so the reset goes through.
func TestOfferingPickupReset_FutureDayAllowedWhenEarlierDaysMatchBooking(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	setSourcePhaseServiceStartDate(t, env, decisionTestToday.AddDays(-30))
	ctx := testpkg.Ctx(t)

	oldOffering := createPickupTimeOffering(t, env, "gehzeit-reset-same-old",
		[]string{"mon"}, map[string]string{"mon": "14:30"})
	newOffering := createPickupTimeOffering(t, env, "gehzeit-reset-same-new",
		[]string{"mon"}, map[string]string{"mon": "16:00"})
	studentID, childID := submitAndApproveOfferingChild(
		t, env, oldOffering.ID, "gehzeit-reset-same@example.com", "Gleich", 2,
	)
	effectiveFrom := decisionTestToday.AddDays(14)
	require.NoError(t, api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant).ScheduleRequestChildOfferings(
		ctx, childID, enrollmentTest.Date(effectiveFrom),
		[]*enrollmentTest.RequestChildOffering{{CareOfferingID: newOffering.ID, SelectedDays: []string{"mon"}}},
	))
	author := testpkg.CreateTestStaff(t, env.db, "Gehzeit", "Gleich")
	testpkg.CreateTestPickupSchedule(t, env.db, studentID, timetable.WeekdayMonday, author.ID, "14:30")

	require.NoError(t, pickupTimeService(t, env).ResetStudentPickupDayToOffering(ctx, studentID, effectiveFrom))
	stored, err := careplantest.NewScheduleRecords(env.repos.CarePlan()).StudentPickupSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	assert.Empty(t, stored)
}
