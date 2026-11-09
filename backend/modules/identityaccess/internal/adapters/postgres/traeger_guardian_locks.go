package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// LockLiveTraegerGuardianAccounts uses the narrow database capability because
// a Träger transaction cannot read the accounts and school memberships itself.
func (*Store) LockLiveTraegerGuardianAccounts(ctx context.Context, organizationID int64, candidates []int64) ([]int64, error) {
	raw, ok := tenant.TransactionFromContext(ctx)
	if !ok || organizationID <= 0 || tenant.TraegerTransactionFromContext(ctx) != organizationID {
		return nil, errors.New("identity access postgres: guardian account locks require this Träger's transaction")
	}
	tx, ok := raw.(bun.Tx)
	if !ok {
		return nil, fmt.Errorf("identity access postgres: unsupported transaction %T", raw)
	}
	var live []int64
	if err := tx.NewRaw(`SELECT * FROM auth.lock_live_traeger_guardian_accounts(?, ?::bigint[])`,
		organizationID, pgdialect.Array(candidates)).Scan(ctx, &live); err != nil {
		return nil, fmt.Errorf("identity access postgres: lock live Träger guardian accounts: %w", err)
	}
	return live, nil
}
