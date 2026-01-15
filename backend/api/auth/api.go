package auth

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// Constants for permission strings, headers, and route patterns (S1192 - avoid duplicate string literals)
const (
	permUsersManage  = "users:manage"
	permUsersList    = "users:list"
	permRolesRead    = "roles:read"
	permUsersUpdate  = "users:update"
	permRolesManage  = "roles:manage"
	headerUserAgent  = "User-Agent"
	pathPermissionID = "/{permissionId}"
	pathPermissions  = "/permissions"
)

// Resource defines the auth resource
type Resource struct {
	AuthService       authService.AuthService
	InvitationService authService.InvitationService
}

// NewResource creates a new auth resource
func NewResource(authService authService.AuthService, invitationService authService.InvitationService) *Resource {
	return &Resource{
		AuthService:       authService,
		InvitationService: invitationService,
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
	r.Get("/invitations/{token}", rs.validateInvitation)
	r.Post("/invitations/{token}/accept", rs.acceptInvitation)

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
				r.With(authorize.RequiresPermission(permRolesRead)).Get("/", rs.listRoles)
				r.Route("/{id}", func(r chi.Router) {
					r.With(authorize.RequiresPermission(permRolesRead)).Get("/", rs.getRoleByID)
					r.With(authorize.RequiresPermission("roles:update")).Put("/", rs.updateRole)
					r.With(authorize.RequiresPermission("roles:delete")).Delete("/", rs.deleteRole)
					r.With(authorize.RequiresPermission(permRolesRead)).Get(pathPermissions, rs.getRolePermissions)
				})
			})

			// Permission management routes
			r.Route(pathPermissions, func(r chi.Router) {
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
				r.With(authorize.RequiresPermission(permUsersList)).Get("/", rs.listAccounts)
				r.With(authorize.RequiresPermission("users:read")).Get("/by-role/{roleName}", rs.getAccountsByRole)

				r.Route("/{accountId}", func(r chi.Router) {
					// Account update operations
					r.With(authorize.RequiresPermission(permUsersUpdate)).Put("/", rs.updateAccount)
					r.With(authorize.RequiresPermission(permUsersUpdate)).Put("/activate", rs.activateAccount)
					r.With(authorize.RequiresPermission(permUsersUpdate)).Put("/deactivate", rs.deactivateAccount)

					// Role assignments
					r.Route("/roles", func(r chi.Router) {
						r.With(authorize.RequiresPermission(permUsersManage)).Get("/", rs.getAccountRoles)
						r.With(authorize.RequiresPermission(permUsersManage)).Post("/{roleId}", rs.assignRoleToAccount)
						r.With(authorize.RequiresPermission(permUsersManage)).Delete("/{roleId}", rs.removeRoleFromAccount)
					})

					// Permission assignments
					r.Route(pathPermissions, func(r chi.Router) {
						r.With(authorize.RequiresPermission(permUsersManage)).Get("/", rs.getAccountPermissions)
						r.With(authorize.RequiresPermission(permUsersManage)).Get("/direct", rs.getAccountDirectPermissions)
						r.With(authorize.RequiresPermission(permUsersManage)).Post(pathPermissionID+"/grant", rs.grantPermissionToAccount)
						r.With(authorize.RequiresPermission(permUsersManage)).Post(pathPermissionID+"/deny", rs.denyPermissionToAccount)
						r.With(authorize.RequiresPermission(permUsersManage)).Delete(pathPermissionID, rs.removePermissionFromAccount)
					})

					// Token management
					r.Route("/tokens", func(r chi.Router) {
						r.With(authorize.RequiresPermission(permUsersManage)).Get("/", rs.getActiveTokens)
						r.With(authorize.RequiresPermission(permUsersManage)).Delete("/", rs.revokeAllTokens)
					})
				})
			})

			// Role permission assignments
			r.Route("/roles/{roleId}/permissions", func(r chi.Router) {
				r.With(authorize.RequiresPermission(permRolesManage)).Get("/", rs.getRolePermissions)
				r.With(authorize.RequiresPermission(permRolesManage)).Post(pathPermissionID, rs.assignPermissionToRole)
				r.With(authorize.RequiresPermission(permRolesManage)).Delete(pathPermissionID, rs.removePermissionFromRole)
			})

			// Token cleanup
			r.Route("/tokens", func(r chi.Router) {
				r.With(authorize.RequiresPermission("admin:*")).Delete("/expired", rs.cleanupExpiredTokens)
			})

			r.Route("/invitations", func(r chi.Router) {
				r.With(authorize.RequiresPermission("users:create")).Post("/", rs.createInvitation)
				r.With(authorize.RequiresPermission(permUsersList)).Get("/", rs.listPendingInvitations)
				r.Route("/{id}", func(r chi.Router) {
					r.With(authorize.RequiresPermission(permUsersManage)).Post("/resend", rs.resendInvitation)
					r.With(authorize.RequiresPermission(permUsersManage)).Delete("/", rs.revokeInvitation)
				})
			})

			// Parent account management
			r.Route("/parent-accounts", func(r chi.Router) {
				r.With(authorize.RequiresPermission("users:create")).Post("/", rs.createParentAccount)
				r.With(authorize.RequiresPermission(permUsersList)).Get("/", rs.listParentAccounts)
				r.Route("/{id}", func(r chi.Router) {
					r.With(authorize.RequiresPermission("users:read")).Get("/", rs.getParentAccountByID)
					r.With(authorize.RequiresPermission(permUsersUpdate)).Put("/", rs.updateParentAccount)
					r.With(authorize.RequiresPermission(permUsersUpdate)).Put("/activate", rs.activateParentAccount)
					r.With(authorize.RequiresPermission(permUsersUpdate)).Put("/deactivate", rs.deactivateParentAccount)
				})
			})
		})
	})

	return r
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
		logger.Logger.Warn("Security: Unauthenticated register attempt with role_id, ignoring role_id")
		return nil, false
	}

	token := authHeader[7:]
	callerAccount, err := rs.AuthService.ValidateToken(r.Context(), token)
	if err != nil {
		logger.Logger.Warn("Security: Invalid token in register with role_id, ignoring role_id")
		return nil, false
	}

	if !hasAdminRole(callerAccount.Roles) {
		logger.Logger.WithField("account_id", callerAccount.ID).Warn("Security: Non-admin attempted to set role_id, denying")
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
		logger.Logger.WithFields(map[string]interface{}{
			"ip":    ipAddress,
			"error": err.Error(),
		}).Warn("Logout audit logging failed (client logout still successful)")
	}

	common.RespondNoContent(w, r)
}

// changePassword handles password change
func (rs *Resource) changePassword(w http.ResponseWriter, r *http.Request) {
	req := &ChangePasswordRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
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
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrInvalidCredentials))
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrAccountNotFound))
			case errors.Is(authErr.Err, authService.ErrPasswordTooWeak):
				common.RenderError(w, r, ErrorInvalidRequest(authService.ErrPasswordTooWeak))
			default:
				common.RenderError(w, r, ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, ErrorInternalServer(err))
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
				common.RenderError(w, r, ErrorNotFound(authService.ErrAccountNotFound))
				return
			}
		}
		common.RenderError(w, r, ErrorInternalServer(err))
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
