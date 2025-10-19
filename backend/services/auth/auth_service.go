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
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

const (
	passwordResetRateLimitThreshold = 3
)

// Updated Service struct with all repositories

type Service struct {
	accountRepo                auth.AccountRepository
	accountParentRepo          auth.AccountParentRepository // Add this
	accountRoleRepo            auth.AccountRoleRepository
	accountPermissionRepo      auth.AccountPermissionRepository
	permissionRepo             auth.PermissionRepository
	roleRepo                   auth.RoleRepository           // Add this
	rolePermissionRepo         auth.RolePermissionRepository // Add this
	tokenRepo                  auth.TokenRepository
	passwordResetTokenRepo     auth.PasswordResetTokenRepository // Add this
	passwordResetRateLimitRepo auth.PasswordResetRateLimitRepository
	personRepo                 users.PersonRepository    // Add this for first name
	authEventRepo              audit.AuthEventRepository // Add for audit logging
	tokenAuth                  *jwt.TokenAuth
	mailer                     email.Mailer
	defaultFrom                email.Email
	frontendURL                string
	passwordResetExpiry        time.Duration
	jwtExpiry                  time.Duration
	jwtRefreshExpiry           time.Duration
	txHandler                  *base.TxHandler
	db                         *bun.DB // Add database connection
}

// Updated NewService constructor

func NewService(
	accountRepo auth.AccountRepository,
	accountRoleRepo auth.AccountRoleRepository,
	accountPermissionRepo auth.AccountPermissionRepository,
	permissionRepo auth.PermissionRepository,
	tokenRepo auth.TokenRepository,
	accountParentRepo auth.AccountParentRepository, // Add this
	roleRepo auth.RoleRepository, // Add this
	rolePermissionRepo auth.RolePermissionRepository, // Add this
	passwordResetTokenRepo auth.PasswordResetTokenRepository, // Add this
	passwordResetRateLimitRepo auth.PasswordResetRateLimitRepository,
	personRepo users.PersonRepository, // Add this for first name
	authEventRepo audit.AuthEventRepository, // Add for audit logging
	db *bun.DB,
	mailer email.Mailer,
	frontendURL string,
	defaultFrom email.Email,
	passwordResetExpiry time.Duration,
) (*Service, error) {

	tokenAuth, err := jwt.NewTokenAuth()
	if err != nil {
		return nil, &AuthError{Op: "create token auth", Err: err}
	}

	return &Service{
		accountRepo:                accountRepo,
		accountParentRepo:          accountParentRepo, // Add this
		accountRoleRepo:            accountRoleRepo,
		accountPermissionRepo:      accountPermissionRepo,
		permissionRepo:             permissionRepo,
		roleRepo:                   roleRepo,           // Add this
		rolePermissionRepo:         rolePermissionRepo, // Add this
		tokenRepo:                  tokenRepo,
		passwordResetTokenRepo:     passwordResetTokenRepo, // Add this
		passwordResetRateLimitRepo: passwordResetRateLimitRepo,
		personRepo:                 personRepo,    // Add this for first name
		authEventRepo:              authEventRepo, // Add for audit logging
		tokenAuth:                  tokenAuth,
		mailer:                     mailer,
		defaultFrom:                defaultFrom,
		frontendURL:                frontendURL,
		passwordResetExpiry:        passwordResetExpiry,
		jwtExpiry:                  tokenAuth.JwtExpiry,
		jwtRefreshExpiry:           tokenAuth.JwtRefreshExpiry,
		txHandler:                  base.NewTxHandler(db),
		db:                         db, // Add database connection
	}, nil
}

// Updated WithTx method to include all repositories

func (s *Service) WithTx(tx bun.Tx) interface{} {
	// Get repositories with transaction if they implement the TransactionalRepository interface
	var accountRepo = s.accountRepo
	var accountParentRepo = s.accountParentRepo // Add this
	var accountRoleRepo = s.accountRoleRepo
	var accountPermissionRepo = s.accountPermissionRepo
	var permissionRepo = s.permissionRepo
	var roleRepo = s.roleRepo                     // Add this
	var rolePermissionRepo = s.rolePermissionRepo // Add this
	var tokenRepo = s.tokenRepo
	var passwordResetTokenRepo = s.passwordResetTokenRepo // Add this
	var passwordResetRateLimitRepo = s.passwordResetRateLimitRepo
	var personRepo = s.personRepo       // Add this for first name
	var authEventRepo = s.authEventRepo // Add for audit logging

	// Try to cast repositories to TransactionalRepository and apply the transaction
	if txRepo, ok := s.accountRepo.(base.TransactionalRepository); ok {
		accountRepo = txRepo.WithTx(tx).(auth.AccountRepository)
	}
	if txRepo, ok := s.accountParentRepo.(base.TransactionalRepository); ok { // Add this
		accountParentRepo = txRepo.WithTx(tx).(auth.AccountParentRepository)
	}
	if txRepo, ok := s.accountRoleRepo.(base.TransactionalRepository); ok {
		accountRoleRepo = txRepo.WithTx(tx).(auth.AccountRoleRepository)
	}
	if txRepo, ok := s.accountPermissionRepo.(base.TransactionalRepository); ok {
		accountPermissionRepo = txRepo.WithTx(tx).(auth.AccountPermissionRepository)
	}
	if txRepo, ok := s.permissionRepo.(base.TransactionalRepository); ok {
		permissionRepo = txRepo.WithTx(tx).(auth.PermissionRepository)
	}
	if txRepo, ok := s.roleRepo.(base.TransactionalRepository); ok { // Add this
		roleRepo = txRepo.WithTx(tx).(auth.RoleRepository)
	}
	if txRepo, ok := s.rolePermissionRepo.(base.TransactionalRepository); ok { // Add this
		rolePermissionRepo = txRepo.WithTx(tx).(auth.RolePermissionRepository)
	}
	if txRepo, ok := s.tokenRepo.(base.TransactionalRepository); ok {
		tokenRepo = txRepo.WithTx(tx).(auth.TokenRepository)
	}
	if txRepo, ok := s.passwordResetTokenRepo.(base.TransactionalRepository); ok { // Add this
		passwordResetTokenRepo = txRepo.WithTx(tx).(auth.PasswordResetTokenRepository)
	}
	if txRepo, ok := s.passwordResetRateLimitRepo.(base.TransactionalRepository); ok {
		passwordResetRateLimitRepo = txRepo.WithTx(tx).(auth.PasswordResetRateLimitRepository)
	}
	if txRepo, ok := s.personRepo.(base.TransactionalRepository); ok { // Add this for first name
		personRepo = txRepo.WithTx(tx).(users.PersonRepository)
	}
	if txRepo, ok := s.authEventRepo.(base.TransactionalRepository); ok { // Add for audit logging
		authEventRepo = txRepo.WithTx(tx).(audit.AuthEventRepository)
	}

	// Return a new service with the transaction
	return &Service{
		accountRepo:                accountRepo,
		accountParentRepo:          accountParentRepo, // Add this
		accountRoleRepo:            accountRoleRepo,
		accountPermissionRepo:      accountPermissionRepo,
		permissionRepo:             permissionRepo,
		roleRepo:                   roleRepo,           // Add this
		rolePermissionRepo:         rolePermissionRepo, // Add this
		tokenRepo:                  tokenRepo,
		passwordResetTokenRepo:     passwordResetTokenRepo, // Add this
		passwordResetRateLimitRepo: passwordResetRateLimitRepo,
		personRepo:                 personRepo,    // Add this for first name
		authEventRepo:              authEventRepo, // Add for audit logging
		tokenAuth:                  s.tokenAuth,
		mailer:                     s.mailer,
		defaultFrom:                s.defaultFrom,
		frontendURL:                s.frontendURL,
		passwordResetExpiry:        s.passwordResetExpiry,
		jwtExpiry:                  s.jwtExpiry,
		jwtRefreshExpiry:           s.jwtRefreshExpiry,
		txHandler:                  s.txHandler.WithTx(tx),
		db:                         s.db, // Add database connection
	}
}

