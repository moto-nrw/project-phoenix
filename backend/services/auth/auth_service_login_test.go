package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	baseModel "github.com/moto-nrw/project-phoenix/models/base"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// newTestAuthService creates an auth service for testing without real database
func newTestAuthService(t *testing.T) (*Service, *stubAccountRepository, *mockTokenRepositoryImpl, *stubRoleRepository, *mockPermissionRepositoryImpl, *stubAccountRoleRepository, *stubPersonRepository, sqlmock.Sqlmock, func()) {
	t.Helper()

	// Create mock SQL database
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	// Create stub repositories
	accountRepo := newStubAccountRepository()
	tokenRepo := newMockTokenRepositoryImpl()
	roleRepo := newStubRoleRepository()
	permissionRepo := newMockPermissionRepositoryImpl()
	accountRoleRepo := newStubAccountRoleRepository()
	personRepo := newStubPersonRepository()

	config, err := NewServiceConfig(
		nil, // No email dispatcher
		newDefaultFromEmail(),
		"http://localhost:3000",
		30*time.Minute,
	)
	require.NoError(t, err)

	// Create service using the NewService constructor
	// We can't directly inject mocks into the factory, so we'll create the service
	// and then manually override the repos field (this is test-only)
	service, err := NewService(nil, config, bunDB) // Pass nil factory initially
	require.Error(t, err) // This will error because factory is nil

	// Set up viper configuration for JWT
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	// Create service manually for testing
	tokenAuth, err := jwt.NewTokenAuthWithSecret("test-secret-key-minimum-32-chars-long-for-security")
	require.NoError(t, err)

	service = &Service{
		repos:               nil, // We'll set this manually per test
		tokenAuth:           tokenAuth,
		dispatcher:          config.Dispatcher,
		defaultFrom:         config.DefaultFrom,
		frontendURL:         config.FrontendURL,
		passwordResetExpiry: config.PasswordResetExpiry,
		jwtExpiry:           tokenAuth.JwtExpiry,
		jwtRefreshExpiry:    tokenAuth.JwtRefreshExpiry,
		txHandler:           baseModel.NewTxHandler(bunDB),
		db:                  bunDB,
	}

	// Create a repositories.Factory with our test repositories
	testRepos := &repositories.Factory{
		Account:                accountRepo,
		Token:                  tokenRepo,
		Role:                   roleRepo,
		Permission:             permissionRepo,
		AccountRole:            accountRoleRepo,
		Person:                 personRepo,
		PasswordResetToken:     newStubPasswordResetTokenRepository(),
		PasswordResetRateLimit: newTestRateLimitRepo(),
	}

	service.repos = testRepos

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Logf("Mock expectations not met: %v", err)
		}
	}

	return service, accountRepo, tokenRepo, roleRepo, permissionRepo, accountRoleRepo, personRepo, mock, cleanup
}

// mockTokenRepositoryImpl is a complete mock implementation for testing
type mockTokenRepositoryImpl struct {
	mu                     sync.Mutex
	tokens                 map[string]*authModel.Token
	byID                   map[int64]*authModel.Token
	byAccountID            map[int64][]*authModel.Token
	nextID                 int64
	deleteByAccountIDCalls []int64
	cleanupCalls           []struct {
		accountID int64
		maxTokens int
	}
}

func newMockTokenRepositoryImpl() *mockTokenRepositoryImpl {
	return &mockTokenRepositoryImpl{
		tokens:      make(map[string]*authModel.Token),
		byID:        make(map[int64]*authModel.Token),
		byAccountID: make(map[int64][]*authModel.Token),
		nextID:      1,
	}
}

func (r *mockTokenRepositoryImpl) Create(_ context.Context, token *authModel.Token) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if token.ID == 0 {
		r.nextID++
		token.ID = r.nextID
	}

	r.tokens[token.Token] = token
	r.byID[token.ID] = token
	r.byAccountID[token.AccountID] = append(r.byAccountID[token.AccountID], token)

	return nil
}

func (r *mockTokenRepositoryImpl) FindByToken(_ context.Context, tokenStr string) (*authModel.Token, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if token, ok := r.tokens[tokenStr]; ok {
		return token, nil
	}
	return nil, sql.ErrNoRows
}

