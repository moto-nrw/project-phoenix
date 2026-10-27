package careplanhttp

import (
	json "encoding/json"
	errors "errors"
	http "net/http"

	render "github.com/go-chi/render"
	common "github.com/moto-nrw/project-phoenix/api/common"
	carerequests "github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	sessiontoken "github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// DecideCareRequestBody is the body of POST
// .../care-schedule-change-requests/{requestId}/decide.
type DecideCareRequestBody struct {
	Approve     *bool   `json:"approve"`
	Reason      string  `json:"reason"`
	ImpactToken *string `json:"impact_token"`
	// ExpectedVersion is the expected_version the list emitted for this row.
	// Empty is accepted (old clients) and skips the check.
	ExpectedVersion string `json:"expected_version"`
}

// decideCareScheduleChangeRequest approves (applies the weekly plan) or
// rejects (reason required) one pending request.
func (rs *Resource) decideCareScheduleChangeRequest(w http.ResponseWriter, r *http.Request) {
	if rs.CareRequestService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care request service not configured")))
		return
	}
	input, ok := decodeCareRequestDecision(w, r)
	if !ok {
		return
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	input.ReviewedBy = int64(claims.ID)
	input.ReasonRequired = rs.staffReasonRequired(r)
	item, err := rs.CareRequestService.Decide(r.Context(), input)
	if err != nil {
		renderError(w, r, careRequestDecisionErrorRenderer(err))
		return
	}
	rs.WriteCareDecision(w, r, item)
}

func decodeCareRequestDecision(w http.ResponseWriter, r *http.Request) (carerequests.DecideInput, bool) {
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request id")
	if !ok {
		return carerequests.DecideInput{}, false
	}
	var body DecideCareRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return carerequests.DecideInput{}, false
	}
	if body.Approve == nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("approve is required")))
		return carerequests.DecideInput{}, false
	}
	return carerequests.DecideInput{
		RequestID: requestID, Approve: *body.Approve, Reason: body.Reason,
		ExpectedImpactToken: body.ImpactToken, RequireImpactToken: true,
		ExpectedVersion: body.ExpectedVersion,
	}, true
}

var careRequestDecisionErrorRenderer = common.RulesRenderer(parentRequestRules(
	common.ErrorRule{Target: carerequests.ErrNotFound, Render: common.ErrorNotFound},
	common.ErrorRule{Target: carerequests.ErrNotPending, Render: conflictWithCode(common.CodeStudentsChangeRequestNotPending)},
	common.ErrorRule{Target: carerequests.ErrGuardianAccessRevoked, Render: conflictWithCode(common.CodeStudentsGuardianAccessRevoked)},
	common.ErrorRule{Target: carerequests.ErrCareRequestForbidden, Render: common.ErrorForbidden},
	common.ErrorRule{Target: carerequests.ErrPickupChangeConflict, Render: conflictWithCode(common.CodeStudentsPickupChangeConflict)},
	common.ErrorRule{Target: carerequests.ErrPickupPermissionContactGone, Render: conflictWithCode(common.CodeStudentsPickupPermissionContactGone)},
	common.ErrorRule{Target: carerequests.ErrPickupChangeAlreadyCompleted, Render: conflictWithCode(common.CodeStudentsPickupChangeCompleted)},
	common.ErrorRule{Target: carerequests.ErrPickupChangeExpired, Render: conflictWithCode(common.CodeStudentsPickupChangeExpired)},
	common.ErrorRule{Target: carerequests.ErrPickupChangeImpactChanged, Render: conflictWithCode(common.CodeStudentsPickupChangeImpactChanged)},
	common.ErrorRule{Target: carerequests.ErrCareDayManagedByBooking, Render: conflictWithCode(common.CodeStudentsCareDayManagedByBooking)},
	common.ErrorRule{Match: isInvalidCareRequestDecision, Render: common.ErrorInvalidRequest},
), careRequestDecisionFallback)

func conflictWithCode(code string) func(error) render.Renderer {
	return func(err error) render.Renderer { return common.ErrorConflictWithCode(err, code) }
}

func invalidWithCode(code string) func(error) render.Renderer {
	return func(err error) render.Renderer { return common.ErrorInvalidRequestWithCode(err, code) }
}

func notFoundWithCode(code string) func(error) render.Renderer {
	return func(err error) render.Renderer { return common.ErrorNotFoundWithCode(err, code) }
}

func careRequestDecisionFallback(err error) render.Renderer {
	if renderer := companionPlanErrorRenderer(err); renderer != nil {
		return renderer
	}
	return common.ErrorInternalServer(err)
}

func isInvalidCareRequestDecision(err error) bool {
	return errors.Is(err, carerequests.ErrRejectReasonRequired) ||
		errors.Is(err, carerequests.ErrRejectReasonTooLong) ||
		errors.Is(err, carerequests.ErrInvalidPayload) ||
		errors.Is(err, carerequests.ErrPickupChangeImpactRequired)
}
