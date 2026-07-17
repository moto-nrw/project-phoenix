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
// Phase A (the read-only dry run: day-locking, stale/move detection, staff
// validation, the three planning passes, overstaffing rejection, and ack
// reconciliation) lives in TimetableDataService.PlanDeviations (#1886). The
// handler parses the request, runs Phase A via one service call, then executes
// the returned plan (Phase B writes) through InstanceService — so a 409 rendered
// mid-flow never commits partial state (Phase A did not write, and a Phase-B
// failure rolls the tenant tx back).
//
// Permission: SchedulesManage. Same tenant tx as the other /instances routes.
package timetable

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
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

	plan, err := rs.TimetableData.PlanDeviations(ctx, id, toDeviationInput(req))
	if err != nil {
		renderDeviationPlanError(w, r, err)
		return
	}

	// Cancel is exclusive: the shared Cancel service both validates the
	// transition and ends any active bridge. Nothing else is applied.
	if plan.Cancel {
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

	affected, ok := rs.applyDeviationWrites(w, r, id, plan)
	if !ok {
		return
	}

	common.Respond(w, r, http.StatusOK, ApplyDeviationsResponse{
		InstanceID:        id,
		Cancelled:         false,
		UnderstaffedAck:   plan.FinalAck,
		AffectedInstances: affected,
		Warnings:          plan.Warnings,
	}, "Deviations applied")
}

// toDeviationInput maps the parsed HTTP body onto the service input.
func toDeviationInput(req applyDeviationsRequest) scheduleSvc.DeviationInput {
	in := scheduleSvc.DeviationInput{
		Cancel:           req.Cancel,
		UnderstaffedAck:  req.UnderstaffedAck,
		UnderstaffedNote: req.UnderstaffedNote,
		Presences:        req.Presences,
	}
	for _, a := range req.Absences {
		in.Absences = append(in.Absences, scheduleSvc.DeviationAbsenceInput{StaffID: a.StaffID, Reason: a.Reason})
	}
	for _, sub := range req.Substitutions {
		in.Substitutions = append(in.Substitutions, scheduleSvc.DeviationSubstitutionInput{
			AbsentStaffID:     sub.AbsentStaffID,
			SubstituteStaffID: sub.SubstituteStaffID,
			Reason:            sub.Reason,
		})
	}
	return in
}

// applyDeviationWrites executes the validated plan (Phase B): the roster writes
// (present/absent/substitute), the acknowledgement reconciliation, and the SSE
// broadcast. Returns the affected-instance list and ok=false when it has already
// rendered an error.
func (rs *Resource) applyDeviationWrites(w http.ResponseWriter, r *http.Request, id int64, plan *scheduleSvc.DeviationPlan) ([]AffectedInstance, bool) {
	ctx := r.Context()
	now := time.Now()
	actor := resolveActorAccountID(ctx)
	affected := make([]AffectedInstance, 0, len(plan.Presences)+len(plan.Absences)+len(plan.Subs))
	touched := make(map[int64]*scheduleModel.ActivityInstance)

	for _, op := range plan.Presences {
		if err := rs.InstanceService.ApplyPresence(ctx, op.Row, op.Instance, actor, touched); err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap("clear absence failed", err))
			return nil, false
		}
		affected = append(affected, affectedInstanceOf(op.Instance, substituteActionMarkedPresent))
	}
	for _, op := range plan.Absences {
		if err := rs.InstanceService.ApplyAbsence(ctx, op.Row, op.Instance, op.Reason, actor, touched); err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap("mark absent failed", err))
			return nil, false
		}
		affected = append(affected, affectedInstanceOf(op.Instance, substituteActionMarkedAbsent))
	}
	for _, op := range plan.Subs {
		if err := rs.InstanceService.ApplySubstitute(ctx, op.Op, op.SubID, op.Reason, now, actor, touched); err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap("assign substitute failed", err))
			return nil, false
		}
		affected = append(affected, affectedInstanceOf(op.Instance, op.Action))
	}

	if !rs.reconcileDeviationAcks(w, r, id, plan) {
		return nil, false
	}

	rs.broadcastDeviationSaveEvents(ctx, touched, len(affected), plan.AckChanged, len(plan.ClearAckIDs))

	rs.getLogger().Info("deviations applied",
		slog.Int64("instance_id", id),
		slog.Int("absences", len(plan.Absences)),
		slog.Int("presences", len(plan.Presences)),
		slog.Int("substitutions", len(plan.Subs)),
		slog.Bool("understaffed_ack", plan.FinalAck),
	)

	return affected, true
}

// reconcileDeviationAcks applies the selected instance's acknowledgement change
// and clears every stale acknowledgement the save covered on OTHER instances
// (#1840). A concurrent cancel/full-staffing of THIS instance makes
// SetUnderstaffedAck return a 4xx after the roster writes already succeeded;
// TenantTxMiddleware commits non-5xx responses unless we roll back, so force the
// whole tx to roll back before rendering.
func (rs *Resource) reconcileDeviationAcks(w http.ResponseWriter, r *http.Request, id int64, plan *scheduleSvc.DeviationPlan) bool {
	ctx := r.Context()
	actor := resolveActorAccountID(ctx)
	if plan.AckChanged {
		if _, err := rs.InstanceService.SetUnderstaffedAck(ctx, id, plan.FinalAck, plan.AckNote, actor); err != nil {
			tenant.MarkRollback(ctx)
			renderInstanceLifecycleError(w, r, err)
			return false
		}
	}
	for _, cid := range plan.ClearAckIDs {
		if err := rs.InstanceService.ClearUnderstaffedAckIfStaffed(ctx, cid, actor); err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap("clear stale understaffed ack failed", err))
			return false
		}
	}
	return true
}

// renderDeviationPlanError maps a Phase-A classification error onto the exact
// HTTP response the former in-handler common.Error* calls produced.
func renderDeviationPlanError(w http.ResponseWriter, r *http.Request, err error) {
	var de *scheduleSvc.DeviationPlanError
	if !errors.As(err, &de) {
		common.RenderError(w, r, common.ErrorInternalServerWrap("plan deviations failed", err))
		return
	}
	switch de.HTTPStatus {
	case http.StatusBadRequest:
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(de.Message)))
	case http.StatusNotFound:
		common.RenderError(w, r, common.ErrorNotFound(errors.New(de.Message)))
	case http.StatusConflict:
		common.RenderError(w, r, common.ErrorConflictWithCode(errors.New(de.Message), de.Code))
	default:
		if de.Cause != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap(de.Message, de.Cause))
		} else {
			common.RenderError(w, r, common.ErrorInternalServer(errors.New(de.Message)))
		}
	}
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
