// Package timetable — atomic Vertretungsplan save (#1840).
//
//	POST /api/timetable/instances/{id}/deviations
//
// The Vertretungsplan slide-over lets an admin, in one Save, mark people absent,
// assign a substitute, swap the current substitute for another, flip the
// "deliberately unstaffed" acknowledgement, and/or cancel the block. Previously
// the frontend fired one HTTP request per edit — each its own tenant transaction
// — so a later failure left earlier edits committed (partial state). This
// endpoint applies the WHOLE form in the single request transaction so it either
// all lands or all rolls back.
//
// Atomicity note (same as /substitute): TenantTxMiddleware rolls the request tx
// back only on 5xx. A 409 rendered mid-handler would commit prior writes. We use
// the same strict two-phase flow:
//
//	Phase A (Dry-Run): validate + classify everything, no writes. Any 4xx aborts
//	                   here, before a single row changed.
//	Phase B (Apply):   only reached when Phase A validated cleanly. All writes
//	                   happen here, so a 409 never commits partial state.
//
// Permission: SchedulesManage. Same tenant tx as the other /instances routes.
package timetable

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// deviationAbsence marks one staff member absent day-wide with no substitute
// (a planned person left open, or a removed substitute).
type deviationAbsence struct {
	StaffID int64   `json:"staff_id"`
	Reason  *string `json:"reason,omitempty"`
}

// deviationSubstitution assigns substituteStaffID to cover absentStaffID day-wide.
type deviationSubstitution struct {
	AbsentStaffID     int64   `json:"absent_staff_id"`
	SubstituteStaffID int64   `json:"substitute_staff_id"`
	Reason            *string `json:"reason,omitempty"`
}

// applyDeviationsRequest is the POST body. All mutation fields are optional; a
// Cancel request is exclusive (other fields are ignored, mirroring the UI where
// "Block absagen" disables everything else). understaffed_ack is a pointer so an
// omitted field ("no change") is distinguishable from an explicit false ("clear").
type applyDeviationsRequest struct {
	Cancel           bool                    `json:"cancel"`
	CancelReason     *string                 `json:"cancel_reason,omitempty"`
	UnderstaffedAck  *bool                   `json:"understaffed_ack,omitempty"`
	UnderstaffedNote *string                 `json:"understaffed_note,omitempty"`
	Absences         []deviationAbsence      `json:"absences,omitempty"`
	Substitutions    []deviationSubstitution `json:"substitutions,omitempty"`
}

// ApplyDeviationsResponse is the 200 body.
type ApplyDeviationsResponse struct {
	InstanceID        int64                                `json:"instance_id"`
	Cancelled         bool                                 `json:"cancelled"`
	UnderstaffedAck   bool                                 `json:"understaffed_ack"`
	AffectedInstances []AffectedInstance                   `json:"affected_instances"`
	Warnings          []scheduleSvc.SubstituteTimeConflict `json:"warnings"`
}

