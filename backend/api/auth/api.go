package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Constants for permission strings, headers, route patterns, and error messages (S1192 - avoid duplicate string literals)
const (
	permUsersCreate      = "users:create"
	permUsersManage      = "users:manage"
	permUsersList        = "users:list"
	permRolesRead        = "roles:read"
	permUsersUpdate      = "users:update"
	permRolesManage      = "roles:manage"
	headerUserAgent      = "User-Agent"
	pathPermissionID     = "/{permissionId}"
	pathPermissions      = "/permissions"
	errPasswordsNotMatch = "passwords do not match"
)

// Resource defines the auth resource
type Resource struct {
	AuthService                authService.AuthService
	InvitationService          authService.InvitationService
	GuardianInvitationService  authService.GuardianInvitationService
	CaregiverCapabilityService usersService.CaregiverCapabilityService
	SchoolService              platformSvc.SchoolService
	// SettingsService is optional — when non-nil, resolveTenant enriches its
	// response with tenant-scoped presence_mode so the frontend can decide
	// between PresenceBadge and LocationBadge without a second fetch.
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
		withTx := tenant.TenantTxMiddleware(rs.db)
		r.Group(func(r chi.Router) {
			r.Use(withTx)

			// Account creation — uses users:manage (not users:create) because
			// the "user" role is granted users:create in migrations; manage
			// restricts this to actual administrators.
			r.With(authorize.RequiresPermission(permUsersManage)).Post("/register", rs.register)
			r.With(authorize.RequiresPermission(permUsersCreate)).Post("/link-to-tenant", rs.linkToTenant)

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
					r.With(authorize.RequiresPermission(permUsersUpdate)).Put("/activate", idAction("accountId", common.MsgInvalidAccountID, rs.AuthService.ActivateAccount, common.ErrorInternalServer))
					r.With(authorize.RequiresPermission(permUsersUpdate)).Put("/deactivate", idAction("accountId", common.MsgInvalidAccountID, rs.AuthService.DeactivateAccount, common.ErrorInternalServer))
					r.With(authorize.RequiresPermission(permUsersManage)).Get("/caregiver-capability", rs.getCaregiverCapability)
					r.With(authorize.RequiresPermission(permUsersManage)).Post("/caregiver-capability", rs.enableCaregiverCapability)
					r.With(authorize.RequiresPermission(permUsersManage)).Delete("/caregiver-capability", rs.disableCaregiverCapability)

					// Role assignments
					r.Route("/roles", func(r chi.Router) {
						r.With(authorize.RequiresPermission(permUsersManage)).Get("/", rs.getAccountRoles)
						r.With(authorize.RequiresPermission(permUsersManage)).Post("/{roleId}", twoIDAction("accountId", common.MsgInvalidAccountID, "roleId", common.MsgInvalidRoleID, rs.AuthService.AssignRoleToAccount, common.ErrorInternalServer))
						r.With(authorize.RequiresPermission(permUsersManage)).Delete("/{roleId}", twoIDAction("accountId", common.MsgInvalidAccountID, "roleId", common.MsgInvalidRoleID, rs.AuthService.RemoveRoleFromAccount, common.ErrorInternalServer))
					})

					// Permission assignments
					r.Route(pathPermissions, func(r chi.Router) {
						r.With(authorize.RequiresPermission(permUsersManage)).Get("/", rs.getAccountPermissions)
						r.With(authorize.RequiresPermission(permUsersManage)).Get("/direct", rs.getAccountDirectPermissions)
						r.With(authorize.RequiresPermission(permUsersManage)).Post(pathPermissionID+"/grant", twoIDAction("accountId", common.MsgInvalidAccountID, "permissionId", common.MsgInvalidPermissionID, rs.AuthService.GrantPermissionToAccount, common.ErrorInternalServer))
						r.With(authorize.RequiresPermission(permUsersManage)).Post(pathPermissionID+"/deny", twoIDAction("accountId", common.MsgInvalidAccountID, "permissionId", common.MsgInvalidPermissionID, rs.AuthService.DenyPermissionToAccount, common.ErrorInternalServer))
						r.With(authorize.RequiresPermission(permUsersManage)).Delete(pathPermissionID, twoIDAction("accountId", common.MsgInvalidAccountID, "permissionId", common.MsgInvalidPermissionID, rs.AuthService.RemovePermissionFromAccount, common.ErrorInternalServer))
					})

					// Token management
					r.Route("/tokens", func(r chi.Router) {
						r.With(authorize.RequiresPermission(permUsersManage)).Get("/", rs.getActiveTokens)
						r.With(authorize.RequiresPermission(permUsersManage)).Delete("/", idAction("accountId", common.MsgInvalidAccountID, rs.AuthService.RevokeAllTokens, common.ErrorInternalServer))
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
				r.With(authorize.RequiresPermission(permRolesManage)).Post(pathPermissionID, twoIDAction("roleId", common.MsgInvalidRoleID, "permissionId", common.MsgInvalidPermissionID, rs.AuthService.AssignPermissionToRole, renderRoleMutationError))
				r.With(authorize.RequiresPermission(permRolesManage)).Delete(pathPermissionID, twoIDAction("roleId", common.MsgInvalidRoleID, "permissionId", common.MsgInvalidPermissionID, rs.AuthService.RemovePermissionFromRole, renderRoleMutationError))
			})

			// Token cleanup
			r.Route("/tokens", func(r chi.Router) {
				r.With(authorize.RequiresPermission("admin:*")).Delete("/expired", rs.cleanupExpiredTokens)
			})

			r.Route("/invitations", func(r chi.Router) {
				r.With(authorize.RequiresPermission(permUsersCreate)).Post("/", rs.createInvitation)
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
					r.With(authorize.RequiresPermission(permUsersUpdate)).Put("/activate", idAction("id", common.MsgInvalidParentAccountID, rs.AuthService.ActivateParentAccount, common.ErrorInternalServer))
					r.With(authorize.RequiresPermission(permUsersUpdate)).Put("/deactivate", idAction("id", common.MsgInvalidParentAccountID, rs.AuthService.DeactivateParentAccount, common.ErrorInternalServer))
				})
			})
		})
	})

	return r
}

