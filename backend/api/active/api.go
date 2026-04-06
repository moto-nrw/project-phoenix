package active

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/authorize/policy"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	base "github.com/moto-nrw/project-phoenix/models/base"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	userSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
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

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (rs *Resource) getLogger() *slog.Logger {
	if rs.logger != nil {
		return rs.logger
	}
	return slog.Default()
}

// getDB returns the tenant-scoped transaction from context if available, otherwise the base DB.
// Raw rs.db queries bypass RLS — always use this instead.
func (rs *Resource) getDB(ctx context.Context) bun.IDB {
	if tx, ok := base.TxFromContext(ctx); ok && tx != nil {
		return tx
	}
	return rs.db
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
	tokenAuth := jwt.MustNewTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)
		withTx := tenant.TenantTxMiddleware(rs.db)

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
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate), withTx).Post(routeEndByID, rs.endActiveGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate), withTx).Post("/{id}/claim", rs.claimGroup)
		})

		// Visits
		r.Route("/visits", func(r chi.Router) {
			// Read operations
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/", rs.listVisits)
			// RequiresResourceAccess needs DB → withTx goes before resource access check
			r.With(withTx, authorize.GetResourceAuthorizer().RequiresResourceAccess("visit", policy.ActionView, VisitIDExtractor())).Get("/{id}", rs.getVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/student/{studentId}", rs.getStudentVisits)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/student/{studentId}/current", rs.getStudentCurrentVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get(routeGroupByGroupID, rs.getVisitsByGroup)

			// Write operations
			r.With(authorize.RequiresPermission(permissions.GroupsCreate), withTx).Post("/", rs.createVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate), withTx).Put("/{id}", rs.updateVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsDelete), withTx).Delete("/{id}", rs.deleteVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate), withTx).Post(routeEndByID, rs.endVisit)

			// Immediate checkout for students
			r.With(authorize.RequiresPermission(permissions.VisitsUpdate), withTx).Post("/student/{studentId}/checkout", rs.checkoutStudent)

			// Immediate check-in for students (from home)
			r.With(authorize.RequiresPermission(permissions.VisitsUpdate), withTx).Post("/student/{studentId}/checkin", rs.checkinStudent)
		})

		// Supervisors
		r.Route("/supervisors", func(r chi.Router) {
			// Read operations
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/", rs.listSupervisors)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/{id}", rs.getSupervisor)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/staff/{staffId}", rs.getStaffSupervisions)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/staff/{staffId}/active", rs.getStaffActiveSupervisions)
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
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/counts", rs.getCounts)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/dashboard", rs.getDashboardAnalytics)
		})

		// Cross-tenant students (Ferienbetreuung / holiday care)
		r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/cross-tenant-students", rs.getCrossTenantStudents)

		// Schulhof (schoolyard) - permanent outdoor supervision area
		r.Route("/schulhof", func(r chi.Router) {
			schulhofResource := NewSchulhofResource(rs.SchulhofService, rs.UserContextService)
			r.With(authorize.RequiresPermission(permissions.GroupsRead), withTx).Get("/status", schulhofResource.getSchulhofStatus)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate), withTx).Post("/supervise", schulhofResource.toggleSchulhofSupervision)
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
