package api

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/observability"
)

func AuthorizationObserverMiddleware(observer securityruntime.AuthorizationObserver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := securityruntime.WithAuthorizationObserver(r.Context(), observer)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func recordAuthorizationEvent(event securityruntime.AuthorizationEvent) {
	observability.RecordAuthorizationEvent(event.Outcome, event.Reason, event.Elapsed)
}
