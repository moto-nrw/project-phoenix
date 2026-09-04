package operator

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/realtime"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// Resource defines the operator API resource
type Resource struct {
	authResource            *AuthResource
	passkeyService          platformSvc.OperatorPasskeyService
	mfaResource             *MFAResource
	provisioningResource    *ProvisioningResource
	settingsResource        *SettingsResource
	announcementsResource   *AnnouncementsResource
	profileResource         *ProfileResource
	invitationsResource     *InvitationsResource
	unregisteredTagScans    auditSvc.UnregisteredTagScanService
	tokenAuth               *jwt.TokenAuth
	authRateLimiter         func(http.Handler) http.Handler
	emailConfirmRateLimiter func(http.Handler) http.Handler
	invitationRateLimiter   func(http.Handler) http.Handler
	operatorLookup          OperatorLookup
}

// ResourceConfig holds dependencies for the operator resource
type ResourceConfig struct {
	AppEnv                     string
	AuthService                platformSvc.OperatorAuthService
	PasskeyService             platformSvc.OperatorPasskeyService
	MFAService                 platformSvc.OperatorMFAService
	InvitationService          platformSvc.OperatorInvitationService
	ProvisioningService        platformSvc.OperatorProvisioningService
	CaregiverCapabilityService usersSvc.CaregiverCapabilityService
	AnnouncementsService       communication.Capability
	UnregisteredTagScanService auditSvc.UnregisteredTagScanService
	SettingsService            configSvc.SettingsService
	// Broadcaster is optional. When supplied, the inner SettingsResource emits
	// a tenant_settings_changed SSE event after every successful Set/Reset so
	// open tenant tabs invalidate their settings caches across origins.
	Broadcaster realtime.Broadcaster
	// SchoolRepo lets the SettingsResource emit `school_slug` in set/reset
	// responses so the frontend operator proxy can bust the slug-keyed
	// `tenant-${slug}` cache after tenant-resolve-affecting toggles.
	SchoolService platformSvc.SchoolService
	ActiveService activeSvc.Service
	CareLifecycle usersSvc.CareLifecycleService
	// TenantMFAService is the tenant-side MFA service (auth package).
	// The operator dashboard reuses it to read + write per-account MFA
	// state on behalf of school staff. Distinct from MFAService above,
	// which is the operator's own MFA service (operator login flow).
	TenantMFAService authSvc.MFAService
	TokenAuth        *jwt.TokenAuth
	DB               *bun.DB
}

// SetAuthRateLimiter sets the rate limiter middleware for operator auth endpoints.
func (rs *Resource) SetAuthRateLimiter(mw func(http.Handler) http.Handler) {
	rs.authRateLimiter = mw
}

// SetEmailConfirmRateLimiter sets a dedicated rate limiter for the public
// email-confirm endpoint, isolated from login to prevent cross-endpoint
// rate limit exhaustion.
func (rs *Resource) SetEmailConfirmRateLimiter(mw func(http.Handler) http.Handler) {
	rs.emailConfirmRateLimiter = mw
}

// SetInvitationRateLimiter sets a dedicated rate limiter for the public
// invitation validate/accept endpoints, isolated from email-confirm so that
// repeated validate calls (page refreshes) cannot exhaust the accept budget.
func (rs *Resource) SetInvitationRateLimiter(mw func(http.Handler) http.Handler) {
	rs.invitationRateLimiter = mw
}

// OnSettingValueSet forwards to the internal SettingsResource.OnValueSet hook
// so the caller can register a side-effect callback (e.g. auto-provisioning
// system rooms when checkout toggles flip on). The callback runs in-tx; the
// optional postCommit closure it returns runs only on successful commit so
// non-transactional side effects (file unlinks, external API calls) never
// outlive a rolled-back DB write.
//
// No-op when the settings resource is unconfigured (cfg.SettingsService was nil).
func (rs *Resource) OnSettingValueSet(fn func(ctx context.Context, tenantID int64, key string, value any) (postCommit func(), err error)) {
	if rs.settingsResource == nil {
		return
	}
	rs.settingsResource.OnValueSet(fn)
}

