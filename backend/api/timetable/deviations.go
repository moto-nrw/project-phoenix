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
	// Presences lists staff to mark present again — clearing a persisted day-wide
	// absence so an admin who marked the wrong person can correct the plan (#1840).
	Presences []int64 `json:"presences,omitempty"`
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
		// Serialize the cancellation against concurrent absence/substitute saves
		// on the same day. The non-cancel path below takes this (tenant, date) day
		// lock before it reads and rewrites staff rows; a cancel that skipped it
		// could commit between another admin's lock acquisition and their writes,
		// leaving a cancelled block with freshly-rewritten staffing — a rewritten
		// historical block. Take the same lock and reload the instance under it so a
		// concurrent move/complete/cancel is seen before we act (#1840).
		if err := rs.TimetableData.AcquireSubstituteDayLock(ctx, instance.Date); err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap("lock day failed", err))
			return
		}
		locked, err := rs.TimetableData.GetActivityInstance(ctx, id)
		if err != nil {
			if base.IsNoRows(err) {
				common.RenderError(w, r, common.ErrorNotFound(errors.New("instance not found")))
				return
			}
			common.RenderError(w, r, common.ErrorInternalServerWrap("reload instance failed", err))
			return
		}
		if locked == nil {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("instance not found")))
			return
		}
		// A concurrent PUT may have MOVED the block to another day between the
		// initial read and this locked reload. The day lock we hold is keyed on the
		// stale date, so cancelling the moved block now would (a) cancel it without
		// holding its real day's lock and (b) break the ascending day-lock ordering
		// the other flows rely on to avoid deadlock. Mirror the non-cancel path and
		// abort with instance_moved so the client reopens it on its new day (#1840).
		if locked.Date != instance.Date {
			common.RenderError(w, r, common.ErrorConflictWithCode(
				errors.New("block was changed concurrently; reopen it and try again"),
				"instance_moved",
			))
			return
		}
		// A concurrent PUT may have moved the block to a past day; cancelling it
		// then would rewrite history. The initial past-date guard above ran against
		// a possibly-stale read, so re-check under the lock before delegating.
		if locked.Date.Before(timezone.TodayDate()) {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("block date is in the past")))
			return
		}
		cancelled, err := rs.InstanceService.Cancel(ctx, id, trimReason(req.CancelReason), resolveActorAccountID(ctx))
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

	// Reject non-positive staff ids on the RAW request, before dedupePositive
	// silently drops them below. Absence and presence ids are otherwise filtered
	// (id <= 0) inside dedupePositive, so {"absences":[{"staff_id":0}]} would be
	// classified as an empty no-op and return 200 instead of the documented
	// positive-id validation error. Substitution ids are already validated in
	// validateDeviationStaff (they are not deduped), so only these two lists need
	// the up-front check (#1840).
	for _, a := range req.Absences {
		if a.StaffID <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("absent staff must be a positive id")))
			return
		}
	}
	for _, pid := range req.Presences {
		if pid <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("present staff must be a positive id")))
			return
		}
	}

	// ==== PHASE A — validate + classify, no writes ====

	// Serialize concurrent saves for the whole (tenant, date) BEFORE any
	// classification read. A substitution/absence is day-wide: it touches every
	// same-day block of the affected staff, not just this one. Locking only this
	// instance would let two admins editing DIFFERENT blocks of the same absent
	// employee both read the pre-write state, classify "no existing substitute",
	// and each insert a substitute row (distinct staff_id slips past the
	// UNIQUE(instance_id, staff_id) constraint), leaving two live supervisors on a
	// shared block. The /substitute route takes the same day lock, so the two
	// endpoints also serialize against each other. The second save blocks until
	// the first commits, then re-reads and classifies the now-present substitute
	// as a conflict. Released automatically when this request's tenant tx
	// commits/rolls back.
	if err := rs.TimetableData.AcquireSubstituteDayLock(ctx, date); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("lock day failed", err))
		return
	}

	// Re-read the instance now that the day is locked. PUT /instances/{id} takes
	// the same (tenant, date) lock, so once we hold it the block's date and rows
	// are stable — but that PUT may have MOVED the block to another day between the
	// initial read above and this lock. Planning against the stale date would mark
	// the old day's rows absent while the moved block is left untouched, then
	// return 200. Detect a move (or a concurrent cancel/complete of this block) and
	// abort so the client reopens it on its new day (#1840).
	locked, err := rs.TimetableData.GetActivityInstance(ctx, id)
	if err != nil {
		if base.IsNoRows(err) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("instance not found")))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("reload instance failed", err))
		return
	}
	if locked == nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("instance not found")))
		return
	}
	if locked.Date != date || !isPlannableInstance(locked) {
		common.RenderError(w, r, common.ErrorConflictWithCode(
			errors.New("block was changed concurrently; reopen it and try again"),
			"instance_moved",
		))
		return
	}
	instance = locked

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

	// presences: staff to mark present again (clear a persisted day-wide absence,
	// #1840). Deduped. The same person cannot be both cleared and (re)marked
	// absent in one save — validateDeviationStaff rejects the contradiction.
	presences := dedupePositive(req.Presences)
	presenceSet := toSet(presences)

	if err := rs.validateDeviationStaff(ctx, req, absenceOnlySet, fullAbsent, presenceSet, date); err != nil {
		common.RenderError(w, r, err)
		return
	}

	absencePlan, rndr := rs.planAbsences(ctx, absenceOnly, date)
	if rndr != nil {
		common.RenderError(w, r, rndr)
		return
	}

	presencePlan, rndr := rs.planPresences(ctx, presences, date)
	if rndr != nil {
		common.RenderError(w, r, rndr)
		return
	}

	subPlan, newSubByInstance, rndr := rs.planSubstitutions(ctx, req.Substitutions, absenceOnlySet, date)
	if rndr != nil {
		common.RenderError(w, r, rndr)
		return
	}

	// Restoring a persisted absence (a "presence") must not leave a block
	// over-staffed. There is no explicit substitute→absent link in the data model,
	// so clearing an absence after a replacement was already assigned would orphan
	// that replacement: e.g. a planned person is covered by substitute Y on a prior
	// save, then in this save the planned person — or a previously-removed
	// substitute — is restored to present while Y stays active. The block then has
	// more people present than it has planned positions, with no UI path telling
	// which substitute is now redundant. In a consistent state present count never
	// exceeds the planned baseline (each substitute fills exactly one absent planned
	// position), so present > baseline is a reliable orphan detector. Check every
	// instance a presence touches against its projected post-save coverage and
	// reject before any write, so the admin removes the stale substitute first
	// rather than silently double-staffing the block (#1840).
	if rndr := rs.rejectOverstaffingPresences(ctx, presencePlan, fullAbsent, presenceSet, newSubByInstance); rndr != nil {
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
	projectedPresent := projectedNonAbsentCount(thisRows, fullAbsent, presenceSet, newSubByInstance[id])
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

	// ==== PHASE B — apply writes (service layer, protokolliert, #1886) ====
	now := time.Now()
	actor := resolveActorAccountID(ctx)
	affected := make([]AffectedInstance, 0, len(absencePlan)+len(subPlan)+len(presencePlan))
	activeTouched := make(map[int64]*scheduleModel.ActivityInstance)

	for _, op := range presencePlan {
		if err := rs.InstanceService.ApplyPresence(ctx, op.row, op.instance, actor, activeTouched); err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap("clear absence failed", err))
			return
		}
		affected = append(affected, affectedInstanceOf(op.instance, substituteActionMarkedPresent))
	}
	for _, op := range absencePlan {
		if err := rs.InstanceService.ApplyAbsence(ctx, op.row, op.instance, absenceReason[op.row.StaffID], actor, activeTouched); err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap("mark absent failed", err))
			return
		}
		affected = append(affected, affectedInstanceOf(op.instance, substituteActionMarkedAbsent))
	}
	for _, op := range subPlan {
		if err := rs.InstanceService.ApplySubstitute(ctx, op.op.writeOp(), op.subID, op.reason, now, actor, activeTouched); err != nil {
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
		if _, err := rs.InstanceService.SetUnderstaffedAck(ctx, id, finalAck, note, actor); err != nil {
			// A concurrent cancel or full-staffing of THIS instance between Phase A
			// and here makes SetUnderstaffedAck return a 4xx after the absence and
			// substitute writes above already succeeded. TenantTxMiddleware commits
			// non-5xx responses unless we ask otherwise, so without this the
			// endpoint would report a failed atomic save while committing those
			// earlier writes. Force the whole tx to roll back before rendering.
			tenant.MarkRollback(ctx)
			renderInstanceLifecycleError(w, r, err)
			return
		}
	}

	// Reconcile stale acknowledgements on the OTHER instances this save covered.
	// When the absent person had several same-day blocks, the substitute was
	// added to each; any of those that carried a "deliberately unstaffed"
	// acknowledgement and is now fully staffed must have that stale ack cleared,
	// or it stays amber while /gaps counts it staffed and its ack checkbox is
	// disabled (fully staffed), leaving no UI path to clear it (#1840). The
	// selected block's ack is handled explicitly above, so skip it here. Mirrors
	// /substitute's clearAck pass; runs in the same tenant tx and only ever
	// clears (a DB error renders 500 → the middleware rolls back).
	clearAck := make(map[int64]bool)
	for _, op := range subPlan {
		if op.op.instance.ID == id {
			continue
		}
		if op.op.instance.UnderstaffedAck &&
			(op.op.action == substituteActionSubstituted || op.op.action == substituteActionAlreadyOnInstance) {
			clearAck[op.op.instance.ID] = true
		}
	}
	// Restoring a staff member on their OTHER same-day blocks can likewise lift
	// one out of understaffing → clear its now-stale acknowledgement (#1840).
	for _, op := range presencePlan {
		if op.instance.ID == id {
			continue
		}
		if op.instance.UnderstaffedAck {
			clearAck[op.instance.ID] = true
		}
	}
	for instanceID := range clearAck {
		if err := rs.InstanceService.ClearUnderstaffedAckIfStaffed(ctx, instanceID, actor); err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap("clear stale understaffed ack failed", err))
			return
		}
	}

	warnings := rs.collectDeviationWarnings(ctx, req.Substitutions, subPlan, date)

	rs.InstanceService.QueueActivityUpdates(ctx, activeTouched)

	rs.getLogger().Info("deviations applied",
		slog.Int64("instance_id", id),
		slog.Int("absences", len(absencePlan)),
		slog.Int("presences", len(presencePlan)),
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
	absenceOnlySet, fullAbsent, presenceSet map[int64]bool,
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
	for id := range presenceSet {
		// A person cannot be marked present and absent (or substituted-away) in the
		// same save — the two are contradictory day-wide states.
		if fullAbsent[id] {
			return common.ErrorInvalidRequest(errors.New("staff cannot be marked present and absent in the same request"))
		}
		if rndr := ensure(id, "present staff"); rndr != nil {
			return rndr
		}
	}
	seenAbsentSub := make(map[int64]bool)
	for _, sub := range req.Substitutions {
		// Each absent person's substitution is day-wide (it covers ALL their
		// same-day blocks), so an absent staff id must appear at most once. Two
		// substitutions for the same absent person with DIFFERENT substitutes would
		// both classify against the same pre-write row and both insert a substitute
		// row in Phase B — the staging map only dedupes (instance, substitute), so
		// one planned position would end up with two active replacements. Reject the
		// contradiction here (#1840).
		if seenAbsentSub[sub.AbsentStaffID] {
			return common.ErrorInvalidRequest(errors.New("each absent staff member may have at most one substitute per request"))
		}
		seenAbsentSub[sub.AbsentStaffID] = true
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

// presenceOp pairs a currently-absent planned row that should be cleared with
// its instance, ready for the Phase-B write.
type presenceOp struct {
	row      *scheduleModel.InstanceStaff
	instance *scheduleModel.ActivityInstance
}

// planPresences loads every to-be-restored staff member's plannable same-day
// rows that are currently marked absent (day-wide clear, #1840). This clears BOTH
// a wrongly-marked planned absence and a removed substitute's absence: "Entfernen"
// marks a substitute absent day-wide, and the inverse must bring that substitute
// back so an accidental removal is correctable without a DB edit. A non-absent row
// is a no-op.
func (rs *Resource) planPresences(ctx context.Context, presentStaffIDs []int64, date timezone.Date) ([]presenceOp, render.Renderer) {
	plan := make([]presenceOp, 0)
	for _, staffID := range presentStaffIDs {
		rows, err := rs.TimetableData.GetInstanceStaffByStaffAndDate(ctx, staffID, date)
		if err != nil {
			return nil, common.ErrorInternalServerWrap("load present assignments failed", err)
		}
		for _, row := range rows {
			if !row.IsAbsent {
				continue // only a persisted absence (planned or substitute) can be cleared
			}
			instance, rndr := rs.loadPlannableInstance(ctx, row)
			if rndr != nil {
				return nil, rndr
			}
			if instance == nil {
				continue // terminal instance, skip
			}
			plan = append(plan, presenceOp{row: row, instance: instance})
		}
	}
	return plan, nil
}

// rejectOverstaffingPresences returns a 409 when clearing a persisted absence
// would push any touched instance above its planned headcount, which only
// happens when a restore orphans an already-assigned substitute (#1840). It
// reloads each affected instance's current rows and projects the full post-save
// coverage (day-wide absences applied, presences cleared, new substitutes added)
// against the planned (non-substitute) baseline. Read-only: runs in Phase A so a
// rejection never commits partial state.
func (rs *Resource) rejectOverstaffingPresences(
	ctx context.Context,
	presencePlan []presenceOp,
	fullAbsent, presenceSet map[int64]bool,
	newSubByInstance map[int64]int,
) render.Renderer {
	checked := make(map[int64]bool)
	for _, op := range presencePlan {
		if checked[op.instance.ID] {
			continue
		}
		checked[op.instance.ID] = true
		rows, err := rs.TimetableData.GetInstanceStaff(ctx, op.instance.ID)
		if err != nil {
			return common.ErrorInternalServerWrap("load instance staff failed", err)
		}
		baseline := 0
		for _, row := range rows {
			if !row.IsSubstitute {
				baseline++
			}
		}
		if projectedNonAbsentCount(rows, fullAbsent, presenceSet, newSubByInstance[op.instance.ID]) > baseline {
			return common.ErrorConflictWithCode(
				errors.New("das Wiederherstellen dieser Anwesenheit würde den Block überbesetzen; bitte zuerst die nicht mehr benötigte Vertretung entfernen"),
				"presence_would_overstaff",
			)
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
	// Track which NEW substitute rows are staged per instance in THIS request,
	// keyed by (instance, substitute) — NOT just instance. A block with several
	// absent planned positions legitimately gets several DISTINCT substitutes: the
	// only DB constraint is UNIQUE(instance_id, staff_id), and the sequential
	// /substitute path already produces exactly that. Keying on the instance alone
	// rejected such a valid multi-position save with a 409, so the accepted result
	// depended on how the edits were grouped (#1840). Only a REPEATED
	// (instance, substitute) pairing must be collapsed, or Phase B would insert the
	// same row twice and hit the unique constraint.
	stagedSubs := make(map[int64]map[int64]bool)

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
					fmt.Errorf("instance %d is already fully covered by another substitute (staff_id=%d); remove a replacement before adding one",
						instance.ID, conflictOther),
					"substitute_conflict",
				)
			}
			// Two absent colleagues on one block both classify "substituted"
			// against the pre-write rows. Distinct substitutes are fine (two new
			// rows, two absent positions). But the SAME substitute staged twice on
			// one block would insert the same (instance_id, staff_id) twice in
			// Phase B — a unique-constraint 500. Collapse the repeat into the
			// already-on-instance action (mark the absent's row, insert nothing),
			// exactly how the sequential /substitute path classifies a repeat, so
			// the single covering row still lands (#1840).
			if action == substituteActionSubstituted {
				staged := stagedSubs[instance.ID]
				if staged == nil {
					staged = make(map[int64]bool)
					stagedSubs[instance.ID] = staged
				}
				if staged[sub.SubstituteStaffID] {
					action = substituteActionAlreadyOnInstance
				} else {
					staged[sub.SubstituteStaffID] = true
					newSubByInstance[instance.ID]++
				}
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
	// One substitute can cover several absent colleagues, which duplicates
	// conflicts two ways (#1840):
	//   1. the same SubstituteStaffID appears in multiple `subs` rows, and the
	//      ops loop below already gathers EVERY op for that id — reprocessing a
	//      repeated id rebuilds the identical conflicts; and
	//   2. planSubstitutions stages two ops on ONE block for that substitute
	//      (one "substituted", one "already_on_instance"), and
	//      buildSubstituteTimeConflicts keeps BOTH as targets, so it emits the
	//      same (instance, other) conflict twice within a single pass.
	// Skip repeated substitute ids (1) and drop duplicate (substitute, instance,
	// other) conflict triples (2) so each real overlap surfaces exactly once. The
	// triple keeps genuinely distinct substitutes colliding on the same block-pair
	// separate.
	seenSub := make(map[int64]bool, len(subs))
	seenConflict := make(map[[3]int64]bool)
	for _, sub := range subs {
		if seenSub[sub.SubstituteStaffID] {
			continue
		}
		seenSub[sub.SubstituteStaffID] = true
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
		for _, conflict := range w {
			key := [3]int64{sub.SubstituteStaffID, conflict.InstanceID, conflict.OtherID}
			if seenConflict[key] {
				continue
			}
			seenConflict[key] = true
			warnings = append(warnings, conflict)
		}
	}
	return warnings
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
// marked absent, plus rows whose persisted absence is being cleared (presence),
// plus each newly-added substitute row.
func projectedNonAbsentCount(rows []*scheduleModel.InstanceStaff, fullAbsent, presence map[int64]bool, newSubs int) int {
	count := 0
	for _, row := range rows {
		if fullAbsent[row.StaffID] {
			continue
		}
		// Present if not currently absent, or its persisted absence is being cleared.
		if row.IsAbsent && !presence[row.StaffID] {
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
