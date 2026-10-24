package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	worktimemodelsHTTP "github.com/moto-nrw/project-phoenix/modules/workforce/http/worktimemodels"
)

// newWorkTimeModelsResource wires the /api/work-time-models adapter over the Workforce
// capability. NotifyChanged invalidates the time-account views after a
// template edit rewrote the assigned staff schedules.
func newWorkTimeModelsResource(models workforce.Capability, notifyChanged func(context.Context)) *worktimemodelsHTTP.Resource {
	if models == nil || notifyChanged == nil {
		panic("work-time-models HTTP composition: all dependencies are required")
	}
	return worktimemodelsHTTP.NewResource(models, workTimeModelsRuntime(notifyChanged))
}

func workTimeModelsRuntime(notifyChanged func(context.Context)) worktimemodelsHTTP.Runtime {
	return worktimemodelsHTTP.Runtime{
		Protected:     workTimeModelsProtectedRoutes(),
		Permission:    apiCommon.RequiresPermission,
		ParseID:       apiCommon.ParseID,
		Success:       apiCommon.Respond,
		Failure:       renderWorkTimeModelsFailure,
		Manage:        permissions.TimeTrackingManage,
		NotifyChanged: notifyChanged,
	}
}

func workTimeModelsProtectedRoutes() func(chi.Router, func(chi.Router, worktimemodelsHTTP.Middleware)) {
	return func(router chi.Router, routes func(chi.Router, worktimemodelsHTTP.Middleware)) {
		ProtectedTenantRoutes(router, routes)
	}
}

func renderWorkTimeModelsFailure(w http.ResponseWriter, r *http.Request, kind worktimemodelsHTTP.FailureKind, err error) {
	switch kind {
	case worktimemodelsHTTP.FailureInvalid:
		apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
	case worktimemodelsHTTP.FailureNotFound:
		apiCommon.RenderError(w, r, apiCommon.ErrorNotFound(err))
	case worktimemodelsHTTP.FailureConflict:
		apiCommon.RenderError(w, r, apiCommon.ErrorConflict(err))
	default:
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServer(err))
	}
}
