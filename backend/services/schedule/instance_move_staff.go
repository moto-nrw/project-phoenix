// Package schedule — atomic staff move between two same-day blocks (#1884).
//
// MoveStaffBetweenBlocks applies "Mensa nimmt eine Person vom Schulhof" as ONE
// save: the removal from the source block and the assignment to the target
// block happen in the caller's tenant tx, guarded by the shared day lock, and
// leave a single staff_moved Änderungsprotokoll entry. Assigning a free
// on-shift person from the pool is the same operation without a source block.
//
// Same plan-then-write discipline as deviation_apply.go: every 4xx is decided
// before the first row is touched, because TenantTxMiddleware only rolls back
// on 5xx.
package schedule

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Move actions returned to the handler.
const (
	MoveStaffActionMoved          = "moved"
	MoveStaffActionAssigned       = "assigned"
	MoveStaffActionAlreadyApplied = "already_applied"
)

// MoveStaffInput is the parsed move request. A nil SourceInstanceID assigns a
// person from the pool (create) instead of relocating an existing assignment.
// Shift gaps and other assignments remain advisory; absence is the one
// day-wide availability state that blocks the write.
type MoveStaffInput struct {
	StaffID          int64
	SourceInstanceID *int64
	ActorAccountID   *int64
}

// MoveStaffResult is what MoveStaffBetweenBlocks returns on success.
type MoveStaffResult struct {
	Target *scheduleModel.ActivityInstance
	Source *scheduleModel.ActivityInstance
	Action string
	// Warnings lists the moved person's remaining same-day time overlaps with
	// the target window (advisory, never blocking — #1873 semantics).
	Warnings      []SubstituteTimeConflict
	ActiveTouched map[int64]*scheduleModel.ActivityInstance
}

// MoveStaffBetweenBlocks moves (or pool-assigns) one staff member onto the
// target block atomically. Runs inside the caller's tenant tx.
func (s *instanceService) MoveStaffBetweenBlocks(ctx context.Context, targetID int64, in MoveStaffInput) (*MoveStaffResult, error) {
	if in.StaffID <= 0 {
		return nil, devErrBadRequest("staff_id must be a positive id")
	}
	if in.SourceInstanceID != nil && *in.SourceInstanceID == targetID {
		return nil, devErrBadRequest("source and target block must differ")
	}

	target, err := s.loadDeviationInstance(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if target.Date.Before(timezone.TodayDate()) {
		return nil, devErrBadRequest("block date is in the past")
	}

	// Serialize with every other day-wide staffing mutation, then re-read both
	// blocks under the lock (a concurrent PUT may have moved either one).
	if err := s.acquireSubstituteDayLock(ctx, target.Date); err != nil {
		return nil, devErrInternal("lock day failed", err)
	}
	plan, err := s.planStaffMove(ctx, targetID, target.Date, in)
	if err != nil {
		return nil, err
	}
	if plan.action == MoveStaffActionAlreadyApplied {
		return &MoveStaffResult{
			Target:        plan.target,
			Source:        plan.source,
			Action:        plan.action,
			Warnings:      []SubstituteTimeConflict{},
			ActiveTouched: map[int64]*scheduleModel.ActivityInstance{},
		}, nil
	}
	return s.executeStaffMove(ctx, plan, in)
}

// staffMovePlan is the fully-validated Phase-A result.
type staffMovePlan struct {
	target    *scheduleModel.ActivityInstance
	source    *scheduleModel.ActivityInstance
	sourceRow *scheduleModel.InstanceStaff
	action    string
	staffID   int64
}

// planStaffMove re-reads both blocks under the day lock and runs every 4xx
// precondition without writing a row.
func (s *instanceService) planStaffMove(ctx context.Context, targetID int64, lockedDate timezone.Date, in MoveStaffInput) (*staffMovePlan, error) {
	target, err := s.loadDeviationInstance(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if target.Date != lockedDate || !isPlannableInstance(target) {
		return nil, devErrConflict("instance_moved", "block was changed concurrently; reopen it and try again")
	}

	var source *scheduleModel.ActivityInstance
	if in.SourceInstanceID != nil {
		source, err = s.loadDeviationInstance(ctx, *in.SourceInstanceID)
		if err != nil {
			return nil, err
		}
		if !isPlannableInstance(source) {
			return nil, devErrConflict("invalid_transition", "source block is no longer editable")
		}
		// The move is a same-window operation: both blocks share the day lock,
		// and a cross-day "move" is really a different planning action.
		if source.Date != target.Date {
			return nil, devErrBadRequest("source and target block must be on the same day")
		}
	}

	staff, err := s.deps.StaffRepo.FindByID(ctx, in.StaffID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, devErrNotFound("staff not found")
		}
		return nil, devErrInternal("load staff failed", err)
	}
	if staff == nil || staff.ID == 0 {
		return nil, devErrNotFound("staff not found")
	}

	targetRows, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, targetID)
	if err != nil {
		return nil, devErrInternal("load target staff failed", err)
	}
	var onTarget *scheduleModel.InstanceStaff
	for _, row := range targetRows {
		if row.StaffID == in.StaffID {
			onTarget = row
			break
		}
	}

	plan := &staffMovePlan{target: target, source: source, staffID: in.StaffID}

	if source == nil {
		// Pool assign: create a fresh row. Retrying after success is a no-op.
		if onTarget != nil {
			if onTarget.IsAbsent {
				return nil, devErrConflict("staff_absent_on_target", "die Person ist auf dem Zielblock abwesend markiert")
			}
			plan.action = MoveStaffActionAlreadyApplied
			return plan, nil
		}
		// Absence is day-wide (#1840), not target-row-wide. The pool UI never
		// offers absent staff, but a stale/direct API request must enforce the
		// same rule authoritatively under the shared day lock.
		if err := s.rejectDayWideAbsence(ctx, in.StaffID, target.Date); err != nil {
			return nil, err
		}
		plan.action = MoveStaffActionAssigned
		return plan, nil
	}

	sourceRows, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, source.ID)
	if err != nil {
		return nil, devErrInternal("load source staff failed", err)
	}
	var onSource *scheduleModel.InstanceStaff
	for _, row := range sourceRows {
		if row.StaffID == in.StaffID {
			onSource = row
			break
		}
	}

	if onTarget != nil {
		if onTarget.IsAbsent {
			// Same state, same code as the pool-assign path above.
			return nil, devErrConflict("staff_absent_on_target", "die Person ist auf dem Zielblock abwesend markiert")
		}
		if onSource == nil {
			// A successful move retried: already on target, gone from source. This
			// deliberately cannot distinguish a genuine retry from a client that
			// supplied the wrong historical source after the person reached target.
			plan.action = MoveStaffActionAlreadyApplied
			return plan, nil
		}
		return nil, devErrConflict("staff_already_on_target", "die Person ist bereits dem Zielblock zugeordnet")
	}
	if onSource == nil {
		return nil, devErrBadRequest("die Person ist dem Quellblock nicht zugeordnet")
	}
	if onSource.IsAbsent {
		return nil, devErrBadRequest("eine abwesend markierte Person kann nicht verschoben werden")
	}
	// Absence is day-wide (#1840): an absent row on ANY same-day block blocks
	// the move, even when the source row itself is not the one carrying it.
	// Same stale/direct-request defense as the pool-assign path above.
	if err := s.rejectDayWideAbsence(ctx, in.StaffID, target.Date); err != nil {
		return nil, err
	}

	plan.sourceRow = onSource
	plan.action = MoveStaffActionMoved
	return plan, nil
}

