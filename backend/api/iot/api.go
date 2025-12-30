package iot

import (
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
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
	feedbackSvc "github.com/moto-nrw/project-phoenix/services/feedback"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

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

// Router returns a configured router for IoT endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

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
		r.Mount("/", dataResource.TeachersRouter())
	})

	// Device-authenticated routes for RFID devices
	r.Group(func(r chi.Router) {
		r.Use(device.DeviceAuthenticator(rs.IoTService, rs.UsersService))

		// Mount check-in sub-router (handles student RFID check-in/checkout workflow)
		checkinResource := checkinAPI.NewResource(
			rs.IoTService,
			rs.UsersService,
			rs.ActiveService,
			rs.FacilityService,
			rs.ActivitiesService,
			rs.EducationService,
		)
		r.Mount("/", checkinResource.Router())

		// Mount feedback sub-router (handles device-based feedback submission)
		feedbackResource := feedbackAPI.NewResource(rs.IoTService, rs.UsersService, rs.FeedbackService)
		r.Mount("/", feedbackResource.Router())

		// Mount data sub-router for device data queries (device + PIN auth)
		dataResourceAuth := dataAPI.NewResource(rs.IoTService, rs.UsersService, rs.ActivitiesService, rs.FacilityService)
		r.Mount("/", dataResourceAuth.Router())

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
