package test

import (
	"context"
	"errors"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// WithinTraeger runs fn in the Träger transaction of organizationID on the
// unit of work in ctx (ContextWithTenantRuntime). It serves tests whose role
// may not import the tenant runtime, such as the race tests of the
// Betreuungsvertrag's conclusion (#4680).
func WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	return tenant.WithinTraeger(ctx, organizationID, fn)
}

// WithoutRequestRole drops the request role of the transaction in ctx, so
// the rest of it runs as the owner of the tables, without row level
// security. A test of a store's own tenant or Träger filter uses it to see
// that filter beneath RLS (#4679).
func WithoutRequestRole(ctx context.Context) error {
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return errors.New("test: a transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return errors.New("test: unsupported transaction")
	}
	_, err := tx.ExecContext(ctx, "RESET ROLE")
	return err
}
