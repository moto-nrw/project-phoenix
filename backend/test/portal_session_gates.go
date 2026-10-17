package test

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// TenantSessionGates supplies the real authenticator and tenant scope adapter
// to route tests that compose a consumer-owned session port without a root cycle.
func TenantSessionGates() (func(http.Handler) http.Handler, func(http.Handler) http.Handler) {
	return sessiontoken.Authenticator, sessiontoken.TenantMiddleware(tenant.ClaimScope{})
}
