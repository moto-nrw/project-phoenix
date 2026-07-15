package schedule

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

// MoveShiftInput is the complete desired slot for one concrete shift. The
// source owner is an optimistic-concurrency guard: a retry after a successful
// cross-person move returns the already-moved row, while a genuinely stale
// request is rejected instead of overwriting a later move.
type MoveShiftInput struct {
	ShiftID       int64
	SourceStaffID int64
	TargetStaffID int64
	Date          timezone.Date
	StartTime     time.Time
	EndTime       time.Time
	BreakMinutes  int
	ShiftTypeID   *int64
	ActorStaffID  int64
}

// MoveShift applies a move as one row update inside the caller's tenant
// transaction. Keeping the row ID makes PUT retries naturally idempotent and
// avoids the duplicate/lost-shift states of client-side create/delete.
func (s *staffShiftService) MoveShift(ctx context.Context, input MoveShiftInput) (*scheduleModels.StaffShift, error) {
	if input.ShiftID <= 0 {
		return nil, ErrShiftNotFound
	}
	if input.SourceStaffID <= 0 || input.TargetStaffID <= 0 {
		return nil, fmt.Errorf("%w: source and target staff IDs are required", ErrShiftInvalid)
	}

	existing, err := s.findShiftForMove(ctx, input.ShiftID)
	if err != nil {
		return nil, err
	}

	lockIDs := []int64{input.SourceStaffID, input.TargetStaffID}
	if existing.OriginShiftID != nil {
		origin, err := s.loadOriginShift(ctx, *existing.OriginShiftID)
		if err != nil {
			return nil, err
		}
		lockIDs = append(lockIDs, origin.StaffID)
	}
	if err := s.lockStaffWritesOrdered(ctx, lockIDs); err != nil {
		return nil, err
	}

	// The pre-lock read only discovered the origin lock. All decisions and the
	// idempotency check use the authoritative post-lock row.
	existing, err = s.findShiftForMove(ctx, input.ShiftID)
	if err != nil {
		return nil, err
	}
	if existing.StaffID != input.SourceStaffID {
		if moveAlreadyApplied(existing, input) {
			return existing, nil
		}
		return nil, fmt.Errorf("%w: source staff no longer owns shift", ErrShiftConflict)
	}
	if existing.Cancelled {
		return nil, fmt.Errorf("%w: cancelled shifts cannot be moved", ErrShiftInvalid)
	}

	staff, err := s.staffRepo.FindByID(ctx, input.TargetStaffID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: target staff member not found", ErrShiftInvalid)
		}
		return nil, fmt.Errorf("find target staff member for shift move: %w", err)
	}
	if staff == nil {
		return nil, fmt.Errorf("%w: target staff member not found", ErrShiftInvalid)
	}

	moved := *existing
	moved.StaffID = input.TargetStaffID
	moved.Date = input.Date
	moved.StartTime = input.StartTime
	moved.EndTime = input.EndTime
	moved.BreakMinutes = input.BreakMinutes
	moved.ShiftTypeID = input.ShiftTypeID
	if input.ActorStaffID > 0 {
		moved.UpdatedBy = &input.ActorStaffID
	}

	if err := moved.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrShiftInvalid, err.Error())
	}
	// A type assigned to another person is a fresh assignment, so an inactive
	// type cannot hitchhike across staff. Same-person moves may retain their
	// already-attached inactive type, matching ordinary edits.
	if moved.StaffID != existing.StaffID || !sameShiftTypeID(moved.ShiftTypeID, existing.ShiftTypeID) {
		if err := s.ensureShiftTypeActive(ctx, moved.ShiftTypeID); err != nil {
			return nil, err
		}
	}
	if err := s.validateMovedOrigin(ctx, existing, &moved); err != nil {
		return nil, err
	}
	if err := s.checkOverlap(ctx, &moved); err != nil {
		return nil, err
	}

	vacatesSeriesOccurrence := existing.SeriesID != nil &&
		(existing.StaffID != moved.StaffID || existing.Date != moved.Date)
	if vacatesSeriesOccurrence {
		if s.exceptionRepo == nil {
			return nil, errors.New("series exception repository is required for shift move")
		}
		if err := s.recordSeriesException(ctx, existing, existing.Date); err != nil {
			return nil, fmt.Errorf("record original series occurrence for shift move: %w", err)
		}
	}
	if existing.SeriesID != nil {
		if existing.StaffID == moved.StaffID {
			moved.Detached = true
		} else {
			// A series belongs to its original staff member. The moved target row is
			// standalone; the exception above keeps the original occurrence consumed.
			moved.SeriesID = nil
			moved.Detached = false
		}
	}

	if err := s.repo.Update(ctx, &moved); err != nil {
		return nil, err
	}
	s.getLogger().Info("staff shift moved",
		"shift_id", moved.ID,
		"source_staff_id", existing.StaffID,
		"target_staff_id", moved.StaffID,
		"date", moved.Date.String(),
	)
	return &moved, nil
}

func (s *staffShiftService) findShiftForMove(ctx context.Context, id int64) (*scheduleModels.StaffShift, error) {
	shift, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrShiftNotFound
		}
		return nil, fmt.Errorf("find staff shift for move: %w", err)
	}
	if shift == nil {
		return nil, ErrShiftNotFound
	}
	return shift, nil
}

func moveAlreadyApplied(shift *scheduleModels.StaffShift, input MoveShiftInput) bool {
	return shift.StaffID == input.TargetStaffID &&
		shift.Date == input.Date &&
		timezone.SameClockTime(shift.StartTime, input.StartTime) &&
		timezone.SameClockTime(shift.EndTime, input.EndTime) &&
		shift.BreakMinutes == input.BreakMinutes &&
		sameShiftTypeID(shift.ShiftTypeID, input.ShiftTypeID) &&
		shift.SeriesID == nil
}

func (s *staffShiftService) validateMovedOrigin(ctx context.Context, existing, moved *scheduleModels.StaffShift) error {
	if moved.OriginShiftID != nil {
		return s.validateOriginLink(ctx, moved)
	}
	windowChanged := !timezone.SameClockTime(moved.StartTime, existing.StartTime) ||
		!timezone.SameClockTime(moved.EndTime, existing.EndTime)
	if moved.StaffID == existing.StaffID && moved.Date == existing.Date && !windowChanged {
		return nil
	}
	covers, err := s.repo.FindByOriginShiftID(ctx, existing.ID)
	if err != nil {
		return fmt.Errorf("check covers before shift move: %w", err)
	}
	if len(covers) > 0 {
		return fmt.Errorf("%w: cannot move a shift that has replacements", ErrShiftInvalid)
	}
	return nil
}

func (s *staffShiftService) recordSeriesException(ctx context.Context, shift *scheduleModels.StaffShift, date timezone.Date) error {
	exception := &scheduleModels.StaffShiftSeriesException{
		SeriesID:  *shift.SeriesID,
		Date:      date,
		CreatedBy: shift.CreatedBy,
	}
	exception.TenantID = shift.TenantID
	return s.exceptionRepo.Create(ctx, exception)
}
