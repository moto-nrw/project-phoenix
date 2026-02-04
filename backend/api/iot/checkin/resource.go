package checkin

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource defines the Check-in API resource for student RFID check-in/checkout
type Resource struct {
	IoTService        iotSvc.Service
	UsersService      usersSvc.PersonService
	ActiveService     activeSvc.Service
	FacilityService   facilitiesSvc.Service
	ActivitiesService activitiesSvc.ActivityService
	EducationService  educationSvc.Service
	logger            *slog.Logger
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (rs *Resource) getLogger() *slog.Logger {
	if rs.logger != nil {
		return rs.logger
	}
	return slog.Default()
}

// NewResource creates a new Check-in resource
func NewResource(
	iotService iotSvc.Service,
	usersService usersSvc.PersonService,
	activeService activeSvc.Service,
	facilityService facilitiesSvc.Service,
	activitiesService activitiesSvc.ActivityService,
	educationService educationSvc.Service,
	logger *slog.Logger,
) *Resource {
	return &Resource{
		IoTService:        iotService,
		UsersService:      usersService,
		ActiveService:     activeService,
		FacilityService:   facilityService,
		ActivitiesService: activitiesService,
		EducationService:  educationService,
		logger:            logger,
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