// TenantResolveResponse represents the public tenant info returned by resolve
type TenantResolveResponse struct {
	TenantID         int64           `json:"tenant_id"`
	Slug             string          `json:"slug"`
	Name             string          `json:"name"`
	Subdomain        string          `json:"subdomain"`
	OrganizationID   int64           `json:"organization_id"`
	OrganizationName string          `json:"organization_name"`
	Hidden           bool            `json:"hidden"`
	Settings         json.RawMessage `json:"settings"`
	// PresenceMode is the tenant's resolved operations.presence_mode setting
	// ("detailed" | "binary"). Included so the frontend can render the right
	// badge component (PresenceBadge vs LocationBadge) without a second call.
	// Defaults to "detailed" if SettingsService is unavailable.
	PresenceMode string `json:"presence_mode"`
	// StudentPhotosEnabled is the tenant's resolved
	// operations.student_photos_enabled setting. Surfaced here (rather than
	// via /api/settings/schema) because every authenticated user — including
	// non-admin Betreuer who don't carry config:read — needs the flag to
	// decide whether to render avatars on student cards. Defaults to false
	// when the setting is missing or unresolvable.
	StudentPhotosEnabled bool `json:"student_photos_enabled"`
	// NFCEnabled is the tenant's resolved attendance.nfc_enabled setting.
	// It is public tenant-shell metadata because non-admin staff also need to
	// know whether NFC-only areas like classic activities should appear.
	NFCEnabled bool `json:"nfc_enabled"`
	// ParentMessagingEnabled is the tenant's resolved operations.parent_notes_enabled
	// setting. Surfaced here (like the photo flag) so non-admin staff — who don't
	// carry config:read — can hide the "Neue Nachricht" compose entry points when
	// the school has parent-OGS messaging turned off, instead of composing into a
	// 403 dead-end. Defaults to false when the setting is missing/unresolvable.
	ParentMessagingEnabled bool `json:"parent_messaging_enabled"`
}

// resolveTenant handles GET /auth/tenant/resolve?slug={slug}
func (rs *Resource) resolveTenant(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(r.URL.Query().Get("slug"))
	if slug == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("slug query parameter is required")))
		return
	}

	school, err := rs.SchoolService.GetSchoolBySubdomain(r.Context(), slug)
	if err != nil || school == nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("tenant not found")))
		return
	}

	if school.IsDeleted() {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("tenant not found")))
		return
	}

	if !school.Active {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("tenant not found")))
		return
	}

	// Parse settings JSON; fall back to empty object on invalid data
	settings := json.RawMessage(school.Settings)
	if !json.Valid(settings) {
		settings = json.RawMessage(`{}`)
	}

	var orgName string
	if school.Organization != nil {
		orgName = school.Organization.Name
	}

	// Resolve presence mode in the school's tenant context. resolveTenant is
	// unauthenticated, so we don't have a tenant in the request context — use
	// the ForTenant variant which opens its own tenant transaction.
	presenceMode := configSvc.ResolvePresenceModeForTenant(r.Context(), rs.SettingsService, school.ID, nil)

	// Same out-of-tenant-middleware story for the photo feature flag: read
	// the setting through ResolveStringForTenant (opens its own tenant tx)
	// and parse the boolean string. Failures fall back to false so a
	// settings outage never auto-enables the avatar UI for opt-out schools.
	studentPhotosEnabled := false
	nfcEnabled := false
	parentMessagingEnabled := false
	if rs.SettingsService != nil {
		val, err := rs.SettingsService.ResolveStringForTenant(r.Context(), school.ID, configModel.KeyStudentPhotosEnabled)
		if err == nil {
			studentPhotosEnabled = val == "true"
		}
		if val, err := rs.SettingsService.ResolveBoolForTenant(r.Context(), school.ID, configModel.KeyAttendanceNFCEnabled); err == nil {
			nfcEnabled = val
		}
		// Messaging compose visibility fails OPEN (counts a resolve error as
		// enabled), UNLIKE the photos/NFC flags above. It must agree with the unread
		// badge, the inbox row pills, and the reply path — all centralized in
		// parentmessaging.MessagingEnabled* — so a config-DB blip cannot half-disable
		// the UI (button hidden while the badge/inbox still show unread). Messaging is
		// a soft, non-destructive flag, so brief over-enabling on an outage is the
		// accepted trade for that consistency.
		parentMessagingEnabled = parentmessaging.MessagingEnabledForTenant(r.Context(), rs.SettingsService, school.ID, nil)
	}

	resp := &TenantResolveResponse{
		TenantID:               school.ID,
		Slug:                   school.Slug,
		Name:                   school.Name,
		Subdomain:              school.Subdomain,
		OrganizationID:         school.OrganizationID,
		OrganizationName:       orgName,
		Hidden:                 school.Hidden,
		Settings:               settings,
		PresenceMode:           presenceMode,
		StudentPhotosEnabled:   studentPhotosEnabled,
		NFCEnabled:             nfcEnabled,
		ParentMessagingEnabled: parentMessagingEnabled,
	}

	common.Respond(w, r, http.StatusOK, resp, "Tenant resolved successfully")
}