// NewResource creates a new operator resource
func NewResource(cfg ResourceConfig) *Resource {
	tokenAuth := cfg.TokenAuth
	if tokenAuth == nil {
		// Create internal token auth for JWT verification
		tokenAuth = jwt.MustNewTokenAuth()
	}

	resource := &Resource{
		authResource:          NewAuthResource(cfg.AuthService),
		passkeyService:        cfg.PasskeyService,
		mfaResource:           NewMFAResource(cfg.AuthService, cfg.MFAService, tokenAuth),
		provisioningResource:  NewProvisioningResource(cfg.ProvisioningService),
		announcementsResource: NewAnnouncementsResource(cfg.AnnouncementsService),
		profileResource:       NewProfileResource(cfg.AuthService),
		invitationsResource:   NewInvitationsResource(cfg.InvitationService),
		unregisteredTagScans:  cfg.UnregisteredTagScanService,
		tokenAuth:             tokenAuth,
		operatorLookup:        cfg.AuthService,
	}
	resource.provisioningResource.appEnv = cfg.AppEnv
	if cfg.SettingsService != nil {
		resource.settingsResource = NewSettingsResource(cfg.SettingsService, cfg.DB, cfg.Broadcaster, cfg.SchoolService, cfg.ActiveService, cfg.CareLifecycle)
	}
	resource.provisioningResource.CaregiverCapabilityService = cfg.CaregiverCapabilityService
	resource.provisioningResource.db = cfg.DB
	resource.provisioningResource.TenantMFAService = cfg.TenantMFAService
	return resource
}

// Router returns a configured router for operator endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	rs.mountPublicAuthRoutes(r)
	rs.mountRefreshRoute(r)
	rs.mountMFAEnrollmentRoutes(r)
	rs.mountProtectedRoutes(r)

	return r
}

// useRateLimiter applies primary (falling back to fallback) when non-nil.
func useRateLimiter(r chi.Router, primary, fallback func(http.Handler) http.Handler) {
	limiter := primary
	if limiter == nil {
		limiter = fallback
	}
	if limiter != nil {
		r.Use(limiter)
	}
}

// mountPublicAuthRoutes registers the unauthenticated /auth routes.
// Login and email-confirm use separate rate limiter instances so that
// flooding one endpoint cannot exhaust the budget for the other.
func (rs *Resource) mountPublicAuthRoutes(r chi.Router) {
	r.Route("/auth", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			if rs.authRateLimiter != nil {
				r.Use(rs.authRateLimiter)
			}
			r.Post("/login", rs.authResource.Login)

			// MFA challenge → token-pair exchange (issue #1308). Mirror of
			// the tenant-side endpoints — they take the short-lived
			// challenge JWT in the request body, NOT in the Authorization
			// header, because the operator is mid-login and has no access
			// token yet.
			r.Post("/mfa/verify", rs.mfaResource.Verify)
			r.Post("/mfa/resend", rs.mfaResource.Resend)
			r.Post("/passkeys/login/options", rs.PasskeyLoginOptions)
			r.Post("/passkeys/login/verify", rs.PasskeyLoginVerify)
		})
		r.Group(func(r chi.Router) {
			useRateLimiter(r, rs.emailConfirmRateLimiter, rs.authRateLimiter)
			r.Post("/email-confirm", rs.profileResource.ConfirmEmailChange)
		})
		r.Group(func(r chi.Router) {
			useRateLimiter(r, rs.invitationRateLimiter, rs.authRateLimiter)
			r.Post("/invitations/validate", rs.invitationsResource.ValidateInvitation)
			r.Post("/invitations/accept", rs.invitationsResource.AcceptInvitation)
		})
	})
}

// mountRefreshRoute registers the refresh token route (requires valid refresh
// JWT, no scope check).
func (rs *Resource) mountRefreshRoute(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(rs.tokenAuth.Verifier())
		r.Use(jwt.AuthenticateRefreshJWT)
		r.Post("/auth/refresh", rs.authResource.RefreshToken)
	})
}

