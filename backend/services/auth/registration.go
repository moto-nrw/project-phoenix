package auth

import (
	"context"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/uptrace/bun"
)

// Register creates a new user account
func (s *Service) Register(ctx context.Context, email, username, password string, roleID *int64) (*auth.Account, error) {
	// Validate and normalize registration inputs
	if err := s.validateRegistrationInputs(ctx, email, username, password); err != nil {
		return nil, err
	}

	// Create account object with hashed password
	account, err := s.createAccountObject(email, username, password)
	if err != nil {
		return nil, err
	}

	// Persist account and assign role in transaction
	if err := s.persistAccountWithRole(ctx, account, roleID); err != nil {
		return nil, err
	}

	return account, nil
}

// validateRegistrationInputs validates registration data and checks for conflicts
func (s *Service) validateRegistrationInputs(ctx context.Context, email, username, password string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)

	if err := ValidatePasswordStrength(password); err != nil {
		return &AuthError{Op: "register", Err: err}
	}

	// Check if email already exists
	if _, err := s.repos.Account.FindByEmail(ctx, email); err == nil {
		return &AuthError{Op: "register", Err: ErrEmailAlreadyExists}
	}

	// Check if username already exists
	if _, err := s.repos.Account.FindByUsername(ctx, username); err == nil {
		return &AuthError{Op: "register", Err: ErrUsernameAlreadyExists}
	}

	return nil
}

// createAccountObject creates a new account with hashed password
func (s *Service) createAccountObject(email, username, password string) (*auth.Account, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)

	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, &AuthError{Op: opHashPassword, Err: err}
	}

	usernamePtr := &username
	now := time.Now()

	return &auth.Account{
		Email:        email,
		Username:     usernamePtr,
		Active:       true,
		PasswordHash: &passwordHash,
		LastLogin:    &now,
	}, nil
}

// persistAccountWithRole saves account and assigns role in a transaction
func (s *Service) persistAccountWithRole(ctx context.Context, account *auth.Account, roleID *int64) error {
	return s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*Service)

		// Create account
		if err := txService.repos.Account.Create(ctx, account); err != nil {
			return err
		}

		// Assign role to account
		return s.assignRoleToNewAccount(ctx, txService, account.ID, roleID)
	})
}

// assignRoleToNewAccount determines and assigns appropriate role to new account
func (s *Service) assignRoleToNewAccount(ctx context.Context, txService *Service, accountID int64, roleID *int64) error {
	targetRoleID, err := s.determineRoleForNewAccount(ctx, txService, roleID)
	if err != nil {
		return err
	}

	// No role to assign (default role lookup failed, continue without role)
	if targetRoleID == 0 {
		return nil
	}

	// Create account role mapping
	accountRole := &auth.AccountRole{
		AccountID: accountID,
		RoleID:    targetRoleID,
	}

	if err := txService.repos.AccountRole.Create(ctx, accountRole); err != nil {
		if logger.Logger != nil {
			logger.Logger.WithError(err).Error("Failed to create account role")
		}
		return err // Roll back transaction if role assignment fails
	}

	return nil
}

// determineRoleForNewAccount returns the role ID to assign (provided or default)
func (s *Service) determineRoleForNewAccount(ctx context.Context, txService *Service, roleID *int64) (int64, error) {
	if roleID != nil {
		return *roleID, nil
	}

	// Find default user role
	userRole, err := txService.getRoleByName(ctx, "user")
	if err != nil || userRole == nil {
		if logger.Logger != nil {
			logger.Logger.WithError(err).Warn("Failed to find default user role")
		}
		return 0, nil // Return 0 to indicate no role (continue without role assignment)
	}

	return userRole.ID, nil
}