// rejectDayWideAbsence returns the staff_absent_on_date conflict when any
// same-day instance_staff row marks the person absent (#1840: absence is
// day-wide, whichever block carries it).
func (s *instanceService) rejectDayWideAbsence(ctx context.Context, staffID int64, date timezone.Date) error {
	dayRows, err := s.deps.InstanceStaffRepo.FindByStaffAndDate(ctx, staffID, date)
	if err != nil {
		return devErrInternal("load same-day staff assignments failed", err)
	}
	for _, row := range dayRows {
		if row != nil && row.IsAbsent {
			return devErrConflict("staff_absent_on_date", "die Person ist an diesem Tag abwesend markiert")
		}
	}
	return nil
}

// executeStaffMove runs Phase B: relocate/create the row, sync live
// supervision, reconcile the target acknowledgement, log the protocol entry,
// and collect advisory time conflicts.
func (s *instanceService) executeStaffMove(ctx context.Context, plan *staffMovePlan, in MoveStaffInput) (*MoveStaffResult, error) {
	now := time.Now()
	activeTouched := make(map[int64]*scheduleModel.ActivityInstance)

	if plan.action == MoveStaffActionMoved {
		// Relocate the existing row (keeps its identity, mirrors MoveShift).
		// The deviation state resets: on the target the person is an ordinary
		// planned assignment, not a substitute, and the source room split is
		// meaningless there.
		row := plan.sourceRow
		row.InstanceID = plan.target.ID
		row.RoomID = nil
		row.IsPrimary = false
		row.IsSubstitute = false
		row.IsAbsent = false
		row.AbsenceReason = nil
		row.SickAbsenceID = nil
		if err := s.deps.InstanceStaffRepo.Update(ctx, row); err != nil {
			return nil, devErrInternal("move staff row failed", err)
		}
		if plan.source.Status == scheduleModel.InstanceStatusActive && plan.source.ActiveGroupID != nil {
			if _, err := s.deps.SupervisorRepo.EndByActiveGroupAndStaffID(ctx, *plan.source.ActiveGroupID, plan.staffID); err != nil {
				return nil, devErrInternal("end source supervision failed", err)
			}
			activeTouched[*plan.source.ActiveGroupID] = plan.source
		}
	} else {
		newRow := &scheduleModel.InstanceStaff{
			InstanceID: plan.target.ID,
			StaffID:    plan.staffID,
		}
		if err := s.deps.InstanceStaffRepo.Create(ctx, newRow); err != nil {
			return nil, devErrInternal("assign staff row failed", err)
		}
	}

	if plan.target.Status == scheduleModel.InstanceStatusActive && plan.target.ActiveGroupID != nil {
		// A person can already hold an open supervision on the target's live
		// group via a path that never wrote an instance_staff row; a second
		// Create would trip the end_date IS NULL partial unique index.
		open, err := s.deps.SupervisorRepo.FindByActiveGroupID(ctx, *plan.target.ActiveGroupID, true)
		if err != nil {
			return nil, devErrInternal("load target supervision failed", err)
		}
		alreadySupervising := false
		for _, sup := range open {
			if sup != nil && sup.StaffID == plan.staffID {
				alreadySupervising = true
				break
			}
		}
		if !alreadySupervising {
			newSup := &activeModel.GroupSupervisor{
				StaffID:   plan.staffID,
				GroupID:   *plan.target.ActiveGroupID,
				Role:      "supervisor",
				StartDate: timezone.DateFromTime(now),
			}
			newSup.SetTenantID(tenant.FromContext(ctx))
			if err := s.deps.SupervisorRepo.Create(ctx, newSup); err != nil {
				return nil, devErrInternal("start target supervision failed", err)
			}
		}
		activeTouched[*plan.target.ActiveGroupID] = plan.target
	}

	// The target gained a person: a now-satisfied "deliberately unstaffed"
	// acknowledgement must not linger (#1840 semantics). The source keeps its
	// ack — it is still deliberately understaffed, only more so; the gap list
	// picks up any NEW shortfall derived, without a write.
	if err := s.ClearUnderstaffedAckIfStaffed(ctx, plan.target.ID, in.ActorAccountID); err != nil {
		// A concurrent transition can make this a 4xx after the writes above
		// landed; force the tx back like executeDeviationPlan does.
		tenant.MarkRollback(ctx)
		return nil, err
	}

	if err := s.logStaffMovedEvent(ctx, plan, in.ActorAccountID); err != nil {
		return nil, devErrInternal("log staff move failed", err)
	}

	warnings, err := s.collectStaffMoveWarnings(ctx, plan)
	if err != nil {
		return nil, devErrInternal("staff move time-conflict detection failed", err)
	}

	return &MoveStaffResult{
		Target:        plan.target,
		Source:        plan.source,
		Action:        plan.action,
		Warnings:      warnings,
		ActiveTouched: activeTouched,
	}, nil
}

