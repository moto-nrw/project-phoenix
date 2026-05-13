package operator

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/realtime"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// Resource defines the operator API resource
type Resource struct {
	authResource            *AuthResource
	mfaResource             *MFAResource
	provisioningResource    *ProvisioningResource
	settingsResource        *SettingsResource
	suggestionsResource     *SuggestionsResource
	announcementsResource   *AnnouncementsResource
	profileResource         *ProfileResource
	invitationsResource     *InvitationsResource
	tokenAuth               *jwt.TokenAuth
	authRateLimiter         func(http.Handler) http.Handler
	emailConfirmRateLimiter func(http.Handler) http.Handler
	invitationRateLimiter   func(http.Handler) http.Handler
}

// ResourceConfig holds dependencies for the operator resource
type ResourceConfig struct {
	AuthService                platformSvc.OperatorAuthService
	MFAService                 platformSvc.OperatorMFAService
	InvitationService          platformSvc.OperatorInvitationService
	ProvisioningService        platformSvc.OperatorProvisioningService
	CaregiverCapabilityService usersSvc.CaregiverCapabilityService
	SuggestionsService         platformSvc.OperatorSuggestionsService
	AnnouncementsService       platformSvc.AnnouncementService
	SettingsService            configSvc.SettingsService
	// Broadcaster is optional. When supplied, the inner SettingsResource emits
	// a tenant_settings_changed SSE event after every successful Set/Reset so
	// open tenant tabs invalidate their settings caches across origins.
	Broadcaster realtime.Broadcaster
	// SchoolRepo lets the SettingsResource emit `school_slug` in set/reset
	// responses so the frontend operator proxy can bust the slug-keyed
	// `tenant-${slug}` cache after tenant-resolve-affecting toggles.
	SchoolRepo platformModels.SchoolRepository
	TokenAuth  *jwt.TokenAuth
	DB         *bun.DB
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
		mfaResource:           NewMFAResource(cfg.AuthService, cfg.MFAService, tokenAuth),
		provisioningResource:  NewProvisioningResource(cfg.ProvisioningService),
		suggestionsResource:   NewSuggestionsResource(cfg.SuggestionsService),
		announcementsResource: NewAnnouncementsResource(cfg.AnnouncementsService),
		profileResource:       NewProfileResource(cfg.AuthService),
		invitationsResource:   NewInvitationsResource(cfg.InvitationService),
		tokenAuth:             tokenAuth,
	}
	if cfg.SettingsService != nil {
		resource.settingsResource = NewSettingsResource(cfg.SettingsService, cfg.DB, cfg.Broadcaster, cfg.SchoolRepo)
	}
	resource.provisioningResource.CaregiverCapabilityService = cfg.CaregiverCapabilityService
	resource.provisioningResource.db = cfg.DB
	return resource
}

