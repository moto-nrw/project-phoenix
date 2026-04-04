package operator

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// InvitationsResource handles operator invitation endpoints
type InvitationsResource struct {
	authService platformSvc.OperatorAuthService
}

// NewInvitationsResource creates a new invitations resource
func NewInvitationsResource(authService platformSvc.OperatorAuthService) *InvitationsResource {
	return &InvitationsResource{
		authService: authService,
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
	CreatedAt   time.Time  `json:"created_at"`
}

// --- Handlers ---

// CreateInvitation handles creating a new operator invitation
func (rs *InvitationsResource) CreateInvitation(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	req := &CreateInvitationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	clientIP := getClientIP(r)
	if err := rs.authService.InviteOperator(r.Context(), req.Email, req.DisplayName, operatorID, clientIP); err != nil {
		common.RenderError(w, r, invitationErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, nil, "Invitation sent successfully")
}

// ListInvitations handles listing pending invitations and existing operators
func (rs *InvitationsResource) ListInvitations(w http.ResponseWriter, r *http.Request) {
	pending, err := rs.authService.ListPendingOperatorInvitations(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrInternal("Failed to list invitations"))
		return
	}

	operators, err := rs.authService.ListOperators(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrInternal("Failed to list operators"))
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
			EmailSentAt:     t.EmailSentAt,
			EmailError:      t.EmailError,
			EmailRetryCount: t.EmailRetryCount,
			CreatedAt:       t.CreatedAt,
		}
		invitations = append(invitations, inv)
	}

	operatorList := make([]*OperatorListItem, 0, len(operators))
	for _, op := range operators {
		operatorList = append(operatorList, &OperatorListItem{
			ID:          op.ID,
			Email:       op.Email,
			DisplayName: op.DisplayName,
			Active:      op.Active,
			LastLogin:   op.LastLogin,
			CreatedAt:   op.CreatedAt,
		})
	}

	common.Respond(w, r, http.StatusOK, &InvitationsListResponse{
		Invitations: invitations,
		Operators:   operatorList,
	}, "")
}

// ResendInvitation handles re-sending an invitation email
func (rs *InvitationsResource) ResendInvitation(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	idStr := chi.URLParam(r, "id")
	invitationID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RenderError(w, r, ErrInvalidRequest(errors.New("invalid invitation ID")))
		return
	}

	clientIP := getClientIP(r)
	if err := rs.authService.ResendOperatorInvitation(r.Context(), invitationID, operatorID, clientIP); err != nil {
		common.RenderError(w, r, invitationErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Invitation resent successfully")
}

// RevokeInvitation handles revoking an invitation
func (rs *InvitationsResource) RevokeInvitation(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	idStr := chi.URLParam(r, "id")
	invitationID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RenderError(w, r, ErrInvalidRequest(errors.New("invalid invitation ID")))
		return
	}

	clientIP := getClientIP(r)
	if err := rs.authService.RevokeOperatorInvitation(r.Context(), invitationID, operatorID, clientIP); err != nil {
		common.RenderError(w, r, invitationErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusNoContent, nil, "")
}

// ValidateInvitation handles validating an invitation token (public, unauthenticated)
func (rs *InvitationsResource) ValidateInvitation(w http.ResponseWriter, r *http.Request) {
	req := &ValidateInvitationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	token, err := rs.authService.ValidateOperatorInvitation(r.Context(), req.Token)
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
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	clientIP := getClientIP(r)
	operator, err := rs.authService.AcceptOperatorInvitation(r.Context(), req.Token, req.DisplayName, req.Password, clientIP)
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

// invitationValidationErrors maps English service-layer validation messages to German.
var invitationValidationErrors = map[string]string{
	"invalid email format":                          "Ungültiges E-Mail-Format",
	"email address too long":                        "E-Mail-Adresse ist zu lang",
	"password doesn't meet complexity requirements": "Passwort erfüllt nicht die Anforderungen",
	"display name is required":                      "Anzeigename ist erforderlich",
	"display name must not exceed 100 characters":   "Anzeigename darf maximal 100 Zeichen lang sein",
}

func translateInvitationValidationError(err error) string {
	if err == nil {
		return "Ungültige Eingabe"
	}
	if translated, ok := invitationValidationErrors[err.Error()]; ok {
		return translated
	}
	return "Ungültige Eingabe"
}

// --- Error renderers ---

func invitationErrorRenderer(err error) render.Renderer {
	var notFound *platformSvc.OperatorInvitationNotFoundError
	var emailExists *platformSvc.OperatorInvitationEmailExistsError
	var invalidData *platformSvc.InvalidDataError

	switch {
	case errors.As(err, &notFound):
		return ErrNotFound("Einladung nicht gefunden oder abgelaufen")
	case errors.As(err, &emailExists):
		return ErrConflict("Ein Operator mit dieser E-Mail existiert bereits")
	case errors.As(err, &invalidData):
		return ErrInvalidRequest(errors.New(translateInvitationValidationError(invalidData.Unwrap())))
	default:
		return ErrInternal("Ein Fehler ist aufgetreten")
	}
}

// publicInvitationErrorRenderer maps errors on unauthenticated endpoints.
// Uses generic messages to prevent enumeration.
func publicInvitationErrorRenderer(err error) render.Renderer {
	var notFound *platformSvc.OperatorInvitationNotFoundError
	var emailExists *platformSvc.OperatorInvitationEmailExistsError
	var invalidData *platformSvc.InvalidDataError

	switch {
	case errors.As(err, &notFound), errors.As(err, &emailExists):
		// Generic message for both cases to prevent email enumeration
		return ErrInvalidRequest(errors.New("dieser Link ist abgelaufen oder ungültig"))
	case errors.As(err, &invalidData):
		return ErrInvalidRequest(errors.New(translateInvitationValidationError(invalidData.Unwrap())))
	default:
		return ErrInternal("Ein Serverfehler ist aufgetreten")
	}
}
