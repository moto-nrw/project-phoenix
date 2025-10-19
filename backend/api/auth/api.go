package auth

import (
	"database/sql"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
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
	r.Post("/password-reset", rs.initiatePasswordReset)
	r.Post("/password-reset/confirm", rs.resetPassword)

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

		// Current user routes
		r.Get("/account", rs.getAccount)

		// Password change - users can change their own password without special permissions
		r.Post("/password", rs.changePassword)

		// Admin routes - require admin role or specific permissions
		r.Group(func(r chi.Router) {
			// Role management routes
			r.Route("/roles", func(r chi.Router) {
				r.With(authorize.RequiresPermission("roles:create")).Post("/", rs.createRole)
				r.With(authorize.RequiresPermission("roles:read")).Get("/", rs.listRoles)
				r.Route("/{id}", func(r chi.Router) {
					r.With(authorize.RequiresPermission("roles:read")).Get("/", rs.getRoleByID)
					r.With(authorize.RequiresPermission("roles:update")).Put("/", rs.updateRole)
					r.With(authorize.RequiresPermission("roles:delete")).Delete("/", rs.deleteRole)
					r.With(authorize.RequiresPermission("roles:read")).Get("/permissions", rs.getRolePermissions)
				})
			})

			// Permission management routes
			r.Route("/permissions", func(r chi.Router) {
				r.With(authorize.RequiresPermission("permissions:create")).Post("/", rs.createPermission)
				r.With(authorize.RequiresPermission("permissions:read")).Get("/", rs.listPermissions)
				r.Route("/{id}", func(r chi.Router) {
					r.With(authorize.RequiresPermission("permissions:read")).Get("/", rs.getPermissionByID)
					r.With(authorize.RequiresPermission("permissions:update")).Put("/", rs.updatePermission)
					r.With(authorize.RequiresPermission("permissions:delete")).Delete("/", rs.deletePermission)
				})
			})

			// Account management routes
			r.Route("/accounts", func(r chi.Router) {
				r.With(authorize.RequiresPermission("users:list")).Get("/", rs.listAccounts)
				r.With(authorize.RequiresPermission("users:read")).Get("/by-role/{roleName}", rs.getAccountsByRole)

				r.Route("/{accountId}", func(r chi.Router) {
					// Account update operations
					r.With(authorize.RequiresPermission("users:update")).Put("/", rs.updateAccount)
					r.With(authorize.RequiresPermission("users:update")).Put("/activate", rs.activateAccount)
					r.With(authorize.RequiresPermission("users:update")).Put("/deactivate", rs.deactivateAccount)

					// Role assignments
					r.Route("/roles", func(r chi.Router) {
						r.With(authorize.RequiresPermission("users:manage")).Get("/", rs.getAccountRoles)
						r.With(authorize.RequiresPermission("users:manage")).Post("/{roleId}", rs.assignRoleToAccount)
						r.With(authorize.RequiresPermission("users:manage")).Delete("/{roleId}", rs.removeRoleFromAccount)
					})

					// Permission assignments
					r.Route("/permissions", func(r chi.Router) {
						r.With(authorize.RequiresPermission("users:manage")).Get("/", rs.getAccountPermissions)
						r.With(authorize.RequiresPermission("users:manage")).Get("/direct", rs.getAccountDirectPermissions)
						r.With(authorize.RequiresPermission("users:manage")).Post("/{permissionId}/grant", rs.grantPermissionToAccount)
						r.With(authorize.RequiresPermission("users:manage")).Post("/{permissionId}/deny", rs.denyPermissionToAccount)
						r.With(authorize.RequiresPermission("users:manage")).Delete("/{permissionId}", rs.removePermissionFromAccount)
					})

					// Token management
					r.Route("/tokens", func(r chi.Router) {
						r.With(authorize.RequiresPermission("users:manage")).Get("/", rs.getActiveTokens)
						r.With(authorize.RequiresPermission("users:manage")).Delete("/", rs.revokeAllTokens)
					})
				})
			})

			// Role permission assignments
			r.Route("/roles/{roleId}/permissions", func(r chi.Router) {
				r.With(authorize.RequiresPermission("roles:manage")).Get("/", rs.getRolePermissions)
				r.With(authorize.RequiresPermission("roles:manage")).Post("/{permissionId}", rs.assignPermissionToRole)
				r.With(authorize.RequiresPermission("roles:manage")).Delete("/{permissionId}", rs.removePermissionFromRole)
			})

			// Token cleanup
			r.Route("/tokens", func(r chi.Router) {
				r.With(authorize.RequiresPermission("admin:*")).Delete("/expired", rs.cleanupExpiredTokens)
			})

			// Parent account management
			r.Route("/parent-accounts", func(r chi.Router) {
				r.With(authorize.RequiresPermission("users:create")).Post("/", rs.createParentAccount)
				r.With(authorize.RequiresPermission("users:list")).Get("/", rs.listParentAccounts)
				r.Route("/{id}", func(r chi.Router) {
					r.With(authorize.RequiresPermission("users:read")).Get("/", rs.getParentAccountByID)
					r.With(authorize.RequiresPermission("users:update")).Put("/", rs.updateParentAccount)
					r.With(authorize.RequiresPermission("users:update")).Put("/activate", rs.activateParentAccount)
					r.With(authorize.RequiresPermission("users:update")).Put("/deactivate", rs.deactivateParentAccount)
				})
			})
		})
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
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get IP address and user agent for audit logging
	ipAddress := getClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	accessToken, refreshToken, err := rs.AuthService.LoginWithAudit(r.Context(), req.Email, req.Password, ipAddress, userAgent)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrInvalidCredentials):
				if err := render.Render(w, r, ErrorUnauthorized(authService.ErrInvalidCredentials)); err != nil {
					log.Printf("Error rendering error response: %v", err)
				}
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				if err := render.Render(w, r, ErrorUnauthorized(authService.ErrInvalidCredentials)); err != nil { // Mask the specific error
					log.Printf("Error rendering error response: %v", err)
				}
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				if err := render.Render(w, r, ErrorUnauthorized(authService.ErrAccountInactive)); err != nil {
					log.Printf("Render error: %v", err)
				}
			default:
				if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
					log.Printf("Render error: %v", err)
				}
			}
			return
		}
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	account, err := rs.AuthService.Register(r.Context(), req.Email, req.Username, req.Name, req.Password)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrEmailAlreadyExists):
				if err := render.Render(w, r, ErrorInvalidRequest(authService.ErrEmailAlreadyExists)); err != nil {
					log.Printf("Render error: %v", err)
				}
			case errors.Is(authErr.Err, authService.ErrUsernameAlreadyExists):
				if err := render.Render(w, r, ErrorInvalidRequest(authService.ErrUsernameAlreadyExists)); err != nil {
					log.Printf("Render error: %v", err)
				}
			case errors.Is(authErr.Err, authService.ErrPasswordTooWeak):
				if err := render.Render(w, r, ErrorInvalidRequest(authService.ErrPasswordTooWeak)); err != nil {
					log.Printf("Render error: %v", err)
				}
			default:
				if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
					log.Printf("Render error: %v", err)
				}
			}
			return
		}
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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

	// Get IP address and user agent for audit logging
	ipAddress := getClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	accessToken, newRefreshToken, err := rs.AuthService.RefreshTokenWithAudit(r.Context(), refreshToken, ipAddress, userAgent)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrInvalidToken):
				if err := render.Render(w, r, ErrorUnauthorized(authService.ErrInvalidToken)); err != nil {
					log.Printf("Render error: %v", err)
				}
			case errors.Is(authErr.Err, authService.ErrTokenExpired):
				if err := render.Render(w, r, ErrorUnauthorized(authService.ErrTokenExpired)); err != nil {
					log.Printf("Render error: %v", err)
				}
			case errors.Is(authErr.Err, authService.ErrTokenNotFound):
				if err := render.Render(w, r, ErrorUnauthorized(authService.ErrTokenNotFound)); err != nil {
					log.Printf("Render error: %v", err)
				}
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				if err := render.Render(w, r, ErrorUnauthorized(authService.ErrAccountNotFound)); err != nil {
					log.Printf("Render error: %v", err)
				}
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				if err := render.Render(w, r, ErrorUnauthorized(authService.ErrAccountInactive)); err != nil {
					log.Printf("Render error: %v", err)
				}
			default:
				if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
					log.Printf("Render error: %v", err)
				}
			}
			return
		}
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
	userAgent := r.Header.Get("User-Agent")

	err := rs.AuthService.LogoutWithAudit(r.Context(), refreshToken, ipAddress, userAgent)
	if err != nil {
		// Even if there's an error, we want to consider the logout successful from the client's perspective
		// Just log the error on the server side
		// TODO: Log the error properly
		_ = err
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
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
				if err := render.Render(w, r, ErrorUnauthorized(authService.ErrInvalidCredentials)); err != nil {
					log.Printf("Render error: %v", err)
				}
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				if err := render.Render(w, r, ErrorUnauthorized(authService.ErrAccountNotFound)); err != nil {
					log.Printf("Render error: %v", err)
				}
			case errors.Is(authErr.Err, authService.ErrPasswordTooWeak):
				if err := render.Render(w, r, ErrorInvalidRequest(authService.ErrPasswordTooWeak)); err != nil {
					log.Printf("Render error: %v", err)
				}
			default:
				if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
					log.Printf("Render error: %v", err)
				}
			}
			return
		}
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
				if err := render.Render(w, r, ErrorNotFound(authService.ErrAccountNotFound)); err != nil {
					log.Printf("Render error: %v", err)
				}
				return
			}
		}
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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