// Login authenticates a user and returns access and refresh tokens
func (s *Service) Login(ctx context.Context, email, password string) (string, string, error) {
	return s.LoginWithAudit(ctx, email, password, "", "")
}

// LoginWithAudit authenticates a user and returns access and refresh tokens with audit logging
func (s *Service) LoginWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (string, string, error) {
	// Normalize email
	email = strings.TrimSpace(strings.ToLower(email))

	// Get account by email
	account, err := s.accountRepo.FindByEmail(ctx, email)
	if err != nil {
		// Log failed login attempt with unknown account ID (use 0)
		if ipAddress != "" {
			s.logAuthEvent(ctx, 0, audit.EventTypeLogin, false, ipAddress, userAgent, "Account not found")
		}
		return "", "", &AuthError{Op: "login", Err: ErrAccountNotFound}
	}

	// Check if account is active
	if !account.Active {
		// Log failed login attempt
		if ipAddress != "" {
			s.logAuthEvent(ctx, account.ID, audit.EventTypeLogin, false, ipAddress, userAgent, "Account inactive")
		}
		return "", "", &AuthError{Op: "login", Err: ErrAccountInactive}
	}

	// Verify password
	if account.PasswordHash == nil || *account.PasswordHash == "" {
		return "", "", &AuthError{Op: "login", Err: ErrInvalidCredentials}
	}

	valid, err := userpass.VerifyPassword(password, *account.PasswordHash)
	if err != nil || !valid {
		// Log failed login attempt
		if ipAddress != "" {
			s.logAuthEvent(ctx, account.ID, audit.EventTypeLogin, false, ipAddress, userAgent, "Invalid password")
		}
		return "", "", &AuthError{Op: "login", Err: ErrInvalidCredentials}
	}

	// Create refresh token with new family
	tokenStr := uuid.Must(uuid.NewV4()).String()
	familyID := uuid.Must(uuid.NewV4()).String() // New family for login
	identifier := "Service login"
	now := time.Now()
	token := &auth.Token{
		Token:      tokenStr,
		AccountID:  account.ID,
		Expiry:     now.Add(s.jwtRefreshExpiry),
		Mobile:     false, // This would come from a user agent in a real request
		Identifier: &identifier,
		FamilyID:   familyID,
		Generation: 0, // First token in family
	}

	// Execute in transaction using txHandler with retry for concurrent logins
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
			// Get transactional service
			txService := s.WithTx(tx).(AuthService)

			// Clean up existing tokens for this account
			// This prevents token accumulation while allowing multiple active sessions
			//
			// Option 2: Keep only the N most recent tokens (multiple sessions)
			// This allows frontend sessions to remain active while cleaning up old tokens
			const maxTokensPerAccount = 5
			if err := txService.(*Service).tokenRepo.CleanupOldTokensForAccount(ctx, account.ID, maxTokensPerAccount); err != nil {
				log.Printf("Warning: failed to clean up old tokens for account %d: %v", account.ID, err)
			}

			// Option 1 (Alternative): Delete ALL existing tokens (single session only)
			// Uncomment below and comment out Option 2 to enforce single session
			// if err := txService.(*Service).tokenRepo.DeleteByAccountID(ctx, account.ID); err != nil {
			//     // Log the error but don't fail the login
			//     // This ensures users can still login even if cleanup fails
			//     log.Printf("Warning: failed to clean up existing tokens for account %d: %v", account.ID, err)
			// }

			// Create token using the transaction-aware repositories
			if err := txService.(*Service).tokenRepo.Create(ctx, token); err != nil {
				// Check if it's a duplicate key error on family_id/generation
				if strings.Contains(err.Error(), "uk_tokens_family_generation") {
					// Another concurrent login is happening, regenerate family ID
					token.FamilyID = uuid.Must(uuid.NewV4()).String()
					return err // Will retry with new family ID
				}
				return err
			}

			// Update last login
			loginTime := time.Now()
			account.LastLogin = &loginTime
			return txService.(*Service).accountRepo.Update(ctx, account)
		})

		// If successful or non-retryable error, break
		if err == nil || !strings.Contains(err.Error(), "uk_tokens_family_generation") {
			break
		}

		// Log retry attempt
		log.Printf("Login race condition detected for account %d, retrying (attempt %d/%d)", account.ID, attempt+1, maxRetries)
	}

	if err != nil {
		return "", "", &AuthError{Op: "login transaction", Err: err}
	}

	// Retrieve account roles if not loaded
	if len(account.Roles) == 0 {
		accountRoles, err := s.accountRoleRepo.FindByAccountID(ctx, account.ID)
		if err != nil {
			// Continue even if role retrieval fails, just log the error
			// In a real implementation, you would log this error
		} else {
			// Extract roles from account roles
			for _, ar := range accountRoles {
				if ar.Role != nil {
					account.Roles = append(account.Roles, ar.Role)
				}
			}
		}
	}

	// Retrieve permissions for the account
	permissions, err := s.getAccountPermissions(ctx, account.ID)
	if err != nil {
		// Continue even if permission retrieval fails, just log the error
		// In a real implementation, you would log this error
		permissions = []*auth.Permission{} // Empty array with correct type
	}

	// Convert roles to string slice for token
	var roleNames []string
	for _, role := range account.Roles {
		roleNames = append(roleNames, role.Name)
	}

	// Extract permission names into strings
	var permissionStrs []string
	for _, perm := range permissions {
		permissionStrs = append(permissionStrs, perm.GetFullName())
	}

	// Extract username
	username := ""
	if account.Username != nil {
		username = *account.Username
	}

	firstName := ""
	lastName := ""
	person, err := s.personRepo.FindByAccountID(ctx, account.ID)
	if err == nil && person != nil {
		firstName = person.FirstName
		lastName = person.LastName
	}

	// Check for static role flags
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

	// Generate token pair
	// Create JWT claims
	appClaims := jwt.AppClaims{
		ID:          int(account.ID),
		Sub:         email, // Use email as subject
		Username:    username,
		FirstName:   firstName,
		LastName:    lastName,
		Roles:       roleNames,
		Permissions: permissionStrs, // Use string array here
		IsAdmin:     isAdmin,
		IsTeacher:   isTeacher,
	}

	refreshClaims := jwt.RefreshClaims{
		ID:    int(token.ID),
		Token: token.Token,
	}

	// Generate tokens
	accessToken, refreshToken, err := s.tokenAuth.GenTokenPair(appClaims, refreshClaims)
	if err != nil {
		return "", "", &AuthError{Op: "generate tokens", Err: err}
	}

	// Log successful login
	if ipAddress != "" {
		s.logAuthEvent(ctx, account.ID, audit.EventTypeLogin, true, ipAddress, userAgent, "")
	}

	return accessToken, refreshToken, nil
}

