//go:build testhooks

package enrollmenthttp

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
)

// scopeTestTenantSession installs the same four native session middleware
// values as the root, without importing the root into its own dependency.
func scopeTestTenantSession(authenticate, scope func(http.Handler) http.Handler, preview func(context.Context) bool, principal common.PrincipalFactsReader) func(chi.Router) {
	return func(r chi.Router) {
		r.Use(authenticate, common.ReadOnlyPreviewGuard(preview), scope, common.PrincipalFromFacts(principal))
	}
}
