package authmodels

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// AccountRepository defines operations for managing accounts
type AccountRepository interface {
	base.CRUDRepository[*Account]
	FindByEmail(ctx context.Context, email string) (*Account, error)
}

// AccountTenantRepository defines operations for querying account-tenant mappings.
type AccountTenantRepository interface {
	Create(ctx context.Context, mapping *AccountTenant) error
	EnsureActive(ctx context.Context, mapping *AccountTenant) error
	ExistsByAccountAndTenant(ctx context.Context, accountID, tenantID int64) (bool, error)
}
