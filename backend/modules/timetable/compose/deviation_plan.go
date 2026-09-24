package compose

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Phase A of a deviations save: lock the day, read every referenced row
// once, validate and classify every change — without writing a row.

// deviationAbsenceOp pairs a plannable instance row of an absent staff member
// with its instance and the trimmed absence reason, ready for the Phase-B write.
type deviationAbsenceOp struct {
	row      *scheduleModel.InstanceStaff
	instance *scheduleModel.ActivityInstance
	reason   *string
}

// deviationPresenceOp pairs a currently-absent row to be cleared with its
// instance.
type deviationPresenceOp struct {
	row      *scheduleModel.InstanceStaff
	instance *scheduleModel.ActivityInstance
}

// deviationSubOp tags a classified substitution write with the substitute id and
// trimmed reason.
type deviationSubOp struct {
	write  substituteWriteOp
	subID  int64
	reason *string
}

type deviationSubstitutionRemovalOp struct {
	row      *scheduleModel.InstanceStaff
	instance *scheduleModel.ActivityInstance
}

// deviationStaffByInstance is the projected staff state keyed by concrete
// appointment. A person's state on one appointment must never leak into a
// different appointment on the same day.
type deviationStaffByInstance map[int64]map[int64]bool

func (staff deviationStaffByInstance) add(instanceID, staffID int64) {
	if staff[instanceID] == nil {
		staff[instanceID] = make(map[int64]bool)
	}
	staff[instanceID][staffID] = true
}

// deviationPlan is the fully-classified Phase-A result: nothing here has written
// a row yet.
type deviationPlan struct {
	instance     *scheduleModel.ActivityInstance
	date         timezone.Date
	absencePlan  []deviationAbsenceOp
	presencePlan []deviationPresenceOp
	subPlan      []deviationSubOp
	removalPlan  []deviationSubstitutionRemovalOp
	subs         []timetable.DeviationSubstitutionInput
	finalAck     bool
	finalAckNote *string
	ackChanged   bool
}

// deviationProjection is the projected staff state of every touched
// appointment after the planned writes.
type deviationProjection struct {
	absent  deviationStaffByInstance
	present deviationStaffByInstance
	newSubs map[int64]int
	removed deviationStaffByInstance
}

type deviationReadSet struct {
	staffExists    map[int64]bool
	rowsByStaff    map[int64][]*scheduleModel.InstanceStaff
	instances      map[int64]*scheduleModel.ActivityInstance
	rowsByInstance map[int64][]*scheduleModel.InstanceStaff
}

func (s *staffDeviations) loadDeviationReadSet(
	ctx context.Context,
	instanceID int64,
	in timetable.ApplyDeviationsInput,
	date timezone.Date,
) (*deviationReadSet, error) {
	staffIDs := deviationInputStaffIDs(in)
	staff, err := s.deps.Staff.FindByIDs(ctx, staffIDs)
	if err != nil {
		return nil, timetable.DeviationInternal("load staff failed", err)
	}
	rows, err := s.deps.InstanceStaff.FindByStaffIDsAndDate(ctx, staffIDs, scheduleModel.Date(date))
	if err != nil {
		return nil, timetable.DeviationInternal("load staff assignments failed", err)
	}
	instanceIDs := deviationInputInstanceIDs(instanceID, in, rows)
	instances, err := s.deps.Instances.FindByIDs(ctx, instanceIDs)
	if err != nil {
		return nil, timetable.DeviationInternal("load target instances failed", err)
	}
	allRows, err := s.deps.InstanceStaff.FindByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return nil, timetable.DeviationInternal("load instance staff failed", err)
	}
	readSet := &deviationReadSet{
		staffExists:    make(map[int64]bool, len(staff)),
		rowsByStaff:    make(map[int64][]*scheduleModel.InstanceStaff, len(staffIDs)),
		instances:      make(map[int64]*scheduleModel.ActivityInstance, len(instances)),
		rowsByInstance: indexInstanceStaffRows(allRows),
	}
	for id := range staff {
		readSet.staffExists[id] = true
	}
	for _, row := range rows {
		readSet.rowsByStaff[row.StaffID] = append(readSet.rowsByStaff[row.StaffID], row)
	}
	for _, instance := range instances {
		readSet.instances[instance.ID] = instance
	}
	return readSet, nil
}