// SwitchTenantRequest represents the switch-tenant request payload
type SwitchTenantRequest struct {
	TenantSlug string `json:"tenant_slug"`
}

// Bind validates the switch-tenant request
func (req *SwitchTenantRequest) Bind(_ *http.Request) error {
	req.TenantSlug = strings.TrimSpace(req.TenantSlug)

	return validation.ValidateStruct(req,
		validation.Field(&req.TenantSlug, validation.Required),
	)
}

// switchTenant handles POST /auth/switch-tenant
func (rs *Resource) switchTenant(w http.ResponseWriter, r *http.Request) {
	req := &SwitchTenantRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Get account ID from JWT claims
	claims := jwt.ClaimsFromCtx(r.Context())

	accessToken, refreshToken, err := rs.AuthService.SwitchTenant(r.Context(), int64(claims.ID), req.TenantSlug)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountNotFound))
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountInactive))
			case errors.Is(authErr.Err, authService.ErrTenantNotFound):
				common.RenderError(w, r, common.ErrorNotFound(authService.ErrTenantNotFound))
			case errors.Is(authErr.Err, authService.ErrTenantAccessDenied):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrTenantAccessDenied))
			default:
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Return tokens in the same format as login
	render.JSON(w, r, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// AccountTenantResponse represents a tenant available to the authenticated user
type AccountTenantResponse struct {
	TenantID         int64  `json:"tenant_id"`
	Slug             string `json:"slug"`
	Name             string `json:"name"`
	Subdomain        string `json:"subdomain"`
	OrganizationID   int64  `json:"organization_id"`
	OrganizationName string `json:"organization_name"`
}

// listAccountTenants handles GET /auth/account/tenants
func (rs *Resource) listAccountTenants(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())

	schools, err := rs.SchoolService.ListActiveSchoolsByAccountID(r.Context(), int64(claims.ID))
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]AccountTenantResponse, 0, len(schools))
	for _, school := range schools {
		var orgName string
		if school.Organization != nil {
			orgName = school.Organization.Name
		}
		responses = append(responses, AccountTenantResponse{
			TenantID:         school.ID,
			Slug:             school.Slug,
			Name:             school.Name,
			Subdomain:        school.Subdomain,
			OrganizationID:   school.OrganizationID,
			OrganizationName: orgName,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Account tenants retrieved successfully")
}

// PublicTenantResponse is the public-facing tenant info returned by the
// unauthenticated /auth/tenants endpoint. It intentionally omits internal
// database IDs (tenant_id, organization_id) to prevent enumeration.
type PublicTenantResponse struct {
	Slug             string `json:"slug"`
	Name             string `json:"name"`
	Subdomain        string `json:"subdomain"`
	OrganizationName string `json:"organization_name"`
}

// listTenants handles GET /auth/tenants (public, no auth required).
// Uses ListPublic to exclude hidden schools from the public landing page.
// Hidden schools remain accessible via direct subdomain link.
func (rs *Resource) listTenants(w http.ResponseWriter, r *http.Request) {
	schools, err := rs.SchoolService.ListPublicSchools(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]PublicTenantResponse, 0, len(schools))
	for _, school := range schools {
		var orgName string
		if school.Organization != nil {
			orgName = school.Organization.Name
		}
		responses = append(responses, PublicTenantResponse{
			Slug:             school.Slug,
			Name:             school.Name,
			Subdomain:        school.Subdomain,
			OrganizationName: orgName,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Tenants retrieved successfully")
}

// LoginRequest represents the login request payload
type LoginRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	TenantSlug string `json:"tenant_slug,omitempty"`
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

// LoginResponse is the discriminated response shape /auth/login returns now
// that MFA can short-circuit the token pair. The `status` field tells the
// client which fields to read:
//
//   - status == "authenticated"           → access_token + refresh_token
//     populated; the frontend stores them as before.
//   - status == "mfa_required"             → challenge_token + masked_email
//     populated; the frontend renders the code-entry UI and POSTs to
//     /auth/mfa/verify.
//   - status == "mfa_enrollment_required"  → access_token carries a
//     short-lived enrollment-scoped JWT (no refresh_token). The frontend
//     uses it as the Authorization header for /auth/mfa/enroll/* and
//     replaces it with the real token pair returned by
//     /auth/mfa/enroll/confirm. The token is REJECTED by the regular
//     Authenticator middleware, so it cannot be used to bypass MFA.
//
// mfa_enrollment_required is retained as a legacy hint for clients that key
// off the boolean rather than the status discriminator; new code should
// branch on `status`.
type LoginResponse struct {
	Status                string `json:"status"`
	AccessToken           string `json:"access_token,omitempty"`
	RefreshToken          string `json:"refresh_token,omitempty"`
	ChallengeToken        string `json:"challenge_token,omitempty"`
	MaskedEmail           string `json:"masked_email,omitempty"`
	MFAEnrollmentRequired bool   `json:"mfa_enrollment_required,omitempty"`
	// TrustedDeviceEnabled is sent on the mfa_required branch so the
	// frontend can hide the "remember this device" checkbox when the
	// tenant admin has disabled the feature. A missing field defaults to
	// "enabled" on the client for backwards compatibility.
	TrustedDeviceEnabled *bool `json:"trusted_device_enabled,omitempty"`
	// TrustedDeviceDays mirrors security.mfa_trusted_device_days for the
	// tenant. The frontend uses it to render the dynamic "Auf diesem
	// Gerät N Tage merken" label so the checkbox always reflects the
	// cookie lifetime the backend will actually issue.
	TrustedDeviceDays *int `json:"trusted_device_days,omitempty"`
}

// login handles user login. The handler is a thin orchestrator: it pulls
// the trusted-device cookie off the request, calls LoginWithMFAGate, and
// translates the discriminated LoginResult into a JSON shape the frontend
// can branch on.
func (rs *Resource) login(w http.ResponseWriter, r *http.Request) {
	req := &LoginRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	ipAddress := getClientIP(r)
	userAgent := r.Header.Get(headerUserAgent)

	// Pull the trusted-device cookie if the browser sent one. The MFA gate
	// uses it to skip the second factor for users on a previously-marked
	// device. Empty / missing cookie is fine — the service is nil-tolerant.
	var trustedDeviceCookie string
	if c, err := r.Cookie(trustedDeviceCookieName); err == nil {
		trustedDeviceCookie = c.Value
	}

	result, err := rs.AuthService.LoginWithMFAGate(
		r.Context(), req.Email, req.Password, ipAddress, userAgent, req.TenantSlug, trustedDeviceCookie,
	)
	if err != nil {
		rs.handleLoginError(w, r, err)
		return
	}

	switch result.Status {
	case authService.LoginStatusMFARequired:
		tde := result.TrustedDeviceEnabled
		tdd := result.TrustedDeviceDays
		render.JSON(w, r, LoginResponse{
			Status:               string(authService.LoginStatusMFARequired),
			ChallengeToken:       result.ChallengeToken,
			MaskedEmail:          result.MaskedEmail,
			TrustedDeviceEnabled: &tde,
			TrustedDeviceDays:    &tdd,
		})
		return
	case authService.LoginStatusMFAEnrollmentRequired:
		render.JSON(w, r, LoginResponse{
			Status:                string(authService.LoginStatusMFAEnrollmentRequired),
			AccessToken:           result.AccessToken,
			MaskedEmail:           result.MaskedEmail,
			MFAEnrollmentRequired: true,
		})
		return
	}

	render.JSON(w, r, LoginResponse{
		Status:       string(authService.LoginStatusAuthenticated),
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	})
}

// handleLoginError centralises the error-to-HTTP mapping for /auth/login so
// both the legacy and the MFA-aware paths agree on what comes back.
func (rs *Resource) handleLoginError(w http.ResponseWriter, r *http.Request, err error) {
	var authErr *authService.AuthError
	if errors.As(err, &authErr) {
		switch {
		case errors.Is(authErr.Err, authService.ErrInvalidCredentials):
			common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrInvalidCredentials))
		case errors.Is(authErr.Err, authService.ErrAccountNotFound):
			// Mask the specific error so attackers can't enumerate accounts.
			common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrInvalidCredentials))
		case errors.Is(authErr.Err, authService.ErrAccountInactive):
			common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountInactive))
		case errors.Is(authErr.Err, authService.ErrTenantNotFound):
			common.RenderError(w, r, common.ErrorNotFound(authService.ErrTenantNotFound))
		case errors.Is(authErr.Err, authService.ErrTenantAccessDenied):
			common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrTenantAccessDenied))
		case errors.Is(authErr.Err, authService.ErrParentMustUseParentPortal):
			common.RenderError(w, r, common.ErrorForbidden(authService.ErrParentMustUseParentPortal))
		case errors.Is(authErr.Err, authService.ErrMFARateLimited):
			// MFA challenge initiation tripped the 3/15min sliding-window
			// cap. Surface as 429 so the frontend shows the dedicated "too
			// many code requests" message instead of a generic 5xx.
			common.RenderError(w, r, common.ErrorTooManyRequests(authErr.Err))
		case errors.Is(authErr.Err, authService.ErrMFALocked):
			// Account hit the failed-attempt lockout threshold while we were
			// preparing the next challenge. Same HTTP status as rate limit,
			// distinct message body — handled separately on the frontend.
			common.RenderError(w, r, common.ErrorTooManyRequests(authErr.Err))
		case errors.Is(authErr.Err, authService.ErrMFAStatusUnavailable):
			// MFA gate couldn't determine required/enrolled status (settings
			// or credentials lookup failed with a non-not-found error).
			// Refuse this login rather than fail-open. 503 lets the client
			// retry — the frontend renders it as "Bitte versuche es erneut".
			common.RenderError(w, r, common.ErrorServiceUnavailable(authErr.Err))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}
	common.RenderError(w, r, common.ErrorInternalServer(err))
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
	return validateAccountCredentialFields(req, &req.Email, &req.Username, &req.Password, &req.ConfirmPassword)
}

