package platform

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// Resource defines the platform API resource (user-facing)
type Resource struct {
	announcementsResource *AnnouncementsResource
	tokenAuth             *jwt.TokenAuth
}

// ResourceConfig holds dependencies for the platform resource
type ResourceConfig struct {
	AnnouncementsService platformSvc.AnnouncementService
	TokenAuth            *jwt.TokenAuth
}

// NewResource creates a new platform resource
func NewResource(cfg ResourceConfig) *Resource {
	tokenAuth := cfg.TokenAuth
	if tokenAuth == nil {
		// Create internal token auth for JWT verification
		tokenAuth, _ = jwt.NewTokenAuth()
	}

	return &Resource{
		announcementsResource: NewAnnouncementsResource(cfg.AnnouncementsService),
		tokenAuth:             tokenAuth,
	}
}

// Router returns a configured router for platform endpoints (user-facing)
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// All routes require authentication
	r.Group(func(r chi.Router) {
		r.Use(rs.tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Announcements for users
		r.Route("/announcements", func(r chi.Router) {
			r.Get("/unread", rs.announcementsResource.GetUnread)
			r.Get("/unread/count", rs.announcementsResource.GetUnreadCount)
			r.Post("/{id}/seen", rs.announcementsResource.MarkSeen)
			r.Post("/{id}/dismiss", rs.announcementsResource.MarkDismissed)
		})
	})

	return r
}
