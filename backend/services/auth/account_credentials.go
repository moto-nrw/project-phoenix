package auth

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/models/auth"
)

// ChangePassword updates an account's password
func (s *Service) ChangePassword(ctx context.Context, accountID int, currentPassword, newPassword string) error {
	// Get account
	account, err := s.repos.Account.FindByID(ctx, int64(accountID))
	if err != nil {
		return &AuthError{Op: opGetAccount, Err: ErrAccountNotFound}
	}

	// Verify current password
	if account.PasswordHash == nil || *account.PasswordHash == "" {
		return &AuthError{Op: "verify password", Err: ErrInvalidCredentials}
	}

	valid, err := userpass.VerifyPassword(currentPassword, *account.PasswordHash)
	if err != nil || !valid {
		return &AuthError{Op: "verify password", Err: ErrInvalidCredentials}
	}

	// Validate new password
	if err := ValidatePasswordStrength(newPassword); err != nil {
		return &AuthError{Op: "validate password", Err: err}
	}

	// Hash new password
	passwordHash, err := HashPassword(newPassword)
	if err != nil {
		return &AuthError{Op: opHashPassword, Err: err}
	}

	// Update password
	account.PasswordHash = &passwordHash
	if err := s.repos.Account.Update(ctx, account); err != nil {
		return &AuthError{Op: opUpdateAccount, Err: err}
	}

	return nil
}

// GetAccountByID retrieves an account by ID
func (s *Service) GetAccountByID(ctx context.Context, id int) (*auth.Account, error) {
	var (
		account *auth.Account
		err     error
	)
	if claims := jwt.ClaimsFromCtx(ctx); claims.ID == id {
		account, err = s.repos.Account.FindByID(ctx, int64(id))
	} else {
		account, err = s.repos.Account.FindManageableByID(ctx, int64(id))
	}
	if err != nil {
		return nil, &AuthError{Op: opGetAccount, Err: ErrAccountNotFound}
	}
	return account, nil
}

// CredentialOperations read an account and change its password.
type CredentialOperations interface {
	ChangePassword(ctx context.Context, accountID int, currentPassword, newPassword string) error
	GetAccountByID(ctx context.Context, id int) (*auth.Account, error)
}
