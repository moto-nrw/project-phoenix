package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Atomic staff move between two same-day blocks (#1884): "Mensa nimmt eine
// Person vom Schulhof" is ONE save. The removal from the source and the
// assignment to the target share the day lock and the caller's tenant tx
// and leave one staff_moved Änderungsprotokoll entry; a pool assign is the
// same move without a source. Every 4xx is decided before the first row is
// touched, because the middleware rolls back only on 5xx.

// Client messages of the staff move.
const (
	msgStaffAbsentOnTarget  = "die Person ist auf dem Zielblock abwesend markiert"
	msgStaffAlreadyOnTarget = "die Person ist bereits dem Zielblock zugeordnet"
	msgStaffNotOnSource     = "die Person ist dem Quellblock nicht zugeordnet"
	msgStaffAbsentOnSource  = "eine abwesend markierte Person kann nicht verschoben werden"
	msgStaffAbsentOnDate    = "die Person ist an diesem Tag abwesend markiert"
)

// MoveStaffBetweenBlocks moves (or pool-assigns) one staff member onto the
// target block atomically, inside the caller's tenant tx.
func (s *InstanceLifecycleService) MoveStaffBetweenBlocks(ctx context.Context, targetID int64, in timetable.MoveStaffInput) (*timetable.MoveStaffResult, error) {
	if in.StaffID <= 0 {
		return nil, timetable.DeviationBadRequest("staff_id must be a positive id")
	}
	if in.SourceInstanceID != nil && *in.SourceInstanceID == targetID {
		return nil, timetable.DeviationBadRequest("source and target block must differ")
	}
	target, err := s.loadDeviationInstance(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if target.Date.Before(timezone.TodayDate()) {
		return nil, timetable.DeviationBadRequest("block date is in the past")
	}
	targetDate := timezone.Date(target.Date)
	// Serialize with every other day-wide staffing mutation, then re-read
	// both blocks under the lock (a concurrent edit may have moved either).
	if err := s.acquireSubstituteDayLock(ctx, targetDate); err != nil {
		return nil, timetable.DeviationInternal("lock day failed", err)
	}
	plan, err := s.planStaffMove(ctx, targetID, targetDate, in)
	if err != nil {
		return nil, err
	}
	if plan.action == timetable.MoveStaffActionAlreadyApplied {
		return &timetable.MoveStaffResult{
			Target:        LifecycleInstanceOf(plan.target),
			Source:        LifecycleInstanceOf(plan.source),
			Action:        plan.action,
			Warnings:      []timetable.SubstituteTimeConflict{},
			ActiveTouched: timetable.TouchedActivities{},
		}, nil
	}
	return s.executeStaffMove(ctx, plan, in)
}

// staffMovePlan is the fully validated Phase-A result.
type staffMovePlan struct {
	target    *scheduleModel.ActivityInstance
	source    *scheduleModel.ActivityInstance
	sourceRow *scheduleModel.InstanceStaff
	action    string
	staffID   int64
}

// planStaffMove re-reads both blocks under the day lock and runs every 4xx
// precondition without writing a row.
func (s *InstanceLifecycleService) planStaffMove(ctx context.Context, targetID int64, lockedDate timezone.Date, in timetable.MoveStaffInput) (*staffMovePlan, error) {
	target, err := s.loadDeviationInstance(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if timezone.Date(target.Date) != lockedDate || !isPlannableInstance(target) {
		return nil, timetable.DeviationConflict("instance_moved", "block was changed concurrently; reopen it and try again")
	}
	source, err := s.loadMoveSource(ctx, target, in.SourceInstanceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireStaff(ctx, in.StaffID); err != nil {
		return nil, err
	}
	onTarget, err := s.staffRowOn(ctx, targetID, in.StaffID, "load target staff failed")
	if err != nil {
		return nil, err
	}
	plan := &staffMovePlan{target: target, source: source, staffID: in.StaffID}
	if source == nil {
		return s.planPoolAssign(ctx, plan, onTarget)
	}
	onSource, err := s.staffRowOn(ctx, source.ID, in.StaffID, "load source staff failed")
	if err != nil {
		return nil, err
	}
	return s.planRelocation(ctx, plan, onTarget, onSource)
}

// loadMoveSource loads the source block of a relocation; nil for a pool
// assign. The move is a same-window operation, so both blocks share the day.
func (s *InstanceLifecycleService) loadMoveSource(ctx context.Context, target *scheduleModel.ActivityInstance, sourceID *int64) (*scheduleModel.ActivityInstance, error) {
	if sourceID == nil {
		return nil, nil
	}
	source, err := s.loadDeviationInstance(ctx, *sourceID)
	if err != nil {
		return nil, err
	}
	if !isPlannableInstance(source) {
		return nil, timetable.DeviationConflict("invalid_transition", "source block is no longer editable")
	}
	if source.Date != target.Date {
		return nil, timetable.DeviationBadRequest("source and target block must be on the same day")
	}
	return source, nil
}

func (s *InstanceLifecycleService) requireStaff(ctx context.Context, staffID int64) error {
	staff, err := s.deps.StaffRepo.FindByID(ctx, staffID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return timetable.DeviationNotFound("staff not found")
		}
		return timetable.DeviationInternal("load staff failed", err)
	}
	if staff == nil || staff.ID == 0 {
		return timetable.DeviationNotFound("staff not found")
	}
	return nil
}

func (s *InstanceLifecycleService) staffRowOn(ctx context.Context, instanceID, staffID int64, failure string) (*scheduleModel.InstanceStaff, error) {
	rows, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, timetable.DeviationInternal(failure, err)
	}
	for _, row := range rows {
		if row.StaffID == staffID {
			return row, nil
		}
	}
	return nil, nil
}

// planPoolAssign creates a fresh target row; a retry after success is a
// no-op. Absence is day-wide (#1840): the pool never offers absent staff,
// but a stale or direct request is refused authoritatively under the lock.
func (s *InstanceLifecycleService) planPoolAssign(ctx context.Context, plan *staffMovePlan, onTarget *scheduleModel.InstanceStaff) (*staffMovePlan, error) {
	if onTarget != nil {
		if onTarget.IsAbsent {
			return nil, timetable.DeviationConflict("staff_absent_on_target", msgStaffAbsentOnTarget)
		}
		plan.action = timetable.MoveStaffActionAlreadyApplied
		return plan, nil
	}
	if err := s.rejectDayWideAbsence(ctx, plan.staffID, timezone.Date(plan.target.Date)); err != nil {
		return nil, err
	}
	plan.action = timetable.MoveStaffActionAssigned
	return plan, nil
}

// planRelocation moves the person's source row onto the target. Already on
// the target and gone from the source reads as a retried move; this cannot
// tell a genuine retry from a wrong historical source.
func (s *InstanceLifecycleService) planRelocation(ctx context.Context, plan *staffMovePlan, onTarget, onSource *scheduleModel.InstanceStaff) (*staffMovePlan, error) {
	if onTarget != nil {
		if onTarget.IsAbsent {
			return nil, timetable.DeviationConflict("staff_absent_on_target", msgStaffAbsentOnTarget)
		}
		if onSource == nil {
			plan.action = timetable.MoveStaffActionAlreadyApplied
			return plan, nil
		}
		return nil, timetable.DeviationConflict("staff_already_on_target", msgStaffAlreadyOnTarget)
	}
	if onSource == nil {
		return nil, timetable.DeviationBadRequest(msgStaffNotOnSource)
	}
	if onSource.IsAbsent {
		return nil, timetable.DeviationBadRequest(msgStaffAbsentOnSource)
	}
	// An absent row on ANY same-day block blocks the move (#1840).
	if err := s.rejectDayWideAbsence(ctx, plan.staffID, timezone.Date(plan.target.Date)); err != nil {
		return nil, err
	}
	plan.sourceRow = onSource
	plan.action = timetable.MoveStaffActionMoved
	return plan, nil
}

// rejectDayWideAbsence returns the staff_absent_on_date conflict when any
// same-day row marks the person absent.
func (s *InstanceLifecycleService) rejectDayWideAbsence(ctx context.Context, staffID int64, date timezone.Date) error {
	dayRows, err := s.deps.InstanceStaffRepo.FindByStaffAndDate(ctx, staffID, scheduleModel.Date(date))
	if err != nil {
		return timetable.DeviationInternal("load same-day staff assignments failed", err)
	}
	for _, row := range dayRows {
		if row != nil && row.IsAbsent {
			return timetable.DeviationConflict("staff_absent_on_date", msgStaffAbsentOnDate)
		}
	}
	return nil
}

// executeStaffMove runs Phase B: relocate or create the row, sync the live
// supervision, reconcile the target acknowledgement, log the protocol entry
// and collect the advisory time conflicts.
func (s *InstanceLifecycleService) executeStaffMove(ctx context.Context, plan *staffMovePlan, in timetable.MoveStaffInput) (*timetable.MoveStaffResult, error) {
	activeTouched := make(map[int64]*scheduleModel.ActivityInstance)
	if err := s.placeMovedStaff(ctx, plan, activeTouched); err != nil {
		return nil, err
	}
	if err := s.superviseTarget(ctx, plan, activeTouched); err != nil {
		return nil, err
	}
	// The target gained a person, so a now-satisfied acknowledgement must
	// not linger (#1840). The source keeps its ack: still deliberately
	// understaffed, only more so.
	if err := s.ClearUnderstaffedAckIfStaffed(ctx, plan.target.ID, in.ActorAccountID); err != nil {
		// A concurrent transition can make this a 4xx after the writes
		// above landed; force the tx back.
		tenant.MarkRollback(ctx)
		return nil, err
	}
	if err := s.logStaffMovedEvent(ctx, plan, in.ActorAccountID); err != nil {
		return nil, timetable.DeviationInternal("log staff move failed", err)
	}
	warnings, err := s.collectStaffMoveWarnings(ctx, plan)
	if err != nil {
		return nil, timetable.DeviationInternal("staff move time-conflict detection failed", err)
	}
	return &timetable.MoveStaffResult{
		Target:        LifecycleInstanceOf(plan.target),
		Source:        LifecycleInstanceOf(plan.source),
		Action:        plan.action,
		Warnings:      warnings,
		ActiveTouched: touchedActivitiesOf(activeTouched),
	}, nil
}

// placeMovedStaff relocates the source row (keeping its identity) or
// creates the pool row. On the target the person is an ordinary planned
// assignment, so the deviation state and the source room split reset.
func (s *InstanceLifecycleService) placeMovedStaff(ctx context.Context, plan *staffMovePlan, activeTouched map[int64]*scheduleModel.ActivityInstance) error {
	if plan.action != timetable.MoveStaffActionMoved {
		newRow := &scheduleModel.InstanceStaff{InstanceID: plan.target.ID, StaffID: plan.staffID}
		if err := s.deps.InstanceStaffRepo.Create(ctx, newRow); err != nil {
			return timetable.DeviationInternal("assign staff row failed", err)
		}
		return nil
	}
	row := plan.sourceRow
	row.InstanceID = plan.target.ID
	row.RoomID = nil
	row.IsPrimary = false
	row.IsSubstitute = false
	row.IsAbsent = false
	row.AbsenceReason = nil
	row.SickAbsenceID = nil
	if err := s.deps.InstanceStaffRepo.Update(ctx, row); err != nil {
		return timetable.DeviationInternal("move staff row failed", err)
	}
	if plan.source.Status == scheduleModel.InstanceStatusActive && plan.source.ActiveGroupID != nil {
		if _, err := s.deps.SupervisorRepo.EndByActiveGroupAndStaffID(ctx, *plan.source.ActiveGroupID, plan.staffID); err != nil {
			return timetable.DeviationInternal("end source supervision failed", err)
		}
		activeTouched[*plan.source.ActiveGroupID] = plan.source
	}
	return nil
}

// superviseTarget opens the person's supervision on a running target. The
// person may already supervise its session through a path that never wrote
// a staff row; a second supervision would trip the open-row unique index.
func (s *InstanceLifecycleService) superviseTarget(ctx context.Context, plan *staffMovePlan, activeTouched map[int64]*scheduleModel.ActivityInstance) error {
	if plan.target.Status != scheduleModel.InstanceStatusActive || plan.target.ActiveGroupID == nil {
		return nil
	}
	activeGroupID := *plan.target.ActiveGroupID
	open, err := s.deps.SupervisorRepo.FindByActiveGroupID(ctx, activeGroupID, true)
	if err != nil {
		return timetable.DeviationInternal("load target supervision failed", err)
	}
	for _, sup := range open {
		if sup != nil && sup.StaffID == plan.staffID {
			activeTouched[activeGroupID] = plan.target
			return nil
		}
	}
	newSup := &studentpresence.GroupSupervision{
		StaffID:   plan.staffID,
		GroupID:   activeGroupID,
		Role:      "supervisor",
		StartDate: timezone.DateFromTime(time.Now()).String(),
	}
	newSup.TenantID = tenant.FromContext(ctx)
	if err := s.deps.SupervisorRepo.CreateSupervision(ctx, newSup); err != nil {
		return timetable.DeviationInternal("start target supervision failed", err)
	}
	activeTouched[activeGroupID] = plan.target
	return nil
}

// logStaffMovedEvent appends the single staff_moved entry, anchored on the
// TARGET block; old_value names the source (omitted for a pool assign).
func (s *InstanceLifecycleService) logStaffMovedEvent(ctx context.Context, plan *staffMovePlan, actorAccountID *int64) error {
	var oldValue any
	if plan.source != nil {
		oldValue = staffMoveSlot(plan.source, "from")
	}
	return s.logDeviationEvent(ctx, deviationEventInput{
		instance:       plan.target,
		eventType:      DeviationEventStaffMoved,
		subjectStaffID: &plan.staffID,
		oldValue:       oldValue,
		newValue:       staffMoveSlot(plan.target, "to"),
		actorAccountID: actorAccountID,
	})
}

func staffMoveSlot(inst *scheduleModel.ActivityInstance, prefix string) map[string]any {
	return map[string]any{
		prefix + "_instance_id": inst.ID,
		prefix + "_title":       inst.Title,
		prefix + "_start_time":  timezone.NormalizeWallClock(inst.StartTime).Format("15:04"),
		prefix + "_end_time":    timezone.NormalizeWallClock(inst.EndTime).Format("15:04"),
	}
}

// collectStaffMoveWarnings returns the moved person's remaining same-day
// overlaps with the target window. A lookup failure propagates: it aborts
// the tenant tx, so reporting success would lie about the commit.
func (s *InstanceLifecycleService) collectStaffMoveWarnings(ctx context.Context, plan *staffMovePlan) ([]timetable.SubstituteTimeConflict, error) {
	warnings, err := s.deps.SubstituteConflicts.DetectSubstituteConflicts(ctx, timetable.SubstituteConflictProbe{
		StaffID:   plan.staffID,
		Date:      timezone.Date(plan.target.Date),
		Targets:   []timetable.SubstituteConflictInstance{toConflictInstance(plan.target)},
		TargetIDs: []int64{plan.target.ID},
	})
	if err != nil {
		return nil, err
	}
	if warnings == nil {
		return []timetable.SubstituteTimeConflict{}, nil
	}
	return warnings, nil
}
