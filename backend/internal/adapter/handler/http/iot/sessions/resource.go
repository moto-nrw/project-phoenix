package sessions

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	configSvc "github.com/moto-nrw/project-phoenix/internal/core/service/config"
	activeSvc "github.com/moto-nrw/project-phoenix/internal/core/service/active"
	activitiesSvc "github.com/moto-nrw/project-phoenix/internal/core/service/activities"
	educationSvc "github.com/moto-nrw/project-phoenix/internal/core/service/education"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/internal/core/service/facilities"
	iotSvc "github.com/moto-nrw/project-phoenix/internal/core/service/iot"
	usersSvc "github.com/moto-nrw/project-phoenix/internal/core/service/users"
)

// Resource defines the Sessions API resource for activity session management
type Resource struct {
	IoTService        iotSvc.Service
	UsersService      usersSvc.PersonService
	ActiveService     activeSvc.Service
	ActivitiesService activitiesSvc.ActivityService
	ConfigService     configSvc.Service
	FacilityService   facilitiesSvc.Service
	EducationService  educationSvc.Service
}

// NewResource creates a new Sessions resource
func NewResource(
	iotService iotSvc.Service,
	usersService usersSvc.PersonService,
	activeService activeSvc.Service,
	activitiesService activitiesSvc.ActivityService,
	configService configSvc.Service,
	facilityService facilitiesSvc.Service,
	educationService educationSvc.Service,
) *Resource {
	return &Resource{
		IoTService:        iotService,
		UsersService:      usersService,
		ActiveService:     activeService,
		ActivitiesService: activitiesService,
		ConfigService:     configService,
		FacilityService:   facilityService,
		EducationService:  educationService,
	}
}

// Router returns a configured router for session management endpoints
// This router is mounted under /iot/session/ and handles activity session lifecycle
// All routes require device authentication (API key + Staff PIN)
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Session CRUD operations
	r.Post("/start", rs.startActivitySession)
	r.Post("/end", rs.endActivitySession)
	r.Get("/current", rs.getCurrentSession)
	r.Post("/check-conflict", rs.checkSessionConflict)
	r.Put("/{sessionId}/supervisors", rs.updateSessionSupervisors)

	// Session timeout management
	r.Post("/timeout", rs.processSessionTimeout)
	r.Get("/timeout-config", rs.getSessionTimeoutConfig)
	r.Post("/activity", rs.updateSessionActivity)
	r.Post("/validate-timeout", rs.validateSessionTimeout)
	r.Get("/timeout-info", rs.getSessionTimeoutInfo)

	return r
}
