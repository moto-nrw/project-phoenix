package careplanhttp

import (
	"net/http"

	configSvc "github.com/moto-nrw/project-phoenix/modules/settings"
)

// scopeTestSettingsCache is the real Settings attachment passed through the
// existing adapter-test public capability, without a fixture import cycle.
func scopeTestSettingsCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(configSvc.WithSettingsRequestCache(r.Context())))
	})
}