// Register creates a new user account
func (s *Service) Register(ctx context.Context, email, username, name, password string) (*auth.Account, error) {
	// Normalize input
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)

	// Validate password strength
	if err := ValidatePasswordStrength(password); err != nil {
		return nil, &AuthError{Op: "register", Err: err}
	}

	// Check if email already exists
	_, err := s.accountRepo.FindByEmail(ctx, email)
	if err == nil {
		return nil, &AuthError{Op: "register", Err: ErrEmailAlreadyExists}
	}

	// Check if username already exists
	_, err = s.accountRepo.FindByUsername(ctx, username)
	if err == nil {
		return nil, &AuthError{Op: "register", Err: ErrUsernameAlreadyExists}
	}

	// Hash password
	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, &AuthError{Op: "hash password", Err: err}
	}

	usernamePtr := &username
	now := time.Now()

	// Create account
	account := &auth.Account{
		Email:        email,
		Username:     usernamePtr,
		Active:       true,
		PasswordHash: &passwordHash,
		LastLogin:    &now,
	}

	// Execute in transaction using txHandler
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(AuthService)

		// Create account with transaction
		if err := txService.(*Service).accountRepo.Create(ctx, account); err != nil {
			return err
		}

		// Find the default user role
		userRole, err := txService.(*Service).getRoleByName(ctx, "user")
		if err == nil && userRole != nil {
			// Create account role mapping
			accountRole := &auth.AccountRole{
				AccountID: account.ID,
				RoleID:    userRole.ID,
			}
			err = txService.(*Service).accountRoleRepo.Create(ctx, accountRole)
			if err != nil {
				// Log error but continue
				log.Printf("Failed to create account role: %v", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, &AuthError{Op: "register transaction", Err: err}
	}

	return account, nil
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
	account, err := s.accountRepo.FindByID(ctx, int64(appClaims.ID))
	if err != nil {
		return nil, &AuthError{Op: "get account", Err: ErrAccountNotFound}
	}

	// Ensure account is active
	if !account.Active {
		return nil, &AuthError{Op: "validate token", Err: ErrAccountInactive}
	}

	// Load roles if not already loaded
	if len(account.Roles) == 0 {
		accountRoles, err := s.accountRoleRepo.FindByAccountID(ctx, account.ID)
		if err == nil {
			// Extract roles from account roles
			for _, ar := range accountRoles {
				if ar.Role != nil {
					account.Roles = append(account.Roles, ar.Role)
				}
			}
		}
	}

	// Load permissions if not already loaded
	if len(account.Permissions) == 0 {
		permissions, err := s.getAccountPermissions(ctx, account.ID)
		if err == nil {
			account.Permissions = permissions
		}
	}

	return account, nil
}

// RefreshToken generates new token pair from a refresh token
func (s *Service) RefreshToken(ctx context.Context, refreshTokenStr string) (string, string, error) {
	return s.RefreshTokenWithAudit(ctx, refreshTokenStr, "", "")
}

// RefreshTokenWithAudit generates new token pair from a refresh token with audit logging
func (s *Service) RefreshTokenWithAudit(ctx context.Context, refreshTokenStr, ipAddress, userAgent string) (string, string, error) {
	// Parse JWT refresh token
	jwtToken, err := s.tokenAuth.JwtAuth.Decode(refreshTokenStr)
	if err != nil {
		return "", "", &AuthError{Op: "parse refresh token", Err: ErrInvalidToken}
	}

	// Extract claims
	claims := extractClaims(jwtToken)

	// Parse refresh token claims
	var refreshClaims jwt.RefreshClaims
	err = refreshClaims.ParseClaims(claims)
	if err != nil {
		return "", "", &AuthError{Op: "parse refresh claims", Err: ErrInvalidToken}
	}

	var dbToken *auth.Token
	var account *auth.Account
	var newToken *auth.Token

	// Execute token lookup and validation in transaction with row lock
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Context already has the transaction from RunInTx

		// Get token from database with row lock
		var lookupErr error
		dbToken, lookupErr = s.tokenRepo.FindByTokenForUpdate(ctx, refreshClaims.Token)
		if lookupErr != nil {
			return &AuthError{Op: "get token", Err: ErrTokenNotFound}
		}

		// Check if token is expired
		if time.Now().After(dbToken.Expiry) {
			// Delete expired token
			_ = s.tokenRepo.Delete(ctx, dbToken.ID)
			// Log expired token attempt
			if ipAddress != "" && dbToken.AccountID > 0 {
				s.logAuthEvent(ctx, dbToken.AccountID, audit.EventTypeTokenExpired, false, ipAddress, userAgent, "Token expired")
			}
			return &AuthError{Op: "check token expiry", Err: ErrTokenExpired}
		}

		// Token family tracking - detect potential token theft
		if dbToken.FamilyID != "" {
			// Check if this is the latest token in the family
			latestToken, err := s.tokenRepo.GetLatestTokenInFamily(ctx, dbToken.FamilyID)
			if err == nil && latestToken != nil && latestToken.Generation > dbToken.Generation {
				// This token has already been refreshed - potential theft detected!
				// Delete entire token family to force re-authentication
				_ = s.tokenRepo.DeleteByFamilyID(ctx, dbToken.FamilyID)

				// Log security event
				if ipAddress != "" {
					s.logAuthEvent(ctx, dbToken.AccountID, audit.EventTypeTokenRefresh, false, ipAddress, userAgent, "Token theft detected - family invalidated")
				}

				return &AuthError{Op: "token theft detection", Err: ErrInvalidToken}
			}
		}

		// Get account
		var accountErr error
		account, accountErr = s.accountRepo.FindByID(ctx, dbToken.AccountID)
		if accountErr != nil {
			return &AuthError{Op: "get account", Err: ErrAccountNotFound}
		}

		// Check if account is active
		if !account.Active {
			// Log failed refresh attempt
			if ipAddress != "" {
				s.logAuthEvent(ctx, account.ID, audit.EventTypeTokenRefresh, false, ipAddress, userAgent, "Account inactive")
			}
			return &AuthError{Op: "check account status", Err: ErrAccountInactive}
		}

		// Generate new refresh token in the same family
		newTokenStr := uuid.Must(uuid.NewV4()).String()
		now := time.Now()

		// Create new token with incremented generation
		newToken = &auth.Token{
			Token:      newTokenStr,
			AccountID:  account.ID,
			Expiry:     now.Add(s.jwtRefreshExpiry),
			Mobile:     dbToken.Mobile,
			Identifier: dbToken.Identifier,
			FamilyID:   dbToken.FamilyID,
			Generation: dbToken.Generation + 1, // Increment generation
		}

		// Delete the old token
		if err := s.tokenRepo.Delete(ctx, dbToken.ID); err != nil {
			return err
		}

		// Create the new token
		if err := s.tokenRepo.Create(ctx, newToken); err != nil {
			return err
		}

		// Update last login
		loginTime := time.Now()
		account.LastLogin = &loginTime
		return s.accountRepo.Update(ctx, account)
	})

	if err != nil {
		return "", "", &AuthError{Op: "refresh transaction", Err: err}
	}

	// Load roles if not loaded
	if len(account.Roles) == 0 {
		accountRoles, err := s.accountRoleRepo.FindByAccountID(ctx, account.ID)
		if err == nil {
			// Extract roles from account roles
			for _, ar := range accountRoles {
				if ar.Role != nil {
					account.Roles = append(account.Roles, ar.Role)
				}
			}
		}
	}

	// Load permissions
	permissions, err := s.getAccountPermissions(ctx, account.ID)
	if err != nil {
		// Continue even if permission retrieval fails, just log the error
		permissions = []*auth.Permission{} // Empty array with correct type
	}

	// Extract roles as strings
	var roleNames []string
	for _, role := range account.Roles {
		roleNames = append(roleNames, role.Name)
	}

	// Extract permission names into strings
	var permissionStrs []string
	for _, perm := range permissions {
		permissionStrs = append(permissionStrs, perm.GetFullName())
	}

	// Extract username
	username := ""
	if account.Username != nil {
		username = *account.Username
	}

	// Get person info for first/last name
	firstName := ""
	lastName := ""
	person, err := s.personRepo.FindByAccountID(ctx, account.ID)
	if err == nil && person != nil {
		firstName = person.FirstName
		lastName = person.LastName
	}

	// Check for static role flags
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

	// Generate token pair
	appClaims := jwt.AppClaims{
		ID:          int(account.ID),
		Sub:         account.Email,
		Username:    username,
		FirstName:   firstName,
		LastName:    lastName,
		Roles:       roleNames,
		Permissions: permissionStrs, // Use string array here
		IsAdmin:     isAdmin,
		IsTeacher:   isTeacher,
	}

	newRefreshClaims := jwt.RefreshClaims{
		ID:    int(newToken.ID),
		Token: newToken.Token,
	}

	// Generate tokens
	accessToken, newRefreshToken, err := s.tokenAuth.GenTokenPair(appClaims, newRefreshClaims)
	if err != nil {
		return "", "", &AuthError{Op: "generate tokens", Err: err}
	}

	// Log successful token refresh
	if ipAddress != "" {
		s.logAuthEvent(ctx, account.ID, audit.EventTypeTokenRefresh, true, ipAddress, userAgent, "")
	}

	return accessToken, newRefreshToken, nil
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
	dbToken, err := s.tokenRepo.FindByToken(ctx, refreshClaims.Token)
	if err != nil {
		// Token not found, consider logout successful
		return nil
	}

	// Delete ALL tokens for this account to ensure complete logout
	// This ensures that all sessions (access and refresh tokens) are invalidated
	err = s.tokenRepo.DeleteByAccountID(ctx, dbToken.AccountID)
	if err != nil {
		// Log the error but don't fail the logout
		log.Printf("Warning: failed to delete all tokens for account %d during logout: %v", dbToken.AccountID, err)
		// Still try to delete the specific token
		if deleteErr := s.tokenRepo.Delete(ctx, dbToken.ID); deleteErr != nil {
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
	account, err := s.accountRepo.FindByID(ctx, int64(accountID))
	if err != nil {
		return &AuthError{Op: "get account", Err: ErrAccountNotFound}
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
		return &AuthError{Op: "hash password", Err: err}
	}

	// Update password
	account.PasswordHash = &passwordHash
	if err := s.accountRepo.Update(ctx, account); err != nil {
		return &AuthError{Op: "update account", Err: err}
	}

	return nil
}

// GetAccountByID retrieves an account by ID
func (s *Service) GetAccountByID(ctx context.Context, id int) (*auth.Account, error) {
	account, err := s.accountRepo.FindByID(ctx, int64(id))
	if err != nil {
		return nil, &AuthError{Op: "get account", Err: ErrAccountNotFound}
	}
	return account, nil
}

// GetAccountByEmail retrieves an account by email
func (s *Service) GetAccountByEmail(ctx context.Context, email string) (*auth.Account, error) {
	// Normalize email
	email = strings.TrimSpace(strings.ToLower(email))

	account, err := s.accountRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, &AuthError{Op: "get account by email", Err: ErrAccountNotFound}
	}
	return account, nil
}

