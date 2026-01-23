// Package auth_test tests the auth API handlers.
package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/tenant"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// =============================================================================
// Mock AuthService Implementation
// =============================================================================

type mockAuthService struct {
	// Login
	loginAccessToken  string
	loginRefreshToken string
	loginErr          error

	// Register
	registerAccount *authModels.Account
	registerErr     error

	// ValidateToken
	validateTokenAccount *authModels.Account
	validateTokenErr     error

	// RefreshToken
	refreshAccessToken string
	refreshNewRefresh  string
	refreshErr         error

	// Logout
	logoutErr error

	// ChangePassword
	changePasswordErr error

	// GetAccountByID
	getAccountByIDAccount *authModels.Account
	getAccountByIDErr     error

	// GetAccountByEmail
	getAccountByEmailAccount *authModels.Account
	getAccountByEmailErr     error

	// Role Management
	createRoleRole *authModels.Role
	createRoleErr  error

	getRoleByIDRole *authModels.Role
	getRoleByIDErr  error

	getRoleByNameRole *authModels.Role
	getRoleByNameErr  error

	updateRoleErr error
	deleteRoleErr error

	listRolesRoles []*authModels.Role
	listRolesErr   error

	assignRoleToAccountErr   error
	removeRoleFromAccountErr error

	getAccountRolesRoles []*authModels.Role
	getAccountRolesErr   error

	// Permission Management
	createPermissionPermission *authModels.Permission
	createPermissionErr        error

	getPermissionByIDPermission *authModels.Permission
	getPermissionByIDErr        error

	getPermissionByNamePermission *authModels.Permission
	getPermissionByNameErr        error

	updatePermissionErr error
	deletePermissionErr error

	listPermissionsPermissions []*authModels.Permission
	listPermissionsErr         error

	grantPermissionToAccountErr    error
	denyPermissionToAccountErr     error
	removePermissionFromAccountErr error

	getAccountPermissionsPermissions []*authModels.Permission
	getAccountPermissionsErr         error

	getAccountDirectPermissionsPermissions []*authModels.Permission
	getAccountDirectPermissionsErr         error

	assignPermissionToRoleErr   error
	removePermissionFromRoleErr error

	getRolePermissionsPermissions []*authModels.Permission
	getRolePermissionsErr         error

	// Account Management
	activateAccountErr   error
	deactivateAccountErr error
	updateAccountErr     error

	listAccountsAccounts []*authModels.Account
	listAccountsErr      error

	getAccountsByRoleAccounts []*authModels.Account
	getAccountsByRoleErr      error

	getAccountsWithRolesAndPermissionsAccounts []*authModels.Account
	getAccountsWithRolesAndPermissionsErr      error

	// Password Reset
	initiatePasswordResetToken *authModels.PasswordResetToken
	initiatePasswordResetErr   error

	resetPasswordErr error

	cleanupExpiredRateLimitsCount int
	cleanupExpiredRateLimitsErr   error

	// Token Management
	cleanupExpiredTokensCount int
	cleanupExpiredTokensErr   error

	cleanupExpiredPasswordResetTokensCount int
	cleanupExpiredPasswordResetTokensErr   error

	revokeAllTokensErr error

	getActiveTokensTokens []*authModels.Token
	getActiveTokensErr    error

	// Parent Account Management
	createParentAccountAccount *authModels.AccountParent
	createParentAccountErr     error

	getParentAccountByIDAccount *authModels.AccountParent
	getParentAccountByIDErr     error

	getParentAccountByEmailAccount *authModels.AccountParent
	getParentAccountByEmailErr     error

	updateParentAccountErr     error
	activateParentAccountErr   error
	deactivateParentAccountErr error

	listParentAccountsAccounts []*authModels.AccountParent
	listParentAccountsErr      error
}

// Implement AuthService interface
func (m *mockAuthService) Login(_ context.Context, _, _ string) (string, string, error) {
	return m.loginAccessToken, m.loginRefreshToken, m.loginErr
}

func (m *mockAuthService) LoginWithAudit(_ context.Context, _, _, _, _ string) (string, string, error) {
	return m.loginAccessToken, m.loginRefreshToken, m.loginErr
}

func (m *mockAuthService) Register(_ context.Context, _, _, _ string, _ *int64) (*authModels.Account, error) {
	return m.registerAccount, m.registerErr
}

func (m *mockAuthService) ValidateToken(_ context.Context, _ string) (*authModels.Account, error) {
	return m.validateTokenAccount, m.validateTokenErr
}

func (m *mockAuthService) RefreshToken(_ context.Context, _ string) (string, string, error) {
	return m.refreshAccessToken, m.refreshNewRefresh, m.refreshErr
}

func (m *mockAuthService) RefreshTokenWithAudit(_ context.Context, _, _, _ string) (string, string, error) {
	return m.refreshAccessToken, m.refreshNewRefresh, m.refreshErr
}

func (m *mockAuthService) Logout(_ context.Context, _ string) error {
	return m.logoutErr
}

