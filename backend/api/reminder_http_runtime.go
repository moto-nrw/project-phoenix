package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	remindersHTTP "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/http"
)

// ReminderHTTPRuntime binds the reminder routes to the tenant transaction,
// the permission checks and the shared responses.
func ReminderHTTPRuntime() remindersHTTP.Runtime {
	return remindersHTTP.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, remindersHTTP.Middleware)) {
			ProtectedTenantRoutes(router, register)
		},
		ReadPermission: common.RequiresPermission(permissions.UsersRead),
		EffectiveAdmin: common.HasEffectiveAdminScope,
		Success:        common.Respond,
		Failure: func(w http.ResponseWriter, r *http.Request, status int, err error) {
			if status == http.StatusForbidden {
				common.RenderError(w, r, common.ErrorForbidden(err))
				return
			}
			common.RenderError(w, r, common.ErrorInternalServer(err))
		},
	}
}
