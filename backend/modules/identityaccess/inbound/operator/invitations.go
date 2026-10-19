package operator

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// InvitationsResource handles operator invitation endpoints. Depends on the
// narrow OperatorInvitationService interface rather than the full
// OperatorAuthService, so tests can mock only the methods this handler
// actually calls.
type InvitationsResource struct {
	invitationService OperatorAccess
}

// NewInvitationsResource creates a new invitations resource
func NewInvitationsResource(invitationService OperatorAccess) *InvitationsResource {
	return &InvitationsResource{
		invitationService: invitationService,
	}
}

// --- Request types ---

// CreateInvitationRequest represents the create invitation request body
type CreateInvitationRequest struct {
	Email       string  `json:"email"`
	DisplayName *string `json:"display_name,omitempty"`
}

// Bind validates the create invitation request
func (req *CreateInvitationRequest) Bind(r *http.Request) error {
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		return errors.New("email is required")
	}
	return nil
}

// ValidateInvitationRequest represents the validate invitation request body
type ValidateInvitationRequest struct {
	Token string `json:"token"`
}

// Bind validates the validate invitation request
func (req *ValidateInvitationRequest) Bind(r *http.Request) error {
	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" {
		return errors.New("token is required")
	}
	if _, err := uuid.FromString(req.Token); err != nil {
		return errors.New("invalid token format")
	}
	return nil
}