// Role Management Request/Response Types

// CreateRoleRequest represents the create role request payload
type CreateRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Bind validates the create role request
func (req *CreateRoleRequest) Bind(r *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)

	return validation.ValidateStruct(req,
		validation.Field(&req.Name, validation.Required, validation.Length(1, 100)),
		validation.Field(&req.Description, validation.Length(0, 500)),
	)
}

// UpdateRoleRequest represents the update role request payload
type UpdateRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Bind validates the update role request
func (req *UpdateRoleRequest) Bind(r *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)

	return validation.ValidateStruct(req,
		validation.Field(&req.Name, validation.Required, validation.Length(1, 100)),
		validation.Field(&req.Description, validation.Length(0, 500)),
	)
}

// RoleResponse represents a role response
type RoleResponse struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	Permissions []string `json:"permissions,omitempty"`
}

// Permission Management Request/Response Types

// CreatePermissionRequest represents the create permission request payload
type CreatePermissionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
}

// Bind validates the create permission request
func (req *CreatePermissionRequest) Bind(r *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Resource = strings.TrimSpace(strings.ToLower(req.Resource))
	req.Action = strings.TrimSpace(strings.ToLower(req.Action))

	return validation.ValidateStruct(req,
		validation.Field(&req.Name, validation.Required, validation.Length(1, 100)),
		validation.Field(&req.Description, validation.Length(0, 500)),
		validation.Field(&req.Resource, validation.Required, validation.Length(1, 50)),
		validation.Field(&req.Action, validation.Required, validation.Length(1, 50)),
	)
}

