// Package auth internal tests for pure helper functions.
// These tests verify logic that doesn't require database access.
package auth

import (
	"net/http/httptest"
	"testing"

	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// getClientIP Tests
// =============================================================================

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "192.168.1.100")

	ip := getClientIP(req)

	assert.Equal(t, "192.168.1.100", ip)
}

func TestGetClientIP_XForwardedFor_Single(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.50")

	ip := getClientIP(req)

	assert.Equal(t, "10.0.0.50", ip)
}

func TestGetClientIP_XForwardedFor_Multiple(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2, 10.0.0.3")

	ip := getClientIP(req)

	// Should return first IP
	assert.Equal(t, "10.0.0.1", ip)
}

func TestGetClientIP_XForwardedFor_WithSpaces(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "  10.0.0.1  ,  10.0.0.2  ")

	ip := getClientIP(req)

	assert.Equal(t, "10.0.0.1", ip)
}

func TestGetClientIP_RemoteAddr_WithPort(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	ip := getClientIP(req)

	assert.Equal(t, "192.168.1.1", ip)
}

func TestGetClientIP_RemoteAddr_IPv6(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "[::1]:8080"

	ip := getClientIP(req)

	assert.Equal(t, "::1", ip)
}

func TestGetClientIP_RemoteAddr_NoPort(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1"

	ip := getClientIP(req)

	// If SplitHostPort fails, returns the full RemoteAddr
	assert.Equal(t, "192.168.1.1", ip)
}

func TestGetClientIP_XRealIP_TakesPrecedence(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "1.1.1.1")
	req.Header.Set("X-Forwarded-For", "2.2.2.2")
	req.RemoteAddr = "3.3.3.3:1234"

	ip := getClientIP(req)

	// X-Real-IP takes precedence
	assert.Equal(t, "1.1.1.1", ip)
}

func TestGetClientIP_XForwardedFor_TakesPrecedence_OverRemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "2.2.2.2")
	req.RemoteAddr = "3.3.3.3:1234"

	ip := getClientIP(req)

	// X-Forwarded-For takes precedence over RemoteAddr
	assert.Equal(t, "2.2.2.2", ip)
}

// =============================================================================
// Request/Response Type Tests
// =============================================================================

func TestLoginRequest_Fields(t *testing.T) {
	req := LoginRequest{
		Email:    "test@example.com",
		Password: "password123",
	}

	assert.Equal(t, "test@example.com", req.Email)
	assert.Equal(t, "password123", req.Password)
}

func TestTokenResponse_Fields(t *testing.T) {
	resp := TokenResponse{
		AccessToken:  "access_token_value",
		RefreshToken: "refresh_token_value",
	}

	assert.Equal(t, "access_token_value", resp.AccessToken)
	assert.Equal(t, "refresh_token_value", resp.RefreshToken)
}

func TestRegisterRequest_Fields(t *testing.T) {
	roleID := int64(5)
	req := RegisterRequest{
		Email:           "test@example.com",
		Username:        "testuser",
		Password:        "password123",
		ConfirmPassword: "password123",
		RoleID:          &roleID,
	}

	assert.Equal(t, "test@example.com", req.Email)
	assert.Equal(t, "testuser", req.Username)
	assert.Equal(t, "password123", req.Password)
	assert.Equal(t, "password123", req.ConfirmPassword)
	assert.Equal(t, int64(5), *req.RoleID)
}

func TestChangePasswordRequest_Fields(t *testing.T) {
	req := ChangePasswordRequest{
		CurrentPassword: "old_password",
		NewPassword:     "new_password",
		ConfirmPassword: "new_password",
	}

	assert.Equal(t, "old_password", req.CurrentPassword)
	assert.Equal(t, "new_password", req.NewPassword)
	assert.Equal(t, "new_password", req.ConfirmPassword)
}

func TestCreateRoleRequest_Fields(t *testing.T) {
	req := CreateRoleRequest{
		Name:        "admin",
		Description: "Administrator role",
	}

	assert.Equal(t, "admin", req.Name)
	assert.Equal(t, "Administrator role", req.Description)
}

func TestUpdateRoleRequest_Fields(t *testing.T) {
	req := UpdateRoleRequest{
		Name:        "updated_role",
		Description: "Updated description",
	}

	assert.Equal(t, "updated_role", req.Name)
	assert.Equal(t, "Updated description", req.Description)
}

func TestCreatePermissionRequest_Fields(t *testing.T) {
	req := CreatePermissionRequest{
		Name:        "read_users",
		Description: "Read users permission",
		Resource:    "users",
		Action:      "read",
	}

	assert.Equal(t, "read_users", req.Name)
	assert.Equal(t, "Read users permission", req.Description)
	assert.Equal(t, "users", req.Resource)
	assert.Equal(t, "read", req.Action)
}

func TestUpdatePermissionRequest_Fields(t *testing.T) {
	req := UpdatePermissionRequest{
		Name:        "updated_permission",
		Description: "Updated description",
		Resource:    "groups",
		Action:      "write",
	}

	assert.Equal(t, "updated_permission", req.Name)
	assert.Equal(t, "Updated description", req.Description)
	assert.Equal(t, "groups", req.Resource)
	assert.Equal(t, "write", req.Action)
}

func TestPasswordResetRequest_Fields(t *testing.T) {
	req := PasswordResetRequest{
		Email: "reset@example.com",
	}

	assert.Equal(t, "reset@example.com", req.Email)
}

