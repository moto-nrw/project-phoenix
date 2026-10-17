package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	pwaHTTP "github.com/moto-nrw/project-phoenix/modules/delivery/http/pwa"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// staffPWAUsage records a staff session that runs in standalone display mode.
type staffPWAUsage interface {
	ReportStaff(ctx context.Context, accountID int64) error
}

func pwaUsageRouter(usage staffPWAUsage) chi.Router {
	router := chi.NewRouter()
	ProtectedTenantRoutes(router, func(r chi.Router, withTx apiCommon.Middleware) {
		pwaHTTP.NewResource(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reportPWAUsage(w, r, usage)
		}), withTx).Register(r)
	})
	return router
}

// reportPWAUsage records that the caller's session runs in standalone display
// mode. Repeated reports only advance last_seen_at.
func reportPWAUsage(w http.ResponseWriter, r *http.Request, usage staffPWAUsage) {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		apiCommon.RenderError(w, r, apiCommon.ErrorUnauthorized(sessiontoken.ErrTokenUnauthorized))
		return
	}
	if err := usage.ReportStaff(r.Context(), int64(claims.ID)); err != nil {
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServerWrap("App-Nutzung konnte nicht gespeichert werden.", err))
		return
	}
	apiCommon.Respond(w, r, http.StatusNoContent, nil, "")
}