// UpdatePermissionRequest represents the update permission request payload
type UpdatePermissionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
}

// Bind validates the update permission request
func (req *UpdatePermissionRequest) Bind(r *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Resource = strings.TrimSpace(strings.ToLower(req.Resource))
	req.Action = strings.TrimSpace(strings.ToLower(req.Action))

	return validation.ValidateStruct(req,
		validation.Field(&req.Name, validation.Required, validation.Length(1, 100)),
		validation.Field(&req.Description, validation.Length(0, 500)),
		validation.Field(&req.Resource, validation.Required, validation.Length(1, 50)),
		validation.Field(&req.Action, validation.Required, validation.Length(1, 50)),
	)
}

// PermissionResponse represents a permission response
type PermissionResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// UpdateAccountRequest represents the update account request payload
type UpdateAccountRequest struct {
	Email    string `json:"email"`
	Username string `json:"username,omitempty"`
}

// Bind validates the update account request
func (req *UpdateAccountRequest) Bind(r *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Username = strings.TrimSpace(req.Username)

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
		validation.Field(&req.Username, validation.Length(3, 30)),
	)
}

// PasswordResetRequest represents the password reset request payload
type PasswordResetRequest struct {
	Email string `json:"email"`
}

// Bind validates the password reset request
func (req *PasswordResetRequest) Bind(r *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
	)
}

// PasswordResetConfirmRequest represents the password reset confirm request payload
type PasswordResetConfirmRequest struct {
	Token           string `json:"token"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// Bind validates the password reset confirm request
func (req *PasswordResetConfirmRequest) Bind(r *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Token, validation.Required),
		validation.Field(&req.NewPassword, validation.Required, validation.Length(8, 0)),
		validation.Field(&req.ConfirmPassword, validation.Required, validation.By(func(value interface{}) error {
			if req.NewPassword != req.ConfirmPassword {
				return errors.New("passwords do not match")
			}
			return nil
		})),
	)
}

// CreateParentAccountRequest represents the create parent account request payload
type CreateParentAccountRequest struct {
	Email           string `json:"email"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

// Bind validates the create parent account request
func (req *CreateParentAccountRequest) Bind(r *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Username = strings.TrimSpace(req.Username)

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
		validation.Field(&req.Username, validation.Required, validation.Length(3, 30)),
		validation.Field(&req.Password, validation.Required, validation.Length(8, 0)),
		validation.Field(&req.ConfirmPassword, validation.Required, validation.By(func(value interface{}) error {
			if req.Password != req.ConfirmPassword {
				return errors.New("passwords do not match")
			}
			return nil
		})),
	)
}

