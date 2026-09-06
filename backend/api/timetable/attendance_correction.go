// Package timetable — attendance correction endpoint (#2898).
//
// Endpoint: POST /api/timetable/instances/{instance_id}/students/{student_id}/correction
// Gate:     permissions.SchedulesManage (registered at the router level in api.go)
//
// Why a separate endpoint instead of relaxing the attendance PATCH:
// correcting a closed record is a different act from recording a running one.
// Keeping them apart lets the PATCH stay frozen after completion — supervisors
// on duty cannot rewrite a finished day — while leadership gets a deliberate,
// named action that demands a reason and leaves an append-only trail.
package timetable

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// CorrectAttendanceRequest reuses the PATCH body shape (same tri-state
// handling for the nullable fields) and adds the mandatory reason.
type CorrectAttendanceRequest struct {
	PatchAttendanceRequest
	Reason string `json:"reason"`
}

// correctInstanceStudent handles the correction of a completed block.
func (rs *Resource) correctInstanceStudent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	instanceID, studentID, ok := parseAttendancePathIDs(w, r)
	if !ok {
		return
	}
	if rs.TimetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("attendance correction not wired")))
		return
	}

	req, ok := decodeCorrectionBody(w, r)
	if !ok {
		return
	}

	patch, parseErrs := parseAttendancePatchRequest(&req.PatchAttendanceRequest)
	if len(parseErrs) > 0 {
		renderValidationErrors(w, r, parseErrs)
		return
	}
	if !patch.HasChanges() {
		renderValidationErrors(w, r, []fieldError{
			{Field: "body", Reason: "at least one of status, substatus, note must be set"},
		})
		return
	}

	accountID, _ := operationActor(ctx)
	updated, err := rs.TimetableData.CorrectInstanceStudentAttendance(ctx, instanceID, studentID, patch, req.Reason, accountID)
	if err != nil {
		rs.renderCorrectionError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, mapAttendanceToResponse(updated), "Attendance corrected")
}

// renderCorrectionError maps the correction sentinels onto the wire. The
// reason rules render as field errors so the form can point at the input that
// needs fixing.
func (rs *Resource) renderCorrectionError(w http.ResponseWriter, r *http.Request, err error) {
	var validationErr *scheduleSvc.TimetableAttendanceValidationError
	switch {
	case errors.As(err, &validationErr):
		renderValidationErrors(w, r, attendancePatchFieldErrors(validationErr.Fields))
	case errors.Is(err, scheduleSvc.ErrCorrectionReasonRequired):
		renderValidationErrors(w, r, []fieldError{{Field: "reason", Reason: "a reason is required"}})
	case errors.Is(err, scheduleSvc.ErrCorrectionReasonTooLong):
		renderValidationErrors(w, r, []fieldError{{Field: "reason", Reason: "reason is too long"}})
	case errors.Is(err, scheduleSvc.ErrAttendanceEntryNotFound),
		errors.Is(err, scheduleSvc.ErrInstanceNotFound):
		common.RenderError(w, r, common.ErrorNotFound(errors.New("instance student not found")))
	case errors.Is(err, scheduleSvc.ErrCorrectionRequiresCompleted):
		common.RenderError(w, r, common.ErrorConflict(errors.New("only a completed block can be corrected")))
	case errors.Is(err, scheduleSvc.ErrCorrectionCancelled):
		common.RenderError(w, r, common.ErrorConflict(errors.New("attendance of a cancelled block cannot be corrected")))
	case errors.Is(err, scheduleSvc.ErrCorrectionTrailUnavailable):
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("correction trail is not available")))
	default:
		common.RenderError(w, r, common.ErrorInternalServerWrap("correct attendance failed", err))
	}
}

// getInstanceStudentCorrections handles
// GET /instances/{instance_id}/students/{student_id}/corrections.
func (rs *Resource) getInstanceStudentCorrections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	instanceID, studentID, ok := parseAttendancePathIDs(w, r)
	if !ok {
		return
	}
	if rs.TimetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("attendance correction not wired")))
		return
	}

	rows, err := rs.TimetableData.GetAttendanceCorrections(ctx, instanceID, studentID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load attendance corrections failed", err))
		return
	}

	items := make([]attendanceCorrectionResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, attendanceCorrectionResponse{
			FieldName:   row.FieldName,
			OldValue:    row.OldValue,
			NewValue:    row.NewValue,
			Reason:      row.Reason,
			ActorName:   row.ActorNameSnapshot,
			CorrectedAt: row.CreatedAt,
		})
	}
	common.Respond(w, r, http.StatusOK, attendanceCorrectionsResponse{Corrections: items}, "Attendance corrections retrieved")
}

// attendanceCorrectionResponse is one entry of the trail on the wire. It
// carries the snapshotted actor name rather than an account id: the trail is
// read by people, and the id would need a second lookup that may no longer
// resolve after the account is gone.
type attendanceCorrectionResponse struct {
	FieldName   string    `json:"field_name"`
	OldValue    *string   `json:"old_value,omitempty"`
	NewValue    *string   `json:"new_value,omitempty"`
	Reason      string    `json:"reason"`
	ActorName   *string   `json:"actor_name,omitempty"`
	CorrectedAt time.Time `json:"corrected_at"`
}

type attendanceCorrectionsResponse struct {
	Corrections []attendanceCorrectionResponse `json:"corrections"`
}

// decodeCorrectionBody mirrors decodePatchBody but keeps the reason field. An
// empty body is NOT tolerated here: a correction without a body cannot carry
// the mandatory reason, so it fails at validation with a field error rather
// than silently reporting "no changes".
func decodeCorrectionBody(w http.ResponseWriter, r *http.Request) (*CorrectAttendanceRequest, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 16*1024))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("failed to read request body")))
		return nil, false
	}
	req := &CorrectAttendanceRequest{}
	if len(body) == 0 {
		return req, true
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("invalid JSON body: %w", err)))
		return nil, false
	}
	return req, true
}
