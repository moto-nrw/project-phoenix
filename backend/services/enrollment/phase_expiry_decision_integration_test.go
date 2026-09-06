package enrollment_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
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
	require.NoError(t, env.repos.CareOffering.Create(ctx, sourceOffering))

	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Phasenende",
		GuardianEmail:     "phase-expiry-decision@example.com",
		ConsentFlags:      rolloverConsentFlags(),
		Children: []enrollmentService.SubmitChild{{
			FirstName:        "Lina",
			LastName:         "Phasenende",
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: testpkg.Int16Ptr(2),
			OfferingIDs:      []int64{sourceOffering.ID},
		}},
	})
	require.NoError(t, err)
	require.Len(t, submitted.Children, 1)

	sourceOutcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  submitted.Request.ID,
		ChildID:    submitted.Children[0].ID,
		Status:     enrollmentService.DecisionApproved,
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
	targetOfferings, err := env.repos.CareOffering.ListByPhase(ctx, rollover.Phase.ID)
	require.NoError(t, err)
	require.Len(t, targetOfferings, 1)
	targetOfferings[0].IsActive = false
	require.NoError(t, env.repos.CareOffering.Update(ctx, targetOfferings[0]))

	scheduledDecision := newDecisionServiceForTest(env.rolloverTestEnv, nil, nil)
	targetOutcome, err := scheduledDecision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  rolledChildren[0].RequestID,
		ChildID:    rolledChildren[0].ID,
		Status:     enrollmentService.DecisionApproved,
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

	warnings, err := enrollmentService.NewPhaseExpiryService(enrollmentService.NewPhaseExpiryProjection(env.repos.Enrollment(), expiryDecisionStudents{env.repos.Student}, expiryDecisionOfferings{env.repos.CarePlan()})).
		ListWarnings(ctx, timezone.NewDate(2027, 7, 3))
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Equal(t, enrollmentService.PhaseExpiryStateIncomplete, warnings[0].State)
	assert.Equal(t, 1, warnings[0].UnresolvedChildren)

	targetOfferings[0].IsActive = true
	require.NoError(t, env.repos.CareOffering.Update(ctx, targetOfferings[0]))
	warnings, err = enrollmentService.NewPhaseExpiryService(enrollmentService.NewPhaseExpiryProjection(env.repos.Enrollment(), expiryDecisionStudents{env.repos.Student}, expiryDecisionOfferings{env.repos.CarePlan()})).
		ListWarnings(ctx, timezone.NewDate(2027, 7, 3))
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

type expiryDecisionStudents struct{ students usersModels.StudentRepository }

func (d expiryDecisionStudents) ListEnrolledStudents(ctx context.Context) ([]enrollmentService.PhaseExpiryStudent, error) {
	students, err := d.students.List(ctx, map[string]any{})
	if err != nil {
		return nil, err
	}
	result := make([]enrollmentService.PhaseExpiryStudent, 0, len(students))
	for _, student := range students {
		if student.IsAlumnus() {
			continue
		}
		result = append(result, toExpiryDecisionStudent(student))
	}
	return result, nil
}

func toExpiryDecisionStudent(student *usersModels.Student) enrollmentService.PhaseExpiryStudent {
	row := enrollmentService.PhaseExpiryStudent{
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

func (d expiryDecisionOfferings) ListCareOfferings(ctx context.Context) ([]enrollmentService.PhaseExpiryOffering, error) {
	values, err := d.query.ListCareOfferings(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderID})
	if err != nil {
		return nil, err
	}
	result := make([]enrollmentService.PhaseExpiryOffering, 0, len(values))
	for _, value := range values {
		result = append(result, enrollmentService.PhaseExpiryOffering{
			ID: value.ID, TenantID: value.TenantID, PhaseID: value.PhaseID,
			DaysOfWeekMode: value.DaysOfWeekMode, AvailableDays: value.AvailableDays, IsActive: value.IsActive,
		})
	}
	return result, nil
}
