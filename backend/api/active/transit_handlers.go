package active

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type assignTransitStudentsRequest struct {
	StudentIDs    []int64 `json:"student_ids"`
	ActiveGroupID int64   `json:"active_group_id"`
}

func (rs *Resource) assignTransitStudents(w http.ResponseWriter, r *http.Request) {
	var req assignTransitStudentsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if len(req.StudentIDs) == 0 || req.ActiveGroupID <= 0 {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("student_ids and active_group_id are required")))
		return
	}

	if !rs.authorizeBulkStudentMove(w, r, req.StudentIDs, &req.ActiveGroupID) {
		return
	}

	result, err := rs.ActiveService.AssignTransitStudentsToActiveGroup(r.Context(), req.StudentIDs, req.ActiveGroupID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, result, "Transit students assigned successfully")
}
