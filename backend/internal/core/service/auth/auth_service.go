package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	jwx "github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/mailer"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/audit"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	"github.com/uptrace/bun"
)

const (
	opCreateService          = "create service"
	opHashPassword           = "hash password"
	opGetAccount             = "get account"
	opUpdateAccount          = "update account"
	opAssignPermissionToRole = "assign permission to role"
	opCreateParentAccount    = "create parent account"
)

// ServiceConfig holds configuration for the auth service
type ServiceConfig struct {
	Dispatcher           *mailer.Dispatcher
	DefaultFrom          port.EmailAddress
	FrontendURL          string
	PasswordResetExpiry  time.Duration
	RateLimitEnabled     bool
	RateLimitMaxRequests int // Maximum password reset requests per time window (default: 3)
}

// NewServiceConfig creates and validates a new ServiceConfig.
// All configuration is passed explicitly following 12-Factor App principles.
func NewServiceConfig(
	dispatcher *mailer.Dispatcher,
	defaultFrom port.EmailAddress,
	frontendURL string,
	passwordResetExpiry time.Duration,
	rateLimitEnabled bool,
	rateLimitMaxRequests int,
) (*ServiceConfig, error) {
	if frontendURL == "" {
		return nil, errors.New("frontendURL cannot be empty")
	}
	if passwordResetExpiry <= 0 {
		return nil, errors.New("passwordResetExpiry must be positive")
	}

	// Apply sensible defaults and bounds for rate limit max requests
	if rateLimitMaxRequests <= 0 {
		rateLimitMaxRequests = 3 // Default: 3 requests per window
	} else if rateLimitMaxRequests > 100 {
		rateLimitMaxRequests = 100 // Cap at 100 to prevent abuse
	}

	return &ServiceConfig{
		Dispatcher:           dispatcher,
		DefaultFrom:          defaultFrom,
		FrontendURL:          frontendURL,
		PasswordResetExpiry:  passwordResetExpiry,
		RateLimitEnabled:     rateLimitEnabled,
		RateLimitMaxRequests: rateLimitMaxRequests,
	}, nil
}

// Service provides authentication and authorization functionality
type Service struct {
	repos                *repositories.Factory
	tokenAuth            *jwt.TokenAuth
	dispatcher           *mailer.Dispatcher
	defaultFrom          port.EmailAddress
	frontendURL          string
	passwordResetExpiry  time.Duration
	rateLimitEnabled     bool
	rateLimitMaxRequests int
	jwtExpiry            time.Duration
	jwtRefreshExpiry     time.Duration
	txHandler            *base.TxHandler
	db                   *bun.DB
}

// NewService creates a new auth service with reduced parameter count
// Uses repository factory pattern and config struct to avoid parameter bloat
func NewService(
	repos *repositories.Factory,
	config *ServiceConfig,
	db *bun.DB,
) (*Service, error) {
	if repos == nil {
		return nil, &AuthError{Op: opCreateService, Err: errors.New("repos factory is nil")}
	}
	if config == nil {
		return nil, &AuthError{Op: opCreateService, Err: errors.New("config is nil")}
	}
	if db == nil {
		return nil, &AuthError{Op: opCreateService, Err: errors.New("database is nil")}
	}

	tokenAuth, err := jwt.NewTokenAuth()
	if err != nil {
		return nil, &AuthError{Op: "create token auth", Err: err}
	}

	return &Service{
		repos:                repos,
		tokenAuth:            tokenAuth,
		dispatcher:           config.Dispatcher,
		defaultFrom:          config.DefaultFrom,
		frontendURL:          config.FrontendURL,
		passwordResetExpiry:  config.PasswordResetExpiry,
		rateLimitEnabled:     config.RateLimitEnabled,
		rateLimitMaxRequests: config.RateLimitMaxRequests,
		jwtExpiry:            tokenAuth.JwtExpiry,
		jwtRefreshExpiry:     tokenAuth.JwtRefreshExpiry,
		txHandler:            base.NewTxHandler(db),
		db:                   db,
	}, nil
}

