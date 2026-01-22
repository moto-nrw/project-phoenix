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
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/tenant"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
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
	serviceFactory, err := services.NewFactory(repoFactory, db)
	require.NoError(t, err, "Failed to create service factory")

	return db, serviceFactory
}

// RequestOption configures an HTTP request for testing.
type RequestOption func(*http.Request)

// WithPermissions adds permissions to the request context.
// Uses tenant context key (tenant.CtxPermissions) for compatibility with
// the authorize middleware which reads permissions from tenant context.
func WithPermissions(permissions ...string) RequestOption {
	return func(req *http.Request) {
		// Set permissions in tenant context (used by authorize middleware)
		ctx := context.WithValue(req.Context(), tenant.CtxPermissions, permissions)
		// Also set in JWT context for backwards compatibility
		ctx = context.WithValue(ctx, jwt.CtxPermissions, permissions)
		*req = *req.WithContext(ctx)
	}
}

// WithClaims adds JWT claims to the request context.
func WithClaims(claims jwt.AppClaims) RequestOption {
	return func(req *http.Request) {
		ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
		*req = *req.WithContext(ctx)
	}
}

// WithDeviceContext adds an IoT device to the request context.
// This is used for testing device-authenticated endpoints.
func WithDeviceContext(d *iot.Device) RequestOption {
	return func(req *http.Request) {
		ctx := context.WithValue(req.Context(), device.CtxDevice, d)
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

// ExecuteRequest executes an HTTP request against a Chi router and returns the response recorder.
func ExecuteRequest(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
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
		Roles:       []string{"teacher"},
		Permissions: []string{"students:read", "groups:read", "visits:read", "visits:create"},
		IsTeacher:   true,
	}
}

// AdminTestClaims returns JWT claims for an admin user.
func AdminTestClaims(accountID int) jwt.AppClaims {
	return jwt.AppClaims{
		ID:          accountID,
		Sub:         "admin@example.com",
		Username:    "admin",
		FirstName:   "Admin",
		LastName:    "User",
		Roles:       []string{"admin"},
		Permissions: []string{"admin:*"},
		IsAdmin:     true,
	}
}

// =============================================================================
// TENANT CONTEXT HELPERS (for BetterAuth-based authentication)
// =============================================================================

// WithTenantContext adds a TenantContext to the request context.
// This replaces WithClaims for tests using the new tenant middleware.
func WithTenantContext(tc *tenant.TenantContext) RequestOption {
	return func(req *http.Request) {
		ctx := tenant.SetTenantContext(req.Context(), tc)
		*req = *req.WithContext(ctx)
	}
}

// SupervisorTenantContext returns a tenant context for a supervisor (front-line staff).
// Supervisors can see location data but have limited admin capabilities.
func SupervisorTenantContext(email string) *tenant.TenantContext {
	return &tenant.TenantContext{
		UserID:      "supervisor-user-id",
		UserEmail:   email,
		UserName:    "Test Supervisor",
		OrgID:       "test-org-id",
		OrgName:     "Test OGS",
		OrgSlug:     "test-ogs",
		Role:        "supervisor",
		Permissions: []string{"student:read", "group:read", "room:read", "visit:read", "visit:create", "visit:update", "activity:read", "location:read", "attendance:read", "attendance:checkin", "attendance:checkout", "attendance:update"},
		TraegerID:   "test-traeger-id",
		TraegerName: "Test Träger",
	}
}

// OGSAdminTenantContext returns a tenant context for an OGS admin.
// OGS admins can manage their facility but not other organizations.
func OGSAdminTenantContext(email string) *tenant.TenantContext {
	return &tenant.TenantContext{
		UserID:      "ogsadmin-user-id",
		UserEmail:   email,
		UserName:    "Test OGS Admin",
		OrgID:       "test-org-id",
		OrgName:     "Test OGS",
		OrgSlug:     "test-ogs",
		Role:        "ogsAdmin",
		Permissions: []string{"student:read", "student:create", "student:update", "student:delete", "group:read", "group:create", "group:update", "group:delete", "staff:read", "staff:create", "staff:update", "staff:delete", "staff:invite", "room:read", "room:create", "room:update", "room:delete", "visit:read", "visit:create", "visit:update", "visit:delete", "activity:read", "activity:create", "activity:update", "activity:delete", "schedule:read", "schedule:create", "schedule:update", "schedule:delete", "feedback:read", "feedback:create", "feedback:update", "feedback:delete", "config:read", "config:update", "import:read", "import:create", "guardian:read", "guardian:create", "guardian:update", "guardian:delete", "location:read", "ogs:update", "attendance:read", "attendance:checkin", "attendance:checkout", "attendance:update", "attendance:delete"},
		TraegerID:   "test-traeger-id",
		TraegerName: "Test Träger",
	}
}
