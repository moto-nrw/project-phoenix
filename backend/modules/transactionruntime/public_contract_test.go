package transactionruntime_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestPublicTransactionContractClassifiesRealWrappedConstraintFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	err := testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `CREATE TEMP TABLE transaction_contract_probe (id integer CONSTRAINT transaction_contract_key UNIQUE) ON COMMIT DROP`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO transaction_contract_probe VALUES (1), (1)`)
		return err
	})
	require.Error(t, err)
	wrapped := fmt.Errorf("command failed: %w", err)
	require.True(t, transactionruntime.IsUniqueViolation(wrapped))
	require.True(t, transactionruntime.IsUniqueViolationOn(wrapped, "transaction_contract_key"))
	require.False(t, transactionruntime.IsUniqueViolationOn(wrapped, "different_constraint"))
	require.False(t, transactionruntime.IsTransientDatabaseError(wrapped))
}
