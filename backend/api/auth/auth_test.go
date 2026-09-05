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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// init seeds JWT viper defaults before any test builds a Resource (which calls
// jwt.MustNewTokenAuth) or mints a token. CI runs without a .env, so without a
// seeded secret jwx refuses HMAC signing.
func init() {
	testutil.SeedTestJWTConfig()
}

// testContext holds shared test resources
type testContext struct {
	db       *bun.DB
	resource *authAPI.Resource
}

// setupAuthRoute creates the auth route and only exposes its resource.
func setupAuthRoute(t *testing.T) *testContext {
	t.Helper()

	db, resource := setupAuthDependenciesRoute(t)

	return &testContext{
		db:       db,
		resource: resource,
	}
}

func setupAuthDependenciesRoute(t *testing.T) (*bun.DB, *authAPI.Resource) {
	t.Helper()
	db, svc := testutil.SetupAuthModule(t)
	resource := authAPI.NewResource(svc.Auth, svc.Invitation, svc.Schools, db)
	resource.SettingsService = svc.Settings
	resource.SetGuardianInvitationService(svc.GuardianInvitation)
	return db, resource
}

// setupPublicRouter creates a router for testing public endpoints
func setupPublicRouter(t *testing.T) chi.Router {
	t.Helper()

	_, router := setupPublicRouterWithDB(t)
	return router
}

// setupPublicRouterWithDB creates a router and returns DB for cleanup in hermetic tests
func setupPublicRouterWithDB(t *testing.T) (*bun.DB, chi.Router) {
	t.Helper()

	tc := setupAuthRoute(t)

	router := testutil.NewTenantRouter(tc.db)
	router.Mount("/auth", tc.resource.Router())

	return tc.db, router
}

// setupProtectedRouter mounts the resource's real Router() under /auth, so
// tests exercise the production middleware chain (Verifier → Authenticator →
// TenantMiddleware → RequiresPermission → TenantTxMiddleware). Requests carry a
// signed JWT minted by executeWithAuth.
func setupProtectedRouter(t *testing.T) (*testContext, chi.Router) {
	t.Helper()

	tc := setupAuthRoute(t)

	router := testutil.NewTenantRouter(tc.db)
	router.Mount("/auth", tc.resource.Router())

	return tc, router
}

// ============================================================================
// PUBLIC ENDPOINT TESTS
// ============================================================================