// Router returns a configured router for operator endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Public routes (no auth required) — rate-limited for brute-force protection.
	// Login and email-confirm use separate rate limiter instances so that
	// flooding one endpoint cannot exhaust the budget for the other.
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
		})
		r.Group(func(r chi.Router) {
			limiter := rs.emailConfirmRateLimiter
			if limiter == nil {
				limiter = rs.authRateLimiter
			}
			if limiter != nil {
				r.Use(limiter)
			}
			r.Post("/email-confirm", rs.profileResource.ConfirmEmailChange)
		})
		r.Group(func(r chi.Router) {
			limiter := rs.invitationRateLimiter
			if limiter == nil {
				limiter = rs.authRateLimiter
			}
			if limiter != nil {
				r.Use(limiter)
			}
			r.Post("/invitations/validate", rs.invitationsResource.ValidateInvitation)
			r.Post("/invitations/accept", rs.invitationsResource.AcceptInvitation)
		})
	})

	// Refresh token route (requires valid refresh JWT, no scope check)
	r.Group(func(r chi.Router) {
		r.Use(rs.tokenAuth.Verifier())
		r.Use(jwt.AuthenticateRefreshJWT)
		r.Post("/auth/refresh", rs.authResource.RefreshToken)
	})

	// Protected routes (require operator auth)
	r.Group(func(r chi.Router) {
		r.Use(rs.tokenAuth.Verifier())
		r.Use(jwt.Authenticator)
		r.Use(RequiresOperatorScope)

		r.Get("/accounts", rs.provisioningResource.ListAllAccounts)
		r.Get("/roles", rs.provisioningResource.ListSystemRoles)
		r.Get("/stats", rs.provisioningResource.GetProvisioningStats)
		r.Route("/devices", func(r chi.Router) {
			r.Get("/", rs.provisioningResource.ListAllDevices)
			r.Post("/", rs.provisioningResource.CreateDevice)
			r.Post("/{id}/set-api-key", rs.provisioningResource.SetDeviceAPIKey)
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
			r.Get("/{id}/devices", rs.provisioningResource.ListSchoolDevices)
			r.Get("/{id}/persons", rs.provisioningResource.ListSchoolPersons)
			if rs.settingsResource != nil {
				r.Route("/{id}/settings", func(r chi.Router) {
					r.Get("/schema", rs.settingsResource.GetSchoolSettingsSchema)
					r.Get("/values/{key}/reveal", rs.settingsResource.RevealSchoolSettingValue)
					r.Put("/values/{key}", rs.settingsResource.SetSchoolSettingValue)
					r.Delete("/values/{key}", rs.settingsResource.ResetSchoolSettingValue)
				})
			}
		})

		r.Route("/persons", func(r chi.Router) {
			r.Delete("/{id}", rs.provisioningResource.SoftDeletePerson)
		})

		// Suggestions management
		r.Route("/suggestions", func(r chi.Router) {
			r.Get("/", rs.suggestionsResource.ListSuggestions)
			r.Get("/unread-count", rs.suggestionsResource.GetUnreadCount)
			r.Get("/unviewed-count", rs.suggestionsResource.GetUnviewedCount)
			r.Get("/{id}", rs.suggestionsResource.GetSuggestion)
			r.Put("/{id}/status", rs.suggestionsResource.UpdateStatus)
			r.Put("/{id}/hidden", rs.suggestionsResource.HidePost)
			r.Delete("/{id}", rs.suggestionsResource.DeletePost)
			r.Post("/{id}/view", rs.suggestionsResource.MarkPostViewed)
			r.Post("/{id}/comments", rs.suggestionsResource.AddComment)
			r.Post("/{id}/comments/read", rs.suggestionsResource.MarkCommentsRead)
			r.Delete("/{id}/comments/{commentId}", rs.suggestionsResource.DeleteComment)
		})

		// Profile management
		r.Route("/profile", func(r chi.Router) {
			r.Get("/", rs.profileResource.GetProfile)
			r.Put("/", rs.profileResource.UpdateProfile)
			r.Post("/password", rs.profileResource.ChangePassword)
			r.Post("/email-change", rs.profileResource.InitiateEmailChange)
		})

		// MFA enrollment for the currently-authenticated operator
		// (issue #1308). Routes are registered as individual leaves rather
		// than via r.Route("/auth/mfa", ...) because the public sibling
		// routes (mfa/verify, mfa/resend) already own the /auth/mfa/*
		// subtree from a different group, and mounting a second sub-router
		// on that prefix here shadows them.
		r.Post("/auth/mfa/enroll/start", rs.mfaResource.EnrollStart)
		r.Post("/auth/mfa/enroll/confirm", rs.mfaResource.EnrollConfirm)

		// Operator invitations
		r.Route("/invitations", func(r chi.Router) {
			r.Post("/", rs.invitationsResource.CreateInvitation)
			r.Get("/", rs.invitationsResource.ListInvitations)
			r.Post("/{id}/resend", rs.invitationsResource.ResendInvitation)
			r.Delete("/{id}", rs.invitationsResource.RevokeInvitation)
		})

		// Announcements management
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
	})

	return r
}
