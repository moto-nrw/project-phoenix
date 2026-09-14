package active

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
)

// Resource defines the active API resource
type Resource struct {
	Operations         PresenceOperations
	Presence           PresenceQueries
	PersonService      People
	EducationService   func(context.Context, int64) ([]int64, error)
	SchulhofService    SchulhofStatusQuery
	UserContextService StaffAccess
	SettingsService    Settings
	// SupervisionDashboardService backs the aggregated supervision dashboard
	// endpoint (#2096); assigned after construction to keep the positional
	// constructor's existing call sites unchanged.
	SupervisionDashboardService supervisiondashboard.Query
	protectedRoutes             func(chi.Router, func(chi.Router, common.Middleware))
	logger                      *slog.Logger
	runtime                     RequestRuntime
	authorization               Authorization
}

// getLogger returns the logger required by resource construction.
func (rs *Resource) getLogger() *slog.Logger {
	return rs.logger
}

// NewResource creates a new active resource
func NewResource(operations PresenceOperations, personService People, educationService func(context.Context, int64) ([]int64, error), schulhofService SchulhofStatusQuery, userContextService StaffAccess, settingsService Settings, protectedRoutes func(chi.Router, func(chi.Router, common.Middleware)), logger *slog.Logger, presence PresenceQueries, runtime RequestRuntime, authorization Authorization) *Resource {
	if logger == nil {
		panic("active API: logger is required")
	}
	if protectedRoutes == nil {
		panic("active API: protected tenant routes are required")
	}
	if presence == nil {
		panic("active API: student presence queries are required")
	}
	if runtime.WithStaff == nil {
		panic("active API: staff attribution is required")
	}
	if runtime.TenantID == nil || runtime.MarkRollback == nil {
		panic("active API: tenant request runtime is required")
	}
	if authorization.Visit == nil {
		panic("active API: visit authorization is required")
	}
	if authorization.OperationalOverview == nil {
		panic("active API: overview authorization is required")
	}
	return &Resource{
		Operations:         operations,
		Presence:           presence,
		PersonService:      personService,
		EducationService:   educationService,
		SchulhofService:    schulhofService,
		UserContextService: userContextService,
		SettingsService:    settingsService,
		protectedRoutes:    protectedRoutes,
		logger:             logger,
		runtime:            runtime,
		authorization:      authorization,
	}
}

