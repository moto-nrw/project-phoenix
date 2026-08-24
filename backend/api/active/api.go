package active

import (
	"cmp"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/services/supervisiondashboard"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	userSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// Resource defines the active API resource
type Resource struct {
	ActiveService      activeSvc.Service
	PersonService      userSvc.PersonService
	EducationService   educationSvc.Service
	SchulhofService    facilities.SchulhofService
	UserContextService usercontext.UserContextService
	SettingsService    configSvc.SettingsService
	// SupervisionDashboardService backs the aggregated supervision dashboard
	// endpoint (#2096); assigned after construction to keep the positional
	// constructor's existing call sites unchanged.
	SupervisionDashboardService supervisiondashboard.Getter
	db                          *bun.DB
	logger                      *slog.Logger
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (rs *Resource) getLogger() *slog.Logger {
	return cmp.Or(rs.logger, slog.Default())
}

// NewResource creates a new active resource
func NewResource(activeService activeSvc.Service, personService userSvc.PersonService, educationService educationSvc.Service, schulhofService facilities.SchulhofService, userContextService usercontext.UserContextService, settingsService configSvc.SettingsService, db *bun.DB, logger *slog.Logger) *Resource {
	return &Resource{
		ActiveService:      activeService,
		PersonService:      personService,
		EducationService:   educationService,
		SchulhofService:    schulhofService,
		UserContextService: userContextService,
		SettingsService:    settingsService,
		db:                 db,
		logger:             logger,
	}
}

// Router returns a configured router for active endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {

		// Active Groups
		r.Route("/groups", func(r chi.Router) {
			// Read operations
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/", rs.listActiveGroups)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/unclaimed", rs.listUnclaimedGroups)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/{id}", rs.getActiveGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/room/{roomId}", rs.getActiveGroupsByRoom)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get(routeGroupByGroupID, rs.getActiveGroupsByGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/{id}/visits", rs.getActiveGroupVisits)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/{id}/visits/display", rs.getActiveGroupVisitsWithDisplay)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/{id}/supervisors", rs.getActiveGroupSupervisors)

			// Write operations
			r.With(authorize.RequiresPermission(permissions.GroupsCreate), withTx).Post("/", rs.createActiveGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate), withTx).Put("/{id}", rs.updateActiveGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsDelete), withTx).Delete("/{id}", rs.deleteActiveGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post(routeEndByID, rs.endActiveGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate), withTx).Post("/{id}/claim", rs.claimGroup)
		})

		// Visits
		r.Route("/visits", func(r chi.Router) {
			// Read operations
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/", rs.listVisits)
			// RequireVisitView needs DB → withTx goes before the access check
			r.With(withTx, authorize.RequireVisitView(rs.ActiveService, rs.PersonService, rs.EducationService)).Get("/{id}", rs.getVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/student/{studentId}", rs.getStudentVisits)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/student/{studentId}/current", rs.getStudentCurrentVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get(routeGroupByGroupID, rs.getVisitsByGroup)

			// Write operations
			r.With(authorize.RequiresPermission(permissions.GroupsCreate), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post("/", rs.createVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Put("/{id}", rs.updateVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsDelete), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Delete("/{id}", rs.deleteVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post(routeEndByID, rs.endVisit)

			// Immediate checkout for students
			r.With(authorize.RequiresPermission(permissions.VisitsUpdate), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post("/student/{studentId}/checkout", rs.checkoutStudent)

			// Immediate check-in for students (from home)
			r.With(authorize.RequiresPermission(permissions.VisitsUpdate), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post("/student/{studentId}/checkin", rs.checkinStudent)

			// Bulk assign checked-in students without a room visit to an active room session.
			r.With(authorize.RequiresPermission(permissions.VisitsUpdate), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post("/transit/assign", rs.assignTransitStudents)
			r.With(authorize.RequiresPermission(permissions.VisitsUpdate), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post("/move-to-group", rs.moveStudentsToActiveGroup)
			r.With(authorize.RequiresPermission(permissions.VisitsUpdate), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post("/move-to-transit", rs.moveStudentsToTransit)
		})

		// Aggregated projection for the "Aktuelle Aufsicht" page (#2096):
		// replaces the former Next.js BFF fan-out. Gated on groups:read (the
		// widest common permission of the replaced endpoints); sections that
		// required schedules:read or users:read are redacted deterministically
		// inside the service for callers missing them.
		r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/supervision-dashboard", rs.getSupervisionDashboard)

		// Supervisors
		r.Route("/supervisors", func(r chi.Router) {
			// Read operations
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/", rs.listSupervisors)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/{id}", rs.getSupervisor)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/staff/{staffId}", rs.getStaffSupervisions)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/staff/{staffId}/active", rs.getStaffActiveSupervisions)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/all", rs.getAllActiveSupervisions)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get(routeGroupByGroupID, rs.getSupervisorsByGroup)

			// Write operations
			r.With(authorize.RequiresPermission(permissions.GroupsAssign), withTx).Post("/", rs.createSupervisor)
			r.With(authorize.RequiresPermission(permissions.GroupsAssign), withTx).Put("/{id}", rs.updateSupervisor)
			r.With(authorize.RequiresPermission(permissions.GroupsAssign), withTx).Delete("/{id}", rs.deleteSupervisor)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate), withTx).Post(routeEndByID, rs.endSupervision)
		})

		// Combined Groups
		r.Route("/combined", func(r chi.Router) {
			// Read operations
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/", rs.listCombinedGroups)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/active", rs.getActiveCombinedGroups)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/{id}", rs.getCombinedGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/{id}/groups", rs.getCombinedGroupGroups)

			// Write operations
			r.With(authorize.RequiresPermission(permissions.GroupsCreate), withTx).Post("/", rs.createCombinedGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate), withTx).Put("/{id}", rs.updateCombinedGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsDelete), withTx).Delete("/{id}", rs.deleteCombinedGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate), withTx).Post(routeEndByID, rs.endCombinedGroup)
		})

		// Group Mappings
		r.Route("/mappings", func(r chi.Router) {
			// Read operations
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get(routeGroupByGroupID, rs.getGroupMappings)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/combined/{combinedId}", rs.getCombinedGroupMappings)

			// Write operations
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate), withTx).Post("/add", rs.addGroupToCombination)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate), withTx).Post("/remove", rs.removeGroupFromCombination)
		})

		// Analytics
		r.Route("/analytics", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/dashboard", rs.getDashboardAnalytics)
		})

		// Tracking indicators (bulk check if students visited configured rooms/activities today)
		r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Post("/tracking-indicators", rs.getTrackingIndicators)

		// Cross-tenant students (Ferienbetreuung / holiday care)
		r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/cross-tenant-students", rs.getCrossTenantStudents)

		// Schulhof (schoolyard) - status read model for the permanent tab.
		// Supervision itself runs through the generic spontaneous-start and
		// claim endpoints since #2161.
		r.Route("/schulhof", func(r chi.Router) {
			schulhofResource := NewSchulhofResource(rs.SchulhofService, rs.UserContextService)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/status", schulhofResource.getSchulhofStatus)
		})

	})

	return r
}