// Helper methods

// getAccountPermissions retrieves all permissions for an account (both direct and role-based)
func (s *Service) getAccountPermissions(ctx context.Context, accountID int64) ([]*auth.Permission, error) {
	// Get permissions directly assigned to the account
	directPermissions, err := s.permissionRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		return []*auth.Permission{}, err // Return empty slice with correct type
	}

	// Create a map to prevent duplicate permissions
	permMap := make(map[int64]*auth.Permission)

	// Add direct permissions to the map
	for _, p := range directPermissions {
		permMap[p.ID] = p
	}

	// Get permissions from roles
	// First, get account roles
	accountRoles, err := s.accountRoleRepo.FindByAccountID(ctx, accountID)
	if err == nil { // Continue even if error occurs
		// For each role, get permissions
		for _, ar := range accountRoles {
			if ar.RoleID > 0 {
				rolePermissions, err := s.permissionRepo.FindByRoleID(ctx, ar.RoleID)
				if err == nil { // Continue even if error occurs
					// Add role permissions to the map
					for _, p := range rolePermissions {
						permMap[p.ID] = p
					}
				}
			}
		}
	}

	// Convert map to slice
	permissions := make([]*auth.Permission, 0, len(permMap))
	for _, p := range permMap {
		permissions = append(permissions, p)
	}

	return permissions, nil
}

