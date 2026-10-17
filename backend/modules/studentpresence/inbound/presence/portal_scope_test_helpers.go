//go:build testhooks

package presence

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
)

// scopeTestTenantRoutes binds real native gates supplied by the shared
// fixture catalog without adding an internal-test import of the HTTP runtime.
func scopeTestTenantRoutes(authenticate, scope func(http.Handler) http.Handler, preview func(context.Context) bool, principal common.PrincipalFactsReader, settingsCache, identityCache func(http.Handler) http.Handler) func(chi.Router, func(chi.Router, func(http.Handler) http.Handler)) {
	return func(r chi.Router, register func(chi.Router, func(http.Handler) http.Handler)) {
		common.ProtectedTenantRoutes(r, register, authenticate, common.ReadOnlyPreviewGuard(preview), scope, common.PrincipalFromFacts(principal), settingsCache, identityCache)
	}
}
