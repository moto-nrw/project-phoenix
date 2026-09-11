package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	pwaAPI "github.com/moto-nrw/project-phoenix/api/pwa"
	projectJWT "github.com/moto-nrw/project-phoenix/auth/jwt"
)

func (a *API) pwaUsageRouter() chi.Router {
	router := chi.NewRouter()
	apiCommon.ProtectedTenantGroup(router, a.db, func(r chi.Router, withTx apiCommon.Middleware) {
		pwaAPI.NewResource(http.HandlerFunc(a.reportPWAUsage), withTx).Register(r)
	})
	return router
}

// reportPWAUsage records that the caller's session runs in standalone display
// mode. Repeated reports only advance last_seen_at.
func (a *API) reportPWAUsage(w http.ResponseWriter, r *http.Request) {
	claims := projectJWT.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		apiCommon.RenderError(w, r, apiCommon.ErrorUnauthorized(projectJWT.ErrTokenUnauthorized))
		return
	}
	if err := a.Services.PWAUsage.ReportStaff(r.Context(), int64(claims.ID)); err != nil {
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServerWrap("App-Nutzung konnte nicht gespeichert werden.", err))
		return
	}
	apiCommon.Respond(w, r, http.StatusNoContent, nil, "")
}
