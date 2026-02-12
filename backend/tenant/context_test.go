package tenant_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestMustFromContext_Panics(t *testing.T) {
	assert.Panics(t, func() {
		tenant.MustFromContext(context.Background())
	})
}

func TestMustFromContext_PanicsOnZero(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), 0)
	assert.Panics(t, func() {
		tenant.MustFromContext(ctx)
	})
}

func TestMustFromContext_Success(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), 99)
	require.NotPanics(t, func() {
		got := tenant.MustFromContext(ctx)
		assert.Equal(t, int64(99), got)
	})
}

func TestWithOrgID_RoundTrip(t *testing.T) {
	ctx := context.Background()
	ctx = tenant.WithOrgID(ctx, 7)

	got := tenant.OrgFromContext(ctx)
	assert.Equal(t, int64(7), got)
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

func TestIsPlatformScope(t *testing.T) {
	tests := []struct {
		name     string
		scope    string
		expected bool
	}{
		{"platform scope", tenant.ScopePlatform, true},
		{"tenant scope", tenant.ScopeTenant, false},
		{"empty scope", "", false},
		{"unknown scope", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tenant.WithScope(context.Background(), tt.scope)
			assert.Equal(t, tt.expected, tenant.IsPlatformScope(ctx))
		})
	}
}

func TestMultipleContextValues(t *testing.T) {
	ctx := context.Background()
	ctx = tenant.WithTenantID(ctx, 10)
	ctx = tenant.WithOrgID(ctx, 20)
	ctx = tenant.WithScope(ctx, tenant.ScopeTenant)

	assert.Equal(t, int64(10), tenant.FromContext(ctx))
	assert.Equal(t, int64(20), tenant.OrgFromContext(ctx))
	assert.Equal(t, tenant.ScopeTenant, tenant.ScopeFromContext(ctx))
	assert.False(t, tenant.IsPlatformScope(ctx))
}
