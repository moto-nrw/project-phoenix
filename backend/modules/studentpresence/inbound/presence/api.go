package presence

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
)

// Resource defines the active API resource
type Resource struct {
	Operations         PresenceOperations
	Presence           PresenceQueries
	PersonService      People
	EducationService   func(context.Context, int64) ([]int64, error)
	UserContextService StaffAccess
	SettingsService    Settings
	displaySettings    DisplaySettings

	protectedRoutes func(chi.Router, func(chi.Router, common.Middleware))
	logger          *slog.Logger
	runtime         RequestRuntime
	authorization   Authorization
}

// getLogger returns the logger required by resource construction.
func (rs *Resource) getLogger() *slog.Logger {
	return rs.logger
}

// NewResource creates a new active resource
func NewResource(operations PresenceOperations, personService People, educationService func(context.Context, int64) ([]int64, error), userContextService StaffAccess, settingsService Settings, protectedRoutes func(chi.Router, func(chi.Router, common.Middleware)), logger *slog.Logger, presence PresenceQueries, runtime RequestRuntime, authorization Authorization, displaySettings DisplaySettings) *Resource {
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
	if authorization.StudentDataAccess == nil {
		panic("active API: student data authorization is required")
	}
	if displaySettings == nil {
		panic("active API: display settings facts are required")
	}
	return &Resource{
		Operations:         operations,
		Presence:           presence,
		PersonService:      personService,
		EducationService:   educationService,
		UserContextService: userContextService,
		SettingsService:    settingsService,
		displaySettings:    displaySettings,
		protectedRoutes:    protectedRoutes,
		logger:             logger,
		runtime:            runtime,
		authorization:      authorization,
	}
}

// Router returns a configured router for active endpoints
func (rs *Resource) Router() chi.Router { return rs.RouterWithOwnerRoutes() }

func (rs *Resource) RouterWithOwnerRoutes(mounts ...func(chi.Router, common.Middleware)) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	rs.protectedRoutes(r, func(r chi.Router, withTx common.Middleware) {

		for _, mount := range mounts {
			mount(r, withTx)
		}

		rs.registerActiveGroupRoutes(r, withTx)
		rs.registerVisitRoutes(r, withTx)
		rs.registerSupervisorRoutes(r, withTx)
		rs.registerCombinedRoutes(r, withTx)
		rs.registerMappingRoutes(r, withTx)
		rs.registerAnalyticsRoutes(r, withTx)

	})

	return r
}

func (rs *Resource) registerActiveGroupRoutes(r chi.Router, withTx common.Middleware) {
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
		r.With(common.RequireActiveGroupUpdate(), withTx, common.RequireWebAttendanceEnabled(rs.displaySettings.WebAttendanceEnabledReader())).Post(routeEndByID, rs.endActiveGroup)
		r.With(common.RequireActiveGroupUpdate(), withTx).Post("/{id}/claim", rs.claimGroup)
	})

}

func (rs *Resource) registerVisitRoutes(r chi.Router, withTx common.Middleware) {
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
		r.With(common.RequireActiveGroupCreate(), withTx, common.RequireWebAttendanceEnabled(rs.displaySettings.WebAttendanceEnabledReader())).Post("/", rs.createVisit)
		r.With(common.RequireActiveGroupUpdate(), withTx, common.RequireWebAttendanceEnabled(rs.displaySettings.WebAttendanceEnabledReader())).Put("/{id}", rs.updateVisit)
		r.With(common.RequireActiveGroupDelete(), withTx, common.RequireWebAttendanceEnabled(rs.displaySettings.WebAttendanceEnabledReader())).Delete("/{id}", rs.deleteVisit)
		r.With(common.RequireActiveGroupUpdate(), withTx, common.RequireWebAttendanceEnabled(rs.displaySettings.WebAttendanceEnabledReader())).Post(routeEndByID, rs.endVisit)

		// Immediate checkout for students
		r.With(common.RequireVisitUpdate(), withTx, common.RequireWebAttendanceEnabled(rs.displaySettings.WebAttendanceEnabledReader())).Post("/student/{studentId}/checkout", rs.checkoutStudent)

		// Bulk assign checked-in students without a room visit to an active room session.
		r.With(common.RequireVisitUpdate(), withTx, common.RequireWebAttendanceEnabled(rs.displaySettings.WebAttendanceEnabledReader())).Post("/transit/assign", rs.assignTransitStudents)
		r.With(common.RequireVisitUpdate(), withTx, common.RequireWebAttendanceEnabled(rs.displaySettings.WebAttendanceEnabledReader())).Post("/move-to-group", rs.moveStudentsToActiveGroup)

		r.With(common.RequireVisitUpdate(), withTx, common.RequireWebAttendanceEnabled(rs.displaySettings.WebAttendanceEnabledReader())).Post("/move-to-transit", rs.moveStudentsToTransit)
	})

}

func (rs *Resource) registerSupervisorRoutes(r chi.Router, withTx common.Middleware) {
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

}

func (rs *Resource) registerCombinedRoutes(r chi.Router, withTx common.Middleware) {
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

}

func (rs *Resource) registerMappingRoutes(r chi.Router, withTx common.Middleware) {
	// Group Mappings
	r.Route("/mappings", func(r chi.Router) {
		// Read operations
		r.With(common.RequireActiveGroupRead(), withTx).Get(routeGroupByGroupID, rs.getGroupMappings)
		r.With(common.RequireActiveGroupRead(), withTx).Get("/combined/{combinedId}", rs.getCombinedGroupMappings)

		// Write operations
		r.With(common.RequireActiveGroupUpdate(), withTx).Post("/add", rs.addGroupToCombination)
		r.With(common.RequireActiveGroupUpdate(), withTx).Post("/remove", rs.removeGroupFromCombination)
	})

}

func (rs *Resource) registerAnalyticsRoutes(r chi.Router, withTx common.Middleware) {
	// Analytics
	r.Route("/analytics", func(r chi.Router) {
		r.With(common.RequireActiveGroupRead(), withTx).Get("/dashboard", rs.getDashboardAnalytics)
	})

	// Tracking indicators (bulk check if students visited configured rooms/activities today)
	r.With(common.RequireActiveGroupRead(), withTx).Post("/tracking-indicators", rs.getTrackingIndicators)

	// Cross-tenant students (Ferienbetreuung / holiday care)
	r.With(common.RequireActiveGroupRead(), withTx).Get("/cross-tenant-students", rs.getCrossTenantStudents)

}
