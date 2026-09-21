package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// newAccountAdministration composes the account administration flows
// (#3332) over the session flows whose revocation a deactivation schedules.
// The manageable-school set an organisation administrator is bounded by
// comes from the school directory the session dependencies already bind.
func newAccountAdministration(
	store ports.AccountAdministrationStore,
	auth *application.AccountAuthentication,
	sessions *SessionDependencies,
	deps *LifecycleDependencies,
) (*application.AccountAdministration, error) {
	if deps == nil {
		return nil, nil
	}
	if auth == nil || sessions == nil {
		return nil, errors.New("identity access compose: the administration flows require the session dependencies")
	}
	if sessions.Schools == nil || sessions.Passwords == nil {
		return nil, errors.New("identity access compose: the administration flows require the school directory and the password verifier")
	}
	attach := sessions.TenantRuntime
	if attach == nil {
		attach = func(ctx context.Context) context.Context { return ctx }
	}
	return application.NewAccountAdministration(auth, application.AccountAdministrationDependencies{
		Store:     store,
		Schools:   schoolDirectory{sessions.Schools},
		Verifier:  sessions.Passwords,
		Passwords: deps.Passwords,
		Runtime:   tenantRuntime{attach: attach, runner: newTransactionRunner()},
		Logger:    deps.Logger,
	})
}

// --- engine methods --------------------------------------------------------

var errAccountAdministrationUnavailable = identityaccess.ErrAccountAdministrationUnavailable

func (e engine) AnonymizeAccountForDeletion(ctx context.Context, accountID int64, email string) error {
	if e.administration == nil {
		return errAccountAdministrationUnavailable
	}
	return administrationError(e.administration.AnonymizeAccountForDeletion(e.attach(ctx), accountID, email))
}

func (e engine) FindOwnAccount(ctx context.Context, accountID int64) (identityaccess.ManagedAccount, error) {
	if e.administration == nil {
		return identityaccess.ManagedAccount{}, errAccountAdministrationUnavailable
	}
	account, err := e.administration.FindOwnAccount(e.attach(ctx), accountID)
	return publicManagedAccount(account), administrationError(err)
}

func (e engine) FindManageableAccount(ctx context.Context, accountID int64) (identityaccess.ManagedAccount, error) {
	if e.administration == nil {
		return identityaccess.ManagedAccount{}, errAccountAdministrationUnavailable
	}
	account, err := e.administration.FindManageableAccount(e.attach(ctx), accountID)
	return publicManagedAccount(account), administrationError(err)
}

func (e engine) ListManageableAccounts(ctx context.Context, filter identityaccess.AccountListFilter) ([]identityaccess.ManagedAccount, error) {
	if e.administration == nil {
		return nil, errAccountAdministrationUnavailable
	}
	accounts, err := e.administration.ListManageableAccounts(e.attach(ctx), domain.AccountListFilter{
		Email: filter.Email, Active: filter.Active,
	})
	if err != nil {
		return nil, administrationError(err)
	}
	return publicManagedAccounts(accounts), nil
}

func (e engine) ListManageableAccountsByRole(ctx context.Context, roleName string) ([]identityaccess.ManagedAccount, error) {
	if e.administration == nil {
		return nil, errAccountAdministrationUnavailable
	}
	accounts, err := e.administration.ListManageableAccountsByRole(e.attach(ctx), roleName)
	if err != nil {
		return nil, administrationError(err)
	}
	return publicManagedAccounts(accounts), nil
}

func (e engine) UpdateManageableAccount(ctx context.Context, update identityaccess.AccountIdentityUpdate) error {
	if e.administration == nil {
		return errAccountAdministrationUnavailable
	}
	return administrationError(e.administration.UpdateManageableAccount(e.attach(ctx), domain.AccountIdentityUpdate{
		AccountID: update.AccountID, Email: update.Email, Username: update.Username,
	}))
}

func (e engine) ActivateAccount(ctx context.Context, accountID int64) error {
	if e.administration == nil {
		return errAccountAdministrationUnavailable
	}
	return administrationError(e.administration.ActivateAccount(e.attach(ctx), accountID))
}

func (e engine) DeactivateAccount(ctx context.Context, accountID int64) error {
	if e.administration == nil {
		return errAccountAdministrationUnavailable
	}
	return administrationError(e.administration.DeactivateAccount(e.attach(ctx), accountID))
}

func (e engine) ChangeAccountPassword(ctx context.Context, accountID int64, currentPassword, newPassword string) error {
	if e.administration == nil {
		return errAccountAdministrationUnavailable
	}
	return administrationError(e.administration.ChangeAccountPassword(e.attach(ctx), accountID, currentPassword, newPassword))
}

func publicManagedAccount(account domain.ManagedAccountRecord) identityaccess.ManagedAccount {
	return identityaccess.ManagedAccount{
		ID: account.ID, Email: account.Email, Username: account.Username, Active: account.Active,
		CreatedAt: account.CreatedAt, UpdatedAt: account.UpdatedAt, LastLogin: account.LastLogin,
	}
}

func publicManagedAccounts(accounts []domain.ManagedAccountRecord) []identityaccess.ManagedAccount {
	result := make([]identityaccess.ManagedAccount, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, publicManagedAccount(account))
	}
	return result
}

// administrationError translates a flow error to the public contract. The
// administration reports no sentinel of its own: the account, credential
// and password-policy outcomes it refuses on are the authentication
// mapping's, which the lifecycle mapping chains to.
func administrationError(err error) error {
	return lifecycleError(err)
}
