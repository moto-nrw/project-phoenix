package common

import (
	"net/http"

	configSvc "github.com/moto-nrw/project-phoenix/services/config"
)

// RequestSettingsCacheMiddleware attaches the request-scoped settings memo
// cache (issue #2065) to every request context. It does no database work —
// it only seeds an empty per-request map — so it is safe to run before
// authorization and to stack: WithSettingsRequestCache is idempotent, letting
// the router-wide attachment in api/base.go and the group-wide attachment in
// ProtectedTenantGroup share one cache.
func RequestSettingsCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(configSvc.WithSettingsRequestCache(r.Context())))
	})
}
