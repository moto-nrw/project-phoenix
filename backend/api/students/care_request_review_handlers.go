package students

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/legacy/careschedule"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
	requestreviewcompose "github.com/moto-nrw/project-phoenix/modules/requestreview/compose"
)

// CareRequestResponse is the staff-facing projection of one parent
// care-schedule change request, including the live "current → requested"
// weekly diff; the shared request-review projection (#2705) owns the shape
// and the decide route answers with the same one.
type CareRequestResponse = requestreview.CareRequestResponse

type AffectedCareBlock = requestreview.AffectedCareBlock

// CareRequestDiffResponse mirrors the request-diff wire shape the messaging
// thread page used, so the frontend's RequestDiffPanel renders it unchanged.
type CareRequestDiffResponse = requestreview.CareRequestDiffResponse

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
	claims := jwt.ClaimsFromCtx(r.Context())
	input.ReviewedBy = int64(claims.ID)
	input.ReasonRequired = rs.staffReasonRequired(r)
	item, err := rs.CareRequestService.Decide(r.Context(), input)
	if err != nil {
		renderError(w, r, careRequestDecisionErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, careDecisionResponse(item), "Decision applied")
}

func nativeCareDiffs(entries []careschedule.RequestDiffEntry) []carerequests.DiffEntry {
	result := make([]carerequests.DiffEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, carerequests.DiffEntry{Label: entry.Label, Old: entry.Old, New: entry.New, Weekday: entry.Weekday, CareKind: entry.CareKind, OldModes: entry.OldModes, NewMode: entry.NewMode})
	}
	return result
}

func careDecisionResponse(item *careschedule.CareRequestReviewItem) requestreview.CareRequestResponse {
	r := item.Request
	blocks := make([]carerequests.Block, 0, len(item.AffectedBlocks))
	for _, block := range item.AffectedBlocks {
		blocks = append(blocks, carerequests.Block{ID: block.ID, Title: block.Title, StartTime: block.StartTime, EndTime: block.EndTime})
	}
	response := requestreviewcompose.ToCareRequestResponse(&carerequests.ReviewItem{
		Request:   &carerequests.Request{ID: r.ID, StudentID: r.StudentID, Status: r.Status, RequestKind: r.RequestKind, DecisionReason: r.DecisionReason, CreatedAt: r.CreatedAt, ReviewedAt: r.ReviewedAt},
		FirstName: item.FirstName, LastName: item.LastName, Diff: nativeCareDiffs(item.Diff), Reason: item.Reason, AffectedBlocks: blocks, ImpactAvailable: item.ImpactAvailable,
	})
	response.ImpactToken = item.ImpactToken
	return response
}

func decodeCareRequestDecision(w http.ResponseWriter, r *http.Request) (careschedule.CareRequestDecideInput, bool) {
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request id")
	if !ok {
		return careschedule.CareRequestDecideInput{}, false
	}
	var body DecideCareRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return careschedule.CareRequestDecideInput{}, false
	}
	if body.Approve == nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("approve is required")))
		return careschedule.CareRequestDecideInput{}, false
	}
	return careschedule.CareRequestDecideInput{
		RequestID: requestID, Approve: *body.Approve, Reason: body.Reason,
		ExpectedImpactToken: body.ImpactToken, RequireImpactToken: true,
		ExpectedVersion: body.ExpectedVersion,
	}, true
}

var careRequestDecisionErrorRenderer = common.RulesRenderer(parentRequestRules(
	common.ErrorRule{Target: scheduleModels.ErrCareRequestNotFound, Render: common.ErrorNotFound},
	common.ErrorRule{Target: scheduleModels.ErrCareRequestNotPending, Render: conflictWithCode("change_request_not_pending")},
	common.ErrorRule{Target: careschedule.ErrCareRequestGuardianAccessRevoked, Render: conflictWithCode("guardian_access_revoked")},
	common.ErrorRule{Target: careschedule.ErrCareRequestForbidden, Render: common.ErrorForbidden},
	common.ErrorRule{Target: careschedule.ErrPickupChangeConflict, Render: conflictWithCode("pickup_change_conflict")},
	common.ErrorRule{Target: careschedule.ErrPickupChangeAlreadyCompleted, Render: conflictWithCode("pickup_change_completed")},
	common.ErrorRule{Target: careschedule.ErrPickupChangeExpired, Render: conflictWithCode("pickup_change_expired")},
	common.ErrorRule{Target: careschedule.ErrPickupChangeImpactChanged, Render: conflictWithCode("pickup_change_impact_changed")},
	common.ErrorRule{Target: careschedule.ErrCareDayManagedByBooking, Render: conflictWithCode("care_day_managed_by_booking")},
	common.ErrorRule{Match: isInvalidCareRequestDecision, Render: common.ErrorInvalidRequest},
), careRequestDecisionFallback)

func conflictWithCode(code string) func(error) render.Renderer {
	return func(err error) render.Renderer { return common.ErrorConflictWithCode(err, code) }
}

func careRequestDecisionFallback(err error) render.Renderer {
	if renderer := companionPlanErrorRenderer(err); renderer != nil {
		return renderer
	}
	return common.ErrorInternalServer(err)
}

func isInvalidCareRequestDecision(err error) bool {
	return errors.Is(err, careschedule.ErrCareRequestRejectReasonRequired) ||
		errors.Is(err, careschedule.ErrCareRequestRejectReasonTooLong) ||
		errors.Is(err, careschedule.ErrInvalidCareRequestPayload) ||
		errors.Is(err, careschedule.ErrPickupChangeImpactRequired)
}
