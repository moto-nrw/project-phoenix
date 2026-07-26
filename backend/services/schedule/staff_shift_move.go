package schedule

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
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
	// ActorAccountID stamps the shift_moved Änderungsprotokoll entry (#1884);
	// nil records an actor-less event.
	ActorAccountID *int64
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
	moveChanged := staffShiftMoveChanged(existing, &moved)
	if !moveChanged {
		return existing, nil
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
		(existing.StaffID != moved.StaffID || seriesOccurrenceDate(existing) != moved.Date)
	if vacatesSeriesOccurrence {
		if s.exceptionRepo == nil {
			return nil, errors.New("series exception repository is required for shift move")
		}
		if err := s.recordSeriesException(ctx, existing, seriesOccurrenceDate(existing)); err != nil {
			return nil, fmt.Errorf("record original series occurrence for shift move: %w", err)
		}
	}
	if existing.SeriesID != nil {
		if existing.StaffID == moved.StaffID {
			if moveChanged {
				moved.Detached = true
			}
		} else {
			// A series belongs to its original staff member. The moved target row is
			// standalone; the exception above keeps the original occurrence consumed.
			moved.SeriesID = nil
			moved.Detached = false
			moved.SeriesOccurrenceDate = nil
		}
	}

	if err := s.repo.Update(ctx, &moved); err != nil {
		return nil, err
	}
	if err := s.logShiftMovedEvent(ctx, existing, &moved, input.ActorAccountID); err != nil {
		return nil, err
	}
	s.getLogger().Info("staff shift moved",
		"shift_id", moved.ID,
		"source_staff_id", existing.StaffID,
		"target_staff_id", moved.StaffID,
		"date", moved.Date.String(),
	)
	s.broadcastTimeTrackingChanged(ctx)
	return &moved, nil
}

// logShiftMovedEvent appends the shift_moved Änderungsprotokoll row (#1884)
// inside the caller's tenant tx. The event anchors via StaffShiftID (a shift
// is not an activity slot); occurrence_date/start_time snapshot the NEW slot.
// Fail closed when the repo is wired: an audit failure aborts the move. The
// repo is optional only for unit tests constructed without audit wiring —
// production (services/factory.go) always sets it.
func (s *staffShiftService) logShiftMovedEvent(ctx context.Context, existing, moved *scheduleModels.StaffShift, actorAccountID *int64) error {
	if s.deviationEventRepo == nil {
		return nil
	}
	event := &auditModels.DeviationEvent{
		OccurrenceDate: moved.Date,
		StartTime:      timezone.WallClock(moved.StartTime),
		StaffShiftID:   &moved.ID,
		SubjectStaffID: &existing.StaffID,
		EventType:      auditModels.DeviationEventShiftMoved,
		ActorAccountID: normalizeActor(actorAccountID),
	}
	if moved.StaffID != existing.StaffID {
		event.RelatedStaffID = &moved.StaffID
	}
	var err error
	if event.OldValue, err = marshalDeviationValue(shiftMoveSlot(existing)); err != nil {
		return fmt.Errorf("log shift move: marshal old value: %w", err)
	}
	if event.NewValue, err = marshalDeviationValue(shiftMoveSlot(moved)); err != nil {
		return fmt.Errorf("log shift move: marshal new value: %w", err)
	}
	if err := s.deviationEventRepo.Create(ctx, event); err != nil {
		return fmt.Errorf("log shift move event: %w", err)
	}
	return nil
}

// shiftMoveSlot snapshots the audit-relevant slot of one shift state.
func shiftMoveSlot(shift *scheduleModels.StaffShift) map[string]any {
	return map[string]any{
		"staff_id":   shift.StaffID,
		"date":       shift.Date.String(),
		"start_time": timezone.WallClock(shift.StartTime).Format("15:04"),
		"end_time":   timezone.WallClock(shift.EndTime).Format("15:04"),
	}
}

func staffShiftMoveChanged(existing, moved *scheduleModels.StaffShift) bool {
	return existing.StaffID != moved.StaffID ||
		existing.Date != moved.Date ||
		!timezone.SameClockTime(existing.StartTime, moved.StartTime) ||
		!timezone.SameClockTime(existing.EndTime, moved.EndTime) ||
		existing.BreakMinutes != moved.BreakMinutes ||
		!sameShiftTypeID(existing.ShiftTypeID, moved.ShiftTypeID)
}

// seriesOccurrenceDate returns the immutable recurrence slot for a materialized
// row. The Date fallback keeps mock-built and pre-migration in-memory rows safe;
// migration 1.15.202 backfills every persisted series row.
func seriesOccurrenceDate(shift *scheduleModels.StaffShift) timezone.Date {
	if shift.SeriesOccurrenceDate != nil {
		return *shift.SeriesOccurrenceDate
	}
	return shift.Date
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
