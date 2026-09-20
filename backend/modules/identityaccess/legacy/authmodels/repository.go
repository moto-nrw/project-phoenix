package authmodels

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// AccountRepository defines operations for managing accounts
type AccountRepository interface {
	base.CRUDRepository[*Account]
	FindManageableByID(ctx context.Context, id int64) (*Account, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*Account, error)
	FindByEmail(ctx context.Context, email string) (*Account, error)
	// AnonymizeForDeletion overwrites the email with an anonymized
	// placeholder and clears the username (GDPR person deletion).
	AnonymizeForDeletion(ctx context.Context, accountID int64, anonymizedEmail string) error
}

// AccountTenantRepository defines operations for querying account-tenant mappings.
type AccountTenantRepository interface {
	Create(ctx context.Context, mapping *AccountTenant) error
	EnsureActive(ctx context.Context, mapping *AccountTenant) error
	ExistsByAccountAndTenant(ctx context.Context, accountID, tenantID int64) (bool, error)
}
