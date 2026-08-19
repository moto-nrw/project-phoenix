// Package testutil provides shared test utilities for API handler tests.
//
// # Test Pattern
//
// API tests follow the hermetic test pattern established in the codebase:
// - Real database with test fixtures
// - Real services via factory
// - httptest for HTTP request/response
// - Context injection for JWT claims and permissions
//
// Example:
//
//	func TestHandler(t *testing.T) {
//	    db, services := testutil.SetupAPITest(t)
//	    defer db.Close()
//
//	    resource := NewResource(services.Auth, services.Invitation)
//	    router := chi.NewRouter()
//	    router.Mount("/auth", resource.Router())
//
//	    req := testutil.NewAuthenticatedRequest("GET", "/auth/account", nil,
//	        testutil.WithPermissions("users:read"),
//	        testutil.WithClaims(jwt.AppClaims{ID: 1, Username: "test"}),
//	    )
//
//	    rr := testutil.ExecuteRequest(router, req)
//	    testutil.AssertSuccessResponse(t, rr, http.StatusOK)
//	}
package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// HTTP header constants (S1192 - avoid duplicate string literals)
const (
	headerContentType = "Content-Type"
	contentTypeJSON   = "application/json"
)

// SetupAPITest initializes test database and service factory for API tests.
// Returns the database connection and service factory.
// The caller must close the database connection when done.
func SetupAPITest(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()

	// Set JWT config defaults (normally set in cmd/serve.go)
	viper.SetDefault("auth_jwt_secret", "test-secret-for-unit-tests-minimum-32-chars")
	viper.SetDefault("auth_jwt_expiry", "15m")
	viper.SetDefault("auth_jwt_refresh_expiry", "1h")

	db := testpkg.SetupTestDB(t)

	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err, "Failed to create service factory")

	return db, serviceFactory
}

// RequestOption configures an HTTP request for testing.
type RequestOption func(*http.Request)

// WithPermissions adds permissions to the request context.
func WithPermissions(permissions ...string) RequestOption {
	return func(req *http.Request) {
		ctx := context.WithValue(req.Context(), jwt.CtxPermissions, permissions)
		*req = *req.WithContext(ctx)
	}
}

// WithClaims adds JWT claims to the request context.
// Also injects tenant context (mirroring TenantMiddleware) so that
// handler-level WithTenantTx can read the tenant ID.
func WithClaims(claims jwt.AppClaims) RequestOption {
	return func(req *http.Request) {
		ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
		if claims.TenantID != 0 {
			ctx = tenant.WithTenantID(ctx, claims.TenantID)
		}
		*req = *req.WithContext(ctx)
	}
}

// WithJWTBearer sets an Authorization: Bearer <token> header on the request.
// Use together with MintTestJWT when exercising a Resource via Router(), where
// the production JWT middleware chain (Verifier → Authenticator → TenantMiddleware)
// runs and rejects requests that lack a real signed token.
func WithJWTBearer(token string) RequestOption {
	return func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// MintTestJWT signs a JWT for the given claims using the same configuration as
// production (jwt.NewTokenAuth reads the JWT secret from viper / env). Pair it
// with WithJWTBearer when calling handlers through Resource.Router() so the
// production auth middleware accepts the request.
//
// Callers must arrange for a non-empty auth_jwt_secret to be present before the
// Resource is constructed (typically via SeedTestJWTConfig in init() or
// TestMain). Without a secret, jwx refuses to HMAC-sign and this helper fails.
func MintTestJWT(t *testing.T, claims jwt.AppClaims) string {
	t.Helper()
	tokenAuth, err := jwt.NewTokenAuth()
	require.NoError(t, err, "MintTestJWT: NewTokenAuth")
	token, err := tokenAuth.CreateJWT(claims)
	require.NoError(t, err, "MintTestJWT: CreateJWT")
	return token
}

// SeedTestJWTConfig installs deterministic viper defaults for JWT auth so that
// tests work in environments without a populated .env (e.g. CI). Use it from
// init() or TestMain in test packages that build a Resource which calls
// jwt.MustNewTokenAuth() and then use MintTestJWT to sign requests — both
// callers must see the same secret.
//
// SetDefault leaves env-supplied values alone, so this is safe to call when
// AUTH_JWT_SECRET is already set in the environment.
func SeedTestJWTConfig() {
	viper.SetDefault("auth_jwt_secret", "test-secret-for-unit-tests-minimum-32-chars")
	viper.SetDefault("auth_jwt_expiry", "15m")
	viper.SetDefault("auth_jwt_refresh_expiry", "1h")
}

// WithDeviceContext adds an IoT device to the request context.
// This is used for testing device-authenticated endpoints.
// Also injects the device's tenant_id so TenantTxMiddleware can create
// a tenant-scoped transaction (mirrors production device auth middleware).
func WithDeviceContext(d *iot.Device) RequestOption {
	return func(req *http.Request) {
		ctx := context.WithValue(req.Context(), device.CtxDevice, d)
		if tid := d.GetTenantID(); tid != 0 {
			ctx = tenant.WithTenantID(ctx, tid)
		}
		*req = *req.WithContext(ctx)
	}
}

// WithStaffContext adds a staff member to the request context.
// This is used for testing endpoints that require staff authentication.
func WithStaffContext(s *users.Staff) RequestOption {
	return func(req *http.Request) {
		ctx := context.WithValue(req.Context(), device.CtxStaff, s)
		*req = *req.WithContext(ctx)
	}
}

// NewRequest creates a new HTTP request for testing.
func NewRequest(method, target string, body io.Reader, opts ...RequestOption) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Header.Set(headerContentType, contentTypeJSON)

	for _, opt := range opts {
		opt(req)
	}

	return req
}