// getRoleByName retrieves a role by its name
func (s *Service) getRoleByName(ctx context.Context, name string) (*auth.Role, error) {
	return s.permissionRepo.FindByRoleByName(ctx, name)
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

// Add these methods to the existing Service struct in auth_service.go

// Role Management

// CreateRole creates a new role
func (s *Service) CreateRole(ctx context.Context, name, description string) (*auth.Role, error) {
	role := &auth.Role{
		Name:        name,
		Description: description,
	}

	if err := s.roleRepo.Create(ctx, role); err != nil {
		return nil, &AuthError{Op: "create role", Err: err}
	}

	return role, nil
}

// GetRoleByID retrieves a role by its ID
func (s *Service) GetRoleByID(ctx context.Context, id int) (*auth.Role, error) {
	role, err := s.roleRepo.FindByID(ctx, int64(id))
	if err != nil {
		return nil, &AuthError{Op: "get role", Err: err}
	}
	return role, nil
}

// GetRoleByName retrieves a role by its name
func (s *Service) GetRoleByName(ctx context.Context, name string) (*auth.Role, error) {
	role, err := s.roleRepo.FindByName(ctx, name)
	if err != nil {
		return nil, &AuthError{Op: "get role by name", Err: err}
	}
	return role, nil
}

// UpdateRole updates an existing role
func (s *Service) UpdateRole(ctx context.Context, role *auth.Role) error {
	if err := s.roleRepo.Update(ctx, role); err != nil {
		return &AuthError{Op: "update role", Err: err}
	}
	return nil
}

// DeleteRole deletes a role
func (s *Service) DeleteRole(ctx context.Context, id int) error {
	// First remove all account-role mappings for this role
	accountRoles, err := s.accountRoleRepo.FindByRoleID(ctx, int64(id))
	if err == nil {
		for _, ar := range accountRoles {
			if err := s.accountRoleRepo.Delete(ctx, ar.ID); err != nil {
				return &AuthError{Op: "delete account role mapping", Err: err}
			}
		}
	}

	// Then remove all role-permission mappings
	if err := s.rolePermissionRepo.DeleteByRoleID(ctx, int64(id)); err != nil {
		return &AuthError{Op: "delete role permissions", Err: err}
	}

	// Finally delete the role
	if err := s.roleRepo.Delete(ctx, int64(id)); err != nil {
		return &AuthError{Op: "delete role", Err: err}
	}

	return nil
}

// ListRoles retrieves roles matching the provided filters
func (s *Service) ListRoles(ctx context.Context, filters map[string]interface{}) ([]*auth.Role, error) {
	roles, err := s.roleRepo.List(ctx, filters)
	if err != nil {
		return nil, &AuthError{Op: "list roles", Err: err}
	}
	return roles, nil
}

// AssignRoleToAccount assigns a role to an account
func (s *Service) AssignRoleToAccount(ctx context.Context, accountID, roleID int) error {
	// Verify account exists
	if _, err := s.accountRepo.FindByID(ctx, int64(accountID)); err != nil {
		return &AuthError{Op: "assign role", Err: ErrAccountNotFound}
	}

	// Verify role exists
	if _, err := s.roleRepo.FindByID(ctx, int64(roleID)); err != nil {
		return &AuthError{Op: "assign role", Err: errors.New("role not found")}
	}

	// Check if role is already assigned using the repository
	existingRole, err := s.accountRoleRepo.FindByAccountAndRole(ctx, int64(accountID), int64(roleID))
	if err != nil && !strings.Contains(err.Error(), "no rows") {
		return &AuthError{Op: "check role assignment", Err: err}
	}

	if existingRole != nil {
		// Role already assigned, no action needed
		return nil
	}

	// Create the role assignment using the repository
	accountRole := &auth.AccountRole{
		AccountID: int64(accountID),
		RoleID:    int64(roleID),
	}

	if err := s.accountRoleRepo.Create(ctx, accountRole); err != nil {
		return &AuthError{Op: "assign role to account", Err: err}
	}

	return nil
}

// RemoveRoleFromAccount removes a role from an account
func (s *Service) RemoveRoleFromAccount(ctx context.Context, accountID, roleID int) error {
	// Use the repository to delete the role assignment
	if err := s.accountRoleRepo.DeleteByAccountAndRole(ctx, int64(accountID), int64(roleID)); err != nil {
		return &AuthError{Op: "remove role from account", Err: err}
	}
	return nil
}

// GetAccountRoles retrieves all roles for an account
func (s *Service) GetAccountRoles(ctx context.Context, accountID int) ([]*auth.Role, error) {
	roles, err := s.roleRepo.FindByAccountID(ctx, int64(accountID))
	if err != nil {
		return nil, &AuthError{Op: "get account roles", Err: err}
	}
	return roles, nil
}

// Permission Management

// CreatePermission creates a new permission
func (s *Service) CreatePermission(ctx context.Context, name, description, resource, action string) (*auth.Permission, error) {
	permission := &auth.Permission{
		Name:        name,
		Description: description,
		Resource:    resource,
		Action:      action,
	}

	if err := s.permissionRepo.Create(ctx, permission); err != nil {
		return nil, &AuthError{Op: "create permission", Err: err}
	}

	return permission, nil
}

// GetPermissionByID retrieves a permission by its ID
func (s *Service) GetPermissionByID(ctx context.Context, id int) (*auth.Permission, error) {
	permission, err := s.permissionRepo.FindByID(ctx, int64(id))
	if err != nil {
		return nil, &AuthError{Op: "get permission", Err: err}
	}
	return permission, nil
}

// GetPermissionByName retrieves a permission by its name
func (s *Service) GetPermissionByName(ctx context.Context, name string) (*auth.Permission, error) {
	permission, err := s.permissionRepo.FindByName(ctx, name)
	if err != nil {
		return nil, &AuthError{Op: "get permission by name", Err: err}
	}
	return permission, nil
}

// UpdatePermission updates an existing permission
func (s *Service) UpdatePermission(ctx context.Context, permission *auth.Permission) error {
	if err := s.permissionRepo.Update(ctx, permission); err != nil {
		return &AuthError{Op: "update permission", Err: err}
	}
	return nil
}

// DeletePermission deletes a permission
func (s *Service) DeletePermission(ctx context.Context, id int) error {
	// First remove all account-permission mappings
	accountPermissions, err := s.accountPermissionRepo.FindByPermissionID(ctx, int64(id))
	if err == nil {
		for _, ap := range accountPermissions {
			if err := s.accountPermissionRepo.Delete(ctx, ap.ID); err != nil {
				return &AuthError{Op: "delete account permissions", Err: err}
			}
		}
	}

	// Then remove all role-permission mappings for this permission
	rolePermissions, err := s.rolePermissionRepo.FindByPermissionID(ctx, int64(id))
	if err == nil {
		for _, rp := range rolePermissions {
			if err := s.rolePermissionRepo.Delete(ctx, rp.ID); err != nil {
				return &AuthError{Op: "delete role permissions", Err: err}
			}
		}
	}

	// Finally delete the permission
	if err := s.permissionRepo.Delete(ctx, int64(id)); err != nil {
		return &AuthError{Op: "delete permission", Err: err}
	}

	return nil
}

// ListPermissions retrieves permissions matching the provided filters
func (s *Service) ListPermissions(ctx context.Context, filters map[string]interface{}) ([]*auth.Permission, error) {
	permissions, err := s.permissionRepo.List(ctx, filters)
	if err != nil {
		return nil, &AuthError{Op: "list permissions", Err: err}
	}
	return permissions, nil
}

// GrantPermissionToAccount grants a permission directly to an account
func (s *Service) GrantPermissionToAccount(ctx context.Context, accountID, permissionID int) error {
	// Verify account exists
	if _, err := s.accountRepo.FindByID(ctx, int64(accountID)); err != nil {
		return &AuthError{Op: "grant permission", Err: ErrAccountNotFound}
	}

	// Verify permission exists
	if _, err := s.permissionRepo.FindByID(ctx, int64(permissionID)); err != nil {
		return &AuthError{Op: "grant permission", Err: errors.New("permission not found")}
	}

	if err := s.accountPermissionRepo.GrantPermission(ctx, int64(accountID), int64(permissionID)); err != nil {
		return &AuthError{Op: "grant permission to account", Err: err}
	}

	return nil
}

// DenyPermissionToAccount explicitly denies a permission to an account
func (s *Service) DenyPermissionToAccount(ctx context.Context, accountID, permissionID int) error {
	// Verify account exists
	if _, err := s.accountRepo.FindByID(ctx, int64(accountID)); err != nil {
		return &AuthError{Op: "deny permission", Err: ErrAccountNotFound}
	}

	// Verify permission exists
	if _, err := s.permissionRepo.FindByID(ctx, int64(permissionID)); err != nil {
		return &AuthError{Op: "deny permission", Err: errors.New("permission not found")}
	}

	if err := s.accountPermissionRepo.DenyPermission(ctx, int64(accountID), int64(permissionID)); err != nil {
		return &AuthError{Op: "deny permission to account", Err: err}
	}

	return nil
}

// RemovePermissionFromAccount removes a permission from an account
func (s *Service) RemovePermissionFromAccount(ctx context.Context, accountID, permissionID int) error {
	if err := s.accountPermissionRepo.RemovePermission(ctx, int64(accountID), int64(permissionID)); err != nil {
		return &AuthError{Op: "remove permission from account", Err: err}
	}
	return nil
}

// GetAccountPermissions retrieves all permissions for an account (direct and role-based)
func (s *Service) GetAccountPermissions(ctx context.Context, accountID int) ([]*auth.Permission, error) {
	permissions, err := s.getAccountPermissions(ctx, int64(accountID))
	if err != nil {
		return nil, &AuthError{Op: "get account permissions", Err: err}
	}
	return permissions, nil
}

// GetAccountDirectPermissions retrieves only direct permissions for an account (not role-based)
func (s *Service) GetAccountDirectPermissions(ctx context.Context, accountID int) ([]*auth.Permission, error) {
	permissions, err := s.permissionRepo.FindDirectByAccountID(ctx, int64(accountID))
	if err != nil {
		return nil, &AuthError{Op: "get account direct permissions", Err: err}
	}
	return permissions, nil
}

// AssignPermissionToRole assigns a permission to a role
func (s *Service) AssignPermissionToRole(ctx context.Context, roleID, permissionID int) error {
	// Verify role exists
	if _, err := s.roleRepo.FindByID(ctx, int64(roleID)); err != nil {
		return &AuthError{Op: "assign permission to role", Err: errors.New("role not found")}
	}

	// Verify permission exists
	if _, err := s.permissionRepo.FindByID(ctx, int64(permissionID)); err != nil {
		return &AuthError{Op: "assign permission to role", Err: errors.New("permission not found")}
	}

	if err := s.permissionRepo.AssignPermissionToRole(ctx, int64(roleID), int64(permissionID)); err != nil {
		return &AuthError{Op: "assign permission to role", Err: err}
	}

	return nil
}

// RemovePermissionFromRole removes a permission from a role
func (s *Service) RemovePermissionFromRole(ctx context.Context, roleID, permissionID int) error {
	if err := s.permissionRepo.RemovePermissionFromRole(ctx, int64(roleID), int64(permissionID)); err != nil {
		return &AuthError{Op: "remove permission from role", Err: err}
	}
	return nil
}

// GetRolePermissions retrieves all permissions for a role
func (s *Service) GetRolePermissions(ctx context.Context, roleID int) ([]*auth.Permission, error) {
	permissions, err := s.permissionRepo.FindByRoleID(ctx, int64(roleID))
	if err != nil {
		return nil, &AuthError{Op: "get role permissions", Err: err}
	}
	return permissions, nil
}

// Account Management Extensions

// ActivateAccount activates a user account
func (s *Service) ActivateAccount(ctx context.Context, accountID int) error {
	account, err := s.accountRepo.FindByID(ctx, int64(accountID))
	if err != nil {
		return &AuthError{Op: "activate account", Err: ErrAccountNotFound}
	}

	account.Active = true
	if err := s.accountRepo.Update(ctx, account); err != nil {
		return &AuthError{Op: "activate account", Err: err}
	}

	return nil
}

// DeactivateAccount deactivates a user account
func (s *Service) DeactivateAccount(ctx context.Context, accountID int) error {
	account, err := s.accountRepo.FindByID(ctx, int64(accountID))
	if err != nil {
		return &AuthError{Op: "deactivate account", Err: ErrAccountNotFound}
	}

	account.Active = false
	if err := s.accountRepo.Update(ctx, account); err != nil {
		return &AuthError{Op: "deactivate account", Err: err}
	}

	// Also invalidate all tokens for this account
	if err := s.tokenRepo.DeleteByAccountID(ctx, int64(accountID)); err != nil {
		// Log error but don't fail the deactivation
		log.Printf("Failed to delete tokens for account %d: %v", accountID, err)
	}

	return nil
}

// UpdateAccount updates account information
func (s *Service) UpdateAccount(ctx context.Context, account *auth.Account) error {
	// Verify account exists
	existing, err := s.accountRepo.FindByID(ctx, account.ID)
	if err != nil {
		return &AuthError{Op: "update account", Err: ErrAccountNotFound}
	}

	// Preserve password hash if not changing password
	if account.PasswordHash == nil {
		account.PasswordHash = existing.PasswordHash
	}

	if err := s.accountRepo.Update(ctx, account); err != nil {
		return &AuthError{Op: "update account", Err: err}
	}

	return nil
}

// ListAccounts retrieves accounts matching the provided filters
func (s *Service) ListAccounts(ctx context.Context, filters map[string]interface{}) ([]*auth.Account, error) {
	accounts, err := s.accountRepo.List(ctx, filters)
	if err != nil {
		return nil, &AuthError{Op: "list accounts", Err: err}
	}
	return accounts, nil
}

// GetAccountsByRole retrieves all accounts with a specific role
func (s *Service) GetAccountsByRole(ctx context.Context, roleName string) ([]*auth.Account, error) {
	accounts, err := s.accountRepo.FindByRole(ctx, roleName)
	if err != nil {
		return nil, &AuthError{Op: "get accounts by role", Err: err}
	}
	return accounts, nil
}

// GetAccountsWithRolesAndPermissions retrieves accounts with their roles and permissions
func (s *Service) GetAccountsWithRolesAndPermissions(ctx context.Context, filters map[string]interface{}) ([]*auth.Account, error) {
	accounts, err := s.accountRepo.FindAccountsWithRolesAndPermissions(ctx, filters)
	if err != nil {
		return nil, &AuthError{Op: "get accounts with roles and permissions", Err: err}
	}
	return accounts, nil
}

// Password Reset

// InitiatePasswordReset creates a password reset token for an account
func (s *Service) InitiatePasswordReset(ctx context.Context, emailAddress string) (*auth.PasswordResetToken, error) {
	// Normalize email
	emailAddress = strings.TrimSpace(strings.ToLower(emailAddress))

	if s.passwordResetRateLimitRepo != nil {
		state, err := s.passwordResetRateLimitRepo.CheckRateLimit(ctx, emailAddress)
		if err != nil {
			return nil, &AuthError{Op: "check password reset rate limit", Err: err}
		}

		now := time.Now()
		if state != nil && state.Attempts >= passwordResetRateLimitThreshold && state.RetryAt.After(now) {
			return nil, &AuthError{
				Op: "initiate password reset",
				Err: &RateLimitError{
					Err:      ErrRateLimitExceeded,
					Attempts: state.Attempts,
					RetryAt:  state.RetryAt,
				},
			}
		}

		state, err = s.passwordResetRateLimitRepo.IncrementAttempts(ctx, emailAddress)
		if err != nil {
			return nil, &AuthError{Op: "increment password reset rate limit", Err: err}
		}

		now = time.Now()
		if state != nil && state.Attempts > passwordResetRateLimitThreshold && state.RetryAt.After(now) {
			return nil, &AuthError{
				Op: "initiate password reset",
				Err: &RateLimitError{
					Err:      ErrRateLimitExceeded,
					Attempts: state.Attempts,
					RetryAt:  state.RetryAt,
				},
			}
		}
	}

	log.Printf("Password reset requested for email=%s", emailAddress)

	// Get account by email
	account, err := s.accountRepo.FindByEmail(ctx, emailAddress)
	if err != nil {
		// Don't reveal whether the email exists or not
		return nil, nil
	}

	// Invalidate any existing reset tokens
	if err := s.passwordResetTokenRepo.InvalidateTokensByAccountID(ctx, account.ID); err != nil {
		// Log error but continue
		log.Printf("Failed to invalidate reset tokens for account %d: %v", account.ID, err)
	}

	// Generate new token
	tokenStr := uuid.Must(uuid.NewV4()).String()
	resetToken := &auth.PasswordResetToken{
		AccountID: account.ID,
		Token:     tokenStr,
		Expiry:    time.Now().Add(s.passwordResetExpiry),
		Used:      false,
	}

	if err := s.passwordResetTokenRepo.Create(ctx, resetToken); err != nil {
		return nil, &AuthError{Op: "create password reset token", Err: err}
	}

	log.Printf("Password reset token created for account=%d", account.ID)

	frontendURL := strings.TrimRight(s.frontendURL, "/")
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", frontendURL, resetToken.Token)
	logoURL := fmt.Sprintf("%s/images/moto_transparent.png", frontendURL)
	message := email.Message{
		From:     s.defaultFrom,
		To:       email.NewEmail("", account.Email),
		Subject:  "Password Reset Request",
		Template: "password-reset.html",
		Content: map[string]any{
			"ResetURL":      resetURL,
			"ExpiryMinutes": int(s.passwordResetExpiry.Minutes()),
			"LogoURL":       logoURL,
		},
	}

	go func(to string) {
		if err := s.mailer.Send(message); err != nil {
			log.Printf("Failed to send password reset email to=%s err=%v", to, err)
		}
	}(account.Email)

	return resetToken, nil
}

// ResetPassword resets a password using a reset token
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	// Find valid token
	resetToken, err := s.passwordResetTokenRepo.FindValidByToken(ctx, token)
	if err != nil {
		return &AuthError{Op: "reset password", Err: ErrInvalidToken}
	}

	// Validate new password
	if err := ValidatePasswordStrength(newPassword); err != nil {
		return &AuthError{Op: "reset password", Err: err}
	}

	// Hash new password
	passwordHash, err := HashPassword(newPassword)
	if err != nil {
		return &AuthError{Op: "hash password", Err: err}
	}

	// Execute in transaction
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(AuthService)

		// Update account password
		if err := txService.(*Service).accountRepo.UpdatePassword(ctx, resetToken.AccountID, passwordHash); err != nil {
			return err
		}

		// Mark token as used
		if err := txService.(*Service).passwordResetTokenRepo.MarkAsUsed(ctx, resetToken.ID); err != nil {
			return err
		}

		// Invalidate all existing auth tokens for security
		if err := txService.(*Service).tokenRepo.DeleteByAccountID(ctx, resetToken.AccountID); err != nil {
			// Log error but don't fail the password reset
			log.Printf("Failed to delete tokens during password reset for account %d: %v", resetToken.AccountID, err)
		}

		return nil
	})

	if err != nil {
		return &AuthError{Op: "reset password transaction", Err: err}
	}

	return nil
}

