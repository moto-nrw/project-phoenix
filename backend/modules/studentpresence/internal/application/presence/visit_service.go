package presence

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Visit operations

func (s *service) CreateVisit(ctx context.Context, visit *studentpresence.Visit) error {
	return s.runInSessionTx(ctx, func(txCtx context.Context) error {
		return s.createVisit(txCtx, visit)
	})
}

func (s *service) createVisit(ctx context.Context, visit *studentpresence.Visit) error {
	if !validPresenceVisit(visit) {
		return &ActiveError{Op: "CreateVisit", Err: ErrInvalidData}
	}

	// Validate the student exists and is not a graduated alumnus before INSERT
	// (prevents FK errors in logs and, via the FOR UPDATE lock, closes the race
	// against a concurrent grade-transition apply — see the helper doc, #405).
	// Runs BEFORE the binary-mode short-circuit: binary-mode check-ins reach
	// CreateVisit via the timetable path and would otherwise mark a departed
	// alumnus present without ever hitting this guard.
	if err := s.ensureStudentCheckinAllowed(ctx, visit.StudentID); err != nil {
		return &ActiveError{Op: "CreateVisit", Err: err}
	}

	// Binary-mode tenants don't track room visits — attendance is the only
	// surface. Short-circuit here so every caller (IoT, web, scheduler) stays
	// consistent without having to resolve the mode at each call site.
	mode, err := s.GetPresenceMode(ctx)
	if err != nil {
		return &ActiveError{Op: "CreateVisit", Err: errors.Join(ErrDatabaseOperation, err)}
	}
	if mode == PresenceModeBinary {
		return nil
	}

	if err := s.prepareVisitCheckIn(ctx, visit); err != nil {
		return err
	}
	return s.recordCheckedInVisit(ctx, visit)
}

// prepareVisitCheckIn runs the checks and attendance writes that precede a
// visit insert: the locked open target session, no other open visit, room
// capacity, the day's attendance and the flags a check-in clears.
func (s *service) prepareVisitCheckIn(ctx context.Context, visit *studentpresence.Visit) error {
	// Lock and re-check the target immediately before the visit-side writes.
	// The lock serializes check-ins with session absorption/end paths, which
	// take the same active.groups row lock before closing the session.
	targetGroup, err := s.lockActiveGroupOpenForUpdate(ctx, visit.ActiveGroupID)
	if err != nil {
		return &ActiveError{Op: "CreateVisit", Err: err}
	}

	deviceID, staffID := s.extractContextIDs(ctx)

	// Ensure no existing active visit for this student
	if err := s.ensureStudentHasNoActiveVisit(ctx, visit.StudentID); err != nil {
		return createVisitError(err)
	}
	if visit.ExitTime == nil {
		if err := s.ensureRoomCapacity(ctx, targetGroup.RoomID, 1); err != nil {
			return err
		}
		if err := s.ensureActivityParticipantLimit(ctx, targetGroup, 1); err != nil {
			return err
		}
	}

	// Handle attendance (create new or update on re-entry)
	if err := s.ensureOrUpdateAttendance(ctx, visit, staffID, deviceID); err != nil {
		return createVisitError(err)
	}

	// Auto-clear sickness / excused flags when student checks in
	// (only triggers when the tenant's clear_mode setting is "next_checkin").
	return s.autoClearCheckinStatuses(ctx, visit.StudentID, "CreateVisit")
}

// createVisitError keeps an operation error and hides any other failure
// behind the database sentinel.
func createVisitError(err error) error {
	if activeErr, ok := err.(*ActiveError); ok {
		return activeErr
	}
	return &ActiveError{Op: "CreateVisit", Err: ErrDatabaseOperation}
}

