// Package timetable — three-field attendance PATCH handler (WP-B10).
//
// Endpoint: PATCH /api/timetable/instances/{instance_id}/students/{student_id}
// Gate:     permissions.SchedulesManage (registered at the router level in api.go)
//
// The handler accepts a partial update (status / substatus / note). Pointer-
// and RawMessage-based request parsing distinguishes three JSON states for
// each nullable field:
//
//	MISSING  — leave column untouched
//	null     — clear column (substatus / note only; status is NOT NULL)
//	value    — write value
//
// See parseAttendancePatchRequest + validateAttendancePatch for details.
package timetable

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// PatchAttendanceRequest is the wire shape. Status uses *string (nullable-ish
// via pointer-is-nil means "missing"), while Substatus and Note use
// json.RawMessage so the handler can distinguish "missing" from "explicit
// null" — both columns are nullable and each state is meaningful.
//
// Reviewer focus: look for {note: null} handling. The tri-state resolution
// lives in parseAttendancePatchRequest.
type PatchAttendanceRequest struct {
	Status    *string         `json:"status,omitempty"`
	Substatus json.RawMessage `json:"substatus,omitempty"`
	Note      json.RawMessage `json:"note,omitempty"`
}

// fieldError is a package-local alias for common.FieldError so test tables
// and helper signatures stay compact. The unified ErrResponse carries these
// under the `errors` key — the envelope is the same as every other 400 in
// the codebase, with an optional per-field list.
type fieldError = common.FieldError

// patchInstanceStudent handles PATCH /instances/{instance_id}/students/{student_id}.
//
// Flow:
//  1. Parse path params → instanceID, studentID (400 on bad format).
//  2. Decode body → PatchAttendanceRequest (400 on malformed JSON).
//  3. Resolve tri-state for substatus/note → AttendanceFieldPatch.
//  4. Load current row via FindByInstanceAndStudent (404 if not found OR
//     wrong tenant — RLS makes the row invisible).
//  5. Validate (400 with field errors on violation).
//  6. Write via UpdateAttendanceFields (500 on DB error).
//  7. Re-read and return the updated row.
func (rs *Resource) patchInstanceStudent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	instanceID, studentID, ok := parseAttendancePathIDs(w, r)
	if !ok {
		return
	}

	if rs.TimetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("attendance PATCH not wired")))
		return
	}

	req, ok := decodePatchBody(w, r)
	if !ok {
		return
	}

	patch, parseErrs := parseAttendancePatchRequest(req)
	if len(parseErrs) > 0 {
		renderValidationErrors(w, r, parseErrs)
		return
	}
	if !patch.HasChanges() {
		// 400 the no-op here so the client gets a specific error. The repo
		// short-circuits the same case as a defensive no-op for any future
		// caller that bypasses this handler — the two guards are intentional.
		renderValidationErrors(w, r, []fieldError{
			{Field: "body", Reason: "at least one of status, substatus, note must be set"},
		})
		return
	}

	current, err := rs.TimetableData.GetInstanceStudent(ctx, instanceID, studentID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("instance student not found")))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("load instance student failed", err))
		return
	}
	if current == nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("instance student not found")))
		return
	}

	if rs.rejectFrozenAttendanceWrite(w, r, instanceID) {
		return
	}

	if verrs := validateAttendancePatch(patch, current); len(verrs) > 0 {
		renderValidationErrors(w, r, verrs)
		return
	}

	if err := rs.TimetableData.UpdateInstanceStudentAttendance(ctx, current.ID, patch); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("update attendance failed", err))
		return
	}

	// Re-read so the response body reflects the post-write state (including
	// the bumped updated_at). One extra SELECT is worth the response fidelity.
	updated, err := rs.TimetableData.GetInstanceStudent(ctx, instanceID, studentID)
	if err != nil || updated == nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("reload updated attendance failed", err))
		return
	}

	rs.getLogger().Info("attendance patched",
		slog.Int64("instance_id", instanceID),
		slog.Int64("student_id", studentID),
	)

	common.Respond(w, r, http.StatusOK, mapAttendanceToResponse(updated), "Attendance updated")
}

func (rs *Resource) rejectFrozenAttendanceWrite(w http.ResponseWriter, r *http.Request, instanceID int64) bool {
	inst, err := rs.TimetableData.GetActivityInstance(r.Context(), instanceID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("instance not found")))
			return true
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("load instance failed", err))
		return true
	}
	if inst != nil && (inst.Status == scheduleModel.InstanceStatusCompleted || inst.Status == scheduleModel.InstanceStatusCancelled) {
		common.RenderError(w, r, common.ErrorConflict(errors.New("attendance is frozen after completion")))
		return true
	}
	return false
}

// parseAttendancePathIDs reads instance_id and student_id from the path.
// Returns (0, 0, false) and renders a 400 on failure.
func parseAttendancePathIDs(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	instanceIDStr := chi.URLParam(r, "instance_id")
	studentIDStr := chi.URLParam(r, "student_id")
	instanceID, err1 := strconv.ParseInt(instanceIDStr, 10, 64)
	studentID, err2 := strconv.ParseInt(studentIDStr, 10, 64)
	if err1 != nil || err2 != nil || instanceID <= 0 || studentID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance_id or student_id")))
		return 0, 0, false
	}
	return instanceID, studentID, true
}

