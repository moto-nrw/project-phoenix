package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	pwaHTTP "github.com/moto-nrw/project-phoenix/modules/delivery/http/pwa"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

func pwaUsageRouter(usage pwaHTTP.Usage) chi.Router {
	router := chi.NewRouter()
	ProtectedTenantRoutes(router, func(r chi.Router, withTx apiCommon.Middleware) {
		pwaHTTP.NewResource(pwaHTTP.ReportUsage(usage, pwaUsageRuntime), withTx).Register(r)
	})
	return router
}

// pwaUsageRuntime binds the session claims and the shared response
// envelope.
var pwaUsageRuntime = pwaHTTP.UsageRuntime{
	AccountID: func(ctx context.Context) int64 { return int64(sessiontoken.ClaimsFromCtx(ctx).ID) },
	Unauthorized: func(w http.ResponseWriter, r *http.Request) {
		apiCommon.RenderError(w, r, apiCommon.ErrorUnauthorized(sessiontoken.ErrTokenUnauthorized))
	},
	Failed: func(w http.ResponseWriter, r *http.Request, message string, err error) {
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServerWrap(message, err))
	},
	NoContent: func(w http.ResponseWriter, r *http.Request) { apiCommon.Respond(w, r, http.StatusNoContent, nil, "") },
}