// validateAccountCredentialFields normalizes and validates the shared
// email/username/password(+confirmation) fields of the account-creation
// payloads (staff registration and parent accounts).
func validateAccountCredentialFields(structPtr any, email, username, password, confirmPassword *string) error {
	*email = strings.TrimSpace(strings.ToLower(*email))
	*username = strings.TrimSpace(*username)

	return validation.ValidateStruct(structPtr,
		validation.Field(email, validation.Required, is.Email),
		validation.Field(username, validation.Required, validation.Length(3, 30)),
		validation.Field(password, validation.Required, validation.Length(8, 0)),
		validation.Field(confirmPassword, validation.Required, validation.By(func(value interface{}) error {
			if *password != *confirmPassword {
				return errors.New(errPasswordsNotMatch)
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
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Authorize role assignment (if role_id specified)
	roleID, callerTenantID, shouldReturn := rs.authorizeRoleAssignment(w, r, req.RoleID)
	if shouldReturn {
		return
	}

	account, err := rs.AuthService.Register(r.Context(), req.Email, req.Username, req.Password, roleID, callerTenantID)
	if err != nil {
		rs.handleRegistrationError(w, r, err)
		return
	}

	resp := buildAccountResponse(account)
	common.Respond(w, r, http.StatusCreated, resp, "Account registered successfully")
}

// LinkToTenantRequest represents a request to link an existing account to the current tenant.
type LinkToTenantRequest struct {
	Email  string `json:"email"`
	RoleID *int64 `json:"role_id,omitempty"`
}

func (req *LinkToTenantRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
	)
}

// linkToTenant links an existing account to the caller's tenant.
// Requires admin authentication with a valid tenant context.
func (rs *Resource) linkToTenant(w http.ResponseWriter, r *http.Request) {
	req := &LinkToTenantRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Require admin auth and resolve role + tenant from JWT
	roleID, callerTenantID, shouldReturn := rs.authorizeRoleAssignment(w, r, req.RoleID)
	if shouldReturn {
		return
	}

	account, err := rs.AuthService.LinkAccountToTenant(r.Context(), req.Email, roleID, callerTenantID)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorNotFound(authErr.Err))
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				common.RenderError(w, r, common.ErrorConflict(authErr.Err))
			default:
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Return ONLY id and email — never leak roles, username, or active status from other tenants
	common.Respond(w, r, http.StatusOK, map[string]any{
		"id":    account.ID,
		"email": account.Email,
	}, "Account linked to tenant successfully")
}

// authorizeRoleAssignment validates the role_id from the request and returns it along with the
// caller's tenant ID. Auth and permission checks are handled by middleware (Authenticator +
// TenantMiddleware + RequiresPermission). Returns the role ID, tenant ID, and whether the
// handler should return early (true = error rendered).
func (rs *Resource) authorizeRoleAssignment(w http.ResponseWriter, r *http.Request, requestedRoleID *int64) (*int64, int64, bool) {
	claims := jwt.ClaimsFromCtx(r.Context())

	if requestedRoleID == nil || *requestedRoleID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("role_id is required when creating accounts")))
		return nil, 0, true
	}

	if claims.TenantID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			authService.ErrTenantRequiredForRoleAssignment))
		return nil, 0, true
	}

	// Verify the role exists — TenantTxMiddleware ensures RLS sees tenant-scoped roles
	if _, err := rs.AuthService.GetRoleByID(r.Context(), int(*requestedRoleID)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			common.RenderError(w, r, common.ErrorInvalidRequest(
				errors.New("specified role does not exist")))
		} else {
			slog.Default().Error("role lookup failed",
				"role_id", *requestedRoleID,
				"error", err,
			)
			common.RenderError(w, r, common.ErrorInternalServerWrap("failed to verify role", err))
		}
		return nil, 0, true
	}

	return requestedRoleID, claims.TenantID, false
}

