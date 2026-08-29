package students

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type bulkApproveParentRequestsBody struct {
	Requests []struct {
		Kind            userService.ParentRequestKind `json:"kind"`
		ID              string                        `json:"id"`
		ExpectedVersion string                        `json:"expected_version"`
	} `json:"requests"`
	Reason string `json:"reason"`
}

func (rs *Resource) bulkApproveParentRequests(w http.ResponseWriter, r *http.Request) {
	if rs.ParentRequestBulkService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("parent request bulk service not configured")))
		return
	}
	var body bulkApproveParentRequestsBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	refs := make([]userService.ParentRequestRef, 0, len(body.Requests))
	for _, item := range body.Requests {
		id, err := strconv.ParseInt(item.ID, 10, 64)
		if err != nil || id <= 0 {
			renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request id")))
			return
		}
		refs = append(refs, userService.ParentRequestRef{
			Kind: item.Kind, ID: id, ExpectedVersion: item.ExpectedVersion,
		})
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	err := rs.ParentRequestBulkService.BulkApprove(r.Context(), userService.BulkApproveParentRequestsInput{
		Requests: refs, Reason: strings.TrimSpace(body.Reason), ReviewerID: int64(claims.ID),
		ReasonRequired: rs.staffReasonRequired(r),
	})
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderError(w, r, bulkParentRequestErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]int{"approved_count": len(refs)}, "Requests approved")
}

var bulkParentRequestErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: userService.ErrParentRequestStale, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, "change_request_stale")
	}},
	{Target: userService.ErrBulkIneligible, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, "bulk_approval_ineligible")
	}},
	{Target: userService.ErrParentRequestNotFound, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, "bulk_approval_ineligible")
	}},
	{Target: userService.ErrInvalidBulkRequest, Render: common.ErrorInvalidRequest},
	{Target: userService.ErrParentRequestForbidden, Render: common.ErrorForbidden},
}, common.ErrorInternalServer)
