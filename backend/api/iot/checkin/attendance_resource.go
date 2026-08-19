package checkin

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// AttendanceResource defines the Attendance API resource
type AttendanceResource struct {
	UsersService         usersSvc.PersonService
	ActiveService        activeSvc.Service
	EducationService     educationSvc.Service
	UnregisteredTagScans auditSvc.UnregisteredTagScanService
}

// NewAttendanceResource creates a new Attendance resource
func NewAttendanceResource(usersService usersSvc.PersonService, activeService activeSvc.Service, educationService educationSvc.Service, unregisteredTagScans ...auditSvc.UnregisteredTagScanService) *AttendanceResource {
	var scanService auditSvc.UnregisteredTagScanService
	if len(unregisteredTagScans) > 0 {
		scanService = unregisteredTagScans[0]
	}
	return &AttendanceResource{
		UsersService:         usersService,
		ActiveService:        activeService,
		EducationService:     educationService,
		UnregisteredTagScans: scanService,
	}
}

// Router returns a configured router for attendance tracking endpoints
// This router is mounted under /iot/attendance/ and handles daily attendance status and toggling
// All routes require device authentication (API key + Staff PIN)
func (rs *AttendanceResource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Attendance tracking endpoints
	r.Get("/status/{rfid}", rs.getAttendanceStatus)
	r.Post("/toggle", rs.toggleAttendance)

	return r
}