func TestAccountResponse_Fields(t *testing.T) {
	resp := AccountResponse{
		ID:          1,
		Email:       "test@example.com",
		Username:    "testuser",
		Active:      true,
		Roles:       []string{"admin", "user"},
		Permissions: []string{"read_users", "write_users"},
	}

	assert.Equal(t, int64(1), resp.ID)
	assert.Equal(t, "test@example.com", resp.Email)
	assert.Equal(t, "testuser", resp.Username)
	assert.True(t, resp.Active)
	assert.Len(t, resp.Roles, 2)
	assert.Len(t, resp.Permissions, 2)
}

func TestRoleResponse_Fields(t *testing.T) {
	resp := RoleResponse{
		ID:          1,
		Name:        "admin",
		Description: "Administrator",
		CreatedAt:   "2024-01-01T00:00:00Z",
		UpdatedAt:   "2024-01-01T00:00:00Z",
		Permissions: []string{"read_users"},
	}

	assert.Equal(t, int64(1), resp.ID)
	assert.Equal(t, "admin", resp.Name)
	assert.Equal(t, "Administrator", resp.Description)
	assert.NotEmpty(t, resp.CreatedAt)
	assert.NotEmpty(t, resp.UpdatedAt)
}

func TestPermissionResponse_Fields(t *testing.T) {
	resp := PermissionResponse{
		ID:          1,
		Name:        "read_users",
		Description: "Read users",
		Resource:    "users",
		Action:      "read",
		CreatedAt:   "2024-01-01T00:00:00Z",
		UpdatedAt:   "2024-01-01T00:00:00Z",
	}

	assert.Equal(t, int64(1), resp.ID)
	assert.Equal(t, "read_users", resp.Name)
	assert.Equal(t, "Read users", resp.Description)
	assert.Equal(t, "users", resp.Resource)
	assert.Equal(t, "read", resp.Action)
}

func TestUpdateAccountRequest_Fields(t *testing.T) {
	req := UpdateAccountRequest{
		Email:    "new@example.com",
		Username: "newuser",
	}

	assert.Equal(t, "new@example.com", req.Email)
	assert.Equal(t, "newuser", req.Username)
}

// =============================================================================
// Resource Tests
// =============================================================================

func TestNewResource_ReturnsResource(t *testing.T) {
	// Create resource with nil services (just testing initialization)
	resource := NewResource(nil, nil)

	assert.NotNil(t, resource)
}

// =============================================================================
// isValidAuthHeader Tests
// =============================================================================

func TestIsValidAuthHeader_ValidBearerToken(t *testing.T) {
	result := isValidAuthHeader("Bearer abc123")
	assert.True(t, result)
}

func TestIsValidAuthHeader_ValidBearerToken_LongToken(t *testing.T) {
	// Use a fake long token string (not a real JWT to avoid security scanner false positives)
	result := isValidAuthHeader("Bearer fake-long-token-string-for-testing-purposes-only-not-a-real-jwt")
	assert.True(t, result)
}

func TestIsValidAuthHeader_EmptyString(t *testing.T) {
	result := isValidAuthHeader("")
	assert.False(t, result)
}

func TestIsValidAuthHeader_NoBearer(t *testing.T) {
	result := isValidAuthHeader("abc123")
	assert.False(t, result)
}

func TestIsValidAuthHeader_WrongPrefix(t *testing.T) {
	result := isValidAuthHeader("Basic abc123")
	assert.False(t, result)
}

func TestIsValidAuthHeader_LowercaseBearer(t *testing.T) {
	result := isValidAuthHeader("bearer abc123")
	assert.False(t, result)
}

func TestIsValidAuthHeader_BearerOnly(t *testing.T) {
	// "Bearer " is 7 characters, which fails the >= 8 length check
	result := isValidAuthHeader("Bearer ")
	assert.False(t, result) // fails length check (need at least 8 chars)
}

func TestIsValidAuthHeader_BearerWithOneChar(t *testing.T) {
	// "Bearer x" is 8 characters, which passes the >= 8 length check
	result := isValidAuthHeader("Bearer x")
	assert.True(t, result) // passes length check
}

func TestIsValidAuthHeader_ShortString(t *testing.T) {
	result := isValidAuthHeader("Bear")
	assert.False(t, result)
}

// =============================================================================
// hasAdminRole Tests
// =============================================================================

func TestHasAdminRole_WithAdminRole(t *testing.T) {
	roles := []*authModel.Role{
		{Name: "user"},
		{Name: "admin"},
		{Name: "moderator"},
	}
	result := hasAdminRole(roles)
	assert.True(t, result)
}

func TestHasAdminRole_WithoutAdminRole(t *testing.T) {
	roles := []*authModel.Role{
		{Name: "user"},
		{Name: "moderator"},
	}
	result := hasAdminRole(roles)
	assert.False(t, result)
}

func TestHasAdminRole_EmptyRoles(t *testing.T) {
	roles := []*authModel.Role{}
	result := hasAdminRole(roles)
	assert.False(t, result)
}

func TestHasAdminRole_NilRoles(t *testing.T) {
	var roles []*authModel.Role = nil
	result := hasAdminRole(roles)
	assert.False(t, result)
}

func TestHasAdminRole_OnlyAdminRole(t *testing.T) {
	roles := []*authModel.Role{
		{Name: "admin"},
	}
	result := hasAdminRole(roles)
	assert.True(t, result)
}

func TestHasAdminRole_AdminLikeButNotAdmin(t *testing.T) {
	roles := []*authModel.Role{
		{Name: "administrator"},
		{Name: "Admin"},
		{Name: "ADMIN"},
	}
	result := hasAdminRole(roles)
	assert.False(t, result) // exact match "admin" required
}
