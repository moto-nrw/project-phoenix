package testutil_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/internal/ptrtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Request Option Tests
// =============================================================================

func TestWithPermissions(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/test", nil)

	// Apply permissions
	opt := testutil.WithPermissions("users:read", "users:write")
	opt(req)

	// Verify permissions are in context
	perms := testpkg.PermissionsFromContext(req.Context())
	require.NotNil(t, perms)
	assert.Contains(t, perms, "users:read")
	assert.Contains(t, perms, "users:write")
}

func TestWithPermissions_Empty(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/test", nil)

	// Apply empty permissions
	opt := testutil.WithPermissions()
	opt(req)

	// When called with no args, permissions should be empty slice or nil
	// Either is valid for authorization checks
	perms := testpkg.PermissionsFromContext(req.Context())
	assert.Empty(t, perms)
	// If nil, that's also acceptable - means no permissions
}

func TestWithClaims(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/test", nil)
	tenantID := ptrtest.NewTenantID()

	claims := testpkg.Claims{
		ID:       42,
		TenantID: tenantID,
		Username: "testuser",
		IsAdmin:  true,
	}

	opt := testutil.WithClaims(t, claims)
	opt(req)

	// Verify claims are in context
	appClaims := testpkg.ClaimsFromContext(req.Context())
	assert.Equal(t, 42, appClaims.ID)
	assert.Equal(t, tenantID, appClaims.TenantID)
	assert.Equal(t, "testuser", appClaims.Username)
	assert.True(t, appClaims.IsAdmin)
}

func TestWithDeviceContext(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/test", nil)

	device := &testpkg.Device{
		DeviceID:   "device-123",
		DeviceType: "rfid-reader",
		Status:     testpkg.DeviceStatusActive,
	}

	opt := testutil.WithDeviceContext(device)
	opt(req)

	// Verify device is in context (just check it doesn't panic)
	// The device context key is not exported, so we just verify the option works
	assert.NotNil(t, req.Context())
}

func TestWithStaffContext(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/test", nil)

	opt := testutil.WithStaffContext(123, ptrtest.NewTenantID())
	opt(req)

	// Verify staff is in context (just check it doesn't panic)
	// The staff context key is not exported, so we just verify the option works
	assert.NotNil(t, req.Context())
}

// =============================================================================
// Request Builder Tests
// =============================================================================

func TestNewRequest(t *testing.T) {
	t.Parallel()

	body := bytes.NewBufferString(`{"test": "data"}`)
	req := testutil.NewRequest("POST", "/api/test", body)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "/api/test", req.URL.Path)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestNewRequest_WithOptions(t *testing.T) {
	t.Parallel()

	tenantID := ptrtest.NewTenantID()
	req := testutil.NewRequest("GET", "/api/test", nil,
		testutil.WithPermissions("test:read"),
		testutil.WithClaims(t, testpkg.Claims{ID: 1, TenantID: tenantID}),
	)

	assert.Equal(t, "GET", req.Method)

	// Verify options were applied
	perms := testpkg.PermissionsFromContext(req.Context())
	require.NotNil(t, perms)
}

func TestNewAuthenticatedRequest_WithBody(t *testing.T) {
	t.Parallel()

	body := map[string]interface{}{
		"name": "test",
		"id":   123,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/api/test", body)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "/api/test", req.URL.Path)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.NotNil(t, req.Body)
}

func TestNewAuthenticatedRequest_NilBody(t *testing.T) {
	t.Parallel()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/api/test", nil)

	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, "/api/test", req.URL.Path)
}

func TestNewAuthenticatedRequest_WithOptions(t *testing.T) {
	t.Parallel()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/api/test", nil,
		testutil.WithPermissions("admin:*"),
		testutil.WithClaims(t, testutil.DefaultTestClaims()),
	)

	perms := testpkg.PermissionsFromContext(req.Context())
	require.NotNil(t, perms)
	assert.Contains(t, perms, "admin:*")
}

