package testutil

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// Claims is the authenticated caller as the router sees it. Aliased here so a
// route test names the claims through the test boundary that mints them,
// rather than reaching for the token package itself.
type Claims = jwt.AppClaims

// WithAuthenticatedContext puts the claims and permissions on a context the
// way the auth middleware does. It is the context-level twin of WithClaims,
// for a handler exercised directly instead of through a request.
func WithAuthenticatedContext(ctx context.Context, claims Claims, permissions []string) context.Context {
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	if permissions != nil {
		ctx = context.WithValue(ctx, jwt.CtxPermissions, permissions)
	}
	return ctx
}
