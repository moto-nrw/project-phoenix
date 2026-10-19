package owners

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *CareScheduleRequestService) ConflictCandidate(ctx context.Context, id int64) (*carePlanCompose.ConflictCandidate, error) {
	row, err := s.Requests.conflicts.ConflictCandidate(ctx, id)
	if err != nil {
		return nil, legacyConflictError(err)
	}
	return &carePlanCompose.ConflictCandidate{StudentID: row.StudentID, UpdatedAt: row.UpdatedAt}, nil
}

func (s *CareScheduleRequestService) LockConflictRequest(ctx context.Context, id int64) error {
	return legacyConflictError(s.Requests.conflicts.LockConflictRequest(ctx, id))
}

func (s *CareScheduleRequestService) DecideConflictRequest(ctx context.Context, decision carePlanCompose.ConflictDecision) error {
	return legacyConflictError(s.Requests.conflicts.DecideConflictRequest(ctx, carerequests.DecideInput{
		RequestID: decision.RequestID, Approve: decision.Approve, Reason: decision.Reason,
		ReviewedBy: decision.ReviewerID, ExpectedVersion: decision.ExpectedVersion,
	}))
}

func (s *CareScheduleRequestService) WriteStaffValue(ctx context.Context, write carePlanCompose.StaffValueWrite) error {
	pickup, err := carerequests.StaffPickupValue(write.Value["value"])
	if err != nil {
		return err
	}
	return legacyConflictError(s.Requests.conflicts.WriteStaffValue(ctx, carerequests.StaffValueWrite{
		StudentID: write.StudentID, RequestIDs: write.RequestIDs, ConflictKey: write.ConflictKey,
		Reason: write.Reason, PickupTime: pickup,
	}))
}

func legacyConflictError(err error) error {
	if errors.Is(err, carerequests.ErrStaffValueUnsupported) {
		return parentrequests.ErrStaffValueUnsupported
	}
	return legacyEditError(err)
}

type requestConflictPlans struct{ s *CareScheduleRequestService }

func (a requestConflictPlans) LockStudentAndExceptionDay(ctx context.Context, studentID int64, date string) error {
	if a.s.RequestRecords == nil || a.s.PickupAutoExcusal == nil || a.s.UserContext == nil || a.s.DayLocker == nil {
		return errors.New("schedule: pickup change dependencies not configured")
	}
	return a.s.DayLocker.LockStudentAndExceptionDay(ctx, studentID, date)
}

func (a requestConflictPlans) CurrentStaffID(ctx context.Context) (int64, bool, error) {
	if a.s.UserContext == nil {
		return 0, false, errors.New("schedule: pickup change dependencies not configured")
	}
	return a.s.UserContext.CurrentStaffID(ctx)
}

func (a requestConflictPlans) SaveApprovedException(ctx context.Context, tenantID, studentID int64, date timezone.Date, pickup time.Time, reason string, staffID int64) (int64, error) {
	approvals, err := a.s.pickupApprovals()
	if err != nil {
		return 0, err
	}
	return approvals.SaveApprovedException(ctx, tenantID, studentID, date, pickup, reason, staffID)
}

func (a requestConflictPlans) Sync(ctx context.Context, id int64) error {
	_, err := a.s.PickupAutoExcusal.Sync(ctx, id)
	return err
}

func (a requestConflictPlans) UpsertStudentPickupSchedule(ctx context.Context, row *careplan.PickupSchedule) error {
	if a.s.Pickup == nil {
		return errors.New("schedule: pickup schedule service not configured")
	}
	return a.s.Pickup.UpsertStudentPickupSchedule(ctx, row)
}
