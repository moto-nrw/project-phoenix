package enrollment

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	studentsAPI "github.com/moto-nrw/project-phoenix/api/students"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type ChangeRequestResponse struct {
	ID                string                         `json:"id"`
	RequestID         string                         `json:"request_id"`
	Origin            string                         `json:"origin"`
	Status            string                         `json:"status"`
	ParentNote        *string                        `json:"parent_note,omitempty"`
	AdminDecisionNote *string                        `json:"admin_decision_note,omitempty"`
	BaseSnapshot      map[string]any                 `json:"base_snapshot"`
	ProposedSnapshot  map[string]any                 `json:"proposed_snapshot"`
	Diff              map[string]any                 `json:"diff"`
	CreatedAt         time.Time                      `json:"created_at"`
	UpdatedAt         time.Time                      `json:"updated_at"`
	ReviewedAt        *time.Time                     `json:"reviewed_at,omitempty"`
	ReviewedBy        *int64                         `json:"reviewed_by_account_id,omitempty"`
	Request           *AdminRequestSummary           `json:"request,omitempty"`
	Messages          []ChangeRequestMessageResponse `json:"messages,omitempty"`
}

type ChangeRequestMessageResponse struct {
	ID              string    `json:"id"`
	AuthorType      string    `json:"author_type"`
	AuthorAccountID *int64    `json:"author_account_id,omitempty"`
	Body            string    `json:"body"`
	InternalOnly    bool      `json:"internal_only"`
	CreatedAt       time.Time `json:"created_at"`
}

type CreateChangeRequestRequest struct {
	SubmitEnrollmentRequest
	ParentNote string `json:"parent_note,omitempty"`
}

func (req *CreateChangeRequestRequest) Bind(r *http.Request) error {
	return req.SubmitEnrollmentRequest.Bind(r)
}

type ChangeRequestMessageRequest struct {
	Body string `json:"body"`
}

func (req *ChangeRequestMessageRequest) Bind(_ *http.Request) error { return nil }

type ReviewChangeRequestRequest struct {
	Note string `json:"note"`
}

func (req *ReviewChangeRequestRequest) Bind(_ *http.Request) error { return nil }

func (rs *Resource) createChangeRequest(w http.ResponseWriter, r *http.Request) {
	if rs.ChangeRequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("change request service not configured")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "statusToken"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("status token is required")))
		return
	}
	body := &CreateChangeRequestRequest{}
	if err := render.Bind(r, body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	serviceReq, err := BuildServiceRequest(&body.SubmitEnrollmentRequest, 0, "")
	if err != nil {
		MapSubmitError(w, r, err)
		return
	}
	agg, err := rs.ChangeRequestService.Create(r.Context(), token, enrollmentService.CreateChangeRequestInput{
		Submission: serviceReq,
		ParentNote: body.ParentNote,
	})
	if err != nil {
		mapChangeRequestError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toChangeRequestResponse(agg, false, false), "Change request created")
}

func (rs *Resource) listPublicChangeRequests(w http.ResponseWriter, r *http.Request) {
	if rs.ChangeRequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("change request service not configured")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "statusToken"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("status token is required")))
		return
	}
	rows, err := rs.ChangeRequestService.ListPublic(r.Context(), token)
	if err != nil {
		mapChangeRequestError(w, r, err)
		return
	}
	out := make([]ChangeRequestResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toChangeRequestResponse(row, false, false))
	}
	common.Respond(w, r, http.StatusOK, out, "Change requests retrieved")
}

