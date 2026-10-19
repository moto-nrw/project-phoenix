// Package compose binds the caterer portal workflow to Identity & Access, the
// Meal Plan and the tenant runtime.
package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/catererportal"
	"github.com/moto-nrw/project-phoenix/workflows/catererportal/internal/application"
	"github.com/moto-nrw/project-phoenix/workflows/catererportal/ports"
)

// Dependencies are the owner capabilities the portal coordinates.
type Dependencies struct {
	Caterers    ports.Caterers
	Supplies    ports.Supplies
	SchoolNames ports.SchoolNames
}

// New binds the portal; each school runs in its own tenant transaction.
func New(deps Dependencies) (catererportal.Portal, error) {
	return application.New(application.Dependencies{
		Caterers:      deps.Caterers,
		Supplies:      deps.Supplies,
		SchoolNames:   deps.SchoolNames,
		CurrentSchool: tenant.FromContext,
		WithinSchool: func(ctx context.Context, schoolID int64, fn func(context.Context) error) error {
			id, err := tenant.NewTenantID(schoolID)
			if err != nil {
				return err
			}
			return tenant.WithinTenant(ctx, id, fn)
		},
	})
}
