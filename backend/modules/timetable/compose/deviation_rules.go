package compose

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// The validation and classification rules of Phase A. Every rule reads the
// day's read set only; none writes.

// staffReferences validates each referenced staff member once: a positive id
// (400) that exists in the tenant (404).
type staffReferences struct {
	seen    map[int64]bool
	readSet *deviationReadSet
}

func (refs *staffReferences) ensure(staffID int64, label string) error {
	if staffID <= 0 {
		return timetable.DeviationBadRequest(fmt.Sprintf("die Auswahl für %s ist ungültig", label))
	}
	if refs.seen[staffID] {
		return nil
	}
	refs.seen[staffID] = true
	if !refs.readSet.staffExists[staffID] {
		return timetable.DeviationNotFound(fmt.Sprintf("%s wurde nicht gefunden", label))
	}
	return nil
}

// validateDeviationStaff runs every 4xx precondition on the referenced staff:
// existence (404), self-substitution (400), a substitute also being marked
// absent (400), and a substitute already absent in the DB that day (400).
func validateDeviationStaff(in timetable.ApplyDeviationsInput, readSet *deviationReadSet) error {
	refs := &staffReferences{seen: make(map[int64]bool), readSet: readSet}
	for _, absence := range in.Absences {
		if err := refs.ensure(absence.StaffID, "die abwesende Person"); err != nil {
			return err
		}
	}
	for _, presence := range in.Presences {
		if err := refs.ensure(presence.StaffID, "die anwesende Person"); err != nil {
			return err
		}
	}
	if err := validateSubstitutionStaff(in, refs); err != nil {
		return err
	}
	for _, removal := range in.SubstitutionRemovals {
		if err := refs.ensure(removal.StaffID, "die Ersatzperson"); err != nil {
			return err
		}
	}
	return nil
}

func validateSubstitutionStaff(in timetable.ApplyDeviationsInput, refs *staffReferences) error {
	seenAbsentSub := make(map[int64]bool)
	for _, sub := range in.Substitutions {
		// The editor chooses one scope and one replacement per absent person, so an
		// absent staff id must appear at most once in one save.
		if seenAbsentSub[sub.AbsentStaffID] {
			return timetable.DeviationBadRequest("für eine abwesende Person darf nur eine Ersatzperson gewählt werden")
		}
		seenAbsentSub[sub.AbsentStaffID] = true
		if err := refs.ensure(sub.AbsentStaffID, "die abwesende Person"); err != nil {
			return err
		}
		if err := refs.ensure(sub.SubstituteStaffID, "die Ersatzperson"); err != nil {
			return err
		}
		if err := validateSubstituteAvailable(in, sub, refs.readSet); err != nil {
			return err
		}
	}
	return nil
}

// validateSubstituteAvailable rejects a substitute who cannot cover the
// substitution's appointments.
func validateSubstituteAvailable(in timetable.ApplyDeviationsInput, sub timetable.DeviationSubstitutionInput, readSet *deviationReadSet) error {
	if sub.AbsentStaffID == sub.SubstituteStaffID {
		return timetable.DeviationBadRequest("die abwesende Person kann sich nicht selbst vertreten")
	}
	// A person cannot cover an appointment on which this same save marks them
	// absent. Appointment-scoped absences elsewhere on the day are independent.
	if inputMarksStaffAbsentInScope(in, sub.SubstituteStaffID, sub.InstanceIDs) {
		return timetable.DeviationBadRequest("die Ersatzperson ist in einem ausgewählten Termin selbst abwesend")
	}
	// ...nor if they are already absent on an appointment this substitution
	// targets. A terminbezogene Abwesenheit elsewhere on the day does not make
	// the person unavailable for a different appointment.
	for _, row := range readSet.rowsByStaff[sub.SubstituteStaffID] {
		if row.IsAbsent && scopeContainsInstance(sub.InstanceIDs, row.InstanceID) {
			return timetable.DeviationBadRequest("die Ersatzperson ist in einem ausgewählten Termin selbst abwesend")
		}
	}
	return nil
}

func rejectContradictoryDeviationScopes(in timetable.ApplyDeviationsInput) error {
	for _, presence := range in.Presences {
		if presenceContradictsAbsence(in, presence) {
			return timetable.DeviationBadRequest("eine Person kann im selben Termin nicht anwesend und abwesend sein")
		}
	}
	return nil
}

// presenceContradictsAbsence reports whether the save also marks the present
// person absent on an overlapping scope.
func presenceContradictsAbsence(in timetable.ApplyDeviationsInput, presence timetable.DeviationPresenceInput) bool {
	for _, absence := range in.Absences {
		if presence.StaffID == absence.StaffID && deviationScopesOverlap(presence.InstanceIDs, absence.InstanceIDs) {
			return true
		}
	}
	for _, substitution := range in.Substitutions {
		if presence.StaffID == substitution.AbsentStaffID && deviationScopesOverlap(presence.InstanceIDs, substitution.InstanceIDs) {
			return true
		}
	}
	return false
}

