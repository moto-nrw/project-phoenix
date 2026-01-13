// Package auth_test tests the auth API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
//
// Test Strategy:
// - Public endpoints (login, register, password-reset): Test through full router
// - Protected endpoints: Test handlers directly with context injection (bypass JWT verifier)
package auth_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test resources
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *authAPI.Resource
}

// setupTestContext creates test resources for auth handler tests
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)
	resource := authAPI.NewResource(svc.Auth, svc.Invitation)

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	})

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// setupPublicRouter creates a router for testing public endpoints
func setupPublicRouter(t *testing.T) chi.Router {
	t.Helper()

	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Mount("/auth", tc.resource.Router())

	return router
}

// setupProtectedRouter creates a router for testing protected endpoints
// This bypasses JWT verification by using permission middleware only
func setupProtectedRouter(t *testing.T) (*testContext, chi.Router) {
	t.Helper()

	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))

	// Mount routes without JWT middleware for testing
	// We'll set context values directly in tests
	router.Route("/auth", func(r chi.Router) {
		// Account endpoint
		r.With(authorize.RequiresPermission("")).Get("/account", tc.resource.GetAccountHandler())

		// Role management
		r.Route("/roles", func(r chi.Router) {
			r.With(authorize.RequiresPermission("roles:read")).Get("/", tc.resource.ListRolesHandler())
			r.With(authorize.RequiresPermission("roles:create")).Post("/", tc.resource.CreateRoleHandler())
			r.With(authorize.RequiresPermission("roles:read")).Get("/{id}", tc.resource.GetRoleByIDHandler())
			r.With(authorize.RequiresPermission("roles:delete")).Delete("/{id}", tc.resource.DeleteRoleHandler())
		})

		// Permission management
		r.Route("/permissions", func(r chi.Router) {
			r.With(authorize.RequiresPermission("permissions:read")).Get("/", tc.resource.ListPermissionsHandler())
			r.With(authorize.RequiresPermission("permissions:create")).Post("/", tc.resource.CreatePermissionHandler())
			r.With(authorize.RequiresPermission("permissions:read")).Get("/{id}", tc.resource.GetPermissionByIDHandler())
		})

		// Account management
		r.Route("/accounts", func(r chi.Router) {
			r.With(authorize.RequiresPermission("users:list")).Get("/", tc.resource.ListAccountsHandler())
		})

		// Password change (no permission required, just auth)
		r.Post("/password", tc.resource.ChangePasswordHandler())
	})

	return tc, router
}