// logStaffMovedEvent appends the single staff_moved protocol entry, anchored
// on the TARGET block; old_value names the source block (omitted for a pool
// assign), new_value the target.
func (s *instanceService) logStaffMovedEvent(ctx context.Context, plan *staffMovePlan, actorAccountID *int64) error {
	var oldValue any
	if plan.source != nil {
		oldValue = staffMoveSlot(plan.source, "from")
	}
	return s.logDeviationEvent(ctx, deviationEventInput{
		instance:       plan.target,
		eventType:      auditModel.DeviationEventStaffMoved,
		subjectStaffID: &plan.staffID,
		oldValue:       oldValue,
		newValue:       staffMoveSlot(plan.target, "to"),
		actorAccountID: actorAccountID,
	})
}

// staffMoveSlot snapshots one side of the move for the protocol payload.
func staffMoveSlot(inst *scheduleModel.ActivityInstance, prefix string) map[string]any {
	return map[string]any{
		prefix + "_instance_id": inst.ID,
		prefix + "_title":       inst.Title,
		prefix + "_start_time":  timezone.NormalizeWallClock(inst.StartTime).Format("15:04"),
		prefix + "_end_time":    timezone.NormalizeWallClock(inst.EndTime).Format("15:04"),
	}
}

// collectStaffMoveWarnings returns the moved person's remaining same-day
// overlaps with the target window. A lookup failure propagates as an error:
// the probe runs inside the tenant tx, and a PostgreSQL error aborts that tx,
// so the eventual commit would fail after the client already saw a 200.
func (s *instanceService) collectStaffMoveWarnings(ctx context.Context, plan *staffMovePlan) ([]SubstituteTimeConflict, error) {
	ops := []SubstituteWriteOp{{Instance: plan.target, Action: SubstituteActionSubstituted}}
	warnings, err := s.buildSubstituteTimeConflicts(ctx, ops, plan.staffID, plan.target.Date)
	if err != nil {
		return nil, err
	}
	if warnings == nil {
		return []SubstituteTimeConflict{}, nil
	}
	return warnings, nil
}