// AcceptInvitationRequest represents the accept invitation request body
type AcceptInvitationRequest struct {
	Token           string `json:"token"`
	DisplayName     string `json:"display_name"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

// Bind validates the accept invitation request
func (req *AcceptInvitationRequest) Bind(r *http.Request) error {
	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" {
		return errors.New("token is required")
	}
	if _, err := uuid.FromString(req.Token); err != nil {
		return errors.New("invalid token format")
	}
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.DisplayName == "" {
		return errors.New("display_name is required")
	}
	if req.Password == "" {
		return errors.New("password is required")
	}
	if req.Password != req.ConfirmPassword {
		return errors.New("passwords do not match")
	}
	return nil
}

// --- Response types ---

// InvitationResponse represents a pending invitation in API responses
type InvitationResponse struct {
	ID              int64      `json:"id"`
	Email           string     `json:"email"`
	DisplayName     *string    `json:"display_name,omitempty"`
	CreatedBy       int64      `json:"created_by"`
	CreatorName     string     `json:"creator_name,omitempty"`
	ExpiresAt       time.Time  `json:"expires_at"`
	EmailSentAt     *time.Time `json:"email_sent_at,omitempty"`
	EmailError      *string    `json:"email_error,omitempty"`
	EmailRetryCount int        `json:"email_retry_count"`
	CreatedAt       time.Time  `json:"created_at"`
}

// InvitationValidationResponse is the public response for token validation
type InvitationValidationResponse struct {
	Email       string    `json:"email"`
	DisplayName *string   `json:"display_name,omitempty"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// InvitationsListResponse combines pending invitations and existing operators
type InvitationsListResponse struct {
	Invitations []*InvitationResponse `json:"invitations"`
	Operators   []*OperatorListItem   `json:"operators"`
}

// OperatorListItem represents an operator in the list
type OperatorListItem struct {
	ID          int64      `json:"id"`
	Email       string     `json:"email"`
	DisplayName string     `json:"display_name"`
	Active      bool       `json:"active"`
	LastLogin   *time.Time `json:"last_login,omitempty"`
	// DeactivatedAt is set while the account is deactivated (#4047);
	// DeletionDueAt is when the hourly cleanup deletes it.
	DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
	DeletionDueAt *time.Time `json:"deletion_due_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	// IsSelf marks the signed-in operator, whose account the dashboard
	// offers no deactivation for.
	IsSelf bool `json:"is_self"`
}

// --- Handlers ---

// CreateInvitation handles creating a new operator invitation
func (rs *InvitationsResource) CreateInvitation(w http.ResponseWriter, r *http.Request) {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	req := &CreateInvitationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}

	if err := rs.invitationService.InviteOperator(r.Context(), OperatorInvitationRequest{
		Email: req.Email, DisplayName: req.DisplayName, CreatedBy: operatorID, IPAddress: clientIP(r),
	}); err != nil {
		common.RenderError(w, r, invitationErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, nil, "Invitation sent successfully")
}

// ListInvitations handles listing pending invitations and existing operators
func (rs *InvitationsResource) ListInvitations(w http.ResponseWriter, r *http.Request) {
	pending, err := rs.invitationService.ListPendingOperatorInvitations(r.Context())
	if err != nil {
		common.RenderError(w, r, common.OperatorInternal("Failed to list invitations"))
		return
	}

	operators, err := rs.invitationService.ListOperators(r.Context())
	if err != nil {
		common.RenderError(w, r, common.OperatorInternal("Failed to list operators"))
		return
	}

	// Build operator name lookup for creator resolution
	operatorNames := make(map[int64]string, len(operators))
	for _, op := range operators {
		operatorNames[op.ID] = op.DisplayName
	}

	invitations := make([]*InvitationResponse, 0, len(pending))
	for _, t := range pending {
		inv := &InvitationResponse{
			ID:              t.ID,
			Email:           t.Email,
			DisplayName:     t.DisplayName,
			CreatedBy:       t.CreatedBy,
			CreatorName:     operatorNames[t.CreatedBy],
			ExpiresAt:       t.ExpiresAt,
			EmailSentAt:     t.Delivery.SentAt,
			EmailError:      t.Delivery.Error,
			EmailRetryCount: t.Delivery.RetryCount,
			CreatedAt:       t.CreatedAt,
		}
		invitations = append(invitations, inv)
	}

	signedIn := int64(sessiontoken.ClaimsFromCtx(r.Context()).ID)
	operatorList := make([]*OperatorListItem, 0, len(operators))
	for _, op := range operators {
		item := operatorListItem(op)
		item.IsSelf = op.ID == signedIn
		operatorList = append(operatorList, item)
	}

	common.Respond(w, r, http.StatusOK, &InvitationsListResponse{
		Invitations: invitations,
		Operators:   operatorList,
	}, "")
}

// ResendInvitation handles re-sending an invitation email
func (rs *InvitationsResource) ResendInvitation(w http.ResponseWriter, r *http.Request) {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	idStr := chi.URLParam(r, "id")
	invitationID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(errors.New("invalid invitation ID")))
		return
	}

	if err := rs.invitationService.ResendOperatorInvitationByActor(r.Context(), invitationID, operatorID, clientIP(r)); err != nil {
		common.RenderError(w, r, invitationErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Invitation resent successfully")
}

// RevokeInvitation handles revoking an invitation
func (rs *InvitationsResource) RevokeInvitation(w http.ResponseWriter, r *http.Request) {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	idStr := chi.URLParam(r, "id")
	invitationID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(errors.New("invalid invitation ID")))
		return
	}

	if err := rs.invitationService.RevokeOperatorInvitationByActor(r.Context(), invitationID, operatorID, clientIP(r)); err != nil {
		common.RenderError(w, r, invitationErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusNoContent, nil, "")
}

// ValidateInvitation handles validating an invitation token (public, unauthenticated)
func (rs *InvitationsResource) ValidateInvitation(w http.ResponseWriter, r *http.Request) {
	req := &ValidateInvitationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}

	token, err := rs.invitationService.ValidateOperatorInvitation(r.Context(), req.Token)
	if err != nil {
		common.RenderError(w, r, publicInvitationErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, &InvitationValidationResponse{
		Email:       token.Email,
		DisplayName: token.DisplayName,
		ExpiresAt:   token.ExpiresAt,
	}, "")
}

// AcceptInvitation handles accepting an invitation (public, unauthenticated)
func (rs *InvitationsResource) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	req := &AcceptInvitationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}

	operator, err := rs.invitationService.AcceptOperatorInvitation(r.Context(), OperatorInvitationAcceptance{
		Token: req.Token, DisplayName: req.DisplayName, Password: req.Password, IPAddress: clientIP(r),
	})
	if err != nil {
		common.RenderError(w, r, publicInvitationErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, &OperatorResponse{
		ID:          operator.ID,
		Email:       operator.Email,
		DisplayName: operator.DisplayName,
	}, "Operator account created successfully")
}

// --- Error translation ---

// operatorInputField maps the owner's typed input failures to fields.
func operatorInputField(err error, passwordField string) (field, code string, ok bool) {
	switch {
	case errors.Is(err, identityaccess.ErrOperatorEmailInvalid), errors.Is(err, identityaccess.ErrOperatorEmailTooLong):
		return "email", common.CodeGeneralInput, true
	case errors.Is(err, identityaccess.ErrOperatorPasswordTooWeak):
		return passwordField, common.CodeIdentityPasswordTooWeak, true
	case errors.Is(err, identityaccess.ErrOperatorDisplayNameRequired), errors.Is(err, identityaccess.ErrOperatorDisplayNameTooLong):
		return "display_name", common.CodeGeneralInput, true
	default:
		return "", "", false
	}
}

// --- Error renderers ---

func invitationErrorRenderer(err error) render.Renderer {
	if invalid, ok := InvalidInput(err); ok {
		return invalidOperatorInput(invalid, "password")
	}
	switch {
	case errors.Is(err, ErrOperatorInvitationNotFound):
		return common.OperatorRejection(http.StatusNotFound, common.CodeIdentityInvitationNotFound, "Einladung nicht gefunden oder abgelaufen")
	case errors.Is(err, ErrOperatorEmailExists):
		return common.OperatorRejection(http.StatusConflict, common.CodeIdentityEmailAlreadyExists, "Ein Operator mit dieser E-Mail existiert bereits")
	case errors.Is(err, ErrOperatorInvitationRateLimited):
		return common.OperatorRejection(http.StatusTooManyRequests, common.CodeIdentityInvitationRateLimited, "Zu viele Einladungen. Bitte warte eine Stunde.")
	default:
		return common.OperatorInternal("Ein Fehler ist aufgetreten")
	}
}

// publicInvitationErrorRenderer maps errors on unauthenticated endpoints.
// Uses generic messages to prevent enumeration.
func publicInvitationErrorRenderer(err error) render.Renderer {
	if invalid, ok := InvalidInput(err); ok {
		return invalidOperatorInput(invalid, "password")
	}
	switch {
	case errors.Is(err, ErrOperatorInvitationNotFound),
		errors.Is(err, ErrOperatorEmailExists):
		// One code for both cases to prevent email enumeration
		return common.OperatorRejection(http.StatusBadRequest, common.CodeIdentityOperatorInvitationInvalid, "dieser Link ist abgelaufen oder ungültig")
	default:
		return common.OperatorInternal("Ein Serverfehler ist aufgetreten")
	}
}