func planSubstitutionRemovals(
	removals []timetable.DeviationSubstitutionRemovalInput,
	date timezone.Date,
	readSet *deviationReadSet,
) ([]deviationSubstitutionRemovalOp, error) {
	plan := make([]deviationSubstitutionRemovalOp, 0)
	seenRows := make(map[int64]bool)
	for _, removal := range removals {
		if err := validateExplicitScopeInstances(date, removal.InstanceIDs, readSet); err != nil {
			return nil, err
		}
		ops, err := planSubstitutionRemoval(removal, readSet, seenRows)
		if err != nil {
			return nil, err
		}
		plan = append(plan, ops...)
	}
	return plan, nil
}

// planSubstitutionRemoval stages the substitute rows one removal targets. An
// explicit scope must name only appointments the person substitutes on.
func planSubstitutionRemoval(
	removal timetable.DeviationSubstitutionRemovalInput,
	readSet *deviationReadSet,
	seenRows map[int64]bool,
) ([]deviationSubstitutionRemovalOp, error) {
	var ops []deviationSubstitutionRemovalOp
	selected := make(map[int64]bool)
	for _, row := range readSet.rowsByStaff[removal.StaffID] {
		if !row.IsSubstitute || !scopeContainsInstance(removal.InstanceIDs, row.InstanceID) {
			continue
		}
		selected[row.InstanceID] = true
		op, err := stageSubstitutionRemoval(row, removal.InstanceIDs, readSet, seenRows)
		if err != nil {
			return nil, err
		}
		if op != nil {
			ops = append(ops, *op)
		}
	}
	if err := requireSelectedSubstitutions(removal.InstanceIDs, selected); err != nil {
		return nil, err
	}
	return ops, nil
}

// stageSubstitutionRemoval stages one substitute row; nil skips a row seen
// before or a terminal appointment of a day-wide removal. A row a sick report
// stamped stays locked.
func stageSubstitutionRemoval(row *scheduleModel.InstanceStaff, scope *[]int64, readSet *deviationReadSet, seenRows map[int64]bool) (*deviationSubstitutionRemovalOp, error) {
	if seenRows[row.ID] {
		return nil, nil
	}
	instance, err := loadScopedPlannableInstance(row, scope, readSet)
	if err != nil || instance == nil {
		return nil, err
	}
	if row.SickAbsenceID != nil {
		return nil, timetable.DeviationConflict("sick_absence_scope_locked", msgSickScopeLocked)
	}
	seenRows[row.ID] = true
	return &deviationSubstitutionRemovalOp{row: row, instance: instance}, nil
}

// requireSelectedSubstitutions rejects an explicit scope naming an
// appointment the person does not substitute on.
func requireSelectedSubstitutions(scope *[]int64, selected map[int64]bool) error {
	if scope == nil {
		return nil
	}
	for _, instanceID := range *scope {
		if !selected[instanceID] {
			return timetable.DeviationBadRequest("die Ersatzperson ist nicht für jeden ausgewählten Termin eingetragen")
		}
	}
	return nil
}

func validateExplicitScopeInstances(date timezone.Date, instanceIDs *[]int64, readSet *deviationReadSet) error {
	if instanceIDs == nil {
		return nil
	}
	if len(*instanceIDs) == 0 {
		return timetable.DeviationBadRequest("wählen Sie mindestens einen Termin aus")
	}
	seen := make(map[int64]bool, len(*instanceIDs))
	for _, instanceID := range *instanceIDs {
		if instanceID <= 0 || seen[instanceID] {
			return timetable.DeviationBadRequest("die Terminauswahl ist ungültig")
		}
		seen[instanceID] = true
		if err := validateScopeInstance(date, readSet.instances[instanceID]); err != nil {
			return err
		}
	}
	return nil
}

// validateScopeInstance checks one explicitly selected appointment.
func validateScopeInstance(date timezone.Date, instance *scheduleModel.ActivityInstance) error {
	if instance == nil {
		return timetable.DeviationNotFound(msgInstanceNotFound)
	}
	if timezone.Date(instance.Date) != date {
		return timetable.DeviationBadRequest("alle ausgewählten Termine müssen am bearbeiteten Tag liegen")
	}
	if !isPlannableInstance(instance) {
		return timetable.DeviationConflict("instance_not_editable", msgInstanceNotEditable)
	}
	return nil
}