// WithTx returns a new service instance with transaction-aware repositories
// The factory pattern simplifies this - repositories use TxFromContext(ctx) to detect transactions
func (s *Service) WithTx(tx bun.Tx) any {
	return &Service{
		repos:                s.repos, // Repositories detect transaction from context via TxFromContext(ctx)
		tokenAuth:            s.tokenAuth,
		dispatcher:           s.dispatcher,
		defaultFrom:          s.defaultFrom,
		frontendURL:          s.frontendURL,
		passwordResetExpiry:  s.passwordResetExpiry,
		rateLimitEnabled:     s.rateLimitEnabled,
		rateLimitMaxRequests: s.rateLimitMaxRequests,
		jwtExpiry:            s.jwtExpiry,
		jwtRefreshExpiry:     s.jwtRefreshExpiry,
		txHandler:            s.txHandler.WithTx(tx),
		db:                   s.db,
	}
}

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

// Account metadata loading is in account_metadata.go

// buildJWTClaims constructs JWT claims from account and metadata
func (s *Service) buildJWTClaims(
	account *auth.Account,
	token *auth.Token,
	metadata *accountMetadata,
	email string,
) (jwt.AppClaims, jwt.RefreshClaims) {
	appClaims := jwt.AppClaims{
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

	refreshClaims := jwt.RefreshClaims{
		ID:    int(token.ID),
		Token: token.Token,
	}

	return appClaims, refreshClaims
}

// generateAndLogTokens generates JWT token pair and logs the authentication event
func (s *Service) generateAndLogTokens(
	ctx context.Context,
	accountID int64,
	appClaims jwt.AppClaims,
	refreshClaims jwt.RefreshClaims,
	ipAddress, userAgent, eventType string,
) (string, string, error) {
	accessToken, refreshToken, err := s.tokenAuth.GenTokenPair(appClaims, refreshClaims)
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

// ValidateToken validates an access token and returns the associated account
func (s *Service) ValidateToken(ctx context.Context, tokenString string) (*auth.Account, error) {
	// Parse and validate JWT token
	jwtToken, err := s.tokenAuth.JwtAuth.Decode(tokenString)
	if err != nil {
		return nil, &AuthError{Op: "validate token", Err: ErrInvalidToken}
	}

	// Extract claims
	claims := extractClaims(jwtToken)

	// Parse claims into AppClaims
	var appClaims jwt.AppClaims
	err = appClaims.ParseClaims(claims)
	if err != nil {
		return nil, &AuthError{Op: "parse claims", Err: ErrInvalidToken}
	}

	// Get account by ID
	account, err := s.repos.Account.FindByID(ctx, int64(appClaims.ID))
	if err != nil {
		return nil, &AuthError{Op: opGetAccount, Err: ErrAccountNotFound}
	}

	// Ensure account is active
	if !account.Active {
		return nil, &AuthError{Op: "validate token", Err: ErrAccountInactive}
	}

	// Load roles and permissions if not already loaded
	s.ensureAccountRolesLoaded(ctx, account)
	s.ensureAccountPermissionsLoaded(ctx, account)

	return account, nil
}

// RefreshToken, RefreshTokenWithAudit, Logout, LogoutWithAudit are in token_refresh.go

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

	valid, err := auth.VerifyPassword(currentPassword, *account.PasswordHash)
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
	account, err := s.repos.Account.FindByID(ctx, int64(id))
	if err != nil {
		return nil, &AuthError{Op: opGetAccount, Err: ErrAccountNotFound}
	}
	return account, nil
}

// GetAccountByEmail retrieves an account by email
func (s *Service) GetAccountByEmail(ctx context.Context, email string) (*auth.Account, error) {
	// Normalize email
	email = strings.TrimSpace(strings.ToLower(email))

	account, err := s.repos.Account.FindByEmail(ctx, email)
	if err != nil {
		return nil, &AuthError{Op: "get account by email", Err: ErrAccountNotFound}
	}
	return account, nil
}

// Helper methods

// getAccountPermissions retrieves all permissions for an account (both direct and role-based)
func (s *Service) getAccountPermissions(ctx context.Context, accountID int64) ([]*auth.Permission, error) {
	// Get permissions directly assigned to the account
	directPermissions, err := s.repos.Permission.FindByAccountID(ctx, accountID)
	if err != nil {
		return []*auth.Permission{}, err
	}

	// Create a map to prevent duplicate permissions
	permMap := make(map[int64]*auth.Permission)

	// Add direct permissions to the map
	s.addPermissionsToMap(permMap, directPermissions)

	// Add role-based permissions to the map
	s.addRolePermissionsToMap(ctx, accountID, permMap)

	// Convert map to slice
	return s.convertPermissionMapToSlice(permMap), nil
}

// addPermissionsToMap adds permissions to the map to prevent duplicates
func (s *Service) addPermissionsToMap(permMap map[int64]*auth.Permission, permissions []*auth.Permission) {
	for _, p := range permissions {
		permMap[p.ID] = p
	}
}

// addRolePermissionsToMap adds permissions from account roles to the map
func (s *Service) addRolePermissionsToMap(ctx context.Context, accountID int64, permMap map[int64]*auth.Permission) {
	accountRoles, err := s.repos.AccountRole.FindByAccountID(ctx, accountID)
	if err != nil {
		return // Continue even if error occurs
	}

	for _, ar := range accountRoles {
		if ar.RoleID <= 0 {
			continue
		}

		rolePermissions, err := s.repos.Permission.FindByRoleID(ctx, ar.RoleID)
		if err != nil {
			continue // Continue even if error occurs
		}

		s.addPermissionsToMap(permMap, rolePermissions)
	}
}

// convertPermissionMapToSlice converts permission map to slice
func (s *Service) convertPermissionMapToSlice(permMap map[int64]*auth.Permission) []*auth.Permission {
	permissions := make([]*auth.Permission, 0, len(permMap))
	for _, p := range permMap {
		permissions = append(permissions, p)
	}
	return permissions
}

// getRoleByName retrieves a role by its name
func (s *Service) getRoleByName(ctx context.Context, name string) (*auth.Role, error) {
	return s.repos.Permission.FindByRoleByName(ctx, name)
}

// Helper functions

// extractClaims extracts all claims from a jwt token into a map
func extractClaims(token jwx.Token) map[string]interface{} {
	claims := make(map[string]interface{})

	// Extract private claims
	for k, v := range token.PrivateClaims() {
		claims[k] = v
	}

	// Add registered claims if present
	if sub, ok := token.Get(jwx.SubjectKey); ok {
		claims[jwx.SubjectKey] = sub
	}
	if exp, ok := token.Get(jwx.ExpirationKey); ok {
		claims[jwx.ExpirationKey] = exp
	}

	return claims
}

// Token Management

// CleanupExpiredTokens removes expired authentication tokens
func (s *Service) CleanupExpiredTokens(ctx context.Context) (int, error) {
	count, err := s.repos.Token.DeleteExpiredTokens(ctx)
	if err != nil {
		return 0, &AuthError{Op: "cleanup expired tokens", Err: err}
	}
	return count, nil
}

// RevokeAllTokens revokes all tokens for an account
func (s *Service) RevokeAllTokens(ctx context.Context, accountID int) error {
	if err := s.repos.Token.DeleteByAccountID(ctx, int64(accountID)); err != nil {
		return &AuthError{Op: "revoke all tokens", Err: err}
	}
	return nil
}

// GetActiveTokens retrieves all active tokens for an account
func (s *Service) GetActiveTokens(ctx context.Context, accountID int) ([]*auth.Token, error) {
	filters := map[string]interface{}{
		"account_id": int64(accountID),
		"active":     true,
	}

	tokens, err := s.repos.Token.List(ctx, filters)
	if err != nil {
		return nil, &AuthError{Op: "get active tokens", Err: err}
	}
	return tokens, nil
}

// logAuthEvent logs an authentication event for audit purposes
func (s *Service) logAuthEvent(ctx context.Context, accountID int64, eventType string, success bool, ipAddress, userAgent string, errorMessage string) {
	event := audit.NewAuthEvent(accountID, eventType, success, ipAddress)
	event.UserAgent = userAgent
	if errorMessage != "" {
		event.ErrorMessage = errorMessage
	}

	// Log asynchronously to avoid blocking auth operations
	go func() {
		// Create a new context with timeout for the logging operation
		// Use WithoutCancel to detach from parent cancellation while preserving context values
		logCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()

		if err := s.repos.AuthEvent.Create(logCtx, event); err != nil {
			// Log the error but don't fail the auth operation
			if logger.Logger != nil {
				logger.Logger.WithError(err).Warn("Failed to log auth event")
			}
		}
	}()
}
