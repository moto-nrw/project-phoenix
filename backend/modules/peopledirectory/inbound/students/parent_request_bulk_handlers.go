package students

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type bulkApproveParentRequestsBody struct {
	Requests []struct {
		Kind            parentrequests.Kind `json:"kind"`
		ID              string              `json:"id"`
		ExpectedVersion string              `json:"expected_version"`
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
	refs := make([]parentrequests.Ref, 0, len(body.Requests))
	for _, item := range body.Requests {
		id, err := strconv.ParseInt(item.ID, 10, 64)
		if err != nil || id <= 0 {
			renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request id")))
			return
		}
		refs = append(refs, parentrequests.Ref{
			Kind: item.Kind, ID: id, ExpectedVersion: item.ExpectedVersion,
		})
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	err := rs.ParentRequestBulkService.BulkApprove(r.Context(), parentrequests.BulkApproveInput{
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
	{Target: parentrequests.ErrStale, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, "change_request_stale")
	}},
	{Target: parentrequests.ErrBulkIneligible, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, "bulk_approval_ineligible")
	}},
	{Target: parentrequests.ErrNotFound, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, "bulk_approval_ineligible")
	}},
	{Target: parentrequests.ErrInvalidBulkRequest, Render: common.ErrorInvalidRequest},
	{Target: parentrequests.ErrForbidden, Render: common.ErrorForbidden},
}, common.ErrorInternalServer)