// applyDeviations handles POST /api/timetable/instances/{id}/deviations.
func (rs *Resource) applyDeviations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance id")))
		return
	}
	if rs.TimetableData == nil || rs.PersonService == nil || rs.InstanceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}

	var req applyDeviationsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid JSON body")))
		return
	}

	instance, err := rs.TimetableData.GetActivityInstance(ctx, id)
	if err != nil {
		// FindByID wraps sql.ErrNoRows in a DatabaseError (it never returns
		// (nil, nil)), so a stale link or deleted/other-tenant instance must be
		// mapped to 404 here rather than falling through to a 500.
		if base.IsNoRows(err) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("instance not found")))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("load instance failed", err))
		return
	}
	if instance == nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("instance not found")))
		return
	}

	// Past blocks are historical record; no deviation — including a
	// cancellation — may rewrite them. Guard here, before the exclusive cancel
	// branch, so cancelling a past block is rejected exactly like every other
	// deviation on it (the page can browse past weeks). Mirrors /substitute and
	// /gaps.
	if instance.Date.Before(timezone.TodayDate()) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("block date is in the past")))
		return
	}

	// Cancel is exclusive: the shared Cancel service both validates the
	// transition and ends any active bridge. Nothing else is applied.
	if req.Cancel {
		cancelled, err := rs.InstanceService.Cancel(ctx, id, trimReason(req.CancelReason))
		if err != nil {
			renderInstanceLifecycleError(w, r, err)
			return
		}
		common.Respond(w, r, http.StatusOK, ApplyDeviationsResponse{
			InstanceID:        cancelled.ID,
			Cancelled:         true,
			UnderstaffedAck:   cancelled.UnderstaffedAck,
			AffectedInstances: []AffectedInstance{},
			Warnings:          []scheduleSvc.SubstituteTimeConflict{},
		}, "Block cancelled")
		return
	}

	// Only planned/active blocks are editable; completed/cancelled are history.
	if !isPlannableInstance(instance) {
		common.RenderError(w, r, common.ErrorConflictWithCode(
			fmt.Errorf("cannot edit instance in status %q", instance.Status),
			"invalid_transition",
		))
		return
	}
	// The block's own date is the day-wide scope of every absence/substitute.
	// The past-date guard already ran above (before the cancel branch).
	date := instance.Date
	if req.UnderstaffedNote != nil && utf8.RuneCountInString(*req.UnderstaffedNote) > understaffedAckNoteMaxLength {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("note is too long")))
		return
	}

	// ==== PHASE A — validate + classify, no writes ====

	// absenceOnly staff: marked absent with no substitute (planned-open + removed
	// substitutes). Deduped. These project onto substitute classification below so
	// that removing the current substitute and picking another in the same save
	// does not 409 (the old substitute reads as absent, freeing the block).
	absenceOnly := dedupePositive(func() []int64 {
		ids := make([]int64, 0, len(req.Absences))
		for _, a := range req.Absences {
			ids = append(ids, a.StaffID)
		}
		return ids
	}())
	absenceReason := make(map[int64]*string, len(req.Absences))
	for _, a := range req.Absences {
		absenceReason[a.StaffID] = trimReason(a.Reason)
	}
	absenceOnlySet := toSet(absenceOnly)

	// fullAbsent = absence-only staff plus every substituted person. Used for the
	// "still staffed?" acknowledgement check on this instance.
	fullAbsent := toSet(absenceOnly)
	for _, sub := range req.Substitutions {
		fullAbsent[sub.AbsentStaffID] = true
	}

	if err := rs.validateDeviationStaff(ctx, req, absenceOnlySet, fullAbsent, date); err != nil {
		common.RenderError(w, r, err)
		return
	}

	absencePlan, rndr := rs.planAbsences(ctx, absenceOnly, date)
	if rndr != nil {
		common.RenderError(w, r, rndr)
		return
	}

	subPlan, newSubByInstance, rndr := rs.planSubstitutions(ctx, req.Substitutions, absenceOnlySet, date)
	if rndr != nil {
		common.RenderError(w, r, rndr)
		return
	}

	// Acknowledgement reconciliation. "Deliberately unstaffed" is valid whenever
	// the block ends up understaffed after the save — nobody present, or fewer
	// present than planned (#1840: a single position may be left unfilled while
	// other staff remain). Compute the projected coverage vs the planned baseline
	// on THIS instance and decide the final flag:
	//   - client sent understaffed_ack → honour it (reject ack=true only if the
	//     block is fully staffed after the save)
	//   - client sent nothing but the block was acked and is now fully staffed →
	//     clear the stale acknowledgement so /gaps and the amber card cannot
	//     contradict.
	// The deviation writes never change the planned baseline: marking someone
	// absent keeps is_substitute, and an added substitute is is_substitute=true
	// (not a planned position), so the baseline is the current non-substitute
	// count of thisRows.
	thisRows, err := rs.TimetableData.GetInstanceStaff(ctx, id)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load instance staff failed", err))
		return
	}
	projectedPresent := projectedNonAbsentCount(thisRows, fullAbsent, newSubByInstance[id])
	plannedBaseline := 0
	for _, row := range thisRows {
		if !row.IsSubstitute {
			plannedBaseline++
		}
	}
	projectedUnderstaffed := scheduleSvc.IsUnderstaffedCounts(projectedPresent, plannedBaseline)

	finalAck := instance.UnderstaffedAck
	ackChanged := false
	if req.UnderstaffedAck != nil {
		if *req.UnderstaffedAck && !projectedUnderstaffed {
			common.RenderError(w, r, common.ErrorConflictWithCode(
				errors.New("dieser Block kann nicht als bewusst unbesetzt markiert werden, solange er vollständig besetzt ist"),
				"understaffed_still_staffed",
			))
			return
		}
		finalAck = *req.UnderstaffedAck
		ackChanged = finalAck != instance.UnderstaffedAck ||
			(finalAck && !sameNote(instance.UnderstaffedNote, req.UnderstaffedNote))
	} else if instance.UnderstaffedAck && !projectedUnderstaffed {
		finalAck = false
		ackChanged = true
	}

	// ==== PHASE B — apply writes ====
	now := time.Now()
	affected := make([]AffectedInstance, 0, len(absencePlan)+len(subPlan))
	activeTouched := make(map[int64]*scheduleModel.ActivityInstance)

	for _, op := range absencePlan {
		if err := rs.applyAbsenceWrite(ctx, op.row, op.instance, absenceReason[op.row.StaffID], activeTouched); err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap("mark absent failed", err))
			return
		}
		affected = append(affected, affectedInstanceOf(op.instance, substituteActionMarkedAbsent))
	}
	for _, op := range subPlan {
		if err := rs.applySubstituteWrite(ctx, op.op, op.subID, op.reason, now, activeTouched); err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap("assign substitute failed", err))
			return
		}
		affected = append(affected, affectedInstanceOf(op.op.instance, op.op.action))
	}

	if ackChanged {
		var note *string
		if finalAck {
			note = trimReason(req.UnderstaffedNote)
		}
		if _, err := rs.InstanceService.SetUnderstaffedAck(ctx, id, finalAck, note); err != nil {
			renderInstanceLifecycleError(w, r, err)
			return
		}
	}

	warnings := rs.collectDeviationWarnings(ctx, req.Substitutions, subPlan, date)

	rs.broadcastSubstituteEvents(ctx, activeTouched)

	rs.getLogger().Info("deviations applied",
		slog.Int64("instance_id", id),
		slog.Int("absences", len(absencePlan)),
		slog.Int("substitutions", len(subPlan)),
		slog.Bool("understaffed_ack", finalAck),
	)

	common.Respond(w, r, http.StatusOK, ApplyDeviationsResponse{
		InstanceID:        id,
		Cancelled:         false,
		UnderstaffedAck:   finalAck,
		AffectedInstances: affected,
		Warnings:          warnings,
	}, "Deviations applied")
}