// recordCheckedInVisit inserts the visit, mirrors it into its timetable slot
// and queues the success event.
func (s *service) recordCheckedInVisit(ctx context.Context, visit *studentpresence.Visit) error {
	// Create the visit record
	visit.TenantID = tenant.FromContext(ctx)
	stored, err := s.SchoolPresence.RecordVisit(ctx, studentpresence.Visit{
		ID: visit.ID, TenantID: visit.TenantID, CreatedAt: visit.CreatedAt, UpdatedAt: visit.UpdatedAt,
		StudentID: visit.StudentID, ActiveGroupID: visit.ActiveGroupID,
		EntryTime: visit.EntryTime, ExitTime: visit.ExitTime,
	})
	if err != nil {
		// The partial unique index uniq_active_visits_open_per_student is the
		// race-safety net behind ensureStudentHasNoActiveVisit above. When
		// two concurrent requests both pass the read-then-write check, the
		// loser hits 23505 here. Translate to ErrStudentAlreadyActive so the
		// IoT handler maps it to 409 Conflict instead of 500.
		if isDuplicateActiveVisitViolation(err) {
			return &ActiveError{Op: "CreateVisit", Err: ErrStudentAlreadyActive}
		}
		return &ActiveError{Op: "CreateVisit", Err: ErrDatabaseOperation}
	}
	visit.ID, visit.CreatedAt, visit.UpdatedAt = stored.ID, stored.CreatedAt, stored.UpdatedAt

	// Mirror the visit into its timetable instance in the same transaction.
	// Missing assignments are valid; failures must precede any success event.
	var snapshot *AttendanceSnapshot
	if s.AttendanceSyncer != nil {
		snapshot, err = s.AttendanceSyncer.MirrorCheckInForVisit(ctx, presenceVisitSnapshot(visit))
		if err != nil {
			return &ActiveError{Op: "CreateVisit", Err: errors.Join(ErrDatabaseOperation, err)}
		}
	}

	// Queue the success event for commit.
	s.broadcastVisitCreated(ctx, visit, snapshot)

	return nil
}

// isDuplicateActiveVisitViolation returns true when the store refused the
// visit on the partial unique index uniq_active_visits_open_per_student,
// defined in migration 1.15.47 on active.visits (tenant_id, student_id) WHERE
// exit_time IS NULL. The Postgres adapter matches the constraint name rather
// than just the error code, so a future unrelated unique index on
// active.visits doesn't accidentally translate into ErrStudentAlreadyActive.
func isDuplicateActiveVisitViolation(err error) bool {
	return errors.Is(err, ports.ErrOpenVisitExists)
}

func (s *service) UpdateVisit(ctx context.Context, visit *studentpresence.Visit) error {
	var operationErr error
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		operationErr = s.updateVisit(txCtx, visit)
		return operationErr
	})
	if err != nil && operationErr == nil {
		return &ActiveError{Op: "UpdateVisit", Err: errors.Join(ErrDatabaseOperation, err)}
	}
	return err
}

func (s *service) updateVisit(ctx context.Context, visit *studentpresence.Visit) error {
	existing, err := s.prepareVisitRevision(ctx, visit)
	if err != nil {
		return err
	}
	isActiveGroupMove, transferAt, err := s.prepareVisitTransfer(ctx, existing, visit)
	if err != nil {
		return err
	}
	intervalChanged := visitIntervalChanged(existing, visit)

	if _, err := s.SchoolPresence.ReviseVisit(ctx, *visit); err != nil {
		if isDuplicateActiveVisitViolation(err) {
			return &ActiveError{Op: "UpdateVisit", Err: ErrStudentAlreadyActive}
		}
		return &ActiveError{Op: "UpdateVisit", Err: ErrDatabaseOperation}
	}

	if intervalChanged {
		if err := s.syncAttendanceForVisitRevision(ctx, existing, visit); err != nil {
			return &ActiveError{Op: "UpdateVisit", Err: ErrDatabaseOperation}
		}
	}

	if isActiveGroupMove {
		sourceSnapshot, targetSnapshot, err := s.syncMovedVisitAttendance(ctx, existing, visit, transferAt)
		if err != nil {
			return &ActiveError{Op: "UpdateVisit", Err: errors.Join(ErrDatabaseOperation, err)}
		}
		s.broadcastVisitMoved(ctx, existing, visit, sourceSnapshot, targetSnapshot)
		s.trackProductEvent(ctx, "room_transfer", nil)
		return nil
	}
	if intervalChanged && s.AttendanceSyncer != nil {
		if err := s.AttendanceSyncer.MirrorVisitRevision(ctx, presenceVisitSnapshot(existing), presenceVisitSnapshot(visit)); err != nil {
			return &ActiveError{Op: "UpdateVisit", Err: errors.Join(ErrDatabaseOperation, err)}
		}
	}

	return nil
}

