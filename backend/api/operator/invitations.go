package operator

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	emailpkg "github.com/moto-nrw/project-phoenix/email"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// OperatorsResource handles operator listing and invitation endpoints
type OperatorsResource struct {
	authService       platformSvc.OperatorAuthService
	invitationService platformSvc.OperatorInvitationService
}

// NewOperatorsResource creates a new operators resource
func NewOperatorsResource(authService platformSvc.OperatorAuthService, invitationService platformSvc.OperatorInvitationService) *OperatorsResource {
	return &OperatorsResource{
		authService:       authService,
		invitationService: invitationService,
	}
}

// --- Request DTOs ---

// CreateOperatorInvitationRequest represents the invitation creation request body
type CreateOperatorInvitationRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// Bind validates the create invitation request
func (req *CreateOperatorInvitationRequest) Bind(r *http.Request) error {
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		return errors.New("email is required")
	}
	return nil
}

// AcceptOperatorInvitationRequest represents the invitation acceptance request body
type AcceptOperatorInvitationRequest struct {
	Token       string `json:"token"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

// Bind validates the accept invitation request
func (req *AcceptOperatorInvitationRequest) Bind(r *http.Request) error {
	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" {
		return errors.New("token is required")
	}
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.DisplayName == "" {
		return errors.New("display_name is required")
	}
	if req.Password == "" {
		return errors.New("password is required")
	}
	return nil
}

// --- Response DTOs ---

// OperatorListItem represents an operator in the operators list
type OperatorListItem struct {
	ID          int64      `json:"id"`
	Email       string     `json:"email"`
	DisplayName string     `json:"display_name"`
	Active      bool       `json:"active"`
	LastLogin   *time.Time `json:"last_login,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// PendingInvitationResponse represents a pending invitation in the list
type PendingInvitationResponse struct {
	ID              int64      `json:"id"`
	Email           string     `json:"email"`
	DisplayName     *string    `json:"display_name,omitempty"`
	InvitedBy       int64      `json:"invited_by"`
	InviterName     string     `json:"inviter_name"`
	ExpiresAt       string     `json:"expires_at"`
	CreatedAt       string     `json:"created_at"`
	DeliveryStatus  string     `json:"delivery_status"`
	EmailSentAt     *time.Time `json:"email_sent_at,omitempty"`
	EmailError      *string    `json:"email_error,omitempty"`
	EmailRetryCount int        `json:"email_retry_count"`
}

// --- Protected Handlers ---

// ListOperators handles GET /operator/operators
func (rs *OperatorsResource) ListOperators(w http.ResponseWriter, r *http.Request) {
	operators, err := rs.authService.ListOperators(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrInternal("Failed to list operators"))
		return
	}

	response := make([]OperatorListItem, 0, len(operators))
	for _, op := range operators {
		response = append(response, OperatorListItem{
			ID:          op.ID,
			Email:       op.Email,
			DisplayName: op.DisplayName,
			Active:      op.Active,
			LastLogin:   op.LastLogin,
			CreatedAt:   op.CreatedAt,
		})
	}

	common.Respond(w, r, http.StatusOK, response, "Operators listed successfully")
}

// CreateInvitation handles POST /operator/invitations
func (rs *OperatorsResource) CreateInvitation(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	inviterID := int64(claims.ID)

	req := &CreateOperatorInvitationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	token, err := rs.invitationService.CreateInvitation(r.Context(), req.Email, req.DisplayName, inviterID)
	if err != nil {
		common.RenderError(w, r, InvitationErrorRenderer(err))
		return
	}

	inviterName := ""
	if token.Inviter != nil {
		inviterName = token.Inviter.DisplayName
	}

	response := &PendingInvitationResponse{
		ID:              token.ID,
		Email:           token.Email,
		DisplayName:     token.DisplayName,
		InvitedBy:       token.InvitedBy,
		InviterName:     inviterName,
		ExpiresAt:       token.Expiry.Format(time.RFC3339),
		CreatedAt:       token.CreatedAt.Format(time.RFC3339),
		DeliveryStatus:  string(emailpkg.DeliveryStatusPending),
		EmailRetryCount: 0,
	}

	common.Respond(w, r, http.StatusCreated, response, "Invitation created successfully")
}

