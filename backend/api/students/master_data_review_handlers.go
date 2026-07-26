package students

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// MasterDataChangeRequestResponse is the staff-facing projection of one parent
// Stammdaten change request in the review queue.
type MasterDataChangeRequestResponse struct {
	ID         string          `json:"id"`
	StudentID  string          `json:"student_id"`
	FirstName  string          `json:"first_name"`
	LastName   string          `json:"last_name"`
	Target     string          `json:"target"`
	FieldKey   string          `json:"field_key"`
	OldValue   json.RawMessage `json:"old_value,omitempty"`
	NewValue   json.RawMessage `json:"new_value"`
	Status     string          `json:"status"`
	CreatedAt  time.Time       `json:"created_at"`
	ReviewedAt *time.Time      `json:"reviewed_at,omitempty"`
}

func toMasterDataChangeRequestResponse(item *userService.MasterDataReviewItem) MasterDataChangeRequestResponse {
	r := item.Request
	return MasterDataChangeRequestResponse{
		ID:         strconv.FormatInt(r.ID, 10),
		StudentID:  strconv.FormatInt(r.StudentID, 10),
		FirstName:  item.FirstName,
		LastName:   item.LastName,
		Target:     r.Target,
		FieldKey:   r.FieldKey,
		OldValue:   r.OldValue,
		NewValue:   r.NewValue,
		Status:     r.Status,
		CreatedAt:  r.CreatedAt,
		ReviewedAt: r.ReviewedAt,
	}
}

// listMasterDataChangeRequests returns the tenant's pending parent change
// requests for the staff review queue.
func (rs *Resource) listMasterDataChangeRequests(w http.ResponseWriter, r *http.Request) {
	if rs.MasterDataReviewService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("master data review service not configured")))
		return
	}
	items, err := rs.MasterDataReviewService.ListPending(r.Context())
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	out := make([]MasterDataChangeRequestResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toMasterDataChangeRequestResponse(item))
	}
	common.Respond(w, r, http.StatusOK, out, "Change requests retrieved")
}

// DecideMasterDataChangeRequestBody is the body of POST
// .../master-data-change-requests/{requestId}/decide.
type DecideMasterDataChangeRequestBody struct {
	Approve *bool  `json:"approve"`
	Reason  string `json:"reason"`
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

	claims := jwt.ClaimsFromCtx(r.Context())
	item, err := rs.MasterDataReviewService.Decide(r.Context(), userService.MasterDataReviewDecideInput{
		RequestID:  requestID,
		Approve:    *body.Approve,
		Reason:     body.Reason,
		ReviewedBy: int64(claims.ID),
	})
	if err != nil {
		switch {
		case errors.Is(err, userService.ErrReviewNotFound):
			renderError(w, r, common.ErrorNotFound(err))
		case errors.Is(err, userService.ErrReviewForbidden):
			renderError(w, r, common.ErrorForbidden(err))
		case errors.Is(err, userService.ErrReviewNotPending):
			renderError(w, r, common.ErrorConflictWithCode(err, "change_request_not_pending"))
		case errors.Is(err, userService.ErrReviewStaleValue):
			renderError(w, r, common.ErrorConflictWithCode(err, "change_request_stale"))
		case errors.Is(err, userService.ErrReviewInvalidTarget),
			errors.Is(err, userService.ErrReviewInvalidValue):
			renderError(w, r, common.ErrorInvalidRequest(err))
		// Approving an allowed_departure_modes change rewrites the child's
		// departure plan, so it can strand or collide with a "läuft mit" link
		// exactly like the student PUT does. Both conditions are expected and
		// actionable, not a server failure (#1694).
		case companionPlanErrorRenderer(err) != nil:
			renderError(w, r, companionPlanErrorRenderer(err))
		default:
			renderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}

	common.Respond(w, r, http.StatusOK, toMasterDataChangeRequestResponse(item), "Decision applied")
}
