package data

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// RFIDResource defines the RFID API resource
type RFIDResource struct {
	runtime Runtime
	Tags    devicescan.StaffTagAssignments
}

// NewRFIDResource creates a new RFID resource
func NewRFIDResource(tags devicescan.StaffTagAssignments, runtime Runtime) *RFIDResource {
	if !runtime.valid() {
		panic("IoT RFID: runtime is required")
	}
	return &RFIDResource{
		runtime: runtime,
		Tags:    tags,
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