// ListPendingInvitations handles GET /operator/invitations
func (rs *OperatorsResource) ListPendingInvitations(w http.ResponseWriter, r *http.Request) {
	invitations, err := rs.invitationService.ListPendingInvitations(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrInternal("Failed to list invitations"))
		return
	}

	response := make([]PendingInvitationResponse, 0, len(invitations))
	for _, inv := range invitations {
		inviterName := ""
		if inv.Inviter != nil {
			inviterName = inv.Inviter.DisplayName
		}

		response = append(response, PendingInvitationResponse{
			ID:              inv.ID,
			Email:           inv.Email,
			DisplayName:     inv.DisplayName,
			InvitedBy:       inv.InvitedBy,
			InviterName:     inviterName,
			ExpiresAt:       inv.Expiry.Format(time.RFC3339),
			CreatedAt:       inv.CreatedAt.Format(time.RFC3339),
			DeliveryStatus:  deriveInvitationDeliveryStatus(inv.EmailSentAt, inv.EmailError),
			EmailSentAt:     inv.EmailSentAt,
			EmailError:      inv.EmailError,
			EmailRetryCount: inv.EmailRetryCount,
		})
	}

	common.Respond(w, r, http.StatusOK, response, "Invitations listed successfully")
}

// ResendInvitation handles POST /operator/invitations/{id}/resend
func (rs *OperatorsResource) ResendInvitation(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	actorID := int64(claims.ID)

	invitationID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		common.RenderError(w, r, ErrInvalidRequest(errors.New("invalid invitation ID")))
		return
	}

	if err := rs.invitationService.ResendInvitation(r.Context(), invitationID, actorID); err != nil {
		common.RenderError(w, r, InvitationErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Invitation resent successfully")
}

// RevokeInvitation handles DELETE /operator/invitations/{id}
func (rs *OperatorsResource) RevokeInvitation(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	actorID := int64(claims.ID)

	invitationID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		common.RenderError(w, r, ErrInvalidRequest(errors.New("invalid invitation ID")))
		return
	}

	if err := rs.invitationService.RevokeInvitation(r.Context(), invitationID, actorID); err != nil {
		common.RenderError(w, r, InvitationErrorRenderer(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Public Handlers (no auth) ---

// ValidateInvitationRequest is the request body for POST /operator/auth/invite-validate
type ValidateInvitationRequest struct {
	Token string `json:"token"`
}

// Bind validates the validate invitation request
func (req *ValidateInvitationRequest) Bind(r *http.Request) error {
	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" {
		return errors.New("token is required")
	}
	return nil
}

// ValidateInvitation handles POST /operator/auth/invite-validate
// Token is sent in the request body to prevent leaking into server access logs.
func (rs *OperatorsResource) ValidateInvitation(w http.ResponseWriter, r *http.Request) {
	req := &ValidateInvitationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	result, err := rs.invitationService.ValidateInvitation(r.Context(), req.Token)
	if err != nil {
		common.RenderError(w, r, InvitationAcceptErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, result, "Invitation is valid")
}

// AcceptInvitation handles POST /operator/auth/invite-accept
func (rs *OperatorsResource) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	req := &AcceptOperatorInvitationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	clientIP := getClientIP(r)
	operator, err := rs.invitationService.AcceptInvitation(r.Context(), req.Token, req.DisplayName, req.Password, clientIP)
	if err != nil {
		common.RenderError(w, r, InvitationAcceptErrorRenderer(err))
		return
	}

	response := map[string]any{
		"operator_id": operator.ID,
		"email":       operator.Email,
	}

	common.Respond(w, r, http.StatusCreated, response, "Operator account created successfully")
}

// --- Helpers ---

func deriveInvitationDeliveryStatus(sentAt *time.Time, emailError *string) string {
	if sentAt != nil {
		return string(emailpkg.DeliveryStatusSent)
	}
	if emailError != nil && strings.TrimSpace(*emailError) != "" {
		return string(emailpkg.DeliveryStatusFailed)
	}
	return string(emailpkg.DeliveryStatusPending)
}
