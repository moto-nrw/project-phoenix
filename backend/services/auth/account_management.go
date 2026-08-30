package auth

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Account Management Extensions

// ActivateAccount activates a user account
func (s *Service) ActivateAccount(ctx context.Context, accountID int) error {
	account, err := s.repos.Account.FindManageableByID(ctx, int64(accountID))
	if err != nil {
		return &AuthError{Op: "activate account", Err: ErrAccountNotFound}
	}

	account.Active = true
	if err := s.repos.Account.UpdateManageable(ctx, account); err != nil {
		return accountWriteError("activate account", err)
	}

	s.clearPendingAccountWideWipes(ctx, int64(accountID))
	return nil
}

func (s *Service) clearPendingAccountWideWipes(ctx context.Context, accountID int64) {
	run := func() {
		if err := s.markPendingWipeCompletedIndependently(ctx, accountID); err != nil {
			s.getLogger().Warn("failed to clear pending account-wide wipe after reactivation",
				"account_id", accountID,
				"error", err,
			)
		}
	}
	if tenant.HasAfterCommitHooks(ctx) {
		tenant.RegisterAfterCommit(ctx, run)
		return
	}
	if hasAmbientTx(ctx) && !tenant.IsAdminTx(ctx) {
		return
	}
	run()
}

func (s *Service) markPendingWipeCompletedIndependently(ctx context.Context, accountID int64) error {
	if tenant.IsAdminTx(ctx) || s.db == nil {
		return s.markAccountWideWipeCompleted(ctx, accountID)
	}
	return tenant.WithAdminTx(s.withTenantRuntime(s.independentCleanupCtx(ctx)), s.db, func(adminCtx context.Context, _ bun.Tx) error {
		return s.markAccountWideWipeCompleted(adminCtx, accountID)
	})
}

// DeactivateAccount deactivates a user account
func (s *Service) DeactivateAccount(ctx context.Context, accountID int) error {
	err := s.runInTx(ctx, func(txCtx context.Context) error {
		account, err := s.repos.Account.FindManageableByID(txCtx, int64(accountID))
		if err != nil {
			return &AuthError{Op: "deactivate account", Err: ErrAccountNotFound}
		}
		account.Active = false
		if err := s.repos.Account.UpdateManageable(txCtx, account); err != nil {
			return accountWriteError("deactivate account", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if revokeErr := s.scheduleAccountWideRevoke(ctx, int64(accountID), "account_deactivated", "", ""); revokeErr != nil {
		return &AuthError{Op: "revoke tokens during account deactivation", Err: revokeErr}
	}
	return nil
}

// UpdateAccount updates account information
func (s *Service) UpdateAccount(ctx context.Context, account *auth.Account) error {
	// Verify account exists
	existing, err := s.repos.Account.FindManageableByID(ctx, account.ID)
	if err != nil {
		return &AuthError{Op: opUpdateAccount, Err: ErrAccountNotFound}
	}

	// Preserve password hash if not changing password
	if account.PasswordHash == nil {
		account.PasswordHash = existing.PasswordHash
	}

	if err := s.repos.Account.UpdateManageable(ctx, account); err != nil {
		return accountWriteError(opUpdateAccount, err)
	}

	return nil
}

func accountWriteError(op string, err error) error {
	if modelBase.IsNoRows(err) {
		return &AuthError{Op: op, Err: ErrAccountNotFound}
	}
	return &AuthError{Op: op, Err: err}
}

// ListAccounts retrieves accounts matching the provided filters
func (s *Service) ListAccounts(ctx context.Context, filters map[string]interface{}) ([]*auth.Account, error) {
	accounts, err := s.repos.Account.ListManageable(ctx, filters)
	if err != nil {
		return nil, &AuthError{Op: "list accounts", Err: err}
	}
	return accounts, nil
}

// GetAccountsByRole retrieves all accounts with a specific role
func (s *Service) GetAccountsByRole(ctx context.Context, roleName string) ([]*auth.Account, error) {
	accounts, err := s.repos.Account.FindByRole(ctx, roleName)
	if err != nil {
		return nil, &AuthError{Op: "get accounts by role", Err: err}
	}
	return accounts, nil
}

// GetAccountsWithRolesAndPermissions retrieves accounts with their roles and permissions
func (s *Service) GetAccountsWithRolesAndPermissions(ctx context.Context, filters map[string]interface{}) ([]*auth.Account, error) {
	accounts, err := s.repos.Account.FindAccountsWithRolesAndPermissions(ctx, filters)
	if err != nil {
		return nil, &AuthError{Op: "get accounts with roles and permissions", Err: err}
	}
	return accounts, nil
}
