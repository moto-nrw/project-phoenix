package auth

import (
	"context"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/uptrace/bun"
)

// RefreshToken generates new token pair from a refresh token
func (s *Service) RefreshToken(ctx context.Context, refreshTokenStr string) (string, string, error) {
	return s.RefreshTokenWithAudit(ctx, refreshTokenStr, "", "")
}

// RefreshTokenWithAudit generates new token pair from a refresh token with audit logging
func (s *Service) RefreshTokenWithAudit(ctx context.Context, refreshTokenStr, ipAddress, userAgent string) (string, string, error) {
	// Parse and validate refresh token claims
	refreshClaims, err := s.parseRefreshTokenClaims(refreshTokenStr)
	if err != nil {
		return "", "", err
	}

	// Validate and refresh token in transaction
	account, newToken, err := s.refreshTokenInTransaction(ctx, refreshClaims, ipAddress, userAgent)
	if err != nil {
		return "", "", err
	}

	// Load account metadata (roles, permissions, person info)
	metadata := s.loadAccountMetadata(ctx, account)

	// Build JWT claims from account and metadata
	appClaims, newRefreshClaims := s.buildJWTClaims(account, newToken, metadata, account.Email)

	// Generate token pair and log success as token refresh
	return s.generateAndLogTokens(ctx, account.ID, appClaims, newRefreshClaims, ipAddress, userAgent, audit.EventTypeTokenRefresh)
}

// parseRefreshTokenClaims parses and validates JWT refresh token claims
func (s *Service) parseRefreshTokenClaims(refreshTokenStr string) (*jwt.RefreshClaims, error) {
	jwtToken, err := s.tokenAuth.JwtAuth.Decode(refreshTokenStr)
	if err != nil {
		return nil, &AuthError{Op: "parse refresh token", Err: ErrInvalidToken}
	}

	claims := extractClaims(jwtToken)

	var refreshClaims jwt.RefreshClaims
	err = refreshClaims.ParseClaims(claims)
	if err != nil {
		return nil, &AuthError{Op: "parse refresh claims", Err: ErrInvalidToken}
	}

	return &refreshClaims, nil
}

// refreshTokenInTransaction validates and refreshes token in a transaction
func (s *Service) refreshTokenInTransaction(ctx context.Context, refreshClaims *jwt.RefreshClaims, ipAddress, userAgent string) (*auth.Account, *auth.Token, error) {
	var dbToken *auth.Token
	var account *auth.Account
	var newToken *auth.Token

	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		var err error

		// Fetch and validate token
		dbToken, err = s.fetchAndValidateToken(ctx, refreshClaims.Token, ipAddress, userAgent)
		if err != nil {
			return err
		}

		// Detect potential token theft
		if err := s.detectTokenTheft(ctx, dbToken, ipAddress, userAgent); err != nil {
			return err
		}

		// Fetch and validate account
		account, err = s.fetchAndValidateAccount(ctx, dbToken.AccountID, ipAddress, userAgent)
		if err != nil {
			return err
		}

		// Create and persist new token
		newToken, err = s.createAndPersistNewToken(ctx, dbToken, account.ID)
		if err != nil {
			return err
		}

		// Update last login
		loginTime := time.Now()
		account.LastLogin = &loginTime
		return s.repos.Account.Update(ctx, account)
	})

	if err != nil {
		return nil, nil, &AuthError{Op: "refresh transaction", Err: err}
	}

	return account, newToken, nil
}

// fetchAndValidateToken retrieves token and checks expiry
func (s *Service) fetchAndValidateToken(ctx context.Context, tokenStr, ipAddress, userAgent string) (*auth.Token, error) {
	dbToken, err := s.repos.Token.FindByTokenForUpdate(ctx, tokenStr)
	if err != nil {
		return nil, &AuthError{Op: "get token", Err: ErrTokenNotFound}
	}

	if time.Now().After(dbToken.Expiry) {
		_ = s.repos.Token.Delete(ctx, dbToken.ID)
		if ipAddress != "" && dbToken.AccountID > 0 {
			s.logAuthEvent(ctx, dbToken.AccountID, audit.EventTypeTokenExpired, false, ipAddress, userAgent, "Token expired")
		}
		return nil, &AuthError{Op: "check token expiry", Err: ErrTokenExpired}
	}

	return dbToken, nil
}

