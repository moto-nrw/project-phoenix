package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	jwx "github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
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
	Dispatcher          *email.Dispatcher
	DefaultFrom         email.Email
	FrontendURL         string
	PasswordResetExpiry time.Duration
}

// NewServiceConfig creates and validates a new ServiceConfig
func NewServiceConfig(
	dispatcher *email.Dispatcher,
	defaultFrom email.Email,
	frontendURL string,
	passwordResetExpiry time.Duration,
) (*ServiceConfig, error) {
	if frontendURL == "" {
		return nil, errors.New("frontendURL cannot be empty")
	}
	if passwordResetExpiry <= 0 {
		return nil, errors.New("passwordResetExpiry must be positive")
	}

	return &ServiceConfig{
		Dispatcher:          dispatcher,
		DefaultFrom:         defaultFrom,
		FrontendURL:         frontendURL,
		PasswordResetExpiry: passwordResetExpiry,
	}, nil
}

// Service provides authentication and authorization functionality
type Service struct {
	repos               *repositories.Factory
	tokenAuth           *jwt.TokenAuth
	dispatcher          *email.Dispatcher
	defaultFrom         email.Email
	frontendURL         string
	passwordResetExpiry time.Duration
	jwtExpiry           time.Duration
	jwtRefreshExpiry    time.Duration
	txHandler           *base.TxHandler
	db                  *bun.DB
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
		repos:               repos,
		tokenAuth:           tokenAuth,
		dispatcher:          config.Dispatcher,
		defaultFrom:         config.DefaultFrom,
		frontendURL:         config.FrontendURL,
		passwordResetExpiry: config.PasswordResetExpiry,
		jwtExpiry:           tokenAuth.JwtExpiry,
		jwtRefreshExpiry:    tokenAuth.JwtRefreshExpiry,
		txHandler:           base.NewTxHandler(db),
		db:                  db,
	}, nil
}

