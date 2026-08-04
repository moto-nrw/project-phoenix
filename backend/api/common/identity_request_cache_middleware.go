package common

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/services/usercontext"
)

// RequestIdentityCacheMiddleware attaches the request-scoped identity memo
// cache (issue #2099) to every request context. It does no database work —
// it only seeds an empty per-request map — so it is safe to run before
// authorization and to stack: WithIdentityRequestCache is idempotent, letting
// the router-wide attachment in api/base.go and the group-wide attachment in
// ProtectedTenantGroup share one cache. The consistency contract lives on the
// cache struct in services/usercontext/identity_request_cache.go.
func RequestIdentityCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(usercontext.WithIdentityRequestCache(r.Context())))
	})
}
