package operator

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	identityoperator "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
)

// The display name and password changes are Identity & Access routes
// (#3252) mounted from modules/identityaccess/inbound/operator; the profile
// read and the e-mail change stay here.
type (
	// UpdateProfileRequest represents the profile update request body.
	UpdateProfileRequest = identityoperator.UpdateProfileRequest
	// ChangePasswordRequest represents the password change request body.
	ChangePasswordRequest = identityoperator.ChangePasswordRequest
)

// ProfileResource handles the retained operator profile endpoints
type ProfileResource struct {
	authService OperatorAccess
}

// NewProfileResource creates a new profile resource
func NewProfileResource(authService OperatorAccess) *ProfileResource {
	return &ProfileResource{
		authService: authService,
	}
}

// GetProfile handles retrieving the current operator's profile
func (rs *ProfileResource) GetProfile(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	operator, err := rs.authService.FindOperator(r.Context(), operatorID)
	if err != nil {
		common.RenderError(w, r, ProfileErrorRenderer(err))
		return
	}

	response := &OperatorResponse{
		ID:          operator.ID,
		Email:       operator.Email,
		DisplayName: operator.DisplayName,
	}

	common.Respond(w, r, http.StatusOK, response, "Profile retrieved successfully")
}

// InitiateEmailChangeRequest represents the email change initiation request body
type InitiateEmailChangeRequest struct {
	NewEmail        string `json:"new_email"`
	CurrentPassword string `json:"current_password"`
}

// Bind validates the initiate email change request
func (req *InitiateEmailChangeRequest) Bind(r *http.Request) error {
	req.NewEmail = strings.TrimSpace(req.NewEmail)
	if req.NewEmail == "" || req.CurrentPassword == "" {
		return errors.New("new_email and current_password are required")
	}
	return nil
}

// ConfirmEmailChangeRequest represents the email change confirmation request body
type ConfirmEmailChangeRequest struct {
	Token string `json:"token"`
}

// Bind validates the confirm email change request
func (req *ConfirmEmailChangeRequest) Bind(r *http.Request) error {
	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" {
		return errors.New("dieser Link ist abgelaufen oder ungültig")
	}
	if _, err := uuid.FromString(req.Token); err != nil {
		return errors.New("dieser Link ist abgelaufen oder ungültig")
	}
	return nil
}

// InitiateEmailChange handles starting the email change verification flow
func (rs *ProfileResource) InitiateEmailChange(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)

	req := &InitiateEmailChangeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	if err := rs.authService.InitiateOperatorEmailChange(r.Context(), identityoperator.OperatorEmailChangeRequest{
		OperatorID: operatorID, NewEmail: req.NewEmail, CurrentPassword: req.CurrentPassword,
		IPAddress: clientAddress(r),
	}); err != nil {
		common.RenderError(w, r, ProfileErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Verification email will be sent shortly")
}

// ConfirmEmailChange handles confirming the email change via token
func (rs *ProfileResource) ConfirmEmailChange(w http.ResponseWriter, r *http.Request) {
	req := &ConfirmEmailChangeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	// Return value intentionally discarded — no PII on unauthenticated endpoint.
	_, err := rs.authService.ConfirmOperatorEmailChange(r.Context(), req.Token, clientAddress(r))
	if err != nil {
		common.RenderError(w, r, confirmEmailChangeErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Email changed successfully")
}

// confirmEmailChangeErrorRenderer maps known application errors to a generic
// 400 response for anti-enumeration (this endpoint is unauthenticated), but
// lets infrastructure errors (DB failures, tx errors) pass through as 500 so
// the frontend can offer a retry — the token remains unconsumed on rollback.
func confirmEmailChangeErrorRenderer(err error) render.Renderer {
	switch {
	case errors.Is(err, identityoperator.ErrOperatorEmailChangeNotFound),
		errors.Is(err, identityoperator.ErrOperatorEmailInUse),
		errors.Is(err, identityoperator.ErrOperatorInactive):
		return ErrInvalidRequest(errors.New("dieser Link ist abgelaufen oder ungültig. Bitte starte den Vorgang erneut"))
	default:
		return ErrInternal("Ein Serverfehler ist aufgetreten")
	}
}

// ProfileErrorRenderer maps profile-related service errors to HTTP responses
func ProfileErrorRenderer(err error) render.Renderer {
	if invalid, ok := identityoperator.InvalidInput(err); ok {
		return ErrInvalidRequest(invalid)
	}
	switch {
	case errors.Is(err, identityoperator.ErrOperatorPasswordMismatch):
		return ErrInvalidRequest(errors.New("das aktuelle Passwort ist falsch"))
	case errors.Is(err, identityoperator.ErrOperatorNotFound):
		return ErrNotFound("Operator not found")
	case errors.Is(err, identityoperator.ErrOperatorInactive):
		return ErrForbidden("Dieser Account ist deaktiviert")
	case errors.Is(err, identityoperator.ErrOperatorEmailInUse):
		// Defensive: InitiateEmailChange returns nil for duplicates
		// (anti-enumeration), and ConfirmEmailChange uses
		// confirmEmailChangeErrorRenderer. No current caller surfaces this
		// error, but the mapping exists as a safety net if future profile
		// endpoints produce it.
		return ErrConflict("E-Mail-Adresse wird bereits verwendet")
	case errors.Is(err, identityoperator.ErrOperatorEmailChangeRateLimited):
		return ErrTooManyRequests("Zu viele Versuche. Bitte warte eine Stunde.")
	case errors.Is(err, identityoperator.ErrOperatorEmailChangeSameEmail):
		return ErrInvalidRequest(errors.New("die neue E-Mail ist identisch mit der aktuellen"))
	case errors.Is(err, identityoperator.ErrOperatorEmailChangeNotFound):
		return ErrInvalidRequest(errors.New("dieser Link ist abgelaufen oder ungültig"))
	default:
		return ErrInternal("An error occurred")
	}
}