// mountMFAEnrollmentRoutes registers the enrollment-only routes (issue #1308):
// operator-side mirror of the tenant /auth/mfa/enroll/* group. Accepts the
// narrow enrollment JWT that operator login mints when no MFA credential is on
// file. The enrollment authenticator guarantees mfa_enrollment_pending=true so
// these routes are reachable only from a pre-enrollment session, never from a
// full operator access token.
func (rs *Resource) mountMFAEnrollmentRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(rs.tokenAuth.Verifier())
		r.Use(jwt.MFAEnrollmentAuthenticator)
		r.Post("/auth/mfa/enroll/start", rs.mfaResource.EnrollStart)
		r.Post("/auth/mfa/enroll/confirm", rs.mfaResource.EnrollConfirm)
	})
}

// mountProtectedRoutes registers every route behind the operator auth chain.
func (rs *Resource) mountProtectedRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(rs.tokenAuth.Verifier())
		r.Use(jwt.Authenticator)
		r.Use(common.ReadOnlyPreviewMiddleware)
		r.Use(RequiresOperatorScope)
		r.Use(common.SecurityPrincipalMiddleware)
		r.Use(RequiresActiveOperator(rs.operatorLookup))

		rs.mountPasskeyRoutes(r)
		rs.mountProvisioningRoutes(r)
		rs.mountSchoolRoutes(r)
		rs.mountPersonRoutes(r)
		rs.mountTagScanRoutes(r)
		rs.mountAccountMFARoutes(r)
		rs.mountAccountTenantAccessRoutes(r)
		rs.mountProfileRoutes(r)
		rs.mountTrustedDeviceRoutes(r)
		rs.mountInvitationRoutes(r)
		rs.mountAnnouncementRoutes(r)
	})
}

// mountPasskeyRoutes registers passkey management for the currently-authenticated
// operator.
//
// Keep these as individual leaves instead of r.Route("/auth/passkeys", ...)
// because public passkey login also lives under /auth/passkeys/login/*.
// A protected subtree on /auth/passkeys shadows those public login routes in
// chi and makes anonymous passkey login fail with 401.
//
// Register the list endpoint under BOTH the no-slash and trailing-slash forms:
// chi treats "/auth/passkeys" and "/auth/passkeys/" as distinct patterns (no
// RedirectSlashes on this router), and the old r.Route("/auth/passkeys").Get("/")
// form answered both. The proxy sends the slash form, but direct authenticated
// operator clients hit the no-slash form, so dropping it 404s them.
func (rs *Resource) mountPasskeyRoutes(r chi.Router) {
	r.Get("/auth/passkeys", rs.PasskeyList)
	r.Get("/auth/passkeys/", rs.PasskeyList)
	r.Post("/auth/passkeys/enrollment/challenge", rs.PasskeyEnrollmentChallenge)
	r.Post("/auth/passkeys/register/options", rs.PasskeyRegisterOptions)
	r.Post("/auth/passkeys/register/verify", rs.PasskeyRegisterVerify)
	r.Delete("/auth/passkeys/{passkeyId}", rs.PasskeyRevoke)
}

// mountProvisioningRoutes registers account/role/stat/device/organization
// provisioning endpoints.
func (rs *Resource) mountProvisioningRoutes(r chi.Router) {
	r.Get("/accounts", rs.provisioningResource.ListAllAccounts)
	r.Get("/roles", rs.provisioningResource.ListSystemRoles)
	r.Get("/stats", rs.provisioningResource.GetProvisioningStats)
	r.Route("/devices", func(r chi.Router) {
		r.Get("/", rs.provisioningResource.ListAllDevices)
		r.Post("/", rs.provisioningResource.CreateDevice)
		r.Post("/{id}/set-api-key", rs.provisioningResource.SetDeviceAPIKey)
		r.Get("/{id}/transfer-status", rs.provisioningResource.GetDeviceTransferStatus)
		r.Post("/{id}/transfer", rs.provisioningResource.TransferDevice)
		r.Delete("/{id}", rs.provisioningResource.DeleteDevice)
	})

	r.Route("/organizations", func(r chi.Router) {
		r.Get("/", rs.provisioningResource.ListOrganizations)
		r.Get("/summaries", rs.provisioningResource.ListOrganizationSummaries)
		r.Post("/", rs.provisioningResource.CreateOrganization)
		r.Put("/{id}", rs.provisioningResource.UpdateOrganization)
		r.Delete("/{id}", rs.provisioningResource.SoftDeleteOrganization)
		r.Post("/{id}/restore", rs.provisioningResource.RestoreOrganization)
		r.Get("/{id}/accounts", rs.provisioningResource.ListOrganizationAccounts)
		r.Get("/{id}/devices", rs.provisioningResource.ListOrganizationDevices)
		r.Get("/{id}/schools", rs.provisioningResource.ListOrganizationSchoolSummaries)
		r.Get("/{id}/persons", rs.provisioningResource.ListOrganizationPersons)
	})
}