// Router returns a configured router for active endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	rs.protectedRoutes(r, func(r chi.Router, withTx common.Middleware) {

		// Active Groups
		r.Route("/groups", func(r chi.Router) {
			// Read operations
			r.With(common.RequireActiveGroupRead(), withTx).Get("/", rs.listActiveGroups)
			r.With(common.RequireActiveGroupRead(), withTx).Get("/unclaimed", rs.listUnclaimedGroups)
			r.With(common.RequireActiveGroupRead(), withTx).Get("/{id}", rs.getActiveGroup)
			r.With(common.RequireActiveGroupRead(), withTx).Get("/room/{roomId}", rs.getActiveGroupsByRoom)
			r.With(common.RequireActiveGroupRead(), withTx).Get(routeGroupByGroupID, rs.getActiveGroupsByGroup)
			r.With(common.RequireActiveGroupRead(), withTx).Get("/{id}/visits", rs.getActiveGroupVisits)
			r.With(common.RequireActiveGroupRead(), withTx).Get("/{id}/visits/display", rs.getActiveGroupVisitsWithDisplay)
			r.With(common.RequireActiveGroupRead(), withTx).Get("/{id}/supervisors", rs.getActiveGroupSupervisors)

			// Write operations
			r.With(common.RequireActiveGroupCreate(), withTx).Post("/", rs.createActiveGroup)
			r.With(common.RequireActiveGroupUpdate(), withTx).Put("/{id}", rs.updateActiveGroup)
			r.With(common.RequireActiveGroupDelete(), withTx).Delete("/{id}", rs.deleteActiveGroup)
			r.With(common.RequireActiveGroupUpdate(), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post(routeEndByID, rs.endActiveGroup)
			r.With(common.RequireActiveGroupUpdate(), withTx).Post("/{id}/claim", rs.claimGroup)
		})

		// Visits
		r.Route("/visits", func(r chi.Router) {
			// Read operations
			r.With(common.RequireActiveGroupRead(), withTx).Get("/", rs.listVisits)
			// RequireVisitView needs DB → withTx goes before the access check
			r.With(withTx, rs.requireVisitView).Get("/{id}", rs.getVisit)
			r.With(common.RequireActiveGroupRead(), withTx).Get("/student/{studentId}", rs.getStudentVisits)
			r.With(common.RequireActiveGroupRead(), withTx).Get("/student/{studentId}/current", rs.getStudentCurrentVisit)
			r.With(common.RequireActiveGroupRead(), withTx).Get(routeGroupByGroupID, rs.getVisitsByGroup)

			// Write operations
			r.With(common.RequireActiveGroupCreate(), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post("/", rs.createVisit)
			r.With(common.RequireActiveGroupUpdate(), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Put("/{id}", rs.updateVisit)
			r.With(common.RequireActiveGroupDelete(), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Delete("/{id}", rs.deleteVisit)
			r.With(common.RequireActiveGroupUpdate(), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post(routeEndByID, rs.endVisit)

			// Immediate checkout for students
			r.With(common.RequireVisitUpdate(), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post("/student/{studentId}/checkout", rs.checkoutStudent)

			// Immediate check-in for students (from home)
			r.With(common.RequireVisitUpdate(), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post("/student/{studentId}/checkin", rs.checkinStudent)

			// Bulk assign checked-in students without a room visit to an active room session.
			r.With(common.RequireVisitUpdate(), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post("/transit/assign", rs.assignTransitStudents)
			r.With(common.RequireVisitUpdate(), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post("/move-to-group", rs.moveStudentsToActiveGroup)
			r.With(common.RequireVisitUpdate(), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post("/move-to-transit", rs.moveStudentsToTransit)
		})

		// Aggregated projection for the "Aktuelle Aufsicht" page (#2096):
		// replaces the former Next.js BFF fan-out. Gated on groups:read (the
		// widest common permission of the replaced endpoints); sections that
		// required schedules:read or users:read are redacted deterministically
		// inside the service for callers missing them.
		r.With(common.RequireActiveGroupRead(), withTx).Get("/supervision-dashboard", rs.getSupervisionDashboard)

		// Supervisors
		r.Route("/supervisors", func(r chi.Router) {
			// Read operations
			r.With(common.RequireActiveGroupRead(), withTx).Get("/", rs.listSupervisors)
			r.With(common.RequireActiveGroupRead(), withTx).Get("/{id}", rs.getSupervisor)
			r.With(common.RequireActiveGroupRead(), withTx).Get("/staff/{staffId}", rs.getStaffSupervisions)
			r.With(common.RequireActiveGroupRead(), withTx).Get("/staff/{staffId}/active", rs.getStaffActiveSupervisions)
			r.With(common.RequireActiveGroupRead(), withTx).Get("/all", rs.getAllActiveSupervisions)
			r.With(common.RequireActiveGroupRead(), withTx).Get(routeGroupByGroupID, rs.getSupervisorsByGroup)

			// Write operations
			r.With(common.RequireActiveGroupAssign(), withTx).Post("/", rs.createSupervisor)
			r.With(common.RequireActiveGroupAssign(), withTx).Put("/{id}", rs.updateSupervisor)
			r.With(common.RequireActiveGroupAssign(), withTx).Delete("/{id}", rs.deleteSupervisor)
			r.With(common.RequireActiveGroupUpdate(), withTx).Post(routeEndByID, rs.endSupervision)
		})

		// Combined Groups
		r.Route("/combined", func(r chi.Router) {
			// Read operations
			r.With(common.RequireActiveGroupRead(), withTx).Get("/", rs.listCombinedGroups)
			r.With(common.RequireActiveGroupRead(), withTx).Get("/active", rs.getActiveCombinedGroups)
			r.With(common.RequireActiveGroupRead(), withTx).Get("/{id}", rs.getCombinedGroup)
			r.With(common.RequireActiveGroupRead(), withTx).Get("/{id}/groups", rs.getCombinedGroupGroups)

			// Write operations
			r.With(common.RequireActiveGroupCreate(), withTx).Post("/", rs.createCombinedGroup)
			r.With(common.RequireActiveGroupUpdate(), withTx).Put("/{id}", rs.updateCombinedGroup)
			r.With(common.RequireActiveGroupDelete(), withTx).Delete("/{id}", rs.deleteCombinedGroup)
			r.With(common.RequireActiveGroupUpdate(), withTx).Post(routeEndByID, rs.endCombinedGroup)
		})

		// Group Mappings
		r.Route("/mappings", func(r chi.Router) {
			// Read operations
			r.With(common.RequireActiveGroupRead(), withTx).Get(routeGroupByGroupID, rs.getGroupMappings)
			r.With(common.RequireActiveGroupRead(), withTx).Get("/combined/{combinedId}", rs.getCombinedGroupMappings)

			// Write operations
			r.With(common.RequireActiveGroupUpdate(), withTx).Post("/add", rs.addGroupToCombination)
			r.With(common.RequireActiveGroupUpdate(), withTx).Post("/remove", rs.removeGroupFromCombination)
		})

		// Analytics
		r.Route("/analytics", func(r chi.Router) {
			r.With(common.RequireActiveGroupRead(), withTx).Get("/dashboard", rs.getDashboardAnalytics)
		})

		// Tracking indicators (bulk check if students visited configured rooms/activities today)
		r.With(common.RequireActiveGroupRead(), withTx).Post("/tracking-indicators", rs.getTrackingIndicators)

		// Cross-tenant students (Ferienbetreuung / holiday care)
		r.With(common.RequireActiveGroupRead(), withTx).Get("/cross-tenant-students", rs.getCrossTenantStudents)

		// Schulhof (schoolyard) - status read model for the permanent tab.
		// Supervision itself runs through the generic spontaneous-start and
		// claim endpoints since #2161.
		r.Route("/schulhof", func(r chi.Router) {
			schulhofResource := NewSchulhofResource(rs.SchulhofService, rs.UserContextService)
			r.With(common.RequireActiveGroupRead(), withTx).Get("/status", schulhofResource.getSchulhofStatus)
		})

	})

	return r
}
