package reminders

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	remindersService "github.com/moto-nrw/project-phoenix/services/reminders"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/tenant"
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

	tokenAuth := jwt.MustNewTokenAuth()
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)
		withTx := tenant.TenantTxMiddleware(rs.db)

		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/", rs.listReminders)
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

	claims := jwt.ClaimsFromCtx(ctx)
	scope := remindersService.Scope{IsAdmin: claims.IsAdmin}

	if !claims.IsAdmin {
		if rs.UserContext == nil {
			common.RenderError(w, r, common.ErrorInternalServer(errors.New("user context is not configured")))
			return
		}
		staff, err := rs.UserContext.GetCurrentStaff(ctx)
		switch {
		case errors.Is(err, usercontext.ErrUserNotLinkedToStaff):
			// A tenant account with users:read but no staff row is an
			// authorization miss for this staff-only endpoint, not a server
			// fault — deny it so the header poll doesn't spam 500s.
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
