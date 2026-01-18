package auth

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// LoginRequest represents the login request payload
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Bind validates the login request
func (req *LoginRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
		validation.Field(&req.Password, validation.Required),
	)
}

// TokenResponse represents the token response payload
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// login handles user login
func (rs *Resource) login(w http.ResponseWriter, r *http.Request) {
	req := &LoginRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get IP address and user agent for audit logging
	ipAddress := getClientIP(r)
	userAgent := r.Header.Get(headerUserAgent)

	accessToken, refreshToken, err := rs.AuthService.LoginWithAudit(r.Context(), req.Email, req.Password, ipAddress, userAgent)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrInvalidCredentials):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrInvalidCredentials))
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrInvalidCredentials)) // Mask the specific error
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrAccountInactive))
			default:
				common.RenderError(w, r, ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Special case for login endpoint - frontend expects direct token response
	render.JSON(w, r, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// RegisterRequest represents the register request payload
type RegisterRequest struct {
	Email           string `json:"email"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
	RoleID          *int64 `json:"role_id,omitempty"` // Optional role assignment
}

// Bind validates the register request
func (req *RegisterRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Username = strings.TrimSpace(req.Username)

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
		validation.Field(&req.Username, validation.Required, validation.Length(3, 30)),
		validation.Field(&req.Password, validation.Required, validation.Length(8, 0)),
		validation.Field(&req.ConfirmPassword, validation.Required, validation.By(func(value interface{}) error {
			if req.Password != req.ConfirmPassword {
				return errors.New(errPasswordsNotMatch)
			}
			return nil
		})),
	)
}

// register handles user registration
func (rs *Resource) register(w http.ResponseWriter, r *http.Request) {
	req := &RegisterRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Authorize role assignment (if role_id specified)
	roleID, shouldReturn := rs.authorizeRoleAssignment(w, r, req.RoleID)
	if shouldReturn {
		return
	}

	account, err := rs.AuthService.Register(r.Context(), req.Email, req.Username, req.Password, roleID)
	if err != nil {
		rs.handleRegistrationError(w, r, err)
		return
	}

	resp := buildAccountResponse(account)
	common.Respond(w, r, http.StatusCreated, resp, "Account registered successfully")
}

// authorizeRoleAssignment checks if the caller is authorized to assign a role during registration.
// Returns the authorized role ID and a boolean indicating if the handler should return early.
func (rs *Resource) authorizeRoleAssignment(w http.ResponseWriter, r *http.Request, requestedRoleID *int64) (*int64, bool) {
	if requestedRoleID == nil {
		return nil, false
	}

	authHeader := r.Header.Get("Authorization")
	if !isValidAuthHeader(authHeader) {
		log.Printf("Security: Unauthenticated register attempt with role_id, ignoring role_id")
		return nil, false
	}

	token := authHeader[7:]
	callerAccount, err := rs.AuthService.ValidateToken(r.Context(), token)
	if err != nil {
		log.Printf("Security: Invalid token in register with role_id, ignoring role_id")
		return nil, false
	}

	if !hasAdminRole(callerAccount.Roles) {
		log.Printf("Security: Non-admin (account %d) attempted to set role_id, denying", callerAccount.ID)
		common.RenderError(w, r, ErrorUnauthorized(errors.New("only administrators can assign roles")))
		return nil, true
	}

	return requestedRoleID, false
}

// isValidAuthHeader checks if the Authorization header contains a valid Bearer token format
func isValidAuthHeader(authHeader string) bool {
	return authHeader != "" && len(authHeader) >= 8 && authHeader[:7] == "Bearer "
}

// hasAdminRole checks if any of the roles has the "admin" name
func hasAdminRole(roles []*authModel.Role) bool {
	for _, role := range roles {
		if role.Name == "admin" {
			return true
		}
	}
	return false
}

// handleRegistrationError handles authentication errors during registration
func (rs *Resource) handleRegistrationError(w http.ResponseWriter, r *http.Request, err error) {
	var authErr *authService.AuthError
	if !errors.As(err, &authErr) {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	switch {
	case errors.Is(authErr.Err, authService.ErrEmailAlreadyExists):
		common.RenderError(w, r, ErrorInvalidRequest(authService.ErrEmailAlreadyExists))
	case errors.Is(authErr.Err, authService.ErrUsernameAlreadyExists):
		common.RenderError(w, r, ErrorInvalidRequest(authService.ErrUsernameAlreadyExists))
	case errors.Is(authErr.Err, authService.ErrPasswordTooWeak):
		common.RenderError(w, r, ErrorInvalidRequest(authService.ErrPasswordTooWeak))
	default:
		common.RenderError(w, r, ErrorInternalServer(err))
	}
}

// buildAccountResponse constructs an AccountResponse from an Account model
func buildAccountResponse(account *authModel.Account) *AccountResponse {
	resp := &AccountResponse{
		ID:     account.ID,
		Email:  account.Email,
		Active: account.Active,
	}

	if account.Username != nil {
		resp.Username = *account.Username
	}

	roleNames := make([]string, 0, len(account.Roles))
	for _, role := range account.Roles {
		roleNames = append(roleNames, role.Name)
	}
	resp.Roles = roleNames

	return resp
}

// refreshToken handles token refresh
func (rs *Resource) refreshToken(w http.ResponseWriter, r *http.Request) {
	// Get refresh token from context
	refreshToken := jwt.RefreshTokenFromCtx(r.Context())

	// Get IP address and user agent for audit logging
	ipAddress := getClientIP(r)
	userAgent := r.Header.Get(headerUserAgent)

	accessToken, newRefreshToken, err := rs.AuthService.RefreshTokenWithAudit(r.Context(), refreshToken, ipAddress, userAgent)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrInvalidToken):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrInvalidToken))
			case errors.Is(authErr.Err, authService.ErrTokenExpired):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrTokenExpired))
			case errors.Is(authErr.Err, authService.ErrTokenNotFound):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrTokenNotFound))
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrAccountNotFound))
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrAccountInactive))
			default:
				common.RenderError(w, r, ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Special case for token refresh endpoint - frontend expects direct token response
	render.JSON(w, r, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	})
}

// logout handles user logout
func (rs *Resource) logout(w http.ResponseWriter, r *http.Request) {
	// Get refresh token from context
	refreshToken := jwt.RefreshTokenFromCtx(r.Context())

	// Get IP address and user agent for audit logging
	ipAddress := getClientIP(r)
	userAgent := r.Header.Get(headerUserAgent)

	err := rs.AuthService.LogoutWithAudit(r.Context(), refreshToken, ipAddress, userAgent)
	if err != nil {
		// Even if there's an error, we want to consider the logout successful from the client's perspective
		// Log the error on the server side for debugging
		log.Printf("Logout audit logging failed (client logout still successful): ip=%s, error=%v", ipAddress, err)
	}

	common.RespondNoContent(w, r)
}
