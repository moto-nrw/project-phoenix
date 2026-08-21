package students

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// CareRequestResponse is the staff-facing projection of one parent
// care-schedule change request in the review queue, including the live
// "current → requested" weekly diff.
type CareRequestResponse struct {
	ID             string                    `json:"id"`
	StudentID      string                    `json:"student_id"`
	FirstName      string                    `json:"first_name"`
	LastName       string                    `json:"last_name"`
	Status         string                    `json:"status"`
	RequestKind    string                    `json:"request_kind"`
	Diff           []CareRequestDiffResponse `json:"diff"`
	RequestReason  *string                   `json:"request_reason,omitempty"`
	DecisionReason *string                   `json:"decision_reason,omitempty"`
	CreatedAt      time.Time                 `json:"created_at"`
	ReviewedAt     *time.Time                `json:"reviewed_at,omitempty"`
}

// CareRequestDiffResponse mirrors the request-diff wire shape the messaging
// thread page used, so the frontend's RequestDiffPanel renders it unchanged.
type CareRequestDiffResponse struct {
	Label    string   `json:"label"`
	Old      string   `json:"old"`
	New      string   `json:"new"`
	Weekday  int      `json:"weekday,omitempty"`
	CareKind string   `json:"care_kind,omitempty"`
	OldModes []string `json:"old_modes,omitempty"`
	NewMode  string   `json:"new_mode,omitempty"`
}

// toCareRequestDiffResponses maps service diff entries onto the wire shape —
// shared by the review queue and both history projections (frozen diff and
// requested summary; the latter's Old/OldModes are empty by construction).
func toCareRequestDiffResponses(entries []scheduleService.RequestDiffEntry) []CareRequestDiffResponse {
	out := make([]CareRequestDiffResponse, 0, len(entries))
	for _, e := range entries {
		out = append(out, CareRequestDiffResponse{
			Label:    e.Label,
			Old:      e.Old,
			New:      e.New,
			Weekday:  e.Weekday,
			CareKind: e.CareKind,
			OldModes: e.OldModes,
			NewMode:  e.NewMode,
		})
	}
	return out
}

func toCareRequestResponse(item *scheduleService.CareRequestReviewItem) CareRequestResponse {
	r := item.Request
	diff := toCareRequestDiffResponses(item.Diff)
	return CareRequestResponse{
		ID:             strconv.FormatInt(r.ID, 10),
		StudentID:      strconv.FormatInt(r.StudentID, 10),
		FirstName:      item.FirstName,
		LastName:       item.LastName,
		Status:         r.Status,
		RequestKind:    r.RequestKind,
		Diff:           diff,
		RequestReason:  item.Reason,
		DecisionReason: r.DecisionReason,
		CreatedAt:      r.CreatedAt,
		ReviewedAt:     r.ReviewedAt,
	}
}

// DecideCareRequestBody is the body of POST
// .../care-schedule-change-requests/{requestId}/decide.
type DecideCareRequestBody struct {
	Approve *bool  `json:"approve"`
	Reason  string `json:"reason"`
}

// decideCareScheduleChangeRequest approves (applies the weekly plan) or
// rejects (reason required) one pending request.
func (rs *Resource) decideCareScheduleChangeRequest(w http.ResponseWriter, r *http.Request) {
	if rs.CareRequestService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care request service not configured")))
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request id")
	if !ok {
		return
	}
	var body DecideCareRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if body.Approve == nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("approve is required")))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	item, err := rs.CareRequestService.Decide(r.Context(), scheduleService.CareRequestDecideInput{
		RequestID:  requestID,
		Approve:    *body.Approve,
		Reason:     body.Reason,
		ReviewedBy: int64(claims.ID),
	})
	if err != nil {
		switch {
		case errors.Is(err, scheduleModels.ErrCareRequestNotFound):
			renderError(w, r, common.ErrorNotFound(err))
		case errors.Is(err, scheduleModels.ErrCareRequestNotPending):
			renderError(w, r, common.ErrorConflictWithCode(err, "change_request_not_pending"))
		case errors.Is(err, scheduleService.ErrCareRequestGuardianAccessRevoked):
			renderError(w, r, common.ErrorConflictWithCode(err, "guardian_access_revoked"))
		case errors.Is(err, scheduleService.ErrCareRequestForbidden):
			renderError(w, r, common.ErrorForbidden(err))
		case errors.Is(err, scheduleService.ErrPickupChangeConflict):
			renderError(w, r, common.ErrorConflictWithCode(err, "pickup_change_conflict"))
		case errors.Is(err, scheduleService.ErrPickupChangeAlreadyCompleted):
			renderError(w, r, common.ErrorConflictWithCode(err, "pickup_change_completed"))
		case errors.Is(err, scheduleService.ErrPickupChangeExpired):
			renderError(w, r, common.ErrorConflictWithCode(err, "pickup_change_expired"))
		case errors.Is(err, scheduleService.ErrCareRequestRejectReasonRequired),
			errors.Is(err, scheduleService.ErrCareRequestRejectReasonTooLong),
			errors.Is(err, scheduleService.ErrInvalidCareRequestPayload):
			renderError(w, r, common.ErrorInvalidRequest(err))
		// Approving a care-schedule change rewrites the child's departure plan,
		// so it can strand or collide with a "läuft mit" link exactly like the
		// student PUT does. Both conditions are expected and actionable, not a
		// server failure (#1694).
		case companionPlanErrorRenderer(err) != nil:
			renderError(w, r, companionPlanErrorRenderer(err))
		default:
			renderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}

	common.Respond(w, r, http.StatusOK, toCareRequestResponse(item), "Decision applied")
}
