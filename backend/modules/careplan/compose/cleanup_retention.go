package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewCleanupRetention composes the Care Plan retention of finished records in
// the nightly cleanup (#4046). It runs on the tenant transaction of the
// school in context and needs no other owner.
func NewCleanupRetention() careplan.CleanupRetention {
	return cleanupRetention{store: postgres.New(retentionDatabase), now: time.Now}
}

// cleanupRetention binds each deletion to its window; the store deletes what
// finished before the cutoff.
type cleanupRetention struct {
	store *postgres.Store
	now   func() time.Time
}

func (r cleanupRetention) DeleteFinishedStudentDocumentCleanups(ctx context.Context) (int, int, error) {
	return r.deleteFinished(ctx, domain.StudentDocumentCleanupRetentionWindow, r.store.DeleteFinishedStudentDocumentFileCleanups)
}

func (r cleanupRetention) DeleteFinishedWithdrawalCompletions(ctx context.Context) (int, int, error) {
	return r.deleteFinished(ctx, domain.WithdrawalCompletionRetentionWindow, r.store.DeleteFinishedWithdrawalCompletions)
}

func (r cleanupRetention) deleteFinished(ctx context.Context, window time.Duration, deleteBefore func(context.Context, time.Time) (int, error)) (int, int, error) {
	deleted, err := deleteBefore(ctx, r.now().Add(-window))
	if err != nil {
		return 0, 0, err
	}
	return deleted, int(window.Hours() / 24), nil
}

// retentionDatabase is the transaction of the school in context; without one
// the retention refuses to run instead of falling back to a pool.
func retentionDatabase(ctx context.Context) (bun.IDB, int64, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("care plan postgres: tenant is required: %w", err)
	}
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, 0, errors.New("care plan postgres: transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, 0, fmt.Errorf("care plan postgres: unsupported transaction %T", transaction)
	}
	return tx, tenantID.Int64(), nil
}
