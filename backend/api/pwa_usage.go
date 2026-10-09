package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	pwaAPI "github.com/moto-nrw/project-phoenix/api/pwa"
	projectJWT "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// staffPWAUsage records a staff session that runs in standalone display mode.
type staffPWAUsage interface {
	ReportStaff(ctx context.Context, accountID int64) error
}

func pwaUsageRouter(usage staffPWAUsage) chi.Router {
	router := chi.NewRouter()
	apiCommon.ProtectedTenantRoutes(router, func(r chi.Router, withTx apiCommon.Middleware) {
		pwaAPI.NewResource(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reportPWAUsage(w, r, usage)
		}), withTx).Register(r)
	})
	return router
}

// reportPWAUsage records that the caller's session runs in standalone display
// mode. Repeated reports only advance last_seen_at.
func reportPWAUsage(w http.ResponseWriter, r *http.Request, usage staffPWAUsage) {
	claims := projectJWT.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		apiCommon.RenderError(w, r, apiCommon.ErrorUnauthorized(projectJWT.ErrTokenUnauthorized))
		return
	}
	if err := usage.ReportStaff(r.Context(), int64(claims.ID)); err != nil {
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServerWrap("App-Nutzung konnte nicht gespeichert werden.", err))
		return
	}
	apiCommon.Respond(w, r, http.StatusNoContent, nil, "")
}
