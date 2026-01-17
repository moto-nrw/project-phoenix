package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/audit"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	"github.com/moto-nrw/project-phoenix/internal/core/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	"github.com/uptrace/bun"
)

// Login authenticates a user and returns access and refresh tokens
func (s *Service) Login(ctx context.Context, email, password string) (string, string, error) {
	return s.LoginWithAudit(ctx, email, password, "", "")
}

// LoginWithAudit authenticates a user and returns access and refresh tokens with audit logging
func (s *Service) LoginWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (string, string, error) {
	// Validate credentials and get account
	account, err := s.validateLoginCredentials(ctx, email, password, ipAddress, userAgent)
	if err != nil {
		return "", "", err
	}

	// Create refresh token with transaction retry logic
	token, err := s.createRefreshTokenWithRetry(ctx, account)
	if err != nil {
		return "", "", err
	}

	// Load account metadata (roles, permissions, person info)
	metadata := s.loadAccountMetadata(ctx, account)

	// Build JWT claims from account and metadata
	appClaims, refreshClaims := s.buildJWTClaims(account, token, metadata, email)

	// Generate token pair and log success
	return s.generateAndLogTokens(ctx, account.ID, appClaims, refreshClaims, ipAddress, userAgent, audit.EventTypeLogin)
}

// validateLoginCredentials validates email, password, and account status
func (s *Service) validateLoginCredentials(ctx context.Context, email, password, ipAddress, userAgent string) (*auth.Account, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	account, err := s.repos.Account.FindByEmail(ctx, email)
	if err != nil {
		s.logFailedLogin(ctx, 0, ipAddress, userAgent, "Account not found")
		return nil, &AuthError{Op: "login", Err: ErrAccountNotFound}
	}

	if !account.Active {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Account inactive")
		return nil, &AuthError{Op: "login", Err: ErrAccountInactive}
	}

	if err := s.verifyPassword(account, password); err != nil {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Invalid password")
		return nil, err
	}

	return account, nil
}

// verifyPassword checks if the provided password matches the account's hash
func (s *Service) verifyPassword(account *auth.Account, password string) error {
	if account.PasswordHash == nil || *account.PasswordHash == "" {
		return &AuthError{Op: "login", Err: ErrInvalidCredentials}
	}

	valid, err := auth.VerifyPassword(password, *account.PasswordHash)
	if err != nil || !valid {
		return &AuthError{Op: "login", Err: ErrInvalidCredentials}
	}

	return nil
}

// createRefreshTokenWithRetry creates a refresh token with retry logic for concurrent logins
func (s *Service) createRefreshTokenWithRetry(ctx context.Context, account *auth.Account) (*auth.Token, error) {
	token := s.newRefreshToken(account.ID)

	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := s.persistTokenInTransaction(ctx, account, token)

		if err == nil {
			return token, nil
		}

		if !s.isTokenFamilyConflict(err) {
			return nil, &AuthError{Op: "login transaction", Err: err}
		}

		// Regenerate family ID and retry
		token.FamilyID = uuid.Must(uuid.NewV4()).String()
		if logger.Logger != nil {
			logger.Logger.WithFields(map[string]interface{}{
				"account_id":  account.ID,
				"attempt":     attempt + 1,
				"max_retries": maxRetries,
			}).Warn("Login race condition detected, retrying")
		}
	}

	return nil, &AuthError{Op: "login transaction", Err: fmt.Errorf("max retries exceeded")}
}

// newRefreshToken creates a new refresh token for the given account
func (s *Service) newRefreshToken(accountID int64) *auth.Token {
	identifier := "Service login"
	return &auth.Token{
		Token:      uuid.Must(uuid.NewV4()).String(),
		AccountID:  accountID,
		Expiry:     time.Now().Add(s.jwtRefreshExpiry),
		Mobile:     false,
		Identifier: &identifier,
		FamilyID:   uuid.Must(uuid.NewV4()).String(),
		Generation: 0,
	}
}

// persistTokenInTransaction saves the token and updates last login in a transaction
func (s *Service) persistTokenInTransaction(ctx context.Context, account *auth.Account, token *auth.Token) error {
	return s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*Service)

		// Clean up old tokens (keep 5 most recent)
		const maxTokensPerAccount = 5
		if err := txService.repos.Token.CleanupOldTokensForAccount(ctx, account.ID, maxTokensPerAccount); err != nil {
			if logger.Logger != nil {
				logger.Logger.WithFields(map[string]interface{}{
					"account_id": account.ID,
					"error":      err.Error(),
				}).Warn("Failed to clean up old tokens for account")
			}
		}

		// Create new token
		if err := txService.repos.Token.Create(ctx, token); err != nil {
			if s.isTokenFamilyConflict(err) {
				return err // Will retry with new family ID
			}
			return err
		}

		// Update last login
		loginTime := time.Now()
		account.LastLogin = &loginTime
		return txService.repos.Account.Update(ctx, account)
	})
}

// isTokenFamilyConflict checks if error is due to token family conflict
func (s *Service) isTokenFamilyConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "uk_tokens_family_generation")
}

// buildJWTClaims constructs JWT claims from account and metadata
func (s *Service) buildJWTClaims(
	account *auth.Account,
	token *auth.Token,
	metadata *accountMetadata,
	email string,
) (port.AppClaims, port.RefreshClaims) {
	appClaims := port.AppClaims{
		ID:          int(account.ID),
		Sub:         email,
		Username:    metadata.username,
		FirstName:   metadata.firstName,
		LastName:    metadata.lastName,
		Roles:       metadata.roleNames,
		Permissions: metadata.permissionStrs,
		IsAdmin:     metadata.isAdmin,
		IsTeacher:   metadata.isTeacher,
	}

	refreshClaims := port.RefreshClaims{
		ID:    int(token.ID),
		Token: token.Token,
	}

	return appClaims, refreshClaims
}

// generateAndLogTokens generates JWT token pair and logs the authentication event
func (s *Service) generateAndLogTokens(
	ctx context.Context,
	accountID int64,
	appClaims port.AppClaims,
	refreshClaims port.RefreshClaims,
	ipAddress, userAgent, eventType string,
) (string, string, error) {
	accessToken, refreshToken, err := s.tokenProvider.GenerateTokenPair(appClaims, refreshClaims)
	if err != nil {
		return "", "", &AuthError{Op: "generate tokens", Err: err}
	}

	if ipAddress != "" {
		s.logAuthEvent(ctx, accountID, eventType, true, ipAddress, userAgent, "")
	}

	return accessToken, refreshToken, nil
}

// logFailedLogin logs a failed login attempt if IP address is provided
func (s *Service) logFailedLogin(ctx context.Context, accountID int64, ipAddress, userAgent, reason string) {
	if ipAddress != "" {
		s.logAuthEvent(ctx, accountID, audit.EventTypeLogin, false, ipAddress, userAgent, reason)
	}
}