// NewAuthenticatedRequest creates a request with authentication context.
// This is a convenience function that combines common options.
func NewAuthenticatedRequest(t *testing.T, method, target string, body interface{}, opts ...RequestOption) *http.Request {
	t.Helper()

	var reader io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal JSON body: %v", err)
		}
		reader = bytes.NewBuffer(jsonBytes)
	}

	req := httptest.NewRequest(method, target, reader)
	req.Header.Set(headerContentType, contentTypeJSON)

	for _, opt := range opts {
		opt(req)
	}

	return req
}

// NewJSONRequest creates a request with JSON body.
func NewJSONRequest(t *testing.T, method, target string, body interface{}) *http.Request {
	t.Helper()

	var reader io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal JSON body: %v", err)
		}
		reader = bytes.NewBuffer(jsonBytes)
	}

	req := httptest.NewRequest(method, target, reader)
	req.Header.Set(headerContentType, contentTypeJSON)

	return req
}

// NewMultipartRequest creates a multipart form request with file upload.
func NewMultipartRequest(t *testing.T, method, target string, fieldName, fileName, content string, opts ...RequestOption) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Create form file field
	fw, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}

	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write file content: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(method, target, &buf)
	req.Header.Set(headerContentType, writer.FormDataContentType())

	for _, opt := range opts {
		opt(req)
	}

	return req
}

// NewTenantRouter creates a chi.Router pre-configured with TenantTxMiddleware.
// Use this in integration tests instead of chi.NewRouter() to match production
// middleware behavior (RLS enforcement via SET LOCAL ROLE + set_config).
//
// NOTE: Production routers apply TenantTxMiddleware per-route (via .With(withTx))
// so that permission checks reject unauthorized requests before a DB transaction
// is opened. Tests keep group-level r.Use() for simplicity since test helpers
// control their own request context and don't have the same connection-waste concern.
func NewTenantRouter(db *bun.DB) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Use(tenant.TenantTxMiddleware(db))
	return router
}

// TenantContext returns a context with tenant_id set.
// Use this when calling service methods directly in test setup (not through HTTP handlers).
func TenantContext(tenantID int64) context.Context {
	return tenant.WithTenantID(context.Background(), tenantID)
}

