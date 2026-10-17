package compose

import (
	"context"
	"errors"
	"fmt"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
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
func retentionDatabase(ctx context.Context) (bun.IDB, int64, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("workforce postgres: tenant is required: %w", err)
	}
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, 0, errors.New("workforce postgres: transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, 0, fmt.Errorf("workforce postgres: unsupported transaction %T", transaction)
	}
	return tx, tenantID.Int64(), nil
}

// SickNoteRetentionResolver resolves the retention windows of the school in
// context.
type SickNoteRetentionResolver interface {
	ResolveInt(ctx context.Context, key string) (int, error)
}

// NewSickNoteRetention composes the sick note retention of the nightly
// cleanup (#3961). It runs on the tenant transaction of the school in
// context and resolves the school's window itself.
func NewSickNoteRetention(settings SickNoteRetentionResolver) workforce.SickNoteRetention {
	if settings == nil {
		panic("workforce compose: sick note retention requires settings")
	}
	return application.NewSickNoteRetention(
		postgres.New(retentionDatabase), sickNoteRetentionWindow{settings: settings})
}

type sickNoteRetentionWindow struct{ settings SickNoteRetentionResolver }

func (w sickNoteRetentionWindow) StaffSickNoteRetentionDays(ctx context.Context) (int, error) {
	return w.settings.ResolveInt(ctx, configModels.KeyGDPRStaffSickNoteRetentionDays)
}
