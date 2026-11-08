package enrollmenthttp_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #5365: the planning period ends before the Stichtag and the school has not
// created the next one yet. The open hand list on Fridays stays, and the
// period the school creates later plans the child on it again, so the
// booking change must still report it.
func TestUpdateChildOfferings_ReportsHandListWithoutAFuturePeriod(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	stichtag := calendar.NewDate(2027, 5, 31)
	createCareOfferingTestPeriod(t, env.db, "ends-before-stichtag",
		calendar.Date(env.sourcePhase.ServiceStartDate).AddDays(-31), stichtag.AddDays(-1))

	randstunde := createAdjustmentCareOfferingWith(t, env, "Kurzbetreuung", nil)
	handList := createCareOfferingTemplateGroup(t, env.db, "Freitagsliste")
	createCareOfferingTemplateSchedule(t, env.db, handList.ID, timetable.WeekdayFriday, nil)

	submitted, err := env.requestSvc.Submit(ctx, enrollmentAPI.SubmitRequest{
		TenantID: testpkg.Tenant(t), PhaseID: env.sourcePhase.ID,
		GuardianFirstName: "Eltern", GuardianLastName: "OhneZeitraum",
		GuardianEmail: "ohne-zeitraum-5365@example.com",
		ConsentFlags: testFormValues(map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		}),
		Children: []enrollmentAPI.SubmitChild{{
			FirstName: "Ben", LastName: "OhneZeitraum",
			DateOfBirth:      calendar.NewDate(2019, 3, 4),
			TargetGradeLevel: testpkg.Int16Ptr(1),
			OfferingIDs:      []int64{randstunde.ID},
			OfferingDays: []enrollmentAPI.SubmitOfferingDays{{
				OfferingID: randstunde.ID, SelectedDays: []string{"mon", "fri"},
			}},
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
	// An open hand row: no period pin, no end, no weekday scope.
	_, err = env.timetable.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: studentID, ActivityGroupID: handList.ID,
		ValidFrom: string(env.sourcePhase.ServiceStartDate),
	})
	require.NoError(t, err)

	update, err := testpkg.TenantQuery1(t, ctx, env.db, env.decision.UpdateChildOfferings, enrollmentAPI.UpdateChildOfferingsInput{
		RequestID: submitted.Request.ID, ChildID: childID,
		ActorAccountID: env.creatorID, ActorRole: "admin",
		Reason: "Randstunde freitags abgemeldet", EffectiveFrom: &stichtag,
		Offerings: []enrollmentAPI.OfferingAdjustmentSelection{{
			OfferingID: randstunde.ID, SelectedDays: []string{"mon"},
		}},
	})
	require.NoError(t, err)

	require.Len(t, update.RosterImpact.NotAdjusted, 1, "the open hand list keeps the child on Fridays")
	assert.Equal(t, handList.ID, update.RosterImpact.NotAdjusted[0].TemplateID)
	assert.Equal(t, []int{timetable.WeekdayFriday}, update.RosterImpact.NotAdjusted[0].Weekdays)
	rows := listStudentEnrollmentRowsForDecisionTest(t, env, studentID)
	var handRows []studentEnrollment
	for _, row := range rows {
		if row.ActivityGroupID == handList.ID {
			handRows = append(handRows, row)
		}
	}
	require.Len(t, handRows, 1, "the change does not edit the hand list")
	assert.Nil(t, handRows[0].ValidUntil)

	// The school creates the next period later: materialization plans the
	// child from the hand row again, which is what the notice warned about.
	createCareOfferingTestPeriod(t, env.db, "from-stichtag", stichtag, calendar.Date(env.sourcePhase.ServiceEndDate))
	module, _, _, err := api.NewTimetableTestModule(testpkg.NewDatabase(t, env.db), testpkg.TenantRuntime(t, env.db))
	require.NoError(t, err)
	friday := calendar.NewDate(2027, 6, 4)
	_, err = module.Materialization.MaterializeForTenant(ctx, friday, friday, timetable.MaterializationSourceManual)
	require.NoError(t, err)
	var instanceIDs []int64
	require.NoError(t, env.db.NewSelect().Column("id").TableExpr("schedule.activity_instances").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("activity_group_id = ?", handList.ID).
		Where("date = ?", friday).
		Scan(ctx, &instanceIDs))
	require.Len(t, instanceIDs, 1)
	assert.True(t, instanceHasStudent(t, env, instanceIDs[0], studentID), "the new period plans the child from the hand row")
}
