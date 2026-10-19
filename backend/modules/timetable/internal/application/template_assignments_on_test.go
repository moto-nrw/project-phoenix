package application

import (
	"context"
	"errors"
	"testing"
	"time"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errConversionMock = errors.New("conversion mock failure")

type conversionEnrollmentRepo struct {
	ports.ActivityEnrollmentRecords
	rows []*activityContract.StudentEnrollment
	err  error
}

func (r *conversionEnrollmentRepo) FindByGroupID(context.Context, int64) ([]*activityContract.StudentEnrollment, error) {
	return r.rows, r.err
}

type conversionSupervisorRepo struct {
	ports.PlannedSupervisorRecords
	rows []*activityContract.SupervisorPlanned
	err  error
}

func (r *conversionSupervisorRepo) FindByGroupID(context.Context, int64) ([]*activityContract.SupervisorPlanned, error) {
	return r.rows, r.err
}

// Implements both GroupRepository and GroupTargetRepository so the type
// assertion in templateAssignmentsOn can exercise the target-student branch.
type conversionGroupTargetRepo struct {
	ports.ActivityGroupRecords
	targetIDs []int64
	err       error
}

func (r *conversionGroupTargetRepo) FindTargetStudentIDs(context.Context, int64) ([]int64, error) {
	return r.targetIDs, r.err
}

func (r *conversionGroupTargetRepo) FindTargetsByGroupIDs(context.Context, []int64) (map[int64][]*activityContract.GroupTarget, error) {
	return nil, nil
}

func (r *conversionGroupTargetRepo) ReplaceTargets(context.Context, int64, []*activityContract.GroupTarget) error {
	return nil
}

func (r *conversionGroupTargetRepo) FindTargetStudentIDsByGroupIDs(context.Context, []int64) (map[int64][]int64, error) {
	return nil, nil
}

type conversionPlainGroupRepo struct {
	ports.ActivityGroupRecords
}

func conversionValidFrom() calendar.Date {
	return calendar.NewDate(2026, time.May, 4)
}

func conversionEnrollment(studentID int64, from calendar.Date) *activityContract.StudentEnrollment {
	return &activityContract.StudentEnrollment{
		StudentID: studentID,
		ValidFrom: activityContract.Date(from),
	}
}

func conversionSupervisor(staffID int64, from calendar.Date) *activityContract.SupervisorPlanned {
	return &activityContract.SupervisorPlanned{
		StaffID:   staffID,
		ValidFrom: activityContract.Date(from),
	}
}

func TestTemplateAssignmentsOn_NilDependencies(t *testing.T) {
	t.Parallel()

	svc := NewTemplateService(TemplateServiceDependencies{})
	assignments, err := svc.TemplateAssignmentsOn(context.Background(), 1, conversionValidFrom(), 11)
	require.Error(t, err)
	assert.Nil(t, assignments.StudentIDs)
	assert.Nil(t, assignments.StaffIDs)
	var scheduleErr *ports.ScheduleError
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
	_, err := svc.TemplateAssignmentsOn(context.Background(), 1, conversionValidFrom(), 11)
	require.Error(t, err)
	assert.ErrorIs(t, err, errConversionMock)
	assert.Contains(t, err.Error(), "load enrollments")
}

func TestTemplateAssignmentsOn_SupervisorLoadError(t *testing.T) {
	t.Parallel()

	from := conversionValidFrom()
	svc := NewTemplateService(TemplateServiceDependencies{
		StudentEnrollmentRepo:  &conversionEnrollmentRepo{rows: []*activityContract.StudentEnrollment{}},
		ActivitySupervisorRepo: &conversionSupervisorRepo{err: errConversionMock},
		ActivityGroupRepo:      &conversionPlainGroupRepo{},
	})
	_, err := svc.TemplateAssignmentsOn(context.Background(), 1, from, 11)
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
	_, err := svc.TemplateAssignmentsOn(context.Background(), 1, from, 11)
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

	enrollments := []*activityContract.StudentEnrollment{
		conversionEnrollment(21, from),
		conversionEnrollment(21, from),   // duplicate student id
		conversionEnrollment(22, future), // not yet valid
		{
			StudentID:        23,
			ValidFrom:        activityContract.Date(from),
			CalendarPeriodID: &wrongPeriod,
		},
		conversionEnrollment(24, from),
	}
	supervisors := []*activityContract.SupervisorPlanned{
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

	assignments, err := svc.TemplateAssignmentsOn(context.Background(), 7, from, periodID)
	require.NoError(t, err)
	assert.Equal(t, []int64{21, 24, 25}, assignments.StudentIDs)
	assert.Equal(t, []int64{31, 33}, assignments.StaffIDs)
}

func TestTemplateAssignmentsOn_SkipsTargetBranchWithoutTargetRepo(t *testing.T) {
	t.Parallel()

	from := conversionValidFrom()
	svc := NewTemplateService(TemplateServiceDependencies{
		StudentEnrollmentRepo: &conversionEnrollmentRepo{
			rows: []*activityContract.StudentEnrollment{conversionEnrollment(41, from)},
		},
		ActivitySupervisorRepo: &conversionSupervisorRepo{
			rows: []*activityContract.SupervisorPlanned{conversionSupervisor(51, from)},
		},
		ActivityGroupRepo: &conversionPlainGroupRepo{},
	})

	assignments, err := svc.TemplateAssignmentsOn(context.Background(), 8, from, 11)
	require.NoError(t, err)
	assert.Equal(t, []int64{41}, assignments.StudentIDs)
	assert.Equal(t, []int64{51}, assignments.StaffIDs)
}

// Compile-time guard: conversionGroupTargetRepo must satisfy the target interface
// the conversion roster path type-asserts against.
var _ ports.ActivityGroupTargets = (*conversionGroupTargetRepo)(nil)
