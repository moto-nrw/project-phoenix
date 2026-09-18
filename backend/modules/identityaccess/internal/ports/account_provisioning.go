package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// SchoolAccountStore is the persistence port over the auth.accounts row and
// the auth.account_tenants mapping an admin- or operator-led account
// creation writes (#3332). Every statement runs on the connection the
// caller's context carries; the flows hold the school's transaction.
type SchoolAccountStore interface {
	// InsertSchoolAccount creates the account of a registration. Its
	// username and its first-login stamp belong to this path only, which is
	// why it does not share the invitation's InsertAccount.
	InsertSchoolAccount(ctx context.Context, account domain.NewSchoolAccount) (domain.RegisteredAccount, domain.OperationStats, error)
	// FindLoginAccountByUsername resolves the account holding that username,
	// matched case-insensitively and across every school, so a registration
	// cannot claim a name another school already uses.
	FindLoginAccountByUsername(ctx context.Context, username string) (domain.LoginAccount, bool, domain.OperationStats, error)
	// InsertTenantMappingIfAbsent maps the account to the school and leaves
	// an existing mapping untouched: a membership an offboarding
	// deactivated stays deactivated, which is what keeps that offboarding in
	// force when the same account is linked again.
	InsertTenantMappingIfAbsent(ctx context.Context, accountID, tenantID int64) (domain.OperationStats, error)
}
