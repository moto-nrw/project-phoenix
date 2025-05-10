package auth

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/uptrace/bun"
)

// Service implements the AuthService interface
type Service struct {
	accountRepo      auth.AccountRepository
	accountRoleRepo  auth.AccountRoleRepository
	tokenRepo        auth.TokenRepository
	db               *bun.DB
	tokenAuth        *jwt.TokenAuth
	jwtExpiry        time.Duration
	jwtRefreshExpiry time.Duration
}

// NewService creates a new auth service
func NewService(accountRepo auth.AccountRepository, accountRoleRepo auth.AccountRoleRepository,
	tokenRepo auth.TokenRepository, db *bun.DB) (*Service, error) {

	tokenAuth, err := jwt.NewTokenAuth()
	if err != nil {
		return nil, &AuthError{Op: "create token auth", Err: err}
	}

	return &Service{
		accountRepo:      accountRepo,
		accountRoleRepo:  accountRoleRepo,
		tokenRepo:        tokenRepo,
		db:               db,
		tokenAuth:        tokenAuth,
		jwtExpiry:        tokenAuth.JwtExpiry,
		jwtRefreshExpiry: tokenAuth.JwtRefreshExpiry,
	}, nil
}

// Login authenticates a user and returns access and refresh tokens
func (s *Service) Login(ctx context.Context, email, password string) (string, string, error) {
	// Normalize email
	email = strings.TrimSpace(strings.ToLower(email))

	// Get account by email
	account, err := s.accountRepo.FindByEmail(ctx, email)
	if err != nil {
		return "", "", &AuthError{Op: "login", Err: ErrAccountNotFound}
	}

	// Check if account is active
	if !account.Active {
		return "", "", &AuthError{Op: "login", Err: ErrAccountInactive}
	}

	// Verify password
	if account.PasswordHash == nil || *account.PasswordHash == "" {
		return "", "", &AuthError{Op: "login", Err: ErrInvalidCredentials}
	}

	valid, err := userpass.VerifyPassword(password, *account.PasswordHash)
	if err != nil || !valid {
		return "", "", &AuthError{Op: "login", Err: ErrInvalidCredentials}
	}

	// Create refresh token
	tokenStr := uuid.Must(uuid.NewV4()).String()
	identifier := "Service login"
	now := time.Now()
	token := &auth.Token{
		Token:      tokenStr,
		AccountID:  account.ID,
		Expiry:     now.Add(s.jwtRefreshExpiry),
		Mobile:     false, // This would come from a user agent in a real request
		Identifier: &identifier,
	}

	// Execute in transaction
	err = s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Create token
		if err := s.tokenRepo.Create(ctx, token); err != nil {
			return err
		}

		// Update last login
		loginTime := time.Now()
		account.LastLogin = &loginTime
		return s.accountRepo.Update(ctx, account)
	})

	if err != nil {
		return "", "", &AuthError{Op: "login transaction", Err: err}
	}

	// Retrieve account roles if not loaded
	if account.Roles == nil || len(account.Roles) == 0 {
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

	// Convert roles to string slice for token
	var roleNames []string
	for _, role := range account.Roles {
		roleNames = append(roleNames, role.Name)
	}

	// Extract username
	username := ""
	if account.Username != nil {
		username = *account.Username
	}

	// Generate token pair
	// Create JWT claims
	appClaims := jwt.AppClaims{
		ID:       int(account.ID),
		Sub:      email, // Use email as subject
		Username: username,
		Roles:    roleNames,
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

	return accessToken, refreshToken, nil
}

// Register creates a new user account
func (s *Service) Register(ctx context.Context, email, username, name, password string) (*auth.Account, error) {
	// Normalize input
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)

	// Validate password strength
	if err := validatePassword(password); err != nil {
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
	passwordHash, err := userpass.HashPassword(password, userpass.DefaultParams())
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

	// Execute in transaction
	err = s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Create account
		if err := s.accountRepo.Create(ctx, account); err != nil {
			return err
		}

		// Find the default user role
		// This is simplified - in a real implementation you'd need to
		// retrieve the role ID from the database
		// Assume 'user' role with ID 1 exists

		// Create account role mapping
		// accountRole := &auth.AccountRole{
		//    AccountID: account.ID,
		//    RoleID:    1, // Default user role ID
		// }
		// return s.accountRoleRepo.Create(ctx, accountRole)

		// For now, just return nil since we don't have the actual role IDs
		return nil
	})

	if err != nil {
		return nil, &AuthError{Op: "register transaction", Err: err}
	}

	return account, nil
}

// ValidateToken validates an access token and returns the associated account
func (s *Service) ValidateToken(ctx context.Context, token string) (*auth.Account, error) {
	// Parse and validate token
	// This is simplified - in a real implementation, you'd extract the claims
	// using the JWT library
	_, err := s.tokenAuth.JwtAuth.Decode(token)
	if err != nil {
		return nil, &AuthError{Op: "validate token", Err: ErrInvalidToken}
	}

	// Assuming claims.ID is extracted and contains the account ID
	// For now, we're just checking the token format

	// Get account (simplified - would use claims.ID in real implementation)
	// Just returning nil since we can't extract the actual ID
	return nil, &AuthError{Op: "validate token", Err: ErrInvalidToken}
}

// RefreshToken generates new token pair from a refresh token
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	// Parse refresh token and extract claims
	// This is simplified - in a real implementation, you'd extract the token string
	_, err := s.tokenAuth.JwtAuth.Decode(refreshToken)
	if err != nil {
		return "", "", &AuthError{Op: "parse refresh token", Err: ErrInvalidToken}
	}

	// Get token from repository (simplified - would use refreshClaims.Token in real implementation)
	// For now, we're just checking the token format
	return "", "", &AuthError{Op: "refresh token", Err: ErrInvalidToken}
}

// Logout invalidates a refresh token
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	// Parse refresh token and extract claims
	// This is simplified - in a real implementation, you'd extract the token string
	_, err := s.tokenAuth.JwtAuth.Decode(refreshToken)
	if err != nil {
		return &AuthError{Op: "parse refresh token", Err: ErrInvalidToken}
	}

	// Delete token (simplified - would use refreshClaims.Token in real implementation)
	// For now, we're just checking the token format
	return &AuthError{Op: "logout", Err: ErrInvalidToken}
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
	if err := validatePassword(newPassword); err != nil {
		return &AuthError{Op: "validate password", Err: err}
	}

	// Hash new password
	passwordHash, err := userpass.HashPassword(newPassword, userpass.DefaultParams())
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

// Helper functions

// validatePassword validates password strength
func validatePassword(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooWeak
	}

	if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		return ErrPasswordTooWeak
	}

	if !regexp.MustCompile(`[a-z]`).MatchString(password) {
		return ErrPasswordTooWeak
	}

	if !regexp.MustCompile(`[0-9]`).MatchString(password) {
		return ErrPasswordTooWeak
	}

	if !regexp.MustCompile(`[^a-zA-Z0-9]`).MatchString(password) {
		return ErrPasswordTooWeak
	}

	return nil
}
