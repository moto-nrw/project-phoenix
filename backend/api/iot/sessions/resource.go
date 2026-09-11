package sessions

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend"
)

// Resource is the HTTP adapter for manual kiosk session controls.
type Resource struct {
	Lifecycle  devicescan.SessionLifecycle
	SessionEnd sessionend.Command
	runtime    Runtime
}

func NewResource(lifecycle devicescan.SessionLifecycle, end sessionend.Command, runtime Runtime) *Resource {
	if runtime.ParseID == nil || runtime.Authenticated == nil || runtime.Success == nil || runtime.Failure == nil || runtime.MarkRollback == nil {
		panic("IoT sessions: runtime is required")
	}
	return &Resource{Lifecycle: lifecycle, SessionEnd: end, runtime: runtime}
}

// Router returns a configured router for session management endpoints
// This router is mounted under /iot/session/ and handles activity session lifecycle
// All routes require device authentication (API key + Staff PIN)
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Session CRUD operations
	r.Post("/start", rs.startActivitySession)
	r.Post("/end", rs.endActivitySession)
	r.Get("/current", rs.getCurrentSession)
	r.Post("/check-conflict", rs.checkSessionConflict)
	r.Put("/{sessionId}/supervisors", rs.updateSessionSupervisors)

	// Session timeout management
	r.Post("/timeout", rs.processSessionTimeout)
	r.Post("/activity", rs.updateSessionActivity)
	r.Post("/validate-timeout", rs.validateSessionTimeout)
	r.Get("/timeout-info", rs.getSessionTimeoutInfo)

	return r
}