// orderedIDs collects distinct positive ids in first-seen order.
type orderedIDs struct {
	seen map[int64]bool
	ids  []int64
}

func newOrderedIDs() *orderedIDs {
	return &orderedIDs{seen: make(map[int64]bool), ids: make([]int64, 0)}
}

func (set *orderedIDs) add(id int64) {
	if id > 0 && !set.seen[id] {
		set.seen[id] = true
		set.ids = append(set.ids, id)
	}
}

func (set *orderedIDs) addScope(scope *[]int64) {
	if scope == nil {
		return
	}
	for _, id := range *scope {
		set.add(id)
	}
}

func deviationInputStaffIDs(in timetable.ApplyDeviationsInput) []int64 {
	set := newOrderedIDs()
	for _, row := range in.Absences {
		set.add(row.StaffID)
	}
	for _, row := range in.Presences {
		set.add(row.StaffID)
	}
	for _, row := range in.Substitutions {
		set.add(row.AbsentStaffID)
		set.add(row.SubstituteStaffID)
	}
	for _, row := range in.SubstitutionRemovals {
		set.add(row.StaffID)
	}
	return set.ids
}

func deviationInputInstanceIDs(instanceID int64, in timetable.ApplyDeviationsInput, rows []*scheduleModel.InstanceStaff) []int64 {
	set := newOrderedIDs()
	set.add(instanceID)
	for _, row := range in.Absences {
		set.addScope(row.InstanceIDs)
	}
	for _, row := range in.Presences {
		set.addScope(row.InstanceIDs)
	}
	for _, row := range in.Substitutions {
		set.addScope(row.InstanceIDs)
	}
	for _, row := range in.SubstitutionRemovals {
		set.addScope(row.InstanceIDs)
	}
	for _, row := range rows {
		set.add(row.InstanceID)
	}
	return set.ids
}

// planDeviations runs the whole Phase-A dry-run: it takes the day lock, re-reads
// under it, validates every staff reference, classifies the absence / presence /
// substitution writes, and reconciles the selected block's acknowledgement — all
// without writing a row.
func (s *staffDeviations) planDeviations(ctx context.Context, instanceID int64, instance *scheduleModel.ActivityInstance, in timetable.ApplyDeviationsInput) (*deviationPlan, error) {
	date := timezone.Date(instance.Date)
	locked, err := s.lockDeviationDay(ctx, instanceID, date)
	if err != nil {
		return nil, err
	}

	readSet, err := s.loadDeviationReadSet(ctx, instanceID, in, date)
	if err != nil {
		return nil, err
	}
	if err := validateDeviationStaff(in, readSet); err != nil {
		return nil, err
	}
	if err := rejectContradictoryDeviationScopes(in); err != nil {
		return nil, err
	}
	plan, projection, err := classifyDeviations(in, date, readSet)
	if err != nil {
		return nil, err
	}
	plan.instance = locked

	// Restoring a persisted absence must not orphan an already-assigned
	// substitute (over-staffing). Reject before any write (#1840).
	if err := rejectOverstaffingPresences(plan.presencePlan, projection, readSet); err != nil {
		return nil, err
	}

	plan.finalAck, plan.finalAckNote, plan.ackChanged, err = reconcileSelectedAck(instanceID, locked, in, projection, readSet)
	if err != nil {
		return nil, err
	}
	return plan, nil
}

// lockDeviationDay serializes concurrent saves for the whole (tenant, date)
// BEFORE any classification read — one request may target several
// appointments on the day, while the Sammel-Vertretung still targets the
// whole day — then re-reads the block: PUT /instances/{id} may have MOVED it
// to another day between the initial read and this lock. A move (or a
// concurrent cancel/complete) aborts the save (#1840).
func (s *staffDeviations) lockDeviationDay(ctx context.Context, instanceID int64, date timezone.Date) (*scheduleModel.ActivityInstance, error) {
	if err := s.acquireSubstituteDayLock(ctx, date); err != nil {
		return nil, timetable.DeviationInternal("lock day failed", err)
	}
	locked, err := s.loadDeviationInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if timezone.Date(locked.Date) != date || !isPlannableInstance(locked) {
		return nil, timetable.DeviationConflict("instance_moved", msgInstanceMoved)
	}
	return locked, nil
}