func (m *mockAuthService) LogoutWithAudit(_ context.Context, _, _, _ string) error {
	return m.logoutErr
}

func (m *mockAuthService) ChangePassword(_ context.Context, _ int, _, _ string) error {
	return m.changePasswordErr
}

func (m *mockAuthService) GetAccountByID(_ context.Context, _ int) (*authModels.Account, error) {
	return m.getAccountByIDAccount, m.getAccountByIDErr
}

func (m *mockAuthService) GetAccountByEmail(_ context.Context, _ string) (*authModels.Account, error) {
	return m.getAccountByEmailAccount, m.getAccountByEmailErr
}

func (m *mockAuthService) CreateRole(_ context.Context, _, _ string) (*authModels.Role, error) {
	return m.createRoleRole, m.createRoleErr
}

func (m *mockAuthService) GetRoleByID(_ context.Context, _ int) (*authModels.Role, error) {
	return m.getRoleByIDRole, m.getRoleByIDErr
}

func (m *mockAuthService) GetRoleByName(_ context.Context, _ string) (*authModels.Role, error) {
	return m.getRoleByNameRole, m.getRoleByNameErr
}

func (m *mockAuthService) UpdateRole(_ context.Context, _ *authModels.Role) error {
	return m.updateRoleErr
}

func (m *mockAuthService) DeleteRole(_ context.Context, _ int) error {
	return m.deleteRoleErr
}

func (m *mockAuthService) ListRoles(_ context.Context, _ map[string]interface{}) ([]*authModels.Role, error) {
	return m.listRolesRoles, m.listRolesErr
}

func (m *mockAuthService) AssignRoleToAccount(_ context.Context, _, _ int) error {
	return m.assignRoleToAccountErr
}

func (m *mockAuthService) RemoveRoleFromAccount(_ context.Context, _, _ int) error {
	return m.removeRoleFromAccountErr
}

func (m *mockAuthService) GetAccountRoles(_ context.Context, _ int) ([]*authModels.Role, error) {
	return m.getAccountRolesRoles, m.getAccountRolesErr
}

func (m *mockAuthService) CreatePermission(_ context.Context, _, _, _, _ string) (*authModels.Permission, error) {
	return m.createPermissionPermission, m.createPermissionErr
}

func (m *mockAuthService) GetPermissionByID(_ context.Context, _ int) (*authModels.Permission, error) {
	return m.getPermissionByIDPermission, m.getPermissionByIDErr
}

func (m *mockAuthService) GetPermissionByName(_ context.Context, _ string) (*authModels.Permission, error) {
	return m.getPermissionByNamePermission, m.getPermissionByNameErr
}

func (m *mockAuthService) UpdatePermission(_ context.Context, _ *authModels.Permission) error {
	return m.updatePermissionErr
}

func (m *mockAuthService) DeletePermission(_ context.Context, _ int) error {
	return m.deletePermissionErr
}

func (m *mockAuthService) ListPermissions(_ context.Context, _ map[string]interface{}) ([]*authModels.Permission, error) {
	return m.listPermissionsPermissions, m.listPermissionsErr
}

func (m *mockAuthService) GrantPermissionToAccount(_ context.Context, _, _ int) error {
	return m.grantPermissionToAccountErr
}

func (m *mockAuthService) DenyPermissionToAccount(_ context.Context, _, _ int) error {
	return m.denyPermissionToAccountErr
}

func (m *mockAuthService) RemovePermissionFromAccount(_ context.Context, _, _ int) error {
	return m.removePermissionFromAccountErr
}

func (m *mockAuthService) GetAccountPermissions(_ context.Context, _ int) ([]*authModels.Permission, error) {
	return m.getAccountPermissionsPermissions, m.getAccountPermissionsErr
}

func (m *mockAuthService) GetAccountDirectPermissions(_ context.Context, _ int) ([]*authModels.Permission, error) {
	return m.getAccountDirectPermissionsPermissions, m.getAccountDirectPermissionsErr
}

func (m *mockAuthService) AssignPermissionToRole(_ context.Context, _, _ int) error {
	return m.assignPermissionToRoleErr
}

func (m *mockAuthService) RemovePermissionFromRole(_ context.Context, _, _ int) error {
	return m.removePermissionFromRoleErr
}

func (m *mockAuthService) GetRolePermissions(_ context.Context, _ int) ([]*authModels.Permission, error) {
	return m.getRolePermissionsPermissions, m.getRolePermissionsErr
}

func (m *mockAuthService) ActivateAccount(_ context.Context, _ int) error {
	return m.activateAccountErr
}

func (m *mockAuthService) DeactivateAccount(_ context.Context, _ int) error {
	return m.deactivateAccountErr
}

func (m *mockAuthService) UpdateAccount(_ context.Context, _ *authModels.Account) error {
	return m.updateAccountErr
}

func (m *mockAuthService) ListAccounts(_ context.Context, _ map[string]interface{}) ([]*authModels.Account, error) {
	return m.listAccountsAccounts, m.listAccountsErr
}