// executeWithAuth executes a request with JWT context values set
func executeWithAuth(router chi.Router, req *http.Request, claims jwt.AppClaims, permissions []string) *httptest.ResponseRecorder {
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	ctx = context.WithValue(ctx, jwt.CtxPermissions, permissions)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// ============================================================================
// PUBLIC ENDPOINT TESTS
// ============================================================================

// TestLogin tests the login endpoint
func TestLogin(t *testing.T) {
	router := setupPublicRouter(t)

	t.Run("success with valid credentials", func(t *testing.T) {
		body := map[string]string{
			"email":    "admin@example.com",
			"password": "Test1234%",
		}

		req := testutil.NewJSONRequest("POST", "/auth/login", body)
		rr := testutil.ExecuteRequest(router, req)

		assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		assert.NotEmpty(t, response["access_token"], "Expected access_token in response")
		assert.NotEmpty(t, response["refresh_token"], "Expected refresh_token in response")
	})

	t.Run("unauthorized with invalid password", func(t *testing.T) {
		body := map[string]string{
			"email":    "admin@example.com",
			"password": "WrongPassword123!",
		}

		req := testutil.NewJSONRequest("POST", "/auth/login", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertUnauthorized(t, rr)
	})

	t.Run("unauthorized with non-existent email", func(t *testing.T) {
		body := map[string]string{
			"email":    "nonexistent@example.com",
			"password": "Test1234%",
		}

		req := testutil.NewJSONRequest("POST", "/auth/login", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertUnauthorized(t, rr)
	})

	t.Run("bad request with invalid email format", func(t *testing.T) {
		body := map[string]string{
			"email":    "not-an-email",
			"password": "Test1234%",
		}

		req := testutil.NewJSONRequest("POST", "/auth/login", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad request with missing password", func(t *testing.T) {
		body := map[string]string{
			"email": "admin@example.com",
		}

		req := testutil.NewJSONRequest("POST", "/auth/login", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad request with empty body", func(t *testing.T) {
		req := testutil.NewJSONRequest("POST", "/auth/login", nil)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})
}

// TestRegister tests the registration endpoint
func TestRegister(t *testing.T) {
	router := setupPublicRouter(t)

	t.Run("success with valid data", func(t *testing.T) {
		// Use unique email to avoid conflicts with seed data
		email := fmt.Sprintf("testregister_%d@example.com", time.Now().UnixNano())
		username := fmt.Sprintf("user%d", time.Now().UnixNano()%100000)

		body := map[string]string{
			"email":            email,
			"username":         username,
			"password":         "SecurePass123!",
			"confirm_password": "SecurePass123!",
		}

		req := testutil.NewJSONRequest("POST", "/auth/register", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "Expected data to be an object")
		assert.Equal(t, email, data["email"])
		assert.Equal(t, username, data["username"])
	})

	t.Run("bad request with duplicate email", func(t *testing.T) {
		// Use unique email that we register twice
		uniqueEmail := fmt.Sprintf("duplicate_%d@example.com", time.Now().UnixNano())

		// First registration
		body := map[string]string{
			"email":            uniqueEmail,
			"username":         fmt.Sprintf("user1_%d", time.Now().UnixNano()%100000),
			"password":         "SecurePass123!",
			"confirm_password": "SecurePass123!",
		}
		req := testutil.NewJSONRequest("POST", "/auth/register", body)
		rr := testutil.ExecuteRequest(router, req)
		require.Equal(t, http.StatusCreated, rr.Code, "First registration should succeed. Body: %s", rr.Body.String())

		// Second registration with same email, different username
		body["username"] = fmt.Sprintf("user2_%d", time.Now().UnixNano()%100000)
		req = testutil.NewJSONRequest("POST", "/auth/register", body)
		rr = testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad request with weak password", func(t *testing.T) {
		body := map[string]string{
			"email":            "weakpass@example.com",
			"username":         "weakpassuser",
			"password":         "weak",
			"confirm_password": "weak",
		}

		req := testutil.NewJSONRequest("POST", "/auth/register", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad request with password mismatch", func(t *testing.T) {
		body := map[string]string{
			"email":            "mismatch@example.com",
			"username":         "mismatchuser",
			"password":         "SecurePass123!",
			"confirm_password": "DifferentPass123!",
		}

		req := testutil.NewJSONRequest("POST", "/auth/register", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad request with invalid email", func(t *testing.T) {
		body := map[string]string{
			"email":            "invalid-email",
			"username":         "invaliduser",
			"password":         "SecurePass123!",
			"confirm_password": "SecurePass123!",
		}

		req := testutil.NewJSONRequest("POST", "/auth/register", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad request with short username", func(t *testing.T) {
		body := map[string]string{
			"email":            "shortuser@example.com",
			"username":         "ab",
			"password":         "SecurePass123!",
			"confirm_password": "SecurePass123!",
		}

		req := testutil.NewJSONRequest("POST", "/auth/register", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})
}

// TestPasswordReset tests the password reset endpoints
func TestPasswordReset(t *testing.T) {
	router := setupPublicRouter(t)

	t.Run("initiate always returns success", func(t *testing.T) {
		body := map[string]string{
			"email": "admin@example.com",
		}

		req := testutil.NewJSONRequest("POST", "/auth/password-reset", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("initiate returns success even for non-existent email", func(t *testing.T) {
		body := map[string]string{
			"email": "nonexistent@example.com",
		}

		req := testutil.NewJSONRequest("POST", "/auth/password-reset", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("initiate bad request with invalid email", func(t *testing.T) {
		body := map[string]string{
			"email": "not-an-email",
		}

		req := testutil.NewJSONRequest("POST", "/auth/password-reset", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("confirm bad request with invalid token", func(t *testing.T) {
		body := map[string]string{
			"token":            "invalid-token",
			"new_password":     "NewSecurePass123!",
			"confirm_password": "NewSecurePass123!",
		}

		req := testutil.NewJSONRequest("POST", "/auth/password-reset/confirm", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("confirm bad request with password mismatch", func(t *testing.T) {
		body := map[string]string{
			"token":            "some-token",
			"new_password":     "NewSecurePass123!",
			"confirm_password": "DifferentPass123!",
		}

		req := testutil.NewJSONRequest("POST", "/auth/password-reset/confirm", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})
}

// TestInvitationValidation tests invitation validation endpoint (public)
func TestInvitationValidation(t *testing.T) {
	router := setupPublicRouter(t)

	t.Run("not found with invalid token", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/auth/invitations/invalid-token", nil)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertNotFound(t, rr)
	})
}

// TestInvitationAcceptance tests invitation acceptance endpoint (public)
func TestInvitationAcceptance(t *testing.T) {
	router := setupPublicRouter(t)

	t.Run("not found with invalid token", func(t *testing.T) {
		body := map[string]string{
			"password":         "SecurePass123!",
			"confirm_password": "SecurePass123!",
		}

		req := testutil.NewJSONRequest("POST", "/auth/invitations/invalid-token/accept", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertNotFound(t, rr)
	})

	// Note: Password validation tests require a valid invitation token.
	// The API validates token existence before password validation.
	// To test password validation, we would need to create a real invitation first.
	// These scenarios are covered by service-layer tests instead.

	t.Run("bad request with empty body", func(t *testing.T) {
		req := testutil.NewJSONRequest("POST", "/auth/invitations/some-token/accept", nil)
		rr := testutil.ExecuteRequest(router, req)

		// Either 400 (bad request for empty body) or 404 (token not found)
		// depends on order of validation - both are acceptable
		assert.True(t, rr.Code == http.StatusBadRequest || rr.Code == http.StatusNotFound,
			"Expected 400 or 404, got %d. Body: %s", rr.Code, rr.Body.String())
	})
}

// ============================================================================
// PROTECTED ENDPOINT TESTS
// ============================================================================

// TestGetAccount tests the get account endpoint (protected)
func TestGetAccount(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create a test account
	account := testpkg.CreateTestAccount(t, tc.db, "getaccount@example.com")
	defer testpkg.CleanupActivityFixtures(t, tc.db, account.ID)

	t.Run("success with valid claims", func(t *testing.T) {
		claims := jwt.AppClaims{
			ID:          int(account.ID),
			Sub:         account.Email,
			Username:    "testuser",
			Roles:       []string{"user"},
			Permissions: []string{"users:read"},
		}

		req := testutil.NewJSONRequest("GET", "/auth/account", nil)
		rr := executeWithAuth(router, req, claims, []string{"users:read"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "Expected data to be an object")
		assert.Equal(t, float64(account.ID), data["id"])
		assert.Equal(t, account.Email, data["email"])
	})

	t.Run("returns permissions from claims", func(t *testing.T) {
		claims := jwt.AppClaims{
			ID:          int(account.ID),
			Sub:         account.Email,
			Username:    "testuser",
			Roles:       []string{"admin"},
			Permissions: []string{"admin:*", "users:manage", "roles:manage"},
		}

		req := testutil.NewJSONRequest("GET", "/auth/account", nil)
		rr := executeWithAuth(router, req, claims, []string{"admin:*", "users:manage", "roles:manage"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "Expected data to be an object")

		permissions, ok := data["permissions"].([]interface{})
		require.True(t, ok, "Expected permissions to be an array")
		assert.Len(t, permissions, 3)
	})
}

// TestChangePassword tests the change password endpoint (protected)
func TestChangePassword(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	t.Run("bad request with wrong current password", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, tc.db, "changepass@example.com")
		defer testpkg.CleanupActivityFixtures(t, tc.db, account.ID)

		claims := jwt.AppClaims{
			ID:          int(account.ID),
			Sub:         account.Email,
			Roles:       []string{"user"},
			Permissions: []string{},
		}

		body := map[string]string{
			"current_password": "WrongCurrentPassword!",
			"new_password":     "NewSecurePass123!",
			"confirm_password": "NewSecurePass123!",
		}

		req := testutil.NewJSONRequest("POST", "/auth/password", body)
		rr := executeWithAuth(router, req, claims, []string{})

		testutil.AssertUnauthorized(t, rr)
	})

	t.Run("bad request with password mismatch", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, tc.db, "passmismatch@example.com")
		defer testpkg.CleanupActivityFixtures(t, tc.db, account.ID)

		claims := jwt.AppClaims{
			ID:          int(account.ID),
			Sub:         account.Email,
			Roles:       []string{"user"},
			Permissions: []string{},
		}

		body := map[string]string{
			"current_password": "Test1234%",
			"new_password":     "NewSecurePass123!",
			"confirm_password": "DifferentPass123!",
		}

		req := testutil.NewJSONRequest("POST", "/auth/password", body)
		rr := executeWithAuth(router, req, claims, []string{})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad request with weak new password", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, tc.db, "weaknewpass@example.com")
		defer testpkg.CleanupActivityFixtures(t, tc.db, account.ID)

		claims := jwt.AppClaims{
			ID:          int(account.ID),
			Sub:         account.Email,
			Roles:       []string{"user"},
			Permissions: []string{},
		}

		body := map[string]string{
			"current_password": "Test1234%",
			"new_password":     "weak",
			"confirm_password": "weak",
		}

		req := testutil.NewJSONRequest("POST", "/auth/password", body)
		rr := executeWithAuth(router, req, claims, []string{})

		testutil.AssertBadRequest(t, rr)
	})
}

// TestRoleManagement tests role CRUD endpoints (protected)
func TestRoleManagement(t *testing.T) {
	_, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("list roles with permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/auth/roles", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{"roles:read"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].([]interface{})
		require.True(t, ok, "Expected data to be an array")
		assert.NotEmpty(t, data, "Expected at least one role")
	})

	t.Run("list roles forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/auth/roles", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("create role with permission", func(t *testing.T) {
		roleName := fmt.Sprintf("test-role-%d", time.Now().UnixNano())
		body := map[string]string{
			"name":        roleName,
			"description": "A test role",
		}

		req := testutil.NewJSONRequest("POST", "/auth/roles", body)
		rr := executeWithAuth(router, req, adminClaims, []string{"roles:create"})

		testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "Expected data to be an object")
		assert.Equal(t, roleName, data["name"])
	})

	t.Run("create role bad request with empty name", func(t *testing.T) {
		body := map[string]string{
			"name":        "",
			"description": "A test role",
		}

		req := testutil.NewJSONRequest("POST", "/auth/roles", body)
		rr := executeWithAuth(router, req, adminClaims, []string{"roles:create"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("get role not found", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/auth/roles/99999", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{"roles:read"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("get role by valid id", func(t *testing.T) {
		// First create a role with unique name
		roleName := fmt.Sprintf("test-role-get-%d", time.Now().UnixNano())
		body := map[string]string{
			"name":        roleName,
			"description": "A test role for get",
		}

		createReq := testutil.NewJSONRequest("POST", "/auth/roles", body)
		createRr := executeWithAuth(router, createReq, adminClaims, []string{"roles:create"})
		require.Equal(t, http.StatusCreated, createRr.Code, "Role creation failed: %s", createRr.Body.String())

		createResp := testutil.ParseJSONResponse(t, createRr.Body.Bytes())
		data := createResp["data"].(map[string]interface{})
		roleID := int64(data["id"].(float64))

		// Now get the role
		req := testutil.NewJSONRequest("GET", fmt.Sprintf("/auth/roles/%d", roleID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{"roles:read"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}

// TestPermissionManagement tests permission CRUD endpoints (protected)
func TestPermissionManagement(t *testing.T) {
	_, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("list permissions with permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/auth/permissions", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{"permissions:read"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].([]interface{})
		require.True(t, ok, "Expected data to be an array")
		assert.NotEmpty(t, data, "Expected at least one permission")
	})

	t.Run("list permissions forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/auth/permissions", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("create permission with permission", func(t *testing.T) {
		permName := fmt.Sprintf("test-permission-%d", time.Now().UnixNano())
		resource := fmt.Sprintf("test%d", time.Now().UnixNano()%100000)
		body := map[string]string{
			"name":        permName,
			"description": "A test permission",
			"resource":    resource,
			"action":      "read",
		}

		req := testutil.NewJSONRequest("POST", "/auth/permissions", body)
		rr := executeWithAuth(router, req, adminClaims, []string{"permissions:create"})

		testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "Expected data to be an object")
		assert.Equal(t, permName, data["name"])
		assert.Equal(t, resource, data["resource"])
		assert.Equal(t, "read", data["action"])
	})

	t.Run("create permission bad request with missing resource", func(t *testing.T) {
		body := map[string]string{
			"name":        fmt.Sprintf("incomplete-permission-%d", time.Now().UnixNano()),
			"description": "Missing resource",
			"action":      "read",
		}

		req := testutil.NewJSONRequest("POST", "/auth/permissions", body)
		rr := executeWithAuth(router, req, adminClaims, []string{"permissions:create"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("get permission not found", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/auth/permissions/99999", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{"permissions:read"})

		testutil.AssertNotFound(t, rr)
	})
}

// TestAccountManagement tests account management endpoints (protected)
func TestAccountManagement(t *testing.T) {
	_, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("list accounts with permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/auth/accounts", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{"users:list"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].([]interface{})
		require.True(t, ok, "Expected data to be an array")
		assert.NotEmpty(t, data, "Expected at least one account")
	})

	t.Run("list accounts forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/auth/accounts", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("list accounts with email filter", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/auth/accounts?email=admin", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{"users:list"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("list accounts with active filter", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/auth/accounts?active=true", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{"users:list"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}