// absenceOp pairs a plannable instance row of an absent staff member with its
// instance, ready for the Phase-B write.
type absenceOp struct {
	row      *scheduleModel.InstanceStaff
	instance *scheduleModel.ActivityInstance
}

// subOp tags a classified substitution op with the substitute id + trimmed reason.
type subOp struct {
	op     plannedOp
	subID  int64
	reason *string
}

// validateDeviationStaff runs every 4xx precondition on the referenced staff:
// existence (404), self-substitution (400), a substitute who is also being
// marked absent (400), and a substitute already absent in the DB that day (400).
func (rs *Resource) validateDeviationStaff(
	ctx context.Context,
	req applyDeviationsRequest,
	absenceOnlySet, fullAbsent map[int64]bool,
	date timezone.Date,
) render.Renderer {
	seen := make(map[int64]bool)
	ensure := func(staffID int64, label string) render.Renderer {
		if staffID <= 0 {
			return common.ErrorInvalidRequest(fmt.Errorf("%s must be a positive id", label))
		}
		if seen[staffID] {
			return nil
		}
		seen[staffID] = true
		staff, err := rs.PersonService.GetStaffByID(ctx, staffID)
		if err != nil {
			if base.IsNoRows(err) {
				return common.ErrorNotFound(fmt.Errorf("%s not found", label))
			}
			return common.ErrorInternalServerWrap("load staff failed", err)
		}
		if staff == nil || staff.ID == 0 {
			return common.ErrorNotFound(fmt.Errorf("%s not found", label))
		}
		return nil
	}

	for id := range absenceOnlySet {
		if rndr := ensure(id, "absent staff"); rndr != nil {
			return rndr
		}
	}
	for _, sub := range req.Substitutions {
		if rndr := ensure(sub.AbsentStaffID, "absent staff"); rndr != nil {
			return rndr
		}
		if rndr := ensure(sub.SubstituteStaffID, "substitute staff"); rndr != nil {
			return rndr
		}
		if sub.AbsentStaffID == sub.SubstituteStaffID {
			return common.ErrorInvalidRequest(errors.New("absent and substitute staff must differ"))
		}
		// A person being marked absent (anywhere in this form) cannot also cover a
		// shift — absence is day-wide.
		if fullAbsent[sub.SubstituteStaffID] {
			return common.ErrorInvalidRequest(errors.New("substitute is marked absent in this request"))
		}
		// ...nor if they are already absent that day in the DB.
		subRows, err := rs.TimetableData.GetInstanceStaffByStaffAndDate(ctx, sub.SubstituteStaffID, date)
		if err != nil {
			return common.ErrorInternalServerWrap("load substitute assignments failed", err)
		}
		for _, row := range subRows {
			if row.IsAbsent {
				return common.ErrorInvalidRequest(errors.New("substitute is marked absent on this date"))
			}
		}
	}
	return nil
}

