package behavior_test

import (
	"context"
	"database/sql"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// A PostgreSQL error contains a map and is not comparable. It must leave the
// public caller capability as an error, so consumers can apply their fallback.
func TestCallerStaffPreservesPostgresFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Caller", "DatabaseError")
	caller := setupCallerRows(t, db).Caller()
	for _, table := range []string{"users.persons", "users.staff_school_memberships"} {
		t.Run(table, func(t *testing.T) {
			ctx := testpkg.WithRequestIdentityCache(callerCtx(t, account.ID))
			blocker, err := db.BeginTx(ctx, &sql.TxOptions{})
			require.NoError(t, err)
			defer func() { require.NoError(t, blocker.Rollback()) }()
			_, err = blocker.ExecContext(ctx, "LOCK TABLE "+table+" IN ACCESS EXCLUSIVE MODE")
			require.NoError(t, err)
			err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
				_, err := tx.ExecContext(txCtx, "SET LOCAL lock_timeout = '10ms'")
				require.NoError(t, err)
				_, err = caller.StaffID(txCtx)
				return err
			})
			require.ErrorContains(t, err, "canceling statement due to lock timeout")
		})
	}
}
