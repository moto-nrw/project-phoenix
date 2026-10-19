package compose

import (
	"context"

	runtime "github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/adapters/postgres/runtime"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/application/groups"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// LegacyRepositoryRuntime is the tenant capability consumed by the retained bindings.
type LegacyRepositoryRuntime interface {
	runtime.ContextRuntime
	DB(context.Context) bun.IDB
}

// NewLegacyRepositoryRuntime constructs the owner's tenant runtime.
func NewLegacyRepositoryRuntime(db *bun.DB) LegacyRepositoryRuntime {
	return runtime.NewDatabase(groups.RepositoryRuntime{}, db, tenant.TransactionFromContext)
}