func (rs *Resource) replyToChangeRequest(w http.ResponseWriter, r *http.Request) {
	if rs.ChangeRequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("change request service not configured")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "statusToken"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid change request")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "changeRequestId", "invalid change request")
	if !ok {
		return
	}
	body := &ChangeRequestMessageRequest{}
	if err := render.Bind(r, body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	agg, err := rs.ChangeRequestService.ParentReply(r.Context(), token, id, enrollmentService.ChangeRequestMessageInput{
		Body: body.Body,
	})
	if err != nil {
		mapChangeRequestError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toChangeRequestResponse(agg, false, false), "Change request reply created")
}

func (rs *Resource) listAdminChangeRequests(w http.ResponseWriter, r *http.Request) {
	if rs.ChangeRequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("change request service not configured")))
		return
	}
	filters := enrollmentService.ChangeRequestFilters{}
	if v := r.URL.Query().Get("request_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request_id")))
			return
		}
		filters.RequestID = id
	}
	filters.Status = strings.TrimSpace(r.URL.Query().Get("status"))

	var rows []*enrollmentService.ChangeRequestAggregate
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		list, listErr := rs.ChangeRequestService.ListAdmin(ctx, filters)
		rows = list
		return listErr
	})
	if err != nil {
		mapChangeRequestError(w, r, err)
		return
	}
	out := make([]ChangeRequestResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toChangeRequestResponse(row, true, true))
	}
	common.Respond(w, r, http.StatusOK, out, "Admin change requests retrieved")
}

func (rs *Resource) getAdminChangeRequest(w http.ResponseWriter, r *http.Request) {
	id, ok := changeRequestIDParam(w, r)
	if !ok {
		return
	}
	var agg *enrollmentService.ChangeRequestAggregate
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		row, getErr := rs.ChangeRequestService.GetAdmin(ctx, id)
		agg = row
		return getErr
	})
	if err != nil {
		mapChangeRequestError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toChangeRequestResponse(agg, true, true), "Admin change request retrieved")
}

func (rs *Resource) askChangeRequestQuestion(w http.ResponseWriter, r *http.Request) {
	id, ok := changeRequestIDParam(w, r)
	if !ok {
		return
	}
	body := &ChangeRequestMessageRequest{}
	if err := render.Bind(r, body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	var agg *enrollmentService.ChangeRequestAggregate
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		row, askErr := rs.ChangeRequestService.AskQuestion(ctx, id, enrollmentService.ChangeRequestMessageInput{
			Body:           body.Body,
			ActorAccountID: int64(claims.ID),
		})
		agg = row
		return askErr
	})
	if err != nil {
		mapChangeRequestError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toChangeRequestResponse(agg, true, true), "Change request question created")
}

func (rs *Resource) approveChangeRequest(w http.ResponseWriter, r *http.Request) {
	rs.reviewChangeRequest(w, r, true)
}

func (rs *Resource) rejectChangeRequest(w http.ResponseWriter, r *http.Request) {
	rs.reviewChangeRequest(w, r, false)
}

func (rs *Resource) reviewChangeRequest(w http.ResponseWriter, r *http.Request, approve bool) {
	id, ok := changeRequestIDParam(w, r)
	if !ok {
		return
	}
	body := &ReviewChangeRequestRequest{}
	if err := render.Bind(r, body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	input := enrollmentService.ReviewChangeRequestInput{
		Note:           body.Note,
		ActorAccountID: int64(claims.ID),
		ActorRole:      actorRoleFromClaims(claims.Roles),
	}
	var agg *enrollmentService.ChangeRequestAggregate
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		var reviewErr error
		if approve {
			agg, reviewErr = rs.ChangeRequestService.Approve(ctx, id, input)
		} else {
			agg, reviewErr = rs.ChangeRequestService.Reject(ctx, id, input)
		}
		return reviewErr
	})
	if err != nil {
		mapChangeRequestWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toChangeRequestResponse(agg, true, true), "Change request reviewed")
}

func changeRequestIDParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
}

