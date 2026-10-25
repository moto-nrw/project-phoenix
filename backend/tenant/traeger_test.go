package tenant_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTransaction struct{}

func (fakeTransaction) Commit() error   { return nil }
func (fakeTransaction) Rollback() error { return nil }

func traegerRuntime(t *testing.T, opened *[]int64) tenant.UnitOfWork {
	t.Helper()
	uow, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, fakeTransaction{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			return fn(ctx, fakeTransaction{})
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)
	return uow.WithTraegerTransactions(func(ctx context.Context, organizationID int64, fn func(context.Context, any) error) error {
		*opened = append(*opened, organizationID)
		return fn(ctx, fakeTransaction{})
	})
}

// TestWithinTraegerOpensOneUnitWithoutSchool pins the Träger transaction of
// ADR 0086 section 5: it masks any school, joins only its own Träger and
// refuses a foreign ambient unit.
func TestWithinTraegerOpensOneUnitWithoutSchool(t *testing.T) {
	t.Parallel()
	var opened []int64
	ctx := tenant.WithUnitOfWork(tenant.WithTenantID(context.Background(), 9), traegerRuntime(t, &opened))
	const traeger int64 = 4

	err := tenant.WithinTraeger(ctx, traeger, func(txCtx context.Context) error {
		assert.Equal(t, traeger, tenant.TraegerTxOrganization(txCtx))
		assert.Zero(t, tenant.FromContext(txCtx), "a Träger transaction names no school")
		assert.False(t, tenant.IsAdminTx(txCtx), "a Träger transaction is not administrative")
		assert.NoError(t, tenant.RequireTransaction(txCtx), "adapters run in it without a school")
		assert.NoError(t, tenant.WithinTraeger(txCtx, 4, func(context.Context) error { return nil }), "the same Träger joins")
		assert.Error(t, tenant.WithinTraeger(txCtx, 5, func(context.Context) error { return nil }), "another Träger is refused")
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{4}, opened, "one unit for the outer call only")

	id, err := tenant.NewTenantID(9)
	require.NoError(t, err)
	err = tenant.WithinTenant(ctx, id, func(txCtx context.Context) error {
		return tenant.WithinTraeger(txCtx, 4, func(context.Context) error { return nil })
	})
	assert.Error(t, err, "no Träger transaction inside a school transaction")
	assert.ErrorIs(t, tenant.WithinTraeger(ctx, 0, func(context.Context) error { return nil }), tenant.ErrTraegerRequired)
}

func TestWithinTraegerNeedsTheRuntimeFunction(t *testing.T) {
	t.Parallel()
	uow, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, fakeTransaction{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			return fn(ctx, fakeTransaction{})
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)
	err = tenant.WithinTraeger(tenant.WithUnitOfWork(context.Background(), uow), 4, func(context.Context) error { return nil })
	assert.ErrorIs(t, err, tenant.ErrRuntimeRequired, "a runtime without Träger transactions opens none")
}
