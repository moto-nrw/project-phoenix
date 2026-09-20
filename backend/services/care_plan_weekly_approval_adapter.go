package services

import (
	"context"
	"errors"
	"fmt"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	userContextService "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/usercontext"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func (s *careScheduleRequestService) applyCareScheduleRequest(ctx context.Context, request *carerequests.Request, actorID int64) (bool, error) {
	if s.people == nil || s.arrival == nil || s.pickup == nil || s.userContext == nil {
		return false, errors.New("schedule: care request apply dependencies not configured")
	}
	a := &weeklyApprovalAdapter{s: s}
	service, err := compose.NewWeeklyApprovals(a, a, a)
	if err != nil {
		return false, err
	}
	return service.ApplyWeekly(ctx, request, actorID)
}

type weeklyApprovalAdapter struct {
	s        *careScheduleRequestService
	student  peopledirectory.StudentRecord
	before   peopledirectory.StudentRecord
	baseline peopledirectory.StudentPlan
}

func (*weeklyApprovalAdapter) TrackCompanionChanges(ctx context.Context) (context.Context, func() bool) {
	ctx, recorder := usersModels.ContextWithCompanionChangeRecorder(ctx)
	return ctx, recorder.Changed
}
func (a *weeklyApprovalAdapter) ActingStaffID(ctx context.Context) (int64, error) {
	staff, err := a.s.userContext.GetCurrentStaff(ctx)
	if err != nil {
		if errors.Is(err, userContextService.ErrUserNotLinkedToStaff) || errors.Is(err, userContextService.ErrUserNotLinkedToPerson) {
			return 0, carerequests.ErrCareRequestForbidden
		}
		return 0, fmt.Errorf("schedule: resolve acting staff: %w", err)
	}
	if staff == nil {
		return 0, carerequests.ErrCareRequestForbidden
	}
	return staff.ID, nil
}
func (a *weeklyApprovalAdapter) LockDepartureModes(ctx context.Context, id int64) (map[string][]string, error) {
	student, err := a.s.people.FindStudentRecordForMutation(ctx, id)
	if err != nil {
		return nil, requestStudentReadError(err)
	}
	a.student, a.before = student, student
	a.baseline = requestDeparturePlan(student)
	result := make(map[string][]string, len(a.baseline.AllowedDepartureModes))
	for day, modes := range a.baseline.AllowedDepartureModes {
		result[day] = nil
		for _, mode := range modes {
			result[day] = append(result[day], string(mode))
		}
	}
	return result, nil
}
func (a *weeklyApprovalAdapter) SaveDepartureModes(ctx context.Context, _ int64, modes map[string][]string) error {
	merged := usersModels.AllowedDepartureModes{}
	for day, values := range modes {
		merged[day] = nil
		for _, value := range values {
			merged[day] = append(merged[day], usersModels.DepartureMode(value))
		}
	}
	updated, err := a.s.people.UpdateStudent(ctx, peopledirectory.StudentWrite{
		Record:        a.student,
		Plan:          peopledirectory.StudentPlan{AllowedDepartureModes: merged, PickupStatus: a.student.PickupStatus},
		Baseline:      &a.baseline,
		CompanionNote: a.student.DepartureCompanionNote,
		NoteSupplied:  a.student.DepartureCompanionNote != nil,
	})
	if err != nil {
		return requestStudentWriteError(err)
	}
	a.student = updated
	return nil
}
func (a *weeklyApprovalAdapter) AuditDepartureModes(ctx context.Context, _, actorID int64) error {
	if a.s.studentAudit == nil {
		return nil
	}
	return a.s.studentAudit.RecordChangesForActor(ctx, requestStudentAuditSnapshot(a.before), requestStudentAuditSnapshot(a.student), actorID)
}
func (a *weeklyApprovalAdapter) GetStudentArrivalSchedules(ctx context.Context, id int64) ([]*careplan.ArrivalSchedule, error) {
	return a.s.arrival.GetStudentArrivalSchedules(ctx, id)
}
func (a *weeklyApprovalAdapter) DeleteStudentArrivalSchedule(ctx context.Context, id int64) error {
	return a.s.arrival.DeleteStudentArrivalSchedule(ctx, id)
}
func (a *weeklyApprovalAdapter) UpsertStudentArrivalSchedule(ctx context.Context, row *careplan.ArrivalSchedule) error {
	return a.s.arrival.UpsertStudentArrivalSchedule(ctx, row)
}
func (a *weeklyApprovalAdapter) GetStudentPickupSchedules(ctx context.Context, id int64) ([]*careplan.PickupSchedule, error) {
	return a.s.pickup.GetStudentPickupSchedules(ctx, id)
}
func (a *weeklyApprovalAdapter) DeleteStudentPickupSchedule(ctx context.Context, id int64) error {
	return a.s.pickup.DeleteStudentPickupSchedule(ctx, id)
}
func (a *weeklyApprovalAdapter) UpsertStudentPickupSchedule(ctx context.Context, row *careplan.PickupSchedule) error {
	return a.s.pickup.UpsertStudentPickupSchedule(ctx, row)
}
func (a *weeklyApprovalAdapter) HasBookedOfferingPickupForWeekday(ctx context.Context, id int64, day int) (bool, error) {
	return a.s.pickup.HasBookedOfferingPickupForWeekday(ctx, id, day)
}
