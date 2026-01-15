package attendance

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	activeSvc "github.com/moto-nrw/project-phoenix/internal/core/service/active"
	educationSvc "github.com/moto-nrw/project-phoenix/internal/core/service/education"
	usersSvc "github.com/moto-nrw/project-phoenix/internal/core/service/users"
)

// Resource defines the Attendance API resource
type Resource struct {
	UsersService     usersSvc.PersonService
	ActiveService    activeSvc.Service
	EducationService educationSvc.Service
}

// NewResource creates a new Attendance resource
func NewResource(usersService usersSvc.PersonService, activeService activeSvc.Service, educationService educationSvc.Service) *Resource {
	return &Resource{
		UsersService:     usersService,
		ActiveService:    activeService,
		EducationService: educationService,
	}
}

// Router returns a configured router for attendance tracking endpoints
// This router is mounted under /iot/attendance/ and handles daily attendance status and toggling
// All routes require device authentication (API key + Staff PIN)
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Attendance tracking endpoints
	r.Get("/status/{rfid}", rs.getAttendanceStatus)
	r.Post("/toggle", rs.toggleAttendance)

	return r
}
