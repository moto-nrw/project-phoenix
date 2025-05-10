package auth

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	jwx "github.com/lestrrat-go/jwx/v2/jwt"
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
	if account.Roles == nil || len(account.Roles) == 0 {
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

	return account, nil
}

// RefreshToken generates new token pair from a refresh token
func (s *Service) RefreshToken(ctx context.Context, refreshTokenStr string) (string, string, error) {
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

	// Get token from database using token string from claims
	dbToken, err := s.tokenRepo.FindByToken(ctx, refreshClaims.Token)
	if err != nil {
		return "", "", &AuthError{Op: "get token", Err: ErrTokenNotFound}
	}

	// Check if token is expired
	if time.Now().After(dbToken.Expiry) {
		// Delete expired token
		_ = s.tokenRepo.Delete(ctx, dbToken)
		return "", "", &AuthError{Op: "check token expiry", Err: ErrTokenExpired}
	}

	// Get account
	account, err := s.accountRepo.FindByID(ctx, dbToken.AccountID)
	if err != nil {
		return "", "", &AuthError{Op: "get account", Err: ErrAccountNotFound}
	}

	// Check if account is active
	if !account.Active {
		return "", "", &AuthError{Op: "check account status", Err: ErrAccountInactive}
	}

	// Generate new refresh token
	newTokenStr := uuid.Must(uuid.NewV4()).String()
	now := time.Now()

	// Update token in database
	dbToken.Token = newTokenStr
	dbToken.Expiry = now.Add(s.jwtRefreshExpiry)

	// Execute in transaction
	err = s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Update token
		if err := s.tokenRepo.Update(ctx, dbToken); err != nil {
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
	if account.Roles == nil || len(account.Roles) == 0 {
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

	// Extract roles as strings
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
	appClaims := jwt.AppClaims{
		ID:       int(account.ID),
		Sub:      account.Email,
		Username: username,
		Roles:    roleNames,
	}

	newRefreshClaims := jwt.RefreshClaims{
		ID:    int(dbToken.ID),
		Token: dbToken.Token,
	}

	// Generate tokens
	accessToken, newRefreshToken, err := s.tokenAuth.GenTokenPair(appClaims, newRefreshClaims)
	if err != nil {
		return "", "", &AuthError{Op: "generate tokens", Err: err}
	}

	return accessToken, newRefreshToken, nil
}

// Logout invalidates a refresh token
func (s *Service) Logout(ctx context.Context, refreshTokenStr string) error {
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

	// Get token from database
	dbToken, err := s.tokenRepo.FindByToken(ctx, refreshClaims.Token)
	if err != nil {
		// Token not found, consider logout successful
		return nil
	}

	// Delete token from database
	err = s.tokenRepo.Delete(ctx, dbToken)
	if err != nil {
		return &AuthError{Op: "delete token", Err: err}
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
