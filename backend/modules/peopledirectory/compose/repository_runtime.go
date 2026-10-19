package compose

import (
	"context"

	runtime "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/postgres/runtime"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// LegacyRepositoryRuntime is the tenant capability consumed by the retained bindings.
type LegacyRepositoryRuntime interface {
	DB(context.Context) bun.IDB
	TenantID(context.Context) int64
	RequireTenantID(context.Context) (int64, error)
	InTransaction(context.Context) bool
	RunInTx(context.Context, func(context.Context) error) error
	WithSavepoint(context.Context, func(context.Context) error) error
	WithinCurrentTenant(context.Context, func(context.Context) error) error
	InTenantTx(context.Context, func(context.Context) error) error
	MarkRollback(context.Context)
}

// NewLegacyRepositoryRuntime constructs the owner's tenant runtime.
func NewLegacyRepositoryRuntime(db *bun.DB) LegacyRepositoryRuntime {
	return struct {
		application.RepositoryRuntime
		runtime.Database
	}{RepositoryRuntime: application.RepositoryRuntime{}, Database: runtime.NewDatabase(db, tenant.TransactionFromContext, tenant.FromContext, tenant.WithTenantTx[*bun.DB, bun.Tx])}
}
