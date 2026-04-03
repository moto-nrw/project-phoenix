package operator

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// Resource defines the operator API resource
type Resource struct {
	authResource            *AuthResource
	operatorsResource       *OperatorsResource
	provisioningResource    *ProvisioningResource
	suggestionsResource     *SuggestionsResource
	announcementsResource   *AnnouncementsResource
	profileResource         *ProfileResource
	tokenAuth               *jwt.TokenAuth
	authRateLimiter         func(http.Handler) http.Handler
	emailConfirmRateLimiter func(http.Handler) http.Handler
}

// ResourceConfig holds dependencies for the operator resource
type ResourceConfig struct {
	AuthService          platformSvc.OperatorAuthService
	InvitationService    platformSvc.OperatorInvitationService
	ProvisioningService  platformSvc.OperatorProvisioningService
	SuggestionsService   platformSvc.OperatorSuggestionsService
	AnnouncementsService platformSvc.AnnouncementService
	TokenAuth            *jwt.TokenAuth
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

// NewResource creates a new operator resource
func NewResource(cfg ResourceConfig) *Resource {
	tokenAuth := cfg.TokenAuth
	if tokenAuth == nil {
		// Create internal token auth for JWT verification
		tokenAuth, _ = jwt.NewTokenAuth()
	}

	return &Resource{
		authResource:          NewAuthResource(cfg.AuthService),
		operatorsResource:     NewOperatorsResource(cfg.AuthService, cfg.InvitationService),
		provisioningResource:  NewProvisioningResource(cfg.ProvisioningService),
		suggestionsResource:   NewSuggestionsResource(cfg.SuggestionsService),
		announcementsResource: NewAnnouncementsResource(cfg.AnnouncementsService),
		profileResource:       NewProfileResource(cfg.AuthService),
		tokenAuth:             tokenAuth,
	}
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
		// Operator invitation public endpoints (rate-limited with email-confirm limiter)
		r.Group(func(r chi.Router) {
			limiter := rs.emailConfirmRateLimiter
			if limiter == nil {
				limiter = rs.authRateLimiter
			}
			if limiter != nil {
				r.Use(limiter)
			}
			r.Post("/invite-validate", rs.operatorsResource.ValidateInvitation)
			r.Post("/invite-accept", rs.operatorsResource.AcceptInvitation)
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

		// Operator management
		r.Get("/operators", rs.operatorsResource.ListOperators)
		r.Route("/invitations", func(r chi.Router) {
			r.Get("/", rs.operatorsResource.ListPendingInvitations)
			r.Post("/", rs.operatorsResource.CreateInvitation)
			r.Post("/{id}/resend", rs.operatorsResource.ResendInvitation)
			r.Delete("/{id}", rs.operatorsResource.RevokeInvitation)
		})

		r.Get("/accounts", rs.provisioningResource.ListAllAccounts)
		r.Get("/roles", rs.provisioningResource.ListSystemRoles)
		r.Route("/devices", func(r chi.Router) {
			r.Get("/", rs.provisioningResource.ListAllDevices)
			r.Post("/", rs.provisioningResource.CreateDevice)
			r.Post("/{id}/set-api-key", rs.provisioningResource.SetDeviceAPIKey)
			r.Delete("/{id}", rs.provisioningResource.DeleteDevice)
		})

		r.Route("/organizations", func(r chi.Router) {
			r.Get("/", rs.provisioningResource.ListOrganizations)
			r.Post("/", rs.provisioningResource.CreateOrganization)
			r.Put("/{id}", rs.provisioningResource.UpdateOrganization)
			r.Get("/{id}/accounts", rs.provisioningResource.ListOrganizationAccounts)
			r.Get("/{id}/devices", rs.provisioningResource.ListOrganizationDevices)
		})

		r.Route("/schools", func(r chi.Router) {
			r.Get("/", rs.provisioningResource.ListSchools)
			r.Post("/", rs.provisioningResource.CreateSchool)
			r.Put("/{id}", rs.provisioningResource.UpdateSchool)
			r.Delete("/{id}", rs.provisioningResource.SoftDeleteSchool)
			r.Post("/{id}/restore", rs.provisioningResource.RestoreSchool)
			r.Post("/{id}/invite-admin", rs.provisioningResource.InviteSchoolAdmin)
			r.Post("/{id}/create-account", rs.provisioningResource.CreateSchoolAccount)
			r.Get("/{id}/accounts", rs.provisioningResource.ListSchoolAccounts)
			r.Get("/{id}/devices", rs.provisioningResource.ListSchoolDevices)
			r.Get("/{id}/persons", rs.provisioningResource.ListSchoolPersons)
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
