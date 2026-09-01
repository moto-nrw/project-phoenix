package absence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// Abwesenheiten side of the conflict resolver (#2267, stories 6-10). Two open
// requests for the same day contradict each other — one asks for "krank", the
// other for "entschuldigt" — and deciding them one after the other leaves the
// day carrying whichever was decided last. The resolver closes the whole day
// at once; this file only supplies the payload.

var _ usersService.ParentRequestConflictPort = (*excusedAbsenceRequestService)(nil)

func (s *excusedAbsenceRequestService) ConflictCandidate(
	ctx context.Context, requestID int64,
) (*usersService.ParentRequestConflictCandidate, error) {
	req, err := s.requestRepo.FindByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if req.Status != activeModels.ExcusedRequestStatusPending {
		return nil, activeModels.ErrExcusedRequestNotPending
	}
	return &usersService.ParentRequestConflictCandidate{
		StudentID: req.StudentID,
		UpdatedAt: req.UpdatedAt,
	}, nil
}

func (s *excusedAbsenceRequestService) LockConflictRequest(ctx context.Context, requestID int64) error {
	return s.LockExcusedBulkRequest(ctx, requestID)
}

func (s *excusedAbsenceRequestService) DecideConflictRequest(
	ctx context.Context, decision usersService.ParentRequestConflictDecision,
) error {
	_, err := s.Decide(ctx, ExcusedRequestDecideInput{
		RequestID:       decision.RequestID,
		Approve:         decision.Approve,
		Reason:          decision.Reason,
		ReviewedBy:      decision.ReviewerID,
		ExpectedVersion: decision.ExpectedVersion,
	})
	return err
}

// WriteStaffValue records the staff member's own verdict for exactly the days
// the rejected requests fought over, as a manual entry — the same thing the
// staff member would get by setting the status on the child's own screen.
//
// The days come from the requests, never from the client: the resolve may only
// write the days the conflict was actually about. Value is
// {"value": "<status>"} with one of present, sick, excused or class_trip;
// "present" clears the day instead of writing a status.
func (s *excusedAbsenceRequestService) WriteStaffValue(
	ctx context.Context, write usersService.ParentRequestStaffValueWrite,
) error {
	status, err := staffAbsenceStatus(write.Value)
	if err != nil {
		return err
	}
	dates, err := s.conflictGroupDates(ctx, write.RequestIDs)
	if err != nil {
		return err
	}
	if len(dates) == 0 {
		return usersService.ErrStaffValueUnsupported
	}
	now := time.Now()
	// Clear every status the staff verdict is NOT, then write the one it is.
	// Clearing first keeps a day from carrying two active statuses for the
	// instant between the two writes.
	for _, other := range activeModels.StudentStatusDayStatusesExcept(status) {
		if err := s.statusDayRepo.MarkClearedForDates(
			ctx, write.StudentID, other, dates, now, activeModels.StudentStatusSourceManual,
		); err != nil {
			return fmt.Errorf("active: clear status days for staff value: %w", err)
		}
	}
	if status == activeModels.StudentStatusDayPresent {
		return nil
	}
	reason := write.Reason
	for _, date := range dates {
		if err := s.statusDayRepo.UpsertReported(ctx, &activeModels.StudentStatusDay{
			StudentID:  write.StudentID,
			Date:       date,
			Status:     status,
			ReportedAt: now,
			Source:     activeModels.StudentStatusSourceManual,
			Note:       &reason,
		}); err != nil {
			return fmt.Errorf("active: write status day for staff value: %w", err)
		}
	}
	return nil
}

// conflictGroupDates is the union of the days the group's requests covered,
// deduplicated and sorted so the writes below run in a stable order.
func (s *excusedAbsenceRequestService) conflictGroupDates(
	ctx context.Context, requestIDs []int64,
) ([]timezone.Date, error) {
	var dates []timezone.Date
	for _, requestID := range requestIDs {
		req, err := s.requestRepo.FindByID(ctx, requestID)
		if err != nil {
			if errors.Is(err, activeModels.ErrExcusedRequestNotFound) {
				continue
			}
			return nil, fmt.Errorf("active: load conflict group dates: %w", err)
		}
		dates = append(dates, req.Dates...)
	}
	return dedupeSortedDates(dates), nil
}

func staffAbsenceStatus(value map[string]any) (string, error) {
	raw, ok := value["value"].(string)
	if !ok {
		return "", ErrAbsenceRequestInvalidStatus
	}
	switch raw {
	case activeModels.StudentStatusDayPresent,
		activeModels.StudentStatusDaySick,
		activeModels.StudentStatusDayExcused,
		activeModels.StudentStatusDayClassTrip:
		return raw, nil
	default:
		return "", ErrAbsenceRequestInvalidStatus
	}
}
