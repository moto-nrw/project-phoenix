package sessions

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	activeSvc "github.com/moto-nrw/project-phoenix/internal/core/service/active"
	activitiesSvc "github.com/moto-nrw/project-phoenix/internal/core/service/activities"
	configSvc "github.com/moto-nrw/project-phoenix/internal/core/service/config"
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

// =============================================================================
// HANDLER ACCESSOR METHODS (for testing)
// =============================================================================

// StartSessionHandler returns the startActivitySession handler
func (rs *Resource) StartSessionHandler() http.HandlerFunc { return rs.startActivitySession }

// EndSessionHandler returns the endActivitySession handler
func (rs *Resource) EndSessionHandler() http.HandlerFunc { return rs.endActivitySession }

// GetCurrentSessionHandler returns the getCurrentSession handler
func (rs *Resource) GetCurrentSessionHandler() http.HandlerFunc { return rs.getCurrentSession }

// CheckConflictHandler returns the checkSessionConflict handler
func (rs *Resource) CheckConflictHandler() http.HandlerFunc { return rs.checkSessionConflict }

// UpdateSupervisorsHandler returns the updateSessionSupervisors handler
func (rs *Resource) UpdateSupervisorsHandler() http.HandlerFunc { return rs.updateSessionSupervisors }

// ProcessTimeoutHandler returns the processSessionTimeout handler
func (rs *Resource) ProcessTimeoutHandler() http.HandlerFunc { return rs.processSessionTimeout }

// GetTimeoutConfigHandler returns the getSessionTimeoutConfig handler
func (rs *Resource) GetTimeoutConfigHandler() http.HandlerFunc { return rs.getSessionTimeoutConfig }

// UpdateActivityHandler returns the updateSessionActivity handler
func (rs *Resource) UpdateActivityHandler() http.HandlerFunc { return rs.updateSessionActivity }

// ValidateTimeoutHandler returns the validateSessionTimeout handler
func (rs *Resource) ValidateTimeoutHandler() http.HandlerFunc { return rs.validateSessionTimeout }

// GetTimeoutInfoHandler returns the getSessionTimeoutInfo handler
func (rs *Resource) GetTimeoutInfoHandler() http.HandlerFunc { return rs.getSessionTimeoutInfo }