// planAbsences loads every absent-only staff member's plannable same-day rows.
func (rs *Resource) planAbsences(ctx context.Context, absentStaffIDs []int64, date timezone.Date) ([]absenceOp, render.Renderer) {
	plan := make([]absenceOp, 0)
	for _, staffID := range absentStaffIDs {
		rows, err := rs.TimetableData.GetInstanceStaffByStaffAndDate(ctx, staffID, date)
		if err != nil {
			return nil, common.ErrorInternalServerWrap("load absent assignments failed", err)
		}
		for _, row := range rows {
			if row.IsAbsent {
				continue // idempotent: already absent, no write
			}
			instance, rndr := rs.loadPlannableInstance(ctx, row)
			if rndr != nil {
				return nil, rndr
			}
			if instance == nil {
				continue // terminal instance, skip
			}
			plan = append(plan, absenceOp{row: row, instance: instance})
		}
	}
	return plan, nil
}

// planSubstitutions classifies every substitution against a projected view of
// each instance (absence-only staff read as absent). Returns the write plan and,
// per instance, how many NEW substitute rows will be added (for the ack check).
func (rs *Resource) planSubstitutions(
	ctx context.Context,
	subs []deviationSubstitution,
	absenceOnlySet map[int64]bool,
	date timezone.Date,
) ([]subOp, map[int64]int, render.Renderer) {
	plan := make([]subOp, 0)
	newSubByInstance := make(map[int64]int)
	// Guard: the data model allows only one substitute per instance. Track the
	// substitute planned per instance so two substitutions in one form cannot
	// both target the same block.
	subByInstance := make(map[int64]int64)

	for _, sub := range subs {
		origRows, err := rs.TimetableData.GetInstanceStaffByStaffAndDate(ctx, sub.AbsentStaffID, date)
		if err != nil {
			return nil, nil, common.ErrorInternalServerWrap("load absent assignments failed", err)
		}
		reason := trimReason(sub.Reason)
		for _, orig := range origRows {
			instance, rndr := rs.loadPlannableInstance(ctx, orig)
			if rndr != nil {
				return nil, nil, rndr
			}
			if instance == nil {
				continue
			}
			allRows, err := rs.TimetableData.GetInstanceStaff(ctx, instance.ID)
			if err != nil {
				return nil, nil, common.ErrorInternalServerWrap("load instance staff failed", err)
			}
			projectedRows, origProjected := projectAbsent(allRows, absenceOnlySet, orig)
			action, conflictOther, ok := classifySubstitute(projectedRows, origProjected, sub.SubstituteStaffID)
			if !ok {
				return nil, nil, common.ErrorConflictWithCode(
					fmt.Errorf("instance %d has a different substitute already assigned (staff_id=%d); remove the existing substitute first",
						instance.ID, conflictOther),
					"substitute_conflict",
				)
			}
			if existing, taken := subByInstance[instance.ID]; taken && existing != sub.SubstituteStaffID {
				return nil, nil, common.ErrorConflictWithCode(
					fmt.Errorf("instance %d already has a substitute staged in this request", instance.ID),
					"substitute_conflict",
				)
			}
			if action == substituteActionSubstituted {
				subByInstance[instance.ID] = sub.SubstituteStaffID
				newSubByInstance[instance.ID]++
			}
			plan = append(plan, subOp{
				op:     plannedOp{instance: instance, origRow: orig, action: action},
				subID:  sub.SubstituteStaffID,
				reason: reason,
			})
		}
	}
	return plan, newSubByInstance, nil
}