// classifyDeviations stages the absence, presence, removal and substitution
// writes and projects the resulting staff state per appointment.
func classifyDeviations(in timetable.ApplyDeviationsInput, date timezone.Date, readSet *deviationReadSet) (*deviationPlan, deviationProjection, error) {
	absencePlan, err := planAbsences(in.Absences, date, readSet)
	if err != nil {
		return nil, deviationProjection{}, err
	}
	presencePlan, err := planPresences(in.Presences, date, readSet)
	if err != nil {
		return nil, deviationProjection{}, err
	}
	removalPlan, err := planSubstitutionRemovals(in.SubstitutionRemovals, date, readSet)
	if err != nil {
		return nil, deviationProjection{}, err
	}
	absenceOnlyByInstance := absentStaffFromAbsences(absencePlan)
	removedSubstitutes := removedSubstituteStaffFromPlans(removalPlan)
	// Removing an absent substitute row subsumes restoring that exact row. Keep
	// any other planned assignments in the presence plan so one atomic request
	// can remove the obsolete role here and restore the person elsewhere (#2577).
	presencePlan = withoutRemovedSubstitutePresences(presencePlan, removedSubstitutes)
	subPlan, newSubByInstance, err := planSubstitutions(in.Substitutions, absenceOnlyByInstance, removedSubstitutes, readSet)
	if err != nil {
		return nil, deviationProjection{}, err
	}
	absencePlan = withoutSubstitutionTargets(absencePlan, subPlan)
	plan := &deviationPlan{
		date:         date,
		absencePlan:  absencePlan,
		presencePlan: presencePlan,
		subPlan:      subPlan,
		removalPlan:  removalPlan,
		subs:         in.Substitutions,
	}
	return plan, deviationProjection{
		absent:  absentStaffFromPlans(absencePlan, subPlan),
		present: presentStaffFromPlans(presencePlan),
		newSubs: newSubByInstance,
		removed: removedSubstitutes,
	}, nil
}

// withoutSubstitutionTargets prevents an all-day absence plus appointment-
// scoped coverage from staging the same original row twice. applySubstitute
// already marks its original row absent, so the standalone absence operation is
// needed only for the uncovered appointments.
func withoutSubstitutionTargets(absences []deviationAbsenceOp, substitutions []deviationSubOp) []deviationAbsenceOp {
	coveredRows := make(map[int64]bool, len(substitutions))
	for _, substitution := range substitutions {
		if substitution.write.OrigRow != nil {
			coveredRows[substitution.write.OrigRow.ID] = true
		}
	}
	filtered := make([]deviationAbsenceOp, 0, len(absences))
	for _, absence := range absences {
		if !coveredRows[absence.row.ID] {
			filtered = append(filtered, absence)
		}
	}
	return filtered
}

func absentStaffFromAbsences(absences []deviationAbsenceOp) deviationStaffByInstance {
	absent := make(deviationStaffByInstance)
	for _, op := range absences {
		absent.add(op.instance.ID, op.row.StaffID)
	}
	return absent
}

func absentStaffFromPlans(absences []deviationAbsenceOp, substitutions []deviationSubOp) deviationStaffByInstance {
	absent := absentStaffFromAbsences(absences)
	for _, op := range substitutions {
		if op.write.OrigRow != nil {
			absent.add(op.write.Instance.ID, op.write.OrigRow.StaffID)
		}
	}
	return absent
}

func presentStaffFromPlans(presences []deviationPresenceOp) deviationStaffByInstance {
	present := make(deviationStaffByInstance)
	for _, op := range presences {
		present.add(op.instance.ID, op.row.StaffID)
	}
	return present
}

func removedSubstituteStaffFromPlans(removals []deviationSubstitutionRemovalOp) deviationStaffByInstance {
	removed := make(deviationStaffByInstance)
	for _, op := range removals {
		removed.add(op.instance.ID, op.row.StaffID)
	}
	return removed
}

