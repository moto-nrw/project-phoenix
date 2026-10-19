package postgres

import (
	context "context"
	fmt "fmt"

	tenant "github.com/moto-nrw/project-phoenix/tenant"
	bun "github.com/uptrace/bun"
)

func RequireBunTransaction(ctx context.Context) error {
	raw, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return fmt.Errorf("disable caregiver capability: unit of work did not provide a transaction")
	}
	if _, ok := raw.(bun.Tx); !ok {
		return fmt.Errorf("disable caregiver capability: unsupported transaction type %T", raw)
	}
	return nil
}
