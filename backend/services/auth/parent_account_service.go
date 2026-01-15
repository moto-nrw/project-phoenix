package auth

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/auth"
)

// CreateParentAccount creates a new parent account
func (s *Service) CreateParentAccount(ctx context.Context, email, username, password string) (*auth.AccountParent, error) {
	// Normalize input
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)

	// Validate password strength
	if err := ValidatePasswordStrength(password); err != nil {
		return nil, &AuthError{Op: opCreateParentAccount, Err: err}
	}

	// Check if email already exists
	_, err := s.repos.AccountParent.FindByEmail(ctx, email)
	if err == nil {
		return nil, &AuthError{Op: opCreateParentAccount, Err: ErrEmailAlreadyExists}
	}

	// Check if username already exists
	_, err = s.repos.AccountParent.FindByUsername(ctx, username)
	if err == nil {
		return nil, &AuthError{Op: opCreateParentAccount, Err: ErrUsernameAlreadyExists}
	}

	// Hash password
	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, &AuthError{Op: opHashPassword, Err: err}
	}

	usernamePtr := &username

	// Create parent account
	parentAccount := &auth.AccountParent{
		Email:        email,
		Username:     usernamePtr,
		Active:       true,
		PasswordHash: &passwordHash,
	}

	if err := s.repos.AccountParent.Create(ctx, parentAccount); err != nil {
		return nil, &AuthError{Op: opCreateParentAccount, Err: err}
	}

	return parentAccount, nil
}

// GetParentAccountByID retrieves a parent account by ID
func (s *Service) GetParentAccountByID(ctx context.Context, id int) (*auth.AccountParent, error) {
	account, err := s.repos.AccountParent.FindByID(ctx, int64(id))
	if err != nil {
		return nil, &AuthError{Op: "get parent account", Err: err}
	}
	return account, nil
}

// GetParentAccountByEmail retrieves a parent account by email
func (s *Service) GetParentAccountByEmail(ctx context.Context, email string) (*auth.AccountParent, error) {
	// Normalize email
	email = strings.TrimSpace(strings.ToLower(email))

	account, err := s.repos.AccountParent.FindByEmail(ctx, email)
	if err != nil {
		return nil, &AuthError{Op: "get parent account by email", Err: err}
	}
	return account, nil
}

// UpdateParentAccount updates a parent account
func (s *Service) UpdateParentAccount(ctx context.Context, account *auth.AccountParent) error {
	// Verify account exists
	existing, err := s.repos.AccountParent.FindByID(ctx, account.ID)
	if err != nil {
		return &AuthError{Op: "update parent account", Err: ErrParentAccountNotFound}
	}

	// Preserve password hash if not changing password
	if account.PasswordHash == nil {
		account.PasswordHash = existing.PasswordHash
	}

	if err := s.repos.AccountParent.Update(ctx, account); err != nil {
		return &AuthError{Op: "update parent account", Err: err}
	}

	return nil
}

// ActivateParentAccount activates a parent account
func (s *Service) ActivateParentAccount(ctx context.Context, accountID int) error {
	account, err := s.repos.AccountParent.FindByID(ctx, int64(accountID))
	if err != nil {
		return &AuthError{Op: "activate parent account", Err: ErrParentAccountNotFound}
	}

	account.Active = true
	if err := s.repos.AccountParent.Update(ctx, account); err != nil {
		return &AuthError{Op: "activate parent account", Err: err}
	}

	return nil
}

// DeactivateParentAccount deactivates a parent account
func (s *Service) DeactivateParentAccount(ctx context.Context, accountID int) error {
	account, err := s.repos.AccountParent.FindByID(ctx, int64(accountID))
	if err != nil {
		return &AuthError{Op: "deactivate parent account", Err: ErrParentAccountNotFound}
	}

	account.Active = false
	if err := s.repos.AccountParent.Update(ctx, account); err != nil {
		return &AuthError{Op: "deactivate parent account", Err: err}
	}

	return nil
}

// ListParentAccounts retrieves parent accounts matching the provided filters
func (s *Service) ListParentAccounts(ctx context.Context, filters map[string]interface{}) ([]*auth.AccountParent, error) {
	accounts, err := s.repos.AccountParent.List(ctx, filters)
	if err != nil {
		return nil, &AuthError{Op: "list parent accounts", Err: err}
	}
	return accounts, nil
}