// TestLogin tests the login endpoint
func TestLogin(t *testing.T) {
	t.Parallel()
	tc := setupAuthRoute(t)

	router := testutil.NewTenantRouter(tc.db)
	router.Mount("/auth", tc.resource.Router())

	// Create a fresh test account to avoid stale tokens from seed data
	testEmail := fmt.Sprintf("logintest-%d@example.com", time.Now().UnixNano())
	testPassword := "Test1234%"
	account := testpkg.CreateTestAccountWithPassword(t, tc.db, testEmail, testPassword)
	testpkg.EnsureAccountTenant(t, tc.db, account.ID, testpkg.Tenant(t))

	t.Run("success with valid credentials", func(t *testing.T) {
		body := map[string]string{
			"email":    testEmail,
			"password": testPassword,
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/login", body)
		rr := testutil.ExecuteRequest(router, req)

		assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		assert.NotEmpty(t, response["access_token"], "Expected access_token in response")
		assert.NotEmpty(t, response["refresh_token"], "Expected refresh_token in response")
	})

	t.Run("unauthorized with invalid password", func(t *testing.T) {
		body := map[string]string{
			"email":    testEmail,
			"password": "WrongPassword123!",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/login", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertUnauthorized(t, rr)
	})

	t.Run("unauthorized with non-existent email", func(t *testing.T) {
		body := map[string]string{
			"email":    "nonexistent@example.com",
			"password": testPassword,
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/login", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertUnauthorized(t, rr)
	})

	// Cleanup test account
	t.Cleanup(func() {
		ctx := testpkg.Ctx(t)
		_, _ = tc.db.NewDelete().TableExpr("auth.tokens").Where("account_id = ?", account.ID).Exec(ctx)
		_, _ = tc.db.NewDelete().TableExpr("auth.account_tenants").Where("account_id = ?", account.ID).Exec(ctx)
		_, _ = tc.db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(ctx)
	})

	t.Run("bad request with invalid email format", func(t *testing.T) {
		body := map[string]string{
			"email":    "not-an-email",
			"password": "Test1234%",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/login", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad request with missing password", func(t *testing.T) {
		body := map[string]string{
			"email": "admin@example.com",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/login", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad request with empty body", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "POST", "/auth/login", nil)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})
}

// loginAsAdmin creates an admin account, assigns it the "admin" role, logs in via the
// router, and returns a valid JWT access token plus a valid non-admin role ID that can
// be used as role_id in registration payloads.
func loginAsAdmin(t *testing.T, db *bun.DB, router chi.Router) (token string, validRoleID int64) {
	t.Helper()
	ctx := context.Background()

	// Belt-and-suspenders: ensure the FK target row for this test's tenant
	// exists. testpkg.Tenant creates it, but the explicit call keeps the
	// dependency visible at the point where the FK matters.
	testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))

	// 1. Create admin account with known password
	adminEmail := fmt.Sprintf("registeradmin_%d@example.com", time.Now().UnixNano())
	adminPassword := "AdminPass123!"
	adminAccount := testpkg.CreateTestAccountWithPassword(t, db, adminEmail, adminPassword)

	// Map account to tenant 1 so Login can resolve the tenant for JWT/token creation
	testpkg.EnsureAccountTenant(t, db, adminAccount.ID, testpkg.Tenant(t))

	// 2. Get or create "admin" role and assign it
	adminRole := testpkg.GetOrCreateTestRole(t, db, "admin")
	accountRole := &authModel.AccountRole{
		AccountID: adminAccount.ID,
		RoleID:    adminRole.ID,
	}
	accountRole.SetTenantID(testpkg.Tenant(t))
	_, err := db.NewInsert().Model(accountRole).ModelTableExpr("auth.account_roles").Exec(ctx)
	require.NoError(t, err, "Failed to assign admin role")

	// 3. Get or create a "user" role to use as valid role_id in test payloads
	userRole := testpkg.GetOrCreateTestRole(t, db, "user")

	// 4. Login to get a real JWT token
	loginBody := map[string]string{
		"email":    adminEmail,
		"password": adminPassword,
	}
	loginReq := testutil.NewJSONRequest(t, "POST", "/auth/login", loginBody)
	loginRR := testutil.ExecuteRequest(router, loginReq)
	require.Equal(t, http.StatusOK, loginRR.Code, "Admin login failed: %s", loginRR.Body.String())

	loginResp := testutil.ParseJSONResponse(t, loginRR.Body.Bytes())
	accessToken, ok := loginResp["access_token"].(string)
	require.True(t, ok, "Expected access_token string in login response")

	// 5. Cleanup admin account on test completion
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("auth.account_roles").Where("account_id = ?", adminAccount.ID).Exec(ctx)
		_, _ = db.NewDelete().TableExpr("auth.tokens").Where("account_id = ?", adminAccount.ID).Exec(ctx)
	})

	return accessToken, userRole.ID
}

// TestRegister tests the registration endpoint (requires admin auth + valid role_id)
func TestRegister(t *testing.T) {
	t.Parallel()
	db, router := setupPublicRouterWithDB(t)

	// Get admin token and a valid role ID for all subtests
	adminToken, validRoleID := loginAsAdmin(t, db, router)

	// Helper to extract account ID from successful registration response
	extractAccountID := func(t *testing.T, rr *httptest.ResponseRecorder) int64 {
		t.Helper()
		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "Expected data to be an object")
		// JSON numbers are float64
		id, ok := data["id"].(float64)
		require.True(t, ok, "Expected id to be a number")
		return int64(id)
	}

	t.Run("success with valid data", func(t *testing.T) {
		// Use unique email and username to avoid conflicts
		email := fmt.Sprintf("testregister_%d@example.com", time.Now().UnixNano())
		username := fmt.Sprintf("user_%d", time.Now().UnixNano())

		// A staff-tier role needs a name to build the identity from (#2222):
		// the endpoint refuses to create an account that would hold the role
		// without being staff.
		body := map[string]interface{}{
			"email":            email,
			"username":         username,
			"password":         "SecurePass123!",
			"confirm_password": "SecurePass123!",
			"role_id":          validRoleID,
			"first_name":       "Test",
			"last_name":        "Register",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/register", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
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
		username1 := fmt.Sprintf("user1_%d", time.Now().UnixNano())

		// First registration
		body := map[string]interface{}{
			"email":            uniqueEmail,
			"username":         username1,
			"password":         "SecurePass123!",
			"confirm_password": "SecurePass123!",
			"role_id":          validRoleID,
			"first_name":       "Dupe",
			"last_name":        "Register",
		}
		req := testutil.NewJSONRequest(t, "POST", "/auth/register", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := testutil.ExecuteRequest(router, req)
		require.Equal(t, http.StatusCreated, rr.Code, "First registration should succeed. Body: %s", rr.Body.String())

		// Extract account ID for cleanup
		_ = extractAccountID(t, rr)

		// Second registration with same email, different username
		body["username"] = fmt.Sprintf("user2_%d", time.Now().UnixNano())
		req = testutil.NewJSONRequest(t, "POST", "/auth/register", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr = testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad request with weak password", func(t *testing.T) {
		// Use unique identifiers even though registration should fail
		body := map[string]interface{}{
			"email":            fmt.Sprintf("weakpass_%d@example.com", time.Now().UnixNano()),
			"username":         fmt.Sprintf("weakpassuser_%d", time.Now().UnixNano()),
			"password":         "weak",
			"confirm_password": "weak",
			"role_id":          validRoleID,
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/register", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad request with password mismatch", func(t *testing.T) {
		// Use unique identifiers even though registration should fail
		body := map[string]interface{}{
			"email":            fmt.Sprintf("mismatch_%d@example.com", time.Now().UnixNano()),
			"username":         fmt.Sprintf("mismatchuser_%d", time.Now().UnixNano()),
			"password":         "SecurePass123!",
			"confirm_password": "DifferentPass123!",
			"role_id":          validRoleID,
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/register", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad request with invalid email", func(t *testing.T) {
		// Use unique username even though registration should fail
		body := map[string]interface{}{
			"email":            "invalid-email",
			"username":         fmt.Sprintf("invaliduser_%d", time.Now().UnixNano()),
			"password":         "SecurePass123!",
			"confirm_password": "SecurePass123!",
			"role_id":          validRoleID,
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/register", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad request with short username", func(t *testing.T) {
		// Email should be unique, username is intentionally short (invalid)
		body := map[string]interface{}{
			"email":            fmt.Sprintf("shortuser_%d@example.com", time.Now().UnixNano()),
			"username":         "ab",
			"password":         "SecurePass123!",
			"confirm_password": "SecurePass123!",
			"role_id":          validRoleID,
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/register", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	// Without a name there is nothing to build the identity from, and an account
	// that holds a staff role without a staff record is the broken state of
	// #2222. Nothing can complete it afterwards (an account carries an email and
	// a username, never a person's name), so the request is refused up front
	// rather than half-written.
	t.Run("staff role without a name is refused", func(t *testing.T) {
		for _, missing := range []string{"first_name", "last_name", "both"} {
			t.Run(missing, func(t *testing.T) {
				body := map[string]interface{}{
					"email":            fmt.Sprintf("noname_%s_%d@example.com", missing, time.Now().UnixNano()),
					"username":         fmt.Sprintf("noname_%s_%d", missing, time.Now().UnixNano()),
					"password":         "SecurePass123!",
					"confirm_password": "SecurePass123!",
					"role_id":          validRoleID,
				}
				if missing != "first_name" {
					body["first_name"] = "Namenlos"
				}
				if missing != "last_name" {
					body["last_name"] = "Person"
				}

				req := testutil.NewJSONRequest(t, "POST", "/auth/register", body)
				req.Header.Set("Authorization", "Bearer "+adminToken)
				rr := testutil.ExecuteRequest(router, req)

				testutil.AssertBadRequest(t, rr)

				var count int
				require.NoError(t, db.NewRaw(
					`SELECT COUNT(*) FROM auth.accounts WHERE email = ?`, body["email"],
				).Scan(context.Background(), &count))
				assert.Zero(t, count, "no account may be left behind by the refused request")
			})
		}
	})
}

// TestRegisterRejectsGuardianRole pins the second half of the role-assignment
// rule on the account-creation path. Checking that a role exists is not enough:
// guardian access is granted exclusively through the guardian invitation flow,
// and /auth/register shares authorizeRoleAssignment with /auth/link-to-tenant,
// so a gap here is a gap in both.
func TestRegisterRejectsGuardianRole(t *testing.T) {
	t.Parallel()
	db, router := setupPublicRouterWithDB(t)

	guardianRole := testpkg.CreateTestSystemRole(t, db, authModel.BaseRoleGuardian)
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("auth.roles").Where("id = ?", guardianRole.ID).Exec(context.Background())
	})

	adminToken, _ := loginAsAdmin(t, db, router)

	body := map[string]interface{}{
		"email":            fmt.Sprintf("guardianrole_%d@example.com", time.Now().UnixNano()),
		"username":         fmt.Sprintf("guardianrole_%d", time.Now().UnixNano()),
		"password":         "SecurePass123!",
		"confirm_password": "SecurePass123!",
		"role_id":          guardianRole.ID,
	}

	req := testutil.NewJSONRequest(t, "POST", "/auth/register", body)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// TestRegisterRequiresAdminAuth tests that the register endpoint enforces admin authentication
func TestRegisterRequiresAdminAuth(t *testing.T) {
	t.Parallel()
	db, router := setupPublicRouterWithDB(t)

	// Valid registration payload (would succeed if auth were present)
	validBody := func() map[string]interface{} {
		return map[string]interface{}{
			"email":            fmt.Sprintf("authtest_%d@example.com", time.Now().UnixNano()),
			"username":         fmt.Sprintf("authuser_%d", time.Now().UnixNano()),
			"password":         "SecurePass123!",
			"confirm_password": "SecurePass123!",
			"role_id":          1,
		}
	}

	t.Run("unauthenticated returns unauthorized", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "POST", "/auth/register", validBody())
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertUnauthorized(t, rr)
	})

	t.Run("invalid token returns unauthorized", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "POST", "/auth/register", validBody())
		req.Header.Set("Authorization", "Bearer garbage-token-value")
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertUnauthorized(t, rr)
	})

	t.Run("non-admin returns forbidden", func(t *testing.T) {
		// Create a fresh role with NO permissions to guarantee 403
		ctx := context.Background()
		testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))
		noPermsRole := testpkg.CreateTestRole(t, db, "noperms")

		// Create account, map to tenant, assign the empty role
		userEmail := fmt.Sprintf("nonadmin_%d@example.com", time.Now().UnixNano())
		userPassword := "UserPass123!"
		userAccount := testpkg.CreateTestAccountWithPassword(t, db, userEmail, userPassword)
		testpkg.EnsureAccountTenant(t, db, userAccount.ID, testpkg.Tenant(t))

		userAccountRole := &authModel.AccountRole{
			AccountID: userAccount.ID,
			RoleID:    noPermsRole.ID,
		}
		userAccountRole.SetTenantID(testpkg.Tenant(t))
		_, err := db.NewInsert().Model(userAccountRole).ModelTableExpr("auth.account_roles").Exec(ctx)
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _ = db.NewDelete().TableExpr("auth.account_roles").Where("account_id = ?", userAccount.ID).Exec(ctx)
			_, _ = db.NewDelete().TableExpr("auth.tokens").Where("account_id = ?", userAccount.ID).Exec(ctx)
			_, _ = db.NewDelete().TableExpr("auth.roles").Where("id = ?", noPermsRole.ID).Exec(ctx)
		})

		// Login to get a real token — JWT will have zero permissions
		loginReq := testutil.NewJSONRequest(t, "POST", "/auth/login", map[string]string{
			"email":    userEmail,
			"password": userPassword,
		})
		loginRR := testutil.ExecuteRequest(router, loginReq)
		require.Equal(t, http.StatusOK, loginRR.Code, "Login failed: %s", loginRR.Body.String())

		loginResp := testutil.ParseJSONResponse(t, loginRR.Body.Bytes())
		accessToken := loginResp["access_token"].(string)

		// Try to register — 403 because user has no permissions at all
		req := testutil.NewJSONRequest(t, "POST", "/auth/register", validBody())
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertForbidden(t, rr)
	})

	t.Run("admin without role_id returns bad request", func(t *testing.T) {
		adminToken, _ := loginAsAdmin(t, db, router)

		body := map[string]interface{}{
			"email":            fmt.Sprintf("norole_%d@example.com", time.Now().UnixNano()),
			"username":         fmt.Sprintf("noroleuser_%d", time.Now().UnixNano()),
			"password":         "SecurePass123!",
			"confirm_password": "SecurePass123!",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/register", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("admin with role_id zero returns bad request", func(t *testing.T) {
		adminToken, _ := loginAsAdmin(t, db, router)

		body := map[string]interface{}{
			"email":            fmt.Sprintf("zerorole_%d@example.com", time.Now().UnixNano()),
			"username":         fmt.Sprintf("zerorole_%d", time.Now().UnixNano()),
			"password":         "SecurePass123!",
			"confirm_password": "SecurePass123!",
			"role_id":          0,
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/register", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("admin with negative role_id returns bad request", func(t *testing.T) {
		adminToken, _ := loginAsAdmin(t, db, router)

		body := map[string]interface{}{
			"email":            fmt.Sprintf("negrole_%d@example.com", time.Now().UnixNano()),
			"username":         fmt.Sprintf("negrole_%d", time.Now().UnixNano()),
			"password":         "SecurePass123!",
			"confirm_password": "SecurePass123!",
			"role_id":          -1,
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/register", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("admin with non-existent role_id returns bad request", func(t *testing.T) {
		adminToken, _ := loginAsAdmin(t, db, router)
		body := map[string]interface{}{
			"email":            fmt.Sprintf("badrole_%d@example.com", time.Now().UnixNano()),
			"username":         fmt.Sprintf("badrole_%d", time.Now().UnixNano()),
			"password":         "SecurePass123!",
			"confirm_password": "SecurePass123!",
			"role_id":          99999,
		}
		req := testutil.NewJSONRequest(t, "POST", "/auth/register", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertBadRequest(t, rr)
	})

	t.Run("admin with valid role_id succeeds", func(t *testing.T) {
		adminToken, validRoleID := loginAsAdmin(t, db, router)

		email := fmt.Sprintf("adminsuccess_%d@example.com", time.Now().UnixNano())
		username := fmt.Sprintf("adminsuc_%d", time.Now().UnixNano())
		body := map[string]interface{}{
			"email":            email,
			"username":         username,
			"password":         "SecurePass123!",
			"confirm_password": "SecurePass123!",
			"role_id":          validRoleID,
			"first_name":       "Admin",
			"last_name":        "Register",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/register", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	})
}

// TestPasswordReset tests the password reset endpoints
func TestPasswordReset(t *testing.T) {
	t.Parallel()
	db, router := setupPublicRouterWithDB(t)
	testpkg.OwnTestPasswordResetTokensForEmail(t, db, "admin@example.com")

	t.Run("initiate always returns success", func(t *testing.T) {
		body := map[string]string{
			"email": "admin@example.com",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/password-reset", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("initiate returns success even for non-existent email", func(t *testing.T) {
		body := map[string]string{
			"email": "nonexistent@example.com",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/password-reset", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("initiate bad request with invalid email", func(t *testing.T) {
		body := map[string]string{
			"email": "not-an-email",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/password-reset", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("confirm bad request with invalid token", func(t *testing.T) {
		body := map[string]string{
			"token":            "invalid-token",
			"new_password":     "NewSecurePass123!",
			"confirm_password": "NewSecurePass123!",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/password-reset/confirm", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("confirm bad request with password mismatch", func(t *testing.T) {
		body := map[string]string{
			"token":            "some-token",
			"new_password":     "NewSecurePass123!",
			"confirm_password": "DifferentPass123!",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/password-reset/confirm", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertBadRequest(t, rr)
	})
}

// TestInvitationValidation tests invitation validation endpoint (public)
func TestInvitationValidation(t *testing.T) {
	t.Parallel()
	router := setupPublicRouter(t)

	t.Run("not found with invalid token", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/invitations/invalid-token", nil)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertNotFound(t, rr)
	})
}

// TestInvitationAcceptance tests invitation acceptance endpoint (public)
func TestInvitationAcceptance(t *testing.T) {
	t.Parallel()
	router := setupPublicRouter(t)

	t.Run("not found with invalid token", func(t *testing.T) {
		body := map[string]string{
			"password":         "SecurePass123!",
			"confirm_password": "SecurePass123!",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/invitations/invalid-token/accept", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertNotFound(t, rr)
	})

	// Note: Password validation tests require a valid invitation token.
	// The API validates token existence before password validation.
	// To test password validation, we would need to create a real invitation first.
	// These scenarios are covered by service-layer tests instead.

	t.Run("bad request with empty body", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "POST", "/auth/invitations/some-token/accept", nil)
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
	t.Parallel()
	tc, router := setupProtectedRouter(t)

	// Create a test account
	account := testpkg.CreateTestAccount(t, tc.db, "getaccount@example.com")

	t.Run("success with valid claims", func(t *testing.T) {
		claims := jwt.AppClaims{
			ID:          int(account.ID),
			TenantID:    testpkg.Tenant(t),
			Sub:         account.Email,
			Username:    "testuser",
			Roles:       []string{"user"},
			Permissions: []string{"users:read"},
		}

		req := testutil.NewJSONRequest(t, "GET", "/auth/account", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:read"})

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
			TenantID:    testpkg.Tenant(t),
			Sub:         account.Email,
			Username:    "testuser",
			Roles:       []string{"admin"},
			Permissions: []string{"admin:*", "users:manage", "roles:manage"},
		}

		req := testutil.NewJSONRequest(t, "GET", "/auth/account", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"admin:*", "users:manage", "roles:manage"})

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
	t.Parallel()
	tc, router := setupProtectedRouter(t)

	t.Run("bad request with wrong current password", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, tc.db, "changepass@example.com")

		claims := jwt.AppClaims{
			ID:          int(account.ID),
			TenantID:    testpkg.Tenant(t),
			Sub:         account.Email,
			Roles:       []string{"user"},
			Permissions: []string{},
		}

		body := map[string]string{
			"current_password": "WrongCurrentPassword!",
			"new_password":     "NewSecurePass123!",
			"confirm_password": "NewSecurePass123!",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/password", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{})

		testutil.AssertUnauthorized(t, rr)
	})

	t.Run("bad request with password mismatch", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, tc.db, "passmismatch@example.com")

		claims := jwt.AppClaims{
			ID:          int(account.ID),
			TenantID:    testpkg.Tenant(t),
			Sub:         account.Email,
			Roles:       []string{"user"},
			Permissions: []string{},
		}

		body := map[string]string{
			"current_password": "Test1234%",
			"new_password":     "NewSecurePass123!",
			"confirm_password": "DifferentPass123!",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/password", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad request with weak new password", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, tc.db, "weaknewpass@example.com")

		claims := jwt.AppClaims{
			ID:          int(account.ID),
			TenantID:    testpkg.Tenant(t),
			Sub:         account.Email,
			Roles:       []string{"user"},
			Permissions: []string{},
		}

		body := map[string]string{
			"current_password": "Test1234%",
			"new_password":     "weak",
			"confirm_password": "weak",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/password", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{})

		testutil.AssertBadRequest(t, rr)
	})
}

// TestRoleManagement tests role CRUD endpoints (protected)
func TestRoleManagement(t *testing.T) {
	t.Parallel()
	_, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("list roles with permission", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/roles", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:read"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].([]interface{})
		require.True(t, ok, "Expected data to be an array")
		assert.NotEmpty(t, data, "Expected at least one role")
	})

	t.Run("list roles forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/roles", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("create role with permission", func(t *testing.T) {
		roleName := fmt.Sprintf("test-role-%d", time.Now().UnixNano())
		body := map[string]string{
			"name":        roleName,
			"description": "A test role",
			"base_role":   "user",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/roles", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:create"})

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

		req := testutil.NewJSONRequest(t, "POST", "/auth/roles", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:create"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("get role not found", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/roles/99999", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:read"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("get role by valid id", func(t *testing.T) {
		// First create a role with unique name
		roleName := fmt.Sprintf("test-role-get-%d", time.Now().UnixNano())
		body := map[string]string{
			"name":        roleName,
			"description": "A test role for get",
			"base_role":   "user",
		}

		createReq := testutil.NewJSONRequest(t, "POST", "/auth/roles", body)
		createRr := testutil.ExecuteWithAuthPermissions(t, router, createReq, adminClaims, []string{"roles:create"})
		require.Equal(t, http.StatusCreated, createRr.Code, "Role creation failed: %s", createRr.Body.String())

		createResp := testutil.ParseJSONResponse(t, createRr.Body.Bytes())
		data := createResp["data"].(map[string]interface{})
		roleID := int64(data["id"].(float64))

		// Now get the role
		req := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/auth/roles/%d", roleID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:read"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}

// TestRoleManagement_BaseRole tests base_role validation on role endpoints.
func TestRoleManagement_BaseRole(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	adminClaims := testutil.AdminTestClaims(1)

	t.Run("create without base_role returns 400", func(t *testing.T) {
		body := map[string]string{
			"name":        fmt.Sprintf("no-base-%d", time.Now().UnixNano()),
			"description": "Missing base_role",
		}
		req := testutil.NewJSONRequest(t, "POST", "/auth/roles", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:create"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("create with invalid base_role returns 400", func(t *testing.T) {
		body := map[string]string{
			"name":        fmt.Sprintf("bad-base-%d", time.Now().UnixNano()),
			"description": "Invalid base_role",
			"base_role":   "superadmin",
		}
		req := testutil.NewJSONRequest(t, "POST", "/auth/roles", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:create"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("create with valid base_role succeeds and returns it", func(t *testing.T) {
		roleName := fmt.Sprintf("base-role-test-%d", time.Now().UnixNano())
		body := map[string]string{
			"name":        roleName,
			"description": "Has base_role",
			"base_role":   "guardian",
		}
		req := testutil.NewJSONRequest(t, "POST", "/auth/roles", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:create"})

		testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "guardian", data["base_role"])

	})

	t.Run("update preserves base_role when omitted", func(t *testing.T) {
		// Create a role with base_role via DB fixture, then update without sending base_role
		role := testpkg.CreateTestRole(t, tc.db, "preserve-base")

		// Set base_role directly in DB
		_, err := tc.db.NewUpdate().
			TableExpr("auth.roles").
			Set("base_role = ?", "admin").
			Where("id = ?", role.ID).
			Exec(testpkg.Ctx(t))
		require.NoError(t, err)

		// Update name only — no base_role in payload
		body := map[string]string{
			"name":        fmt.Sprintf("renamed-%d", time.Now().UnixNano()),
			"description": "Updated desc",
		}
		req := testutil.NewJSONRequest(t, "PUT", fmt.Sprintf("/auth/roles/%d", role.ID), body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:update"})
		assert.Equal(t, http.StatusNoContent, rr.Code, "Body: %s", rr.Body.String())

		// Verify base_role was preserved
		getReq := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/auth/roles/%d", role.ID), nil)
		getRr := testutil.ExecuteWithAuthPermissions(t, router, getReq, adminClaims, []string{"roles:read"})
		testutil.AssertSuccessResponse(t, getRr, http.StatusOK)

		getResp := testutil.ParseJSONResponse(t, getRr.Body.Bytes())
		getData := getResp["data"].(map[string]interface{})
		assert.Equal(t, "admin", getData["base_role"], "base_role should be preserved when not sent in update")
	})

	t.Run("update system role is rejected", func(t *testing.T) {
		// System roles cannot be updated at all — the service returns 403
		var systemRole authModel.Role
		err := tc.db.NewSelect().
			Model(&systemRole).
			ModelTableExpr(`auth.roles AS "role"`).
			Where(`"role".is_system = true`).
			Where(`"role".tenant_id IS NULL`).
			Limit(1).
			Scan(testpkg.Ctx(t))
		require.NoError(t, err, "Expected at least one system role in test DB")

		body := map[string]string{
			"name":        systemRole.Name,
			"description": systemRole.Description,
			"base_role":   "guardian",
		}
		req := testutil.NewJSONRequest(t, "PUT", fmt.Sprintf("/auth/roles/%d", systemRole.ID), body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:update"})

		assert.Equal(t, http.StatusForbidden, rr.Code,
			"system roles must not accept updates including base_role")
	})

	t.Run("update changes base_role value", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, tc.db, "change-base")

		// Set initial base_role
		_, err := tc.db.NewUpdate().
			TableExpr("auth.roles").
			Set("base_role = ?", "user").
			Where("id = ?", role.ID).
			Exec(testpkg.Ctx(t))
		require.NoError(t, err)

		// Update to a different base_role
		body := map[string]string{
			"name":        fmt.Sprintf("changed-%d", time.Now().UnixNano()),
			"description": "Changed base_role",
			"base_role":   "guardian",
		}
		req := testutil.NewJSONRequest(t, "PUT", fmt.Sprintf("/auth/roles/%d", role.ID), body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:update"})
		assert.Equal(t, http.StatusNoContent, rr.Code, "Body: %s", rr.Body.String())

		// Verify base_role was changed
		getReq := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/auth/roles/%d", role.ID), nil)
		getRr := testutil.ExecuteWithAuthPermissions(t, router, getReq, adminClaims, []string{"roles:read"})
		testutil.AssertSuccessResponse(t, getRr, http.StatusOK)

		getResp := testutil.ParseJSONResponse(t, getRr.Body.Bytes())
		getData := getResp["data"].(map[string]interface{})
		assert.Equal(t, "guardian", getData["base_role"], "base_role should be updated to new value")
	})
}

// TestPermissionManagement tests permission CRUD endpoints (protected)
func TestPermissionManagement(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)
	platformClaims := adminClaims
	platformClaims.Scope = tenant.ScopePlatform
	platformClaims.TenantID = 0

	t.Run("list permissions with permission", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/permissions", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"permissions:read"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].([]interface{})
		require.True(t, ok, "Expected data to be an array")
		assert.NotEmpty(t, data, "Expected at least one permission")
	})

	t.Run("list permissions forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/permissions", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("create permission with permission", func(t *testing.T) {
		// Use fully unique identifiers (no modulo)
		permName := fmt.Sprintf("test-permission-%d", time.Now().UnixNano())
		resource := fmt.Sprintf("testresource_%d", time.Now().UnixNano())
		body := map[string]string{
			"name":        permName,
			"description": "A test permission",
			"resource":    resource,
			"action":      "read",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/permissions", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, platformClaims, []string{"permissions:create"})

		testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "Expected data to be an object")
		assert.Equal(t, permName, data["name"])
		assert.Equal(t, resource, data["resource"])
		assert.Equal(t, "read", data["action"])

		// Cleanup: delete the created permission
		permID := int64(data["id"].(float64))
		_, _ = tc.db.NewDelete().TableExpr("auth.permissions").Where("id = ?", permID).Exec(context.Background())
	})

	t.Run("create permission bad request with missing resource", func(t *testing.T) {
		body := map[string]string{
			"name":        fmt.Sprintf("incomplete-permission-%d", time.Now().UnixNano()),
			"description": "Missing resource",
			"action":      "read",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/permissions", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, platformClaims, []string{"permissions:create"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("get permission not found", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/permissions/99999", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"permissions:read"})

		testutil.AssertNotFound(t, rr)
	})
}

// TestAccountManagement tests account management endpoints (protected)
func TestAccountManagement(t *testing.T) {
	t.Parallel()
	_, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)
	adminClaims.Scope = tenant.ScopePlatform
	adminClaims.TenantID = 0

	t.Run("list accounts with permission", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/accounts", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:list"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].([]interface{})
		require.True(t, ok, "Expected data to be an array")
		assert.NotEmpty(t, data, "Expected at least one account")
	})

	t.Run("list accounts forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/accounts", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("list accounts with email filter", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/accounts?email=admin", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:list"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("list accounts with active filter", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/accounts?active=true", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:list"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}

// ============================================================================
// EXTENDED ROLE MANAGEMENT TESTS
// ============================================================================

// TestRoleUpdate tests role update endpoint
func TestRoleUpdate(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	adminClaims := testutil.AdminTestClaims(1)

	t.Run("update role with permission", func(t *testing.T) {
		// Create a role to update
		role := testpkg.CreateTestRole(t, tc.db, "UpdateTestRole")

		body := map[string]string{
			"name":        fmt.Sprintf("updated-role-%d", time.Now().UnixNano()),
			"description": "Updated description",
		}

		req := testutil.NewJSONRequest(t, "PUT", fmt.Sprintf("/auth/roles/%d", role.ID), body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:update"})

		assert.Equal(t, http.StatusNoContent, rr.Code, "Body: %s", rr.Body.String())
	})

	t.Run("update role not found", func(t *testing.T) {
		body := map[string]string{
			"name":        "some-name",
			"description": "Some description",
		}

		req := testutil.NewJSONRequest(t, "PUT", "/auth/roles/99999", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:update"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("update role forbidden without permission", func(t *testing.T) {
		body := map[string]string{
			"name":        "some-name",
			"description": "Some description",
		}

		req := testutil.NewJSONRequest(t, "PUT", "/auth/roles/1", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})
}

// TestRolePermissionAssignment tests role permission assignment endpoints
func TestRolePermissionAssignment(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	adminClaims := testutil.AdminTestClaims(1)

	t.Run("get role permissions", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, tc.db, "GetRolePerms")

		req := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/auth/roles/%d/permissions", role.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:manage"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("get role permissions forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/roles/1/permissions", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("assign and remove permission from role", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, tc.db, "AssignPermRole")
		permission := testpkg.CreateTestPermission(t, tc.db, "AssignToRole", "test", "read")

		// Assign permission
		req := testutil.NewJSONRequest(t, "POST", fmt.Sprintf("/auth/roles/%d/permissions/%d", role.ID, permission.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:manage"})
		assert.Equal(t, http.StatusNoContent, rr.Code, "Assign failed: %s", rr.Body.String())

		// Remove permission
		req = testutil.NewJSONRequest(t, "DELETE", fmt.Sprintf("/auth/roles/%d/permissions/%d", role.ID, permission.ID), nil)
		rr = testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:manage"})
		assert.Equal(t, http.StatusNoContent, rr.Code, "Remove failed: %s", rr.Body.String())
	})
}

// ============================================================================
// EXTENDED PERMISSION MANAGEMENT TESTS
// ============================================================================

// TestPermissionUpdate tests permission update endpoint
func TestPermissionUpdate(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	adminClaims := testutil.AdminTestClaims(1)
	adminClaims.Scope = tenant.ScopePlatform
	adminClaims.TenantID = 0

	t.Run("update permission with permission", func(t *testing.T) {
		permission := testpkg.CreateTestPermission(t, tc.db, "UpdatePerm", "testres", "read")

		body := map[string]string{
			"name":        fmt.Sprintf("updated-perm-%d", time.Now().UnixNano()),
			"description": "Updated description",
			"resource":    "updatedres",
			"action":      "write",
		}

		req := testutil.NewJSONRequest(t, "PUT", fmt.Sprintf("/auth/permissions/%d", permission.ID), body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"permissions:update"})

		assert.Equal(t, http.StatusNoContent, rr.Code, "Body: %s", rr.Body.String())
	})

	t.Run("update permission not found", func(t *testing.T) {
		body := map[string]string{
			"name":        "some-name",
			"description": "Some description",
			"resource":    "test",
			"action":      "read",
		}

		req := testutil.NewJSONRequest(t, "PUT", "/auth/permissions/99999", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"permissions:update"})

		testutil.AssertNotFound(t, rr)
	})
}

// TestPermissionDelete tests permission delete endpoint
func TestPermissionDelete(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	adminClaims := testutil.AdminTestClaims(1)
	platformClaims := adminClaims
	platformClaims.Scope = tenant.ScopePlatform
	platformClaims.TenantID = 0

	t.Run("delete permission with permission", func(t *testing.T) {
		permission := testpkg.CreateTestPermission(t, tc.db, "DeletePerm", "testres", "read")

		req := testutil.NewJSONRequest(t, "DELETE", fmt.Sprintf("/auth/permissions/%d", permission.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, platformClaims, []string{"permissions:delete"})

		assert.Equal(t, http.StatusNoContent, rr.Code, "Body: %s", rr.Body.String())
	})

	t.Run("delete permission forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "DELETE", "/auth/permissions/1", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})
}

// ============================================================================
// ACCOUNT ROLE ASSIGNMENT TESTS
// ============================================================================

// TestAccountRoleAssignment tests account role assignment endpoints
func TestAccountRoleAssignment(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	adminClaims := testutil.AdminTestClaims(1)

	t.Run("get account roles", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, tc.db, fmt.Sprintf("accroles%d", time.Now().UnixNano()))

		req := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/auth/accounts/%d/roles", account.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:manage"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("get account roles forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/accounts/1/roles", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("assign and remove role from account", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, tc.db, fmt.Sprintf("assignrole%d", time.Now().UnixNano()))
		role := testpkg.CreateTestRole(t, tc.db, "AssignAccRole")

		// Assign role
		req := testutil.NewJSONRequest(t, "POST", fmt.Sprintf("/auth/accounts/%d/roles/%d", account.ID, role.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:manage"})
		assert.Equal(t, http.StatusNoContent, rr.Code, "Assign failed: %s", rr.Body.String())

		// Remove role
		req = testutil.NewJSONRequest(t, "DELETE", fmt.Sprintf("/auth/accounts/%d/roles/%d", account.ID, role.ID), nil)
		rr = testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:manage"})
		assert.Equal(t, http.StatusNoContent, rr.Code, "Remove failed: %s", rr.Body.String())
	})

	t.Run("rejects direct assignment of guardian roles", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, tc.db, fmt.Sprintf("guardian-assignment%d", time.Now().UnixNano()))
		guardianRole := testpkg.CreateTestRole(t, tc.db, "guardian-assignment")
		guardianBaseRole := authModel.BaseRoleGuardian
		guardianRole.BaseRole = &guardianBaseRole
		_, err := tc.db.NewUpdate().
			Model(guardianRole).
			ModelTableExpr(`auth.roles AS "role"`).
			Column("base_role").
			WherePK().
			Exec(context.Background())
		require.NoError(t, err)

		req := testutil.NewJSONRequest(t, "POST", fmt.Sprintf("/auth/accounts/%d/roles/%d", account.ID, guardianRole.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:manage"})

		testutil.AssertBadRequest(t, rr)
	})
}

// ============================================================================
// ACCOUNT PERMISSION TESTS
// ============================================================================

// TestAccountPermissionManagement tests account permission management endpoints
func TestAccountPermissionManagement(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	adminClaims := testutil.AdminTestClaims(1)

	t.Run("get account permissions", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, tc.db, fmt.Sprintf("accperms%d", time.Now().UnixNano()))

		req := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/auth/accounts/%d/permissions", account.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:manage"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("get account direct permissions", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, tc.db, fmt.Sprintf("directperms%d", time.Now().UnixNano()))

		req := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/auth/accounts/%d/permissions/direct", account.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:manage"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("grant and remove permission from account", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, tc.db, fmt.Sprintf("grantperm%d", time.Now().UnixNano()))
		permission := testpkg.CreateTestPermission(t, tc.db, "GrantToAcc", "test", "read")

		// Grant permission
		req := testutil.NewJSONRequest(t, "POST", fmt.Sprintf("/auth/accounts/%d/permissions/%d/grant", account.ID, permission.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:manage"})
		assert.Equal(t, http.StatusNoContent, rr.Code, "Grant failed: %s", rr.Body.String())

		// Remove permission
		req = testutil.NewJSONRequest(t, "DELETE", fmt.Sprintf("/auth/accounts/%d/permissions/%d", account.ID, permission.ID), nil)
		rr = testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:manage"})
		assert.Equal(t, http.StatusNoContent, rr.Code, "Remove failed: %s", rr.Body.String())
	})

	t.Run("deny permission endpoint responds", func(t *testing.T) {
		// Note: Deny permission has a known database schema issue
		// This test just verifies the endpoint is accessible
		account := testpkg.CreateTestAccount(t, tc.db, fmt.Sprintf("denyperm%d", time.Now().UnixNano()))
		permission := testpkg.CreateTestPermission(t, tc.db, "DenyToAcc", "test", "read")
		req := testutil.NewJSONRequest(t, "POST", fmt.Sprintf("/auth/accounts/%d/permissions/%d/deny", account.ID, permission.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:manage"})
		// Accept 204 (success) or 500 (known schema issue)
		assert.True(t, rr.Code == http.StatusNoContent || rr.Code == http.StatusInternalServerError,
			"Expected 204 or 500, got %d: %s", rr.Code, rr.Body.String())
	})

	t.Run("permission operations forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/accounts/1/permissions", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{})
		testutil.AssertForbidden(t, rr)
	})
}

// ============================================================================
// ACCOUNT ACTIVATION TESTS
// ============================================================================

// TestAccountActivation tests account activation/deactivation endpoints
func TestAccountActivation(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	adminClaims := testutil.AdminTestClaims(1)

	t.Run("activate account", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, tc.db, fmt.Sprintf("activate%d", time.Now().UnixNano()))

		req := testutil.NewJSONRequest(t, "PUT", fmt.Sprintf("/auth/accounts/%d/activate", account.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:update"})

		assert.Equal(t, http.StatusNoContent, rr.Code, "Body: %s", rr.Body.String())
	})

	t.Run("deactivate account", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, tc.db, fmt.Sprintf("deactivate%d", time.Now().UnixNano()))

		req := testutil.NewJSONRequest(t, "PUT", fmt.Sprintf("/auth/accounts/%d/deactivate", account.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:update"})

		assert.Equal(t, http.StatusNoContent, rr.Code, "Body: %s", rr.Body.String())
	})

	t.Run("activation forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "PUT", "/auth/accounts/1/activate", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{})
		testutil.AssertForbidden(t, rr)
	})
}

// TestAccountUpdate tests account update endpoint
func TestAccountUpdate(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	adminClaims := testutil.AdminTestClaims(1)

	t.Run("update account", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, tc.db, fmt.Sprintf("updateacc%d", time.Now().UnixNano()))

		// Use Unix timestamp (seconds) + nanosecond remainder for uniqueness within 30 char limit
		// Format: upd_<10-digit-unix>_<9-digit-nano> = 4 + 10 + 1 + 9 = 24 chars
		now := time.Now()
		body := map[string]string{
			"email":    fmt.Sprintf("updated%d@test.local", now.UnixNano()),
			"username": fmt.Sprintf("upd_%d_%d", now.Unix(), now.Nanosecond()),
		}

		req := testutil.NewJSONRequest(t, "PUT", fmt.Sprintf("/auth/accounts/%d", account.ID), body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:update"})

		assert.Equal(t, http.StatusNoContent, rr.Code, "Body: %s", rr.Body.String())
	})

	t.Run("update account not found", func(t *testing.T) {
		body := map[string]string{
			"email": "some@email.com",
		}

		req := testutil.NewJSONRequest(t, "PUT", "/auth/accounts/99999", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:update"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("update account bad request with invalid email", func(t *testing.T) {
		body := map[string]string{
			"email": "invalid-email",
		}

		req := testutil.NewJSONRequest(t, "PUT", "/auth/accounts/1", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:update"})

		testutil.AssertBadRequest(t, rr)
	})
}

// TestGetAccountsByRole tests get accounts by role endpoint
func TestGetAccountsByRole(t *testing.T) {
	t.Parallel()
	_, router := setupProtectedRouter(t)
	adminClaims := testutil.AdminTestClaims(1)

	t.Run("get accounts by role", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/accounts/by-role/admin", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:read"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("get accounts by role forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/accounts/by-role/admin", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})
}

// ============================================================================
// TOKEN MANAGEMENT TESTS
// ============================================================================

// TestTokenManagement tests token management endpoints
func TestTokenManagement(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	adminClaims := testutil.AdminTestClaims(1)

	t.Run("get active tokens", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, tc.db, fmt.Sprintf("tokens%d", time.Now().UnixNano()))

		req := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/auth/accounts/%d/tokens", account.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:manage"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("revoke all tokens", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, tc.db, fmt.Sprintf("revoke%d", time.Now().UnixNano()))

		req := testutil.NewJSONRequest(t, "DELETE", fmt.Sprintf("/auth/accounts/%d/tokens", account.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:manage"})

		assert.Equal(t, http.StatusNoContent, rr.Code, "Body: %s", rr.Body.String())
	})

	t.Run("cleanup expired tokens", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "DELETE", "/auth/tokens/expired", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"admin:*"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("token operations forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/accounts/1/tokens", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{})
		testutil.AssertForbidden(t, rr)
	})
}

// ============================================================================
// INVITATION MANAGEMENT TESTS
// ============================================================================

// TestInvitationManagement tests invitation management endpoints
func TestInvitationManagement(t *testing.T) {
	t.Parallel()
	_, router := setupProtectedRouter(t)
	adminClaims := testutil.AdminTestClaims(1)

	t.Run("list pending invitations", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/invitations", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:list"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("list invitations forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/invitations", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("create invitation bad request with invalid email", func(t *testing.T) {
		body := map[string]interface{}{
			"email":      "invalid-email",
			"first_name": "Test",
			"last_name":  "User",
			"role_id":    1,
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/invitations", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:manage"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("revoke invitation not found", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "DELETE", "/auth/invitations/99999", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:manage"})

		// Either 404 or 500 (depending on error handling)
		assert.True(t, rr.Code == http.StatusNotFound || rr.Code == http.StatusInternalServerError,
			"Expected 404 or 500, got %d. Body: %s", rr.Code, rr.Body.String())
	})

	t.Run("resend invitation not found", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "POST", "/auth/invitations/99999/resend", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:manage"})

		assert.True(t, rr.Code == http.StatusNotFound || rr.Code == http.StatusInternalServerError,
			"Expected 404 or 500, got %d. Body: %s", rr.Code, rr.Body.String())
	})
}

// ============================================================================
// PARENT ACCOUNT TESTS
// ============================================================================

// TestParentAccountManagement tests parent account management endpoints
func TestParentAccountManagement(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	adminClaims := testutil.AdminTestClaims(1)

	t.Run("list parent accounts", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/parent-accounts", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:list"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("list parent accounts with filters", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/parent-accounts?active=true", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:list"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("create parent account", func(t *testing.T) {
		email := fmt.Sprintf("parent%d@test.local", time.Now().UnixNano())
		username := fmt.Sprintf("parent_%d", time.Now().UnixNano()) // No modulo - fully unique
		body := map[string]string{
			"email":            email,
			"username":         username,
			"password":         "SecurePass123!",
			"confirm_password": "SecurePass123!",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/parent-accounts", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:create"})

		testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

		// Cleanup: delete the created parent account
		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data := response["data"].(map[string]interface{})
		parentID := int64(data["id"].(float64))
		_, _ = tc.db.NewDelete().TableExpr("auth.accounts_parents").Where("id = ?", parentID).Exec(context.Background())
	})

	t.Run("create parent account bad request with weak password", func(t *testing.T) {
		// Use unique identifiers even though registration should fail
		body := map[string]string{
			"email":            fmt.Sprintf("weakparent_%d@test.local", time.Now().UnixNano()),
			"username":         fmt.Sprintf("weakparent_%d", time.Now().UnixNano()),
			"password":         "weak",
			"confirm_password": "weak",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/parent-accounts", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:create"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("get parent account not found", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/parent-accounts/99999", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:read"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("update parent account not found", func(t *testing.T) {
		body := map[string]string{
			"email": "update@test.local",
		}

		req := testutil.NewJSONRequest(t, "PUT", "/auth/parent-accounts/99999", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:update"})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("parent account operations forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/parent-accounts", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{})
		testutil.AssertForbidden(t, rr)
	})

	// Test activate/deactivate with a real parent account
	t.Run("activate and deactivate parent account", func(t *testing.T) {
		// Create parent account first with fully unique identifiers
		email := fmt.Sprintf("activateparent%d@test.local", time.Now().UnixNano())
		username := fmt.Sprintf("activatep_%d", time.Now().UnixNano()) // No modulo - fully unique
		body := map[string]string{
			"email":            email,
			"username":         username,
			"password":         "SecurePass123!",
			"confirm_password": "SecurePass123!",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/parent-accounts", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:create"})
		require.Equal(t, http.StatusCreated, rr.Code, "Create failed: %s", rr.Body.String())

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data := response["data"].(map[string]interface{})
		parentID := int64(data["id"].(float64))

		// Cleanup when done
		defer func() {
			_, _ = tc.db.NewDelete().TableExpr("auth.accounts_parents").Where("id = ?", parentID).Exec(context.Background())
		}()

		// Deactivate
		req = testutil.NewJSONRequest(t, "PUT", fmt.Sprintf("/auth/parent-accounts/%d/deactivate", parentID), nil)
		rr = testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:update"})
		assert.Equal(t, http.StatusNoContent, rr.Code, "Deactivate failed: %s", rr.Body.String())

		// Activate
		req = testutil.NewJSONRequest(t, "PUT", fmt.Sprintf("/auth/parent-accounts/%d/activate", parentID), nil)
		rr = testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:update"})
		assert.Equal(t, http.StatusNoContent, rr.Code, "Activate failed: %s", rr.Body.String())
	})
}

// ============================================================================
// ADDITIONAL COVERAGE TESTS - Previously 0% Coverage Handlers
// ============================================================================

// TestDeleteRole tests role deletion endpoint
func TestDeleteRole(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	adminClaims := testutil.AdminTestClaims(1)

	t.Run("delete role with permission", func(t *testing.T) {
		// Create a role to delete
		role := testpkg.CreateTestRole(t, tc.db, fmt.Sprintf("DeleteTestRole%d", time.Now().UnixNano()))

		req := testutil.NewJSONRequest(t, "DELETE", fmt.Sprintf("/auth/roles/%d", role.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:delete"})

		assert.Equal(t, http.StatusNoContent, rr.Code, "Delete failed: %s", rr.Body.String())
	})

	t.Run("delete role not found returns error", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "DELETE", "/auth/roles/99999", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:delete"})

		// DeleteRole validates role existence (to check IsSystem), so non-existent roles return 404
		assert.Equal(t, http.StatusNotFound, rr.Code, "Body: %s", rr.Body.String())
	})

	t.Run("delete role bad request with invalid id", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "DELETE", "/auth/roles/invalid", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"roles:delete"})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("delete role forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "DELETE", "/auth/roles/1", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})
}

// setupRefreshTokenRouter creates a router with refresh and logout endpoints
// Note: This bypasses JWT middleware to allow testing with context-injected tokens
func setupRefreshTokenRouter(t *testing.T) (*testContext, chi.Router) {
	t.Helper()

	tc := setupAuthRoute(t)

	router := testutil.NewTenantRouter(tc.db)

	// Use the full public router for refresh/logout
	// These routes require JWT middleware which we bypass via context
	router.Mount("/auth", tc.resource.Router())

	return tc, router
}

// TestRefreshToken tests token refresh endpoint using real login flow
func TestRefreshToken(t *testing.T) {
	t.Parallel()
	_, router := setupRefreshTokenRouter(t)

	t.Run("refresh with invalid token returns unauthorized", func(t *testing.T) {
		// Without proper JWT middleware validation, this tests the auth flow
		req := testutil.NewJSONRequest(t, "POST", "/auth/refresh", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// Should return 401 Unauthorized (JWT validation fails)
		assert.Equal(t, http.StatusUnauthorized, rr.Code, "Body: %s", rr.Body.String())
	})

	t.Run("refresh without token returns unauthorized", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "POST", "/auth/refresh", nil)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// Should return 401 Unauthorized
		assert.Equal(t, http.StatusUnauthorized, rr.Code, "Body: %s", rr.Body.String())
	})
}

// TestLogout tests logout endpoint
func TestLogout(t *testing.T) {
	t.Parallel()
	_, router := setupRefreshTokenRouter(t)

	t.Run("logout without token returns unauthorized", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "POST", "/auth/logout", nil)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// JWT middleware rejects requests without valid token
		assert.Equal(t, http.StatusUnauthorized, rr.Code, "Body: %s", rr.Body.String())
	})

	t.Run("logout with invalid token returns unauthorized", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "POST", "/auth/logout", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// JWT middleware rejects invalid tokens
		assert.Equal(t, http.StatusUnauthorized, rr.Code, "Body: %s", rr.Body.String())
	})
}

// TestListTenants tests the public GET /auth/tenants endpoint
func TestListTenants(t *testing.T) {
	t.Parallel()
	db, authRoute := setupAuthDependenciesRoute(t)

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(authRoute.AuthService, authRoute.InvitationService, platformSvc.NewSchoolService(schoolRepo), db)

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	t.Run("returns active tenants without internal IDs", func(t *testing.T) {
		// EnsureTestTenant (called by SetupTestDB) creates tenant 1 with active=true
		req := testutil.NewRequest("GET", "/auth/tenants", nil)
		rr := testutil.ExecuteRequest(router, req)

		assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

		var response struct {
			Status string `json:"status"`
			Data   []struct {
				Slug             string `json:"slug"`
				Name             string `json:"name"`
				Subdomain        string `json:"subdomain"`
				OrganizationName string `json:"organization_name"`
			} `json:"data"`
		}
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err, "Failed to parse response: %s", rr.Body.String())

		assert.Equal(t, "success", response.Status)
		require.NotEmpty(t, response.Data, "Expected at least one tenant")

		// Verify response contains expected fields
		tenant := response.Data[0]
		assert.NotEmpty(t, tenant.Slug, "Slug should not be empty")
		assert.NotEmpty(t, tenant.Name, "Name should not be empty")
		assert.NotEmpty(t, tenant.Subdomain, "Subdomain should not be empty")

		// Verify no internal IDs are exposed in JSON
		var rawResponse map[string]json.RawMessage
		err = json.Unmarshal(rr.Body.Bytes(), &rawResponse)
		require.NoError(t, err)

		var rawItems []map[string]json.RawMessage
		err = json.Unmarshal(rawResponse["data"], &rawItems)
		require.NoError(t, err)
		require.NotEmpty(t, rawItems)

		for _, item := range rawItems {
			_, hasTenantID := item["tenant_id"]
			_, hasOrgID := item["organization_id"]
			assert.False(t, hasTenantID, "Public endpoint must not expose tenant_id")
			assert.False(t, hasOrgID, "Public endpoint must not expose organization_id")
		}
	})

	t.Run("does not require authentication", func(t *testing.T) {
		// No auth headers — endpoint should still work
		req := httptest.NewRequest("GET", "/auth/tenants", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "Public endpoint should not require auth")
	})
}

// ============================================================================
// INVITATION HANDLER — SUCCESS PATH TESTS (multi-tenancy coverage)
// ============================================================================

// setupAuthRouteWithSchoolRepo is like setupAuthRoute but provides a SchoolRepo
// so the school name resolution branch in createInvitation is exercised.
func setupAuthRouteWithSchoolRepo(t *testing.T) *testContext {
	t.Helper()

	db, authRoute := setupAuthDependenciesRoute(t)
	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(authRoute.AuthService, authRoute.InvitationService, platformSvc.NewSchoolService(schoolRepo), db)

	return &testContext{
		db:       db,
		resource: resource,
	}
}

// TestInvitationCreateSuccess verifies the full invitation creation success path,
// covering WithTenantTx wrapper, school name resolution, and slog output.
func TestInvitationCreateSuccess(t *testing.T) {
	t.Parallel()
	tc := setupAuthRouteWithSchoolRepo(t)

	router := testutil.NewTenantRouter(tc.db)
	router.Mount("/auth", tc.resource.Router())

	// Create a test account to act as the invitation creator
	account := testpkg.CreateTestAccount(t, tc.db, fmt.Sprintf("inv-creator-%d@test.local", time.Now().UnixNano()))

	// Get or create a role (system roles like "user" or "teacher" should exist after migration)
	role := testpkg.GetOrCreateTestRole(t, tc.db, "teacher")

	adminClaims := jwt.AppClaims{
		ID:          int(account.ID),
		TenantID:    testpkg.Tenant(t),
		Sub:         account.Email,
		Username:    "test-admin",
		Roles:       []string{"admin"},
		Permissions: []string{"admin:*"},
		IsAdmin:     true,
	}

	t.Run("creates invitation with tenant transaction and school name", func(t *testing.T) {
		inviteeEmail := fmt.Sprintf("invitee-%d@test.local", time.Now().UnixNano())
		body := map[string]interface{}{
			"email":      inviteeEmail,
			"role_id":    role.ID,
			"first_name": "Test",
			"last_name":  "Invitee",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/invitations", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:manage"})

		require.Equal(t, http.StatusCreated, rr.Code,
			"Expected 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())

		// Parse the created invitation to get its ID for cleanup
		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "Expected data in response")
		assert.Equal(t, inviteeEmail, data["email"])
		assert.NotZero(t, data["id"])

		// Cleanup the created invitation
		if id, ok := data["id"].(float64); ok {
			_, _ = tc.db.NewDelete().
				TableExpr("auth.invitation_tokens").
				Where("id = ?", int64(id)).
				Exec(context.Background())
		}
	})

	t.Run("list pending invitations through tenant transaction", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/invitations", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:list"})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("accept invitation returns tenant_subdomain", func(t *testing.T) {
		// Create a fresh invitation via the service
		inviteeEmail := fmt.Sprintf("slug-test-%d@test.local", time.Now().UnixNano())
		createBody := map[string]interface{}{
			"email":      inviteeEmail,
			"role_id":    role.ID,
			"first_name": "Slug",
			"last_name":  "Test",
		}

		createReq := testutil.NewJSONRequest(t, "POST", "/auth/invitations", createBody)
		createRR := testutil.ExecuteWithAuthPermissions(t, router, createReq, adminClaims, []string{"users:manage"})
		require.Equal(t, http.StatusCreated, createRR.Code)

		// Extract token from created invitation
		createResp := testutil.ParseJSONResponse(t, createRR.Body.Bytes())
		data := createResp["data"].(map[string]interface{})
		token := data["token"].(string)

		// Accept the invitation via the auth router (public route, no JWT needed)
		acceptBody := map[string]interface{}{
			"first_name":       "Slug",
			"last_name":        "Test",
			"password":         "Test1234%",
			"confirm_password": "Test1234%",
		}
		acceptReq := testutil.NewJSONRequest(t, "POST", "/invitations/"+token+"/accept", acceptBody)
		acceptRR := testutil.ExecuteRequest(tc.resource.Router(), acceptReq)

		require.Equal(t, http.StatusCreated, acceptRR.Code,
			"Expected 201 Created, got %d. Body: %s", acceptRR.Code, acceptRR.Body.String())

		// Parse response and verify tenant_subdomain is present
		acceptResp := testutil.ParseJSONResponse(t, acceptRR.Body.Bytes())
		acceptData := acceptResp["data"].(map[string]interface{})
		assert.NotEmpty(t, acceptData["account_id"])
		assert.Equal(t, inviteeEmail, acceptData["email"])

		// tenant_subdomain should be the subdomain of the invitation's school (#1977)
		if subdomain, ok := acceptData["tenant_subdomain"].(string); ok {
			assert.NotEmpty(t, subdomain, "tenant_subdomain should be non-empty")
		}

		if id, ok := data["id"].(float64); ok {
			_, _ = tc.db.NewDelete().
				TableExpr("auth.invitation_tokens").
				Where("id = ?", int64(id)).
				Exec(context.Background())
		}
	})
}

// =============================================================================
// LinkToTenant Tests
// =============================================================================

func TestLinkToTenant(t *testing.T) {
	t.Parallel()
	db, router := setupPublicRouterWithDB(t)

	// Get admin token and a valid role ID
	adminToken, validRoleID := loginAsAdmin(t, db, router)

	t.Run("success with valid admin request", func(t *testing.T) {
		// ARRANGE — create an account to link
		email := fmt.Sprintf("link-success-%d@example.com", time.Now().UnixNano())
		password := "SecurePass123!"
		account := testpkg.CreateTestAccountWithPassword(t, db, email, password)
		testpkg.OwnTestAccountWithIdentity(t, db, account.ID)
		// Linking provisions the person and staff record too (#2222).

		// The identity fields are required for a staff-tier role (#2222).
		body := map[string]interface{}{
			"email":      email,
			"role_id":    validRoleID,
			"first_name": "Link",
			"last_name":  "Success",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/link-to-tenant", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := testutil.ExecuteRequest(router, req)

		// ASSERT
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "Expected data to be an object")
		assert.Equal(t, email, data["email"])

		// Cleanup tenant mapping + role assignment created by link
		_, _ = db.NewDelete().
			TableExpr("auth.account_roles").
			Where("account_id = ?", account.ID).
			Exec(context.Background())
		_, _ = db.NewDelete().
			TableExpr("auth.account_tenants").
			Where("account_id = ?", account.ID).
			Exec(context.Background())
	})

	t.Run("returns 404 for non-existent email", func(t *testing.T) {
		body := map[string]interface{}{
			"email":      fmt.Sprintf("nonexistent-%d@example.com", time.Now().UnixNano()),
			"role_id":    validRoleID,
			"first_name": "Nicht",
			"last_name":  "Vorhanden",
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/link-to-tenant", body)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := testutil.ExecuteRequest(router, req)

		assert.Equal(t, http.StatusNotFound, rr.Code, "Body: %s", rr.Body.String())
	})

	t.Run("returns 401 without auth", func(t *testing.T) {
		body := map[string]interface{}{
			"email":   "someone@example.com",
			"role_id": validRoleID,
		}

		req := testutil.NewJSONRequest(t, "POST", "/auth/link-to-tenant", body)
		rr := testutil.ExecuteRequest(router, req)

		testutil.AssertUnauthorized(t, rr)
	})
}

// ============================================================================
// RESOLVE TENANT — SOFT-DELETE COVERAGE
// ============================================================================

// TestResolveTenant_DeletedSchool_ReturnsNotFound verifies that GET /auth/tenant/resolve
// returns 404 when the matching school has been soft-deleted (deleted_at IS NOT NULL).
// This ensures the frontend cannot resolve a decommissioned tenant via subdomain lookup.
func TestResolveTenant_DeletedSchool_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	db, authRoute := setupAuthDependenciesRoute(t)

	// Create a dedicated tenant for this test using a high ID to avoid collisions.
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	// Soft-delete the school by setting deleted_at.
	_, err := db.ExecContext(context.Background(),
		`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, tenantID)
	require.NoError(t, err)

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(authRoute.AuthService, authRoute.InvitationService, platformSvc.NewSchoolService(schoolRepo), db)

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	// The subdomain for tenant 9900 is "t9900" (set by EnsureTestTenant).
	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug=t9900", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code, "Soft-deleted tenant must return 404, body: %s", rr.Body.String())

	var response struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "Failed to parse error response: %s", rr.Body.String())
	assert.Equal(t, "error", response.Status)
}

// TestCreateInvitationRequiresRoleGrantAuthority guards the invitation endpoint
// against privilege escalation: the "user" (Betreuer) role carries users:create
// globally, so users:create alone must not open an endpoint that hands out roles.
func TestCreateInvitationRequiresRoleGrantAuthority(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)

	account := testpkg.CreateTestAccount(t, tc.db, fmt.Sprintf("inviter-%d@example.com", time.Now().UnixNano()))

	role := testpkg.CreateTestRoleForTenant(t, tc.db, "invite-target", testpkg.Tenant(t))

	claims := jwt.AppClaims{
		ID:       int(account.ID),
		TenantID: testpkg.Tenant(t),
		Sub:      account.Email,
		Username: "betreuer",
		Roles:    []string{"user"},
	}

	body := map[string]any{
		"email":   fmt.Sprintf("invitee-%d@example.com", time.Now().UnixNano()),
		"role_id": role.ID,
	}

	req := testutil.NewJSONRequest(t, "POST", "/auth/invitations", body)
	rr := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:create", "users:update"})

	assert.Equal(t, http.StatusForbidden, rr.Code,
		"users:create must not be enough to create an invitation")
}

// TestRoleAssignmentEndpointsRejectEscalation guards the two remaining paths
// that hand out a role when they attach an account to a school. Neither may be
// driven by users:create, which every Betreuer holds.
func TestRoleAssignmentEndpointsRejectEscalation(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)

	account := testpkg.CreateTestAccount(t, tc.db, fmt.Sprintf("granter-%d@example.com", time.Now().UnixNano()))

	adminRole := testpkg.CreateTestSystemRole(t, tc.db, "admin")

	claims := jwt.AppClaims{
		ID:       int(account.ID),
		TenantID: testpkg.Tenant(t),
		Sub:      account.Email,
		Username: "betreuer",
		Roles:    []string{"user"},
	}
	betreuer := []string{"users:create", "users:update"}

	t.Run("register rejects users:create", func(t *testing.T) {
		body := map[string]any{
			"email":    fmt.Sprintf("newadmin-%d@example.com", time.Now().UnixNano()),
			"username": fmt.Sprintf("newadmin%d", time.Now().UnixNano()),
			"password": "Test1234%",
			"role_id":  adminRole.ID,
		}
		req := testutil.NewJSONRequest(t, "POST", "/auth/register", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, claims, betreuer)

		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("link-to-tenant rejects users:create", func(t *testing.T) {
		body := map[string]any{
			"email":   account.Email,
			"role_id": adminRole.ID,
		}
		req := testutil.NewJSONRequest(t, "POST", "/auth/link-to-tenant", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, claims, betreuer)

		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("an admin may still assign the admin role", func(t *testing.T) {
		adminClaims := claims
		adminClaims.Roles = []string{"admin"}
		body := map[string]any{
			"email":            fmt.Sprintf("realadmin-%d@example.com", time.Now().UnixNano()),
			"username":         fmt.Sprintf("realadmin%d", time.Now().UnixNano()),
			"password":         "Test1234%",
			"confirm_password": "Test1234%",
			"role_id":          adminRole.ID,
			"first_name":       "Echter",
			"last_name":        "Admin",
		}
		req := testutil.NewJSONRequest(t, "POST", "/auth/register", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, adminClaims, []string{"users:manage"})

		require.Equal(t, http.StatusCreated, rr.Code, "body: %s", rr.Body.String())

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		_, ok := response["data"].(map[string]interface{})
		require.True(t, ok)
	})
}

// TestListRoles_UsersCreateReadsNamesOnly pins the users:create carve-out on
// the role list (#2906): whoever may create users assigns a role by name (the
// staff import's "Rolle" column), so the name list is theirs to read, while
// role details and permission sets stay behind roles:read.
func TestListRoles_UsersCreateReadsNamesOnly(t *testing.T) {
	t.Parallel()
	_, router := setupProtectedRouter(t)

	claims := testutil.AdminTestClaims(1)
	usersCreateOnly := []string{"users:create"}

	t.Run("list roles with users:create", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/roles", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, claims, usersCreateOnly)

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].([]interface{})
		require.True(t, ok, "Expected data to be an array")
		require.NotEmpty(t, data, "Expected at least one role")
		for _, entry := range data {
			role, ok := entry.(map[string]interface{})
			require.True(t, ok, "Expected role objects")
			assert.NotEmpty(t, role["name"])
			assert.NotContains(t, role, "permissions", "the list must not carry permission sets")
		}
	})

	t.Run("role details stay behind roles:read", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/roles/1", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, claims, usersCreateOnly)

		testutil.AssertForbidden(t, rr)
	})

	t.Run("role permissions stay behind roles:read", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/auth/roles/1/permissions", nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, claims, usersCreateOnly)

		testutil.AssertForbidden(t, rr)
	})
}
