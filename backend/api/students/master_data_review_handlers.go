package students

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/render"
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
	common.ErrorRule{Target: userService.ErrReviewNotFound, Render: common.ErrorNotFound},
	common.ErrorRule{Target: userService.ErrReviewForbidden, Render: common.ErrorForbidden},
	common.ErrorRule{Target: userService.ErrReviewNotPending, Render: conflictWithCode("change_request_not_pending")},
	common.ErrorRule{Target: userService.ErrReviewStaleValue, Render: conflictWithCode(codeChangeRequestStale)},
	common.ErrorRule{Target: userService.ErrReviewInvalidTarget, Render: common.ErrorInvalidRequest},
	common.ErrorRule{Target: userService.ErrReviewInvalidValue, Render: common.ErrorInvalidRequest},
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

	claims := jwt.ClaimsFromCtx(r.Context())
	item, err := rs.MasterDataReviewService.Decide(r.Context(), userService.MasterDataReviewDecideInput{
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

	common.Respond(w, r, http.StatusOK, toMasterDataChangeRequestResponse(item), "Decision applied")
}
