package auth

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	authService "github.com/moto-nrw/project-phoenix/internal/core/service/auth"
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
	tokenAuth := jwt.MustTokenAuth()

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