// handleRegistrationError handles authentication errors during registration
func (rs *Resource) handleRegistrationError(w http.ResponseWriter, r *http.Request, err error) {
	var authErr *authService.AuthError
	if !errors.As(err, &authErr) {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	switch {
	case errors.Is(authErr.Err, authService.ErrEmailAlreadyExists):
		common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrEmailAlreadyExists))
	case errors.Is(authErr.Err, authService.ErrUsernameAlreadyExists):
		common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrUsernameAlreadyExists))
	case errors.Is(authErr.Err, authService.ErrPasswordTooWeak):
		common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrPasswordTooWeak))
	case errors.Is(authErr.Err, authService.ErrTenantRequiredForRoleAssignment):
		common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrTenantRequiredForRoleAssignment))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrInvalidToken))
			case errors.Is(authErr.Err, authService.ErrTokenExpired):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrTokenExpired))
			case errors.Is(authErr.Err, authService.ErrTokenNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrTokenNotFound))
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountNotFound))
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountInactive))
			case errors.Is(authErr.Err, authService.ErrTenantNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrTenantNotFound))
			default:
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
		slog.Default().WarnContext(r.Context(), "Logout audit logging failed (client logout still successful)",
			slog.String("ip", ipAddress),
			slog.String("error", err.Error()),
		)
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
func (req *ChangePasswordRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.CurrentPassword, validation.Required),
		validation.Field(&req.NewPassword, validation.Required, validation.Length(8, 0)),
		validation.Field(&req.ConfirmPassword, validation.Required, validation.By(func(value interface{}) error {
			if req.NewPassword != req.ConfirmPassword {
				return errors.New(errPasswordsNotMatch)
			}
			return nil
		})),
	)
}

