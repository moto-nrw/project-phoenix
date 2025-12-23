package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// MockAuthService is a mock implementation of AuthService
type MockAuthService struct {
	mock.Mock
}

// Ensure MockAuthService implements AuthService
var _ authService.AuthService = (*MockAuthService)(nil)

func (m *MockAuthService) WithTx(tx bun.Tx) interface{} {
	args := m.Called(tx)
	return args.Get(0)
}

func (m *MockAuthService) Login(ctx context.Context, email, password string) (string, string, error) {
	args := m.Called(ctx, email, password)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockAuthService) LoginWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (string, string, error) {
	args := m.Called(ctx, email, password, ipAddress, userAgent)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockAuthService) Register(ctx context.Context, email, username, name, password string, roleID *int64) (*auth.Account, error) {
	args := m.Called(ctx, email, username, name, password, roleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.Account), args.Error(1)
}

func (m *MockAuthService) ValidateToken(ctx context.Context, token string) (*auth.Account, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.Account), args.Error(1)
}

func (m *MockAuthService) RefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	args := m.Called(ctx, refreshToken)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockAuthService) RefreshTokenWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) (string, string, error) {
	args := m.Called(ctx, refreshToken, ipAddress, userAgent)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockAuthService) Logout(ctx context.Context, refreshToken string) error {
	args := m.Called(ctx, refreshToken)
	return args.Error(0)
}

func (m *MockAuthService) LogoutWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) error {
	args := m.Called(ctx, refreshToken, ipAddress, userAgent)
	return args.Error(0)
}

func (m *MockAuthService) ChangePassword(ctx context.Context, accountID int, currentPassword, newPassword string) error {
	args := m.Called(ctx, accountID, currentPassword, newPassword)
	return args.Error(0)
}

func (m *MockAuthService) GetAccountByID(ctx context.Context, id int) (*auth.Account, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.Account), args.Error(1)
}

func (m *MockAuthService) GetAccountByEmail(ctx context.Context, email string) (*auth.Account, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.Account), args.Error(1)
}

func (m *MockAuthService) CreateRole(ctx context.Context, name, description string) (*auth.Role, error) {
	args := m.Called(ctx, name, description)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.Role), args.Error(1)
}

func (m *MockAuthService) GetRoleByID(ctx context.Context, id int) (*auth.Role, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.Role), args.Error(1)
}

func (m *MockAuthService) GetRoleByName(ctx context.Context, name string) (*auth.Role, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.Role), args.Error(1)
}

func (m *MockAuthService) UpdateRole(ctx context.Context, role *auth.Role) error {
	args := m.Called(ctx, role)
	return args.Error(0)
}

func (m *MockAuthService) DeleteRole(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAuthService) ListRoles(ctx context.Context, filters map[string]interface{}) ([]*auth.Role, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*auth.Role), args.Error(1)
}

func (m *MockAuthService) AssignRoleToAccount(ctx context.Context, accountID, roleID int) error {
	args := m.Called(ctx, accountID, roleID)
	return args.Error(0)
}

func (m *MockAuthService) RemoveRoleFromAccount(ctx context.Context, accountID, roleID int) error {
	args := m.Called(ctx, accountID, roleID)
	return args.Error(0)
}

func (m *MockAuthService) GetAccountRoles(ctx context.Context, accountID int) ([]*auth.Role, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*auth.Role), args.Error(1)
}

func (m *MockAuthService) CreatePermission(ctx context.Context, name, description, resource, action string) (*auth.Permission, error) {
	args := m.Called(ctx, name, description, resource, action)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.Permission), args.Error(1)
}

func (m *MockAuthService) GetPermissionByID(ctx context.Context, id int) (*auth.Permission, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.Permission), args.Error(1)
}

func (m *MockAuthService) GetPermissionByName(ctx context.Context, name string) (*auth.Permission, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.Permission), args.Error(1)
}

func (m *MockAuthService) UpdatePermission(ctx context.Context, permission *auth.Permission) error {
	args := m.Called(ctx, permission)
	return args.Error(0)
}