func inputMarksStaffAbsentInScope(in timetable.ApplyDeviationsInput, staffID int64, targetScope *[]int64) bool {
	for _, absence := range in.Absences {
		if absence.StaffID == staffID && deviationScopesOverlap(absence.InstanceIDs, targetScope) {
			return true
		}
	}
	for _, substitution := range in.Substitutions {
		if substitution.AbsentStaffID == staffID && deviationScopesOverlap(substitution.InstanceIDs, targetScope) {
			return true
		}
	}
	return false
}

func deviationScopesOverlap(left, right *[]int64) bool {
	if left == nil || right == nil {
		return true
	}
	rightIDs := make(map[int64]bool, len(*right))
	for _, instanceID := range *right {
		rightIDs[instanceID] = true
	}
	for _, instanceID := range *left {
		if rightIDs[instanceID] {
			return true
		}
	}
	return false
}

func scopeContainsInstance(instanceIDs *[]int64, instanceID int64) bool {
	if instanceIDs == nil {
		return true
	}
	for _, selectedID := range *instanceIDs {
		if selectedID == instanceID {
			return true
		}
	}
	return false
}

// scopedDayRows validates a change's explicit scope and resolves the
// person's rows it covers.
func scopedDayRows(date timezone.Date, staffID int64, scope *[]int64, readSet *deviationReadSet) ([]*scheduleModel.InstanceStaff, error) {
	if err := validateExplicitScopeInstances(date, scope, readSet); err != nil {
		return nil, err
	}
	return scopedStaffRows(readSet.rowsByStaff[staffID], scope)
}

// planAbsences resolves every absence scope and stages its currently-present
// plannable rows. Explicit scopes reject terminal appointments rather than
// silently widening or partially applying the request.
func planAbsences(absences []timetable.DeviationAbsenceInput, date timezone.Date, readSet *deviationReadSet) ([]deviationAbsenceOp, error) {
	plan := make([]deviationAbsenceOp, 0)
	seenRows := make(map[int64]bool)
	for _, absence := range absences {
		rows, err := scopedDayRows(date, absence.StaffID, absence.InstanceIDs, readSet)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if seenRows[row.ID] || row.IsAbsent {
				continue // idempotent: already absent, no write
			}
			instance, err := loadScopedPlannableInstance(row, absence.InstanceIDs, readSet)
			if err != nil {
				return nil, err
			}
			if instance == nil {
				continue // terminal instance, skip
			}
			seenRows[row.ID] = true
			plan = append(plan, deviationAbsenceOp{row: row, instance: instance, reason: trimDeviationReason(absence.Reason)})
		}
	}
	return plan, nil
}

// planPresences loads every to-be-restored staff member's scoped, plannable
// rows that are currently marked absent. A non-absent row is a no-op.
func planPresences(presences []timetable.DeviationPresenceInput, date timezone.Date, readSet *deviationReadSet) ([]deviationPresenceOp, error) {
	plan := make([]deviationPresenceOp, 0)
	seenRows := make(map[int64]bool)
	for _, presence := range presences {
		rows, err := scopedDayRows(date, presence.StaffID, presence.InstanceIDs, readSet)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			op, err := planPresence(row, presence.InstanceIDs, readSet, seenRows)
			if err != nil {
				return nil, err
			}
			if op != nil {
				plan = append(plan, *op)
			}
		}
	}
	return plan, nil
}

// planPresence stages one row's restore; nil skips a terminal appointment, a
// row seen before, or a row that is not absent.
func planPresence(row *scheduleModel.InstanceStaff, scope *[]int64, readSet *deviationReadSet, seenRows map[int64]bool) (*deviationPresenceOp, error) {
	instance, err := loadScopedPlannableInstance(row, scope, readSet)
	if err != nil || instance == nil {
		return nil, err // nil instance: terminal, skip
	}
	if seenRows[row.ID] || !row.IsAbsent {
		return nil, nil // only a persisted absence can be cleared
	}
	if row.SickAbsenceID != nil {
		return nil, timetable.DeviationConflict("sick_absence_scope_locked", msgSickScopeLocked)
	}
	seenRows[row.ID] = true
	return &deviationPresenceOp{row: row, instance: instance}, nil
}

// substituteStaging tracks which NEW substitute rows are staged per instance
// in THIS request, keyed by (instance, substitute). Only a REPEATED
// (instance, substitute) pairing collapses, or Phase B would insert the same
// row twice (#1840). newByInstance counts the new rows for the ack check.
type substituteStaging struct {
	staged        map[int64]map[int64]bool
	newByInstance map[int64]int
}

// stage returns the action for one classified substitution: the SAME
// substitute staged twice on one block collapses into already_on_instance.
func (st *substituteStaging) stage(instanceID, substituteID int64, action string) string {
	if action != timetable.SubstituteActionSubstituted {
		return action
	}
	staged := st.staged[instanceID]
	if staged == nil {
		staged = make(map[int64]bool)
		st.staged[instanceID] = staged
	}
	if staged[substituteID] {
		return timetable.SubstituteActionAlreadyOnInstance
	}
	staged[substituteID] = true
	st.newByInstance[instanceID]++
	return action
}

