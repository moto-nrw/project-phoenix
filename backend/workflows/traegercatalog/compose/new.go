// Package compose binds the Träger catalog rollout (ADR 0195) to the tenant
// runtime; the root binds its owner ports.
package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/traegercatalog"
	"github.com/moto-nrw/project-phoenix/workflows/traegercatalog/internal/application"
	"github.com/moto-nrw/project-phoenix/workflows/traegercatalog/ports"
)

// New builds the rollout over the owner ports; it opens the Träger
// transaction and each school's tenant transaction itself, never nested.
func New(deps ports.Dependencies) (traegercatalog.Catalog, error) {
	deps.Runtime, deps.Today = runtime{}, calendar.TodayDate
	return application.New(deps)
}

type runtime struct{}

func (runtime) WithinExclusive(ctx context.Context, key string, fn func(context.Context) error) error {
	return tenant.WithinExclusive(ctx, key, fn)
}

func (runtime) WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	return tenant.WithinTraeger(ctx, organizationID, fn)
}

func (runtime) WithinSchool(ctx context.Context, schoolID int64, fn func(context.Context) error) error {
	id, err := tenant.NewTenantID(schoolID)
	if err != nil {
		return err
	}
	return tenant.WithinTenant(ctx, id, fn)
}