// loadPlannableInstance loads the instance behind a staff row, returning nil when
// it is terminal (completed/cancelled) so callers skip it.
func (rs *Resource) loadPlannableInstance(ctx context.Context, row *scheduleModel.InstanceStaff) (*scheduleModel.ActivityInstance, render.Renderer) {
	instance, err := rs.TimetableData.GetActivityInstance(ctx, row.InstanceID)
	if err != nil {
		return nil, common.ErrorInternalServerWrap("load target instance failed", err)
	}
	if instance == nil {
		return nil, common.ErrorInternalServer(
			fmt.Errorf("instance_staff %d references missing instance %d", row.ID, row.InstanceID))
	}
	if !isPlannableInstance(instance) {
		return nil, nil
	}
	return instance, nil
}

// collectDeviationWarnings merges per-substitute time-conflict advisories. Best
// effort: a lookup failure logs and yields an empty list (never blocks a save).
func (rs *Resource) collectDeviationWarnings(
	ctx context.Context,
	subs []deviationSubstitution,
	plan []subOp,
	date timezone.Date,
) []scheduleSvc.SubstituteTimeConflict {
	warnings := make([]scheduleSvc.SubstituteTimeConflict, 0)
	for _, sub := range subs {
		ops := make([]plannedOp, 0, len(plan))
		for _, p := range plan {
			if p.subID == sub.SubstituteStaffID {
				ops = append(ops, p.op)
			}
		}
		w, err := rs.buildSubstituteTimeConflicts(ctx, ops, sub.SubstituteStaffID, date)
		if err != nil {
			rs.getLogger().Warn("deviation time-conflict detection failed",
				slog.Int64("substitute_staff_id", sub.SubstituteStaffID),
				slog.String("error", err.Error()),
			)
			continue
		}
		warnings = append(warnings, w...)
	}
	return warnings
}

// applyAbsenceWrite marks one staff row absent (day-wide semantics; the caller
// loops over all same-day rows) and, for an active instance, ends the absent
// supervisor. Shared by /substitute's absent-only path and /deviations.
func (rs *Resource) applyAbsenceWrite(
	ctx context.Context,
	row *scheduleModel.InstanceStaff,
	instance *scheduleModel.ActivityInstance,
	reason *string,
	activeTouched map[int64]*scheduleModel.ActivityInstance,
) error {
	row.IsAbsent = true
	row.AbsenceReason = reason
	if err := rs.TimetableData.UpdateInstanceStaff(ctx, row); err != nil {
		return err
	}
	if instance.Status == scheduleModel.InstanceStatusActive && instance.ActiveGroupID != nil {
		if _, err := rs.TimetableData.EndGroupSupervisor(ctx, *instance.ActiveGroupID, row.StaffID); err != nil {
			return err
		}
		activeTouched[*instance.ActiveGroupID] = instance
	}
	return nil
}

