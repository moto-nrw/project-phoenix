package data

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// Resource defines the Data API resource for device data queries
type Resource struct {
	runtime        Runtime
	Directory      devicescan.Directory
	TagAssignments devicescan.TagAssignmentQuery
	Rooms          devicescan.RoomAvailability
}

// NewResource creates a new Data resource
func NewResource(directory devicescan.Directory, tags devicescan.TagAssignmentQuery, rooms devicescan.RoomAvailability, runtime Runtime) *Resource {
	if !runtime.valid() {
		panic("IoT data: runtime is required")
	}
	return &Resource{
		runtime:        runtime,
		Directory:      directory,
		TagAssignments: tags,
		Rooms:          rooms,
	}
}

// Router returns a configured router for device data query endpoints
// This router handles queries for students, activities, rooms, and RFID assignments
// All routes require device + PIN authentication
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Device data query endpoints
	r.Get("/students", rs.getTeacherStudents)
	r.Get("/activities", rs.getTeacherActivities)
	r.Get("/rooms/available", rs.getAvailableRoomsForDevice)
	r.Get("/rfid/{tagId}", rs.checkRFIDTagAssignment)

	return r
}

// TeachersRouter returns a router specifically for the teachers endpoint
// This endpoint only requires device-only authentication (no PIN)
func (rs *Resource) TeachersRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	r.Get("/", rs.getAvailableTeachers)

	return r
}
