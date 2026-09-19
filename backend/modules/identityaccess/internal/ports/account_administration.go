package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// AccountAdministrationStore is the persistence port over the auth.accounts
// row the account administration reads and writes (#3332). Every read and
// every write that an administrator drives carries the caller's visibility
// predicate in the same statement, so the boundary cannot be lost between a
// read and the write it authorizes. Statements run on the connection the
// caller's context carries.
type AccountAdministrationStore interface {
	// FindAccountRecord reads the account without the administration
	// predicate. It serves the caller's own account and the credential
	// verification of a password change, both of which are the account
	// holder's own business.
	FindAccountRecord(ctx context.Context, id int64) (domain.ManagedAccountRecord, bool, domain.OperationStats, error)
	// FindManageableAccountRecord reads one account within the caller's
	// visibility.
	FindManageableAccountRecord(ctx context.Context, id int64, visibility domain.AccountVisibility) (domain.ManagedAccountRecord, bool, domain.OperationStats, error)
	// ListManageableAccountRecords lists the accounts within the caller's
	// visibility that match the filter, ascending by account id.
	ListManageableAccountRecords(ctx context.Context, filter domain.AccountListFilter, visibility domain.AccountVisibility) ([]domain.ManagedAccountRecord, domain.OperationStats, error)
	// ListManageableAccountRecordsByRole lists the accounts within the
	// caller's visibility that hold the named role at a school the caller
	// may administer.
	ListManageableAccountRecordsByRole(ctx context.Context, roleName string, visibility domain.AccountVisibility) ([]domain.ManagedAccountRecord, domain.OperationStats, error)
	// UpdateManageableAccountIdentity writes the address and, when set, the
	// name. found=false when the predicate matched no row.
	UpdateManageableAccountIdentity(ctx context.Context, update domain.AccountIdentityUpdate, visibility domain.AccountVisibility) (bool, domain.OperationStats, error)
	// SetManageableAccountActive flips the active flag within the caller's
	// visibility. found=false when the predicate matched no row.
	SetManageableAccountActive(ctx context.Context, id int64, active bool, visibility domain.AccountVisibility) (bool, domain.OperationStats, error)
	// UpdateAccountPasswordHash replaces the credential of the account
	// holder; found=false when the row is gone.
	UpdateAccountPasswordHash(ctx context.Context, id int64, hash string) (bool, domain.OperationStats, error)
}

// ManageableSchools is the consumer-owned port over the Organisation &
// Tenancy fact an organisation-scoped administrator is bounded by: the live,
// active schools of their organisation. An organisation with none leaves the
// administrator with no account to manage, which is the refusal the
// retained repository applied.
type ManageableSchools interface {
	ManageableSchoolIDs(ctx context.Context, organizationID int64) ([]int64, error)
}
