package active

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/tenant"
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

	auth, ok := rs.bulkStudentMoveAuthorization(w, r)
	if !ok {
		return
	}

	result, err := rs.ActiveService.MoveStudentsToActiveGroupAuthorized(r.Context(), req.StudentIDs, req.TargetActiveGroupID, *auth)
	if err != nil {
		tenant.MarkRollback(r.Context())
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

	result, err := rs.ActiveService.MoveStudentsToTransitAuthorized(r.Context(), req.StudentIDs, *auth)
	if err != nil {
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, result, "Students moved to transit successfully")
}
