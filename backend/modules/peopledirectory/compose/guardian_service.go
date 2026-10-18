package compose

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewGuardianService composes the guardian service over the retained
// repositories in deps and the tenant transactions of db.
func NewGuardianService(deps peoplePorts.GuardianServiceDependencies, db *bun.DB) peoplePorts.GuardianService {
	return application.NewRetainedGuardianService(deps, guardianTransactions{db: db})
}

// guardianTransactions binds the guardian service's transactions to the
// tenant runtime and the database.
type guardianTransactions struct{ db *bun.DB }

func (guardianTransactions) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, fn)
}

func (t guardianTransactions) WithTenantTx(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithTenantTx(ctx, t.db, tenant.FromContext(ctx), func(ctx context.Context, _ bun.Tx) error {
		return fn(ctx)
	})
}

func (guardianTransactions) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

// ApplyDefaultGuardianRole gives a relationship the default preset its
// relationship type, primary, emergency and pickup facts select, with the
// permissions that preset grants.
func ApplyDefaultGuardianRole(presets peoplePorts.GuardianRolePresets, link *peopleValues.GuardianRelationshipSnapshot) {
	application.ApplyDefaultGuardianRole(presets, link)
}
