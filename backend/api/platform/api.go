package platform

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// Viewer is the authenticated account the announcement routes act for.
type Viewer struct {
	AccountID int64
	Roles     []string
	TenantID  int64
	OrgID     int64
}

// Failure is an internal error the composition root renders as a 500.
type Failure struct {
	Message string
	Err     error
}

// Runtime carries the HTTP runtime the composition root binds: the
// authenticated route group, the verified viewer, URL-parameter parsing, and
// the response bodies.
type Runtime struct {
	Protected func(chi.Router, func(chi.Router))
	Viewer    func(*http.Request) Viewer
	// IDParam parses an int64 URL parameter and renders a 400 with errMsg
	// when it is not numeric.
	IDParam func(w http.ResponseWriter, r *http.Request, param, errMsg string) (int64, bool)
	Success func(http.ResponseWriter, *http.Request, int, any, string)
	Failure func(http.ResponseWriter, *http.Request, Failure)
}

// Resource defines the platform API resource (user-facing)
type Resource struct {
	announcementsResource *AnnouncementsResource
	runtime               Runtime
}

// ResourceConfig holds dependencies for the platform resource
type ResourceConfig struct {
	AnnouncementsService communication.Capability
	Runtime              Runtime
}

// NewResource creates a new platform resource
func NewResource(cfg ResourceConfig) *Resource {
	return &Resource{
		announcementsResource: NewAnnouncementsResource(cfg.AnnouncementsService, cfg.Runtime),
		runtime:               cfg.Runtime,
	}
}

// Router returns a configured router for platform endpoints (user-facing)
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// All routes require authentication; the composition root binds the
	// authenticated group including the tenant scope guard (#2207).
	rs.runtime.Protected(r, func(r chi.Router) {
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
