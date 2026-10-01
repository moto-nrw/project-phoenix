package enrollment_test

import (
	"context"
	"testing"
	"time"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/api/enrollment"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestPhaseExpiryService_ApprovedRolloverWithInactiveOfferingStaysOpen(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{
		mode: configModels.EnrollmentActivationModeImmediate,
	})
	defer cleanup()
	ctx := testpkg.Ctx(t)

	sourceOffering := &enrollmentModels.CareOffering{
		PhaseID:         env.sourcePhase.ID,
		Name:            "Phase expiry source offering",
		DaysOfWeekMode:  enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:   []string{"mon"},
		IsActive:        true,
		CountsAsCare:    false,
		CountsAsCareSet: true,
		IncludesLunch:   true,
	}
	sourceOffering.TenantID = testpkg.Tenant(t)
	require.NoError(t, newCareOfferingFixtures(env.repos.CarePlan()).Create(ctx, sourceOffering))

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
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: testpkg.Int16Ptr(2),
			OfferingIDs:      []int64{sourceOffering.ID},
		}},
	})
	require.NoError(t, err)
	require.Len(t, submitted.Children, 1)

	sourceOutcome, err := env.decision.Decide(ctx, enrollmentAPI.DecideInput{
		RequestID:  submitted.Request.ID,
		ChildID:    submitted.Children[0].ID,
		Status:     enrollmentOwner.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, sourceOutcome.Child.CreatedStudentID)
	studentID := *sourceOutcome.Child.CreatedStudentID
	require.NoError(t, env.repos.Student.UpdateStatus(ctx, studentID, usersModels.StudentStatusInactive))

	rolloverRequest := validRolloverRequest(
		env.rolloverTestEnv,
		enrollmentModels.PhaseRolloverModeOptOut,
		false,
	)
	rolloverRequest.ServiceStartDate = timezone.NewDate(2027, 8, 1)
	rolloverRequest.RolloverDeadline = time.Date(2027, 7, 15, 0, 0, 0, 0, time.UTC)
	rollover, err := env.rolloverSvc.CreatePhaseFromSource(ctx, rolloverRequest)
	require.NoError(t, err)

	rolledChildren, err := env.repos.Enrollment().ChildrenByPhaseStatuses(
		ctx,
		rollover.Phase.ID,
		[]string{enrollmentModels.ChildStatusAutoRenewed},
	)
	require.NoError(t, err)
	require.Len(t, rolledChildren, 1)
	targetOfferings, err := newCareOfferingFixtures(env.repos.CarePlan()).ListByPhase(ctx, rollover.Phase.ID)
	require.NoError(t, err)
	require.Len(t, targetOfferings, 1)
	targetOfferings[0].IsActive = false
	require.NoError(t, newCareOfferingFixtures(env.repos.CarePlan()).Update(ctx, targetOfferings[0]))

	scheduledDecision := newDecisionServiceForTest(env.rolloverTestEnv, nil, nil)
	targetOutcome, err := scheduledDecision.Decide(ctx, enrollmentAPI.DecideInput{
		RequestID:  rolledChildren[0].RequestID,
		ChildID:    rolledChildren[0].ID,
		Status:     enrollmentOwner.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, targetOutcome.Child.CreatedStudentID)
	student, err := env.repos.Student.FindByID(ctx, *targetOutcome.Child.CreatedStudentID)
	require.NoError(t, err)
	require.NotNil(t, student.EnrolledFrom)
	assert.Equal(t, usersModels.StudentStatusPending, student.Status,
		"scheduled approval after phase-driven inactivation must leave the future child pending")
	assert.Equal(t, timezone.Date(rollover.Phase.ServiceStartDate), *student.EnrolledFrom,
		"the real approval must replace the student's source enrollment window")

	warnings, err := enrollmentAPI.NewTestPhaseExpiryWarnings(enrollmentAPI.NewTestPhaseExpirySnapshots(env.repos.Enrollment(), expiryDecisionStudents{env.repos.Student}, expiryDecisionOfferings{env.repos.CarePlan()}, env.repos.Enrollment())).
		ListWarnings(ctx, timezone.NewDate(2027, 7, 3))
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Equal(t, enrollmentOwner.PhaseExpiryStateIncomplete, warnings[0].State)
	assert.Equal(t, 1, warnings[0].UnresolvedChildren)

	targetOfferings[0].IsActive = true
	require.NoError(t, newCareOfferingFixtures(env.repos.CarePlan()).Update(ctx, targetOfferings[0]))
	warnings, err = enrollmentAPI.NewTestPhaseExpiryWarnings(enrollmentAPI.NewTestPhaseExpirySnapshots(env.repos.Enrollment(), expiryDecisionStudents{env.repos.Student}, expiryDecisionOfferings{env.repos.CarePlan()}, env.repos.Enrollment())).
		ListWarnings(ctx, timezone.NewDate(2027, 7, 3))
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

type expiryDecisionStudents struct{ students usersModels.StudentRepository }

func (d expiryDecisionStudents) ListEnrolledStudents(ctx context.Context) ([]enrollmentOwner.PhaseExpiryStudent, error) {
	students, err := d.students.List(ctx, map[string]any{})
	if err != nil {
		return nil, err
	}
	result := make([]enrollmentOwner.PhaseExpiryStudent, 0, len(students))
	for _, student := range students {
		if student.IsAlumnus() {
			continue
		}
		result = append(result, toExpiryDecisionStudent(student))
	}
	return result, nil
}

func toExpiryDecisionStudent(student *usersModels.Student) enrollmentOwner.PhaseExpiryStudent {
	row := enrollmentOwner.PhaseExpiryStudent{
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

func (d expiryDecisionOfferings) ListCareOfferings(ctx context.Context) ([]enrollmentOwner.PhaseExpiryOffering, error) {
	values, err := d.query.ListCareOfferings(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderID})
	if err != nil {
		return nil, err
	}
	result := make([]enrollmentOwner.PhaseExpiryOffering, 0, len(values))
	for _, value := range values {
		result = append(result, enrollmentOwner.PhaseExpiryOffering{
			ID: value.ID, TenantID: value.TenantID, PhaseID: value.PhaseID,
			DaysOfWeekMode: value.DaysOfWeekMode, AvailableDays: value.AvailableDays, IsActive: value.IsActive,
		})
	}
	return result, nil
}