func (m *MockAuthService) DeletePermission(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAuthService) ListPermissions(ctx context.Context, filters map[string]interface{}) ([]*auth.Permission, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*auth.Permission), args.Error(1)
}

func (m *MockAuthService) GrantPermissionToAccount(ctx context.Context, accountID, permissionID int) error {
	args := m.Called(ctx, accountID, permissionID)
	return args.Error(0)
}

func (m *MockAuthService) DenyPermissionToAccount(ctx context.Context, accountID, permissionID int) error {
	args := m.Called(ctx, accountID, permissionID)
	return args.Error(0)
}

func (m *MockAuthService) RemovePermissionFromAccount(ctx context.Context, accountID, permissionID int) error {
	args := m.Called(ctx, accountID, permissionID)
	return args.Error(0)
}

func (m *MockAuthService) GetAccountPermissions(ctx context.Context, accountID int) ([]*auth.Permission, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*auth.Permission), args.Error(1)
}

func (m *MockAuthService) GetAccountDirectPermissions(ctx context.Context, accountID int) ([]*auth.Permission, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*auth.Permission), args.Error(1)
}

func (m *MockAuthService) AssignPermissionToRole(ctx context.Context, roleID, permissionID int) error {
	args := m.Called(ctx, roleID, permissionID)
	return args.Error(0)
}

func (m *MockAuthService) RemovePermissionFromRole(ctx context.Context, roleID, permissionID int) error {
	args := m.Called(ctx, roleID, permissionID)
	return args.Error(0)
}

func (m *MockAuthService) GetRolePermissions(ctx context.Context, roleID int) ([]*auth.Permission, error) {
	args := m.Called(ctx, roleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*auth.Permission), args.Error(1)
}

func (m *MockAuthService) ActivateAccount(ctx context.Context, accountID int) error {
	args := m.Called(ctx, accountID)
	return args.Error(0)
}

func (m *MockAuthService) DeactivateAccount(ctx context.Context, accountID int) error {
	args := m.Called(ctx, accountID)
	return args.Error(0)
}

func (m *MockAuthService) UpdateAccount(ctx context.Context, account *auth.Account) error {
	args := m.Called(ctx, account)
	return args.Error(0)
}

func (m *MockAuthService) ListAccounts(ctx context.Context, filters map[string]interface{}) ([]*auth.Account, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*auth.Account), args.Error(1)
}

func (m *MockAuthService) GetAccountsByRole(ctx context.Context, roleName string) ([]*auth.Account, error) {
	args := m.Called(ctx, roleName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*auth.Account), args.Error(1)
}

func (m *MockAuthService) GetAccountsWithRolesAndPermissions(ctx context.Context, filters map[string]interface{}) ([]*auth.Account, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*auth.Account), args.Error(1)
}

func (m *MockAuthService) InitiatePasswordReset(ctx context.Context, email string) (*auth.PasswordResetToken, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.PasswordResetToken), args.Error(1)
}

func (m *MockAuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	args := m.Called(ctx, token, newPassword)
	return args.Error(0)
}

func (m *MockAuthService) CleanupExpiredRateLimits(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockAuthService) CleanupExpiredTokens(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockAuthService) CleanupExpiredPasswordResetTokens(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockAuthService) RevokeAllTokens(ctx context.Context, accountID int) error {
	args := m.Called(ctx, accountID)
	return args.Error(0)
}

func (m *MockAuthService) GetActiveTokens(ctx context.Context, accountID int) ([]*auth.Token, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*auth.Token), args.Error(1)
}

func (m *MockAuthService) CreateParentAccount(ctx context.Context, email, username, password string) (*auth.AccountParent, error) {
	args := m.Called(ctx, email, username, password)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.AccountParent), args.Error(1)
}

func (m *MockAuthService) GetParentAccountByID(ctx context.Context, id int) (*auth.AccountParent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.AccountParent), args.Error(1)
}

func (m *MockAuthService) GetParentAccountByEmail(ctx context.Context, email string) (*auth.AccountParent, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.AccountParent), args.Error(1)
}