// prepareVisitRevision freezes the affected child before locking and re-reading
// the interval. All later attendance/visit writes use that locked state.
func (s *service) prepareVisitRevision(ctx context.Context, visit *studentpresence.Visit) (*studentpresence.Visit, error) {
	if !validPresenceVisit(visit) {
		return nil, &ActiveError{Op: "UpdateVisit", Err: ErrInvalidData}
	}

	previous, err := s.findStoredVisit(ctx, visit.ID)
	if err != nil {
		return nil, err
	}
	if err := s.lockVisitRevisionStudents(ctx, previous.StudentID, visit.StudentID); err != nil {
		return nil, err
	}
	if err := s.SchoolPresence.PrepareVisitWrites(ctx, studentpresence.VisitWriteScope{VisitIDs: []int64{visit.ID}, SessionIDs: []int64{visit.ActiveGroupID}}); err != nil {
		return nil, &ActiveError{Op: "UpdateVisit", Err: err}
	}

	existing, err := s.findStoredVisit(ctx, visit.ID)
	if err != nil {
		return nil, err
	}

	if existing.StudentID != previous.StudentID {
		return nil, &ActiveError{Op: "UpdateVisit", Err: studentpresence.ErrVisitWriteConflict}
	}
	return existing, nil
}

// findStoredVisit reads the interval; the caller prepares its locks separately.
func (s *service) findStoredVisit(ctx context.Context, id int64) (*studentpresence.Visit, error) {
	stored, err := s.SchoolPresence.FindVisit(ctx, id)
	if err != nil {
		if errors.Is(err, studentpresence.ErrVisitNotFound) {
			return nil, &ActiveError{Op: "UpdateVisit", Err: ErrVisitNotFound}
		}
		return nil, &ActiveError{Op: "UpdateVisit", Err: ErrDatabaseOperation}
	}
	if stored == nil {
		return nil, &ActiveError{Op: "UpdateVisit", Err: ErrVisitNotFound}
	}
	return stored, nil
}

// Interval revisions can also update school attendance. Lock the children
// first, as bulk move/care exit do, so visit -> attendance cannot close a
// cycle with those commands' attendance -> visit suffix. Historical edits do
// not apply the check-in eligibility guard.
func (s *service) lockVisitRevisionStudents(ctx context.Context, stored, requested int64) error {
	if s.StudentRepo == nil {
		return nil
	}
	ids := []int64{stored, requested}
	slices.Sort(ids)
	_, err := s.StudentRepo.FindByIDsForUpdate(ctx, slices.Compact(ids))
	if err != nil {
		return &ActiveError{Op: "UpdateVisit", Err: err}
	}
	return nil
}

func visitIntervalChanged(previous, updated *studentpresence.Visit) bool {
	if previous == nil || updated == nil || !previous.EntryTime.Equal(updated.EntryTime) {
		return true
	}
	if previous.ExitTime == nil || updated.ExitTime == nil {
		return previous.ExitTime != updated.ExitTime
	}
	return !previous.ExitTime.Equal(*updated.ExitTime)
}

func (s *service) prepareVisitTransfer(
	ctx context.Context,
	existing, updated *studentpresence.Visit,
) (bool, time.Time, error) {
	isReopening := existing.ExitTime != nil && updated.ExitTime == nil
	isGroupMove := existing.ExitTime == nil && existing.ActiveGroupID != updated.ActiveGroupID
	if !isReopening && !isGroupMove {
		return false, time.Time{}, nil
	}

	targetGroup, err := s.lockVisitTransferTarget(ctx, updated.ActiveGroupID)
	if err != nil {
		return false, time.Time{}, err
	}
	if err := s.ensureVisitTransferCapacity(ctx, existing, updated, targetGroup, isReopening); err != nil {
		return false, time.Time{}, err
	}

	transferAt := time.Now()
	if updated.ExitTime != nil {
		// A combined transfer/checkout has no separate transfer timestamp in the
		// request. Use checkout as the boundary so target check-in precedes close.
		transferAt = *updated.ExitTime
	}
	return isGroupMove, transferAt, nil
}

// lockVisitTransferTarget revalidates the running target and its room after
// PrepareVisitWrites acquired the visit/advisory/session locks in order.
func (s *service) lockVisitTransferTarget(ctx context.Context, groupID int64) (*ports.ActiveGroup, error) {
	target, err := activeVisitTransferTarget(s.GroupRepo.FindByID(ctx, groupID))
	if err != nil {
		return nil, err
	}
	if target.RoomID > 0 {
		if err := s.SchoolPresence.LockRoomSessionWrites(ctx, target.RoomID); err != nil {
			return nil, &ActiveError{Op: "UpdateVisit", Err: ErrDatabaseOperation}
		}
	}
	locked, err := activeVisitTransferTarget(s.GroupRepo.FindByIDForUpdate(ctx, groupID))
	if err != nil {
		return nil, err
	}
	if locked.RoomID != target.RoomID {
		return nil, &ActiveError{Op: "UpdateVisit", Err: ErrDatabaseOperation}
	}
	return locked, nil
}

