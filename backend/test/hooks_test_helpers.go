//go:build testhooks

package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
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

// MintLiveSessionJWT mints an access token backed by a live refresh-token
// family of its account and school. The production router refuses an access
// token whose family is gone (#5194); tests that drive it need this one.
func MintLiveSessionJWT(t testing.TB, db *bun.DB, claims sessiontoken.AppClaims) string {
	t.Helper()
	claims.TenantID = RebaseTenantID(t, claims.TenantID)
	// Parent and caterer tokens name no school; their session row has one.
	school := claims.TenantID
	if school == 0 {
		school = fixtureTenantID(t)
	}
	claims.FamilyID = CreateTestSessionFamily(t, db, int64(claims.ID), school, claims.Scope)
	return MintTestJWT(t, claims)
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
