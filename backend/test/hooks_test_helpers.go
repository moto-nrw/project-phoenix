//go:build testhooks

package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/require"
)

// WithAfterCommitHooks queues realtime side effects until the returned commit
// function is called. It keeps service-package tests on the shared test seam
// instead of importing tenant runtime internals directly.
func WithAfterCommitHooks(ctx context.Context) (context.Context, func()) {
	return tenant.WithAfterCommitHooksForTest(ctx)
}

// ContextWithTransaction makes tx the ambient tenant transaction of ctx, so a
// repository test can hold a lock in one transaction and contend in another.
func ContextWithTransaction(ctx context.Context, tx any) context.Context {
	return tenant.WithTransactionForTest(ctx, tx)
}

// MintTestJWT signs a JWT for the given claims using the same configuration as
// production (sessiontoken.NewTokenAuth reads the JWT secret from viper / env). Pair it
// with routetest.WithJWTBearer when calling handlers through Resource.Router() so the
// production auth middleware accepts the request.
//
// Callers must arrange for a non-empty auth_jwt_secret to be present before the
// Resource is constructed (typically via routetest.SeedTestJWTConfig in init() or
// TestMain). Without a secret, jwx refuses to HMAC-sign and this helper fails.
//
// Claims carrying the bootstrap tenant are rebased onto the tenant the test
// owns, so a test that opted into a per-test tenant gets a matching token
// without passing the tenant through every claims helper (#2419).
func MintTestJWT(t testing.TB, claims sessiontoken.AppClaims) string {
	t.Helper()
	claims.TenantID = RebaseTenantID(t, claims.TenantID)
	tokenAuth, err := ConfiguredTokenAuth()
	require.NoError(t, err, "MintTestJWT: NewTokenAuth")
	token, err := tokenAuth.CreateJWT(claims)
	require.NoError(t, err, "MintTestJWT: CreateJWT")
	return token
}

// CreateTestJWT creates a valid JWT access token for the given account ID.
// This token can be used in the Authorization header for authenticated API requests.
func CreateTestJWT(tb testing.TB, accountID int64, permissions []string) string {
	tb.Helper()

	tokenAuth := GetTestTokenAuth(tb)

	claims := sessiontoken.AppClaims{
		ID:          int(accountID),
		Sub:         fmt.Sprintf("%d", accountID), // Required claim - subject identifier
		Roles:       []string{"user"},
		Permissions: permissions,
		TenantID:    fixtureTenantID(tb),
	}

	token, err := tokenAuth.CreateJWT(claims)
	require.NoError(tb, err, "Failed to create test JWT")

	return token
}