// mountSchoolRoutes registers the /schools subtree, including the optional
// per-school settings endpoints.
func (rs *Resource) mountSchoolRoutes(r chi.Router) {
	r.Route("/schools", func(r chi.Router) {
		r.Get("/", rs.provisioningResource.ListSchools)
		r.Get("/summaries", rs.provisioningResource.ListSchoolSummaries)
		r.Post("/", rs.provisioningResource.CreateSchool)
		r.Put("/{id}", rs.provisioningResource.UpdateSchool)
		r.Delete("/{id}", rs.provisioningResource.SoftDeleteSchool)
		r.Post("/{id}/restore", rs.provisioningResource.RestoreSchool)
		r.Post("/{id}/invite-admin", rs.provisioningResource.InviteSchoolAdmin)
		r.Post("/{id}/create-account", rs.provisioningResource.CreateSchoolAccount)
		r.Get("/{id}/accounts", rs.provisioningResource.ListSchoolAccounts)
		r.Route("/{id}/accounts/{accountId}/caregiver-capability", func(r chi.Router) {
			r.Get("/", rs.provisioningResource.GetSchoolAccountCaregiverCapability)
			r.Post("/", rs.provisioningResource.EnableSchoolAccountCaregiverCapability)
			r.Delete("/", rs.provisioningResource.DisableSchoolAccountCaregiverCapability)
		})
		// MFA admin actions for school staff. Operator-side mirror of the
		// tenant-admin MFA endpoints — same write semantics, separate
		// audit metadata (actor_type=operator).
		r.Route("/{id}/accounts/{accountId}/mfa", func(r chi.Router) {
			r.Get("/", rs.provisioningResource.GetSchoolAccountMFAState)
			r.Delete("/", rs.provisioningResource.ResetSchoolAccountMFA)
			r.Put("/override", rs.provisioningResource.SetSchoolAccountMFAOverride)
		})
		r.Get("/{id}/devices", rs.provisioningResource.ListSchoolDevices)
		r.Get("/{id}/persons", rs.provisioningResource.ListSchoolPersons)
		r.Get("/{id}/pwa-usage", rs.provisioningResource.GetSchoolPWAUsage)
		if rs.settingsResource != nil {
			r.Route("/{id}/settings", func(r chi.Router) {
				r.Get("/schema", rs.settingsResource.GetSchoolSettingsSchema)
				r.Get("/booking-authority-impact", rs.settingsResource.GetBookingAuthorityImpact)
				r.Get("/values/{key}/reveal", rs.settingsResource.RevealSchoolSettingValue)
				r.Put("/values/{key}", rs.settingsResource.SetSchoolSettingValue)
				r.Delete("/values/{key}", rs.settingsResource.ResetSchoolSettingValue)
			})
		}
	})
}

// mountPersonRoutes registers platform-level person management.
func (rs *Resource) mountPersonRoutes(r chi.Router) {
	r.Route("/persons", func(r chi.Router) {
		r.Delete("/{id}", rs.provisioningResource.SoftDeletePerson)
	})
}

// mountTagScanRoutes registers the optional unregistered-tag-scan surface.
func (rs *Resource) mountTagScanRoutes(r chi.Router) {
	if rs.unregisteredTagScans == nil {
		return
	}
	r.Route("/unregistered-tag-scans", func(r chi.Router) {
		r.Get("/", rs.ListUnregisteredTagScans)
		r.Post("/{id}/resolve", rs.ResolveUnregisteredTagScan)
	})
}

