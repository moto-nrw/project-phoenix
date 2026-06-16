package guardians

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// inviteToStudentRequest is the wire shape for POST
// /guardians/students/{studentId}/invite.
type inviteToStudentRequest struct {
	Email            string `json:"email"`
	FirstName        string `json:"first_name"`
	LastName         string `json:"last_name"`
	RelationshipType string `json:"relationship_type"`
}

// inviteToStudentResponse echoes what the resolve did so the UI can show the
// right confirmation.
type inviteToStudentResponse struct {
	Outcome           string `json:"outcome"`
	GuardianProfileID string `json:"guardian_profile_id"`
	InvitationID      string `json:"invitation_id,omitempty"`
}

// pendingApprovalResponse is the parent-initiated approval-queue row, with
// guardian/child/requester names resolved for the staff queue.
type pendingApprovalResponse struct {
	ID                string    `json:"id"`
	GuardianProfileID string    `json:"guardian_profile_id"`
	GuardianName      string    `json:"guardian_name"`
	GuardianEmail     string    `json:"guardian_email,omitempty"`
	StudentID         string    `json:"student_id,omitempty"`
	StudentName       string    `json:"student_name,omitempty"`
	RequestedByEmail  string    `json:"requested_by_email,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	ExpiresAt         time.Time `json:"expires_at"`
}

// inviteGuardianToStudent invites a further guardian to a child by email.
// Staff are always allowed (permission-gated by users:create); the resolve
// runs immediately (no approval queue for staff).
func (rs *Resource) inviteGuardianToStudent(w http.ResponseWriter, r *http.Request) {
	studentID, ok := parseInt64Param(w, r, "studentId")
	if !ok {
		return
	}
	accountID, ok := actingAccountID(w, r)
	if !ok {
		return
	}
	canModify, err := rs.canModifyStudent(r.Context(), studentID)
	if !canModify {
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}

	var body inviteToStudentRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}

	result, err := rs.InvitationService.InviteToStudent(r.Context(), authSvc.InviteToStudentRequest{
		StudentID:        studentID,
		Email:            body.Email,
		FirstName:        body.FirstName,
		LastName:         body.LastName,
		RelationshipType: body.RelationshipType,
		CreatedBy:        accountID,
		RequireApproval:  false,
	})
	if err != nil {
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, toInviteResponse(result), "Guardian invited")
}

// listPendingApprovals returns parent-initiated invitations awaiting staff
// approval for the current tenant.
func (rs *Resource) listPendingApprovals(w http.ResponseWriter, r *http.Request) {
	views, err := rs.InvitationService.ListPendingApprovalsDetailed(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	out := make([]pendingApprovalResponse, 0, len(views))
	for _, v := range views {
		out = append(out, toPendingApprovalResponse(v))
	}
	common.Respond(w, r, http.StatusOK, out, "Pending approvals")
}

// approveInvitation approves a parent-initiated request: links the child and
// grants access (existing account) or dispatches the invitation email.
func (rs *Resource) approveInvitation(w http.ResponseWriter, r *http.Request) {
	invitationID, ok := parseInt64Param(w, r, "invitationId")
	if !ok {
		return
	}
	accountID, ok := actingAccountID(w, r)
	if !ok {
		return
	}
	studentID, err := rs.InvitationService.PendingInvitationStudentID(r.Context(), invitationID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	canModify, err := rs.canModifyStudent(r.Context(), studentID)
	if !canModify {
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}
	if err := rs.InvitationService.ApproveInvitation(r.Context(), invitationID, accountID); err != nil {
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Invitation approved")
}

// rejectInvitation rejects a parent-initiated request. No access is granted.
func (rs *Resource) rejectInvitation(w http.ResponseWriter, r *http.Request) {
	invitationID, ok := parseInt64Param(w, r, "invitationId")
	if !ok {
		return
	}
	accountID, ok := actingAccountID(w, r)
	if !ok {
		return
	}
	studentID, err := rs.InvitationService.PendingInvitationStudentID(r.Context(), invitationID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	canModify, err := rs.canModifyStudent(r.Context(), studentID)
	if !canModify {
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}
	if err := rs.InvitationService.RejectInvitation(r.Context(), invitationID, accountID); err != nil {
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Invitation rejected")
}

// --- helpers ---

func toInviteResponse(result *authSvc.InviteToStudentResult) inviteToStudentResponse {
	resp := inviteToStudentResponse{
		Outcome:           string(result.Outcome),
		GuardianProfileID: strconv.FormatInt(result.GuardianProfileID, 10),
	}
	if result.InvitationID != nil {
		resp.InvitationID = strconv.FormatInt(*result.InvitationID, 10)
	}
	return resp
}

func toPendingApprovalResponse(v *authSvc.PendingApprovalView) pendingApprovalResponse {
	resp := pendingApprovalResponse{
		ID:                strconv.FormatInt(v.InvitationID, 10),
		GuardianProfileID: strconv.FormatInt(v.GuardianProfileID, 10),
		GuardianName:      v.GuardianName,
		GuardianEmail:     v.GuardianEmail,
		StudentName:       v.StudentName,
		RequestedByEmail:  v.RequestedByEmail,
		CreatedAt:         v.CreatedAt,
		ExpiresAt:         v.ExpiresAt,
	}
	if v.StudentID > 0 {
		resp.StudentID = strconv.FormatInt(v.StudentID, 10)
	}
	return resp
}

// parseInt64Param reads a positive int64 chi URL param, rendering a 400 on
// failure.
func parseInt64Param(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := chi.URLParam(r, name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid "+name)))
		return 0, false
	}
	return id, true
}

// actingAccountID reads the authenticated account id from the JWT claims.
func actingAccountID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("user not authenticated")))
		return 0, false
	}
	return int64(claims.ID), true
}