func (m *MockAuthService) UpdateParentAccount(ctx context.Context, account *auth.AccountParent) error {
	args := m.Called(ctx, account)
	return args.Error(0)
}

func (m *MockAuthService) ActivateParentAccount(ctx context.Context, accountID int) error {
	args := m.Called(ctx, accountID)
	return args.Error(0)
}

func (m *MockAuthService) DeactivateParentAccount(ctx context.Context, accountID int) error {
	args := m.Called(ctx, accountID)
	return args.Error(0)
}

func (m *MockAuthService) ListParentAccounts(ctx context.Context, filters map[string]interface{}) ([]*auth.AccountParent, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*auth.AccountParent), args.Error(1)
}

// MockInvitationService is a mock implementation of InvitationService
type MockInvitationService struct {
	mock.Mock
}

// Ensure MockInvitationService implements InvitationService
var _ authService.InvitationService = (*MockInvitationService)(nil)

func (m *MockInvitationService) WithTx(tx bun.Tx) interface{} {
	args := m.Called(tx)
	return args.Get(0)
}

func (m *MockInvitationService) CreateInvitation(ctx context.Context, req authService.InvitationRequest) (*auth.InvitationToken, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.InvitationToken), args.Error(1)
}

func (m *MockInvitationService) ValidateInvitation(ctx context.Context, token string) (*authService.InvitationValidationResult, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authService.InvitationValidationResult), args.Error(1)
}

func (m *MockInvitationService) AcceptInvitation(ctx context.Context, token string, userData authService.UserRegistrationData) (*auth.Account, error) {
	args := m.Called(ctx, token, userData)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.Account), args.Error(1)
}

func (m *MockInvitationService) ResendInvitation(ctx context.Context, invitationID int64, actorAccountID int64) error {
	args := m.Called(ctx, invitationID, actorAccountID)
	return args.Error(0)
}

func (m *MockInvitationService) ListPendingInvitations(ctx context.Context) ([]*auth.InvitationToken, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*auth.InvitationToken), args.Error(1)
}

func (m *MockInvitationService) RevokeInvitation(ctx context.Context, invitationID int64, actorAccountID int64) error {
	args := m.Called(ctx, invitationID, actorAccountID)
	return args.Error(0)
}

