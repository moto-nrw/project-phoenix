package sessions

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/realtime"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource defines the Sessions API resource for activity session management
type Resource struct {
	IoTService        iotSvc.Service
	UsersService      usersSvc.PersonService
	ActiveService     activeSvc.Service
	ActivitiesService activitiesSvc.ActivityService
	FacilityService   facilitiesSvc.Service
	EducationService  educationSvc.Service
	TimetableData     *scheduleSvc.TimetableDataService
	TimetableBridge   *scheduleSvc.TimetableBridgeService
	Broadcaster       realtime.Broadcaster
}

// NewResource creates a new Sessions resource
func NewResource(
	iotService iotSvc.Service,
	usersService usersSvc.PersonService,
	activeService activeSvc.Service,
	activitiesService activitiesSvc.ActivityService,
	facilityService facilitiesSvc.Service,
	educationService educationSvc.Service,
) *Resource {
	return &Resource{
		IoTService:        iotService,
		UsersService:      usersService,
		ActiveService:     activeService,
		ActivitiesService: activitiesService,
		FacilityService:   facilityService,
		EducationService:  educationService,
	}
}

// ConfigureTimetableMirror wires optional repositories used to mirror
// PyrePortal/RFID activity sessions into the timetable instance layer.
func (rs *Resource) ConfigureTimetableMirror(
	timetableData *scheduleSvc.TimetableDataService,
	timetableBridge *scheduleSvc.TimetableBridgeService,
	broadcaster realtime.Broadcaster,
) {
	rs.TimetableData = timetableData
	rs.TimetableBridge = timetableBridge
	rs.Broadcaster = broadcaster
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
	r.Post("/activity", rs.updateSessionActivity)
	r.Post("/validate-timeout", rs.validateSessionTimeout)
	r.Get("/timeout-info", rs.getSessionTimeoutInfo)

	return r
}
