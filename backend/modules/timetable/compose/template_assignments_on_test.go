package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errConversionMock = errors.New("conversion mock failure")

type conversionEnrollmentRepo struct {
	activitiesModel.StudentEnrollmentRepository
	rows []*activitiesModel.StudentEnrollment
	err  error
}

func (r *conversionEnrollmentRepo) FindByGroupID(context.Context, int64) ([]*activitiesModel.StudentEnrollment, error) {
	return r.rows, r.err
}

type conversionSupervisorRepo struct {
	activitiesModel.SupervisorPlannedRepository
	rows []*activitiesModel.SupervisorPlanned
	err  error
}

func (r *conversionSupervisorRepo) FindByGroupID(context.Context, int64) ([]*activitiesModel.SupervisorPlanned, error) {
	return r.rows, r.err
}

// Implements both GroupRepository and GroupTargetRepository so the type
// assertion in templateAssignmentsOn can exercise the target-student branch.
type conversionGroupTargetRepo struct {
	activitiesModel.GroupRepository
	targetIDs []int64
	err       error
}

func (r *conversionGroupTargetRepo) FindTargetStudentIDs(context.Context, int64) ([]int64, error) {
	return r.targetIDs, r.err
}

func (r *conversionGroupTargetRepo) FindTargetsByGroupIDs(context.Context, []int64) (map[int64][]*activitiesModel.GroupTarget, error) {
	return nil, nil
}

func (r *conversionGroupTargetRepo) ReplaceTargets(context.Context, int64, []*activitiesModel.GroupTarget) error {
	return nil
}

func (r *conversionGroupTargetRepo) FindTargetStudentIDsByGroupIDs(context.Context, []int64) (map[int64][]int64, error) {
	return nil, nil
}

type conversionPlainGroupRepo struct {
	activitiesModel.GroupRepository
}

func conversionValidFrom() timezone.Date {
	return timezone.NewDate(2026, time.May, 4)
}

func conversionEnrollment(studentID int64, from timezone.Date) *activitiesModel.StudentEnrollment {
	return &activitiesModel.StudentEnrollment{
		StudentID: studentID,
		ValidFrom: activitiesModel.Date(from),
	}
}

func conversionSupervisor(staffID int64, from timezone.Date) *activitiesModel.SupervisorPlanned {
	return &activitiesModel.SupervisorPlanned{
		StaffID:   staffID,
		ValidFrom: activitiesModel.Date(from),
	}
}

func TestTemplateAssignmentsOn_NilDependencies(t *testing.T) {
	t.Parallel()

	svc := NewTemplateService(TemplateServiceDependencies{})
	assignments, err := svc.templateAssignmentsOn(context.Background(), 1, conversionValidFrom(), 11)
	require.Error(t, err)
	assert.Nil(t, assignments.StudentIDs)
	assert.Nil(t, assignments.StaffIDs)
	var scheduleErr *ScheduleError
	require.ErrorAs(t, err, &scheduleErr)
	assert.Contains(t, scheduleErr.Op, "validate dependencies")
}

func TestTemplateAssignmentsOn_EnrollmentLoadError(t *testing.T) {
	t.Parallel()

	svc := NewTemplateService(TemplateServiceDependencies{
		StudentEnrollmentRepo:  &conversionEnrollmentRepo{err: errConversionMock},
		ActivitySupervisorRepo: &conversionSupervisorRepo{},
		ActivityGroupRepo:      &conversionPlainGroupRepo{},
	})
	_, err := svc.templateAssignmentsOn(context.Background(), 1, conversionValidFrom(), 11)
	require.Error(t, err)
	assert.ErrorIs(t, err, errConversionMock)
	assert.Contains(t, err.Error(), "load enrollments")
}

