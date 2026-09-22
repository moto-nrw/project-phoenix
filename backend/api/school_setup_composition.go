package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	schoolSetupCompose "github.com/moto-nrw/project-phoenix/modules/settings/setup/compose"
	"github.com/uptrace/bun"
)

// moduleRoute is a tenant route group a module serves itself. Modules mount
// through this list instead of new API fields, which are shrink-only (#2580).
type moduleRoute struct {
	pattern string
	router  chi.Router
}

// newSchoolSetupRoute mounts the onboarding wizard for new schools (#2832,
// ADR 0035) at /api/school-setup. The caller supplies the retained services;
// this adds the tenant HTTP mechanics.
func newSchoolSetupRoute(deps schoolSetupCompose.Dependencies, db *bun.DB) (moduleRoute, error) {
	deps.Protected = func(r chi.Router, fn func(chi.Router, schoolSetupCompose.Middleware)) {
		apiCommon.ProtectedTenantGroup(r, db, fn)
	}
	deps.RequireWrite = apiCommon.RequireConfigWrite()
	deps.Actor = func(ctx context.Context) (int64, int64) {
		principal, err := apiCommon.CurrentPrincipal(ctx)
		if err != nil {
			return 0, 0
		}
		return principal.TenantID(), principal.AccountID()
	}
	deps.Respond = apiCommon.Respond
	deps.Failure = renderSchoolSetupFailure
	router, err := schoolSetupCompose.New(deps)
	if err != nil {
		return moduleRoute{}, err
	}
	return moduleRoute{pattern: "/school-setup", router: router}, nil
}

func renderSchoolSetupFailure(w http.ResponseWriter, r *http.Request, status int, err error) {
	switch status {
	case http.StatusBadRequest:
		apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
	case http.StatusForbidden:
		apiCommon.RenderError(w, r, apiCommon.ErrorForbidden(err))
	case http.StatusNotFound:
		apiCommon.RenderError(w, r, apiCommon.ErrorNotFound(err))
	case http.StatusConflict:
		apiCommon.RenderError(w, r, apiCommon.ErrorConflictWithCode(err, schoolSetupCompose.ConflictCode(err)))
	default:
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServer(err))
	}
}
