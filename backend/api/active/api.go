package active

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/authorize/policy"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	userSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// Resource defines the active API resource
type Resource struct {
	ActiveService      activeSvc.Service
	PersonService      userSvc.PersonService
	SchulhofService    facilities.SchulhofService
	UserContextService usercontext.UserContextService
	db                 *bun.DB
	logger             *slog.Logger
}

// NewResource creates a new active resource
func NewResource(activeService activeSvc.Service, personService userSvc.PersonService, schulhofService facilities.SchulhofService, userContextService usercontext.UserContextService, db *bun.DB, logger *slog.Logger) *Resource {
	return &Resource{
		ActiveService:      activeService,
		PersonService:      personService,
		SchulhofService:    schulhofService,
		UserContextService: userContextService,
		db:                 db,
		logger:             logger,
	}
}

// Router returns a configured router for active endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Active Groups
		r.Route("/groups", func(r chi.Router) {
			// Read operations
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", rs.listActiveGroups)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/unclaimed", rs.listUnclaimedGroups)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}", rs.getActiveGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/room/{roomId}", rs.getActiveGroupsByRoom)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get(routeGroupByGroupID, rs.getActiveGroupsByGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}/visits", rs.getActiveGroupVisits)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}/visits/display", rs.getActiveGroupVisitsWithDisplay)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}/supervisors", rs.getActiveGroupSupervisors)

			// Write operations
			r.With(authorize.RequiresPermission(permissions.GroupsCreate)).Post("/", rs.createActiveGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Put("/{id}", rs.updateActiveGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsDelete)).Delete("/{id}", rs.deleteActiveGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post(routeEndByID, rs.endActiveGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post("/{id}/claim", rs.claimGroup)
		})

		// Visits
		r.Route("/visits", func(r chi.Router) {
			// Read operations
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", rs.listVisits)
			r.With(authorize.GetResourceAuthorizer().RequiresResourceAccess("visit", policy.ActionView, VisitIDExtractor())).Get("/{id}", rs.getVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/student/{studentId}", rs.getStudentVisits)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/student/{studentId}/current", rs.getStudentCurrentVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get(routeGroupByGroupID, rs.getVisitsByGroup)

			// Write operations
			r.With(authorize.RequiresPermission(permissions.GroupsCreate)).Post("/", rs.createVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Put("/{id}", rs.updateVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsDelete)).Delete("/{id}", rs.deleteVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post(routeEndByID, rs.endVisit)

			// Immediate checkout for students
			r.With(authorize.RequiresPermission(permissions.VisitsUpdate)).Post("/student/{studentId}/checkout", rs.checkoutStudent)

			// Immediate check-in for students (from home)
			r.With(authorize.RequiresPermission(permissions.VisitsUpdate)).Post("/student/{studentId}/checkin", rs.checkinStudent)
		})

		// Supervisors
		r.Route("/supervisors", func(r chi.Router) {
			// Read operations
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", rs.listSupervisors)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}", rs.getSupervisor)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/staff/{staffId}", rs.getStaffSupervisions)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/staff/{staffId}/active", rs.getStaffActiveSupervisions)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get(routeGroupByGroupID, rs.getSupervisorsByGroup)

			// Write operations
			r.With(authorize.RequiresPermission(permissions.GroupsAssign)).Post("/", rs.createSupervisor)
			r.With(authorize.RequiresPermission(permissions.GroupsAssign)).Put("/{id}", rs.updateSupervisor)
			r.With(authorize.RequiresPermission(permissions.GroupsAssign)).Delete("/{id}", rs.deleteSupervisor)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post(routeEndByID, rs.endSupervision)
		})

		// Combined Groups
		r.Route("/combined", func(r chi.Router) {
			// Read operations
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", rs.listCombinedGroups)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/active", rs.getActiveCombinedGroups)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}", rs.getCombinedGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}/groups", rs.getCombinedGroupGroups)

			// Write operations
			r.With(authorize.RequiresPermission(permissions.GroupsCreate)).Post("/", rs.createCombinedGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Put("/{id}", rs.updateCombinedGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsDelete)).Delete("/{id}", rs.deleteCombinedGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post(routeEndByID, rs.endCombinedGroup)
		})

		// Group Mappings
		r.Route("/mappings", func(r chi.Router) {
			// Read operations
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get(routeGroupByGroupID, rs.getGroupMappings)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/combined/{combinedId}", rs.getCombinedGroupMappings)

			// Write operations
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post("/add", rs.addGroupToCombination)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post("/remove", rs.removeGroupFromCombination)
		})

		// Analytics
		r.Route("/analytics", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/counts", rs.getCounts)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/room/{roomId}/utilization", rs.getRoomUtilization)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/student/{studentId}/attendance", rs.getStudentAttendance)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/dashboard", rs.getDashboardAnalytics)
		})

		// Schulhof (schoolyard) - permanent outdoor supervision area
		r.Route("/schulhof", func(r chi.Router) {
			schulhofResource := NewSchulhofResource(rs.SchulhofService, rs.UserContextService)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/status", schulhofResource.getSchulhofStatus)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post("/supervise", schulhofResource.toggleSchulhofSupervision)
		})

	})

	return r
}

// VisitIDExtractor extracts visit information for authorization
func VisitIDExtractor() authorize.ResourceExtractor {
	return func(r *http.Request) (interface{}, map[string]interface{}) {
		idStr := chi.URLParam(r, "id")
		if idStr == "" {
			return nil, nil
		}

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return nil, nil
		}

		// Return the visit ID as the resource ID
		return id, nil
	}
}
