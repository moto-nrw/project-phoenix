package test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// AssertTenantRowIsolation checks database RLS without application tenant filters.
// Both rows must already exist in their respective tenant contexts.
func AssertTenantRowIsolation(tb testing.TB, db *bun.DB, ownCtx, foreignCtx context.Context, table string, ownID, foreignID int64) {
	tb.Helper()
	require.NoError(tb, tenant.WithTenantTx(ownCtx, db, tenant.FromContext(ownCtx), func(ctx context.Context, tx bun.Tx) error {
		var bypass bool
		require.NoError(tb, tx.NewRaw("SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(ctx, &bypass))
		require.False(tb, bypass, "RLS must be exercised by a restricted role")
		var ids []int64
		require.NoError(tb, tx.NewSelect().Table(table).Column("id").Where("id IN (?, ?)", ownID, foreignID).Scan(ctx, &ids))
		require.Equal(tb, []int64{ownID}, ids, "RLS must hide foreign rows without an explicit tenant filter")
		result, err := tx.NewUpdate().Table(table).Set("updated_at = updated_at").Where("id = ?", foreignID).Exec(ctx)
		require.NoError(tb, err)
		changed, err := result.RowsAffected()
		require.NoError(tb, err)
		assert.Zero(tb, changed)
		result, err = tx.NewDelete().Table(table).Where("id = ?", foreignID).Exec(ctx)
		require.NoError(tb, err)
		changed, err = result.RowsAffected()
		require.NoError(tb, err)
		assert.Zero(tb, changed)
		return nil
	}))
	require.NoError(tb, tenant.WithTenantTx(foreignCtx, db, tenant.FromContext(foreignCtx), func(ctx context.Context, tx bun.Tx) error {
		count, err := tx.NewSelect().Table(table).Where("id = ?", foreignID).Count(ctx)
		require.NoError(tb, err)
		assert.Equal(tb, 1, count, "foreign row must survive attempted mutation")
		return nil
	}))
}