func TestTemplateAssignmentsOn_SupervisorLoadError(t *testing.T) {
	t.Parallel()

	from := conversionValidFrom()
	svc := NewTemplateService(TemplateServiceDependencies{
		StudentEnrollmentRepo:  &conversionEnrollmentRepo{rows: []*activitiesModel.StudentEnrollment{}},
		ActivitySupervisorRepo: &conversionSupervisorRepo{err: errConversionMock},
		ActivityGroupRepo:      &conversionPlainGroupRepo{},
	})
	_, err := svc.templateAssignmentsOn(context.Background(), 1, from, 11)
	require.Error(t, err)
	assert.ErrorIs(t, err, errConversionMock)
	assert.Contains(t, err.Error(), "load supervisors")
}

func TestTemplateAssignmentsOn_TargetStudentLoadError(t *testing.T) {
	t.Parallel()

	from := conversionValidFrom()
	svc := NewTemplateService(TemplateServiceDependencies{
		StudentEnrollmentRepo:  &conversionEnrollmentRepo{},
		ActivitySupervisorRepo: &conversionSupervisorRepo{},
		ActivityGroupRepo:      &conversionGroupTargetRepo{err: errConversionMock},
	})
	_, err := svc.templateAssignmentsOn(context.Background(), 1, from, 11)
	require.Error(t, err)
	assert.ErrorIs(t, err, errConversionMock)
	assert.Contains(t, err.Error(), "load target students")
}

func TestTemplateAssignmentsOn_FiltersDuplicatesTargetsAndInvalidRows(t *testing.T) {
	t.Parallel()

	from := conversionValidFrom()
	// Monday 2026-05-04 — keep weekdays unrestricted so validity is pure date.
	future := from.AddDays(14)
	wrongPeriod := int64(99)
	periodID := int64(11)

	enrollments := []*activitiesModel.StudentEnrollment{
		conversionEnrollment(21, from),
		conversionEnrollment(21, from),   // duplicate student id
		conversionEnrollment(22, future), // not yet valid
		{
			StudentID:        23,
			ValidFrom:        activitiesModel.Date(from),
			CalendarPeriodID: &wrongPeriod,
		},
		conversionEnrollment(24, from),
	}
	supervisors := []*activitiesModel.SupervisorPlanned{
		conversionSupervisor(31, from),
		conversionSupervisor(31, from),   // duplicate staff id
		conversionSupervisor(32, future), // not yet valid
		conversionSupervisor(33, from),
	}

	svc := NewTemplateService(TemplateServiceDependencies{
		StudentEnrollmentRepo:  &conversionEnrollmentRepo{rows: enrollments},
		ActivitySupervisorRepo: &conversionSupervisorRepo{rows: supervisors},
		ActivityGroupRepo: &conversionGroupTargetRepo{
			// 0 is ignored, 21 already seen from enrollments, 25 is new.
			targetIDs: []int64{0, 21, 25},
		},
	})

	assignments, err := svc.templateAssignmentsOn(context.Background(), 7, from, periodID)
	require.NoError(t, err)
	assert.Equal(t, []int64{21, 24, 25}, assignments.StudentIDs)
	assert.Equal(t, []int64{31, 33}, assignments.StaffIDs)
}

func TestTemplateAssignmentsOn_SkipsTargetBranchWithoutTargetRepo(t *testing.T) {
	t.Parallel()

	from := conversionValidFrom()
	svc := NewTemplateService(TemplateServiceDependencies{
		StudentEnrollmentRepo: &conversionEnrollmentRepo{
			rows: []*activitiesModel.StudentEnrollment{conversionEnrollment(41, from)},
		},
		ActivitySupervisorRepo: &conversionSupervisorRepo{
			rows: []*activitiesModel.SupervisorPlanned{conversionSupervisor(51, from)},
		},
		ActivityGroupRepo: &conversionPlainGroupRepo{},
	})

	assignments, err := svc.templateAssignmentsOn(context.Background(), 8, from, 11)
	require.NoError(t, err)
	assert.Equal(t, []int64{41}, assignments.StudentIDs)
	assert.Equal(t, []int64{51}, assignments.StaffIDs)
}

// Compile-time guard: conversionGroupTargetRepo must satisfy the target interface
// the conversion roster path type-asserts against.
var _ activitiesModel.GroupTargetRepository = (*conversionGroupTargetRepo)(nil)
