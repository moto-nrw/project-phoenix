package identityaccess

import (
	"context"
	"errors"
	"time"
)

// ErrAccountAdministrationUnavailable reports a composition without the
// account lifecycle dependencies the administration flows need.
var ErrAccountAdministrationUnavailable = errors.New("account administration is not composed")

// ManagedAccount is one platform account as the administration reports it.
// The credential never leaves the owner.
type ManagedAccount struct {
	ID        int64
	Email     string
	Username  string
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
	LastLogin *time.Time
}

// AccountListFilter narrows an account listing. An empty address and a nil
// active flag list every account the caller may administer.
type AccountListFilter struct {
	// Email matches the stored address case-insensitively and in full.
	Email  string
	Active *bool
}

// AccountIdentityUpdate changes the address and, when Username is set, the
// name of one account. A nil Username keeps the stored one.
type AccountIdentityUpdate struct {
	AccountID int64
	Email     string
	Username  *string
}

// AccountAdministration administers platform accounts (#3332): the listings
// behind the staff screens, the address and name change, activation and
// deactivation, and the credential change an account holder makes for
// themselves.
//
// Accounts are platform-wide rows without a tenant, so ordinary administration
// commands are restricted to the account set the caller may administer:
// the school in context, the manageable schools of their organisation, or every account
// for a platform-scoped caller and an administrative transaction. An
// account outside that set reads as ErrAccountNotFound, so the boundary
// never reveals that it exists. FindOwnAccount and ChangeAccountPassword
// are the account holder's own business and carry no such predicate; who
// may ask for them is the request boundary's decision. AnonymizeAccountForDeletion
// is likewise account-wide and reserved for the authorized platform deletion flow.
//
// Refusals arrive wrapped in an AuthenticationError whose Op names the
// flow, carrying ErrAccountNotFound, ErrInvalidCredentials or
// ErrPasswordTooWeak.
type AccountAdministration interface {
	// AnonymizeAccountForDeletion replaces identifying fields for an already
	// authorized platform deletion workflow. It is account-wide, preserves
	// the caller's transaction, and does not deactivate or unlink the account.
	AnonymizeAccountForDeletion(ctx context.Context, accountID int64, email string) error
	FindOwnAccount(ctx context.Context, accountID int64) (ManagedAccount, error)
	FindManageableAccount(ctx context.Context, accountID int64) (ManagedAccount, error)
	ListManageableAccounts(ctx context.Context, filter AccountListFilter) ([]ManagedAccount, error)
	ListManageableAccountsByRole(ctx context.Context, roleName string) ([]ManagedAccount, error)
	UpdateManageableAccount(ctx context.Context, update AccountIdentityUpdate) error
	ActivateAccount(ctx context.Context, accountID int64) error
	DeactivateAccount(ctx context.Context, accountID int64) error
	ChangeAccountPassword(ctx context.Context, accountID int64, currentPassword, newPassword string) error
}

func (m *Module) AnonymizeAccountForDeletion(ctx context.Context, accountID int64, email string) error {
	return m.engine.AnonymizeAccountForDeletion(ctx, accountID, email)
}

func (m *Module) FindOwnAccount(ctx context.Context, accountID int64) (ManagedAccount, error) {
	return m.engine.FindOwnAccount(ctx, accountID)
}

func (m *Module) FindManageableAccount(ctx context.Context, accountID int64) (ManagedAccount, error) {
	return m.engine.FindManageableAccount(ctx, accountID)
}

func (m *Module) ListManageableAccounts(ctx context.Context, filter AccountListFilter) ([]ManagedAccount, error) {
	return m.engine.ListManageableAccounts(ctx, filter)
}

func (m *Module) ListManageableAccountsByRole(ctx context.Context, roleName string) ([]ManagedAccount, error) {
	return m.engine.ListManageableAccountsByRole(ctx, roleName)
}

func (m *Module) UpdateManageableAccount(ctx context.Context, update AccountIdentityUpdate) error {
	return m.engine.UpdateManageableAccount(ctx, update)
}

func (m *Module) ActivateAccount(ctx context.Context, accountID int64) error {
	return m.engine.ActivateAccount(ctx, accountID)
}

func (m *Module) DeactivateAccount(ctx context.Context, accountID int64) error {
	return m.engine.DeactivateAccount(ctx, accountID)
}

func (m *Module) ChangeAccountPassword(ctx context.Context, accountID int64, currentPassword, newPassword string) error {
	return m.engine.ChangeAccountPassword(ctx, accountID, currentPassword, newPassword)
}