func (r *mockTokenRepositoryImpl) FindByTokenForUpdate(ctx context.Context, tokenStr string) (*authModel.Token, error) {
	return r.FindByToken(ctx, tokenStr)
}

func (r *mockTokenRepositoryImpl) Delete(_ context.Context, id interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var tokenID int64
	switch v := id.(type) {
	case int64:
		tokenID = v
	case int:
		tokenID = int64(v)
	default:
		return errors.New("invalid ID type")
	}

	token, ok := r.byID[tokenID]
	if !ok {
		return sql.ErrNoRows
	}

	delete(r.tokens, token.Token)
	delete(r.byID, tokenID)

	// Remove from byAccountID
	accountTokens := r.byAccountID[token.AccountID]
	for i, t := range accountTokens {
		if t.ID == tokenID {
			r.byAccountID[token.AccountID] = append(accountTokens[:i], accountTokens[i+1:]...)
			break
		}
	}

	return nil
}

func (r *mockTokenRepositoryImpl) DeleteByAccountID(_ context.Context, accountID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.deleteByAccountIDCalls = append(r.deleteByAccountIDCalls, accountID)

	// Delete all tokens for this account
	for _, token := range r.byAccountID[accountID] {
		delete(r.tokens, token.Token)
		delete(r.byID, token.ID)
	}
	delete(r.byAccountID, accountID)

	return nil
}

func (r *mockTokenRepositoryImpl) FindByAccountID(_ context.Context, accountID int64) ([]*authModel.Token, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tokens := r.byAccountID[accountID]
	if tokens == nil {
		return []*authModel.Token{}, nil
	}

	result := make([]*authModel.Token, len(tokens))
	copy(result, tokens)
	return result, nil
}

func (r *mockTokenRepositoryImpl) CleanupOldTokensForAccount(_ context.Context, accountID int64, maxTokens int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cleanupCalls = append(r.cleanupCalls, struct {
		accountID int64
		maxTokens int
	}{accountID, maxTokens})

	tokens := r.byAccountID[accountID]
	if len(tokens) <= maxTokens {
		return nil
	}

	// Keep only the most recent maxTokens
	toRemove := len(tokens) - maxTokens
	for i := 0; i < toRemove; i++ {
		token := tokens[i]
		delete(r.tokens, token.Token)
		delete(r.byID, token.ID)
	}

	r.byAccountID[accountID] = tokens[toRemove:]
	return nil
}

func (r *mockTokenRepositoryImpl) GetLatestTokenInFamily(_ context.Context, familyID string) (*authModel.Token, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var latestToken *authModel.Token
	for _, token := range r.tokens {
		if token.FamilyID == familyID {
			if latestToken == nil || token.Generation > latestToken.Generation {
				latestToken = token
			}
		}
	}

	if latestToken == nil {
		return nil, sql.ErrNoRows
	}

	return latestToken, nil
}

func (r *mockTokenRepositoryImpl) DeleteByFamilyID(_ context.Context, familyID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for tokenStr, token := range r.tokens {
		if token.FamilyID == familyID {
			delete(r.tokens, tokenStr)
			delete(r.byID, token.ID)

			accountTokens := r.byAccountID[token.AccountID]
			for i, t := range accountTokens {
				if t.ID == token.ID {
					r.byAccountID[token.AccountID] = append(accountTokens[:i], accountTokens[i+1:]...)
					break
				}
			}
		}
	}

	return nil
}

// Stub implementations for unused methods
func (r *mockTokenRepositoryImpl) FindByID(context.Context, interface{}) (*authModel.Token, error) {
	return nil, nil
}
func (r *mockTokenRepositoryImpl) Update(context.Context, *authModel.Token) error {
	return nil
}
func (r *mockTokenRepositoryImpl) List(context.Context, map[string]interface{}) ([]*authModel.Token, error) {
	return nil, nil
}
func (r *mockTokenRepositoryImpl) FindByAccountIDAndIdentifier(context.Context, int64, string) (*authModel.Token, error) {
	return nil, nil
}
func (r *mockTokenRepositoryImpl) DeleteExpiredTokens(context.Context) (int, error) {
	return 0, nil
}
func (r *mockTokenRepositoryImpl) DeleteByAccountIDAndIdentifier(context.Context, int64, string) error {
	return nil
}
func (r *mockTokenRepositoryImpl) FindValidTokens(context.Context, map[string]interface{}) ([]*authModel.Token, error) {
	return nil, nil
}
func (r *mockTokenRepositoryImpl) FindTokensWithAccount(context.Context, map[string]interface{}) ([]*authModel.Token, error) {
	return nil, nil
}
func (r *mockTokenRepositoryImpl) FindByFamilyID(context.Context, string) ([]*authModel.Token, error) {
	return nil, nil
}