func toChangeRequestResponse(agg *enrollmentService.ChangeRequestAggregate, includeRequest bool, includeInternal bool) ChangeRequestResponse {
	if agg == nil || agg.ChangeRequest == nil {
		return ChangeRequestResponse{}
	}
	row := agg.ChangeRequest
	resp := ChangeRequestResponse{
		ID:                strconv.FormatInt(row.ID, 10),
		RequestID:         strconv.FormatInt(row.RequestID, 10),
		Origin:            row.Origin,
		Status:            row.Status,
		ParentNote:        row.ParentNote,
		AdminDecisionNote: row.AdminDecisionNote,
		BaseSnapshot:      row.BaseSnapshot,
		ProposedSnapshot:  row.ProposedSnapshot,
		Diff:              row.Diff,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
		ReviewedAt:        row.ReviewedAt,
	}
	if includeInternal {
		resp.ReviewedBy = row.ReviewedByAccountID
	}
	for _, msg := range agg.Messages {
		out := ChangeRequestMessageResponse{
			ID:           strconv.FormatInt(msg.ID, 10),
			AuthorType:   msg.AuthorType,
			Body:         msg.Body,
			InternalOnly: msg.InternalOnly,
			CreatedAt:    msg.CreatedAt,
		}
		if includeInternal {
			out.AuthorAccountID = msg.AuthorAccountID
		}
		resp.Messages = append(resp.Messages, out)
	}
	if includeRequest && agg.Request != nil {
		summary := &enrollmentService.RequestSummary{
			Request:  agg.Request,
			Phase:    agg.Phase,
			Children: agg.Children,
		}
		request := toAdminRequestSummary(summary)
		resp.Request = &request
	}
	return resp
}

// mapChangeRequestWriteError is mapChangeRequestError for the handlers that
// MUTATE (approve/reject, admin data correction).
//
// runInTenantTx only REUSES the transaction TenantTxMiddleware opened for the
// request (tenant/tx.go), so returning an error from the closure rolls nothing
// back, and the middleware commits on every response below 500. Both flows
// write before their last validation can fail: applyApprovedChange updates the
// guardian and child rows before SyncApprovedChildData rebuilds the departure
// plan, and that rebuild is what raises the companion sentinels (400/409). Left
// alone, a refused review would answer 400 and still commit the guardian,
// contact, schedule, and child writes while the change request stays pending.
// A refused write must leave nothing behind, for the same reason updateStudent
// marks the rollback itself.
func mapChangeRequestWriteError(w http.ResponseWriter, r *http.Request, err error) {
	tenant.MarkRollback(r.Context())
	mapChangeRequestError(w, r, err)
}

func mapChangeRequestError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, enrollmentService.ErrRequestNotFound),
		errors.Is(err, enrollmentService.ErrChangeRequestNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, enrollmentService.ErrChangeRequestChildLocked):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "enrollment.change_request_child_locked"))
	case errors.Is(err, enrollmentService.ErrChangeRequestNotAllowed),
		errors.Is(err, enrollmentService.ErrEditNotAllowed):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, enrollmentService.ErrChangeRequestConflict):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "enrollment.change_request_conflict"))
	case errors.Is(err, enrollmentService.ErrChangeRequestInvalidStatus),
		errors.Is(err, enrollmentService.ErrChangeRequestInvalidData):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	// Approving a change (or correcting child data) replaces the student's
	// departure plan through StudentRepository.Update, which reconciles the
	// "läuft mit" links and can refuse with the two companion sentinels. Both
	// are expected, user-actionable refusals — map them to the same status +
	// wire code the student PUT answers with, and render the bare sentinel so
	// the client shows its clean German instruction instead of the wrapped
	// sync context (#1694).
	case errors.Is(err, userModels.ErrCompanionWouldLoseDeparture):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(userModels.ErrCompanionWouldLoseDeparture, studentsAPI.CodeCompanionWouldLoseDeparture))
	case errors.Is(err, userModels.ErrCompanionLockBusy):
		common.RenderError(w, r, common.ErrorConflictWithCode(userModels.ErrCompanionLockBusy, studentsAPI.CodeCompanionLockBusy))
	default:
		mapEditError(w, r, err)
	}
}
