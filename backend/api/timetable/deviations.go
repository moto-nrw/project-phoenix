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
// The plan-then-write atomicity, the day-lock, and every business rule live in
// InstanceService.ApplyDeviations (services/schedule/deviation_apply.go). The
// handler only parses the body, calls the service once, maps its DeviationError
// onto the wire contract, and fires the post-save SSE signals.
//
// Permission: SchedulesManage. Same tenant tx as the other /instances routes.
package timetable

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// deviationAbsence marks one staff member absent. An omitted instance_ids keeps
// the established day-wide contract; a present list targets exact appointments.
type deviationAbsence struct {
	StaffID     int64    `json:"staff_id"`
	Reason      *string  `json:"reason,omitempty"`
	InstanceIDs *[]int64 `json:"instance_ids,omitempty"`
}

// deviationSubstitution assigns substituteStaffID to cover absentStaffID. An
// omitted instance_ids keeps the established day-wide contract; a present list
// targets exactly those same-day appointments.
type deviationSubstitution struct {
	AbsentStaffID     int64    `json:"absent_staff_id"`
	SubstituteStaffID int64    `json:"substitute_staff_id"`
	Reason            *string  `json:"reason,omitempty"`
	InstanceIDs       *[]int64 `json:"instance_ids,omitempty"`
}

// deviationPresence clears an absence in an optional appointment scope. Its
// decoder accepts the former numeric staff id so an already-open frontend from
// before this change keeps its established day-wide correction behavior.
type deviationPresence struct {
	StaffID     int64    `json:"staff_id"`
	InstanceIDs *[]int64 `json:"instance_ids,omitempty"`
}

type deviationSubstitutionRemoval struct {
	StaffID     int64    `json:"staff_id"`
	InstanceIDs *[]int64 `json:"instance_ids,omitempty"`
}

func (p *deviationPresence) UnmarshalJSON(data []byte) error {
	var legacyStaffID int64
	if err := json.Unmarshal(data, &legacyStaffID); err == nil {
		p.StaffID = legacyStaffID
		p.InstanceIDs = nil
		return nil
	}
	type presenceObject deviationPresence
	var object presenceObject
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	*p = deviationPresence(object)
	return nil
}

// applyDeviationsRequest is the POST body. All mutation fields are optional; a
// Cancel request is exclusive (other fields are ignored, mirroring the UI where
// "Block absagen" disables everything else). understaffed_ack is a pointer so an
// omitted field ("no change") is distinguishable from an explicit false ("clear").
type applyDeviationsRequest struct {
	Cancel               bool                           `json:"cancel"`
	CancelReason         *string                        `json:"cancel_reason,omitempty"`
	UnderstaffedAck      *bool                          `json:"understaffed_ack,omitempty"`
	UnderstaffedNote     *string                        `json:"understaffed_note,omitempty"`
	Absences             []deviationAbsence             `json:"absences,omitempty"`
	Substitutions        []deviationSubstitution        `json:"substitutions,omitempty"`
	SubstitutionRemovals []deviationSubstitutionRemoval `json:"substitution_removals,omitempty"`
	// Presences marks staff present again. Numeric legacy items are day-wide;
	// object items may target selected appointments.
	Presences []deviationPresence `json:"presences,omitempty"`
	// GuardianNotice informs the families of the booked children when cancel
	// is set (#2601).
	GuardianNotice *GuardianNoticeRequest `json:"guardian_notice,omitempty"`
}

// ApplyDeviationsResponse is the 200 body.
type ApplyDeviationsResponse struct {
	InstanceID        int64                                `json:"instance_id"`
	Cancelled         bool                                 `json:"cancelled"`
	UnderstaffedAck   bool                                 `json:"understaffed_ack"`
	AffectedInstances []AffectedInstance                   `json:"affected_instances"`
	Warnings          []scheduleSvc.SubstituteTimeConflict `json:"warnings"`
	GuardianNotice    *GuardianNoticeResponse              `json:"guardian_notice,omitempty"`
}