// mockPermissionRepositoryImpl implements permission repository for testing
type mockPermissionRepositoryImpl struct {
	mu           sync.Mutex
	accountPerms map[int64][]*authModel.Permission
	rolePerms    map[int64][]*authModel.Permission
}

func newMockPermissionRepositoryImpl() *mockPermissionRepositoryImpl {
	return &mockPermissionRepositoryImpl{
		accountPerms: make(map[int64][]*authModel.Permission),
		rolePerms:    make(map[int64][]*authModel.Permission),
	}
}

func (r *mockPermissionRepositoryImpl) FindByAccountID(_ context.Context, accountID int64) ([]*authModel.Permission, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.accountPerms[accountID], nil
}

func (r *mockPermissionRepositoryImpl) FindByRoleID(_ context.Context, roleID int64) ([]*authModel.Permission, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rolePerms[roleID], nil
}

func (r *mockPermissionRepositoryImpl) FindByRoleByName(_ context.Context, name string) (*authModel.Role, error) {
	return nil, sql.ErrNoRows
}

// Stub implementations
func (r *mockPermissionRepositoryImpl) Create(context.Context, *authModel.Permission) error {
	return nil
}
func (r *mockPermissionRepositoryImpl) FindByID(context.Context, interface{}) (*authModel.Permission, error) {
	return nil, nil
}
func (r *mockPermissionRepositoryImpl) FindByName(context.Context, string) (*authModel.Permission, error) {
	return nil, nil
}
func (r *mockPermissionRepositoryImpl) Update(context.Context, *authModel.Permission) error {
	return nil
}
func (r *mockPermissionRepositoryImpl) Delete(context.Context, interface{}) error {
	return nil
}
func (r *mockPermissionRepositoryImpl) List(context.Context, map[string]interface{}) ([]*authModel.Permission, error) {
	return nil, nil
}
func (r *mockPermissionRepositoryImpl) AssignPermissionToRole(context.Context, int64, int64) error {
	return nil
}
func (r *mockPermissionRepositoryImpl) RemovePermissionFromRole(context.Context, int64, int64) error {
	return nil
}
func (r *mockPermissionRepositoryImpl) FindDirectByAccountID(context.Context, int64) ([]*authModel.Permission, error) {
	return nil, nil
}
func (r *mockPermissionRepositoryImpl) AssignPermissionToAccount(context.Context, int64, int64) error {
	return nil
}
func (r *mockPermissionRepositoryImpl) DenyPermissionToAccount(context.Context, int64, int64) error {
	return nil
}
func (r *mockPermissionRepositoryImpl) RemovePermissionFromAccount(context.Context, int64, int64) error {
	return nil
}
func (r *mockPermissionRepositoryImpl) FindByResourceAction(context.Context, string, string) (*authModel.Permission, error) {
	return nil, nil
}