// Token Management

// CleanupExpiredTokens removes expired authentication tokens
func (s *Service) CleanupExpiredTokens(ctx context.Context) (int, error) {
	count, err := s.tokenRepo.DeleteExpiredTokens(ctx)
	if err != nil {
		return 0, &AuthError{Op: "cleanup expired tokens", Err: err}
	}
	return count, nil
}

// CleanupExpiredPasswordResetTokens removes expired password reset tokens
func (s *Service) CleanupExpiredPasswordResetTokens(ctx context.Context) (int, error) {
	count, err := s.passwordResetTokenRepo.DeleteExpiredTokens(ctx)
	if err != nil {
		return 0, &AuthError{Op: "cleanup expired password reset tokens", Err: err}
	}
	return count, nil
}

// CleanupExpiredRateLimits purges stale password reset rate limit windows.
func (s *Service) CleanupExpiredRateLimits(ctx context.Context) (int, error) {
	if s.passwordResetRateLimitRepo == nil {
		return 0, nil
	}

	count, err := s.passwordResetRateLimitRepo.CleanupExpired(ctx)
	if err != nil {
		return 0, &AuthError{Op: "cleanup password reset rate limits", Err: err}
	}

	log.Printf("Password reset rate limit cleanup removed %d records", count)
	return count, nil
}

