package test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// IdentityContext supplies authenticated identity facts to application tests
// without making domain test packages depend on the JWT adapter.
func IdentityContext(ctx context.Context, accountID, tenantID int64, scope string, permissions []string) context.Context {
	ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{
		ID: int(accountID), TenantID: tenantID, Scope: scope, Permissions: permissions,
	})
	return context.WithValue(ctx, jwt.CtxPermissions, permissions)
}
