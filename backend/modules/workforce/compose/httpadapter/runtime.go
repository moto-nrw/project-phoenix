// Package httpadapter supplies the HTTP-platform behavior the
// /api/work-time-models adapter must not own: authentication, tenant
// transaction scoping, the shared response envelope, and the post-commit
// time-tracking notification.
package httpadapter

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	worktimemodelsHTTP "github.com/moto-nrw/project-phoenix/api/work-time-models"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/uptrace/bun"
)

type Resource = worktimemodelsHTTP.Resource

// NewResource wires the /api/work-time-models adapter over the Workforce
// capability. NotifyChanged invalidates the time-account views after a
// template edit rewrote the assigned staff schedules.
func NewResource(models workforce.Capability, db *bun.DB, notifyChanged func(context.Context)) *Resource {
	if models == nil || db == nil || notifyChanged == nil {
		panic("work-time-models HTTP composition: all dependencies are required")
	}
	return worktimemodelsHTTP.NewResource(models, runtime(db, notifyChanged))
}

func runtime(db *bun.DB, notifyChanged func(context.Context)) worktimemodelsHTTP.Runtime {
	return worktimemodelsHTTP.Runtime{
		Protected:     protectedRoutes(db),
		Permission:    apiCommon.RequiresPermission,
		ParseID:       apiCommon.ParseID,
		Success:       apiCommon.Respond,
		Failure:       renderFailure,
		Manage:        permissions.TimeTrackingManage,
		NotifyChanged: notifyChanged,
	}
}

func protectedRoutes(db *bun.DB) func(chi.Router, func(chi.Router, worktimemodelsHTTP.Middleware)) {
	return func(router chi.Router, routes func(chi.Router, worktimemodelsHTTP.Middleware)) {
		apiCommon.ProtectedTenantGroup(router, db, routes)
	}
}

func renderFailure(w http.ResponseWriter, r *http.Request, kind worktimemodelsHTTP.FailureKind, err error) {
	switch kind {
	case worktimemodelsHTTP.FailureInvalid:
		apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
	case worktimemodelsHTTP.FailureNotFound:
		apiCommon.RenderError(w, r, apiCommon.ErrorNotFound(err))
	default:
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServer(err))
	}
}
