package iot

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/device"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/attendance"
	checkinAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/checkin"
	dataAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/data"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/devices"
	feedbackAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/feedback"
	rfidAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/rfid"
	sessionsAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/sessions"
	configSvc "github.com/moto-nrw/project-phoenix/internal/core/service/config"
	activeSvc "github.com/moto-nrw/project-phoenix/internal/core/service/active"
	activitiesSvc "github.com/moto-nrw/project-phoenix/internal/core/service/activities"
	educationSvc "github.com/moto-nrw/project-phoenix/internal/core/service/education"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/internal/core/service/facilities"
	feedbackSvc "github.com/moto-nrw/project-phoenix/internal/core/service/feedback"
	iotSvc "github.com/moto-nrw/project-phoenix/internal/core/service/iot"
	usersSvc "github.com/moto-nrw/project-phoenix/internal/core/service/users"
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
	DevicePIN         string
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
	DevicePIN         string
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
		DevicePIN:         deps.DevicePIN,
	}
}

// Router returns a configured router for IoT endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustTokenAuth()

	// Public routes (if any device endpoints should be public)
	r.Group(func(r chi.Router) {
		// Some basic device info might be public
		// Currently no public routes for IoT devices
	})

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Mount devices sub-router (handles device CRUD and admin operations)
		// All device routes require JWT authentication with IOT permissions
		devicesResource := devices.NewResource(rs.IoTService)
		r.Mount("/", devicesResource.Router())
	})

	// Device-only authenticated routes (API key only, no PIN required)
	r.Group(func(r chi.Router) {
		r.Use(device.DeviceOnlyAuthenticator(rs.IoTService))

		// Mount data sub-router for teachers endpoint (device-only auth)
		dataResource := dataAPI.NewResource(rs.IoTService, rs.UsersService, rs.ActivitiesService, rs.FacilityService)
		r.Mount("/teachers", dataResource.TeachersRouter())
	})

	// Device-authenticated routes for RFID devices
	r.Group(func(r chi.Router) {
		r.Use(device.DeviceAuthenticator(rs.IoTService, rs.UsersService, rs.DevicePIN))

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
