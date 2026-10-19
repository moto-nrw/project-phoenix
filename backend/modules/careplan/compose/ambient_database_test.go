package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

func TestTenantAmbientDatabaseJoinsTheCallersTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	resolve := TenantAmbientDatabase(db)

	t.Run("outside a transaction it fails", func(t *testing.T) {
		resolved, err := resolve(testpkg.Ctx(t))
		assert.Nil(t, resolved)
		assert.ErrorContains(t, err, "transaction is required")
	})
	t.Run("an ambient PostgreSQL transaction is joined", func(t *testing.T) {
		err := testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
			resolved, err := resolve(ctx)
			assert.NoError(t, err)
			assert.Equal(t, bun.IDB(tx), resolved)
			var one int
			return resolved.NewRaw("SELECT 1").Scan(ctx, &one)
		})
		assert.NoError(t, err)
	})
	t.Run("unusable transaction values return errors", func(t *testing.T) {
		var missing *bun.Tx
		for _, raw := range []any{missing, struct{}{}} {
			resolved, err := resolve(tenant.WithTransactionForTest(context.Background(), raw))
			assert.Nil(t, resolved)
			assert.Error(t, err)
		}
	})
}

func TestTenantAmbientDatabaseRequiresADatabase(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { TenantAmbientDatabase(nil) })
}