// WithTx returns a new service instance with transaction-aware repositories
// The factory pattern simplifies this - repositories use TxFromContext(ctx) to detect transactions
func (s *Service) WithTx(tx bun.Tx) interface{} {
	return &Service{
		repos:               s.repos, // Repositories detect transaction from context via TxFromContext(ctx)
		tokenAuth:           s.tokenAuth,
		dispatcher:          s.dispatcher,
		defaultFrom:         s.defaultFrom,
		frontendURL:         s.frontendURL,
		passwordResetExpiry: s.passwordResetExpiry,
		jwtExpiry:           s.jwtExpiry,
		jwtRefreshExpiry:    s.jwtRefreshExpiry,
		txHandler:           s.txHandler.WithTx(tx),
		db:                  s.db,
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

	valid, err := userpass.VerifyPassword(password, *account.PasswordHash)
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
		log.Printf("Login race condition detected for account %d, retrying (attempt %d/%d)", account.ID, attempt+1, maxRetries)
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
			log.Printf("Warning: failed to clean up old tokens for account %d: %v", account.ID, err)
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

// accountMetadata holds account-related metadata for JWT claims
type accountMetadata struct {
	roleNames      []string
	permissionStrs []string
	username       string
	firstName      string
	lastName       string
	isAdmin        bool
	isTeacher      bool
}

// loadAccountMetadata loads roles, permissions, and person information
// Returns partial data with logged warnings if any lookups fail
func (s *Service) loadAccountMetadata(ctx context.Context, account *auth.Account) *accountMetadata {
	s.ensureAccountRolesLoaded(ctx, account)

	permissions := s.loadAccountPermissions(ctx, account.ID)
	roleNames := s.extractRoleNames(account.Roles)
	permissionStrs := s.extractPermissionNames(permissions)

	username := s.extractUsername(account)
	firstName, lastName := s.loadPersonNames(ctx, account.ID)
	isAdmin, isTeacher := s.checkRoleFlags(roleNames)

	return &accountMetadata{
		roleNames:      roleNames,
		permissionStrs: permissionStrs,
		username:       username,
		firstName:      firstName,
		lastName:       lastName,
		isAdmin:        isAdmin,
		isTeacher:      isTeacher,
	}
}

// ensureAccountRolesLoaded loads account roles if not already loaded
func (s *Service) ensureAccountRolesLoaded(ctx context.Context, account *auth.Account) {
	if len(account.Roles) > 0 {
		return
	}

	accountRoles, err := s.repos.AccountRole.FindByAccountID(ctx, account.ID)
	if err != nil {
		log.Printf("Warning: failed to load roles for account %d: %v", account.ID, err)
		return
	}

	for _, ar := range accountRoles {
		if ar.Role != nil {
			account.Roles = append(account.Roles, ar.Role)
		}
	}
}

// loadAccountPermissions retrieves permissions for the account
func (s *Service) loadAccountPermissions(ctx context.Context, accountID int64) []*auth.Permission {
	permissions, err := s.getAccountPermissions(ctx, accountID)
	if err != nil {
		log.Printf("Warning: failed to load permissions for account %d: %v", accountID, err)
		return []*auth.Permission{}
	}
	return permissions
}

// ensureAccountPermissionsLoaded loads account permissions if not already loaded
func (s *Service) ensureAccountPermissionsLoaded(ctx context.Context, account *auth.Account) {
	if len(account.Permissions) > 0 {
		return
	}

	permissions, err := s.getAccountPermissions(ctx, account.ID)
	if err != nil {
		log.Printf("Warning: failed to load permissions for account %d: %v", account.ID, err)
		return
	}

	account.Permissions = permissions
}

// extractRoleNames converts roles to string slice
func (s *Service) extractRoleNames(roles []*auth.Role) []string {
	roleNames := make([]string, 0, len(roles))
	for _, role := range roles {
		roleNames = append(roleNames, role.Name)
	}
	return roleNames
}

// extractPermissionNames converts permissions to string slice
func (s *Service) extractPermissionNames(permissions []*auth.Permission) []string {
	permissionStrs := make([]string, 0, len(permissions))
	for _, perm := range permissions {
		permissionStrs = append(permissionStrs, perm.GetFullName())
	}
	return permissionStrs
}

// extractUsername safely extracts username from account
func (s *Service) extractUsername(account *auth.Account) string {
	if account.Username != nil {
		return *account.Username
	}
	return ""
}

// loadPersonNames retrieves first and last name from person record
func (s *Service) loadPersonNames(ctx context.Context, accountID int64) (string, string) {
	person, err := s.repos.Person.FindByAccountID(ctx, accountID)
	if err != nil || person == nil {
		return "", ""
	}
	return person.FirstName, person.LastName
}

// checkRoleFlags determines if account has admin or teacher roles
func (s *Service) checkRoleFlags(roleNames []string) (bool, bool) {
	isAdmin := false
	isTeacher := false

	for _, roleName := range roleNames {
		if roleName == "admin" {
			isAdmin = true
		}
		if roleName == "teacher" {
			isTeacher = true
		}
	}

	return isAdmin, isTeacher
}

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
		log.Printf("Failed to create account role: %v", err)
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
		log.Printf("Failed to find default user role: %v", err)
		return 0, nil // Return 0 to indicate no role (continue without role assignment)
	}

	return userRole.ID, nil
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

// RefreshToken generates new token pair from a refresh token
func (s *Service) RefreshToken(ctx context.Context, refreshTokenStr string) (string, string, error) {
	return s.RefreshTokenWithAudit(ctx, refreshTokenStr, "", "")
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

// Logout invalidates a refresh token
func (s *Service) Logout(ctx context.Context, refreshTokenStr string) error {
	return s.LogoutWithAudit(ctx, refreshTokenStr, "", "")
}

// LogoutWithAudit invalidates a refresh token with audit logging
func (s *Service) LogoutWithAudit(ctx context.Context, refreshTokenStr, ipAddress, userAgent string) error {
	// Parse JWT refresh token
	jwtToken, err := s.tokenAuth.JwtAuth.Decode(refreshTokenStr)
	if err != nil {
		return &AuthError{Op: "parse refresh token", Err: ErrInvalidToken}
	}

	// Extract claims
	claims := extractClaims(jwtToken)

	// Parse refresh token claims
	var refreshClaims jwt.RefreshClaims
	err = refreshClaims.ParseClaims(claims)
	if err != nil {
		return &AuthError{Op: "parse refresh claims", Err: ErrInvalidToken}
	}

	// Get token from database to find the account ID
	dbToken, err := s.repos.Token.FindByToken(ctx, refreshClaims.Token)
	if err != nil {
		// Token not found, consider logout successful
		return nil
	}

	// Delete ALL tokens for this account to ensure complete logout
	// This ensures that all sessions (access and refresh tokens) are invalidated
	err = s.repos.Token.DeleteByAccountID(ctx, dbToken.AccountID)
	if err != nil {
		// Log the error but don't fail the logout
		log.Printf("Warning: failed to delete all tokens for account %d during logout: %v", dbToken.AccountID, err)
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
			log.Printf("Failed to log auth event: %v", err)
		}
	}()
}
