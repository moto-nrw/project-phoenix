package tenant_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
)

func TestWithTenantID_RoundTrip(t *testing.T) {
	ctx := context.Background()
	ctx = tenant.WithTenantID(ctx, 42)

	got := tenant.FromContext(ctx)
	assert.Equal(t, int64(42), got)
}

func TestFromContext_EmptyContext(t *testing.T) {
	got := tenant.FromContext(context.Background())
	assert.Equal(t, int64(0), got)
}

func TestWithOrgID_RoundTrip(t *testing.T) {
	ctx := context.Background()
	ctx = tenant.WithOrgID(ctx, 77)

	got := tenant.OrgFromContext(ctx)
	assert.Equal(t, int64(77), got)
}

func TestOrgFromContext_EmptyContext(t *testing.T) {
	got := tenant.OrgFromContext(context.Background())
	assert.Equal(t, int64(0), got)
}

func TestWithScope_RoundTrip(t *testing.T) {
	ctx := context.Background()
	ctx = tenant.WithScope(ctx, tenant.ScopePlatform)

	got := tenant.ScopeFromContext(ctx)
	assert.Equal(t, tenant.ScopePlatform, got)
}

func TestScopeFromContext_EmptyContext(t *testing.T) {
	got := tenant.ScopeFromContext(context.Background())
	assert.Equal(t, "", got)
}

func TestScopeTenantIsEmptyString(t *testing.T) {
	// Per architecture plan: "" is the default tenant scope.
	// Existing JWTs have no scope field, which parses as "".
	assert.Equal(t, "", tenant.ScopeTenant)
}

func TestMultipleContextValues(t *testing.T) {
	ctx := context.Background()
	ctx = tenant.WithTenantID(ctx, 10)
	ctx = tenant.WithOrgID(ctx, 20)
	ctx = tenant.WithScope(ctx, tenant.ScopePlatform)

	assert.Equal(t, int64(10), tenant.FromContext(ctx))
	assert.Equal(t, int64(20), tenant.OrgFromContext(ctx))
	assert.Equal(t, tenant.ScopePlatform, tenant.ScopeFromContext(ctx))
}
