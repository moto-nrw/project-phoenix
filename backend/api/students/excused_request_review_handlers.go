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
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	absenceService "github.com/moto-nrw/project-phoenix/services/absence"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// StaffExcusedRequestResponse is the legacy-named staff projection of one
// parent absence approval request in the review queue.
type StaffExcusedRequestResponse struct {
	ID            string     `json:"id"`
	StudentID     string     `json:"student_id"`
	FirstName     string     `json:"first_name"`
	LastName      string     `json:"last_name"`
	AbsenceStatus string     `json:"absence_status"`
	Status        string     `json:"status"`
	Dates         []string   `json:"dates"`
	Note          string     `json:"note"`
	Reason        *string    `json:"reason,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	ReviewedAt    *time.Time `json:"reviewed_at,omitempty"`
}

func toStaffExcusedRequestResponse(item *absenceService.ExcusedRequestReviewItem) StaffExcusedRequestResponse {
	r := item.Request
	dates := make([]string, 0, len(r.Dates))
	for _, d := range r.Dates {
		dates = append(dates, d.String())
	}
	return StaffExcusedRequestResponse{
		ID:            strconv.FormatInt(r.ID, 10),
		StudentID:     strconv.FormatInt(r.StudentID, 10),
		FirstName:     item.FirstName,
		LastName:      item.LastName,
		AbsenceStatus: r.AbsenceStatus,
		Status:        r.Status,
		Dates:         dates,
		Note:          r.Note,
		Reason:        r.DecisionReason,
		CreatedAt:     r.CreatedAt,
		ReviewedAt:    r.ReviewedAt,
	}
}

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
	common.ErrorRule{Target: activeModels.ErrExcusedRequestNotFound, Render: common.ErrorNotFound},
	common.ErrorRule{Target: activeModels.ErrExcusedRequestNotPending, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, "change_request_not_pending")
	}},
	common.ErrorRule{Target: absenceService.ErrExcusedRequestGuardianAccessRevoked, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, "guardian_access_revoked")
	}},
	common.ErrorRule{Target: absenceService.ErrExcusedRequestStatusConflict, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, "excused_request_status_conflict")
	}},
	common.ErrorRule{Target: absenceService.ErrExcusedRequestForbidden, Render: common.ErrorForbidden},
	common.ErrorRule{Target: absenceService.ErrExcusedRequestRejectReasonRequired, Render: common.ErrorInvalidRequest},
	common.ErrorRule{Target: absenceService.ErrExcusedRequestRejectReasonTooLong, Render: common.ErrorInvalidRequest},
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
	item, err := rs.ExcusedRequestService.Decide(r.Context(), absenceService.ExcusedRequestDecideInput{
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

	common.Respond(w, r, http.StatusOK, toStaffExcusedRequestResponse(item), "Decision applied")
}
