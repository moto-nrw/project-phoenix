package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// TenantDatabase is the transaction of the school in context; it refuses a
// call for another school.
func RequireTenantDatabase(ctx context.Context, expectedTenantID int64) (bun.IDB, error) {
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("delivery postgres: transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, fmt.Errorf("delivery postgres: unsupported transaction %T", transaction)
	}
	if expectedTenantID == 0 {
		if !tenant.IsAdminTx(ctx) {
			return nil, errors.New("delivery postgres: system email requires an admin transaction")
		}
		return tx, nil
	}
	actualTenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("delivery postgres: tenant is required: %w", err)
	}
	if actualTenantID.Int64() != expectedTenantID {
		return nil, fmt.Errorf("delivery postgres: intent tenant %d does not match transaction tenant %d", expectedTenantID, actualTenantID.Int64())
	}
	return tx, nil
}

// RequireAdminDatabase resolves only an existing privileged transaction.
func RequireAdminDatabase(ctx context.Context) (bun.IDB, error) {
	raw, ok := tenant.TransactionFromContext(ctx)
	if !ok || !tenant.IsAdminTx(ctx) {
		return nil, errors.New("delivery postgres: administrative transaction is required")
	}
	switch tx := raw.(type) {
	case bun.Tx:
		return tx, nil
	case *bun.Tx:
		if tx != nil {
			return tx, nil
		}
	}
	return nil, fmt.Errorf("delivery postgres: unsupported transaction %T", raw)
}

func (s *Store) inAdminTransaction(ctx context.Context, fn func(context.Context, bun.IDB) error) error {
	db, err := s.adminDB(ctx)
	if err != nil {
		return err
	}
	return fn(ctx, db)
}