// detectTokenTheft checks for token family theft detection
func (s *Service) detectTokenTheft(ctx context.Context, dbToken *auth.Token, ipAddress, userAgent string) error {
	if dbToken.FamilyID == "" {
		return nil
	}

	latestToken, err := s.repos.Token.GetLatestTokenInFamily(ctx, dbToken.FamilyID)
	if err != nil || latestToken == nil || latestToken.Generation <= dbToken.Generation {
		return nil
	}

	// Token theft detected - invalidate entire family
	_ = s.repos.Token.DeleteByFamilyID(ctx, dbToken.FamilyID)

	if ipAddress != "" {
		s.logAuthEvent(ctx, dbToken.AccountID, audit.EventTypeTokenRefresh, false, ipAddress, userAgent, "Token theft detected - family invalidated")
	}

	return &AuthError{Op: "token theft detection", Err: ErrInvalidToken}
}

// fetchAndValidateAccount retrieves account and checks if active
func (s *Service) fetchAndValidateAccount(ctx context.Context, accountID int64, ipAddress, userAgent string) (*auth.Account, error) {
	account, err := s.repos.Account.FindByID(ctx, accountID)
	if err != nil {
		return nil, &AuthError{Op: opGetAccount, Err: ErrAccountNotFound}
	}

	if !account.Active {
		if ipAddress != "" {
			s.logAuthEvent(ctx, account.ID, audit.EventTypeTokenRefresh, false, ipAddress, userAgent, "Account inactive")
		}
		return nil, &AuthError{Op: "check account status", Err: ErrAccountInactive}
	}

	return account, nil
}

// createAndPersistNewToken creates new token and deletes old one
func (s *Service) createAndPersistNewToken(ctx context.Context, oldToken *auth.Token, accountID int64) (*auth.Token, error) {
	newToken := &auth.Token{
		Token:      uuid.Must(uuid.NewV4()).String(),
		AccountID:  accountID,
		Expiry:     time.Now().Add(s.jwtRefreshExpiry),
		Mobile:     oldToken.Mobile,
		Identifier: oldToken.Identifier,
		FamilyID:   oldToken.FamilyID,
		Generation: oldToken.Generation + 1,
	}

	if err := s.repos.Token.Delete(ctx, oldToken.ID); err != nil {
		return nil, err
	}

	if err := s.repos.Token.Create(ctx, newToken); err != nil {
		return nil, err
	}

	return newToken, nil
}

// Logout invalidates a refresh token
func (s *Service) Logout(ctx context.Context, refreshTokenStr string) error {
	return s.LogoutWithAudit(ctx, refreshTokenStr, "", "")
}

// LogoutWithAudit invalidates a refresh token with audit logging
func (s *Service) LogoutWithAudit(ctx context.Context, refreshTokenStr, ipAddress, userAgent string) error {
	// Parse and validate refresh token claims
	refreshClaims, err := s.parseRefreshTokenClaims(refreshTokenStr)
	if err != nil {
		return err
	}

	// Get token from database to find the account ID
	dbToken, err := s.repos.Token.FindByToken(ctx, refreshClaims.Token)
	if err != nil {
		// Token not found, consider logout successful
		return nil
	}

	// Delete ALL tokens for this account to ensure complete logout
	if err := s.repos.Token.DeleteByAccountID(ctx, dbToken.AccountID); err != nil {
		// Log the error but don't fail the logout
		if logging.Logger != nil {
			logging.Logger.WithError(err).WithField("account_id", dbToken.AccountID).Warn("Failed to delete all tokens during logout")
		}
		// Still try to delete the specific token
		if deleteErr := s.repos.Token.Delete(ctx, dbToken.ID); deleteErr != nil {
			return &AuthError{Op: "delete token", Err: deleteErr}
		}
	}

	// Log successful logout
	if ipAddress != "" {
		s.logAuthEvent(ctx, dbToken.AccountID, audit.EventTypeLogout, true, ipAddress, userAgent, "")
	}

	return nil
}
