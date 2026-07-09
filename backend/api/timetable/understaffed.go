// Package timetable — deliberately-unstaffed acknowledgement (issue #1840).
//
//	POST /api/timetable/instances/{id}/acknowledge-understaffed
//
// Body: {ack: bool, note?: string}. Marks a planned/active block as
// intentionally left unstaffed so gap detection reports it as an acknowledged
// shortfall instead of an open gap. Clearing (ack=false) also clears the note.
// Permission: SchedulesManage — same as the lifecycle mutations. Runs in the
// shared tenant tx.
package timetable

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// understaffedAckRequest is the POST body shape. Note is trimmed to the same
// reason ceiling as activity_exceptions so the two "why" surfaces stay
// consistent.
type understaffedAckRequest struct {
	Ack  bool    `json:"ack"`
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
	// Normalize an empty note to nil so we never persist a blank reason, and
	// reject an over-long one instead of silently truncating.
	if req.Note != nil {
		if *req.Note == "" {
			req.Note = nil
		} else if len(*req.Note) > understaffedAckNoteMaxLength {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("note is too long")))
			return
		}
	}

	instance, err := rs.InstanceService.SetUnderstaffedAck(r.Context(), id, req.Ack, req.Note)
	if err != nil {
		renderInstanceLifecycleError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, UnderstaffedAckResponse{
		InstanceID:       instance.ID,
		Status:           instance.Status,
		UnderstaffedAck:  instance.UnderstaffedAck,
		UnderstaffedNote: instance.UnderstaffedNote,
	}, "Understaffed acknowledgement updated")
}
