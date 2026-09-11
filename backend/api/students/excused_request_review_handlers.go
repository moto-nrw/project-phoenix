package students

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
	requestreviewlegacy "github.com/moto-nrw/project-phoenix/modules/requestreview/legacy"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// StaffExcusedRequestResponse is the legacy-named staff projection of one
// parent absence approval request; the shared request-review projection
// (#2705) owns the shape and the decide route answers with the same one.
type StaffExcusedRequestResponse = requestreview.StaffExcusedRequestResponse

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
		return common.ErrorConflictWithCode(err, "change_request_not_pending")
	}},
	common.ErrorRule{Target: excusedrequests.ErrExcusedRequestGuardianAccessRevoked, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, "guardian_access_revoked")
	}},
	common.ErrorRule{Target: excusedrequests.ErrExcusedRequestStatusConflict, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, "excused_request_status_conflict")
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

	claims := jwt.ClaimsFromCtx(r.Context())
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

	common.Respond(w, r, http.StatusOK, requestreviewlegacy.ToStaffExcusedRequestResponse(item), "Decision applied")
}
