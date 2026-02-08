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
	authResource          *AuthResource
	suggestionsResource   *SuggestionsResource
	announcementsResource *AnnouncementsResource
	profileResource       *ProfileResource
	tokenAuth             *jwt.TokenAuth
	authRateLimiter       func(http.Handler) http.Handler
}

// ResourceConfig holds dependencies for the operator resource
type ResourceConfig struct {
	AuthService          platformSvc.OperatorAuthService
	SuggestionsService   platformSvc.OperatorSuggestionsService
	AnnouncementsService platformSvc.AnnouncementService
	TokenAuth            *jwt.TokenAuth
}

// SetAuthRateLimiter sets the rate limiter middleware for operator auth endpoints.
func (rs *Resource) SetAuthRateLimiter(mw func(http.Handler) http.Handler) {
	rs.authRateLimiter = mw
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

	// Public routes (no auth required) — apply auth rate limiter for brute-force protection
	r.Route("/auth", func(r chi.Router) {
		if rs.authRateLimiter != nil {
			r.Use(rs.authRateLimiter)
		}
		r.Post("/login", rs.authResource.Login)
	})

	// Protected routes (require operator auth)
	r.Group(func(r chi.Router) {
		r.Use(rs.tokenAuth.Verifier())
		r.Use(jwt.Authenticator)
		r.Use(RequiresOperatorScope)

		// Suggestions management
		r.Route("/suggestions", func(r chi.Router) {
			r.Get("/", rs.suggestionsResource.ListSuggestions)
			r.Get("/unread-count", rs.suggestionsResource.GetUnreadCount)
			r.Get("/unviewed-count", rs.suggestionsResource.GetUnviewedCount)
			r.Get("/{id}", rs.suggestionsResource.GetSuggestion)
			r.Put("/{id}/status", rs.suggestionsResource.UpdateStatus)
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
