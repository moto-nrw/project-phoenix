package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// Operation names the administration flows report. They are the envelope
// the retained consumers read, so their text is part of the contract.
const (
	opGetAccount         = "get account"
	opUpdateAccount      = "update account"
	opListAccounts       = "list accounts"
	opAccountsByRole     = "get accounts by role"
	opActivateAccount    = "activate account"
	opDeactivateAccount  = "deactivate account"
	opVerifyPassword     = "verify password"
	opValidatePassword   = "validate password"
	opRevokeOnDeactivate = "revoke tokens during account deactivation"
)

// AccountAdministration administers platform accounts (#3332): the account
// listing and the by-role listing behind the staff screens, the address and
// name change, activation and deactivation, and the credential change an
// account holder makes for themselves.
//
// Accounts are platform-wide rows without a tenant, so every administrative
// read and write carries the caller's visibility predicate in the same
// statement. That is the school boundary here: it cannot be lost between a
// read and the write it authorizes, which is what the retained service
// achieved by pairing FindManageableByID with UpdateManageable.
type AccountAdministration struct {
	auth      *AccountAuthentication
	store     ports.AccountAdministrationStore
	schools   ports.ManageableSchools
	verifier  ports.PasswordVerifier
	passwords ports.PasswordPolicy
	runtime   ports.Runtime
	logger    *slog.Logger
}

// AccountAdministrationDependencies are the ports the flows consume.
type AccountAdministrationDependencies struct {
	Store     ports.AccountAdministrationStore
	Schools   ports.ManageableSchools
	Verifier  ports.PasswordVerifier
	Passwords ports.PasswordPolicy
	Runtime   ports.Runtime
	Logger    *slog.Logger
}

// NewAccountAdministration composes the flows over the account
// authentication, whose session revocation a deactivation schedules and
// whose pending account-wide wipe an activation clears.
func NewAccountAdministration(auth *AccountAuthentication, deps AccountAdministrationDependencies) (*AccountAdministration, error) {
	switch {
	case auth == nil:
		return nil, fmt.Errorf("identity access account administration: account authentication is required")
	case deps.Store == nil, deps.Schools == nil:
		return nil, fmt.Errorf("identity access account administration: the account store and the manageable schools are required")
	case deps.Verifier == nil, deps.Passwords == nil, deps.Runtime == nil:
		return nil, fmt.Errorf("identity access account administration: password verifier, password policy and tenant runtime are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &AccountAdministration{
		auth: auth, store: deps.Store, schools: deps.Schools, verifier: deps.Verifier,
		passwords: deps.Passwords, runtime: deps.Runtime, logger: logger,
	}, nil
}

// FindOwnAccount reads the account without the administration predicate.
// The caller's own account is their own business; who may ask for it is the
// request boundary's decision, not this one's.
func (a *AccountAdministration) FindOwnAccount(ctx context.Context, accountID int64) (domain.ManagedAccountRecord, error) {
	account, found, _, err := a.store.FindAccountRecord(ctx, accountID)
	if err != nil {
		return domain.ManagedAccountRecord{}, failed(opGetAccount, err)
	}
	if !found {
		return domain.ManagedAccountRecord{}, failed(opGetAccount, domain.ErrAccountNotFound)
	}
	return account, nil
}

func (a *AccountAdministration) AnonymizeAccountForDeletion(ctx context.Context, accountID int64, email string) error {
	err := a.auth.sessions.run(ctx, a.auth.sessions.tx.RunPlatform, "anonymize_account_for_deletion", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, queryErr := a.store.AnonymizeAccountForDeletion(txCtx, accountID, email)
		stats.Add(queryStats)
		return queryErr
	})
	if err != nil {
		return failed("anonymize account for deletion", err)
	}
	return nil
}

// FindManageableAccount reads one account within the caller's visibility.
// An account outside it is reported as missing, so the read tells nobody
// that an account they may not administer exists.
func (a *AccountAdministration) FindManageableAccount(ctx context.Context, accountID int64) (domain.ManagedAccountRecord, error) {
	visibility, err := a.visibility(ctx)
	if err != nil {
		return domain.ManagedAccountRecord{}, failed(opGetAccount, err)
	}
	account, found, _, err := a.store.FindManageableAccountRecord(ctx, accountID, visibility)
	if err != nil {
		return domain.ManagedAccountRecord{}, failed(opGetAccount, err)
	}
	if !found {
		return domain.ManagedAccountRecord{}, failed(opGetAccount, domain.ErrAccountNotFound)
	}
	return account, nil
}

