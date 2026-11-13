package careplanhttp_test

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	authz "github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"

	careplanhttp "github.com/moto-ogs/project-phoenix/modules/careplan/http"
)

// The principal's account and tenant when a request carries no claims of its
// own. The route gates read only its permissions, and the handlers under test
// read the session, not the principal.
const (
	routeTestAccountID int64 = 9001
	routeTestTenantID  int64 = 9002
)

// serveRoute drives req through the routes the resource registers at the
// students mount (#6914). The route gate checks the request's session
// permissions plus gate: a suite that pins a handler without session
// permissions passes the permission its route requires, and the handler still
// sees the session it set up.
func serveRoute(t *testing.T, rs *careplanhttp.Resource, w http.ResponseWriter, req *http.Request, gate ...string) {
	t.Helper()
	router := chi.NewRouter()
	router.Route("/students", func(r chi.Router) {
		rs.Register(r, func(next http.Handler) http.Handler { return next })
	})

	ctx := req.Context()
	claims := sessiontoken.ClaimsFromCtx(ctx)
	accountID, tenantID := int64(claims.ID), claims.TenantID
	if accountID <= 0 {
		accountID = routeTestAccountID
	}
	if tenantID <= 0 {
		tenantID = routeTestTenantID
	}
	principal, err := authz.NewPrincipal(authz.PrincipalInput{
		AccountID: accountID, TenantID: tenantID, Permissions: slices.Concat(sessiontoken.PermissionsFromCtx(ctx), gate),
	})
	require.NoError(t, err)
	ctx = authz.WithPrincipal(ctx, principal)
	// The router binds the path parameters itself.
	ctx = context.WithValue(ctx, chi.RouteCtxKey, (*chi.Context)(nil))
	router.ServeHTTP(w, req.WithContext(ctx))
}
