package schedule

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// Betreuungszeiten side of the conflict resolver (#2267, stories 6-10). One
// service owns both kinds the resolver knows here: the permanent weekly plan
// (conflict key care:<weekday>:<kind>) and the single-day pickup change
// (conflict key pickup:<date>). They resolve the same way — at most one wish
// wins, the rest are rejected — and differ only in where a staff-typed time
// gets written.

var _ usersService.ParentRequestConflictPort = (*careScheduleRequestService)(nil)

func (s *careScheduleRequestService) ConflictCandidate(
	ctx context.Context, requestID int64,
) (*usersService.ParentRequestConflictCandidate, error) {
	req, err := s.requestRepo.FindByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if req.Status != scheduleModels.CareRequestStatusPending {
		return nil, scheduleModels.ErrCareRequestNotPending
	}
	return &usersService.ParentRequestConflictCandidate{
		StudentID: req.StudentID,
		UpdatedAt: req.UpdatedAt,
	}, nil
}

func (s *careScheduleRequestService) LockConflictRequest(ctx context.Context, requestID int64) error {
	_, err := s.requestRepo.FindPendingByIDForUpdate(ctx, requestID)
	return err
}

func (s *careScheduleRequestService) DecideConflictRequest(
	ctx context.Context, decision usersService.ParentRequestConflictDecision,
) error {
	_, err := s.Decide(ctx, CareRequestDecideInput{
		RequestID:       decision.RequestID,
		Approve:         decision.Approve,
		Reason:          decision.Reason,
		ReviewedBy:      decision.ReviewerID,
		ExpectedVersion: decision.ExpectedVersion,
		// The resolver is an internal caller: it has already locked and
		// version-checked the whole group, so there is no staff-visible
		// impact token to pin.
		RequireImpactToken: false,
	})
	return err
}

// WriteStaffValue writes the time the staff member typed instead of any of the
// rejected wishes. Value is {"value": "HH:MM"}; WHERE it lands comes from the
// group, never from the client:
//
//   - pickup_change → the exception for the day the rejected requests named.
//   - weekly plan   → the weekday named by the group's conflict key.
func (s *careScheduleRequestService) WriteStaffValue(
	ctx context.Context, write usersService.ParentRequestStaffValueWrite,
) error {
	pickupTime, err := staffPickupTime(write.Value)
	if err != nil {
		return err
	}
	req, err := s.conflictGroupRequest(ctx, write.RequestIDs)
	if err != nil {
		return err
	}
	if req.RequestKind == scheduleModels.CareRequestKindPickupChange {
		return s.writeStaffPickupException(ctx, req, write, pickupTime)
	}
	return s.writeStaffWeeklyPickup(ctx, write, pickupTime)
}

// conflictGroupRequest loads the group's first request. Every request in a
// group is of one kind and one child, so it answers both questions the staff
// write still has.
func (s *careScheduleRequestService) conflictGroupRequest(
	ctx context.Context, requestIDs []int64,
) (*scheduleModels.CareScheduleChangeRequest, error) {
	if len(requestIDs) == 0 {
		return nil, usersService.ErrStaffValueUnsupported
	}
	req, err := s.requestRepo.FindByID(ctx, requestIDs[0])
	if err != nil {
		return nil, fmt.Errorf("schedule: load conflict group request: %w", err)
	}
	if req == nil {
		return nil, scheduleModels.ErrCareRequestNotFound
	}
	return req, nil
}

// writeStaffPickupException reuses the very write an approval performs, so a
// staff-decided time is indistinguishable from an approved one afterwards —
// same source, same auto-excusal sync, same conflict guards.
func (s *careScheduleRequestService) writeStaffPickupException(
	ctx context.Context,
	req *scheduleModels.CareScheduleChangeRequest,
	write usersService.ParentRequestStaffValueWrite,
	pickupTime time.Time,
) error {
	if s.pickupExceptions == nil || s.pickupAutoExcusal == nil || s.userContext == nil {
		return errors.New("schedule: pickup change dependencies not configured")
	}
	date, _, _, err := parsePickupChangePayload(req.Payload)
	if err != nil {
		return err
	}
	if date.Before(timezone.TodayDate()) {
		return ErrPickupChangeExpired
	}
	if err := LockCareExceptionDay(ctx, s.pickupAutoExcusal.db, req.StudentID, date); err != nil {
		return fmt.Errorf("schedule: lock staff pickup day: %w", err)
	}
	staff, err := s.resolvePickupChangeStaff(ctx)
	if err != nil {
		return err
	}
	exceptionID, err := s.saveApprovedPickupException(ctx, req, date, pickupTime, write.Reason, staff.ID)
	if err != nil {
		return err
	}
	if _, err := s.pickupAutoExcusal.Sync(ctx, exceptionID); err != nil {
		return fmt.Errorf("schedule: sync staff pickup exception: %w", err)
	}
	return nil
}

// writeStaffWeeklyPickup sets one weekday of the child's permanent plan. Only
// that weekday is touched — the rest of the plan is none of this conflict's
// business.
func (s *careScheduleRequestService) writeStaffWeeklyPickup(
	ctx context.Context, write usersService.ParentRequestStaffValueWrite, pickupTime time.Time,
) error {
	if s.pickup == nil {
		return errors.New("schedule: pickup schedule service not configured")
	}
	weekday, err := conflictKeyWeekday(write.ConflictKey)
	if err != nil {
		return err
	}
	staff, err := s.resolvePickupChangeStaff(ctx)
	if err != nil {
		return err
	}
	reason := write.Reason
	return s.pickup.UpsertStudentPickupSchedule(ctx, &scheduleModels.StudentPickupSchedule{
		StudentID:  write.StudentID,
		Weekday:    weekday,
		PickupTime: timezone.NormalizeWallClock(pickupTime),
		Notes:      &reason,
		CreatedBy:  staff.ID,
	})
}

// conflictKeyWeekday reads the ISO weekday out of a care:<weekday>:<kind> key.
// A key the derivation never produces is refused rather than guessed — writing
// the wrong weekday of a permanent plan is not something a staff member would
// notice.
func conflictKeyWeekday(conflictKey string) (int, error) {
	parts := strings.Split(conflictKey, ":")
	if len(parts) != 3 || parts[0] != "care" {
		return 0, usersService.ErrStaffValueUnsupported
	}
	weekday, err := strconv.Atoi(parts[1])
	if err != nil || weekday < 1 || weekday > 7 {
		return 0, usersService.ErrStaffValueUnsupported
	}
	return weekday, nil
}

func staffPickupTime(value map[string]any) (time.Time, error) {
	raw, ok := value["value"].(string)
	if !ok {
		return time.Time{}, ErrInvalidCareRequestPayload
	}
	pickup, err := parseCareWallClock(strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, ErrInvalidCareRequestPayload
	}
	return pickup, nil
}