// ListManageableAccounts lists the accounts the caller may administer.
func (a *AccountAdministration) ListManageableAccounts(ctx context.Context, filter domain.AccountListFilter) ([]domain.ManagedAccountRecord, error) {
	visibility, err := a.visibility(ctx)
	if err != nil {
		return nil, failed(opListAccounts, err)
	}
	accounts, _, err := a.store.ListManageableAccountRecords(ctx, filter, visibility)
	if err != nil {
		return nil, failed(opListAccounts, err)
	}
	return accounts, nil
}

// ListManageableAccountsByRole lists the accounts holding the named role at
// a school the caller may administer.
//
// An organisation-scoped caller reads it administratively: the assignment
// and mapping rows it has to join belong to several schools, and the
// caller's own school transaction cannot see past its own.
func (a *AccountAdministration) ListManageableAccountsByRole(ctx context.Context, roleName string) ([]domain.ManagedAccountRecord, error) {
	visibility, err := a.visibility(ctx)
	if err != nil {
		return nil, failed(opAccountsByRole, err)
	}
	read := func(readCtx context.Context) ([]domain.ManagedAccountRecord, error) {
		accounts, _, listErr := a.store.ListManageableAccountRecordsByRole(readCtx, roleName, visibility)
		return accounts, listErr
	}
	if visibility.Kind != domain.AccountVisibilityOrganization {
		accounts, listErr := read(ctx)
		if listErr != nil {
			return nil, failed(opAccountsByRole, listErr)
		}
		return accounts, nil
	}
	var accounts []domain.ManagedAccountRecord
	if err := a.runtime.WithAdminTx(a.runtime.WithoutTransaction(ctx), func(adminCtx context.Context) error {
		listed, listErr := read(adminCtx)
		accounts = listed
		return listErr
	}); err != nil {
		return nil, failed(opAccountsByRole, err)
	}
	return accounts, nil
}

// UpdateManageableAccount writes the address and, when the caller supplied
// one, the name. Only those columns: the retained write replaced every
// mapped column of the row it had read, which could undo a PIN or MFA
// counter another request changed in between.
func (a *AccountAdministration) UpdateManageableAccount(ctx context.Context, update domain.AccountIdentityUpdate) error {
	email, err := domain.NormalizeAccountEmail(update.Email)
	if err != nil {
		return failed(opUpdateAccount, err)
	}
	update.Email = email
	visibility, err := a.visibility(ctx)
	if err != nil {
		return failed(opUpdateAccount, err)
	}
	found, _, err := a.store.UpdateManageableAccountIdentity(ctx, update, visibility)
	if err != nil {
		return failed(opUpdateAccount, err)
	}
	if !found {
		return failed(opUpdateAccount, domain.ErrAccountNotFound)
	}
	return nil
}

// ActivateAccount re-enables the account and clears the account-wide
// session wipe a deactivation had scheduled: without that the reactivated
// account's next login would be revoked by the standing intent.
func (a *AccountAdministration) ActivateAccount(ctx context.Context, accountID int64) error {
	if err := a.setActive(ctx, accountID, true, opActivateAccount); err != nil {
		return err
	}
	a.clearPendingWipe(ctx, accountID)
	return nil
}

// DeactivateAccount disables the account and schedules the account-wide
// session revocation. The revocation is requested after the flag committed,
// so a refused deactivation leaves no standing wipe behind.
func (a *AccountAdministration) DeactivateAccount(ctx context.Context, accountID int64) error {
	if err := a.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		return a.setActive(txCtx, accountID, false, opDeactivateAccount)
	}); err != nil {
		return err
	}
	if err := a.auth.ScheduleAccountWideRevoke(ctx, accountID, domain.RevocationReasonAccountDeactivated, "", ""); err != nil {
		return failed(opRevokeOnDeactivate, err)
	}
	return nil
}

func (a *AccountAdministration) setActive(ctx context.Context, accountID int64, active bool, op string) error {
	visibility, err := a.visibility(ctx)
	if err != nil {
		return failed(op, err)
	}
	found, _, err := a.store.SetManageableAccountActive(ctx, accountID, active, visibility)
	if err != nil {
		return failed(op, err)
	}
	if !found {
		return failed(op, domain.ErrAccountNotFound)
	}
	return nil
}