// applySubstituteWrite performs the Phase-B write for one classified substitution
// op. Shared by /substitute and /deviations so the two paths cannot diverge.
func (rs *Resource) applySubstituteWrite(
	ctx context.Context,
	op plannedOp,
	subID int64,
	reason *string,
	now time.Time,
	activeTouched map[int64]*scheduleModel.ActivityInstance,
) error {
	switch op.action {
	case substituteActionAlreadySubstitute:
		// No writes: the substitute already covers this instance.
		return nil

	case substituteActionAlreadyOnInstance:
		// Mark only the absent's original row; the substitute is already a
		// (planned, non-substitute) co-supervisor and is left untouched so
		// reports keep planned co-cover distinct from a Vertretung.
		op.origRow.IsAbsent = true
		if reason != nil {
			op.origRow.AbsenceReason = reason
		}
		if err := rs.TimetableData.UpdateInstanceStaff(ctx, op.origRow); err != nil {
			return err
		}
		if op.instance.Status == scheduleModel.InstanceStatusActive && op.instance.ActiveGroupID != nil {
			if _, err := rs.TimetableData.EndGroupSupervisor(ctx, *op.instance.ActiveGroupID, op.origRow.StaffID); err != nil {
				return err
			}
			activeTouched[*op.instance.ActiveGroupID] = op.instance
		}
		return nil

	case substituteActionSubstituted:
		op.origRow.IsAbsent = true
		if reason != nil {
			op.origRow.AbsenceReason = reason
		}
		if err := rs.TimetableData.UpdateInstanceStaff(ctx, op.origRow); err != nil {
			return err
		}
		newRow := &scheduleModel.InstanceStaff{
			InstanceID:   op.instance.ID,
			StaffID:      subID,
			RoomID:       op.origRow.RoomID, // inherit room split, if any
			IsPrimary:    false,
			IsSubstitute: true,
			IsAbsent:     false,
		}
		if err := rs.TimetableData.CreateInstanceStaff(ctx, newRow); err != nil {
			return err
		}
		if op.instance.Status == scheduleModel.InstanceStatusActive && op.instance.ActiveGroupID != nil {
			if _, err := rs.TimetableData.EndGroupSupervisor(ctx, *op.instance.ActiveGroupID, op.origRow.StaffID); err != nil {
				return err
			}
			newSup := &activeModel.GroupSupervisor{
				StaffID:   subID,
				GroupID:   *op.instance.ActiveGroupID,
				Role:      "supervisor",
				StartDate: timezone.DateFromTime(now),
			}
			newSup.SetTenantID(tenant.FromContext(ctx))
			if err := rs.TimetableData.CreateGroupSupervisor(ctx, newSup); err != nil {
				return err
			}
			activeTouched[*op.instance.ActiveGroupID] = op.instance
		}
		return nil
	}
	return nil
}

// affectedInstanceOf builds an AffectedInstance response row.
func affectedInstanceOf(inst *scheduleModel.ActivityInstance, action string) AffectedInstance {
	return AffectedInstance{
		InstanceID: inst.ID,
		Title:      inst.Title,
		StartTime:  inst.StartTime.Format("15:04"),
		Action:     action,
	}
}

// projectAbsent returns a shallow-copied view of rows with the given staff ids
// forced absent, plus the copy of origRow, so classifySubstitute reads a removed
// substitute as no longer covering. The originals are never mutated.
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
		// origRow was not among the instance's rows (should not happen); fall back
		// to a non-mutated copy so classification still runs.
		clone := *origRow
		origProjected = &clone
	}
	return projected, origProjected
}

// projectedNonAbsentCount counts staff that remain non-absent on an instance
// after the deviation writes: existing non-absent rows whose staff is not being
// marked absent, plus each newly-added substitute row.
func projectedNonAbsentCount(rows []*scheduleModel.InstanceStaff, fullAbsent map[int64]bool, newSubs int) int {
	count := 0
	for _, row := range rows {
		if row.IsAbsent || fullAbsent[row.StaffID] {
			continue
		}
		count++
	}
	return count + newSubs
}

// dedupePositive returns the unique positive ids in order.
func dedupePositive(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func toSet(ids []int64) map[int64]bool {
	set := make(map[int64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// sameNote reports whether two optional notes carry the same trimmed text.
func sameNote(a, b *string) bool {
	av := ""
	if a != nil {
		av = *a
	}
	bv := ""
	if b != nil {
		bv = *b
	}
	return av == bv
}
