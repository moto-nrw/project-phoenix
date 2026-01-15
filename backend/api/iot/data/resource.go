package data

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource defines the Data API resource for device data queries
type Resource struct {
	IoTService        iotSvc.Service
	UsersService      usersSvc.PersonService
	ActivitiesService activitiesSvc.ActivityService
	FacilityService   facilitiesSvc.Service
}

// NewResource creates a new Data resource
func NewResource(iotService iotSvc.Service, usersService usersSvc.PersonService, activitiesService activitiesSvc.ActivityService, facilityService facilitiesSvc.Service) *Resource {
	return &Resource{
		IoTService:        iotService,
		UsersService:      usersService,
		ActivitiesService: activitiesService,
		FacilityService:   facilityService,
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

// =============================================================================
// HANDLER ACCESSOR METHODS (for testing)
// =============================================================================

// GetAvailableTeachersHandler returns the getAvailableTeachers handler
func (rs *Resource) GetAvailableTeachersHandler() http.HandlerFunc { return rs.getAvailableTeachers }

// GetTeacherStudentsHandler returns the getTeacherStudents handler
func (rs *Resource) GetTeacherStudentsHandler() http.HandlerFunc { return rs.getTeacherStudents }

// GetTeacherActivitiesHandler returns the getTeacherActivities handler
func (rs *Resource) GetTeacherActivitiesHandler() http.HandlerFunc { return rs.getTeacherActivities }

// GetAvailableRoomsHandler returns the getAvailableRoomsForDevice handler
func (rs *Resource) GetAvailableRoomsHandler() http.HandlerFunc { return rs.getAvailableRoomsForDevice }

// CheckRFIDTagAssignmentHandler returns the checkRFIDTagAssignment handler
func (rs *Resource) CheckRFIDTagAssignmentHandler() http.HandlerFunc {
	return rs.checkRFIDTagAssignment
}