func withoutRemovedSubstitutePresences(
	presences []deviationPresenceOp,
	removed deviationStaffByInstance,
) []deviationPresenceOp {
	kept := make([]deviationPresenceOp, 0, len(presences))
	for _, presence := range presences {
		if presence.row.IsSubstitute && removed[presence.instance.ID][presence.row.StaffID] {
			continue
		}
		kept = append(kept, presence)
	}
	return kept
}

// rejectOverstaffingPresences returns a 409 when clearing a persisted absence
// would push any touched instance above its planned headcount, which only
// happens when a restore orphans an already-assigned substitute (#1840).
func rejectOverstaffingPresences(presencePlan []deviationPresenceOp, projection deviationProjection, readSet *deviationReadSet) error {
	checked := make(map[int64]bool)
	for _, op := range presencePlan {
		if checked[op.instance.ID] {
			continue
		}
		checked[op.instance.ID] = true
		rows := readSet.rowsByInstance[op.instance.ID]
		if projection.nonAbsentCount(op.instance.ID, rows) > plannedPositions(rows) {
			return timetable.DeviationConflict("presence_would_overstaff",
				"der Termin ist bereits vollständig besetzt. Entfernen Sie zuerst die nicht mehr benötigte Vertretung")
		}
	}
	return nil
}

// plannedPositions counts the block's planned (non-substitute) rows.
func plannedPositions(rows []*scheduleModel.InstanceStaff) int {
	planned := 0
	for _, row := range rows {
		if !row.IsSubstitute {
			planned++
		}
	}
	return planned
}

// nonAbsentCount projects how many people remain non-absent on one
// appointment after the planned writes.
func (p deviationProjection) nonAbsentCount(instanceID int64, rows []*scheduleModel.InstanceStaff) int {
	return projectedNonAbsentCount(rows, p.absent[instanceID], p.present[instanceID], p.newSubs[instanceID], p.removed[instanceID])
}

// reconcileSelectedAck decides the selected block's final acknowledgement after
// the projected save. "Deliberately unstaffed" is valid whenever the block ends
// up understaffed; the deviation writes never change the planned baseline
// (#1840).
func reconcileSelectedAck(
	instanceID int64,
	instance *scheduleModel.ActivityInstance,
	in timetable.ApplyDeviationsInput,
	projection deviationProjection,
	readSet *deviationReadSet,
) (finalAck bool, note *string, ackChanged bool, err error) {
	thisRows := readSet.rowsByInstance[instanceID]
	projectedUnderstaffed := timetable.IsUnderstaffedCounts(projection.nonAbsentCount(instanceID, thisRows), plannedPositions(thisRows))

	finalAck = instance.UnderstaffedAck
	if in.UnderstaffedAck != nil {
		if *in.UnderstaffedAck && !projectedUnderstaffed {
			return false, nil, false, timetable.DeviationConflict("understaffed_still_staffed",
				"dieser Block kann nicht als bewusst unbesetzt markiert werden, solange er vollständig besetzt ist")
		}
		finalAck = *in.UnderstaffedAck
		ackChanged = finalAck != instance.UnderstaffedAck ||
			(finalAck && !sameNote(instance.UnderstaffedNote, in.UnderstaffedNote))
		if finalAck {
			note = trimDeviationReason(in.UnderstaffedNote)
		}
	} else if instance.UnderstaffedAck && !projectedUnderstaffed {
		finalAck = false
		ackChanged = true
	}
	return finalAck, note, ackChanged, nil
}

// loadPlannableInstance loads the instance behind a staff row, returning nil when
// it is terminal (completed/cancelled) so callers skip it.
func loadPlannableInstance(row *scheduleModel.InstanceStaff, readSet *deviationReadSet) (*scheduleModel.ActivityInstance, error) {
	instance := readSet.instances[row.InstanceID]
	if instance == nil {
		return nil, timetable.DeviationInternalDetail(fmt.Sprintf("instance_staff %d references missing instance %d", row.ID, row.InstanceID))
	}
	if !isPlannableInstance(instance) {
		return nil, nil
	}
	return instance, nil
}

