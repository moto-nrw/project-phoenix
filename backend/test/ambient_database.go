package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// AssertAmbientDatabase exercises a persistence resolver against PostgreSQL:
// no pool fallback, the actual transaction handle, and two-school RLS.
func AssertAmbientDatabase(t *testing.T, db *bun.DB, resolve func(context.Context) (bun.IDB, error)) {
	t.Helper()
	ctx := Ctx(t)
	own := CreateTestRoom(t, db, "Ambient scope")
	otherSchool := UniqueTestTenantID(t)
	EnsureTestTenant(t, db, otherSchool)
	foreign := CreateTestRoomForTenant(t, db, otherSchool, "Other ambient scope")
	handle, err := resolve(ctx)
	require.ErrorContains(t, err, "transaction is required")
	require.Nil(t, handle)
	err = WithTenantTx(t, ctx, db, Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		handle, err := resolve(txCtx)
		if err != nil {
			return err
		}
		require.Equal(t, bun.IDB(tx), handle)
		var ids []int64
		if err := handle.NewRaw("SELECT id FROM facilities.rooms WHERE id IN (?, ?) ORDER BY id", own.ID, foreign.ID).Scan(txCtx, &ids); err != nil {
			return err
		}
		require.Equal(t, []int64{own.ID}, ids, "the resolved transaction must enforce school RLS")
		return nil
	})
	require.NoError(t, err)
}
