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
	runtime.ContextRuntime
	DB(context.Context) bun.IDB
	InTenantTx(context.Context, func(context.Context) error) error
}

// NewLegacyRepositoryRuntime constructs the owner's tenant runtime.
func NewLegacyRepositoryRuntime(db *bun.DB) LegacyRepositoryRuntime {
	return runtime.NewDatabase(application.RepositoryRuntime{}, db, tenant.TransactionFromContext)
}
