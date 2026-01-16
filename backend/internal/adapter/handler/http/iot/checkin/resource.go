package checkin

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	activeSvc "github.com/moto-nrw/project-phoenix/internal/core/service/active"
	activitiesSvc "github.com/moto-nrw/project-phoenix/internal/core/service/activities"
	educationSvc "github.com/moto-nrw/project-phoenix/internal/core/service/education"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/internal/core/service/facilities"
	iotSvc "github.com/moto-nrw/project-phoenix/internal/core/service/iot"
	usersSvc "github.com/moto-nrw/project-phoenix/internal/core/service/users"
)

// Resource defines the Check-in API resource for student RFID check-in/checkout
type Resource struct {
	IoTService        iotSvc.Service
	UsersService      usersSvc.PersonService
	ActiveService     activeSvc.Service
	FacilityService   facilitiesSvc.Service
	ActivitiesService activitiesSvc.ActivityService
	EducationService  educationSvc.Service
}

// NewResource creates a new Check-in resource
func NewResource(
	iotService iotSvc.Service,
	usersService usersSvc.PersonService,
	activeService activeSvc.Service,
	facilityService facilitiesSvc.Service,
	activitiesService activitiesSvc.ActivityService,
	educationService educationSvc.Service,
) *Resource {
	return &Resource{
		IoTService:        iotService,
		UsersService:      usersService,
		ActiveService:     activeService,
		FacilityService:   facilityService,
		ActivitiesService: activitiesService,
		EducationService:  educationService,
	}
}

// Router returns a configured router for check-in endpoints
// This router handles student RFID check-in/checkout workflow
// All routes require device authentication (API key + Staff PIN)
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Check-in workflow endpoints
	r.Post("/checkin", rs.deviceCheckin)
	r.Post("/ping", rs.devicePing)
	r.Get("/status", rs.deviceStatus)

	return r
}

// =============================================================================
// EXPORTED HANDLERS FOR TESTING
// =============================================================================

// DeviceCheckinHandler returns the deviceCheckin handler for testing.
func (rs *Resource) DeviceCheckinHandler() http.HandlerFunc { return rs.deviceCheckin }

// DevicePingHandler returns the devicePing handler for testing.
func (rs *Resource) DevicePingHandler() http.HandlerFunc { return rs.devicePing }

// DeviceStatusHandler returns the deviceStatus handler for testing.
func (rs *Resource) DeviceStatusHandler() http.HandlerFunc { return rs.deviceStatus }
