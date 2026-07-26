// Package timetable — deliberately-unstaffed acknowledgement (issue #1840).
//
//	POST /api/timetable/instances/{id}/acknowledge-understaffed
//
// Body: {ack: bool, note?: string}. Marks a planned/active block as
// intentionally left unstaffed so gap detection reports it as an acknowledged
// shortfall instead of an open gap. Clearing (ack=false) also clears the note.
// Permission: SchedulesManage — same as the lifecycle mutations. Runs in the
// shared tenant tx.
//
// The past-block guard, day-lock, and concurrent-move detection live in
// InstanceService.AcknowledgeUnderstaffed; the handler only validates the note
// shape and shapes the response.
package timetable

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// understaffedAckRequest is the POST body shape. Note is trimmed to the same
// reason ceiling as activity_exceptions so the two "why" surfaces stay
// consistent. Ack is a pointer so an omitted field is distinguishable from an
// explicit false: a malformed body such as {} must be rejected, not silently
// treated as "clear the acknowledgement".
type understaffedAckRequest struct {
	Ack  *bool   `json:"ack"`
	Note *string `json:"note,omitempty"`
}

// understaffedAckNoteMaxLength caps the optional reason. Mirrors
// scheduleModel.ActivityExceptionReasonMaxLength (500) so cancel-reason and
// unfilled-reason share one limit.
const understaffedAckNoteMaxLength = 500

// UnderstaffedAckResponse is the 200 body.
type UnderstaffedAckResponse struct {
	InstanceID       int64   `json:"instance_id"`
	Status           string  `json:"status"`
	UnderstaffedAck  bool    `json:"understaffed_ack"`
	UnderstaffedNote *string `json:"understaffed_note,omitempty"`
}

// acknowledgeUnderstaffed handles POST /instances/{id}/acknowledge-understaffed.
func (rs *Resource) acknowledgeUnderstaffed(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance id")))
		return
	}
	if rs.InstanceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("instance service not wired")))
		return
	}

	var req understaffedAckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid JSON body")))
		return
	}
	// `ack` is required: an omitted field ({} or a body missing the key) must not
	// silently clear an existing acknowledgement and note.
	if req.Ack == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("'ack' is required")))
		return
	}
	// Trim the note before the empty and length checks so a whitespace-only note
	// (e.g. "   ") normalizes to nil and is never persisted as an empty-looking
	// reason — matching trimReason on the substitute/deviations path. We keep the
	// reject-on-over-long behavior (rather than trimReason's silent truncation)
	// deliberately for this standalone endpoint. The ceiling counts runes, not
	// bytes, so it matches the frontend's character-based maxLength and the
	// rune-based trimReason ceiling.
	if req.Note != nil {
		trimmed := strings.TrimSpace(*req.Note)
		if trimmed == "" {
			req.Note = nil
		} else if utf8.RuneCountInString(trimmed) > understaffedAckNoteMaxLength {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("note is too long")))
			return
		} else {
			req.Note = &trimmed
		}
	}

	instance, err := rs.InstanceService.AcknowledgeUnderstaffed(r.Context(), id, *req.Ack, req.Note, jwt.ActorAccountIDFromCtx(r.Context()))
	if err != nil {
		renderDeviationError(w, r, err)
		return
	}

	// Understaffed toggles fire no instance_* lifecycle event; without this
	// tenant-wide signal an open staff page keeps the stale flag (#1844).
	rs.broadcastStaffingDeviationChanged(r.Context(), "understaffed_ack")

	common.Respond(w, r, http.StatusOK, UnderstaffedAckResponse{
		InstanceID:       instance.ID,
		Status:           instance.Status,
		UnderstaffedAck:  instance.UnderstaffedAck,
		UnderstaffedNote: instance.UnderstaffedNote,
	}, "Understaffed acknowledgement updated")
}
