package presence

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moto-ogs/project-phoenix/api/common"
)

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
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if len(req.StudentIDs) == 0 || req.TargetActiveGroupID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("student_ids and target_active_group_id are required")))
		return
	}

	auth, ok := rs.BulkStudentMoveAuthorization(w, r)
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
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if len(req.StudentIDs) == 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("student_ids are required")))
		return
	}

	auth, ok := rs.BulkStudentMoveAuthorization(w, r)
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

// checkInStudentsToSession checks children in to the school and places them
// in a running session in one unit (#6953), so the app equals a scan at the
// room's terminal. The request shape is the ordinary move's.
func (rs *Resource) checkInStudentsToSession(w http.ResponseWriter, r *http.Request) {
	var req moveStudentsToActiveGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if len(req.StudentIDs) == 0 || req.TargetActiveGroupID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("student_ids and target_active_group_id are required")))
		return
	}

	staffID, auth, ok := rs.CheckInMoveActor(w, r)
	if !ok {
		return
	}

	result, err := rs.Operations.CheckInStudentsToSessionAuthorized(r.Context(), req.StudentIDs, req.TargetActiveGroupID, staffID, *auth)
	if err != nil {
		rs.runtime.MarkRollback(r.Context())
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, result, "Students checked in successfully")
}