func TestNewJSONRequest_WithBody(t *testing.T) {
	t.Parallel()

	body := map[string]string{"key": "value"}
	req := testutil.NewJSONRequest(t, "PUT", "/api/resource", body)

	assert.Equal(t, "PUT", req.Method)
	assert.Equal(t, "/api/resource", req.URL.Path)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestNewJSONRequest_NilBody(t *testing.T) {
	t.Parallel()

	req := testutil.NewJSONRequest(t, "DELETE", "/api/resource", nil)

	assert.Equal(t, "DELETE", req.Method)
	assert.Equal(t, "/api/resource", req.URL.Path)
}

func TestNewMultipartRequest(t *testing.T) {
	t.Parallel()

	req := testutil.NewMultipartRequest(t, "POST", "/api/upload",
		"file", "test.csv", "name,value\nfoo,bar")

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "/api/upload", req.URL.Path)

	// Verify content type is multipart/form-data
	contentType := req.Header.Get("Content-Type")
	assert.Contains(t, contentType, "multipart/form-data")
	assert.Contains(t, contentType, "boundary=")

	// Verify body is not empty
	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	assert.NotEmpty(t, body)
	assert.Contains(t, string(body), "test.csv")
	assert.Contains(t, string(body), "name,value")
}

func TestNewMultipartRequest_EmptyContent(t *testing.T) {
	t.Parallel()

	req := testutil.NewMultipartRequest(t, "POST", "/api/upload",
		"file", "empty.txt", "")

	assert.Equal(t, "POST", req.Method)

	// Verify content type is still multipart/form-data
	contentType := req.Header.Get("Content-Type")
	assert.Contains(t, contentType, "multipart/form-data")
}

func TestNewMultipartRequest_WithOptions(t *testing.T) {
	t.Parallel()

	tenantID := ptrtest.NewTenantID()
	req := testutil.NewMultipartRequest(t, "POST", "/api/upload",
		"document", "data.json", `{"key": "value"}`,
		testutil.WithPermissions("uploads:create"),
		testutil.WithClaims(t, testpkg.Claims{ID: 42, TenantID: tenantID}),
	)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "/api/upload", req.URL.Path)

	// Verify options were applied
	perms := testpkg.PermissionsFromContext(req.Context())
	require.NotNil(t, perms)
	assert.Contains(t, perms, "uploads:create")

	// Verify claims were applied
	appClaims := testpkg.ClaimsFromContext(req.Context())
	assert.Equal(t, 42, appClaims.ID)
	assert.Equal(t, tenantID, appClaims.TenantID)
}

// =============================================================================
// ExecuteRequest Tests
// =============================================================================

func TestExecuteRequest(t *testing.T) {
	t.Parallel()

	router := chi.NewRouter()
	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "success")
}

// =============================================================================
// Response Parser Tests
// =============================================================================

func TestParseResponse(t *testing.T) {
	t.Parallel()

	body := []byte(`{"status":"success","data":{"id":1},"message":"OK"}`)

	response := testutil.ParseResponse(t, body)

	assert.Equal(t, "success", response.Status)
	assert.Equal(t, "OK", response.Message)
	assert.NotNil(t, response.Data)
}

func TestParseJSONResponse(t *testing.T) {
	t.Parallel()

	body := []byte(`{"status":"success","data":{"id":1,"name":"test"}}`)

	response := testutil.ParseJSONResponse(t, body)

	assert.Equal(t, "success", response["status"])
	data := response["data"].(map[string]interface{})
	assert.Equal(t, float64(1), data["id"])
	assert.Equal(t, "test", data["name"])
}

// =============================================================================
// Assertion Helper Tests
// =============================================================================

