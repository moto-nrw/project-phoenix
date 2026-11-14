package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/auditlog/audittest"
	"github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The Audit adapters read the tenant and transaction of a unit of work from
// their own handoff on the context. Both runtimes the seam composes must set
// it inside a tenant transaction, clear it for administrative work, and mask
// the transaction when a callback detaches from it.
func TestTenantRuntimeHandsTheUnitOfWorkToTheAuditAdapters(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)

	for name, build := range map[string]func(*bun.DB) (tenant.UnitOfWork, error){
		"tenant": compose.NewTenantRuntime,
		"demo":   compose.NewDemoTenantRuntime,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runtime, err := build(db)
			require.NoError(t, err)
			ctx := tenant.WithUnitOfWork(context.Background(), runtime)

			require.NoError(t, tenant.WithinCurrentTenant(tenant.WithTenantID(ctx, tenantID), func(txCtx context.Context) error {
				assert.Equal(t, tenantID, audittest.TenantIDFromContext(txCtx))
				transaction, ok := audittest.TransactionFromContext(txCtx)
				require.True(t, ok, "the Audit adapters see the tenant transaction")
				assert.IsType(t, bun.Tx{}, transaction)

				_, ok = audittest.TransactionFromContext(tenant.ContextWithoutTransaction(txCtx))
				assert.False(t, ok, "detaching masks the transaction for the Audit adapters")
				return nil
			}))

			require.NoError(t, tenant.WithinAdmin(tenant.WithTenantID(ctx, tenantID), func(txCtx context.Context) error {
				assert.Zero(t, audittest.TenantIDFromContext(txCtx), "administrative work carries no Audit tenant")
				_, ok := audittest.TransactionFromContext(txCtx)
				assert.True(t, ok, "the Audit adapters see the administrative transaction")
				return nil
			}))
		})
	}
}

func TestTenantRuntimeRequiresADatabase(t *testing.T) {
	t.Parallel()
	_, err := compose.NewTenantRuntime(nil)
	require.Error(t, err)
	_, err = compose.NewDemoTenantRuntime(nil)
	require.Error(t, err)
}

// WithLockTimeout bounds the lock waits of its callback only: the setting is
// restored afterwards, and a wait longer than the bound fails with
// ErrLockWaitExceeded (#6703).
func TestTenantRuntimeBoundsLockWaits(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	runtime, err := compose.NewTenantRuntime(db)
	require.NoError(t, err)
	ctx := tenant.WithTenantID(tenant.WithUnitOfWork(context.Background(), runtime), testpkg.Tenant(t))
	key := t.Name()
	setting := func(ctx context.Context) string {
		transaction, _ := tenant.TransactionFromContext(ctx)
		var value string
		require.NoError(t, transaction.(bun.Tx).NewRaw(`SELECT current_setting('lock_timeout')`).Scan(ctx, &value))
		return value
	}

	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		before := setting(txCtx)
		require.NoError(t, tenant.WithLockTimeout(txCtx, 250*time.Millisecond, func(boundCtx context.Context) error {
			assert.Equal(t, "250ms", setting(boundCtx))
			return nil
		}))
		assert.Equal(t, before, setting(txCtx), "the bound ends with its callback")
		return nil
	}))

	holder, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	_, err = holder.ExecContext(context.Background(), `SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`, key)
	require.NoError(t, err)
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		return tenant.WithLockTimeout(txCtx, 50*time.Millisecond, func(boundCtx context.Context) error {
			return tenant.AcquireLock(boundCtx, key, false)
		})
	})
	require.ErrorIs(t, err, tenant.ErrLockWaitExceeded)
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(context.Context) error { return nil }), "the runtime stays usable")
}
