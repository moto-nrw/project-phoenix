package owners

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParentPortalRuntimePreservesTransactionLifecycle(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, rollback := range []bool{false, true} {
		name := "commit"
		if rollback {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := testpkg.WithPackageTenantRuntime(testpkg.OwnCtx(t))
			tenantID := testpkg.Tenant(t)
			runtime := ParentPortalRuntime{}
			failure := errors.New("outer transaction refused")
			var calls []string
			err := runtime.WithinTenant(ctx, tenantID, func(outer context.Context) error {
				assert.Equal(t, tenantID, tenant.FromContext(outer))
				outerTx, ok := tenant.TransactionFromContext(outer)
				require.True(t, ok)
				runtime.RegisterAfterCommit(outer, func() { calls = append(calls, "outer") })
				require.NoError(t, runtime.WithinTenant(outer, tenantID, func(inner context.Context) error {
					assert.Equal(t, tenantID, tenant.FromContext(inner))
					innerTx, exists := tenant.TransactionFromContext(inner)
					require.True(t, exists)
					assert.Equal(t, outerTx, innerTx)
					runtime.RegisterAfterCommit(inner, func() { calls = append(calls, "inner") })
					return nil
				}))
				assert.Empty(t, calls, "nested success must not publish before the outer commit")
				if rollback {
					return failure
				}
				return nil
			})
			if rollback {
				assert.ErrorIs(t, err, failure)
				assert.Empty(t, calls, "rollback must discard both callbacks")
			} else {
				require.NoError(t, err)
				assert.Equal(t, []string{"outer", "inner"}, calls)
			}
		})
	}
}

func TestParentPortalRuntimePreservesAdministrativeContext(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	ctx := testpkg.WithPackageTenantRuntime(testpkg.Ctx(t))
	runtime := ParentPortalRuntime{}
	called := false
	failure := errors.New("administrative transaction refused")
	err := runtime.WithinAdmin(ctx, func(admin context.Context) error {
		assert.True(t, tenant.IsAdminTx(admin))
		assert.Zero(t, tenant.FromContext(admin), "admin work must mask an inherited school")
		bound := runtime.WithTenantID(admin, testpkg.Tenant(t))
		assert.Equal(t, testpkg.Tenant(t), tenant.FromContext(bound))
		assert.True(t, tenant.IsAdminTx(bound))
		runtime.RegisterAfterCommit(admin, func() { called = true })
		assert.False(t, called)
		return failure
	})
	assert.ErrorIs(t, err, failure)
	assert.False(t, called, "an admin rollback must not publish")

	require.NoError(t, runtime.WithinAdmin(ctx, func(admin context.Context) error {
		runtime.RegisterAfterCommit(admin, func() { called = true })
		assert.False(t, called)
		return nil
	}))
	assert.True(t, called)
}

func TestParentPortalRuntimePreservesValidationAndMissingRuntime(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	runtime := ParentPortalRuntime{}
	called := false
	callback := func(context.Context) error { called = true; return nil }
	for _, id := range []int64{0, -1} {
		assert.ErrorIs(t, runtime.WithinTenant(context.Background(), id, callback), tenant.ErrInvalidTenantID)
	}
	assert.ErrorIs(t, runtime.WithinTenant(context.Background(), testpkg.Tenant(t), callback), tenant.ErrRuntimeRequired)
	assert.ErrorIs(t, runtime.WithinAdmin(context.Background(), callback), tenant.ErrRuntimeRequired)
	assert.False(t, called)
	assert.PanicsWithError(t, "tenant ID must be positive: 0", func() {
		runtime.WithTenantID(context.Background(), 0)
	})
	runtime.RegisterAfterCommit(context.Background(), func() { called = true })
	assert.True(t, called, "outside a transaction callbacks retain synchronous behavior")
	runtime.RegisterAfterCommit(context.Background(), nil)
}