// mountAccountMFARoutes registers the account-wide MFA override surface
// ("mailbox lockout emergency switch"). Deliberately decoupled from
// /schools/{id}/accounts/{} because the platform-wide override row applies
// regardless of which school an account belongs to — see #1430 review round 2.
func (rs *Resource) mountAccountMFARoutes(r chi.Router) {
	r.Route("/accounts/{accountId}/mfa", func(r chi.Router) {
		r.Get("/global-override", rs.provisioningResource.GetAccountMFAGlobalOverride)
		r.Put("/global-override", rs.provisioningResource.SetAccountMFAGlobalOverride)
	})
}

// mountAccountTenantAccessRoutes registers cross-school access management for a
// single account (issue #1021). Keyed by account, not by school, because the
// whole point is to see and change every school one account can reach.
func (rs *Resource) mountAccountTenantAccessRoutes(r chi.Router) {
	// Chi treats collection paths with and without a trailing slash as distinct
	// routes. The operator client deliberately uses the canonical no-slash form.
	r.Get("/accounts/{accountId}/tenants", rs.provisioningResource.ListAccountTenantAccess)
	r.Get("/accounts/{accountId}/tenants/", rs.provisioningResource.ListAccountTenantAccess)
	r.Post("/accounts/{accountId}/tenants", rs.provisioningResource.GrantAccountTenantAccess)
	r.Post("/accounts/{accountId}/tenants/", rs.provisioningResource.GrantAccountTenantAccess)
	r.Get("/accounts/{accountId}/tenants/{tenantId}/roles", rs.provisioningResource.ListAssignableSchoolRoles)
	r.Put("/accounts/{accountId}/tenants/{tenantId}", rs.provisioningResource.UpdateAccountTenantRole)
	r.Delete("/accounts/{accountId}/tenants/{tenantId}", rs.provisioningResource.RevokeAccountTenantAccess)
}

// mountProfileRoutes registers operator profile management.
func (rs *Resource) mountProfileRoutes(r chi.Router) {
	r.Route("/profile", func(r chi.Router) {
		r.Get("/", rs.profileResource.GetProfile)
		r.Put("/", rs.profileResource.UpdateProfile)
		r.Post("/password", rs.profileResource.ChangePassword)
		r.Post("/email-change", rs.profileResource.InitiateEmailChange)
	})
}

// mountTrustedDeviceRoutes registers self-service trusted-device management.
// MFA enrollment lives in its own group (uses the dedicated
// MFAEnrollmentAuthenticator); ownership here is enforced in the service.
func (rs *Resource) mountTrustedDeviceRoutes(r chi.Router) {
	r.Get("/auth/mfa/trusted-devices", rs.mfaResource.ListTrustedDevices)
	r.Delete("/auth/mfa/trusted-devices/{deviceId}", rs.mfaResource.RevokeTrustedDevice)
}

// mountInvitationRoutes registers operator invitation management.
func (rs *Resource) mountInvitationRoutes(r chi.Router) {
	r.Route("/invitations", func(r chi.Router) {
		r.Post("/", rs.invitationsResource.CreateInvitation)
		r.Get("/", rs.invitationsResource.ListInvitations)
		r.Post("/{id}/resend", rs.invitationsResource.ResendInvitation)
		r.Delete("/{id}", rs.invitationsResource.RevokeInvitation)
	})
}

// mountAnnouncementRoutes registers announcements management.
func (rs *Resource) mountAnnouncementRoutes(r chi.Router) {
	r.Route("/announcements", func(r chi.Router) {
		r.Get("/", rs.announcementsResource.ListAnnouncements)
		r.Post("/", rs.announcementsResource.CreateAnnouncement)
		r.Get("/{id}", rs.announcementsResource.GetAnnouncement)
		r.Put("/{id}", rs.announcementsResource.UpdateAnnouncement)
		r.Delete("/{id}", rs.announcementsResource.DeleteAnnouncement)
		r.Post("/{id}/publish", rs.announcementsResource.PublishAnnouncement)
		r.Get("/{id}/stats", rs.announcementsResource.GetStats)
		r.Get("/{id}/views", rs.announcementsResource.GetViewDetails)
	})
}
