package services

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
)

// The coordinator's care port only translates values and errors. Care Plan
// owns pending checks, conflict-key parsing, day locks, and staff writes.
var _ carePlanCompose.ParentRequestConflicts = (*careScheduleRequestService)(nil)

func (s *careScheduleRequestService) ConflictCandidate(ctx context.Context, id int64) (*carePlanCompose.ConflictCandidate, error) {
	row, err := s.requests.conflicts.ConflictCandidate(ctx, id)
	if err != nil {
		return nil, legacyConflictError(err)
	}
	return &carePlanCompose.ConflictCandidate{StudentID: row.StudentID, UpdatedAt: row.UpdatedAt}, nil
}

func (s *careScheduleRequestService) LockConflictRequest(ctx context.Context, id int64) error {
	return legacyConflictError(s.requests.conflicts.LockConflictRequest(ctx, id))
}

func (s *careScheduleRequestService) DecideConflictRequest(ctx context.Context, decision carePlanCompose.ConflictDecision) error {
	return legacyConflictError(s.requests.conflicts.DecideConflictRequest(ctx, carerequests.DecideInput{
		RequestID: decision.RequestID, Approve: decision.Approve, Reason: decision.Reason,
		ReviewedBy: decision.ReviewerID, ExpectedVersion: decision.ExpectedVersion,
	}))
}

func (s *careScheduleRequestService) WriteStaffValue(ctx context.Context, write carePlanCompose.StaffValueWrite) error {
	pickup, ok := write.Value["value"].(string)
	if !ok {
		return carerequests.ErrInvalidPayload
	}
	return legacyConflictError(s.requests.conflicts.WriteStaffValue(ctx, carerequests.StaffValueWrite{
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

type requestConflictPlans struct{ s *careScheduleRequestService }

func (a requestConflictPlans) LockStudentAndExceptionDay(ctx context.Context, studentID int64, date string) error {
	if a.s.requestRecords == nil || a.s.pickupAutoExcusal == nil || a.s.userContext == nil || a.s.dayLocker == nil {
		return errors.New("schedule: pickup change dependencies not configured")
	}
	return a.s.dayLocker.LockStudentAndExceptionDay(ctx, studentID, date)
}
func (a requestConflictPlans) ActingStaffID(ctx context.Context) (int64, error) {
	if a.s.userContext == nil {
		return 0, errors.New("schedule: pickup change dependencies not configured")
	}
	staffID, err := a.s.resolvePickupChangeStaff(ctx)
	if err != nil {
		return 0, err
	}
	return staffID, nil
}
func (a requestConflictPlans) SaveApprovedException(ctx context.Context, tenantID, studentID int64, date timezone.Date, pickup time.Time, reason string, staffID int64) (int64, error) {
	approvals, err := a.s.pickupApprovals()
	if err != nil {
		return 0, err
	}
	return approvals.SaveApprovedException(ctx, tenantID, studentID, date, pickup, reason, staffID)
}
func (a requestConflictPlans) Sync(ctx context.Context, id int64) error {
	_, err := a.s.pickupAutoExcusal.Sync(ctx, id)
	return err
}
func (a requestConflictPlans) UpsertStudentPickupSchedule(ctx context.Context, row *careplan.PickupSchedule) error {
	if a.s.pickup == nil {
		return errors.New("schedule: pickup schedule service not configured")
	}
	return a.s.pickup.UpsertStudentPickupSchedule(ctx, row)
}
