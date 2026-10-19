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
	DB(context.Context) bun.IDB
	TenantID(context.Context) int64
	RunInTx(context.Context, func(context.Context) error) error
	MarkRollback(context.Context)
}

// NewLegacyRepositoryRuntime constructs the owner's tenant runtime.
func NewLegacyRepositoryRuntime(db *bun.DB) LegacyRepositoryRuntime {
	return struct {
		groups.RepositoryRuntime
		runtime.Database
	}{RepositoryRuntime: groups.RepositoryRuntime{}, Database: runtime.NewDatabase(db, tenant.TransactionFromContext)}
}
