package careplanhttp

import (
	json "encoding/json"
	errors "errors"
	http "net/http"

	render "github.com/go-chi/render"
	common "github.com/moto-nrw/project-phoenix/api/common"
	excusedrequests "github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	sessiontoken "github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	tenant "github.com/moto-nrw/project-phoenix/tenant"
)

// DecideExcusedRequestBody is the body of POST
// .../excused-absence-requests/{requestId}/decide.
type DecideExcusedRequestBody struct {
	Approve *bool  `json:"approve"`
	Reason  string `json:"reason"`
	// ExpectedVersion is the expected_version the list emitted for this row.
	// Empty is accepted (old clients) and skips the check.
	ExpectedVersion string `json:"expected_version"`
}

var excusedDecideErrorRenderer = common.RulesRenderer(parentRequestRules(
	common.ErrorRule{Target: excusedrequests.ErrExcusedRequestNotFound, Render: common.ErrorNotFound},
	common.ErrorRule{Target: excusedrequests.ErrExcusedRequestNotPending, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeStudentsChangeRequestNotPending)
	}},
	common.ErrorRule{Target: excusedrequests.ErrExcusedRequestGuardianAccessRevoked, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeStudentsGuardianAccessRevoked)
	}},
	common.ErrorRule{Target: excusedrequests.ErrExcusedRequestStatusConflict, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeStudentsExcusedRequestStatusConflict)
	}},
	common.ErrorRule{Target: excusedrequests.ErrExcusedRequestForbidden, Render: common.ErrorForbidden},
	common.ErrorRule{Target: excusedrequests.ErrExcusedRequestRejectReasonRequired, Render: common.ErrorInvalidRequest},
	common.ErrorRule{Target: excusedrequests.ErrExcusedRequestRejectReasonTooLong, Render: common.ErrorInvalidRequest},
), common.ErrorInternalServer)

// decideExcusedAbsenceRequest approves (writes the requested status days) or
// rejects (reason required) one pending request.
func (rs *Resource) decideExcusedAbsenceRequest(w http.ResponseWriter, r *http.Request) {
	if rs.ExcusedRequestService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("excused request service not configured")))
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request id")
	if !ok {
		return
	}
	var body DecideExcusedRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if body.Approve == nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("approve is required")))
		return
	}

	claims := sessiontoken.ClaimsFromCtx(r.Context())
	item, err := rs.ExcusedRequestService.Decide(r.Context(), excusedrequests.DecideInput{
		RequestID:       requestID,
		Approve:         *body.Approve,
		Reason:          body.Reason,
		ExpectedVersion: body.ExpectedVersion,
		ReviewedBy:      int64(claims.ID),
		ReasonRequired:  rs.staffReasonRequired(r),
	})
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderError(w, r, excusedDecideErrorRenderer(err))
		return
	}

	rs.WriteExcusedDecision(w, r, item)
}
