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

	t.Run("outside a transaction it is the shared database", func(t *testing.T) {
		assert.Same(t, db, resolve(context.Background()))
	})
	t.Run("a transaction value is joined", func(t *testing.T) {
		tx := bun.Tx{}
		assert.Equal(t, bun.IDB(tx), resolve(tenant.WithTransactionForTest(context.Background(), tx)))
	})
	t.Run("a transaction pointer is joined", func(t *testing.T) {
		tx := &bun.Tx{}
		assert.Same(t, tx, resolve(tenant.WithTransactionForTest(context.Background(), tx)))
	})
	t.Run("a transaction it cannot join fails loudly", func(t *testing.T) {
		var missing *bun.Tx
		assert.Panics(t, func() { resolve(tenant.WithTransactionForTest(context.Background(), missing)) })
		assert.Panics(t, func() { resolve(tenant.WithTransactionForTest(context.Background(), struct{}{})) })
	})
}

func TestTenantAmbientDatabaseRequiresADatabase(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { TenantAmbientDatabase(nil) })
}
