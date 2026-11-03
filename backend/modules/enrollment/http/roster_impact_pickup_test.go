package enrollmenthttp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Schule am Berg, November 2026: a parent's approved weekly request had
// stored Monday and Thursday 14:30 as staff times. A later booking change
// moved both days to the 16:00 offering, but the stored times kept 14:30 in
// force and nothing told the office. The booking change now names every
// weekday whose stored staff time hides the booked time.
func TestUpdateChildOfferings_ReportsPickupTimesThatHideTheBooking(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	careWithPickup := func(name, pickup string) *offeringContract.CareOfferingRecord {
		return createAdjustmentCareOfferingWith(t, env, name, func(offering *offeringContract.CareOfferingRecord) {
			offering.CountsAsCare = true
			offering.PickupTimes = map[string]string{"mon": pickup, "tue": pickup, "wed": pickup, "thu": pickup, "fri": pickup}
		})
	}
	early := careWithPickup("Ganztag bis 14.30", "14:30")
	late := careWithPickup("Ganztag bis 16", "16:00")

	submitted, err := env.requestSvc.Submit(ctx, enrollmentAPI.SubmitRequest{
		TenantID: testpkg.Tenant(t), PhaseID: env.sourcePhase.ID,
		GuardianFirstName: "Eltern", GuardianLastName: "Gehzeit",
		GuardianEmail: "gehzeit-hand@example.com",
		ConsentFlags: map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		},
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
	childID := submitted.Children[0].ID
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
	testpkg.CreateTestPickupSchedule(t, env.db, studentID, timetable.WeekdayThursday, author.ID, "14:30")

	stichtag := calendar.NewDate(2027, 5, 28) // a Friday, like the request of 30.10.
	update, err := testpkg.TenantQuery1(t, ctx, env.db, env.decision.UpdateChildOfferings, enrollmentAPI.UpdateChildOfferingsInput{
		RequestID: submitted.Request.ID, ChildID: childID,
		ActorAccountID: env.creatorID, ActorRole: "admin",
		Reason: "Fußballtraining", EffectiveFrom: &stichtag,
		Offerings: []enrollmentAPI.OfferingAdjustmentSelection{
			{OfferingID: early.ID, SelectedDays: []string{"tue"}},
			{OfferingID: late.ID, SelectedDays: []string{"mon", "wed", "thu"}},
		},
	})
	require.NoError(t, err)

	assert.Equal(t, []enrollmentNative.PickupOverride{
		{Weekday: timetable.WeekdayMonday, Date: calendar.NewDate(2027, 5, 31), StoredTime: "14:30", BookedTime: "16:00"},
		{Weekday: timetable.WeekdayThursday, Date: calendar.NewDate(2027, 6, 3), StoredTime: "14:30", BookedTime: "16:00"},
	}, update.RosterImpact.PickupOverrides,
		"Wednesday repeats the booked time and Tuesday has no stored time; neither hides the booking")
}
