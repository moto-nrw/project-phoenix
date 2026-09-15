package presence

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove"
)

type moveStudentsToOpenRoomRequest struct {
	StudentIDs   []int64 `json:"student_ids"`
	TargetRoomID int64   `json:"target_room_id"`
}

// moveStudentsToOpenRoom records that the children now use a released room
// (#3066): an independent room stay, not participation in an activity there
// and no supervision for the caller. The caller keeps every source-side right
// of the ordinary move; the destination only has to be released.
func (rs *Resource) moveStudentsToOpenRoom(w http.ResponseWriter, r *http.Request) {
	var req moveStudentsToOpenRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if len(req.StudentIDs) == 0 || req.TargetRoomID <= 0 {
		common.RenderError(w, r, ErrorInvalidRequest(openroommove.ErrInvalidMove))
		return
	}

	auth, ok := rs.bulkStudentMoveAuthorization(w, r)
	if !ok {
		return
	}

	result, err := rs.OpenRoomMove.MoveToOpenRoom(r.Context(), openroommove.Move{
		StudentIDs: req.StudentIDs,
		RoomID:     req.TargetRoomID,
		Actor: openroommove.Actor{
			StaffID:                      auth.StaffID,
			BypassResourceChecks:         auth.BypassResourceChecks,
			SchoolWideAttendanceEligible: auth.SchoolWideAttendanceEligible,
		},
	})
	if err != nil {
		rs.runtime.MarkRollback(r.Context())
		common.RenderError(w, r, openRoomMoveErrorRenderer(err))
		return
	}

	// The same wire shape as the ordinary bulk move, so clients handle both
	// results alike.
	response := &studentpresence.StudentMoveResult{
		Moved:         nonNilIDs(result.Moved),
		Unchanged:     nonNilIDs(result.Unchanged),
		Skipped:       make([]studentpresence.StudentMoveSkipped, 0, len(result.Skipped)),
		ActiveGroupID: &result.RoomSessionID,
		RoomID:        &result.RoomID,
	}
	for _, skipped := range result.Skipped {
		response.Skipped = append(response.Skipped, studentpresence.StudentMoveSkipped{StudentID: skipped.StudentID, Reason: skipped.Reason})
	}
	common.Respond(w, r, http.StatusOK, response, "Students moved successfully")
}

// openRoomMoveErrorRenderer maps the workflow's own rejections and hands every
// Student Presence failure to the ordinary move mapping. A release removed
// after the caller loaded the room list is a conflict with current state, not
// a permission problem, so the client can explain the stale choice.
func openRoomMoveErrorRenderer(err error) render.Renderer {
	switch {
	case errors.Is(err, openroommove.ErrInvalidMove):
		return ErrorInvalidRequest(err)
	case errors.Is(err, openroommove.ErrRoomNotFound):
		return common.ErrorNotFound(err)
	case errors.Is(err, openroommove.ErrRoomNotReleased):
		return common.ErrorConflictWithCode(err, "room_not_released")
	}
	return ErrorRenderer(err)
}

func nonNilIDs(ids []int64) []int64 {
	if ids == nil {
		return []int64{}
	}
	return ids
}

type moveStudentsToActiveGroupRequest struct {
	StudentIDs          []int64 `json:"student_ids"`
	TargetActiveGroupID int64   `json:"target_active_group_id"`
}

type moveStudentsToTransitRequest struct {
	StudentIDs []int64 `json:"student_ids"`
}

func (rs *Resource) moveStudentsToActiveGroup(w http.ResponseWriter, r *http.Request) {
	var req moveStudentsToActiveGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if len(req.StudentIDs) == 0 || req.TargetActiveGroupID <= 0 {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("student_ids and target_active_group_id are required")))
		return
	}

	auth, ok := rs.bulkStudentMoveAuthorization(w, r)
	if !ok {
		return
	}

	result, err := rs.Operations.MoveStudentsToSession(r.Context(), req.StudentIDs, req.TargetActiveGroupID, *auth)
	if err != nil {
		rs.runtime.MarkRollback(r.Context())
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, result, "Students moved successfully")
}

func (rs *Resource) moveStudentsToTransit(w http.ResponseWriter, r *http.Request) {
	var req moveStudentsToTransitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if len(req.StudentIDs) == 0 {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("student_ids are required")))
		return
	}

	auth, ok := rs.bulkStudentMoveAuthorization(w, r)
	if !ok {
		return
	}

	result, err := rs.Operations.MoveStudentsToTransit(r.Context(), req.StudentIDs, *auth)
	if err != nil {
		rs.runtime.MarkRollback(r.Context())
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, result, "Students moved to transit successfully")
}
