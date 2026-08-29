package reminders

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	remindersService "github.com/moto-nrw/project-phoenix/services/reminders"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/uptrace/bun"
)

// Resource exposes the staff "Erinnerungen" reminder list (issue #1457).
type Resource struct {
	RemindersService remindersService.Service
	UserContext      usercontext.UserContextService
	db               *bun.DB
}

// NewResource builds the reminders HTTP resource.
func NewResource(service remindersService.Service, userContext usercontext.UserContextService, db *bun.DB) *Resource {
	return &Resource{
		RemindersService: service,
		UserContext:      userContext,
		db:               db,
	}
}

// Router wires the reminder routes behind tenant JWT auth.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {

		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/", rs.listReminders)
	})

	return r
}

// listReminders computes the current reminders for the authenticated user.
// Admins see all present children and activities; caregivers see only what
// they currently supervise.
func (rs *Resource) listReminders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if rs.RemindersService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("reminders service is not configured")))
		return
	}

	// Admin scope must be derived from the effective permissions, not the
	// literal "admin" role name: the route gate (RequiresPermission users:read)
	// also admits wildcard admins (admin:* / *:*) via custom roles or service
	// accounts, and those callers have claims.IsAdmin == false. Relying on the
	// role name alone would push them down the caregiver path, which yields a
	// wrong 403 for accounts without a staff row, or caregiver-scoped reminders
	// for a full admin. This mirrors CanReadStudent and the rest of the
	// authorization layer.
	isAdmin := common.HasEffectiveAdminScope(ctx)
	scope := remindersService.Scope{IsAdmin: isAdmin}

	if !isAdmin {
		if rs.UserContext == nil {
			common.RenderError(w, r, common.ErrorInternalServer(errors.New("user context is not configured")))
			return
		}
		staff, err := rs.UserContext.GetCurrentStaff(ctx)
		switch {
		case errors.Is(err, usercontext.ErrUserNotLinkedToStaff),
			errors.Is(err, usercontext.ErrUserNotLinkedToPerson):
			// A tenant account with users:read but no staff (or even no person)
			// row is an authorization miss for this staff-only endpoint, not a
			// server fault. GetCurrentStaff returns ErrUserNotLinkedToPerson
			// first for a personless account, so both must map to 403 — otherwise
			// the globally mounted header poll spams 500s.
			common.RenderError(w, r, common.ErrorForbidden(errors.New("not linked to staff")))
			return
		case err != nil:
			common.RenderError(w, r, common.ErrorInternalServer(err))
			return
		}
		scope.StaffID = staff.ID
	}

	result, err := rs.RemindersService.Compute(ctx, scope)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, result, "Reminders retrieved successfully")
}
