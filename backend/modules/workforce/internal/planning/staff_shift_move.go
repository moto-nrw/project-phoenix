package planning

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ShiftMoveAudit appends a shift_moved entry inside the caller's tenant
// transaction; the composition binds it to the audit deviation events.
type ShiftMoveAudit func(ctx context.Context, event workforce.ShiftMovedEvent) error

// MoveShiftInput is the complete desired slot for one concrete shift. The
// source owner is an optimistic-concurrency guard: a retry after a successful
// cross-person move returns the already-moved row, while a genuinely stale
// request is rejected instead of overwriting a later move.
type MoveShiftInput struct {
	ShiftID       int64
	SourceStaffID int64
	TargetStaffID int64
	Date          calendar.Date
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
func (s *staffShiftService) MoveShift(ctx context.Context, input MoveShiftInput) (*workforce.ShiftRecord, error) {
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
	if err := s.lockMoveStaff(ctx, existing, input); err != nil {
		return nil, err
	}

	// The pre-lock read only discovered the origin lock. All decisions and the
	// idempotency check use the authoritative post-lock row.
	existing, alreadyApplied, err := s.reloadLockedMoveSource(ctx, input)
	if err != nil || alreadyApplied {
		return existing, err
	}
	if err := s.ensureMoveTargetStaff(ctx, input.TargetStaffID); err != nil {
		return nil, err
	}

	moved := movedShift(existing, input)
	if !staffShiftMoveChanged(existing, &moved) {
		return existing, nil
	}
	if err := s.validateMove(ctx, existing, &moved); err != nil {
		return nil, err
	}
	if err := s.applyMoveSeriesRules(ctx, existing, &moved); err != nil {
		return nil, err
	}

	written, err := s.repo.UpdateStaffShift(ctx, ShiftCapability(&moved))
	if err = staffShiftUpdateError(err); err != nil {
		return nil, err
	}
	applyShiftToRow(&moved, written)
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

// lockMoveStaff locks the source, the target and — for a replacement — the
// origin's staff in one stable order.
func (s *staffShiftService) lockMoveStaff(ctx context.Context, existing *workforce.ShiftRecord, input MoveShiftInput) error {
	lockIDs := []int64{input.SourceStaffID, input.TargetStaffID}
	if existing.OriginShiftID != nil {
		origin, err := s.loadOriginShift(ctx, *existing.OriginShiftID)
		if err != nil {
			return err
		}
		lockIDs = append(lockIDs, origin.StaffID)
	}
	return s.lockStaffWritesOrdered(ctx, lockIDs)
}

// reloadLockedMoveSource re-reads the shift under the lock. It reports
// alreadyApplied with the stored row when a retry finds its own completed
// cross-person move.
func (s *staffShiftService) reloadLockedMoveSource(ctx context.Context, input MoveShiftInput) (*workforce.ShiftRecord, bool, error) {
	existing, err := s.findShiftForMove(ctx, input.ShiftID)
	if err != nil {
		return nil, false, err
	}
	if existing.StaffID != input.SourceStaffID {
		if moveAlreadyApplied(existing, input) {
			return existing, true, nil
		}
		return nil, false, fmt.Errorf("%w: source staff no longer owns shift", ErrShiftConflict)
	}
	if existing.Cancelled {
		return nil, false, fmt.Errorf("%w: cancelled shifts cannot be moved", ErrShiftInvalid)
	}
	return existing, false, nil
}

// ensureMoveTargetStaff rejects a move onto a staff member that does not exist.
func (s *staffShiftService) ensureMoveTargetStaff(ctx context.Context, targetStaffID int64) error {
	staff, err := s.staffRepo.FindByID(ctx, targetStaffID)
	if err != nil {
		if isRecordNotFound(err) {
			return fmt.Errorf("%w: target staff member not found", ErrShiftInvalid)
		}
		return fmt.Errorf("find target staff member for shift move: %w", err)
	}
	if staff == nil {
		return fmt.Errorf("%w: target staff member not found", ErrShiftInvalid)
	}
	return nil
}

// movedShift applies the requested slot to a copy of the stored row.
func movedShift(existing *workforce.ShiftRecord, input MoveShiftInput) workforce.ShiftRecord {
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
	return moved
}

// validateMove checks the moved row's shape, type, origin link and overlap.
func (s *staffShiftService) validateMove(ctx context.Context, existing, moved *workforce.ShiftRecord) error {
	if err := moved.Validate(); err != nil {
		return fmt.Errorf("%w: %s", ErrShiftInvalid, err.Error())
	}
	// A type assigned to another person is a fresh assignment, so an inactive
	// type cannot hitchhike across staff. Same-person moves may retain their
	// already-attached inactive type, matching ordinary edits.
	if moved.StaffID != existing.StaffID || !sameShiftTypeID(moved.ShiftTypeID, existing.ShiftTypeID) {
		if err := s.ensureShiftTypeActive(ctx, moved.ShiftTypeID); err != nil {
			return err
		}
	}
	if err := s.validateMovedOrigin(ctx, existing, moved); err != nil {
		return err
	}
	return s.checkOverlap(ctx, moved)
}

// applyMoveSeriesRules consumes a vacated series occurrence and sets the moved
// row's series fields. The caller only reaches it with a changed slot.
func (s *staffShiftService) applyMoveSeriesRules(ctx context.Context, existing, moved *workforce.ShiftRecord) error {
	vacatesSeriesOccurrence := existing.SeriesID != nil &&
		(existing.StaffID != moved.StaffID || seriesOccurrenceDate(existing) != moved.Date)
	if vacatesSeriesOccurrence {
		if s.exceptionRepo == nil {
			return errors.New("series exception repository is required for shift move")
		}
		if err := s.recordSeriesException(ctx, existing, seriesOccurrenceDate(existing)); err != nil {
			return fmt.Errorf("record original series occurrence for shift move: %w", err)
		}
	}
	if existing.SeriesID == nil {
		return nil
	}
	if existing.StaffID == moved.StaffID {
		moved.Detached = true
		return nil
	}
	// A series belongs to its original staff member. The moved target row is
	// standalone; the exception above keeps the original occurrence consumed.
	moved.SeriesID = nil
	moved.Detached = false
	moved.SeriesOccurrenceDate = nil
	return nil
}

// logShiftMovedEvent appends the shift_moved Änderungsprotokoll row (#1884)
// inside the caller's tenant tx (workforce.ShiftMovedEvent). Fail closed when the
// audit is wired: an audit failure aborts the move. The audit is optional
// only for unit tests constructed without audit wiring — production always
// sets it.
func (s *staffShiftService) logShiftMovedEvent(ctx context.Context, existing, moved *workforce.ShiftRecord, actorAccountID *int64) error {
	if s.shiftMoveAudit == nil {
		return nil
	}
	event := workforce.ShiftMovedEvent{
		OccurrenceDate: moved.Date,
		StartTime:      calendar.NormalizeWallClock(moved.StartTime),
		StaffShiftID:   moved.ID,
		SubjectStaffID: existing.StaffID,
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
	if err := s.shiftMoveAudit(ctx, event); err != nil {
		return fmt.Errorf("log shift move event: %w", err)
	}
	return nil
}

// shiftMoveSlot snapshots the audit-relevant slot of one shift state.
func shiftMoveSlot(shift *workforce.ShiftRecord) map[string]any {
	return map[string]any{
		"staff_id":   shift.StaffID,
		"date":       shift.Date.String(),
		"start_time": calendar.NormalizeWallClock(shift.StartTime).Format("15:04"),
		"end_time":   calendar.NormalizeWallClock(shift.EndTime).Format("15:04"),
	}
}

func staffShiftMoveChanged(existing, moved *workforce.ShiftRecord) bool {
	return existing.StaffID != moved.StaffID ||
		existing.Date != moved.Date ||
		!calendar.SameClockTime(existing.StartTime, moved.StartTime) ||
		!calendar.SameClockTime(existing.EndTime, moved.EndTime) ||
		existing.BreakMinutes != moved.BreakMinutes ||
		!sameShiftTypeID(existing.ShiftTypeID, moved.ShiftTypeID)
}

// seriesOccurrenceDate returns the immutable recurrence slot for a materialized
// row. The Date fallback keeps mock-built and pre-migration in-memory rows safe;
// migration 1.15.202 backfills every persisted series row.
func seriesOccurrenceDate(shift *workforce.ShiftRecord) calendar.Date {
	if shift.SeriesOccurrenceDate != nil {
		return *shift.SeriesOccurrenceDate
	}
	return shift.Date
}

func (s *staffShiftService) findShiftForMove(ctx context.Context, id int64) (*workforce.ShiftRecord, error) {
	shift, err := s.repo.ReadShift(ctx, id)
	err = shiftReadError("find by id", err, workforce.ErrStaffShiftNotFound)
	if err != nil {
		if isRecordNotFound(err) {
			return nil, ErrShiftNotFound
		}
		return nil, fmt.Errorf("find staff shift for move: %w", err)
	}
	if shift == nil {
		return nil, ErrShiftNotFound
	}
	return shift, nil
}

func moveAlreadyApplied(shift *workforce.ShiftRecord, input MoveShiftInput) bool {
	return shift.StaffID == input.TargetStaffID &&
		shift.Date == input.Date &&
		calendar.SameClockTime(shift.StartTime, input.StartTime) &&
		calendar.SameClockTime(shift.EndTime, input.EndTime) &&
		shift.BreakMinutes == input.BreakMinutes &&
		sameShiftTypeID(shift.ShiftTypeID, input.ShiftTypeID) &&
		shift.SeriesID == nil
}

func (s *staffShiftService) validateMovedOrigin(ctx context.Context, existing, moved *workforce.ShiftRecord) error {
	if moved.OriginShiftID != nil {
		return s.validateOriginLink(ctx, moved)
	}
	windowChanged := !calendar.SameClockTime(moved.StartTime, existing.StartTime) ||
		!calendar.SameClockTime(moved.EndTime, existing.EndTime)
	if moved.StaffID == existing.StaffID && moved.Date == existing.Date && !windowChanged {
		return nil
	}
	covers, err := s.repo.ReadShifts(ctx, workforce.StaffShiftFilter{OriginShiftID: existing.ID, Order: shiftOrder(workforce.StaffShiftOrderStartTime)})
	err = shiftReadError("find staff shifts by origin shift id", err, nil)
	if err != nil {
		return fmt.Errorf("check covers before shift move: %w", err)
	}
	if len(covers) > 0 {
		return withReason(fmt.Errorf("%w: cannot move a shift that has replacements", ErrShiftInvalid), workforce.ErrShiftHasReplacements)
	}
	return nil
}

func (s *staffShiftService) recordSeriesException(ctx context.Context, shift *workforce.ShiftRecord, date calendar.Date) error {
	exception := &StaffShiftSeriesException{
		SeriesID:  *shift.SeriesID,
		Date:      date,
		CreatedBy: shift.CreatedBy,
	}
	exception.TenantID = shift.TenantID
	return shiftWriteError("create staff shift series exception", s.exceptionRepo.RecordSeriesException(ctx, workforce.StaffShiftSeriesException{ID: exception.ID, TenantID: exception.TenantID, SeriesID: exception.SeriesID, Date: exception.Date.String(), CreatedBy: exception.CreatedBy, CreatedAt: exception.CreatedAt}))
}