// toServiceInput maps the wire request onto the service input, resolving the
// acting account for the Änderungsprotokoll (#1886).
func (req applyDeviationsRequest) toServiceInput(actor *int64) scheduleSvc.ApplyDeviationsInput {
	absences := make([]scheduleSvc.DeviationAbsenceInput, 0, len(req.Absences))
	for _, a := range req.Absences {
		absences = append(absences, scheduleSvc.DeviationAbsenceInput{
			StaffID: a.StaffID, Reason: a.Reason, InstanceIDs: a.InstanceIDs,
		})
	}
	subs := make([]scheduleSvc.DeviationSubstitutionInput, 0, len(req.Substitutions))
	for _, s := range req.Substitutions {
		subs = append(subs, scheduleSvc.DeviationSubstitutionInput{
			AbsentStaffID:     s.AbsentStaffID,
			SubstituteStaffID: s.SubstituteStaffID,
			Reason:            s.Reason,
			InstanceIDs:       s.InstanceIDs,
		})
	}
	presences := make([]scheduleSvc.DeviationPresenceInput, 0, len(req.Presences))
	for _, p := range req.Presences {
		presences = append(presences, scheduleSvc.DeviationPresenceInput{
			StaffID: p.StaffID, InstanceIDs: p.InstanceIDs,
		})
	}
	removals := make([]scheduleSvc.DeviationSubstitutionRemovalInput, 0, len(req.SubstitutionRemovals))
	for _, removal := range req.SubstitutionRemovals {
		removals = append(removals, scheduleSvc.DeviationSubstitutionRemovalInput{
			StaffID: removal.StaffID, InstanceIDs: removal.InstanceIDs,
		})
	}
	return scheduleSvc.ApplyDeviationsInput{
		Cancel:               req.Cancel,
		CancelReason:         req.CancelReason,
		UnderstaffedAck:      req.UnderstaffedAck,
		UnderstaffedNote:     req.UnderstaffedNote,
		Absences:             absences,
		Substitutions:        subs,
		SubstitutionRemovals: removals,
		Presences:            presences,
		ActorAccountID:       actor,
		GuardianNotice:       req.GuardianNotice.toServiceInput(),
	}
}

// applyDeviations handles POST /api/timetable/instances/{id}/deviations.
func (rs *Resource) applyDeviations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance id")))
		return
	}
	if rs.InstanceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}

	var req applyDeviationsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid JSON body")))
		return
	}

	result, err := rs.InstanceService.ApplyDeviations(ctx, id, req.toServiceInput(jwt.ActorAccountIDFromCtx(ctx)))
	if err != nil {
		renderDeviationError(w, r, err)
		return
	}

	// The cancel branch fires its own instance_cancelled event inside the Cancel
	// service; only the non-cancel save emits the deviation SSE signals and log.
	if !result.Cancelled {
		rs.broadcastDeviationSaveEvents(ctx, result.ActiveTouched, result.AppliedWrites, result.AckChanged, result.ClearedAcks)
		rs.getLogger().Info("deviations applied",
			slog.Int64("instance_id", id),
			slog.Int("absences", result.AbsenceCount),
			slog.Int("presences", result.PresenceCount),
			slog.Int("substitutions", result.SubstitutionCount),
			slog.Int("substitution_removals", result.SubstitutionRemovalCount),
			slog.Bool("understaffed_ack", result.UnderstaffedAck),
		)
	}

	common.Respond(w, r, http.StatusOK, deviationResponseOf(id, result), result.Message)
}

// deviationResponseOf shapes the service result into the wire response, mapping
// the neutral affected list onto AffectedInstance and defaulting nil slices to
// empty ones so the JSON always carries arrays.
func deviationResponseOf(id int64, result *scheduleSvc.ApplyDeviationsResult) ApplyDeviationsResponse {
	affected := make([]AffectedInstance, 0, len(result.Affected))
	for _, a := range result.Affected {
		affected = append(affected, AffectedInstance{
			InstanceID: a.InstanceID,
			Title:      a.Title,
			StartTime:  a.StartTime.Format("15:04"),
			Action:     a.Action,
		})
	}
	warnings := result.Warnings
	if warnings == nil {
		warnings = []scheduleSvc.SubstituteTimeConflict{}
	}
	return ApplyDeviationsResponse{
		InstanceID:        id,
		Cancelled:         result.Cancelled,
		UnderstaffedAck:   result.UnderstaffedAck,
		AffectedInstances: affected,
		Warnings:          warnings,
		GuardianNotice:    guardianNoticeResponseOf(result.GuardianNotice),
	}
}

// renderDeviationError maps a DeviationError onto the exact HTTP response the
// former inline handler produced. Lifecycle sentinels (from Cancel /
// SetUnderstaffedAck) fall through to the shared lifecycle mapper.
func renderDeviationError(w http.ResponseWriter, r *http.Request, err error) {
	var de *scheduleSvc.DeviationError
	if !errors.As(err, &de) {
		renderInstanceLifecycleError(w, r, err)
		return
	}
	switch de.Status {
	case http.StatusBadRequest:
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(de.ClientMsg)))
	case http.StatusNotFound:
		common.RenderError(w, r, common.ErrorNotFound(errors.New(de.ClientMsg)))
	case http.StatusConflict:
		common.RenderError(w, r, common.ErrorConflictWithCode(errors.New(de.ClientMsg), de.Code))
	default:
		if de.Cause != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap(de.ClientMsg, de.Cause))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(errors.New(de.ClientMsg)))
	}
}
