package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// Constants for permission strings, headers, route patterns, and error messages (S1192 - avoid duplicate string literals)
const (
	permUsersCreate  = "users:create"
	permUsersManage  = "users:manage"
	permUsersList    = "users:list"
	permRolesRead    = "roles:read"
	permUsersUpdate  = "users:update"
	permRolesManage  = "roles:manage"
	pathPermissionID = "/{permissionId}"
	pathPermissions  = "/permissions"
)

// Resource defines the auth resource
type Resource struct {
	AuthService                authService.AuthService
	InvitationService          authService.InvitationService
	GuardianInvitationService  authService.GuardianInvitationService
	CaregiverCapabilityService usersService.CaregiverCapabilityService
	SchoolService              platformSvc.SchoolService
	// SettingsService enriches tenant-shell metadata. Some optional feature
	// flags retain defensive fallbacks, but resolveTenant requires this service
	// for the grade-level validation contract and returns 500 when it is absent.
	SettingsService configSvc.SettingsService
	// MFAService is optional during the rollout window — handlers gate on
	// nil and return 503 so deployments without the service wired in don't
	// crash. Once Phase 7 lands the login-flow integration this will become
	// effectively mandatory.
	MFAService      authService.MFAService
	PasskeyService  authService.PasskeyService
	db              *bun.DB
	authRateLimiter func(http.Handler) http.Handler
}

// SetAuthRateLimiter sets the rate limiter middleware for auth endpoints (login, register, password-reset).
func (rs *Resource) SetAuthRateLimiter(mw func(http.Handler) http.Handler) {
	rs.authRateLimiter = mw
}

// SetMFAService wires the optional MFA service. Setter pattern matches
// SetSettingsService — keeps the NewResource constructor signature
// backward-compatible while phases roll in.
func (rs *Resource) SetMFAService(svc authService.MFAService) {
	rs.MFAService = svc
}

func (rs *Resource) SetPasskeyService(svc authService.PasskeyService) {
	rs.PasskeyService = svc
}

// SetGuardianInvitationService injects the guardian invitation service.
// Wired via setter (not constructor) so existing test call sites that pass 4
// positional args keep compiling. When nil, the public guardian invitation
// routes return 500 with errGuardianInvitationServiceUnavailable.
func (rs *Resource) SetGuardianInvitationService(svc authService.GuardianInvitationService) {
	rs.GuardianInvitationService = svc
}

// NewResource creates a new auth resource
func NewResource(authService authService.AuthService, invitationService authService.InvitationService, schoolService platformSvc.SchoolService, db *bun.DB) *Resource {
	return &Resource{
		AuthService:       authService,
		InvitationService: invitationService,
		SchoolService:     schoolService,
		db:                db,
	}
}

func requirePlatformScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := jwt.ClaimsFromCtx(r.Context())
		if !claims.IsPlatformScope() {
			common.RenderError(w, r, authorize.ErrForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Router returns a configured router for auth endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustNewTokenAuth()

	// Rate-limited public routes (brute-force protection)
	r.Group(func(r chi.Router) {
		if rs.authRateLimiter != nil {
			r.Use(rs.authRateLimiter)
		}
		r.Post("/login", rs.login)
		r.Post("/password-reset", rs.initiatePasswordReset)
		r.Post("/password-reset/confirm", rs.resetPassword)

		// MFA challenge → token-pair exchange (issue #1308). These endpoints
		// take the short-lived challenge JWT in the request body, NOT in the
		// Authorization header — the user is mid-login and has no access
		// token yet.
		r.Post("/mfa/verify", rs.mfaVerify)
		r.Post("/mfa/resend", rs.mfaResend)
		r.Post("/passkeys/login/options", rs.passkeyLoginOptions)
		r.Post("/passkeys/login/verify", rs.passkeyLoginVerify)
	})

	// Public routes (no rate limiting — these are read-only lookups)
	r.Get("/invitations/{token}", rs.validateInvitation)
	r.Post("/invitations/{token}/accept", rs.acceptInvitation)
	r.Get("/guardian-invitations/{token}", rs.validateGuardianInvitation)
	r.Post("/guardian-invitations/{token}/accept", rs.acceptGuardianInvitation)
	r.Get("/tenant/resolve", rs.resolveTenant)
	r.Get("/tenants", rs.listTenants)

	// Protected routes that require refresh token
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
		r.Use(jwt.AuthenticateRefreshJWT)
		r.Post("/refresh", rs.refreshToken)
		r.Post("/logout", rs.logout)
	})

	// Enrollment-only routes (issue #1308): accept the narrow enrollment
	// JWT that login mints for accounts on an mfa-required tenant that
	// have no credential yet. Lives in its own group with a dedicated
	// authenticator so the enrollment token NEVER reaches a fully
	// authenticated handler. The full session is minted by
	// mfaEnrollConfirm after successful enrollment.
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
		r.Use(jwt.MFAEnrollmentAuthenticator)
		r.Route("/mfa/enroll", func(r chi.Router) {
			r.Post("/start", rs.mfaEnrollStart)
			r.Post("/confirm", rs.mfaEnrollConfirm)
		})
	})

	// Protected routes that require access token
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)

		// Tenant switching
		r.Post("/switch-tenant", rs.switchTenant)

		// Current user routes
		r.Get("/account", rs.getAccount)
		r.Get("/account/tenants", rs.listAccountTenants)

		// Password change - users can change their own password without special permissions
		r.Post("/password", rs.changePassword)

		// Self-service trusted-device management — every authenticated
		// account can see and revoke its own remembered devices from
		// the admin Sicherheit tab. Ownership is enforced in the
		// service so this stays a plain authenticated route.
		r.Route("/mfa", func(r chi.Router) {
			r.Get("/trusted-devices", rs.mfaListTrustedDevices)
			r.Delete("/trusted-devices/{deviceId}", rs.mfaRevokeTrustedDevice)
		})
		r.Route("/passkeys", func(r chi.Router) {
			r.Get("/", rs.passkeyList)
			r.Post("/enrollment/challenge", rs.passkeyEnrollmentChallenge)
			r.Post("/register/options", rs.passkeyRegisterOptions)
			r.Post("/register/verify", rs.passkeyRegisterVerify)
			r.Delete("/{passkeyId}", rs.passkeyRevoke)
		})

		// Admin routes - require admin role or specific permissions
		// TenantTxMiddleware wraps each request in a DB transaction as phoenix_tenant
		// with RLS scoping (SET LOCAL ROLE + set_config). Without this, queries run
		// as phoenix_auth which only has SELECT on auth tables.
		withTx := common.TenantTxMiddleware
		r.Group(func(r chi.Router) {
			r.Use(withTx)

			// Account creation — uses users:manage (not users:create) because
			// the "user" role is granted users:create in migrations; manage
			// restricts this to actual administrators.
			//
			// link-to-tenant carries the same weight: it assigns a role to an
			// existing account inside the caller's tenant, so guarding it with
			// users:create would let every "user" account grant itself the
			// admin role (issue #1021 review).
			r.With(authorize.RequiresPermission(permUsersManage)).Post("/register", rs.register)
			r.With(authorize.RequiresPermission(permUsersManage)).Post("/link-to-tenant", rs.linkToTenant)

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
				r.With(requirePlatformScope, authorize.RequiresPermission("permissions:create")).Post("/", rs.createPermission)
				r.With(authorize.RequiresPermission("permissions:read")).Get("/", rs.listPermissions)
				r.Route("/{id}", func(r chi.Router) {
					r.With(authorize.RequiresPermission("permissions:read")).Get("/", rs.getPermissionByID)
					r.With(requirePlatformScope, authorize.RequiresPermission("permissions:update")).Put("/", rs.updatePermission)
					r.With(requirePlatformScope, authorize.RequiresPermission("permissions:delete")).Delete("/", rs.deletePermission)
				})
			})

			// Account management routes
			r.Route("/accounts", func(r chi.Router) {
				r.With(authorize.RequiresPermission(permUsersList)).Get("/", rs.listAccounts)
				r.With(authorize.RequiresPermission("users:read")).Get("/by-role/{roleName}", rs.getAccountsByRole)

				r.Route("/{accountId}", func(r chi.Router) {
					// Account update operations
					r.With(authorize.RequiresPermission(permUsersUpdate)).Put("/", rs.updateAccount)
					r.With(authorize.RequiresPermission(permUsersUpdate)).Put("/activate", common.IDAction("accountId", common.MsgInvalidAccountID, rs.AuthService.ActivateAccount, accountManagementErrorRenderer))
					r.With(authorize.RequiresPermission(permUsersUpdate)).Put("/deactivate", common.IDAction("accountId", common.MsgInvalidAccountID, rs.AuthService.DeactivateAccount, accountManagementErrorRenderer))
					r.With(authorize.RequiresPermission(permUsersManage)).Get("/caregiver-capability", rs.getCaregiverCapability)
					r.With(authorize.RequiresPermission(permUsersManage)).Post("/caregiver-capability", rs.enableCaregiverCapability)
					r.With(authorize.RequiresPermission(permUsersManage)).Delete("/caregiver-capability", rs.disableCaregiverCapability)

					// Role assignments
					r.Route("/roles", func(r chi.Router) {
						r.With(authorize.RequiresPermission(permUsersManage)).Get("/", rs.getAccountRoles)
						r.With(authorize.RequiresPermission(permUsersManage)).Post("/{roleId}", rs.assignRoleToAccount)
						r.With(authorize.RequiresPermission(permUsersManage)).Delete("/{roleId}", common.TwoIDAction("accountId", common.MsgInvalidAccountID, "roleId", common.MsgInvalidRoleID, rs.AuthService.RemoveRoleFromAccount, accountManagementErrorRenderer))
					})

					// Permission assignments
					r.Route(pathPermissions, func(r chi.Router) {
						r.With(authorize.RequiresPermission(permUsersManage)).Get("/", rs.getAccountPermissions)
						r.With(authorize.RequiresPermission(permUsersManage)).Get("/direct", rs.getAccountDirectPermissions)
						r.With(authorize.RequiresPermission(permUsersManage)).Post(pathPermissionID+"/grant", common.TwoIDAction("accountId", common.MsgInvalidAccountID, "permissionId", common.MsgInvalidPermissionID, rs.AuthService.GrantPermissionToAccount, accountManagementErrorRenderer))
						r.With(authorize.RequiresPermission(permUsersManage)).Post(pathPermissionID+"/deny", common.TwoIDAction("accountId", common.MsgInvalidAccountID, "permissionId", common.MsgInvalidPermissionID, rs.AuthService.DenyPermissionToAccount, accountManagementErrorRenderer))
						r.With(authorize.RequiresPermission(permUsersManage)).Delete(pathPermissionID, common.TwoIDAction("accountId", common.MsgInvalidAccountID, "permissionId", common.MsgInvalidPermissionID, rs.AuthService.RemovePermissionFromAccount, accountManagementErrorRenderer))
					})

					// Token management
					r.Route("/tokens", func(r chi.Router) {
						r.With(authorize.RequiresPermission(permUsersManage)).Get("/", rs.getActiveTokens)
						r.With(authorize.RequiresPermission(permUsersManage)).Delete("/", common.IDAction("accountId", common.MsgInvalidAccountID, rs.AuthService.RevokeAllTokens, common.ErrorInternalServer))
					})

					// MFA admin override ("Godmode") — issue #1308 Phase 6.
					// users:manage at the route layer; the service does its own
					// defense-in-depth permission re-check.
					r.Route("/mfa", func(r chi.Router) {
						r.With(authorize.RequiresPermission(permUsersManage)).Get("/", rs.mfaAdminGetState)
						r.With(authorize.RequiresPermission(permUsersManage)).Delete("/", rs.mfaAdminDisable)
						r.With(authorize.RequiresPermission(permUsersManage)).Put("/override", rs.mfaAdminSetOverride)
					})
				})
			})

			// Role permission assignments
			r.Route("/roles/{roleId}/permissions", func(r chi.Router) {
				r.With(authorize.RequiresPermission(permRolesManage)).Get("/", rs.getRolePermissions)
				r.With(authorize.RequiresPermission(permRolesManage)).Post(pathPermissionID, common.TwoIDAction("roleId", common.MsgInvalidRoleID, "permissionId", common.MsgInvalidPermissionID, rs.AuthService.AssignPermissionToRole, renderRoleMutationError))
				r.With(authorize.RequiresPermission(permRolesManage)).Delete(pathPermissionID, common.TwoIDAction("roleId", common.MsgInvalidRoleID, "permissionId", common.MsgInvalidPermissionID, rs.AuthService.RemovePermissionFromRole, renderRoleMutationError))
			})

			// Token cleanup
			r.Route("/tokens", func(r chi.Router) {
				r.With(authorize.RequiresPermission("admin:*")).Delete("/expired", rs.cleanupExpiredTokens)
			})

			r.Route("/invitations", func(r chi.Router) {
				// users:manage, not users:create: the "user" role carries
				// users:create globally (migration 1.5.3, so Betreuer can
				// create person records), and an invitation hands out a role.
				// The UI already gates this screen on the admin role.
				r.With(authorize.RequiresPermission(permUsersManage)).Post("/", rs.createInvitation)
				r.With(authorize.RequiresPermission(permUsersList)).Get("/", rs.listPendingInvitations)
				r.Route("/{id}", func(r chi.Router) {
					r.With(authorize.RequiresPermission(permUsersManage)).Post("/resend", rs.resendInvitation)
					r.With(authorize.RequiresPermission(permUsersManage)).Delete("/", rs.revokeInvitation)
				})
			})

			// Guardian invitations — public accept handled above; admins
			// can resend a still-valid invitation. Create endpoint is
			// deliberately omitted in PR 3 (per parent-enrollment plan):
			// the decision service in PR 8 calls Service.Create directly
			// when approving the first child.
			r.Route("/guardian-invitations", func(r chi.Router) {
				r.Route("/{id}", func(r chi.Router) {
					r.With(authorize.RequiresPermission(permUsersManage)).Post("/resend", rs.resendGuardianInvitation)
				})
			})

			// Parent account management
			r.Route("/parent-accounts", func(r chi.Router) {
				r.With(authorize.RequiresPermission(permUsersCreate)).Post("/", rs.createParentAccount)
				r.With(authorize.RequiresPermission(permUsersList)).Get("/", rs.listParentAccounts)
				r.Route("/{id}", func(r chi.Router) {
					r.With(authorize.RequiresPermission("users:read")).Get("/", rs.getParentAccountByID)
					r.With(authorize.RequiresPermission(permUsersUpdate)).Put("/", rs.updateParentAccount)
					r.With(authorize.RequiresPermission(permUsersUpdate)).Put("/activate", common.IDAction("id", common.MsgInvalidParentAccountID, rs.AuthService.ActivateParentAccount, common.ErrorInternalServer))
					r.With(authorize.RequiresPermission(permUsersUpdate)).Put("/deactivate", common.IDAction("id", common.MsgInvalidParentAccountID, rs.AuthService.DeactivateParentAccount, common.ErrorInternalServer))
				})
			})
		})
	})

	return r
}
