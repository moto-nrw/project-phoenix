package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	remindersHTTP "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/http"
)

// HTTPRuntime preserves tenant transactions, permission checks, and responses.
func ReminderHTTPRuntime(protectedTenantRoutes func(chi.Router, func(chi.Router, common.Middleware))) remindersHTTP.Runtime {
	return remindersHTTP.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, remindersHTTP.Middleware)) {
			protectedTenantRoutes(router, register)
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
