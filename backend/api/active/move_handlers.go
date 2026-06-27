package active

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
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
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if len(req.StudentIDs) == 0 || req.TargetActiveGroupID <= 0 {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("student_ids and target_active_group_id are required")))
		return
	}

	result, err := rs.ActiveService.MoveStudentsToActiveGroup(r.Context(), req.StudentIDs, req.TargetActiveGroupID)
	if err != nil {
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

	result, err := rs.ActiveService.MoveStudentsToTransit(r.Context(), req.StudentIDs)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, result, "Students moved to transit successfully")
}