// decodePatchBody reads and JSON-decodes the request body. The 16 KiB cap is
// generous headroom above the 500-char note ceiling — any payload larger
// than that is almost certainly noise. Unknown JSON fields are ignored,
// matching the rest of the timetable/IoT endpoints: a frontend that sends
// a future or stray key does not get a 400 for it.
func decodePatchBody(w http.ResponseWriter, r *http.Request) (*PatchAttendanceRequest, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 16*1024))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("failed to read request body")))
		return nil, false
	}
	if len(body) == 0 {
		// Empty body — treat as missing-all-fields; the handler reports
		// "at least one of ..." via renderValidationErrors downstream.
		return &PatchAttendanceRequest{}, true
	}
	req := &PatchAttendanceRequest{}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("invalid JSON body: %w", err)))
		return nil, false
	}
	return req, true
}

// parseAttendancePatchRequest resolves the JSON tri-state for nullable
// fields into AttendanceFieldPatch. Returns parse-level errors (type
// mismatches) only; business-rule validation happens in validateAttendancePatch
// once the current row is known.
func parseAttendancePatchRequest(req *PatchAttendanceRequest) (scheduleModel.AttendanceFieldPatch, []fieldError) {
	var errs []fieldError
	patch := scheduleModel.AttendanceFieldPatch{}

	if req.Status != nil {
		s := *req.Status
		patch.Status = &s
	}

	if sub, parseErr := decodeNullableString(req.Substatus); parseErr != nil {
		errs = append(errs, fieldError{Field: "substatus", Reason: parseErr.Error()})
	} else if sub.present {
		if sub.isNull {
			patch.SubstatusClear = true
		} else {
			v := sub.value
			patch.Substatus = &v
		}
	}

	if note, parseErr := decodeNullableString(req.Note); parseErr != nil {
		errs = append(errs, fieldError{Field: "note", Reason: parseErr.Error()})
	} else if note.present {
		if note.isNull {
			patch.NoteClear = true
		} else {
			v := note.value
			patch.Note = &v
		}
	}

	return patch, errs
}

// nullableString is the decoded tri-state for a nullable JSON string field:
//
//	{present: false}                    → field missing from body
//	{present: true, isNull: true}       → field is JSON null
//	{present: true, isNull: false, ...} → field is a JSON string
type nullableString struct {
	present bool
	isNull  bool
	value   string
}

// decodeNullableString parses a json.RawMessage into nullableString.
// Non-string, non-null JSON values are rejected with an error.
func decodeNullableString(raw json.RawMessage) (nullableString, error) {
	if len(raw) == 0 {
		return nullableString{present: false}, nil
	}
	// json.RawMessage preserves the null literal as the 4-byte string "null".
	if string(raw) == "null" {
		return nullableString{present: true, isNull: true}, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nullableString{}, errors.New("must be a string or null")
	}
	return nullableString{present: true, value: s}, nil
}

// validateAttendancePatch enforces business rules. Called ONCE the current
// row is known so the substatus-vs-status cross-field rule can be resolved
// against the effective final status.
//
// Truth table for the substatus-vs-status cross-field rule (current
// row.status is the baseline; patch.Status overrides it if provided):
//
//	current.status | patch.status  | effective final status | final substatus non-null | result
//	---------------|---------------|------------------------|--------------------------|-------
//	expected       | nil           | expected               | true                     | REJECT
//	expected       | "present"     | present                | true                     | OK
//	expected       | "absent"      | absent                 | true                     | OK
//	present        | nil           | present                | true                     | OK
//	present        | "expected"    | expected               | true                     | REJECT
//	any            | any           | any                    | false (nil or cleared)   | OK
//
// "Final substatus non-null" means: existing substatus stays set AND patch
// did not clear it, OR patch sets a non-null value. Cleared substatus (with
// SubstatusClear) is treated as nil regardless of context.
func validateAttendancePatch(patch scheduleModel.AttendanceFieldPatch, current *scheduleModel.InstanceStudent) []fieldError {
	return attendancePatchFieldErrors(scheduleSvc.ValidateAttendancePatch(patch, current))
}

// renderValidationErrors emits the 400 body with the full errors slice.
// The summary lands in the standard `error` field (readable by the generic
// frontend handler) and the per-field list in `errors`.
func renderValidationErrors(w http.ResponseWriter, r *http.Request, errs []fieldError) {
	common.RenderError(w, r, common.ErrorValidation("validation failed", errs))
}

func attendancePatchFieldErrors(errs []scheduleModel.AttendancePatchFieldError) []fieldError {
	if len(errs) == 0 {
		return nil
	}
	out := make([]fieldError, 0, len(errs))
	for _, err := range errs {
		out = append(out, fieldError{Field: err.Field, Reason: err.Reason})
	}
	return out
}

// AttendanceResponse is the on-wire shape of a schedule.instance_students row.
type AttendanceResponse struct {
	ID          int64   `json:"id"`
	InstanceID  int64   `json:"instance_id"`
	StudentID   int64   `json:"student_id"`
	Status      string  `json:"status"`
	Substatus   *string `json:"substatus"`
	Note        *string `json:"note"`
	CheckedInAt *string `json:"checked_in_at"`
}

func mapAttendanceToResponse(row *scheduleModel.InstanceStudent) AttendanceResponse {
	resp := AttendanceResponse{
		ID:         row.ID,
		InstanceID: row.InstanceID,
		StudentID:  row.StudentID,
		Status:     row.Status,
		Substatus:  row.Substatus,
		Note:       row.Note,
	}
	if row.CheckedInAt != nil {
		t := row.CheckedInAt.UTC().Format("2006-01-02T15:04:05Z")
		resp.CheckedInAt = &t
	}
	return resp
}
