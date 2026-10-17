package test

import (
	"context"
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// TenantSessionGates supplies the real authenticator, tenant scope adapter and preview fact
// to route tests that compose a consumer-owned session port without a root cycle.
func TenantSessionGates() (func(http.Handler) http.Handler, func(http.Handler) http.Handler, func(context.Context) bool) {
	return sessiontoken.Authenticator, sessiontoken.TenantMiddleware(tenant.ClaimScope{}), previewSessionFact
}

func previewSessionFact(ctx context.Context) bool {
	claims := sessiontoken.ClaimsFromCtx(ctx)
	return claims.IsReadOnlyPreview()
}
