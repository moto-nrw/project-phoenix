package rfid

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	usersSvc "github.com/moto-nrw/project-phoenix/internal/core/service/users"
)

// Resource defines the RFID API resource
type Resource struct {
	UsersService usersSvc.PersonService
}

// NewResource creates a new RFID resource
func NewResource(usersService usersSvc.PersonService) *Resource {
	return &Resource{
		UsersService: usersService,
	}
}

// Router returns a configured router for RFID tag management endpoints
// This router is mounted under /iot/staff/ and handles RFID tag assignment/unassignment
// All routes require device authentication (API key + Staff PIN)
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Staff RFID tag management endpoints
	r.Post("/{staffId}/rfid", rs.assignStaffRFIDTag)
	r.Delete("/{staffId}/rfid", rs.unassignStaffRFIDTag)

	return r
}

// =============================================================================
// HANDLER ACCESSOR METHODS (for testing)
// =============================================================================

// AssignRFIDTagHandler returns the assignStaffRFIDTag handler
func (rs *Resource) AssignRFIDTagHandler() http.HandlerFunc { return rs.assignStaffRFIDTag }

// UnassignRFIDTagHandler returns the unassignStaffRFIDTag handler
func (rs *Resource) UnassignRFIDTagHandler() http.HandlerFunc { return rs.unassignStaffRFIDTag }
