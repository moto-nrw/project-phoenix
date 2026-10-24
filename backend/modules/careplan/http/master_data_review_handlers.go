package careplanhttp

import (
	json "encoding/json"
	errors "errors"
	http "net/http"

	render "github.com/go-chi/render"
	common "github.com/moto-nrw/project-phoenix/api/common"
	masterdatarequests "github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	sessiontoken "github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// DecideMasterDataChangeRequestBody is the body of POST
// .../master-data-change-requests/{requestId}/decide.
type DecideMasterDataChangeRequestBody struct {
	Approve *bool  `json:"approve"`
	Reason  string `json:"reason"`
	// ExpectedVersion is the expected_version the list emitted for this row.
	// Empty is accepted (old clients) and skips the check.
	ExpectedVersion string `json:"expected_version"`
}

var masterDataDecisionErrorRenderer = common.RulesRenderer(parentRequestRules(
	common.ErrorRule{Target: masterdatarequests.ErrReviewNotFound, Render: common.ErrorNotFound},
	common.ErrorRule{Target: masterdatarequests.ErrReviewForbidden, Render: common.ErrorForbidden},
	common.ErrorRule{Target: masterdatarequests.ErrReviewNotPending, Render: conflictWithCode(common.CodeStudentsChangeRequestNotPending)},
	common.ErrorRule{Target: masterdatarequests.ErrReviewStaleValue, Render: conflictWithCode(common.CodeStudentsChangeRequestStale)},
	common.ErrorRule{Target: masterdatarequests.ErrReviewInvalidTarget, Render: common.ErrorInvalidRequest},
	common.ErrorRule{Target: masterdatarequests.ErrReviewInvalidValue, Render: common.ErrorInvalidRequest},
), masterDataDecisionFallback)

// Approving an allowed_departure_modes change rewrites the child's departure
// plan, so it can strand or collide with a "läuft mit" link exactly like the
// student PUT does. Both conditions are expected and actionable, not a server
// failure (#1694).
func masterDataDecisionFallback(err error) render.Renderer {
	if renderer := companionPlanErrorRenderer(err); renderer != nil {
		return renderer
	}
	return common.ErrorInternalServer(err)
}

// decideMasterDataChangeRequest approves (and applies) or rejects one request.
func (rs *Resource) decideMasterDataChangeRequest(w http.ResponseWriter, r *http.Request) {
	if rs.MasterDataReviewService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("master data review service not configured")))
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request id")
	if !ok {
		return
	}
	var body DecideMasterDataChangeRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if body.Approve == nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("approve is required")))
		return
	}

	claims := sessiontoken.ClaimsFromCtx(r.Context())
	item, err := rs.MasterDataReviewService.Decide(r.Context(), masterdatarequests.DecideInput{
		RequestID:       requestID,
		Approve:         *body.Approve,
		Reason:          body.Reason,
		ExpectedVersion: body.ExpectedVersion,
		ReviewedBy:      int64(claims.ID),
		ReasonRequired:  rs.staffReasonRequired(r),
	})
	if err != nil {
		renderError(w, r, masterDataDecisionErrorRenderer(err))
		return
	}

	rs.WriteMasterDataDecision(w, r, item)
}