func (m *mockAuthService) GetAccountsByRole(_ context.Context, _ string) ([]*authModels.Account, error) {
	return m.getAccountsByRoleAccounts, m.getAccountsByRoleErr
}

func (m *mockAuthService) GetAccountsWithRolesAndPermissions(_ context.Context, _ map[string]interface{}) ([]*authModels.Account, error) {
	return m.getAccountsWithRolesAndPermissionsAccounts, m.getAccountsWithRolesAndPermissionsErr
}

func (m *mockAuthService) InitiatePasswordReset(_ context.Context, _ string) (*authModels.PasswordResetToken, error) {
	return m.initiatePasswordResetToken, m.initiatePasswordResetErr
}

func (m *mockAuthService) ResetPassword(_ context.Context, _, _ string) error {
	return m.resetPasswordErr
}

func (m *mockAuthService) CleanupExpiredRateLimits(_ context.Context) (int, error) {
	return m.cleanupExpiredRateLimitsCount, m.cleanupExpiredRateLimitsErr
}

func (m *mockAuthService) CleanupExpiredTokens(_ context.Context) (int, error) {
	return m.cleanupExpiredTokensCount, m.cleanupExpiredTokensErr
}

func (m *mockAuthService) CleanupExpiredPasswordResetTokens(_ context.Context) (int, error) {
	return m.cleanupExpiredPasswordResetTokensCount, m.cleanupExpiredPasswordResetTokensErr
}

func (m *mockAuthService) RevokeAllTokens(_ context.Context, _ int) error {
	return m.revokeAllTokensErr
}

func (m *mockAuthService) GetActiveTokens(_ context.Context, _ int) ([]*authModels.Token, error) {
	return m.getActiveTokensTokens, m.getActiveTokensErr
}

func (m *mockAuthService) CreateParentAccount(_ context.Context, _, _, _ string) (*authModels.AccountParent, error) {
	return m.createParentAccountAccount, m.createParentAccountErr
}

func (m *mockAuthService) GetParentAccountByID(_ context.Context, _ int) (*authModels.AccountParent, error) {
	return m.getParentAccountByIDAccount, m.getParentAccountByIDErr
}

func (m *mockAuthService) GetParentAccountByEmail(_ context.Context, _ string) (*authModels.AccountParent, error) {
	return m.getParentAccountByEmailAccount, m.getParentAccountByEmailErr
}

func (m *mockAuthService) UpdateParentAccount(_ context.Context, _ *authModels.AccountParent) error {
	return m.updateParentAccountErr
}

func (m *mockAuthService) ActivateParentAccount(_ context.Context, _ int) error {
	return m.activateParentAccountErr
}

func (m *mockAuthService) DeactivateParentAccount(_ context.Context, _ int) error {
	return m.deactivateParentAccountErr
}

func (m *mockAuthService) ListParentAccounts(_ context.Context, _ map[string]interface{}) ([]*authModels.AccountParent, error) {
	return m.listParentAccountsAccounts, m.listParentAccountsErr
}

// Transaction methods (no-op for mock)
func (m *mockAuthService) Begin(_ context.Context) (context.Context, error) {
	return context.Background(), nil
}
func (m *mockAuthService) Commit(_ context.Context) error   { return nil }
func (m *mockAuthService) Rollback(_ context.Context) error { return nil }
func (m *mockAuthService) WithTx(_ bun.Tx) interface{}      { return m }

// =============================================================================
// Test Helpers
// =============================================================================

func newTestResource(mock *mockAuthService) *authAPI.Resource {
	return authAPI.NewResource(mock)
}

func newAuthError(op string, err error) error {
	return &authService.AuthError{Op: op, Err: err}
}

// withJWTClaims adds JWT claims to the request context
func withJWTClaims(handler http.HandlerFunc, claims jwt.AppClaims) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), jwt.CtxClaims, claims)
		ctx = context.WithValue(ctx, jwt.CtxPermissions, claims.Permissions)
		handler.ServeHTTP(w, r.WithContext(ctx))
	}
}

// withRefreshToken adds a refresh token to the request context
func withRefreshToken(handler http.HandlerFunc, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), jwt.CtxRefreshToken, token)
		handler.ServeHTTP(w, r.WithContext(ctx))
	}
}

// withTenantContext adds tenant context to the request
func withTenantContext(handler http.HandlerFunc, tc *tenant.TenantContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := tenant.SetTenantContext(r.Context(), tc)
		handler.ServeHTTP(w, r.WithContext(ctx))
	}
}

// =============================================================================
// Login Tests
// =============================================================================