func (m *MockInvitationService) CleanupExpiredInvitations(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

// TestNewResource tests the NewResource constructor
func TestNewResource(t *testing.T) {
	mockAuth := new(MockAuthService)
	mockInvitation := new(MockInvitationService)

	rs := NewResource(mockAuth, mockInvitation)

	assert.NotNil(t, rs)
	assert.Equal(t, mockAuth, rs.AuthService)
	assert.Equal(t, mockInvitation, rs.InvitationService)
}

// TestRouter tests the Router function
func TestRouter(t *testing.T) {
	mockAuth := new(MockAuthService)
	mockInvitation := new(MockInvitationService)

	rs := NewResource(mockAuth, mockInvitation)
	router := rs.Router()

	assert.NotNil(t, router)
}

// TestLoginRequest_Bind tests the LoginRequest validation
func TestLoginRequest_Bind(t *testing.T) {
	tests := []struct {
		name    string
		req     LoginRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: LoginRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			wantErr: false,
		},
		{
			name: "valid request with whitespace",
			req: LoginRequest{
				Email:    "  TEST@EXAMPLE.COM  ",
				Password: "password123",
			},
			wantErr: false,
		},
		{
			name: "missing email",
			req: LoginRequest{
				Email:    "",
				Password: "password123",
			},
			wantErr: true,
		},
		{
			name: "invalid email format",
			req: LoginRequest{
				Email:    "not-an-email",
				Password: "password123",
			},
			wantErr: true,
		},
		{
			name: "missing password",
			req: LoginRequest{
				Email:    "test@example.com",
				Password: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/login", nil)
			err := tt.req.Bind(r)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestRegisterRequest_Bind tests the RegisterRequest validation
func TestRegisterRequest_Bind(t *testing.T) {
	tests := []struct {
		name    string
		req     RegisterRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: RegisterRequest{
				Email:           "test@example.com",
				Username:        "testuser",
				Name:            "Test User",
				Password:        "Password123!",
				ConfirmPassword: "Password123!",
			},
			wantErr: false,
		},
		{
			name: "passwords dont match",
			req: RegisterRequest{
				Email:           "test@example.com",
				Username:        "testuser",
				Name:            "Test User",
				Password:        "Password123!",
				ConfirmPassword: "DifferentPassword!",
			},
			wantErr: true,
		},
		{
			name: "password too short",
			req: RegisterRequest{
				Email:           "test@example.com",
				Username:        "testuser",
				Name:            "Test User",
				Password:        "short",
				ConfirmPassword: "short",
			},
			wantErr: true,
		},
		{
			name: "username too short",
			req: RegisterRequest{
				Email:           "test@example.com",
				Username:        "ab",
				Name:            "Test User",
				Password:        "Password123!",
				ConfirmPassword: "Password123!",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			req: RegisterRequest{
				Email:           "test@example.com",
				Username:        "testuser",
				Name:            "",
				Password:        "Password123!",
				ConfirmPassword: "Password123!",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/register", nil)
			err := tt.req.Bind(r)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestChangePasswordRequest_Bind tests the ChangePasswordRequest validation
func TestChangePasswordRequest_Bind(t *testing.T) {
	tests := []struct {
		name    string
		req     ChangePasswordRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: ChangePasswordRequest{
				CurrentPassword: "oldpassword",
				NewPassword:     "NewPassword123!",
				ConfirmPassword: "NewPassword123!",
			},
			wantErr: false,
		},
		{
			name: "passwords dont match",
			req: ChangePasswordRequest{
				CurrentPassword: "oldpassword",
				NewPassword:     "NewPassword123!",
				ConfirmPassword: "DifferentPassword!",
			},
			wantErr: true,
		},
		{
			name: "new password too short",
			req: ChangePasswordRequest{
				CurrentPassword: "oldpassword",
				NewPassword:     "short",
				ConfirmPassword: "short",
			},
			wantErr: true,
		},
		{
			name: "missing current password",
			req: ChangePasswordRequest{
				CurrentPassword: "",
				NewPassword:     "NewPassword123!",
				ConfirmPassword: "NewPassword123!",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/password", nil)
			err := tt.req.Bind(r)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestLogin_Success tests successful login
func TestLogin_Success(t *testing.T) {
	mockAuth := new(MockAuthService)
	mockInvitation := new(MockInvitationService)
	rs := NewResource(mockAuth, mockInvitation)

	mockAuth.On("LoginWithAudit", mock.Anything, "test@example.com", "password123", mock.Anything, mock.Anything).
		Return("access-token", "refresh-token", nil)

	body, _ := json.Marshal(LoginRequest{
		Email:    "test@example.com",
		Password: "password123",
	})

	r := chi.NewRouter()
	r.Post("/login", rs.login)

	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp TokenResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "access-token", resp.AccessToken)
	assert.Equal(t, "refresh-token", resp.RefreshToken)

	mockAuth.AssertExpectations(t)
}

// TestLogin_InvalidCredentials tests login with invalid credentials
func TestLogin_InvalidCredentials(t *testing.T) {
	mockAuth := new(MockAuthService)
	mockInvitation := new(MockInvitationService)
	rs := NewResource(mockAuth, mockInvitation)

	mockAuth.On("LoginWithAudit", mock.Anything, "test@example.com", "wrongpassword", mock.Anything, mock.Anything).
		Return("", "", &authService.AuthError{Op: "Login", Err: authService.ErrInvalidCredentials})

	body, _ := json.Marshal(LoginRequest{
		Email:    "test@example.com",
		Password: "wrongpassword",
	})

	r := chi.NewRouter()
	r.Post("/login", rs.login)

	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	mockAuth.AssertExpectations(t)
}

// TestLogin_AccountInactive tests login with inactive account
func TestLogin_AccountInactive(t *testing.T) {
	mockAuth := new(MockAuthService)
	mockInvitation := new(MockInvitationService)
	rs := NewResource(mockAuth, mockInvitation)

	mockAuth.On("LoginWithAudit", mock.Anything, "test@example.com", "password123", mock.Anything, mock.Anything).
		Return("", "", &authService.AuthError{Op: "Login", Err: authService.ErrAccountInactive})

	body, _ := json.Marshal(LoginRequest{
		Email:    "test@example.com",
		Password: "password123",
	})

	r := chi.NewRouter()
	r.Post("/login", rs.login)

	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	mockAuth.AssertExpectations(t)
}

// TestLogin_InvalidRequest tests login with invalid request body
func TestLogin_InvalidRequest(t *testing.T) {
	mockAuth := new(MockAuthService)
	mockInvitation := new(MockInvitationService)
	rs := NewResource(mockAuth, mockInvitation)

	// Invalid email format
	body, _ := json.Marshal(map[string]string{
		"email":    "not-an-email",
		"password": "password123",
	})

	r := chi.NewRouter()
	r.Post("/login", rs.login)

	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestLogin_MalformedJSON tests login with malformed JSON
func TestLogin_MalformedJSON(t *testing.T) {
	mockAuth := new(MockAuthService)
	mockInvitation := new(MockInvitationService)
	rs := NewResource(mockAuth, mockInvitation)

	r := chi.NewRouter()
	r.Post("/login", rs.login)

	req := httptest.NewRequest("POST", "/login", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestRegister_Success tests successful registration
func TestRegister_Success(t *testing.T) {
	mockAuth := new(MockAuthService)
	mockInvitation := new(MockInvitationService)
	rs := NewResource(mockAuth, mockInvitation)

	username := "testuser"
	account := &auth.Account{
		Model:    base.Model{ID: 1},
		Email:    "test@example.com",
		Username: &username,
		Active:   true,
		Roles:    []*auth.Role{{Name: "user"}},
	}

	mockAuth.On("Register", mock.Anything, "test@example.com", "testuser", "Test User", "Password123!", (*int64)(nil)).
		Return(account, nil)

	body, _ := json.Marshal(RegisterRequest{
		Email:           "test@example.com",
		Username:        "testuser",
		Name:            "Test User",
		Password:        "Password123!",
		ConfirmPassword: "Password123!",
	})

	r := chi.NewRouter()
	r.Post("/register", rs.register)

	req := httptest.NewRequest("POST", "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	mockAuth.AssertExpectations(t)
}

// TestRegister_EmailAlreadyExists tests registration with existing email
func TestRegister_EmailAlreadyExists(t *testing.T) {
	mockAuth := new(MockAuthService)
	mockInvitation := new(MockInvitationService)
	rs := NewResource(mockAuth, mockInvitation)

	mockAuth.On("Register", mock.Anything, "existing@example.com", "testuser", "Test User", "Password123!", (*int64)(nil)).
		Return(nil, &authService.AuthError{Op: "Register", Err: authService.ErrEmailAlreadyExists})

	body, _ := json.Marshal(RegisterRequest{
		Email:           "existing@example.com",
		Username:        "testuser",
		Name:            "Test User",
		Password:        "Password123!",
		ConfirmPassword: "Password123!",
	})

	r := chi.NewRouter()
	r.Post("/register", rs.register)

	req := httptest.NewRequest("POST", "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockAuth.AssertExpectations(t)
}

// TestRegister_PasswordTooWeak tests registration with weak password
func TestRegister_PasswordTooWeak(t *testing.T) {
	mockAuth := new(MockAuthService)
	mockInvitation := new(MockInvitationService)
	rs := NewResource(mockAuth, mockInvitation)

	mockAuth.On("Register", mock.Anything, "test@example.com", "testuser", "Test User", "weakpass1", (*int64)(nil)).
		Return(nil, &authService.AuthError{Op: "Register", Err: authService.ErrPasswordTooWeak})

	body, _ := json.Marshal(RegisterRequest{
		Email:           "test@example.com",
		Username:        "testuser",
		Name:            "Test User",
		Password:        "weakpass1",
		ConfirmPassword: "weakpass1",
	})

	r := chi.NewRouter()
	r.Post("/register", rs.register)

	req := httptest.NewRequest("POST", "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockAuth.AssertExpectations(t)
}

// TestInitiatePasswordReset_Success tests successful password reset initiation
func TestInitiatePasswordReset_Success(t *testing.T) {
	mockAuth := new(MockAuthService)
	mockInvitation := new(MockInvitationService)
	rs := NewResource(mockAuth, mockInvitation)

	mockAuth.On("InitiatePasswordReset", mock.Anything, "test@example.com").
		Return(&auth.PasswordResetToken{}, nil)

	body, _ := json.Marshal(map[string]string{
		"email": "test@example.com",
	})

	r := chi.NewRouter()
	r.Post("/password-reset", rs.initiatePasswordReset)

	req := httptest.NewRequest("POST", "/password-reset", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Password reset always returns 200 to prevent email enumeration
	assert.Equal(t, http.StatusOK, w.Code)
	mockAuth.AssertExpectations(t)
}

// TestGetClientIP tests the getClientIP helper function
func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expected   string
	}{
		{
			name:       "X-Forwarded-For header",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.195, 70.41.3.18, 150.172.238.178"},
			remoteAddr: "127.0.0.1:8080",
			expected:   "203.0.113.195",
		},
		{
			name:       "X-Real-IP header",
			headers:    map[string]string{"X-Real-IP": "203.0.113.195"},
			remoteAddr: "127.0.0.1:8080",
			expected:   "203.0.113.195",
		},
		{
			name:       "fallback to RemoteAddr with port",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.1:54321",
			expected:   "192.168.1.1",
		},
		{
			name:       "fallback to RemoteAddr without port",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.1",
			expected:   "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			ip := getClientIP(req)
			assert.Equal(t, tt.expected, ip)
		})
	}
}

// TestCreateRoleRequest_Bind tests CreateRoleRequest validation
func TestCreateRoleRequest_Bind(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateRoleRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: CreateRoleRequest{
				Name:        "admin",
				Description: "Administrator role",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			req: CreateRoleRequest{
				Name:        "",
				Description: "Administrator role",
			},
			wantErr: true,
		},
		{
			name: "trims whitespace",
			req: CreateRoleRequest{
				Name:        "  admin  ",
				Description: "  Administrator role  ",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/roles", nil)
			err := tt.req.Bind(r)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestPasswordResetRequest_Bind tests PasswordResetRequest validation
func TestPasswordResetRequest_Bind(t *testing.T) {
	tests := []struct {
		name    string
		req     PasswordResetRequest
		wantErr bool
	}{
		{
			name: "valid email",
			req: PasswordResetRequest{
				Email: "test@example.com",
			},
			wantErr: false,
		},
		{
			name: "missing email",
			req: PasswordResetRequest{
				Email: "",
			},
			wantErr: true,
		},
		{
			name: "invalid email format",
			req: PasswordResetRequest{
				Email: "not-an-email",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/password-reset", nil)
			err := tt.req.Bind(r)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestPasswordResetConfirmRequest_Bind tests PasswordResetConfirmRequest validation
func TestPasswordResetConfirmRequest_Bind(t *testing.T) {
	tests := []struct {
		name    string
		req     PasswordResetConfirmRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: PasswordResetConfirmRequest{
				Token:           "valid-token",
				NewPassword:     "NewPassword123!",
				ConfirmPassword: "NewPassword123!",
			},
			wantErr: false,
		},
		{
			name: "missing token",
			req: PasswordResetConfirmRequest{
				Token:           "",
				NewPassword:     "NewPassword123!",
				ConfirmPassword: "NewPassword123!",
			},
			wantErr: true,
		},
		{
			name: "passwords dont match",
			req: PasswordResetConfirmRequest{
				Token:           "valid-token",
				NewPassword:     "NewPassword123!",
				ConfirmPassword: "DifferentPassword!",
			},
			wantErr: true,
		},
		{
			name: "password too short",
			req: PasswordResetConfirmRequest{
				Token:           "valid-token",
				NewPassword:     "short",
				ConfirmPassword: "short",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/password-reset/confirm", nil)
			err := tt.req.Bind(r)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
