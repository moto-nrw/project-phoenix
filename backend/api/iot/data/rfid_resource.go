package data

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// RFIDResource defines the RFID API resource
type RFIDResource struct {
	UsersService usersSvc.PersonService
}

// NewRFIDResource creates a new RFID resource
func NewRFIDResource(usersService usersSvc.PersonService) *RFIDResource {
	return &RFIDResource{
		UsersService: usersService,
	}
}

// Router returns a configured router for RFID tag management endpoints
// This router is mounted under /iot/staff/ and handles RFID tag assignment/unassignment
// All routes require device authentication (API key + Staff PIN)
func (rs *RFIDResource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Staff RFID tag management endpoints
	r.Post("/{staffId}/rfid", rs.assignStaffRFIDTag)
	r.Delete("/{staffId}/rfid", rs.unassignStaffRFIDTag)

	return r
}