// planSubstitutions classifies every substitution against a projected view of
// each instance (absence-only staff read as absent). Returns the write plan and,
// per instance, how many NEW substitute rows will be added (for the ack check).
func planSubstitutions(
	subs []timetable.DeviationSubstitutionInput,
	absenceOnlyByInstance deviationStaffByInstance,
	removedSubstitutes deviationStaffByInstance,
	readSet *deviationReadSet,
) ([]deviationSubOp, map[int64]int, error) {
	plan := make([]deviationSubOp, 0)
	staging := &substituteStaging{staged: make(map[int64]map[int64]bool), newByInstance: make(map[int64]int)}
	for _, sub := range subs {
		origRows, err := scopedStaffRows(readSet.rowsByStaff[sub.AbsentStaffID], sub.InstanceIDs)
		if err != nil {
			return nil, nil, err
		}
		reason := trimDeviationReason(sub.Reason)
		for _, orig := range origRows {
			op, err := planSubstitutionTarget(sub, orig, absenceOnlyByInstance, removedSubstitutes, readSet, staging)
			if err != nil {
				return nil, nil, err
			}
			if op != nil {
				op.reason = reason
				plan = append(plan, *op)
			}
		}
	}
	return plan, staging.newByInstance, nil
}

// planSubstitutionTarget classifies one substitution on one of the absent
// person's rows; nil skips a terminal appointment of a day-wide change.
func planSubstitutionTarget(
	sub timetable.DeviationSubstitutionInput,
	orig *scheduleModel.InstanceStaff,
	absenceOnlyByInstance deviationStaffByInstance,
	removedSubstitutes deviationStaffByInstance,
	readSet *deviationReadSet,
	staging *substituteStaging,
) (*deviationSubOp, error) {
	instance, err := loadScopedPlannableInstance(orig, sub.InstanceIDs, readSet)
	if err != nil || instance == nil {
		return nil, err
	}
	allRows := withoutRemovedSubstitutes(readSet.rowsByInstance[instance.ID], removedSubstitutes[instance.ID])
	projectedRows, origProjected := projectAbsent(allRows, absenceOnlyByInstance[instance.ID], orig)
	action, _, ok := classifySubstitute(projectedRows, origProjected, sub.SubstituteStaffID)
	if !ok {
		return nil, timetable.DeviationConflict("substitute_conflict",
			"dieser Termin hat bereits eine andere Ersatzperson. Entfernen Sie diese zuerst")
	}
	action = staging.stage(instance.ID, sub.SubstituteStaffID, action)
	return &deviationSubOp{
		write: substituteWriteOp{Instance: instance, OrigRow: orig, Action: action},
		subID: sub.SubstituteStaffID,
	}, nil
}

func withoutRemovedSubstitutes(rows []*scheduleModel.InstanceStaff, removed map[int64]bool) []*scheduleModel.InstanceStaff {
	if len(removed) == 0 {
		return rows
	}
	kept := make([]*scheduleModel.InstanceStaff, 0, len(rows))
	for _, row := range rows {
		if row.IsSubstitute && removed[row.StaffID] {
			continue
		}
		kept = append(kept, row)
	}
	return kept
}

// scopedStaffRows resolves a day-wide or explicit appointment scope against
// the absent person's assignments. Explicit scopes fail closed: an empty list,
// duplicate/non-positive id, or an appointment that does not belong to this
// person on this day is rejected instead of broadening to the whole day.
func scopedStaffRows(rows []*scheduleModel.InstanceStaff, instanceIDs *[]int64) ([]*scheduleModel.InstanceStaff, error) {
	if instanceIDs == nil {
		return rows, nil
	}
	if len(*instanceIDs) == 0 {
		return nil, timetable.DeviationBadRequest("wählen Sie mindestens einen Termin aus")
	}

	requested := make(map[int64]bool, len(*instanceIDs))
	for _, instanceID := range *instanceIDs {
		if instanceID <= 0 {
			return nil, timetable.DeviationBadRequest("die Terminauswahl ist ungültig")
		}
		if requested[instanceID] {
			return nil, timetable.DeviationBadRequest("die Terminauswahl enthält einen Termin mehrfach")
		}
		requested[instanceID] = true
	}

	selected := make([]*scheduleModel.InstanceStaff, 0, len(requested))
	for _, row := range rows {
		if requested[row.InstanceID] {
			selected = append(selected, row)
			delete(requested, row.InstanceID)
		}
	}
	if len(requested) > 0 {
		return nil, timetable.DeviationBadRequest("mindestens ein ausgewählter Termin gehört nicht zu dieser Person")
	}
	return selected, nil
}
