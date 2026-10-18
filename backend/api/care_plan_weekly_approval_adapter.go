package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func (s *careScheduleRequestService) applyCareScheduleRequest(ctx context.Context, request *carerequests.Request, actorID int64) (bool, error) {
	if s.people == nil || s.arrival == nil || s.pickup == nil || s.userContext == nil {
		return false, errors.New("schedule: care request apply dependencies not configured")
	}
	a := &weeklyApprovalAdapter{
		people: s.people, arrival: s.arrival, pickup: s.pickup, staff: s.userContext, audit: s.studentAudit,
	}
	service, err := compose.NewWeeklyApprovals(a, a, a)
	if err != nil {
		return false, err
	}
	return service.ApplyWeekly(ctx, request, actorID)
}

// weeklyApprovalAdapter applies one weekly request. It holds the request
// service's collaborators it calls and the student row the approval locked.
type weeklyApprovalAdapter struct {
	people   RequestPeople
	arrival  RequestArrivalPlans
	pickup   RequestPickupPlans
	staff    CareRequestStaff
	audit    requestStudentAudit
	student  peopledirectory.StudentRecord
	before   peopledirectory.StudentRecord
	baseline peopledirectory.StudentPlan
}

func (*weeklyApprovalAdapter) TrackCompanionChanges(ctx context.Context) (context.Context, func() bool) {
	ctx, recorder := peopledirectory.ContextWithCompanionChangeRecorder(ctx)
	return ctx, recorder.Changed
}
func (a *weeklyApprovalAdapter) CurrentStaffID(ctx context.Context) (int64, bool, error) {
	return a.staff.CurrentStaffID(ctx)
}
func (a *weeklyApprovalAdapter) LockDepartureModes(ctx context.Context, id int64) (map[string][]string, error) {
	student, err := a.people.FindStudentRecordForMutation(ctx, id)
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
func (a *weeklyApprovalAdapter) SaveDepartureModes(ctx context.Context, id int64, modes map[string][]string) error {
	// The write builds on the row LockDepartureModes loaded; without it the
	// update would persist an empty student record.
	if a.student.ID == 0 || a.student.ID != id {
		return fmt.Errorf("schedule: departure modes not locked for student %d", id)
	}
	merged := peopledirectory.AllowedDepartureModes{}
	for day, values := range modes {
		merged[day] = nil
		for _, value := range values {
			merged[day] = append(merged[day], peopledirectory.DepartureMode(value))
		}
	}
	updated, err := a.people.UpdateStudent(ctx, peopledirectory.StudentWrite{
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
	if a.audit == nil {
		return nil
	}
	return a.audit.RecordChangesForActor(ctx, requestStudentAuditSnapshot(a.before), requestStudentAuditSnapshot(a.student), actorID)
}
func (a *weeklyApprovalAdapter) GetStudentArrivalSchedules(ctx context.Context, id int64) ([]*careplan.ArrivalSchedule, error) {
	return a.arrival.GetStudentArrivalSchedules(ctx, id)
}
func (a *weeklyApprovalAdapter) DeleteStudentArrivalSchedule(ctx context.Context, id int64) error {
	return a.arrival.DeleteStudentArrivalSchedule(ctx, id)
}
func (a *weeklyApprovalAdapter) UpsertStudentArrivalSchedule(ctx context.Context, row *careplan.ArrivalSchedule) error {
	return a.arrival.UpsertStudentArrivalSchedule(ctx, row)
}
func (a *weeklyApprovalAdapter) GetStudentPickupSchedules(ctx context.Context, id int64) ([]*careplan.PickupSchedule, error) {
	return a.pickup.GetStudentPickupSchedules(ctx, id)
}
func (a *weeklyApprovalAdapter) DeleteStudentPickupSchedule(ctx context.Context, id int64) error {
	return a.pickup.DeleteStudentPickupSchedule(ctx, id)
}
func (a *weeklyApprovalAdapter) UpsertStudentPickupSchedule(ctx context.Context, row *careplan.PickupSchedule) error {
	return a.pickup.UpsertStudentPickupSchedule(ctx, row)
}
func (a *weeklyApprovalAdapter) HasBookedOfferingPickupForWeekday(ctx context.Context, id int64, day int) (bool, error) {
	return a.pickup.HasBookedOfferingPickupForWeekday(ctx, id, day)
}