// loadScopedPlannableInstance is loadPlannableInstance for a change with a
// scope: a terminal appointment is skipped for a day-wide change and rejected
// for an explicit one, rather than silently widening or partially applying
// the request.
func loadScopedPlannableInstance(row *scheduleModel.InstanceStaff, scope *[]int64, readSet *deviationReadSet) (*scheduleModel.ActivityInstance, error) {
	instance, err := loadPlannableInstance(row, readSet)
	if err != nil {
		return nil, err
	}
	if instance == nil && scope != nil {
		return nil, timetable.DeviationConflict("instance_not_editable", msgInstanceNotEditable)
	}
	return instance, nil
}

// classifySubstitute decides the action for a single target instance. Pure
// logic, no DB. Returns (action, conflictingOtherStaffID, ok=false on 409).
func classifySubstitute(
	allRows []*scheduleModel.InstanceStaff,
	origRow *scheduleModel.InstanceStaff,
	subID int64,
) (action string, conflictOtherStaff int64, ok bool) {
	var existingSubOfSub *scheduleModel.InstanceStaff
	var subAsNonAbsent *scheduleModel.InstanceStaff
	var anyActiveSubOfOther *scheduleModel.InstanceStaff
	absentPlanned := 0
	activeSubsOfOther := 0
	for _, row := range allRows {
		switch {
		case row.IsSubstitute && row.StaffID == subID:
			existingSubOfSub = row
		case row.IsSubstitute && !row.IsAbsent:
			activeSubsOfOther++
			anyActiveSubOfOther = row
		case !row.IsSubstitute && row.IsAbsent:
			absentPlanned++
		}
		if !row.IsSubstitute && row.StaffID == subID && !row.IsAbsent {
			subAsNonAbsent = row
		}
	}

	// The substitute already holds a substitute row on this instance. A second
	// insert would violate UNIQUE(instance_id, staff_id) (#1840).
	if existingSubOfSub != nil {
		if origRow.IsAbsent {
			return timetable.SubstituteActionAlreadySubstitute, 0, true
		}
		return timetable.SubstituteActionAlreadyOnInstance, 0, true
	}

	// Count-based overstaffing guard (#1840): reject a new substitute only when
	// origRow is already flagged absent AND every absent position is already
	// covered — adding one more would overstaff.
	if origRow.IsAbsent && activeSubsOfOther >= absentPlanned {
		return "", anyActiveSubOfOther.StaffID, false
	}

	if subAsNonAbsent != nil {
		// Substitute is already a co-supervisor on this instance; mark the absent's
		// row and leave the co-supervisor row untouched (is_substitute=false).
		return timetable.SubstituteActionAlreadyOnInstance, 0, true
	}
	return timetable.SubstituteActionSubstituted, 0, true
}

// projectAbsent returns a shallow-copied view of rows with the given staff ids
// forced absent, plus the copy of origRow. The originals are never mutated.
func projectAbsent(rows []*scheduleModel.InstanceStaff, absent map[int64]bool, origRow *scheduleModel.InstanceStaff) (projected []*scheduleModel.InstanceStaff, origProjected *scheduleModel.InstanceStaff) {
	projected = make([]*scheduleModel.InstanceStaff, len(rows))
	for i, row := range rows {
		clone := *row
		if absent[clone.StaffID] {
			clone.IsAbsent = true
		}
		projected[i] = &clone
		if row.ID == origRow.ID {
			origProjected = &clone
		}
	}
	if origProjected == nil {
		clone := *origRow
		origProjected = &clone
	}
	return projected, origProjected
}

// projectedNonAbsentCount counts staff that remain non-absent on an instance
// after the deviation writes.
func projectedNonAbsentCount(rows []*scheduleModel.InstanceStaff, absent, presence map[int64]bool, newSubs int, removedSubs map[int64]bool) int {
	count := 0
	for _, row := range rows {
		if row.IsSubstitute && removedSubs[row.StaffID] {
			continue
		}
		if absent[row.StaffID] {
			continue
		}
		if row.IsAbsent && !presence[row.StaffID] {
			continue
		}
		count++
	}
	return count + newSubs
}