func TestLogin_Success(t *testing.T) {
	mock := &mockAuthService{
		loginAccessToken:  "access-token",
		loginRefreshToken: "refresh-token",
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{"email": "test@example.com", "password": "Password123!"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp authAPI.TokenResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "access-token", resp.AccessToken)
	assert.Equal(t, "refresh-token", resp.RefreshToken)
}

func TestLogin_InvalidCredentials(t *testing.T) {
	mock := &mockAuthService{
		loginErr: newAuthError("login", authService.ErrInvalidCredentials),
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{"email": "test@example.com", "password": "wrong"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestLogin_AccountNotFound(t *testing.T) {
	mock := &mockAuthService{
		loginErr: newAuthError("login", authService.ErrAccountNotFound),
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{"email": "notfound@example.com", "password": "Password123!"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Should mask as invalid credentials
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestLogin_AccountInactive(t *testing.T) {
	mock := &mockAuthService{
		loginErr: newAuthError("login", authService.ErrAccountInactive),
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{"email": "inactive@example.com", "password": "Password123!"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestLogin_InvalidRequest(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)
	router := resource.Router()

	// Missing email
	body := map[string]string{"password": "Password123!"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestLogin_InvalidEmail(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{"email": "not-an-email", "password": "Password123!"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestLogin_InternalServerError(t *testing.T) {
	mock := &mockAuthService{
		loginErr: newAuthError("login", errors.New("database error")),
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{"email": "test@example.com", "password": "Password123!"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestLogin_NonAuthError(t *testing.T) {
	mock := &mockAuthService{
		loginErr: errors.New("unexpected error"),
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{"email": "test@example.com", "password": "Password123!"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =============================================================================
// Register Tests
// =============================================================================

func TestRegister_Success(t *testing.T) {
	username := "testuser"
	mock := &mockAuthService{
		registerAccount: &authModels.Account{
			Email:    "new@example.com",
			Username: &username,
			Active:   true,
		},
	}
	mock.registerAccount.ID = 1
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{
		"email":            "new@example.com",
		"username":         "testuser",
		"password":         "Password123!",
		"confirm_password": "Password123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestRegister_EmailAlreadyExists(t *testing.T) {
	mock := &mockAuthService{
		registerErr: newAuthError("register", authService.ErrEmailAlreadyExists),
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{
		"email":            "existing@example.com",
		"username":         "newuser",
		"password":         "Password123!",
		"confirm_password": "Password123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRegister_UsernameAlreadyExists(t *testing.T) {
	mock := &mockAuthService{
		registerErr: newAuthError("register", authService.ErrUsernameAlreadyExists),
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{
		"email":            "new@example.com",
		"username":         "existinguser",
		"password":         "Password123!",
		"confirm_password": "Password123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRegister_PasswordTooWeak(t *testing.T) {
	mock := &mockAuthService{
		registerErr: newAuthError("register", authService.ErrPasswordTooWeak),
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{
		"email":            "new@example.com",
		"username":         "newuser",
		"password":         "Password123!",
		"confirm_password": "Password123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRegister_PasswordMismatch(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{
		"email":            "new@example.com",
		"username":         "newuser",
		"password":         "Password123!",
		"confirm_password": "Different123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRegister_InvalidRequest(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)
	router := resource.Router()

	// Missing username
	body := map[string]string{
		"email":            "new@example.com",
		"password":         "Password123!",
		"confirm_password": "Password123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRegister_PasswordTooShort(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{
		"email":            "new@example.com",
		"username":         "newuser",
		"password":         "short",
		"confirm_password": "short",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRegister_InternalServerError(t *testing.T) {
	mock := &mockAuthService{
		registerErr: newAuthError("register", errors.New("database error")),
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{
		"email":            "new@example.com",
		"username":         "newuser",
		"password":         "Password123!",
		"confirm_password": "Password123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestRegister_NonAuthError(t *testing.T) {
	mock := &mockAuthService{
		registerErr: errors.New("unexpected error"),
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{
		"email":            "new@example.com",
		"username":         "newuser",
		"password":         "Password123!",
		"confirm_password": "Password123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =============================================================================
// RefreshToken Tests
// =============================================================================

func TestRefreshToken_Success(t *testing.T) {
	mock := &mockAuthService{
		refreshAccessToken: "new-access-token",
		refreshNewRefresh:  "new-refresh-token",
	}
	resource := newTestResource(mock)

	handler := withRefreshToken(resource.RefreshTokenHandler(), "valid-refresh-token")
	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp authAPI.TokenResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "new-access-token", resp.AccessToken)
	assert.Equal(t, "new-refresh-token", resp.RefreshToken)
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	mock := &mockAuthService{
		refreshErr: newAuthError("refresh", authService.ErrInvalidToken),
	}
	resource := newTestResource(mock)

	handler := withRefreshToken(resource.RefreshTokenHandler(), "invalid-token")
	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRefreshToken_TokenExpired(t *testing.T) {
	mock := &mockAuthService{
		refreshErr: newAuthError("refresh", authService.ErrTokenExpired),
	}
	resource := newTestResource(mock)

	handler := withRefreshToken(resource.RefreshTokenHandler(), "expired-token")
	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRefreshToken_TokenNotFound(t *testing.T) {
	mock := &mockAuthService{
		refreshErr: newAuthError("refresh", authService.ErrTokenNotFound),
	}
	resource := newTestResource(mock)

	handler := withRefreshToken(resource.RefreshTokenHandler(), "notfound-token")
	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRefreshToken_AccountNotFound(t *testing.T) {
	mock := &mockAuthService{
		refreshErr: newAuthError("refresh", authService.ErrAccountNotFound),
	}
	resource := newTestResource(mock)

	handler := withRefreshToken(resource.RefreshTokenHandler(), "token-for-deleted-account")
	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRefreshToken_AccountInactive(t *testing.T) {
	mock := &mockAuthService{
		refreshErr: newAuthError("refresh", authService.ErrAccountInactive),
	}
	resource := newTestResource(mock)

	handler := withRefreshToken(resource.RefreshTokenHandler(), "token-for-inactive-account")
	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRefreshToken_InternalError(t *testing.T) {
	mock := &mockAuthService{
		refreshErr: newAuthError("refresh", errors.New("database error")),
	}
	resource := newTestResource(mock)

	handler := withRefreshToken(resource.RefreshTokenHandler(), "valid-token")
	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestRefreshToken_NonAuthError(t *testing.T) {
	mock := &mockAuthService{
		refreshErr: errors.New("unexpected error"),
	}
	resource := newTestResource(mock)

	handler := withRefreshToken(resource.RefreshTokenHandler(), "valid-token")
	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =============================================================================
// Logout Tests
// =============================================================================

func TestLogout_Success(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)

	handler := withRefreshToken(resource.LogoutHandler(), "valid-refresh-token")
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestLogout_WithError_StillSucceeds(t *testing.T) {
	mock := &mockAuthService{
		logoutErr: errors.New("audit logging failed"),
	}
	resource := newTestResource(mock)

	handler := withRefreshToken(resource.LogoutHandler(), "valid-refresh-token")
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Logout should succeed even if audit logging fails
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

// =============================================================================
// ChangePassword JWT Tests
// =============================================================================

func TestChangePasswordJWT_Success(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)

	claims := jwt.AppClaims{ID: 1}
	handler := withJWTClaims(resource.ChangePasswordJWTHandler(), claims)

	body := map[string]string{
		"current_password": "OldPassword123!",
		"new_password":     "NewPassword123!",
		"confirm_password": "NewPassword123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestChangePasswordJWT_InvalidCredentials(t *testing.T) {
	mock := &mockAuthService{
		changePasswordErr: newAuthError("change_password", authService.ErrInvalidCredentials),
	}
	resource := newTestResource(mock)

	claims := jwt.AppClaims{ID: 1}
	handler := withJWTClaims(resource.ChangePasswordJWTHandler(), claims)

	body := map[string]string{
		"current_password": "WrongPassword123!",
		"new_password":     "NewPassword123!",
		"confirm_password": "NewPassword123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestChangePasswordJWT_AccountNotFound(t *testing.T) {
	mock := &mockAuthService{
		changePasswordErr: newAuthError("change_password", authService.ErrAccountNotFound),
	}
	resource := newTestResource(mock)

	claims := jwt.AppClaims{ID: 999}
	handler := withJWTClaims(resource.ChangePasswordJWTHandler(), claims)

	body := map[string]string{
		"current_password": "OldPassword123!",
		"new_password":     "NewPassword123!",
		"confirm_password": "NewPassword123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestChangePasswordJWT_PasswordTooWeak(t *testing.T) {
	mock := &mockAuthService{
		changePasswordErr: newAuthError("change_password", authService.ErrPasswordTooWeak),
	}
	resource := newTestResource(mock)

	claims := jwt.AppClaims{ID: 1}
	handler := withJWTClaims(resource.ChangePasswordJWTHandler(), claims)

	body := map[string]string{
		"current_password": "OldPassword123!",
		"new_password":     "NewPassword123!",
		"confirm_password": "NewPassword123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestChangePasswordJWT_InvalidRequest(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)

	claims := jwt.AppClaims{ID: 1}
	handler := withJWTClaims(resource.ChangePasswordJWTHandler(), claims)

	// Missing confirm_password
	body := map[string]string{
		"current_password": "OldPassword123!",
		"new_password":     "NewPassword123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestChangePasswordJWT_PasswordMismatch(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)

	claims := jwt.AppClaims{ID: 1}
	handler := withJWTClaims(resource.ChangePasswordJWTHandler(), claims)

	body := map[string]string{
		"current_password": "OldPassword123!",
		"new_password":     "NewPassword123!",
		"confirm_password": "DifferentPassword!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestChangePasswordJWT_InternalError(t *testing.T) {
	mock := &mockAuthService{
		changePasswordErr: newAuthError("change_password", errors.New("database error")),
	}
	resource := newTestResource(mock)

	claims := jwt.AppClaims{ID: 1}
	handler := withJWTClaims(resource.ChangePasswordJWTHandler(), claims)

	body := map[string]string{
		"current_password": "OldPassword123!",
		"new_password":     "NewPassword123!",
		"confirm_password": "NewPassword123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestChangePasswordJWT_NonAuthError(t *testing.T) {
	mock := &mockAuthService{
		changePasswordErr: errors.New("unexpected error"),
	}
	resource := newTestResource(mock)

	claims := jwt.AppClaims{ID: 1}
	handler := withJWTClaims(resource.ChangePasswordJWTHandler(), claims)

	body := map[string]string{
		"current_password": "OldPassword123!",
		"new_password":     "NewPassword123!",
		"confirm_password": "NewPassword123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =============================================================================
// ChangePassword Tenant Tests
// =============================================================================

func TestChangePasswordTenant_Success(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)

	accountID := int64(1)
	tc := &tenant.TenantContext{AccountID: &accountID}
	handler := withTenantContext(resource.ChangePasswordTenantHandler(), tc)

	body := map[string]string{
		"current_password": "OldPassword123!",
		"new_password":     "NewPassword123!",
		"confirm_password": "NewPassword123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestChangePasswordTenant_NoAccountID(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)

	tc := &tenant.TenantContext{AccountID: nil}
	handler := withTenantContext(resource.ChangePasswordTenantHandler(), tc)

	body := map[string]string{
		"current_password": "OldPassword123!",
		"new_password":     "NewPassword123!",
		"confirm_password": "NewPassword123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// =============================================================================
// GetAccount JWT Tests
// =============================================================================

func TestGetAccountJWT_Success(t *testing.T) {
	username := "testuser"
	mock := &mockAuthService{
		getAccountByIDAccount: &authModels.Account{
			Email:    "test@example.com",
			Username: &username,
			Active:   true,
			Roles: []*authModels.Role{
				{Name: "admin"},
			},
		},
	}
	mock.getAccountByIDAccount.ID = 1
	resource := newTestResource(mock)

	claims := jwt.AppClaims{ID: 1, Permissions: []string{"users:read"}}
	handler := withJWTClaims(resource.GetAccountJWTHandler(), claims)

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGetAccountJWT_NotFound(t *testing.T) {
	mock := &mockAuthService{
		getAccountByIDErr: newAuthError("get_account", authService.ErrAccountNotFound),
	}
	resource := newTestResource(mock)

	claims := jwt.AppClaims{ID: 999}
	handler := withJWTClaims(resource.GetAccountJWTHandler(), claims)

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestGetAccountJWT_InternalError(t *testing.T) {
	mock := &mockAuthService{
		getAccountByIDErr: newAuthError("get_account", errors.New("database error")),
	}
	resource := newTestResource(mock)

	claims := jwt.AppClaims{ID: 1}
	handler := withJWTClaims(resource.GetAccountJWTHandler(), claims)

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGetAccountJWT_NonAuthError(t *testing.T) {
	mock := &mockAuthService{
		getAccountByIDErr: errors.New("unexpected error"),
	}
	resource := newTestResource(mock)

	claims := jwt.AppClaims{ID: 1}
	handler := withJWTClaims(resource.GetAccountJWTHandler(), claims)

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =============================================================================
// GetAccount Tenant Tests
// =============================================================================

func TestGetAccountTenant_Success(t *testing.T) {
	username := "testuser"
	mock := &mockAuthService{
		getAccountByIDAccount: &authModels.Account{
			Email:    "test@example.com",
			Username: &username,
			Active:   true,
		},
	}
	mock.getAccountByIDAccount.ID = 1
	resource := newTestResource(mock)

	accountID := int64(1)
	tc := &tenant.TenantContext{
		AccountID:   &accountID,
		Permissions: []string{"users:read"},
	}
	handler := withTenantContext(resource.GetAccountTenantHandler(), tc)

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGetAccountTenant_NoTenantContext(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)

	// No tenant context
	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	rr := httptest.NewRecorder()

	resource.GetAccountTenantHandler().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestGetAccountTenant_NoAccountID(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)

	tc := &tenant.TenantContext{AccountID: nil}
	handler := withTenantContext(resource.GetAccountTenantHandler(), tc)

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestGetAccountTenant_NotFound(t *testing.T) {
	mock := &mockAuthService{
		getAccountByIDErr: newAuthError("get_account", authService.ErrAccountNotFound),
	}
	resource := newTestResource(mock)

	accountID := int64(999)
	tc := &tenant.TenantContext{AccountID: &accountID}
	handler := withTenantContext(resource.GetAccountTenantHandler(), tc)

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// =============================================================================
// Password Reset Tests
// =============================================================================

func TestInitiatePasswordReset_Success(t *testing.T) {
	mock := &mockAuthService{
		initiatePasswordResetToken: &authModels.PasswordResetToken{},
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{"email": "test@example.com"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password-reset", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Should return success regardless of whether email exists (security)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestInitiatePasswordReset_InvalidEmail(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{"email": "not-an-email"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password-reset", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestResetPassword_Success(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{
		"token":            "valid-reset-token",
		"new_password":     "NewPassword123!",
		"confirm_password": "NewPassword123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password-reset/confirm", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestResetPassword_InvalidToken(t *testing.T) {
	mock := &mockAuthService{
		resetPasswordErr: newAuthError("reset_password", authService.ErrInvalidToken),
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{
		"token":            "invalid-token",
		"new_password":     "NewPassword123!",
		"confirm_password": "NewPassword123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password-reset/confirm", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestResetPassword_TokenExpired(t *testing.T) {
	mock := &mockAuthService{
		resetPasswordErr: newAuthError("reset_password", authService.ErrTokenExpired),
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{
		"token":            "expired-token",
		"new_password":     "NewPassword123!",
		"confirm_password": "NewPassword123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password-reset/confirm", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// ErrTokenExpired falls through to internal server error in current implementation
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestResetPassword_PasswordMismatch(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{
		"token":            "valid-token",
		"new_password":     "NewPassword123!",
		"confirm_password": "DifferentPassword!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password-reset/confirm", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// =============================================================================
// Register with RoleID Tests (authorizeRoleAssignment coverage)
// =============================================================================

func TestRegister_WithRoleID_AdminCanAssignRole(t *testing.T) {
	username := "newuser"
	mock := &mockAuthService{
		// ValidateToken returns admin account
		validateTokenAccount: &authModels.Account{
			Email:    "admin@example.com",
			Username: ptr("admin"),
			Active:   true,
			Roles:    []*authModels.Role{{Name: "admin"}},
		},
		// Register returns created account
		registerAccount: &authModels.Account{
			Email:    "new@example.com",
			Username: &username,
			Active:   true,
		},
	}
	mock.validateTokenAccount.ID = 1
	mock.registerAccount.ID = 2
	resource := newTestResource(mock)
	router := resource.Router()

	roleID := int64(5)
	body := map[string]interface{}{
		"email":            "new@example.com",
		"username":         "newuser",
		"password":         "Password123!",
		"confirm_password": "Password123!",
		"role_id":          roleID,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-admin-token")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestRegister_WithRoleID_NonAdminDenied(t *testing.T) {
	mock := &mockAuthService{
		// ValidateToken returns non-admin account
		validateTokenAccount: &authModels.Account{
			Email:    "user@example.com",
			Username: ptr("user"),
			Active:   true,
			Roles:    []*authModels.Role{{Name: "teacher"}}, // Not admin
		},
	}
	mock.validateTokenAccount.ID = 1
	resource := newTestResource(mock)
	router := resource.Router()

	roleID := int64(5)
	body := map[string]interface{}{
		"email":            "new@example.com",
		"username":         "newuser",
		"password":         "Password123!",
		"confirm_password": "Password123!",
		"role_id":          roleID,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-user-token")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Non-admin should be denied
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRegister_WithRoleID_InvalidTokenIgnoresRoleID(t *testing.T) {
	username := "newuser"
	mock := &mockAuthService{
		// ValidateToken returns error
		validateTokenErr: errors.New("invalid token"),
		// Register should still succeed without role
		registerAccount: &authModels.Account{
			Email:    "new@example.com",
			Username: &username,
			Active:   true,
		},
	}
	mock.registerAccount.ID = 1
	resource := newTestResource(mock)
	router := resource.Router()

	roleID := int64(5)
	body := map[string]interface{}{
		"email":            "new@example.com",
		"username":         "newuser",
		"password":         "Password123!",
		"confirm_password": "Password123!",
		"role_id":          roleID,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer invalid-token")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Should still create account, but ignore role_id
	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestRegister_WithRoleID_NoAuthHeaderIgnoresRoleID(t *testing.T) {
	username := "newuser"
	mock := &mockAuthService{
		registerAccount: &authModels.Account{
			Email:    "new@example.com",
			Username: &username,
			Active:   true,
		},
	}
	mock.registerAccount.ID = 1
	resource := newTestResource(mock)
	router := resource.Router()

	roleID := int64(5)
	body := map[string]interface{}{
		"email":            "new@example.com",
		"username":         "newuser",
		"password":         "Password123!",
		"confirm_password": "Password123!",
		"role_id":          roleID,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Should still create account, but ignore role_id
	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestRegister_WithRoleID_ShortAuthHeaderIgnoresRoleID(t *testing.T) {
	username := "newuser"
	mock := &mockAuthService{
		registerAccount: &authModels.Account{
			Email:    "new@example.com",
			Username: &username,
			Active:   true,
		},
	}
	mock.registerAccount.ID = 1
	resource := newTestResource(mock)
	router := resource.Router()

	roleID := int64(5)
	body := map[string]interface{}{
		"email":            "new@example.com",
		"username":         "newuser",
		"password":         "Password123!",
		"confirm_password": "Password123!",
		"role_id":          roleID,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic x") // Too short, wrong format
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Should still create account, but ignore role_id
	assert.Equal(t, http.StatusCreated, rr.Code)
}

// =============================================================================
// InitiatePasswordReset Additional Tests
// =============================================================================

func TestInitiatePasswordReset_RateLimitExceeded(t *testing.T) {
	retryAt := time.Now().Add(30 * time.Minute)
	mock := &mockAuthService{
		initiatePasswordResetErr: &authService.RateLimitError{
			Err:     authService.ErrRateLimitExceeded,
			RetryAt: retryAt,
		},
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{"email": "test@example.com"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password-reset", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("Retry-After"))
}

func TestInitiatePasswordReset_RateLimitExceeded_NoRetryAt(t *testing.T) {
	mock := &mockAuthService{
		initiatePasswordResetErr: &authService.RateLimitError{
			Err:     authService.ErrRateLimitExceeded,
			RetryAt: time.Time{}, // Zero value
		},
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{"email": "test@example.com"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password-reset", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestInitiatePasswordReset_InternalError(t *testing.T) {
	mock := &mockAuthService{
		initiatePasswordResetErr: errors.New("database connection failed"),
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{"email": "test@example.com"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password-reset", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =============================================================================
// ResetPassword Additional Tests
// =============================================================================

func TestResetPassword_PasswordTooWeak(t *testing.T) {
	mock := &mockAuthService{
		resetPasswordErr: newAuthError("reset_password", authService.ErrPasswordTooWeak),
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{
		"token":            "valid-token",
		"new_password":     "weak",
		"confirm_password": "weak",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password-reset/confirm", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestResetPassword_NonAuthError(t *testing.T) {
	mock := &mockAuthService{
		resetPasswordErr: errors.New("unexpected error"),
	}
	resource := newTestResource(mock)
	router := resource.Router()

	body := map[string]string{
		"token":            "valid-token",
		"new_password":     "NewPassword123!",
		"confirm_password": "NewPassword123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password-reset/confirm", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =============================================================================
// ChangePasswordTenant Additional Tests
// =============================================================================

func TestChangePasswordTenant_InvalidRequest(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)

	accountID := int64(1)
	tc := &tenant.TenantContext{AccountID: &accountID}
	handler := withTenantContext(resource.ChangePasswordTenantHandler(), tc)

	// Missing fields
	body := map[string]string{
		"current_password": "OldPassword123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestChangePasswordTenant_PasswordMismatch(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)

	accountID := int64(1)
	tc := &tenant.TenantContext{AccountID: &accountID}
	handler := withTenantContext(resource.ChangePasswordTenantHandler(), tc)

	body := map[string]string{
		"current_password": "OldPassword123!",
		"new_password":     "NewPassword123!",
		"confirm_password": "DifferentPassword!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestChangePasswordTenant_InternalError(t *testing.T) {
	mock := &mockAuthService{
		changePasswordErr: newAuthError("change_password", errors.New("database error")),
	}
	resource := newTestResource(mock)

	accountID := int64(1)
	tc := &tenant.TenantContext{AccountID: &accountID}
	handler := withTenantContext(resource.ChangePasswordTenantHandler(), tc)

	body := map[string]string{
		"current_password": "OldPassword123!",
		"new_password":     "NewPassword123!",
		"confirm_password": "NewPassword123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestChangePasswordTenant_NonAuthError(t *testing.T) {
	mock := &mockAuthService{
		changePasswordErr: errors.New("unexpected error"),
	}
	resource := newTestResource(mock)

	accountID := int64(1)
	tc := &tenant.TenantContext{AccountID: &accountID}
	handler := withTenantContext(resource.ChangePasswordTenantHandler(), tc)

	body := map[string]string{
		"current_password": "OldPassword123!",
		"new_password":     "NewPassword123!",
		"confirm_password": "NewPassword123!",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/password", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =============================================================================
// GetAccountTenant Additional Tests
// =============================================================================

func TestGetAccountTenant_InternalError(t *testing.T) {
	mock := &mockAuthService{
		getAccountByIDErr: newAuthError("get_account", errors.New("database error")),
	}
	resource := newTestResource(mock)

	accountID := int64(1)
	tc := &tenant.TenantContext{AccountID: &accountID}
	handler := withTenantContext(resource.GetAccountTenantHandler(), tc)

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGetAccountTenant_NonAuthError(t *testing.T) {
	mock := &mockAuthService{
		getAccountByIDErr: errors.New("unexpected error"),
	}
	resource := newTestResource(mock)

	accountID := int64(1)
	tc := &tenant.TenantContext{AccountID: &accountID}
	handler := withTenantContext(resource.GetAccountTenantHandler(), tc)

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =============================================================================
// Tenant Router Specific Tests
// =============================================================================

func TestTenantRefreshNotSupported(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)
	router := resource.TenantRouter()

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Returns 400 Bad Request with error message
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestTenantLogoutNotSupported(t *testing.T) {
	mock := &mockAuthService{}
	resource := newTestResource(mock)
	router := resource.TenantRouter()

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Returns 400 Bad Request with error message
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// =============================================================================
// Helper function
// =============================================================================

func ptr(s string) *string {
	return &s
}
