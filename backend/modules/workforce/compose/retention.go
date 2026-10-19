package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// The Workforce deletions of the nightly cleanup. Each runs on the tenant
// transaction of the school in context.

// NewRetiredStaffRecords erases a retired staff member's records (#3963).
func NewRetiredStaffRecords() workforce.RetiredStaffRecords { return postgres.New(retentionDatabase) }

// NewCleanupRetention composes the staff cleanup-job retention of the nightly
// cleanup (#4046). It runs on the tenant transaction of the school in
// context and needs no object store.
func NewCleanupRetention() workforce.CleanupRetention {
	return application.NewCleanupRetention(postgres.New(retentionDatabase))
}

// retentionDatabase is the transaction of the school in context; without one
// the retention refuses to run instead of falling back to a pool.
func retentionDatabase(ctx context.Context) (bun.IDB, postgres.TenantScope, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, postgres.TenantScope{}, fmt.Errorf("workforce postgres: tenant is required: %w", err)
	}
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, postgres.TenantScope{}, errors.New("workforce postgres: transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, postgres.TenantScope{}, fmt.Errorf("workforce postgres: unsupported transaction %T", transaction)
	}
	return tx, postgres.TenantScope{TenantID: tenantID.Int64()}, nil
}

// SickNoteRetentionWindow resolves the staff sick note retention window of
// the school in context; the root binds it to the Settings Platform.
type SickNoteRetentionWindow interface {
	StaffSickNoteRetentionDays(ctx context.Context) (int, error)
}

// NewSickNoteRetention composes the sick note retention of the nightly
// cleanup (#3961). It runs on the tenant transaction of the school in
// context and resolves the school's window through window.
func NewSickNoteRetention(window SickNoteRetentionWindow) workforce.SickNoteRetention {
	if window == nil {
		panic("workforce compose: sick note retention requires settings")
	}
	return application.NewSickNoteRetention(postgres.New(retentionDatabase), window)
}