// TestLoginSuccess tests successful login
func TestLoginSuccess(t *testing.T) {
	service, accountRepo, tokenRepo, _, _, _, personRepo, mock, cleanup := newTestAuthService(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Create test account with hashed password
	password := "ValidPassword123!"
	hash, err := userpass.HashPassword(password, nil)
	require.NoError(t, err)

	email := "test@example.com"
	account := &authModel.Account{
		Model:        baseModel.Model{ID: 1},
		Email:        strings.ToLower(email),
		PasswordHash: &hash,
		Active:       true,
	}

	accountRepo.storeAccount(account)

	// Create person for account (for JWT claims)
	person := &userModel.Person{
		Model:     baseModel.Model{ID: 1},
		FirstName: "Test",
		LastName:  "User",
		AccountID: &account.ID,
	}
	_ = personRepo.Create(ctx, person)

	// Set up mock expectations for transaction
	mock.ExpectBegin()
	mock.ExpectCommit()

	// Attempt login
	accessToken, refreshToken, err := service.Login(ctx, email, password)

	require.NoError(t, err)
	assert.NotEmpty(t, accessToken, "Access token should not be empty")
	assert.NotEmpty(t, refreshToken, "Refresh token should not be empty")

	// Verify tokens are different
	assert.NotEqual(t, accessToken, refreshToken, "Access and refresh tokens should be different")

	// Verify tokens are valid JWT format
	assert.Contains(t, accessToken, ".", "Access token should be JWT format")
	assert.Contains(t, refreshToken, ".", "Refresh token should be JWT format")

	// Verify refresh token was created
	tokens := tokenRepo.GetTokensByAccountID(account.ID)
	assert.Len(t, tokens, 1, "Should have created one refresh token")
	assert.Equal(t, account.ID, tokens[0].AccountID)
	assert.NotEmpty(t, tokens[0].Token)
}

// TestLoginInvalidPassword tests login with wrong password
func TestLoginInvalidPassword(t *testing.T) {
	service, accountRepo, _, _, _, _, _, _, cleanup := newTestAuthService(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Create test account
	password := "CorrectPassword123!"
	hash, err := userpass.HashPassword(password, nil)
	require.NoError(t, err)

	email := "wrongpass@example.com"
	account := &authModel.Account{
		Model:        baseModel.Model{ID: 2},
		Email:        strings.ToLower(email),
		PasswordHash: &hash,
		Active:       true,
	}

	accountRepo.storeAccount(account)

	// Attempt login with wrong password
	accessToken, refreshToken, err := service.Login(ctx, email, "WrongPassword456!")

	require.Error(t, err)
	assert.Empty(t, accessToken)
	assert.Empty(t, refreshToken)
	assert.Contains(t, err.Error(), "invalid")
}

// TestLoginAccountNotFound tests login with non-existent account
func TestLoginAccountNotFound(t *testing.T) {
	service, _, _, _, _, _, _, _, cleanup := newTestAuthService(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Attempt login with non-existent email
	accessToken, refreshToken, err := service.Login(ctx, "nonexistent@example.com", "password")

	require.Error(t, err)
	assert.Empty(t, accessToken)
	assert.Empty(t, refreshToken)
	assert.Contains(t, err.Error(), "not found")
}

// TestLoginInactiveAccount tests login with inactive account
func TestLoginInactiveAccount(t *testing.T) {
	service, accountRepo, _, _, _, _, _, _, cleanup := newTestAuthService(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Create inactive account
	password := "ValidPassword123!"
	hash, err := userpass.HashPassword(password, nil)
	require.NoError(t, err)

	email := "inactive@example.com"
	account := &authModel.Account{
		Model:        baseModel.Model{ID: 3},
		Email:        strings.ToLower(email),
		PasswordHash: &hash,
		Active:       false, // Inactive
	}

	accountRepo.storeAccount(account)

	// Attempt login
	accessToken, refreshToken, err := service.Login(ctx, email, password)

	require.Error(t, err)
	assert.Empty(t, accessToken)
	assert.Empty(t, refreshToken)
	assert.Contains(t, err.Error(), "inactive")
}

// TestLoginCaseInsensitive tests email case-insensitivity
func TestLoginCaseInsensitive(t *testing.T) {
	service, accountRepo, _, _, _, _, _, mock, cleanup := newTestAuthService(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	password := "ValidPassword123!"
	hash, err := userpass.HashPassword(password, nil)
	require.NoError(t, err)

	email := "casetest@example.com"
	account := &authModel.Account{
		Model:        baseModel.Model{ID: 4},
		Email:        strings.ToLower(email),
		PasswordHash: &hash,
		Active:       true,
	}

	accountRepo.storeAccount(account)

	// Login with different case variations
	testCases := []string{
		"CASETEST@EXAMPLE.COM",
		"CaseTest@Example.Com",
		"casetest@example.com",
	}

	// Set up mock expectations for 3 logins
	for i := 0; i < len(testCases); i++ {
		mock.ExpectBegin()
		mock.ExpectCommit()
	}

	for _, testEmail := range testCases {
		accessToken, refreshToken, err := service.Login(ctx, testEmail, password)
		require.NoError(t, err, "Login should succeed with email: %s", testEmail)
		assert.NotEmpty(t, accessToken)
		assert.NotEmpty(t, refreshToken)
	}
}

// TestLogoutSuccess tests successful logout
func TestLogoutSuccess(t *testing.T) {
	service, accountRepo, tokenRepo, _, _, _, _, mock, cleanup := newTestAuthService(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Create test account
	password := "ValidPassword123!"
	hash, err := userpass.HashPassword(password, nil)
	require.NoError(t, err)

	email := "logout@example.com"
	account := &authModel.Account{
		Model:        baseModel.Model{ID: 10},
		Email:        email,
		PasswordHash: &hash,
		Active:       true,
	}

	accountRepo.storeAccount(account)

	// Set up mock expectations for login
	mock.ExpectBegin()
	mock.ExpectCommit()

	// Login to create a token
	_, refreshToken, err := service.Login(ctx, email, password)
	require.NoError(t, err)

	// Verify token exists
	tokensBefore := tokenRepo.GetTokensByAccountID(account.ID)
	assert.Len(t, tokensBefore, 1)

	// Logout
	err = service.Logout(ctx, refreshToken)
	require.NoError(t, err)

	// Verify all tokens were deleted
	deleteCalls := tokenRepo.GetDeleteByAccountIDCalls()
	assert.Contains(t, deleteCalls, account.ID, "DeleteByAccountID should be called")
}

// TestLogoutInvalidToken tests logout with invalid token format
func TestLogoutInvalidToken(t *testing.T) {
	service, _, _, _, _, _, _, _, cleanup := newTestAuthService(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Logout with invalid JWT format should return error
	err := service.Logout(ctx, "invalid-token-xyz")
	assert.Error(t, err, "Logout with invalid JWT format should return error")
	assert.Contains(t, err.Error(), "invalid")
}

// TestRefreshTokenSuccess tests successful token refresh
func TestRefreshTokenSuccess(t *testing.T) {
	service, accountRepo, tokenRepo, _, _, _, _, mock, cleanup := newTestAuthService(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Create test account
	password := "ValidPassword123!"
	hash, err := userpass.HashPassword(password, nil)
	require.NoError(t, err)

	email := "refresh@example.com"
	account := &authModel.Account{
		Model:        baseModel.Model{ID: 20},
		Email:        email,
		PasswordHash: &hash,
		Active:       true,
	}

	accountRepo.storeAccount(account)

	// Set up mock expectations for login transaction
	mock.ExpectBegin()
	mock.ExpectCommit()

	// Login to get initial tokens
	_, refreshToken1, err := service.Login(ctx, email, password)
	require.NoError(t, err)

	// Verify the token was created with correct expiry
	tokens := tokenRepo.GetTokensByAccountID(account.ID)
	require.Len(t, tokens, 1, "Should have created one token")
	assert.True(t, tokens[0].Expiry.After(time.Now()), "Token should not be expired")

	// Set up mock expectations for refresh transaction
	mock.ExpectBegin()
	mock.ExpectCommit()

	// Small delay to ensure timestamps differ (JWT includes iat timestamp)
	time.Sleep(2 * time.Millisecond)

	// Refresh the token
	accessToken2, refreshToken2, err := service.RefreshToken(ctx, refreshToken1)
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken2, "New access token should not be empty")
	assert.NotEmpty(t, refreshToken2, "New refresh token should not be empty")

	// Note: Access tokens might be identical if generated in the same millisecond
	// The important thing is that refresh succeeds and invalidates the old refresh token
	assert.NotEqual(t, refreshToken1, refreshToken2, "New refresh token should be different")

	// Verify we now have 2 tokens total (old was deleted, new was created)
	// Actually, refresh should delete old token and create new one, so still 1 token
	tokensAfter := tokenRepo.GetTokensByAccountID(account.ID)
	assert.Len(t, tokensAfter, 1, "Should still have 1 token (old deleted, new created)")
}

// TestRefreshTokenInvalid tests refresh with invalid token
func TestRefreshTokenInvalid(t *testing.T) {
	service, _, _, _, _, _, _, _, cleanup := newTestAuthService(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	accessToken, refreshToken, err := service.RefreshToken(ctx, "invalid-token")

	require.Error(t, err)
	assert.Empty(t, accessToken)
	assert.Empty(t, refreshToken)
}

// TestConcurrentLogins tests concurrent login attempts
func TestConcurrentLogins(t *testing.T) {
	service, accountRepo, _, _, _, _, _, mock, cleanup := newTestAuthService(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Create test account
	password := "ValidPassword123!"
	hash, err := userpass.HashPassword(password, nil)
	require.NoError(t, err)

	email := "concurrent@example.com"
	account := &authModel.Account{
		Model:        baseModel.Model{ID: 30},
		Email:        email,
		PasswordHash: &hash,
		Active:       true,
	}

	accountRepo.storeAccount(account)

	// Set up mock expectations for 10 concurrent transactions
	concurrency := 10
	for i := 0; i < concurrency; i++ {
		mock.ExpectBegin()
		mock.ExpectCommit()
	}

	results := make(chan struct {
		accessToken  string
		refreshToken string
		err          error
	}, concurrency)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			accessToken, refreshToken, err := service.Login(ctx, email, password)
			results <- struct {
				accessToken  string
				refreshToken string
				err          error
			}{accessToken, refreshToken, err}
		}()
	}

	wg.Wait()
	close(results)

	// Verify all logins succeeded
	successCount := 0
	uniqueAccessTokens := make(map[string]bool)
	uniqueRefreshTokens := make(map[string]bool)

	for result := range results {
		if result.err == nil {
			successCount++
			uniqueAccessTokens[result.accessToken] = true
			uniqueRefreshTokens[result.refreshToken] = true
		}
	}

	assert.Equal(t, concurrency, successCount, "All concurrent logins should succeed")
	assert.Len(t, uniqueAccessTokens, concurrency, "All access tokens should be unique")
	assert.Len(t, uniqueRefreshTokens, concurrency, "All refresh tokens should be unique")
}

// TestTokenCleanupOnLogin tests that login cleans up old tokens
func TestTokenCleanupOnLogin(t *testing.T) {
	service, accountRepo, tokenRepo, _, _, _, _, mock, cleanup := newTestAuthService(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Create test account
	password := "ValidPassword123!"
	hash, err := userpass.HashPassword(password, nil)
	require.NoError(t, err)

	email := "cleanup@example.com"
	account := &authModel.Account{
		Model:        baseModel.Model{ID: 40},
		Email:        email,
		PasswordHash: &hash,
		Active:       true,
	}

	accountRepo.storeAccount(account)

	// Set up mock expectations for transaction
	mock.ExpectBegin()
	mock.ExpectCommit()

	// Login once
	_, _, err = service.Login(ctx, email, password)
	require.NoError(t, err)

	// Verify cleanup was called with correct parameters
	assert.Len(t, tokenRepo.cleanupCalls, 1, "CleanupOldTokensForAccount should be called")
	assert.Equal(t, account.ID, tokenRepo.cleanupCalls[0].accountID)
	assert.Equal(t, 5, tokenRepo.cleanupCalls[0].maxTokens, "Should keep max 5 tokens")
}

// TestChangePasswordSuccess tests successful password change
func TestChangePasswordSuccess(t *testing.T) {
	service, accountRepo, _, _, _, _, _, _, cleanup := newTestAuthService(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Create test account
	oldPassword := "OldPassword123!"
	hash, err := userpass.HashPassword(oldPassword, nil)
	require.NoError(t, err)

	account := &authModel.Account{
		Model:        baseModel.Model{ID: 50},
		Email:        "change@example.com",
		PasswordHash: &hash,
		Active:       true,
	}

	accountRepo.storeAccount(account)

	newPassword := "NewPassword456!"

	// Change password
	err = service.ChangePassword(ctx, int(account.ID), oldPassword, newPassword)
	require.NoError(t, err)

	// Verify password was updated
	updatedAccount := accountRepo.byID[account.ID]
	require.NotNil(t, updatedAccount.PasswordHash)

	// Verify new password works
	match, err := userpass.VerifyPassword(newPassword, *updatedAccount.PasswordHash)
	require.NoError(t, err)
	assert.True(t, match, "New password should work")

	// Verify old password doesn't work
	match, err = userpass.VerifyPassword(oldPassword, *updatedAccount.PasswordHash)
	require.NoError(t, err)
	assert.False(t, match, "Old password should not work")
}

// TestChangePasswordWrongCurrent tests password change with wrong current password
func TestChangePasswordWrongCurrent(t *testing.T) {
	service, accountRepo, _, _, _, _, _, _, cleanup := newTestAuthService(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Create test account
	password := "CurrentPassword123!"
	hash, err := userpass.HashPassword(password, nil)
	require.NoError(t, err)

	account := &authModel.Account{
		Model:        baseModel.Model{ID: 51},
		Email:        "wrong@example.com",
		PasswordHash: &hash,
		Active:       true,
	}

	accountRepo.storeAccount(account)

	// Try to change with wrong current password
	err = service.ChangePassword(ctx, int(account.ID), "WrongPassword!", "NewPassword123!")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

// TestDeactivateAccountInvalidatesTokens tests account deactivation
func TestDeactivateAccountInvalidatesTokens(t *testing.T) {
	service, accountRepo, tokenRepo, _, _, _, _, _, cleanup := newTestAuthService(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	account := &authModel.Account{
		Model:  baseModel.Model{ID: 80},
		Email:  "deactivate@example.com",
		Active: true,
	}

	accountRepo.storeAccount(account)

	// Create a token for this account
	token := &authModel.Token{
		Model:     baseModel.Model{ID: 1},
		Token:     uuid.Must(uuid.NewV4()).String(),
		AccountID: account.ID,
		Expiry:    time.Now().Add(24 * time.Hour),
	}
	_ = tokenRepo.Create(ctx, token)

	// Deactivate account
	err := service.DeactivateAccount(ctx, int(account.ID))
	require.NoError(t, err)

	// Verify account is inactive
	assert.False(t, account.Active)

	// Verify DeleteByAccountID was called
	deleteCalls := tokenRepo.GetDeleteByAccountIDCalls()
	assert.Contains(t, deleteCalls, account.ID)
}

// TestActivateAccount tests account activation
func TestActivateAccount(t *testing.T) {
	service, accountRepo, _, _, _, _, _, _, cleanup := newTestAuthService(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	account := &authModel.Account{
		Model:  baseModel.Model{ID: 90},
		Email:  "activate@example.com",
		Active: false, // Start inactive
	}

	accountRepo.storeAccount(account)

	// Activate account
	err := service.ActivateAccount(ctx, int(account.ID))
	require.NoError(t, err)

	// Verify account is active
	assert.True(t, account.Active)
}

// TestRevokeAllTokens tests token revocation
func TestRevokeAllTokens(t *testing.T) {
	service, accountRepo, tokenRepo, _, _, _, _, _, cleanup := newTestAuthService(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	account := &authModel.Account{
		Model:  baseModel.Model{ID: 100},
		Email:  "revoke@example.com",
		Active: true,
	}

	accountRepo.storeAccount(account)

	// Create multiple tokens
	for i := 0; i < 3; i++ {
		token := &authModel.Token{
			Model:     baseModel.Model{ID: int64(i + 1)},
			Token:     uuid.Must(uuid.NewV4()).String(),
			AccountID: account.ID,
			Expiry:    time.Now().Add(24 * time.Hour),
		}
		_ = tokenRepo.Create(ctx, token)
	}

	// Verify tokens exist
	tokensBefore := tokenRepo.GetTokensByAccountID(account.ID)
	assert.Len(t, tokensBefore, 3)

	// Revoke all tokens
	err := service.RevokeAllTokens(ctx, int(account.ID))
	require.NoError(t, err)

	// Verify tokens were deleted
	tokensAfter := tokenRepo.GetTokensByAccountID(account.ID)
	assert.Empty(t, tokensAfter)
}

// Helper methods for mockTokenRepositoryImpl
func (r *mockTokenRepositoryImpl) GetTokensByAccountID(accountID int64) []*authModel.Token {
	r.mu.Lock()
	defer r.mu.Unlock()

	tokens := r.byAccountID[accountID]
	result := make([]*authModel.Token, len(tokens))
	copy(result, tokens)
	return result
}

func (r *mockTokenRepositoryImpl) GetDeleteByAccountIDCalls() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]int64, len(r.deleteByAccountIDCalls))
	copy(result, r.deleteByAccountIDCalls)
	return result
}
