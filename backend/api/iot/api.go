package iot

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/iot/attendance"
	checkinAPI "github.com/moto-nrw/project-phoenix/api/iot/checkin"
	dataAPI "github.com/moto-nrw/project-phoenix/api/iot/data"
	"github.com/moto-nrw/project-phoenix/api/iot/devices"
	feedbackAPI "github.com/moto-nrw/project-phoenix/api/iot/feedback"
	rfidAPI "github.com/moto-nrw/project-phoenix/api/iot/rfid"
	sessionsAPI "github.com/moto-nrw/project-phoenix/api/iot/sessions"
	"github.com/moto-nrw/project-phoenix/auth/device"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
	feedbackSvc "github.com/moto-nrw/project-phoenix/services/feedback"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// delegateHandler creates an http.HandlerFunc that delegates to a subrouter.
// This avoids Chi's "Mount() on existing path" error while keeping routes organized.
func delegateHandler(router chi.Router) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		router.ServeHTTP(w, req)
	}
}

// ServiceDependencies groups all service dependencies for the IoT resource
type ServiceDependencies struct {
	IoTService        iotSvc.Service
	UsersService      usersSvc.PersonService
	ActiveService     activeSvc.Service
	ActivitiesService activitiesSvc.ActivityService
	ConfigService     configSvc.Service
	FacilityService   facilitiesSvc.Service
	EducationService  educationSvc.Service
	FeedbackService   feedbackSvc.Service
}

// Resource defines the IoT API resource
type Resource struct {
	IoTService        iotSvc.Service
	UsersService      usersSvc.PersonService
	ActiveService     activeSvc.Service
	ActivitiesService activitiesSvc.ActivityService
	ConfigService     configSvc.Service
	FacilityService   facilitiesSvc.Service
	EducationService  educationSvc.Service
	FeedbackService   feedbackSvc.Service
}

// NewResource creates a new IoT resource
func NewResource(deps ServiceDependencies) *Resource {
	return &Resource{
		IoTService:        deps.IoTService,
		UsersService:      deps.UsersService,
		ActiveService:     deps.ActiveService,
		ActivitiesService: deps.ActivitiesService,
		ConfigService:     deps.ConfigService,
		FacilityService:   deps.FacilityService,
		EducationService:  deps.EducationService,
		FeedbackService:   deps.FeedbackService,
	}
}

// Router returns a configured router for IoT endpoints (DEPRECATED - use DeviceRouter and AdminRouter)
func (rs *Resource) Router() chi.Router {
	return rs.DeviceRouter()
}

// AdminRouter returns a router for IoT admin endpoints (device management).
// These routes require BetterAuth authentication via tenant middleware.
// Mounted at /api/iot/devices by base.go.
func (rs *Resource) AdminRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Device CRUD operations - requires BetterAuth + iot:* permissions
	// Tenant middleware is applied by base.go before these routes
	devicesResource := devices.NewResource(rs.IoTService)
	r.Mount("/", devicesResource.Router())

	return r
}

// DeviceRouter returns a router for IoT device endpoints.
// These routes use device authentication (API key + optional PIN).
// Mounted at /api/iot by base.go.
func (rs *Resource) DeviceRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Device-only authenticated routes (API key only, no PIN required)
	r.Group(func(r chi.Router) {
		r.Use(device.DeviceOnlyAuthenticator(rs.IoTService))

		// Mount data sub-router for teachers endpoint (device-only auth)
		dataResource := dataAPI.NewResource(rs.IoTService, rs.UsersService, rs.ActivitiesService, rs.FacilityService)
		r.Mount("/teachers", dataResource.TeachersRouter())
	})

	// Device-authenticated routes for RFID devices (API key + PIN)
	r.Group(func(r chi.Router) {
		r.Use(device.DeviceAuthenticator(rs.IoTService, rs.UsersService))

		// Check-in endpoints (student RFID check-in/checkout workflow)
		checkinResource := checkinAPI.NewResource(
			rs.IoTService,
			rs.UsersService,
			rs.ActiveService,
			rs.FacilityService,
			rs.ActivitiesService,
			rs.EducationService,
		)
		// Register routes directly instead of mounting at "/" to avoid Chi conflict
		checkinHandler := delegateHandler(checkinResource.Router())
		r.Post("/checkin", checkinHandler)
		r.Post("/ping", checkinHandler)
		r.Get("/status", checkinHandler)

		// Feedback endpoint (device-based feedback submission)
		feedbackResource := feedbackAPI.NewResource(rs.IoTService, rs.UsersService, rs.FeedbackService)
		r.Post("/feedback", delegateHandler(feedbackResource.Router()))

		// Data query endpoints (device + PIN auth)
		dataResourceAuth := dataAPI.NewResource(rs.IoTService, rs.UsersService, rs.ActivitiesService, rs.FacilityService)
		dataHandler := delegateHandler(dataResourceAuth.Router())
		r.Get("/students", dataHandler)
		r.Get("/activities", dataHandler)
		r.Get("/rooms/available", dataHandler)
		r.Get("/rfid/{tagId}", dataHandler)

		// Mount attendance sub-router (handles daily attendance tracking)
		attendanceResource := attendance.NewResource(rs.UsersService, rs.ActiveService, rs.EducationService)
		r.Mount("/attendance", attendanceResource.Router())

		// Mount sessions sub-router (handles activity session management and timeout)
		sessionsResource := sessionsAPI.NewResource(
			rs.IoTService,
			rs.UsersService,
			rs.ActiveService,
			rs.ActivitiesService,
			rs.ConfigService,
			rs.FacilityService,
			rs.EducationService,
		)
		r.Mount("/session", sessionsResource.Router())

		// Mount RFID sub-router (handles RFID tag assignment/unassignment for staff)
		rfidResource := rfidAPI.NewResource(rs.UsersService)
		r.Mount("/staff", rfidResource.Router())
	})

	return r
}