// changePassword handles password change
func (rs *Resource) changePassword(w http.ResponseWriter, r *http.Request) {
	req := &ChangePasswordRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
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
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrInvalidCredentials))
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountNotFound))
			case errors.Is(authErr.Err, authService.ErrPasswordTooWeak):
				common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrPasswordTooWeak))
			default:
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
				common.RenderError(w, r, common.ErrorNotFound(authService.ErrAccountNotFound))
				return
			}
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
	Name        string  `json:"name"`
	Description string  `json:"description"`
	BaseRole    *string `json:"base_role,omitempty"`
}

// Bind validates the create role request
func (req *CreateRoleRequest) Bind(_ *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	// Empty/blank base_role trims to nil, treated as "field not sent" (preserve
	// existing value): once set, a base_role cannot be cleared back to NULL via the API.
	req.BaseRole = strutil.TrimPtrToNil(req.BaseRole)

	if err := validateBaseRole(req.BaseRole); err != nil {
		return err
	}

	return validation.ValidateStruct(req,
		validation.Field(&req.Name, validation.Required, validation.Length(1, 100)),
		validation.Field(&req.Description, validation.Length(0, 500)),
	)
}

// UpdateRoleRequest represents the update role request payload
type UpdateRoleRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	BaseRole    *string `json:"base_role,omitempty"`
}

// Bind validates the update role request.
// BaseRole is optional on update — if omitted, the handler preserves the existing value.
func (req *UpdateRoleRequest) Bind(_ *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.BaseRole = strutil.TrimPtrToNil(req.BaseRole)

	// Only validate base_role if the caller explicitly sent one.
	if req.BaseRole != nil {
		if err := validateBaseRoleValue(*req.BaseRole); err != nil {
			return err
		}
	}

	return validation.ValidateStruct(req,
		validation.Field(&req.Name, validation.Required, validation.Length(1, 100)),
		validation.Field(&req.Description, validation.Length(0, 500)),
	)
}

// validateBaseRoleValue checks that a non-nil base_role value is one of the allowed system roles.
func validateBaseRoleValue(value string) error {
	if !slices.Contains(authModel.ValidBaseRoles(), value) {
		return fmt.Errorf("base_role must be one of: %v", authModel.ValidBaseRoles())
	}
	return nil
}

// validateBaseRole checks that base_role is provided and is one of the allowed system roles.
// This runs at the request layer to return 400 (not 500) for invalid payloads.
func validateBaseRole(br *string) error {
	if br == nil {
		return fmt.Errorf("base_role is required; must be one of: %v", authModel.ValidBaseRoles())
	}
	return validateBaseRoleValue(*br)
}

