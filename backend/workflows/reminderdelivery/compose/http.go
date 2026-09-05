// Package compose binds Reminder Delivery to the shared runtime.
package compose

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	remindersHTTP "github.com/moto-nrw/project-phoenix/api/reminders"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/uptrace/bun"
)

// HTTPRuntime preserves tenant transactions, permission checks, and responses.
func HTTPRuntime(db *bun.DB) remindersHTTP.Runtime {
	return remindersHTTP.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, remindersHTTP.Middleware)) {
			common.ProtectedTenantGroup(router, db, register)
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