// activeVisitTransferTarget maps a target lookup to the running session or
// the transfer's not-found and database errors.
func activeVisitTransferTarget(group *ports.ActiveGroup, err error) (*ports.ActiveGroup, error) {
	if err != nil {
		if IsNoRows(err) {
			return nil, &ActiveError{Op: "UpdateVisit", Err: ErrActiveGroupNotFound}
		}
		return nil, &ActiveError{Op: "UpdateVisit", Err: ErrDatabaseOperation}
	}
	if group == nil || !group.IsActive() {
		return nil, &ActiveError{Op: "UpdateVisit", Err: ErrActiveGroupNotFound}
	}
	return group, nil
}

// ensureVisitTransferCapacity checks the target room has space when a visit
// is reopened, or moved while still open into another room, and that the
// target activity has space when the visit enters its session (#3632).
func (s *service) ensureVisitTransferCapacity(ctx context.Context, existing, updated *studentpresence.Visit, targetGroup *ports.ActiveGroup, isReopening bool) error {
	if isReopening {
		if err := s.ensureRoomCapacity(ctx, targetGroup.RoomID, 1); err != nil {
			return err
		}
		return s.ensureActivityParticipantLimit(ctx, targetGroup, 1)
	}
	sourceGroup, err := s.GroupRepo.FindByID(ctx, existing.ActiveGroupID)
	if err != nil || sourceGroup == nil {
		return &ActiveError{Op: "UpdateVisit", Err: ErrDatabaseOperation}
	}
	if updated.ExitTime != nil {
		return nil
	}
	if sourceGroup.RoomID != targetGroup.RoomID {
		if err := s.ensureRoomCapacity(ctx, targetGroup.RoomID, 1); err != nil {
			return err
		}
	}
	return s.ensureActivityParticipantLimit(ctx, targetGroup, 1)
}

// syncMovedVisitAttendance mirrors a transfer independently of SSE. The
// pre-update visit identifies the source slot; a transfer-time copy identifies
// the target slot. This avoids resolving checkout against the already-mutated
// target group.
func (s *service) syncMovedVisitAttendance(
	ctx context.Context,
	previousVisit, movedVisit *studentpresence.Visit,
	transferAt time.Time,
) (sourceSnapshot, targetSnapshot *AttendanceSnapshot, err error) {
	if s.AttendanceSyncer == nil || previousVisit == nil || movedVisit == nil {
		return nil, nil, nil
	}

	source := *previousVisit
	source.ExitTime = &transferAt
	sourceSnapshot, err = s.AttendanceSyncer.MirrorCheckOutForVisit(ctx, &source)
	if err != nil {
		return nil, nil, err
	}

	target := *movedVisit
	target.EntryTime = transferAt
	target.ExitTime = nil
	targetSnapshot, err = s.AttendanceSyncer.MirrorCheckInForVisit(ctx, &target)
	if err != nil {
		return nil, nil, err
	}

	if movedVisit.ExitTime != nil {
		target.ExitTime = movedVisit.ExitTime
		closedSnapshot, err := s.AttendanceSyncer.MirrorCheckOutForVisit(ctx, &target)
		if err != nil {
			return nil, nil, err
		}
		if closedSnapshot != nil {
			targetSnapshot = closedSnapshot
		}
	}
	return sourceSnapshot, targetSnapshot, nil
}

func (s *service) DeleteVisit(ctx context.Context, id int64) error {
	return s.runInSessionTx(ctx, func(txCtx context.Context) error {
		return s.deleteVisit(txCtx, id)
	})
}

func (s *service) deleteVisit(ctx context.Context, id int64) error {
	if id <= 0 {
		return &ActiveError{Op: "DeleteVisit", Err: ErrVisitNotFound}
	}
	_, err := s.SchoolPresence.FindVisit(ctx, id)
	if err != nil {
		if errors.Is(err, studentpresence.ErrVisitNotFound) {
			return &ActiveError{Op: "DeleteVisit", Err: ErrVisitNotFound}
		}
		return &ActiveError{Op: "DeleteVisit", Err: errors.Join(ErrDatabaseOperation, err)}
	}

	if err := s.SchoolPresence.DeleteVisit(ctx, id); err != nil {
		return &ActiveError{Op: "DeleteVisit", Err: errors.Join(ErrDatabaseOperation, err)}
	}

	return nil
}

