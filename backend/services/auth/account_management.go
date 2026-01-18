package auth

import (
	"context"
	"log"

	"github.com/moto-nrw/project-phoenix/models/auth"
)

// Account Management Extensions

// ActivateAccount activates a user account
func (s *Service) ActivateAccount(ctx context.Context, accountID int) error {
	account, err := s.repos.Account.FindByID(ctx, int64(accountID))
	if err != nil {
		return &AuthError{Op: "activate account", Err: ErrAccountNotFound}
	}

	account.Active = true
	if err := s.repos.Account.Update(ctx, account); err != nil {
		return &AuthError{Op: "activate account", Err: err}
	}

	return nil
}

// DeactivateAccount deactivates a user account
func (s *Service) DeactivateAccount(ctx context.Context, accountID int) error {
	account, err := s.repos.Account.FindByID(ctx, int64(accountID))
	if err != nil {
		return &AuthError{Op: "deactivate account", Err: ErrAccountNotFound}
	}

	account.Active = false
	if err := s.repos.Account.Update(ctx, account); err != nil {
		return &AuthError{Op: "deactivate account", Err: err}
	}

	// Also invalidate all tokens for this account
	if err := s.repos.Token.DeleteByAccountID(ctx, int64(accountID)); err != nil {
		// Log error but don't fail the deactivation
		log.Printf("Failed to delete tokens for account %d: %v", accountID, err)
	}

	return nil
}

// UpdateAccount updates account information
func (s *Service) UpdateAccount(ctx context.Context, account *auth.Account) error {
	// Verify account exists
	existing, err := s.repos.Account.FindByID(ctx, account.ID)
	if err != nil {
		return &AuthError{Op: opUpdateAccount, Err: ErrAccountNotFound}
	}

	// Preserve password hash if not changing password
	if account.PasswordHash == nil {
		account.PasswordHash = existing.PasswordHash
	}

	if err := s.repos.Account.Update(ctx, account); err != nil {
		return &AuthError{Op: opUpdateAccount, Err: err}
	}

	return nil
}

// ListAccounts retrieves accounts matching the provided filters
func (s *Service) ListAccounts(ctx context.Context, filters map[string]interface{}) ([]*auth.Account, error) {
	accounts, err := s.repos.Account.List(ctx, filters)
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
