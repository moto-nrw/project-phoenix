package students

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	absenceService "github.com/moto-nrw/project-phoenix/services/absence"
)

// StaffExcusedRequestResponse is the staff-facing projection of one parent
// excused-absence approval request in the review queue (#1845).
type StaffExcusedRequestResponse struct {
	ID         string     `json:"id"`
	StudentID  string     `json:"student_id"`
	FirstName  string     `json:"first_name"`
	LastName   string     `json:"last_name"`
	Status     string     `json:"status"`
	Dates      []string   `json:"dates"`
	Note       string     `json:"note"`
	Reason     *string    `json:"reason,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
}

func toStaffExcusedRequestResponse(item *absenceService.ExcusedRequestReviewItem) StaffExcusedRequestResponse {
	r := item.Request
	dates := make([]string, 0, len(r.Dates))
	for _, d := range r.Dates {
		dates = append(dates, d.String())
	}
	return StaffExcusedRequestResponse{
		ID:         strconv.FormatInt(r.ID, 10),
		StudentID:  strconv.FormatInt(r.StudentID, 10),
		FirstName:  item.FirstName,
		LastName:   item.LastName,
		Status:     r.Status,
		Dates:      dates,
		Note:       r.Note,
		Reason:     r.DecisionReason,
		CreatedAt:  r.CreatedAt,
		ReviewedAt: r.ReviewedAt,
	}
}

// listExcusedAbsenceRequests returns the tenant's pending parent excused-absence
// approval requests for the staff review queue.
func (rs *Resource) listExcusedAbsenceRequests(w http.ResponseWriter, r *http.Request) {
	if rs.ExcusedRequestService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("excused request service not configured")))
		return
	}
	items, err := rs.ExcusedRequestService.ListPending(r.Context())
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	out := make([]StaffExcusedRequestResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toStaffExcusedRequestResponse(item))
	}
	common.Respond(w, r, http.StatusOK, out, "Excused absence requests retrieved")
}

// DecideExcusedRequestBody is the body of POST
// .../excused-absence-requests/{requestId}/decide.
type DecideExcusedRequestBody struct {
	Approve *bool  `json:"approve"`
	Reason  string `json:"reason"`
}

// decideExcusedAbsenceRequest approves (writes the excused status days) or
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
	item, err := rs.ExcusedRequestService.Decide(r.Context(), absenceService.ExcusedRequestDecideInput{
		RequestID:  requestID,
		Approve:    *body.Approve,
		Reason:     body.Reason,
		ReviewedBy: int64(claims.ID),
	})
	if err != nil {
		switch {
		case errors.Is(err, activeModels.ErrExcusedRequestNotFound):
			renderError(w, r, common.ErrorNotFound(err))
		case errors.Is(err, activeModels.ErrExcusedRequestNotPending):
			renderError(w, r, common.ErrorConflictWithCode(err, "change_request_not_pending"))
		case errors.Is(err, absenceService.ErrExcusedRequestGuardianAccessRevoked):
			renderError(w, r, common.ErrorConflictWithCode(err, "guardian_access_revoked"))
		case errors.Is(err, absenceService.ErrExcusedRequestStatusConflict):
			renderError(w, r, common.ErrorConflictWithCode(err, "excused_request_status_conflict"))
		case errors.Is(err, absenceService.ErrExcusedRequestForbidden):
			renderError(w, r, common.ErrorForbidden(err))
		case errors.Is(err, absenceService.ErrExcusedRequestRejectReasonRequired),
			errors.Is(err, absenceService.ErrExcusedRequestRejectReasonTooLong):
			renderError(w, r, common.ErrorInvalidRequest(err))
		default:
			renderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}

	common.Respond(w, r, http.StatusOK, toStaffExcusedRequestResponse(item), "Decision applied")
}