// ExecuteRequest executes an HTTP request against a Chi router and returns the response recorder.
func ExecuteRequest(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// ExecuteWithAuth signs a JWT for the given claims (used as-is, including any
// permissions they already carry) and executes the request through the router.
func ExecuteWithAuth(t *testing.T, router chi.Router, req *http.Request, claims jwt.AppClaims) *httptest.ResponseRecorder {
	t.Helper()
	req.Header.Set("Authorization", "Bearer "+MintTestJWT(t, claims))
	return ExecuteRequest(router, req)
}

// ExecuteWithAuthPermissions folds the given permission set into the claims
// (replacing whatever they carried — an empty slice deliberately produces a
// permissionless token), signs a JWT, and executes the request.
func ExecuteWithAuthPermissions(t *testing.T, router chi.Router, req *http.Request, claims jwt.AppClaims, permissions []string) *httptest.ResponseRecorder {
	t.Helper()
	claims.Permissions = permissions
	req.Header.Set("Authorization", "Bearer "+MintTestJWT(t, claims))
	return ExecuteRequest(router, req)
}

// Response represents a standard API response for testing.
type Response struct {
	Status  string      `json:"status"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ParseResponse parses the response body into a Response struct.
func ParseResponse(t *testing.T, body []byte) Response {
	t.Helper()

	var response Response
	err := json.Unmarshal(body, &response)
	require.NoError(t, err, "Failed to parse response body: %s", string(body))
	return response
}

// ParseJSONResponse parses the response body into a map.
func ParseJSONResponse(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()

	var response map[string]interface{}
	err := json.Unmarshal(body, &response)
	require.NoError(t, err, "Failed to parse response body: %s", string(body))
	return response
}

// AssertSuccessResponse validates that the response has success status and expected HTTP code.
func AssertSuccessResponse(t *testing.T, rr *httptest.ResponseRecorder, expectedStatus int) {
	t.Helper()

	assert.Equal(t, expectedStatus, rr.Code, "Unexpected HTTP status code. Body: %s", rr.Body.String())

	if rr.Code == http.StatusNoContent {
		return // No body to parse
	}

	response := ParseResponse(t, rr.Body.Bytes())
	assert.Equal(t, "success", response.Status, "Expected success status. Body: %s", rr.Body.String())
}

// AssertErrorResponse validates that the response has error status and expected HTTP code.
// Note: Some handlers return {"status":"Invalid Request"} or {"status":"Not Found"} etc.
// instead of {"status":"error"}, so we only check the HTTP status code.
func AssertErrorResponse(t *testing.T, rr *httptest.ResponseRecorder, expectedStatus int) {
	t.Helper()

	assert.Equal(t, expectedStatus, rr.Code, "Unexpected HTTP status code. Body: %s", rr.Body.String())
}

// AssertUnauthorized validates a 401 Unauthorized response.
func AssertUnauthorized(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	AssertErrorResponse(t, rr, http.StatusUnauthorized)
}

// AssertForbidden validates a 403 Forbidden response.
// Note: The authorize middleware returns {"status":"Forbidden"} not {"status":"error"},
// so we only check the HTTP status code here, not the response body format.
func AssertForbidden(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, http.StatusForbidden, rr.Code, "Expected 403 Forbidden. Body: %s", rr.Body.String())
}

// AssertNotFound validates a 404 Not Found response.
func AssertNotFound(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	AssertErrorResponse(t, rr, http.StatusNotFound)
}

// AssertBadRequest validates a 400 Bad Request response.
func AssertBadRequest(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	AssertErrorResponse(t, rr, http.StatusBadRequest)
}

// DefaultTestClaims returns default JWT claims for testing.
func DefaultTestClaims() jwt.AppClaims {
	return jwt.AppClaims{
		ID:          1,
		Sub:         "test@example.com",
		Username:    "testuser",
		FirstName:   "Test",
		LastName:    "User",
		Roles:       []string{"admin"},
		Permissions: []string{"admin:*"},
		IsAdmin:     true,
		TenantID:    1,
	}
}

// TeacherTestClaims returns JWT claims for a teacher user.
func TeacherTestClaims(accountID int) jwt.AppClaims {
	return jwt.AppClaims{
		ID:          accountID,
		Sub:         "teacher@example.com",
		Username:    "teacher",
		FirstName:   "Test",
		LastName:    "Teacher",
		Roles:       []string{"user"},
		Permissions: []string{"students:read", "groups:read", "groups:update", "groups:list", "visits:read", "visits:create", "visits:update", "visits:delete", "visits:list", "activities:update", "activities:delete", "activities:list", "activities:manage", "activities:enroll", "activities:assign", "users:list", "rooms:list", "schedules:read", "schedules:list", "substitutions:read"},
		TenantID:    1,
	}
}

// AdminTestClaims returns JWT claims for an admin user.
func AdminTestClaims(accountID int) jwt.AppClaims {
	return AdminTestClaimsForTenant(accountID, 1)
}

// AdminTestClaimsForTenant returns admin JWT claims scoped to a specific tenant.
// Use this for tests that run in an isolated tenant (e.g., to avoid cross-package
// fixture interference) instead of the default test tenant id=1.
func AdminTestClaimsForTenant(accountID int, tenantID int64) jwt.AppClaims {
	return jwt.AppClaims{
		ID:          accountID,
		Sub:         "admin@example.com",
		Username:    "admin",
		FirstName:   "Admin",
		LastName:    "User",
		Roles:       []string{"admin"},
		Permissions: []string{"admin:*"},
		IsAdmin:     true,
		TenantID:    tenantID,
	}
}