// RevokeAllTokens revokes all tokens for an account
func (s *Service) RevokeAllTokens(ctx context.Context, accountID int) error {
	if err := s.tokenRepo.DeleteByAccountID(ctx, int64(accountID)); err != nil {
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

	tokens, err := s.tokenRepo.List(ctx, filters)
	if err != nil {
		return nil, &AuthError{Op: "get active tokens", Err: err}
	}
	return tokens, nil
}

// Parent Account Management

// CreateParentAccount creates a new parent account
func (s *Service) CreateParentAccount(ctx context.Context, email, username, password string) (*auth.AccountParent, error) {
	// Normalize input
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)

	// Validate password strength
	if err := ValidatePasswordStrength(password); err != nil {
		return nil, &AuthError{Op: "create parent account", Err: err}
	}

	// Check if email already exists
	_, err := s.accountParentRepo.FindByEmail(ctx, email)
	if err == nil {
		return nil, &AuthError{Op: "create parent account", Err: ErrEmailAlreadyExists}
	}

	// Check if username already exists
	_, err = s.accountParentRepo.FindByUsername(ctx, username)
	if err == nil {
		return nil, &AuthError{Op: "create parent account", Err: ErrUsernameAlreadyExists}
	}

	// Hash password
	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, &AuthError{Op: "hash password", Err: err}
	}

	usernamePtr := &username

	// Create parent account
	parentAccount := &auth.AccountParent{
		Email:        email,
		Username:     usernamePtr,
		Active:       true,
		PasswordHash: &passwordHash,
	}

	if err := s.accountParentRepo.Create(ctx, parentAccount); err != nil {
		return nil, &AuthError{Op: "create parent account", Err: err}
	}

	return parentAccount, nil
}