// ParentAccountResponse represents a parent account response
type ParentAccountResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Username  string `json:"username,omitempty"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Role Management Endpoints

// createRole handles creating a new role
func (rs *Resource) createRole(w http.ResponseWriter, r *http.Request) {
	req := &CreateRoleRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	role, err := rs.AuthService.CreateRole(r.Context(), req.Name, req.Description)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	resp := &RoleResponse{
		ID:          role.ID,
		Name:        role.Name,
		Description: role.Description,
		CreatedAt:   role.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   role.UpdatedAt.Format(time.RFC3339),
	}

	common.Respond(w, r, http.StatusCreated, resp, "Role created successfully")
}

// getRoleByID handles getting a role by ID
func (rs *Resource) getRoleByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid role ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	role, err := rs.AuthService.GetRoleByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("role not found"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Get permissions for the role
	permissions, _ := rs.AuthService.GetRolePermissions(r.Context(), id)
	permissionNames := make([]string, 0, len(permissions))
	for _, perm := range permissions {
		permissionNames = append(permissionNames, perm.GetFullName())
	}

	resp := &RoleResponse{
		ID:          role.ID,
		Name:        role.Name,
		Description: role.Description,
		CreatedAt:   role.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   role.UpdatedAt.Format(time.RFC3339),
		Permissions: permissionNames,
	}

	common.Respond(w, r, http.StatusOK, resp, "Role retrieved successfully")
}

// updateRole handles updating a role
func (rs *Resource) updateRole(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid role ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	req := &UpdateRoleRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	role, err := rs.AuthService.GetRoleByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("role not found"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	role.Name = req.Name
	role.Description = req.Description

	if err := rs.AuthService.UpdateRole(r.Context(), role); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.RespondNoContent(w, r)
}

// deleteRole handles deleting a role
func (rs *Resource) deleteRole(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid role ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	if err := rs.AuthService.DeleteRole(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.RespondNoContent(w, r)
}

// listRoles handles listing roles
func (rs *Resource) listRoles(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters for filtering
	filters := make(map[string]interface{})

	if name := r.URL.Query().Get("name"); name != "" {
		filters["name"] = name
	}

	roles, err := rs.AuthService.ListRoles(r.Context(), filters)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	responses := make([]*RoleResponse, 0, len(roles))
	for _, role := range roles {
		resp := &RoleResponse{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			CreatedAt:   role.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   role.UpdatedAt.Format(time.RFC3339),
		}
		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Roles retrieved successfully")
}

// assignRoleToAccount handles assigning a role to an account
func (rs *Resource) assignRoleToAccount(w http.ResponseWriter, r *http.Request) {
	accountIDStr := chi.URLParam(r, "accountId")
	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid account ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	roleIDStr := chi.URLParam(r, "roleId")
	roleID, err := strconv.Atoi(roleIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid role ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	if err := rs.AuthService.AssignRoleToAccount(r.Context(), accountID, roleID); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.RespondNoContent(w, r)
}

// removeRoleFromAccount handles removing a role from an account
func (rs *Resource) removeRoleFromAccount(w http.ResponseWriter, r *http.Request) {
	accountIDStr := chi.URLParam(r, "accountId")
	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid account ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	roleIDStr := chi.URLParam(r, "roleId")
	roleID, err := strconv.Atoi(roleIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid role ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	if err := rs.AuthService.RemoveRoleFromAccount(r.Context(), accountID, roleID); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.RespondNoContent(w, r)
}

// getAccountRoles handles getting roles for an account
func (rs *Resource) getAccountRoles(w http.ResponseWriter, r *http.Request) {
	accountIDStr := chi.URLParam(r, "accountId")
	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid account ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	roles, err := rs.AuthService.GetAccountRoles(r.Context(), accountID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	responses := make([]*RoleResponse, 0, len(roles))
	for _, role := range roles {
		resp := &RoleResponse{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			CreatedAt:   role.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   role.UpdatedAt.Format(time.RFC3339),
		}
		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Account roles retrieved successfully")
}

// Permission Management Endpoints

// createPermission handles creating a new permission
func (rs *Resource) createPermission(w http.ResponseWriter, r *http.Request) {
	req := &CreatePermissionRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	permission, err := rs.AuthService.CreatePermission(r.Context(), req.Name, req.Description, req.Resource, req.Action)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	resp := &PermissionResponse{
		ID:          permission.ID,
		Name:        permission.Name,
		Description: permission.Description,
		Resource:    permission.Resource,
		Action:      permission.Action,
		CreatedAt:   permission.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   permission.UpdatedAt.Format(time.RFC3339),
	}

	common.Respond(w, r, http.StatusCreated, resp, "Permission created successfully")
}

// getPermissionByID handles getting a permission by ID
func (rs *Resource) getPermissionByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid permission ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	permission, err := rs.AuthService.GetPermissionByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("permission not found"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	resp := &PermissionResponse{
		ID:          permission.ID,
		Name:        permission.Name,
		Description: permission.Description,
		Resource:    permission.Resource,
		Action:      permission.Action,
		CreatedAt:   permission.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   permission.UpdatedAt.Format(time.RFC3339),
	}

	common.Respond(w, r, http.StatusOK, resp, "Permission retrieved successfully")
}

// updatePermission handles updating a permission
func (rs *Resource) updatePermission(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid permission ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	req := &UpdatePermissionRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	permission, err := rs.AuthService.GetPermissionByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("permission not found"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	permission.Name = req.Name
	permission.Description = req.Description
	permission.Resource = req.Resource
	permission.Action = req.Action

	if err := rs.AuthService.UpdatePermission(r.Context(), permission); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.RespondNoContent(w, r)
}

// deletePermission handles deleting a permission
func (rs *Resource) deletePermission(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid permission ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	if err := rs.AuthService.DeletePermission(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.RespondNoContent(w, r)
}

// listPermissions handles listing permissions
func (rs *Resource) listPermissions(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters for filtering
	filters := make(map[string]interface{})

	if resource := r.URL.Query().Get("resource"); resource != "" {
		filters["resource"] = resource
	}

	if action := r.URL.Query().Get("action"); action != "" {
		filters["action"] = action
	}

	permissions, err := rs.AuthService.ListPermissions(r.Context(), filters)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	responses := make([]*PermissionResponse, 0, len(permissions))
	for _, permission := range permissions {
		resp := &PermissionResponse{
			ID:          permission.ID,
			Name:        permission.Name,
			Description: permission.Description,
			Resource:    permission.Resource,
			Action:      permission.Action,
			CreatedAt:   permission.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   permission.UpdatedAt.Format(time.RFC3339),
		}
		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Permissions retrieved successfully")
}

// grantPermissionToAccount handles granting a permission to an account
func (rs *Resource) grantPermissionToAccount(w http.ResponseWriter, r *http.Request) {
	accountIDStr := chi.URLParam(r, "accountId")
	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid account ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	permissionIDStr := chi.URLParam(r, "permissionId")
	permissionID, err := strconv.Atoi(permissionIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid permission ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	if err := rs.AuthService.GrantPermissionToAccount(r.Context(), accountID, permissionID); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.RespondNoContent(w, r)
}

// denyPermissionToAccount handles denying a permission to an account
func (rs *Resource) denyPermissionToAccount(w http.ResponseWriter, r *http.Request) {
	accountIDStr := chi.URLParam(r, "accountId")
	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid account ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	permissionIDStr := chi.URLParam(r, "permissionId")
	permissionID, err := strconv.Atoi(permissionIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid permission ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	if err := rs.AuthService.DenyPermissionToAccount(r.Context(), accountID, permissionID); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.RespondNoContent(w, r)
}

// removePermissionFromAccount handles removing a permission from an account
func (rs *Resource) removePermissionFromAccount(w http.ResponseWriter, r *http.Request) {
	accountIDStr := chi.URLParam(r, "accountId")
	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid account ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	permissionIDStr := chi.URLParam(r, "permissionId")
	permissionID, err := strconv.Atoi(permissionIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid permission ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	if err := rs.AuthService.RemovePermissionFromAccount(r.Context(), accountID, permissionID); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.RespondNoContent(w, r)
}

// getAccountPermissions handles getting permissions for an account
func (rs *Resource) getAccountPermissions(w http.ResponseWriter, r *http.Request) {
	accountIDStr := chi.URLParam(r, "accountId")
	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid account ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	permissions, err := rs.AuthService.GetAccountPermissions(r.Context(), accountID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	responses := make([]*PermissionResponse, 0, len(permissions))
	for _, permission := range permissions {
		resp := &PermissionResponse{
			ID:          permission.ID,
			Name:        permission.Name,
			Description: permission.Description,
			Resource:    permission.Resource,
			Action:      permission.Action,
			CreatedAt:   permission.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   permission.UpdatedAt.Format(time.RFC3339),
		}
		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Account permissions retrieved successfully")
}

// getAccountDirectPermissions handles getting only direct permissions for an account (not role-based)
func (rs *Resource) getAccountDirectPermissions(w http.ResponseWriter, r *http.Request) {
	accountIDStr := chi.URLParam(r, "accountId")
	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid account ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	permissions, err := rs.AuthService.GetAccountDirectPermissions(r.Context(), accountID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	responses := make([]*PermissionResponse, 0, len(permissions))
	for _, permission := range permissions {
		resp := &PermissionResponse{
			ID:          permission.ID,
			Name:        permission.Name,
			Description: permission.Description,
			Resource:    permission.Resource,
			Action:      permission.Action,
			CreatedAt:   permission.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   permission.UpdatedAt.Format(time.RFC3339),
		}
		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Account direct permissions retrieved successfully")
}

// assignPermissionToRole handles assigning a permission to a role
func (rs *Resource) assignPermissionToRole(w http.ResponseWriter, r *http.Request) {
	roleIDStr := chi.URLParam(r, "roleId")
	roleID, err := strconv.Atoi(roleIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid role ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	permissionIDStr := chi.URLParam(r, "permissionId")
	permissionID, err := strconv.Atoi(permissionIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid permission ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	if err := rs.AuthService.AssignPermissionToRole(r.Context(), roleID, permissionID); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.RespondNoContent(w, r)
}

// removePermissionFromRole handles removing a permission from a role
func (rs *Resource) removePermissionFromRole(w http.ResponseWriter, r *http.Request) {
	roleIDStr := chi.URLParam(r, "roleId")
	roleID, err := strconv.Atoi(roleIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid role ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	permissionIDStr := chi.URLParam(r, "permissionId")
	permissionID, err := strconv.Atoi(permissionIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid permission ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	if err := rs.AuthService.RemovePermissionFromRole(r.Context(), roleID, permissionID); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.RespondNoContent(w, r)
}

// getRolePermissions handles getting permissions for a role
func (rs *Resource) getRolePermissions(w http.ResponseWriter, r *http.Request) {
	roleIDStr := chi.URLParam(r, "roleId")
	roleID, err := strconv.Atoi(roleIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid role ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	permissions, err := rs.AuthService.GetRolePermissions(r.Context(), roleID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	responses := make([]*PermissionResponse, 0, len(permissions))
	for _, permission := range permissions {
		resp := &PermissionResponse{
			ID:          permission.ID,
			Name:        permission.Name,
			Description: permission.Description,
			Resource:    permission.Resource,
			Action:      permission.Action,
			CreatedAt:   permission.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   permission.UpdatedAt.Format(time.RFC3339),
		}
		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Role permissions retrieved successfully")
}

// Account Management Extension Endpoints

// activateAccount handles activating an account
func (rs *Resource) activateAccount(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid account ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	if err := rs.AuthService.ActivateAccount(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.RespondNoContent(w, r)
}

// deactivateAccount handles deactivating an account
func (rs *Resource) deactivateAccount(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid account ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	if err := rs.AuthService.DeactivateAccount(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.RespondNoContent(w, r)
}

// updateAccount handles updating an account
func (rs *Resource) updateAccount(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid account ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	req := &UpdateAccountRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	account, err := rs.AuthService.GetAccountByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("account not found"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	account.Email = req.Email
	if req.Username != "" {
		username := req.Username
		account.Username = &username
	}

	if err := rs.AuthService.UpdateAccount(r.Context(), account); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.RespondNoContent(w, r)
}

// listAccounts handles listing accounts
func (rs *Resource) listAccounts(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters for filtering
	filters := make(map[string]interface{})

	if email := r.URL.Query().Get("email"); email != "" {
		filters["email"] = email
	}

	if active := r.URL.Query().Get("active"); active != "" {
		switch active {
		case "true":
			filters["active"] = true
		case "false":
			filters["active"] = false
		}
	}

	accounts, err := rs.AuthService.ListAccounts(r.Context(), filters)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	responses := make([]*AccountResponse, 0, len(accounts))
	for _, account := range accounts {
		resp := &AccountResponse{
			ID:     account.ID,
			Email:  account.Email,
			Active: account.Active,
		}

		if account.Username != nil {
			resp.Username = *account.Username
		}

		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Accounts retrieved successfully")
}

// getAccountsByRole handles getting accounts by role
func (rs *Resource) getAccountsByRole(w http.ResponseWriter, r *http.Request) {
	roleName := chi.URLParam(r, "roleName")

	accounts, err := rs.AuthService.GetAccountsByRole(r.Context(), roleName)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	responses := make([]*AccountResponse, 0, len(accounts))
	for _, account := range accounts {
		resp := &AccountResponse{
			ID:     account.ID,
			Email:  account.Email,
			Active: account.Active,
		}

		if account.Username != nil {
			resp.Username = *account.Username
		}

		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Accounts retrieved successfully")
}

// Password Reset Endpoints

// initiatePasswordReset handles initiating a password reset
func (rs *Resource) initiatePasswordReset(w http.ResponseWriter, r *http.Request) {
	req := &PasswordResetRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Always return success to avoid revealing whether email exists, but handle rate limiting
	_, err := rs.AuthService.InitiatePasswordReset(r.Context(), req.Email)
	if err != nil {
		var rateErr *authService.RateLimitError
		if errors.As(err, &rateErr) {
			// Prefer Retry-After seconds, fallback to RFC1123 format
			retryAfterSeconds := rateErr.RetryAfterSeconds(time.Now())
			if retryAfterSeconds > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
			} else if !rateErr.RetryAt.IsZero() {
				w.Header().Set("Retry-After", rateErr.RetryAt.UTC().Format(http.TimeFormat))
			}

			if renderErr := render.Render(w, r, common.ErrorTooManyRequests(authService.ErrRateLimitExceeded)); renderErr != nil {
				log.Printf("Render error: %v", renderErr)
			}
			return
		}

		if renderErr := render.Render(w, r, ErrorInternalServer(err)); renderErr != nil {
			log.Printf("Render error: %v", renderErr)
		}
		return
	}

	log.Printf("Password reset initiated for email=%s", req.Email)

	common.Respond(w, r, http.StatusOK, nil, "If the email exists, a password reset link has been sent")
}

// resetPassword handles resetting password with token
func (rs *Resource) resetPassword(w http.ResponseWriter, r *http.Request) {
	req := &PasswordResetConfirmRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	if err := rs.AuthService.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		log.Printf("Password reset failed token=%s reason=%v", req.Token, err)

		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrInvalidToken):
				if renderErr := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid or expired reset token"))); renderErr != nil {
					log.Printf("Render error: %v", renderErr)
				}
				return
			case errors.Is(authErr.Err, authService.ErrPasswordTooWeak):
				if renderErr := render.Render(w, r, ErrorInvalidRequest(authService.ErrPasswordTooWeak)); renderErr != nil {
					log.Printf("Render error: %v", renderErr)
				}
				return
			case errors.Is(authErr.Err, sql.ErrNoRows):
				if renderErr := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid or expired reset token"))); renderErr != nil {
					log.Printf("Render error: %v", renderErr)
				}
				return
			}
		}

		if renderErr := render.Render(w, r, ErrorInternalServer(err)); renderErr != nil {
			log.Printf("Render error: %v", renderErr)
		}
		return
	}

	log.Printf("Password reset completed for token=%s", req.Token)

	common.Respond(w, r, http.StatusOK, nil, "Password reset successfully")
}

// Token Management Endpoints

// cleanupExpiredTokens handles cleanup of expired tokens
func (rs *Resource) cleanupExpiredTokens(w http.ResponseWriter, r *http.Request) {
	count, err := rs.AuthService.CleanupExpiredTokens(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	response := map[string]int{"cleaned_tokens": count}
	common.Respond(w, r, http.StatusOK, response, "Expired tokens cleaned up successfully")
}

// revokeAllTokens handles revoking all tokens for an account
func (rs *Resource) revokeAllTokens(w http.ResponseWriter, r *http.Request) {
	accountIDStr := chi.URLParam(r, "accountId")
	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid account ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	if err := rs.AuthService.RevokeAllTokens(r.Context(), accountID); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.RespondNoContent(w, r)
}

// getActiveTokens handles getting active tokens for an account
func (rs *Resource) getActiveTokens(w http.ResponseWriter, r *http.Request) {
	accountIDStr := chi.URLParam(r, "accountId")
	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid account ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	tokens, err := rs.AuthService.GetActiveTokens(r.Context(), accountID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	type TokenResponse struct {
		ID         int64  `json:"id"`
		Token      string `json:"token"`
		Expiry     string `json:"expiry"`
		Mobile     bool   `json:"mobile"`
		Identifier string `json:"identifier,omitempty"`
		CreatedAt  string `json:"created_at"`
	}

	responses := make([]*TokenResponse, 0, len(tokens))
	for _, token := range tokens {
		resp := &TokenResponse{
			ID:        token.ID,
			Token:     token.Token,
			Expiry:    token.Expiry.Format(time.RFC3339),
			Mobile:    token.Mobile,
			CreatedAt: token.CreatedAt.Format(time.RFC3339),
		}

		if token.Identifier != nil {
			resp.Identifier = *token.Identifier
		}

		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Active tokens retrieved successfully")
}

// Parent Account Management Endpoints

// createParentAccount handles creating a parent account
func (rs *Resource) createParentAccount(w http.ResponseWriter, r *http.Request) {
	req := &CreateParentAccountRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	parentAccount, err := rs.AuthService.CreateParentAccount(r.Context(), req.Email, req.Username, req.Password)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrEmailAlreadyExists):
				if err := render.Render(w, r, ErrorInvalidRequest(authService.ErrEmailAlreadyExists)); err != nil {
					log.Printf("Render error: %v", err)
				}
			case errors.Is(authErr.Err, authService.ErrUsernameAlreadyExists):
				if err := render.Render(w, r, ErrorInvalidRequest(authService.ErrUsernameAlreadyExists)); err != nil {
					log.Printf("Render error: %v", err)
				}
			default:
				if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
					log.Printf("Render error: %v", err)
				}
			}
			return
		}
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	resp := &ParentAccountResponse{
		ID:        parentAccount.ID,
		Email:     parentAccount.Email,
		Active:    parentAccount.Active,
		CreatedAt: parentAccount.CreatedAt.Format(time.RFC3339),
		UpdatedAt: parentAccount.UpdatedAt.Format(time.RFC3339),
	}

	if parentAccount.Username != nil {
		resp.Username = *parentAccount.Username
	}

	common.Respond(w, r, http.StatusCreated, resp, "Parent account created successfully")
}

// getParentAccountByID handles getting a parent account by ID
func (rs *Resource) getParentAccountByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid parent account ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	parentAccount, err := rs.AuthService.GetParentAccountByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("parent account not found"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	resp := &ParentAccountResponse{
		ID:        parentAccount.ID,
		Email:     parentAccount.Email,
		Active:    parentAccount.Active,
		CreatedAt: parentAccount.CreatedAt.Format(time.RFC3339),
		UpdatedAt: parentAccount.UpdatedAt.Format(time.RFC3339),
	}

	if parentAccount.Username != nil {
		resp.Username = *parentAccount.Username
	}

	common.Respond(w, r, http.StatusOK, resp, "Parent account retrieved successfully")
}

// updateParentAccount handles updating a parent account
func (rs *Resource) updateParentAccount(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid parent account ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	req := &UpdateAccountRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	parentAccount, err := rs.AuthService.GetParentAccountByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("parent account not found"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	parentAccount.Email = req.Email
	if req.Username != "" {
		username := req.Username
		parentAccount.Username = &username
	}

	if err := rs.AuthService.UpdateParentAccount(r.Context(), parentAccount); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.RespondNoContent(w, r)
}

// activateParentAccount handles activating a parent account
func (rs *Resource) activateParentAccount(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid parent account ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	if err := rs.AuthService.ActivateParentAccount(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.RespondNoContent(w, r)
}

// deactivateParentAccount handles deactivating a parent account
func (rs *Resource) deactivateParentAccount(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid parent account ID"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	if err := rs.AuthService.DeactivateParentAccount(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.RespondNoContent(w, r)
}

// listParentAccounts handles listing parent accounts
func (rs *Resource) listParentAccounts(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters for filtering
	filters := make(map[string]interface{})

	if email := r.URL.Query().Get("email"); email != "" {
		filters["email"] = email
	}

	if active := r.URL.Query().Get("active"); active != "" {
		switch active {
		case "true":
			filters["active"] = true
		case "false":
			filters["active"] = false
		}
	}

	parentAccounts, err := rs.AuthService.ListParentAccounts(r.Context(), filters)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	responses := make([]*ParentAccountResponse, 0, len(parentAccounts))
	for _, account := range parentAccounts {
		resp := &ParentAccountResponse{
			ID:        account.ID,
			Email:     account.Email,
			Active:    account.Active,
			CreatedAt: account.CreatedAt.Format(time.RFC3339),
			UpdatedAt: account.UpdatedAt.Format(time.RFC3339),
		}

		if account.Username != nil {
			resp.Username = *account.Username
		}

		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Parent accounts retrieved successfully")
}

// getClientIP extracts the real client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Real-IP header first (set by reverse proxy)
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Split the header value on commas and trim each entry
		ips := strings.Split(xff, ",")
		for i, ip := range ips {
			ips[i] = strings.TrimSpace(ip)
		}
		// Return the first IP in the list
		if len(ips) > 0 {
			return ips[0]
		}
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}
