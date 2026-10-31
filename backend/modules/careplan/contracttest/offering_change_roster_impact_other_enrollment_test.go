package contracttest_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The approval of a parent request takes the same path as a direct booking
// change (#5366). The child holds two approved enrollments: the school year
// with Ganztag until 16:00 on Mondays, and a custom phase for existing
// students with the Randstunde and Ganztag until 14:30. A parent request
// applies to the enrollment of the latest-starting period that covers its
// date, here the custom phase. The parent drops its Ganztag and keeps the
// Randstunde. The school year's Monday until 16:00 still carries the Lernzeit
// until 14:30, so the approval reports nothing and the review card does not
// wait for "Fertig". With one enrollment the same change reports the Lernzeit
// (TestOfferingChangeRequestService_Decide_ReportsOnlyRegeltermineWithoutCarryingBooking).
func TestOfferingChangeRequestService_Decide_OtherEnrollmentCarriesTheRegeltermin(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	careOffering := func(phaseID int64, name, pickup string) *offeringContract.CareOfferingRecord {
		return createAdjustmentCareOfferingWith(t, env, name, func(o *offeringContract.CareOfferingRecord) {
			o.PhaseID = phaseID
			o.CountsAsCare, o.CountsAsCareSet = true, true
			o.PickupTimes = map[string]string{"mon": pickup, "tue": pickup, "wed": pickup, "thu": pickup, "fri": pickup}
		})
	}
	schoolYearGanztag := careOffering(env.sourcePhase.ID, "Ganztag bis 16 Uhr", "16:00")
	_, _, studentID := submitApprovedAdjustmentChild(t, env, "offer-change-carry-5366@example.com", "Carry",
		[]*offeringContract.CareOfferingRecord{schoolYearGanztag})
	switchDate := calendar.Date(env.sourcePhase.ServiceStartDate).AddDays(150)
	for switchDate.Weekday() != time.Monday {
		switchDate = switchDate.AddDays(1)
	}

	custom := &enrollmentTest.Phase{
		Name:             "carry-other-" + t.Name(),
		Kind:             enrollmentTest.PhaseKindCustom,
		ServiceStartDate: enrollmentTest.Date(switchDate.AddDays(-30)),
		ServiceEndDate:   enrollmentTest.Date(switchDate.AddDays(60)),
		IsActive:         true,
		CareOverflowMode: enrollmentTest.PhaseCareOverflowWaitlist,
		FormSchemaID:     env.sourcePhase.FormSchemaID,
	}
	custom.TenantID = testpkg.Tenant(t)
	require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), env.db, env.reposEnrollment.Enrollment().InsertPhase, custom))
	randstunde := careOffering(custom.ID, "Randstunde", "12:45")
	ganztag := careOffering(custom.ID, "Ganztag bis 14.30", "14:30")
	enrollExistingStudent(t, env, custom.ID, studentID, randstunde, ganztag)

	period := testpkg.CreateTestCalendarPeriod(t, env.db, "Carry "+t.Name(),
		calendar.Date(env.sourcePhase.ServiceStartDate), calendar.Date(env.sourcePhase.ServiceEndDate))
	room := testpkg.CreateTestRoom(t, env.db, "Carry")
	lernzeit := createCareOfferingTemplateGroup(t, env.db, "Lernzeit JG 1")
	instance := testpkg.CreateTestActivityInstance(t, env.db, switchDate.AddDays(7), room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &lernzeit.ID, CalendarPeriodID: &period.ID, StartHHMM: "13:45", EndHHMM: "14:30",
	})
	testpkg.CreateTestInstanceStudent(t, env.db, instance.ID, studentID, "")

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID: studentID, AccountID: env.creatorID, EffectiveFrom: switchDate,
		Selections: []careplan.OfferingChangeSelection{{OfferingID: randstunde.ID, SelectedDays: []string{"mon"}}},
	})
	require.NoError(t, err)
	impact, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.Decide, careplan.OfferingChangeDecisionInput{
		RequestID: row.ID, Approve: true, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	assert.Empty(t, impact.NotAdjusted, "the school year's Monday until 16:00 carries the Lernzeit")
}

// enrollExistingStudent submits a request of the phase for the existing
// student with the offerings on Mondays and approves it.
func enrollExistingStudent(t *testing.T, env *decisionTestEnv, phaseID, studentID int64, offerings ...*offeringContract.CareOfferingRecord) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	ids := make([]int64, 0, len(offerings))
	days := make([]enrollmentTest.SubmitOfferingDays, 0, len(offerings))
	for _, offering := range offerings {
		ids = append(ids, offering.ID)
		days = append(days, enrollmentTest.SubmitOfferingDays{OfferingID: offering.ID, SelectedDays: []string{"mon"}})
	}
	submitted, err := testpkg.TenantQuery1(t, ctx, env.db, env.requestSvc.Submit, enrollmentTest.SubmitRequest{
		TenantID: testpkg.Tenant(t), PhaseID: phaseID,
		GuardianFirstName: "Eltern", GuardianLastName: "Carry", GuardianEmail: "offer-change-carry-5366-other@example.com",
		ConsentFlags: consentFlags(t),
		Children: []enrollmentTest.SubmitChild{{
			FirstName: "Kind", LastName: "Carry",
			DateOfBirth:      calendar.NewDate(2018, 4, 15),
			TargetGradeLevel: testpkg.Int16Ptr(2),
			OfferingIDs:      ids,
			OfferingDays:     days,
		}},
	})
	require.NoError(t, err)
	require.Len(t, submitted.Children, 1)
	matchChildToExistingStudent(t, env, submitted.Children[0].ID, studentID)
	outcome, err := testpkg.TenantQuery1(t, ctx, env.db, env.decision.Decide, enrollmentTest.DecideInput{
		RequestID: submitted.Request.ID, ChildID: submitted.Children[0].ID,
		Status: enrollmentTest.DecisionApproved, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	require.Equal(t, studentID, *outcome.Child.CreatedStudentID)
}
