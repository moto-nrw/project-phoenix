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
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// HTTP header constants (S1192 - avoid duplicate string literals)
const (
	headerContentType   = "Content-Type"
	contentTypeJSON     = "application/json"
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
func WithPermissions(permissions ...string) RequestOption {
	return func(req *http.Request) {
		ctx := context.WithValue(req.Context(), jwt.CtxPermissions, permissions)
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