// LockStudent takes the child's row lock in the caller's transaction. A
// presence write that holds a child locks it before any of the child's
// visits, as the bulk move and the check-in guard do (#5278); a caller that
// ends the child's visit and opens the next one locks the child before it
// reads that visit.
func (s *service) LockStudent(ctx context.Context, studentID int64) error {
	if err := tenant.RequireTransaction(ctx); err != nil {
		return &ActiveError{Op: "LockStudent", Err: err}
	}
	if studentID <= 0 {
		return &ActiveError{Op: "LockStudent", Err: ErrStudentNotFound}
	}
	if s.StudentRepo == nil {
		return nil
	}
	if _, err := s.StudentRepo.FindByIDForUpdate(ctx, studentID); err != nil {
		if IsNoRows(err) {
			return &ActiveError{Op: "LockStudent", Err: ErrStudentNotFound}
		}
		return &ActiveError{Op: "LockStudent", Err: errors.Join(ErrDatabaseOperation, err)}
	}
	return nil
}

func (s *service) EndVisit(ctx context.Context, id int64) error {
	return s.runInSessionTx(ctx, func(txCtx context.Context) error {
		return s.endVisit(txCtx, id)
	})
}

func (s *service) endVisit(ctx context.Context, id int64) error {
	// Binary-mode tenants don't keep visit rows, so there's nothing to end.
	// Callers that hold a stale visit ID from before a mode switch hit this
	// no-op path instead of a missing-row error.
	mode, err := s.GetPresenceMode(ctx)
	if err != nil {
		return &ActiveError{Op: "EndVisit", Err: errors.Join(ErrDatabaseOperation, err)}
	}
	if mode == PresenceModeBinary {
		return nil
	}

	endedVisit, snapshot, err := s.endVisitWithAttendanceSync(ctx, id)
	if err != nil {
		if activeErr, ok := err.(*ActiveError); ok {
			return activeErr
		}
		return &ActiveError{Op: "EndVisit", Err: ErrDatabaseOperation}
	}

	s.broadcastVisitCheckout(ctx, endedVisit, snapshot)
	return nil
}

// endVisitWithAttendanceSync closes one visit and mirrors its exact persisted
// checkout timestamp into slot attendance. Session-end and timeout paths use
// this helper too. The one visit-ending path that bypasses it — the nightly
// bulk close in EndDailySessions — mirrors its checkouts via the scheduler's
// session-end bridge (CloseOpenCheckoutsByActiveGroupIDs).
// A missing bridge returns no snapshot. Sync failures abort the visit close.
func (s *service) endVisitWithAttendanceSync(
	ctx context.Context, id int64,
) (*studentpresence.Visit, *AttendanceSnapshot, error) {
	endedVisit, err := s.endVisitRecord(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	var snapshot *AttendanceSnapshot
	if s.AttendanceSyncer != nil {
		snapshot, err = s.AttendanceSyncer.MirrorCheckOutForVisit(ctx, presenceVisitSnapshot(endedVisit))
		if err != nil {
			return nil, nil, &ActiveError{Op: "EndVisit", Err: errors.Join(ErrDatabaseOperation, err)}
		}
	}
	s.wakeGuardiansAfterCommit(ctx, endedVisit.StudentID)
	return endedVisit, snapshot, nil
}

// endVisitRecord ends the visit record and returns the updated visit. The
// close only touches an open visit; a close that ends nothing reads the visit
// to tell a missing visit from an ended one.
func (s *service) endVisitRecord(ctx context.Context, id int64) (*studentpresence.Visit, error) {
	closed, err := s.SchoolPresence.CloseVisits(ctx, []int64{id}, time.Now())
	if err != nil {
		return nil, &ActiveError{Op: "EndVisit", Err: errors.Join(ErrDatabaseOperation, err)}
	}
	if len(closed) > 0 {
		return &closed[0], nil
	}
	visit, err := s.SchoolPresence.FindVisit(ctx, id)
	if err != nil {
		if errors.Is(err, studentpresence.ErrVisitNotFound) {
			return nil, &ActiveError{Op: "EndVisit", Err: ErrVisitNotFound}
		}
		return nil, &ActiveError{Op: "EndVisit", Err: errors.Join(ErrDatabaseOperation, err)}
	}
	if visit == nil {
		return nil, &ActiveError{Op: "EndVisit", Err: ErrVisitNotFound}
	}
	return nil, &ActiveError{Op: "EndVisit", Err: ErrVisitAlreadyEnded}
}

// visitSSEData holds data needed for SSE broadcasts after a visit is ended
type visitSSEData struct {
	StudentID        int64
	EducationGroupID *int64
}
