package enrollmenthttp_test

import (
	"context"
	"testing"
	"time"

	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestPhaseExpiryService_ApprovedRolloverWithInactiveOfferingStaysOpen(t *testing.T) {
	t.Parallel()

	env := setupDecisionTestWithSettings(t, stubActivationSettings{
		mode: settingValueActivationImmediate,
	})
	ctx := testpkg.Ctx(t)

	sourceOffering := &offeringContract.CareOfferingRecord{
		PhaseID:         env.sourcePhase.ID,
		Name:            "Phase expiry source offering",
		DaysOfWeekMode:  offeringContract.DaysOfWeekModeFixed,
		AvailableDays:   []string{"mon"},
		IsActive:        true,
		CountsAsCare:    false,
		CountsAsCareSet: true,
		IncludesLunch:   true,
	}
	sourceOffering.TenantID = testpkg.Tenant(t)
	require.NoError(t, newCareOfferingFixtures(env.reposCare.CarePlan).Create(ctx, sourceOffering))

	submitted, err := env.requestSvc.Submit(ctx, enrollmentAPI.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Phasenende",
		GuardianEmail:     "phase-expiry-decision@example.com",
		ConsentFlags:      rolloverConsentFlags(),
		Children: []enrollmentAPI.SubmitChild{{
			FirstName:        "Lina",
			LastName:         "Phasenende",
			DateOfBirth:      calendar.NewDate(2018, 4, 15),
			TargetGradeLevel: testpkg.Int16Ptr(2),
			OfferingIDs:      []int64{sourceOffering.ID},
		}},
	})
	require.NoError(t, err)
	require.Len(t, submitted.Children, 1)

	sourceOutcome, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), env.decision.Decide, enrollmentAPI.DecideInput{
		RequestID:  submitted.Request.ID,
		ChildID:    submitted.Children[0].ID,
		Status:     enrollmentNative.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, sourceOutcome.Child.CreatedStudentID)
	studentID := *sourceOutcome.Child.CreatedStudentID
	require.NoError(t, testpkg.TenantCommand2(t, ctx, testpkg.SetupTestDB(t), env.repos.Student.UpdateStatus, studentID, peopleValues.StudentStatusInactive))

	rolloverRequest := validRolloverRequest(
		env.rolloverTestEnv,
		enrollmentNative.PhaseRolloverModeOptOut,
		false,
	)
	rolloverRequest.ServiceStartDate = calendar.NewDate(2027, 8, 1)
	rolloverRequest.RolloverDeadline = time.Date(2027, 7, 15, 0, 0, 0, 0, time.UTC)
	rollover, err := env.rolloverSvc.CreatePhaseFromSource(ctx, rolloverRequest)
	require.NoError(t, err)

	rolledChildren, err := testpkg.TenantQuery2(t, ctx, env.db, env.repos.Enrollment().ChildrenByPhaseStatuses, rollover.Phase.ID, []string{enrollmentNative.ChildStatusAutoRenewed})
	require.NoError(t, err)
	require.Len(t, rolledChildren, 1)
	targetOfferings, err := newCareOfferingFixtures(env.reposCare.CarePlan).ListByPhase(ctx, rollover.Phase.ID)
	require.NoError(t, err)
	require.Len(t, targetOfferings, 1)
	targetOfferings[0].IsActive = false
	require.NoError(t, newCareOfferingFixtures(env.reposCare.CarePlan).Update(ctx, targetOfferings[0]))

	scheduledDecision := newDecisionServiceForTest(env.rolloverTestEnv, nil, nil)
	targetOutcome, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), scheduledDecision.Decide, enrollmentAPI.DecideInput{
		RequestID:  rolledChildren[0].RequestID,
		ChildID:    rolledChildren[0].ID,
		Status:     enrollmentNative.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, targetOutcome.Child.CreatedStudentID)
	student, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), env.repos.Student.FindByID, any(*targetOutcome.Child.CreatedStudentID))
	require.NoError(t, err)
	require.NotNil(t, student.EnrolledFrom)
	assert.Equal(t, peopleValues.StudentStatusPending, student.Status,
		"scheduled approval after phase-driven inactivation must leave the future child pending")
	assert.Equal(t, calendar.Date(rollover.Phase.ServiceStartDate), *student.EnrolledFrom,
		"the real approval must replace the student's source enrollment window")

	warnings, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), NewTestPhaseExpiryWarnings(NewTestPhaseExpirySnapshots(env.repos.Enrollment(), expiryDecisionStudents{env.repos.Student}, expiryDecisionOfferings{env.reposCare.CarePlan}, env.repos.Enrollment())).
		ListWarnings, calendar.NewDate(2027, 7, 3))
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Equal(t, enrollmentNative.PhaseExpiryStateIncomplete, warnings[0].State)
	assert.Equal(t, 1, warnings[0].UnresolvedChildren)

	targetOfferings[0].IsActive = true
	require.NoError(t, newCareOfferingFixtures(env.reposCare.CarePlan).Update(ctx, targetOfferings[0]))
	warnings, err = testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), NewTestPhaseExpiryWarnings(NewTestPhaseExpirySnapshots(env.repos.Enrollment(), expiryDecisionStudents{env.repos.Student}, expiryDecisionOfferings{env.reposCare.CarePlan}, env.repos.Enrollment())).
		ListWarnings, calendar.NewDate(2027, 7, 3))
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

type expiryDecisionStudents struct {
	students interface {
		List(context.Context, map[string]any) ([]*peopleValues.StudentState, error)
	}
}

func (d expiryDecisionStudents) ListEnrolledStudents(ctx context.Context) ([]enrollmentNative.PhaseExpiryStudent, error) {
	students, err := d.students.List(ctx, map[string]any{})
	if err != nil {
		return nil, err
	}
	result := make([]enrollmentNative.PhaseExpiryStudent, 0, len(students))
	for _, student := range students {
		if student.IsAlumnus() {
			continue
		}
		result = append(result, toExpiryDecisionStudent(student))
	}
	return result, nil
}

func toExpiryDecisionStudent(student *peopleValues.StudentState) enrollmentNative.PhaseExpiryStudent {
	row := enrollmentNative.PhaseExpiryStudent{
		ID: student.ID, Status: string(student.Status),
	}
	if student.EnrolledFrom != nil {
		row.EnrolledFrom = student.EnrolledFrom.String()
	}
	if student.EnrolledUntil != nil {
		row.EnrolledUntil = student.EnrolledUntil.String()
	}
	return row
}

type expiryDecisionOfferings struct{ query careplan.Query }

func (d expiryDecisionOfferings) ListCareOfferings(ctx context.Context) ([]enrollmentNative.PhaseExpiryOffering, error) {
	values, err := d.query.ListCareOfferings(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderID})
	if err != nil {
		return nil, err
	}
	result := make([]enrollmentNative.PhaseExpiryOffering, 0, len(values))
	for _, value := range values {
		result = append(result, enrollmentNative.PhaseExpiryOffering{
			ID: value.ID, TenantID: value.TenantID, PhaseID: value.PhaseID,
			DaysOfWeekMode: value.DaysOfWeekMode, AvailableDays: value.AvailableDays, IsActive: value.IsActive,
		})
	}
	return result, nil
}