func TestAssertSuccessResponse(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	rr.WriteHeader(http.StatusOK)
	_, _ = rr.WriteString(`{"status":"success","data":{}}`)

	// Should not panic or fail
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestAssertSuccessResponse_NoContent(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	rr.WriteHeader(http.StatusNoContent)

	// Should handle 204 No Content without parsing body
	testutil.AssertSuccessResponse(t, rr, http.StatusNoContent)
}

func TestAssertErrorResponse(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	rr.WriteHeader(http.StatusBadRequest)
	_, _ = rr.WriteString(`{"status":400,"detail":"bad request","code":"general.input"}`)

	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

func TestAssertUnauthorized(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	rr.WriteHeader(http.StatusUnauthorized)
	_, _ = rr.WriteString(`{"status":401,"detail":"refused","code":"general.permission"}`)

	testutil.AssertUnauthorized(t, rr)
}

func TestAssertForbidden(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	rr.WriteHeader(http.StatusForbidden)
	_, _ = rr.WriteString(`{"status":403,"detail":"refused","code":"general.permission"}`)

	testutil.AssertForbidden(t, rr)
}

func TestAssertNotFound(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	rr.WriteHeader(http.StatusNotFound)
	_, _ = rr.WriteString(`{"status":404,"detail":"refused","code":"general.input"}`)

	testutil.AssertNotFound(t, rr)
}

func TestAssertBadRequest(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	rr.WriteHeader(http.StatusBadRequest)
	_, _ = rr.WriteString(`{"status":400,"detail":"refused","code":"general.input"}`)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// Claims Helper Tests
// =============================================================================

func TestDefaultTestClaims(t *testing.T) {
	t.Parallel()

	claims := testutil.DefaultTestClaims()

	assert.Equal(t, 1, claims.ID)
	assert.Equal(t, "test@example.com", claims.Sub)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, "Test", claims.FirstName)
	assert.Equal(t, "User", claims.LastName)
	assert.Contains(t, claims.Roles, "admin")
	assert.Contains(t, claims.Permissions, "admin:*")
	assert.True(t, claims.IsAdmin)
}

func TestTeacherTestClaims(t *testing.T) {
	t.Parallel()

	claims := testutil.TeacherTestClaims(42)

	assert.Equal(t, 42, claims.ID)
	assert.Equal(t, "teacher@example.com", claims.Sub)
	assert.Equal(t, "teacher", claims.Username)
	assert.Contains(t, claims.Roles, "user")
	assert.Contains(t, claims.Permissions, "students:read")
	assert.Contains(t, claims.Permissions, "groups:read")
	assert.False(t, claims.IsAdmin)
}

func TestAdminTestClaims(t *testing.T) {
	t.Parallel()

	claims := testutil.AdminTestClaims(99)

	assert.Equal(t, 99, claims.ID)
	assert.Equal(t, "admin@example.com", claims.Sub)
	assert.Equal(t, "admin", claims.Username)
	assert.Contains(t, claims.Roles, "admin")
	assert.Contains(t, claims.Permissions, "admin:*")
	assert.True(t, claims.IsAdmin)
}

// TestClaimsFollowTheTestIntoItsOwnTenant pins the mechanism that lets ~950
// claims-helper call sites stay untouched by the per-test-tenant migration
// (#2419): claims carrying the bootstrap tenant are rebased onto the tenant
// the test owns, while claims naming a tenant explicitly are left alone.
func TestClaimsFollowTheTestIntoItsOwnTenant(t *testing.T) {
	t.Parallel()

	own := testpkg.Tenant(t)

	// DefaultTestClaims carries the bootstrap tenant, so this also pins that
	// the shipped helpers keep working untouched.
	req := httptest.NewRequest("GET", "/test", nil)
	testutil.WithClaims(t, testutil.DefaultTestClaims())(req)
	assert.Equal(t, own, testpkg.ClaimsFromContext(req.Context()).TenantID,
		"bootstrap-tenant claims must follow the test")

	otherTenantID := ptrtest.NewTenantID()
	require.NotEqual(t, own, otherTenantID)
	other := httptest.NewRequest("GET", "/test", nil)
	testutil.WithClaims(t, testpkg.Claims{ID: 42, TenantID: otherTenantID})(other)
	assert.Equal(t, otherTenantID, testpkg.ClaimsFromContext(other.Context()).TenantID,
		"an explicitly named tenant must survive untouched")
}