// ChangeAccountPassword replaces the account holder's own credential. The
// current password is the authorization, so the read is unscoped: an
// account holder proves who they are with the credential, not with a
// school membership.
func (a *AccountAdministration) ChangeAccountPassword(ctx context.Context, accountID int64, currentPassword, newPassword string) error {
	account, found, _, err := a.store.FindAccountRecord(ctx, accountID)
	if err != nil {
		return failed(opGetAccount, err)
	}
	if !found {
		return failed(opGetAccount, domain.ErrAccountNotFound)
	}
	if account.PasswordHash == "" {
		return failed(opVerifyPassword, domain.ErrInvalidCredentials)
	}
	valid, verifyErr := a.verifier.VerifyPassword(currentPassword, account.PasswordHash)
	if verifyErr != nil || !valid {
		return failed(opVerifyPassword, domain.ErrInvalidCredentials)
	}
	if strengthErr := a.passwords.ValidatePasswordStrength(newPassword); strengthErr != nil {
		return failed(opValidatePassword, strengthErr)
	}
	hash, hashErr := a.passwords.HashPassword(newPassword)
	if hashErr != nil {
		return failed(opHashPassword, hashErr)
	}
	written, _, err := a.store.UpdateAccountPasswordHash(ctx, accountID, hash)
	if err != nil {
		return failed(opUpdateAccount, err)
	}
	if !written {
		return failed(opUpdateAccount, domain.ErrAccountNotFound)
	}
	return nil
}

// visibility resolves the account set the caller may administer. An
// organisation or school context whose scope does not resolve is denied
// rather than widened: a missing scope must never read as "every account".
func (a *AccountAdministration) visibility(ctx context.Context) (domain.AccountVisibility, error) {
	denied := domain.AccountVisibility{Kind: domain.AccountVisibilityDenied}
	if a.runtime.Scope(ctx) == domain.ScopeOrg {
		organizationID := a.runtime.OrgID(ctx)
		if organizationID == 0 {
			return denied, nil
		}
		schoolIDs, err := a.schools.ManageableSchoolIDs(ctx, organizationID)
		if err != nil {
			return domain.AccountVisibility{}, err
		}
		if len(schoolIDs) == 0 {
			return denied, nil
		}
		return domain.AccountVisibility{Kind: domain.AccountVisibilityOrganization, SchoolIDs: schoolIDs}, nil
	}
	if a.runtime.IsAdminTx(ctx) || a.runtime.Scope(ctx) == domain.ScopePlatform {
		return domain.AccountVisibility{Kind: domain.AccountVisibilityGlobal}, nil
	}
	tenantID := a.runtime.TenantID(ctx)
	if tenantID == 0 {
		return denied, nil
	}
	return domain.AccountVisibility{Kind: domain.AccountVisibilityTenant, TenantID: tenantID}, nil
}

// clearPendingWipe claims the account's standing account-wide wipe. It is
// deferred to after the caller's commit when there are hooks, skipped while
// a foreign school transaction is open (the wipe is platform-wide and must
// not join it), and otherwise run at once on its own administrative
// transaction. A failure is logged, never returned: the reactivation itself
// committed, and the scheduler claims the wipe on its next pass.
func (a *AccountAdministration) clearPendingWipe(ctx context.Context, accountID int64) {
	run := func() {
		if err := a.markWipeCompleted(ctx, accountID); err != nil {
			a.logger.Warn("failed to clear pending account-wide wipe after reactivation",
				"account_id", accountID,
				"error", err,
			)
		}
	}
	if a.runtime.HasAfterCommitHooks(ctx) {
		a.runtime.RegisterAfterCommit(ctx, run)
		return
	}
	if a.runtime.HasTransaction(ctx) && !a.runtime.IsAdminTx(ctx) {
		return
	}
	run()
}

func (a *AccountAdministration) markWipeCompleted(ctx context.Context, accountID int64) error {
	if a.runtime.IsAdminTx(ctx) {
		return a.auth.MarkAccountWideWipeCompleted(ctx, accountID)
	}
	return a.runtime.WithAdminTx(a.runtime.Detach(ctx), func(adminCtx context.Context) error {
		return a.auth.MarkAccountWideWipeCompleted(adminCtx, accountID)
	})
}