// RoleResponse represents a role response
type RoleResponse struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	IsSystem    bool     `json:"is_system"`
	BaseRole    *string  `json:"base_role,omitempty"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	Permissions []string `json:"permissions,omitempty"`
}

// toRoleResponse maps a domain Role to its API response representation.
func toRoleResponse(role *authModel.Role) *RoleResponse {
	return &RoleResponse{
		ID:          role.ID,
		Name:        role.Name,
		Description: role.Description,
		IsSystem:    role.IsSystem,
		BaseRole:    role.BaseRole,
		CreatedAt:   role.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   role.UpdatedAt.Format(time.RFC3339),
	}
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
func (req *CreatePermissionRequest) Bind(_ *http.Request) error {
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

// UpdatePermissionRequest represents the update permission request payload.
// Field set, JSON tags, and Bind validation are identical to the create
// payload, so it is a type alias.
type UpdatePermissionRequest = CreatePermissionRequest

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

type EnableCaregiverCapabilityRequest struct {
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Position  string `json:"position,omitempty"`
}

// Bind validates the update account request
func (req *UpdateAccountRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Username = strings.TrimSpace(req.Username)

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
		validation.Field(&req.Username, validation.Length(3, 30)),
	)
}

func (req *EnableCaregiverCapabilityRequest) Bind(_ *http.Request) error {
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Position = strings.TrimSpace(req.Position)
	return nil
}

// PasswordResetRequest represents the password reset request payload
type PasswordResetRequest struct {
	Email string `json:"email"`
}

// Bind validates the password reset request
func (req *PasswordResetRequest) Bind(_ *http.Request) error {
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
func (req *PasswordResetConfirmRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Token, validation.Required),
		validation.Field(&req.NewPassword, validation.Required, validation.Length(8, 0)),
		validation.Field(&req.ConfirmPassword, validation.Required, validation.By(func(value interface{}) error {
			if req.NewPassword != req.ConfirmPassword {
				return errors.New(errPasswordsNotMatch)
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
func (req *CreateParentAccountRequest) Bind(_ *http.Request) error {
	return validateAccountCredentialFields(req, &req.Email, &req.Username, &req.Password, &req.ConfirmPassword)
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
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	role, err := rs.AuthService.CreateRole(r.Context(), req.Name, req.Description, req.BaseRole)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, toRoleResponse(role), "Role created successfully")
}

// getRoleByID handles getting a role by ID
func (rs *Resource) getRoleByID(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidRoleID)
	if !ok {
		return
	}

	role, err := rs.AuthService.GetRoleByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("role not found")))
		return
	}

	// Get permissions for the role
	permissions, _ := rs.AuthService.GetRolePermissions(r.Context(), id)
	permissionNames := make([]string, 0, len(permissions))
	for _, perm := range permissions {
		permissionNames = append(permissionNames, perm.GetFullName())
	}

	resp := toRoleResponse(role)
	resp.Permissions = permissionNames

	common.Respond(w, r, http.StatusOK, resp, "Role retrieved successfully")
}

// updateRole handles updating a role
func (rs *Resource) updateRole(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidRoleID)
	if !ok {
		return
	}

	req := &UpdateRoleRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	role, err := rs.AuthService.GetRoleByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("role not found")))
		return
	}

	role.Name = req.Name
	role.Description = req.Description
	// Preserve existing base_role when the caller omits the field.
	// System roles must never carry a base_role — ignore silently.
	if req.BaseRole != nil && !role.IsSystem {
		role.BaseRole = req.BaseRole
	}

	if err := rs.AuthService.UpdateRole(r.Context(), role); err != nil {
		common.RenderError(w, r, renderRoleMutationError(err))
		return
	}

	common.RespondNoContent(w, r)
}

// deleteRole handles deleting a role
func (rs *Resource) deleteRole(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidRoleID)
	if !ok {
		return
	}

	if err := rs.AuthService.DeleteRole(r.Context(), id); err != nil {
		if common.IsConstraintViolation(err) {
			common.RenderError(w, r, common.ErrorConflictMessage("Rolle kann nicht gelöscht werden: Rolle ist aktuell Konten zugewiesen"))
			return
		}
		common.RenderError(w, r, renderRoleMutationError(err))
		return
	}

	common.RespondNoContent(w, r)
}

// renderRoleMutationError maps service-layer role errors to appropriate HTTP responses.
func renderRoleMutationError(err error) render.Renderer {
	var authErr *authService.AuthError
	if errors.As(err, &authErr) {
		if errors.Is(authErr.Err, authService.ErrSystemRoleImmutable) {
			return common.ErrorForbidden(authErr.Err)
		}
		if errors.Is(authErr.Err, authService.ErrRoleNotFound) {
			return common.ErrorNotFound(authErr.Err)
		}
	}
	// FindByID failures (sql.ErrNoRows wrapped in DatabaseError) → 404
	if errors.Is(err, sql.ErrNoRows) {
		return common.ErrorNotFound(errors.New("role not found"))
	}
	return common.ErrorInternalServer(err)
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
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]*RoleResponse, 0, len(roles))
	for _, role := range roles {
		responses = append(responses, toRoleResponse(role))
	}

	common.Respond(w, r, http.StatusOK, responses, "Roles retrieved successfully")
}

// getAccountRoles handles getting roles for an account
func (rs *Resource) getAccountRoles(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	roles, err := rs.AuthService.GetAccountRoles(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]*RoleResponse, 0, len(roles))
	for _, role := range roles {
		responses = append(responses, toRoleResponse(role))
	}

	common.Respond(w, r, http.StatusOK, responses, "Account roles retrieved successfully")
}

func (rs *Resource) getCaregiverCapability(w http.ResponseWriter, r *http.Request) {
	if rs.CaregiverCapabilityService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("caregiver capability service is not configured")))
		return
	}

	accountID, ok := common.ParseInt64IDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	state, err := rs.CaregiverCapabilityService.GetCaregiverCapability(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, caregiverCapabilityErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, state, "Caregiver capability retrieved successfully")
}

func (rs *Resource) enableCaregiverCapability(w http.ResponseWriter, r *http.Request) {
	if rs.CaregiverCapabilityService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("caregiver capability service is not configured")))
		return
	}

	accountID, ok := common.ParseInt64IDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	req := &EnableCaregiverCapabilityRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	state, err := rs.CaregiverCapabilityService.EnableCaregiverCapability(r.Context(), accountID, userModel.EnableCaregiverCapabilityInput{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Position:  req.Position,
	})
	if err != nil {
		common.RenderError(w, r, caregiverCapabilityErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, state, "Caregiver capability enabled successfully")
}

func (rs *Resource) disableCaregiverCapability(w http.ResponseWriter, r *http.Request) {
	if rs.CaregiverCapabilityService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("caregiver capability service is not configured")))
		return
	}

	accountID, ok := common.ParseInt64IDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	state, err := rs.CaregiverCapabilityService.DisableCaregiverCapability(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, caregiverCapabilityErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, state, "Caregiver capability disabled successfully")
}

func caregiverCapabilityErrorRenderer(err error) render.Renderer {
	var blockedErr *usersService.CaregiverCapabilityBlockedError
	var accountTenantErr *usersService.AccountNotAssignedToTenantError
	var usersErr *usersService.UsersError
	var validationErr *usersService.ValidationError

	switch {
	case errors.As(err, &blockedErr):
		return common.NewCaregiverCapabilityBlockedResponse(
			http.StatusConflict,
			blockedErr.Error(),
			blockedErr.Reasons,
		)
	case errors.Is(err, authService.ErrAccountNotFound), errors.As(err, &accountTenantErr):
		return common.ErrorNotFound(errors.New("account not found"))
	case errors.As(err, &usersErr) && errors.As(err, &validationErr):
		return common.ErrorInvalidRequest(validationErr)
	case errors.As(err, &usersErr):
		return common.ErrorInternalServer(usersErr.Err)
	default:
		return common.ErrorInternalServer(err)
	}
}

// Permission Management Endpoints

// createPermission handles creating a new permission
func (rs *Resource) createPermission(w http.ResponseWriter, r *http.Request) {
	req := &CreatePermissionRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	permission, err := rs.AuthService.CreatePermission(r.Context(), req.Name, req.Description, req.Resource, req.Action)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidPermissionID)
	if !ok {
		return
	}

	permission, err := rs.AuthService.GetPermissionByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("permission not found")))
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
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidPermissionID)
	if !ok {
		return
	}

	req := &UpdatePermissionRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	permission, err := rs.AuthService.GetPermissionByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("permission not found")))
		return
	}

	permission.Name = req.Name
	permission.Description = req.Description
	permission.Resource = req.Resource
	permission.Action = req.Action

	if err := rs.AuthService.UpdatePermission(r.Context(), permission); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// deletePermission handles deleting a permission
func (rs *Resource) deletePermission(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidPermissionID)
	if !ok {
		return
	}

	if err := rs.AuthService.DeletePermission(r.Context(), id); err != nil {
		if common.IsConstraintViolation(err) {
			common.RenderError(w, r, common.ErrorConflictMessage("Berechtigung kann nicht gelöscht werden: Berechtigung ist aktuell Rollen oder Konten zugewiesen"))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
		common.RenderError(w, r, common.ErrorInternalServer(err))
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

// getAccountPermissions handles getting permissions for an account
func (rs *Resource) getAccountPermissions(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	permissions, err := rs.AuthService.GetAccountPermissions(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	permissions, err := rs.AuthService.GetAccountDirectPermissions(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
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

// getRolePermissions handles getting permissions for a role
func (rs *Resource) getRolePermissions(w http.ResponseWriter, r *http.Request) {
	roleID, ok := common.ParseIntIDWithError(w, r, "roleId", common.MsgInvalidRoleID)
	if !ok {
		return
	}

	permissions, err := rs.AuthService.GetRolePermissions(r.Context(), roleID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
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

// updateAccount handles updating an account
func (rs *Resource) updateAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	req := &UpdateAccountRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	account, err := rs.AuthService.GetAccountByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("account not found")))
		return
	}

	account.Email = req.Email
	if req.Username != "" {
		username := req.Username
		account.Username = &username
	}

	if err := rs.AuthService.UpdateAccount(r.Context(), account); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
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

			common.RenderError(w, r, common.ErrorTooManyRequests(authService.ErrRateLimitExceeded))
			return
		}

		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	slog.Default().Info("Password reset initiated", slog.String("email", req.Email))

	common.Respond(w, r, http.StatusOK, nil, "If the email exists, a password reset link has been sent")
}

// resetPassword handles resetting password with token
func (rs *Resource) resetPassword(w http.ResponseWriter, r *http.Request) {
	req := &PasswordResetConfirmRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	if err := rs.AuthService.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		slog.Default().WarnContext(r.Context(), "Password reset failed", slog.String("error", err.Error()))

		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrInvalidToken),
				errors.Is(authErr.Err, sql.ErrNoRows):
				// Both cases indicate the token is invalid or not found
				common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid or expired reset token")))
				return
			case errors.Is(authErr.Err, authService.ErrPasswordTooWeak):
				common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrPasswordTooWeak))
				return
			}
		}

		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	slog.Default().Info("Password reset completed successfully")

	common.Respond(w, r, http.StatusOK, nil, "Password reset successfully")
}

// Token Management Endpoints

// cleanupExpiredTokens handles cleanup of expired tokens
func (rs *Resource) cleanupExpiredTokens(w http.ResponseWriter, r *http.Request) {
	count, err := rs.AuthService.CleanupExpiredTokens(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	response := map[string]int{"cleaned_tokens": count}
	common.Respond(w, r, http.StatusOK, response, "Expired tokens cleaned up successfully")
}

// getActiveTokens handles getting active tokens for an account
func (rs *Resource) getActiveTokens(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	tokens, err := rs.AuthService.GetActiveTokens(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	parentAccount, err := rs.AuthService.CreateParentAccount(r.Context(), req.Email, req.Username, req.Password)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrEmailAlreadyExists):
				common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrEmailAlreadyExists))
			case errors.Is(authErr.Err, authService.ErrUsernameAlreadyExists):
				common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrUsernameAlreadyExists))
			default:
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidParentAccountID)
	if !ok {
		return
	}

	parentAccount, err := rs.AuthService.GetParentAccountByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("parent account not found")))
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
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidParentAccountID)
	if !ok {
		return
	}

	req := &UpdateAccountRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	parentAccount, err := rs.AuthService.GetParentAccountByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("parent account not found")))
		return
	}

	parentAccount.Email = req.Email
	if req.Username != "" {
		username := req.Username
		parentAccount.Username = &username
	}

	if err := rs.AuthService.UpdateParentAccount(r.Context(), parentAccount); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