// GetParentAccountByID retrieves a parent account by ID
func (s *Service) GetParentAccountByID(ctx context.Context, id int) (*auth.AccountParent, error) {
	account, err := s.accountParentRepo.FindByID(ctx, int64(id))
	if err != nil {
		return nil, &AuthError{Op: "get parent account", Err: err}
	}
	return account, nil
}

// GetParentAccountByEmail retrieves a parent account by email
func (s *Service) GetParentAccountByEmail(ctx context.Context, email string) (*auth.AccountParent, error) {
	// Normalize email
	email = strings.TrimSpace(strings.ToLower(email))

	account, err := s.accountParentRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, &AuthError{Op: "get parent account by email", Err: err}
	}
	return account, nil
}

// UpdateParentAccount updates a parent account
func (s *Service) UpdateParentAccount(ctx context.Context, account *auth.AccountParent) error {
	// Verify account exists
	existing, err := s.accountParentRepo.FindByID(ctx, account.ID)
	if err != nil {
		return &AuthError{Op: "update parent account", Err: errors.New("parent account not found")}
	}

	// Preserve password hash if not changing password
	if account.PasswordHash == nil {
		account.PasswordHash = existing.PasswordHash
	}

	if err := s.accountParentRepo.Update(ctx, account); err != nil {
		return &AuthError{Op: "update parent account", Err: err}
	}

	return nil
}

// ActivateParentAccount activates a parent account
func (s *Service) ActivateParentAccount(ctx context.Context, accountID int) error {
	account, err := s.accountParentRepo.FindByID(ctx, int64(accountID))
	if err != nil {
		return &AuthError{Op: "activate parent account", Err: errors.New("parent account not found")}
	}

	account.Active = true
	if err := s.accountParentRepo.Update(ctx, account); err != nil {
		return &AuthError{Op: "activate parent account", Err: err}
	}

	return nil
}

// DeactivateParentAccount deactivates a parent account
func (s *Service) DeactivateParentAccount(ctx context.Context, accountID int) error {
	account, err := s.accountParentRepo.FindByID(ctx, int64(accountID))
	if err != nil {
		return &AuthError{Op: "deactivate parent account", Err: errors.New("parent account not found")}
	}

	account.Active = false
	if err := s.accountParentRepo.Update(ctx, account); err != nil {
		return &AuthError{Op: "deactivate parent account", Err: err}
	}

	return nil
}

// ListParentAccounts retrieves parent accounts matching the provided filters
func (s *Service) ListParentAccounts(ctx context.Context, filters map[string]interface{}) ([]*auth.AccountParent, error) {
	accounts, err := s.accountParentRepo.List(ctx, filters)
	if err != nil {
		return nil, &AuthError{Op: "list parent accounts", Err: err}
	}
	return accounts, nil
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
		logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.authEventRepo.Create(logCtx, event); err != nil {
			// Log the error but don't fail the auth operation
			log.Printf("Failed to log auth event: %v", err)
		}
	}()
}
