package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// Resource defines the auth resource
type Resource struct {
	AuthService authService.AuthService
}

// NewResource creates a new auth resource
func NewResource(authService authService.AuthService) *Resource {
	return &Resource{
		AuthService: authService,
	}

}

// Router returns a configured router for auth endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// Public routes
	r.Post("/login", rs.login)
	r.Post("/register", rs.register)

	// Protected routes that require refresh token
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
		r.Use(jwt.AuthenticateRefreshJWT)
		r.Post("/refresh", rs.refreshToken)
		r.Post("/logout", rs.logout)
	})

	// Protected routes that require access token
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
		r.Use(jwt.Authenticator)

		// Password change requires auth:update permission
		r.With(authorize.RequiresPermission(permissions.AuthManage)).Post("/password", rs.changePassword)

		// Account info requires just authentication, no specific permission
		r.Get("/account", rs.getAccount)
	})

	return r
}

// LoginRequest represents the login request payload
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Bind validates the login request
func (req *LoginRequest) Bind(r *http.Request) error {
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
		render.Render(w, r, ErrorInvalidRequest(err))
		return
	}

	accessToken, refreshToken, err := rs.AuthService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrInvalidCredentials):
				render.Render(w, r, ErrorUnauthorized(authService.ErrInvalidCredentials))
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				render.Render(w, r, ErrorUnauthorized(authService.ErrInvalidCredentials)) // Mask the specific error
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				render.Render(w, r, ErrorUnauthorized(authService.ErrAccountInactive))
			default:
				render.Render(w, r, ErrorInternalServer(err))
			}
			return
		}
		render.Render(w, r, ErrorInternalServer(err))
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
	Name            string `json:"name"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

// Bind validates the register request
func (req *RegisterRequest) Bind(r *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Username = strings.TrimSpace(req.Username)
	req.Name = strings.TrimSpace(req.Name)

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
		validation.Field(&req.Username, validation.Required, validation.Length(3, 30)),
		validation.Field(&req.Name, validation.Required),
		validation.Field(&req.Password, validation.Required, validation.Length(8, 0)),
		validation.Field(&req.ConfirmPassword, validation.Required, validation.By(func(value interface{}) error {
			if req.Password != req.ConfirmPassword {
				return errors.New("passwords do not match")
			}
			return nil
		})),
	)
}

// AccountResponse represents the account response payload
type AccountResponse struct {
	ID          int64    `json:"id"`
	Email       string   `json:"email"`
	Username    string   `json:"username,omitempty"`
	Active      bool     `json:"active"`
	Roles       []string `json:"roles,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// register handles user registration
func (rs *Resource) register(w http.ResponseWriter, r *http.Request) {
	req := &RegisterRequest{}
	if err := render.Bind(r, req); err != nil {
		render.Render(w, r, ErrorInvalidRequest(err))
		return
	}

	account, err := rs.AuthService.Register(r.Context(), req.Email, req.Username, req.Name, req.Password)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrEmailAlreadyExists):
				render.Render(w, r, ErrorInvalidRequest(authService.ErrEmailAlreadyExists))
			case errors.Is(authErr.Err, authService.ErrUsernameAlreadyExists):
				render.Render(w, r, ErrorInvalidRequest(authService.ErrUsernameAlreadyExists))
			case errors.Is(authErr.Err, authService.ErrPasswordTooWeak):
				render.Render(w, r, ErrorInvalidRequest(authService.ErrPasswordTooWeak))
			default:
				render.Render(w, r, ErrorInternalServer(err))
			}
			return
		}
		render.Render(w, r, ErrorInternalServer(err))
		return
	}

	// Convert account to response
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

	common.Respond(w, r, http.StatusCreated, resp, "Account registered successfully")
}

// refreshToken handles token refresh
func (rs *Resource) refreshToken(w http.ResponseWriter, r *http.Request) {
	// Get refresh token from context
	refreshToken := jwt.RefreshTokenFromCtx(r.Context())

	accessToken, newRefreshToken, err := rs.AuthService.RefreshToken(r.Context(), refreshToken)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrInvalidToken):
				render.Render(w, r, ErrorUnauthorized(authService.ErrInvalidToken))
			case errors.Is(authErr.Err, authService.ErrTokenExpired):
				render.Render(w, r, ErrorUnauthorized(authService.ErrTokenExpired))
			case errors.Is(authErr.Err, authService.ErrTokenNotFound):
				render.Render(w, r, ErrorUnauthorized(authService.ErrTokenNotFound))
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				render.Render(w, r, ErrorUnauthorized(authService.ErrAccountNotFound))
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				render.Render(w, r, ErrorUnauthorized(authService.ErrAccountInactive))
			default:
				render.Render(w, r, ErrorInternalServer(err))
			}
			return
		}
		render.Render(w, r, ErrorInternalServer(err))
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

	err := rs.AuthService.Logout(r.Context(), refreshToken)
	if err != nil {
		// Even if there's an error, we want to consider the logout successful from the client's perspective
		// Just log the error on the server side
	}

	common.RespondNoContent(w, r)
}

// ChangePasswordRequest represents the change password request payload
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// Bind validates the change password request
func (req *ChangePasswordRequest) Bind(r *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.CurrentPassword, validation.Required),
		validation.Field(&req.NewPassword, validation.Required, validation.Length(8, 0)),
		validation.Field(&req.ConfirmPassword, validation.Required, validation.By(func(value interface{}) error {
			if req.NewPassword != req.ConfirmPassword {
				return errors.New("passwords do not match")
			}
			return nil
		})),
	)
}

// changePassword handles password change
func (rs *Resource) changePassword(w http.ResponseWriter, r *http.Request) {
	req := &ChangePasswordRequest{}
	if err := render.Bind(r, req); err != nil {
		render.Render(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get user ID from JWT claims
	claims := jwt.ClaimsFromCtx(r.Context())

	err := rs.AuthService.ChangePassword(r.Context(), claims.ID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrInvalidCredentials):
				render.Render(w, r, ErrorUnauthorized(authService.ErrInvalidCredentials))
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				render.Render(w, r, ErrorUnauthorized(authService.ErrAccountNotFound))
			case errors.Is(authErr.Err, authService.ErrPasswordTooWeak):
				render.Render(w, r, ErrorInvalidRequest(authService.ErrPasswordTooWeak))
			default:
				render.Render(w, r, ErrorInternalServer(err))
			}
			return
		}
		render.Render(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// getAccount returns the current user's account details
func (rs *Resource) getAccount(w http.ResponseWriter, r *http.Request) {
	// Get user ID and permissions from JWT claims
	claims := jwt.ClaimsFromCtx(r.Context())

	account, err := rs.AuthService.GetAccountByID(r.Context(), claims.ID)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			if errors.Is(authErr.Err, authService.ErrAccountNotFound) {
				render.Render(w, r, ErrorNotFound(authService.ErrAccountNotFound))
				return
			}
		}
		render.Render(w, r, ErrorInternalServer(err))
		return
	}

	// Convert account to response
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

	// Include permissions from JWT claims
	resp.Permissions = claims.Permissions

	common.Respond(w, r, http.StatusOK, resp, "Account information retrieved successfully")
}
