package enrollmenthttp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentNative "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	offeringContract "github.com/moto-ogs/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// childWithEnteredPickups approves a child that books the early offering on
// Monday and Thursday and the late one on Tuesday and Wednesday, and stores
// staff pickup times Monday 14:30, Wednesday 16:00 and Thursday 15:15.
func childWithEnteredPickups(t *testing.T, serviceStart calendar.Date) (env *decisionTestEnv, requestID, childID int64, early, late *offeringContract.CareOfferingRecord) {
	t.Helper()
	env = setupDecisionTest(t)
	setSourcePhaseServiceStartDate(t, env, serviceStart)
	ctx := testpkg.Ctx(t)
	careWithPickup := func(name, pickup string) *offeringContract.CareOfferingRecord {
		return createAdjustmentCareOfferingWith(t, env, name, func(offering *offeringContract.CareOfferingRecord) {
			offering.CountsAsCare = true
			offering.PickupTimes = map[string]string{"mon": pickup, "tue": pickup, "wed": pickup, "thu": pickup, "fri": pickup}
		})
	}
	early = careWithPickup("Ganztag bis 14.30", "14:30")
	late = careWithPickup("Ganztag bis 16", "16:00")

	submitted, err := env.requestSvc.Submit(ctx, enrollmentAPI.SubmitRequest{
		TenantID: testpkg.Tenant(t), PhaseID: env.sourcePhase.ID,
		GuardianFirstName: "Eltern", GuardianLastName: "Gehzeit",
		GuardianEmail: "gehzeit-hand@example.com",
		ConsentFlags: testFormValues(map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		}),
		Children: []enrollmentAPI.SubmitChild{{
			FirstName: "Mateo", LastName: "Gehzeit",
			DateOfBirth:      calendar.NewDate(2019, 3, 4),
			TargetGradeLevel: testpkg.Int16Ptr(1),
			OfferingIDs:      []int64{early.ID, late.ID},
			OfferingDays: []enrollmentAPI.SubmitOfferingDays{
				{OfferingID: early.ID, SelectedDays: []string{"mon", "thu"}},
				{OfferingID: late.ID, SelectedDays: []string{"tue", "wed"}},
			},
		}},
	})
	require.NoError(t, err)
	childID = submitted.Children[0].ID
	outcome, err := testpkg.TenantQuery1(t, ctx, env.db, env.decision.Decide, enrollmentAPI.DecideInput{
		RequestID: submitted.Request.ID, ChildID: childID,
		Status: enrollmentNative.DecisionApproved, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	studentID := *outcome.Child.CreatedStudentID

	author := testpkg.CreateTestStaff(t, env.db, "Gehzeit", "Hand")
	testpkg.CreateTestPickupSchedule(t, env.db, studentID, timetable.WeekdayMonday, author.ID, "14:30")
	testpkg.CreateTestPickupSchedule(t, env.db, studentID, timetable.WeekdayWednesday, author.ID, "16:00")
	// Thursday was set by hand to a time no booking ever had.
	testpkg.CreateTestPickupSchedule(t, env.db, studentID, timetable.WeekdayThursday, author.ID, "15:15")

	return env, submitted.Request.ID, childID, early, late
}

// Schule am Berg, November 2026: a parent's approved weekly request had
// stored Monday and Thursday 14:30 as staff times. A later booking change
// moved both days to the 16:00 offering, but the stored times kept 14:30 in
// force and nothing told the office. The booking change now names every
// weekday whose stored staff time hides the booked time.
func TestUpdateChildOfferings_ReportsPickupTimesThatHideTheBooking(t *testing.T) {
	t.Parallel()

	env, requestID, childID, early, late := childWithEnteredPickups(t, decisionTestToday)
	ctx := testpkg.Ctx(t)

	// A Friday ahead of today (24.08.), like the request of 30.10.
	stichtag := calendar.NewDate(2026, 9, 11)
	update, err := testpkg.TenantQuery1(t, ctx, env.db, env.decision.UpdateChildOfferings, enrollmentAPI.UpdateChildOfferingsInput{
		RequestID: requestID, ChildID: childID,
		ActorAccountID: env.creatorID, ActorRole: "admin",
		Reason: "Fußballtraining", EffectiveFrom: &stichtag,
		Offerings: []enrollmentAPI.OfferingAdjustmentSelection{
			{OfferingID: early.ID, SelectedDays: []string{"tue"}},
			{OfferingID: late.ID, SelectedDays: []string{"mon", "wed", "thu"}},
		},
	})
	require.NoError(t, err)

	// Monday's 14:30 is the time the old booking sets until the change, so
	// removing it today changes no earlier day. Thursday's 15:15 still
	// applies until the change; removing it today would end it too early.
	assert.Equal(t, []enrollmentNative.PickupOverride{
		{Weekday: timetable.WeekdayMonday, Date: calendar.NewDate(2026, 9, 14), StoredTime: "14:30", BookedTime: "16:00", ResetNow: true},
		{Weekday: timetable.WeekdayThursday, Date: calendar.NewDate(2026, 9, 17), StoredTime: "15:15", BookedTime: "16:00", ResetNow: false},
	}, update.RosterImpact.PickupOverrides,
		"Wednesday repeats the booked time and Tuesday has no stored time; neither hides the booking")
}

// A change without a date for a phase that starts later books nothing before
// the first service day. The check compares the stored times with the booking
// from that day on, so the hint is not lost.
func TestUpdateChildOfferings_ReportsPickupTimesFromTheFirstServiceDay(t *testing.T) {
	t.Parallel()

	serviceStart := calendar.NewDate(2026, 9, 21) // Monday, four weeks after today (24.08.)
	env, requestID, childID, early, late := childWithEnteredPickups(t, serviceStart)
	ctx := testpkg.Ctx(t)

	update, err := testpkg.TenantQuery1(t, ctx, env.db, env.decision.UpdateChildOfferings, enrollmentAPI.UpdateChildOfferingsInput{
		RequestID: requestID, ChildID: childID,
		ActorAccountID: env.creatorID, ActorRole: "admin",
		Reason: "Fußballtraining",
		Offerings: []enrollmentAPI.OfferingAdjustmentSelection{
			{OfferingID: early.ID, SelectedDays: []string{"tue"}},
			{OfferingID: late.ID, SelectedDays: []string{"mon", "wed", "thu"}},
		},
	})
	require.NoError(t, err)

	// Before the first service day no booking sets a time, so removing a
	// stored time today would change those days: no reset yet.
	assert.Equal(t, []enrollmentNative.PickupOverride{
		{Weekday: timetable.WeekdayMonday, Date: serviceStart, StoredTime: "14:30", BookedTime: "16:00", ResetNow: false},
		{Weekday: timetable.WeekdayThursday, Date: calendar.NewDate(2026, 9, 24), StoredTime: "15:15", BookedTime: "16:00", ResetNow: false},
	}, update.RosterImpact.PickupOverrides)
}
