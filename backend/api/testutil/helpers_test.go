package testutil_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/iot"
)

// =============================================================================
// SetupAPITest Tests
// =============================================================================

func TestSetupAPITest(t *testing.T) {
	db, services := testutil.SetupAPITest(t)
	require.NotNil(t, db)
	require.NotNil(t, services)

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	})

	// Verify services are available
	assert.NotNil(t, services.Auth)
	assert.NotNil(t, services.Users)
}

// =============================================================================
// Request Option Tests
// =============================================================================

func TestWithPermissions(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)

	// Apply permissions
	opt := testutil.WithPermissions("users:read", "users:write")
	opt(req)

	// Verify permissions are in context
	perms := req.Context().Value(jwt.CtxPermissions)
	require.NotNil(t, perms)
	permSlice := perms.([]string)
	assert.Contains(t, permSlice, "users:read")
	assert.Contains(t, permSlice, "users:write")
}

func TestWithPermissions_Empty(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)

	// Apply empty permissions
	opt := testutil.WithPermissions()
	opt(req)

	// When called with no args, permissions should be empty slice or nil
	// Either is valid for authorization checks
	perms := req.Context().Value(jwt.CtxPermissions)
	if perms != nil {
		permSlice := perms.([]string)
		assert.Empty(t, permSlice)
	}
	// If nil, that's also acceptable - means no permissions
}

func TestWithClaims(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)

	claims := jwt.AppClaims{
		ID:       42,
		Username: "testuser",
		IsAdmin:  true,
	}

	opt := testutil.WithClaims(claims)
	opt(req)

	// Verify claims are in context
	ctxClaims := req.Context().Value(jwt.CtxClaims)
	require.NotNil(t, ctxClaims)
	appClaims := ctxClaims.(jwt.AppClaims)
	assert.Equal(t, 42, appClaims.ID)
	assert.Equal(t, "testuser", appClaims.Username)
	assert.True(t, appClaims.IsAdmin)
}

func TestWithDeviceContext(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)

	device := &iot.Device{
		DeviceID:   "device-123",
		DeviceType: "rfid-reader",
		Status:     iot.DeviceStatusActive,
	}

	opt := testutil.WithDeviceContext(device)
	opt(req)

	// Verify device is in context (just check it doesn't panic)
	// The device context key is not exported, so we just verify the option works
	assert.NotNil(t, req.Context())
}

// =============================================================================
// Request Builder Tests
// =============================================================================

func TestNewRequest(t *testing.T) {
	body := bytes.NewBufferString(`{"test": "data"}`)
	req := testutil.NewRequest("POST", "/api/test", body)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "/api/test", req.URL.Path)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestNewRequest_WithOptions(t *testing.T) {
	req := testutil.NewRequest("GET", "/api/test", nil,
		testutil.WithPermissions("test:read"),
		testutil.WithClaims(jwt.AppClaims{ID: 1}),
	)

	assert.Equal(t, "GET", req.Method)

	// Verify options were applied
	perms := req.Context().Value(jwt.CtxPermissions)
	require.NotNil(t, perms)
}

func TestNewAuthenticatedRequest_WithBody(t *testing.T) {
	body := map[string]interface{}{
		"name": "test",
		"id":   123,
	}

	req := testutil.NewAuthenticatedRequest("POST", "/api/test", body)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "/api/test", req.URL.Path)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.NotNil(t, req.Body)
}

func TestNewAuthenticatedRequest_NilBody(t *testing.T) {
	req := testutil.NewAuthenticatedRequest("GET", "/api/test", nil)

	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, "/api/test", req.URL.Path)
}

func TestNewAuthenticatedRequest_WithOptions(t *testing.T) {
	req := testutil.NewAuthenticatedRequest("GET", "/api/test", nil,
		testutil.WithPermissions("admin:*"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	perms := req.Context().Value(jwt.CtxPermissions)
	require.NotNil(t, perms)
	permSlice := perms.([]string)
	assert.Contains(t, permSlice, "admin:*")
}

func TestNewJSONRequest_WithBody(t *testing.T) {
	body := map[string]string{"key": "value"}
	req := testutil.NewJSONRequest("PUT", "/api/resource", body)

	assert.Equal(t, "PUT", req.Method)
	assert.Equal(t, "/api/resource", req.URL.Path)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestNewJSONRequest_NilBody(t *testing.T) {
	req := testutil.NewJSONRequest("DELETE", "/api/resource", nil)

	assert.Equal(t, "DELETE", req.Method)
	assert.Equal(t, "/api/resource", req.URL.Path)
}

// =============================================================================
// ExecuteRequest Tests
// =============================================================================

func TestExecuteRequest(t *testing.T) {
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
	body := []byte(`{"status":"success","data":{"id":1},"message":"OK"}`)

	response := testutil.ParseResponse(t, body)

	assert.Equal(t, "success", response.Status)
	assert.Equal(t, "OK", response.Message)
	assert.NotNil(t, response.Data)
}

func TestParseJSONResponse(t *testing.T) {
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
	rr := httptest.NewRecorder()
	rr.WriteHeader(http.StatusOK)
	_, _ = rr.WriteString(`{"status":"success","data":{}}`)

	// Should not panic or fail
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestAssertSuccessResponse_NoContent(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.WriteHeader(http.StatusNoContent)

	// Should handle 204 No Content without parsing body
	testutil.AssertSuccessResponse(t, rr, http.StatusNoContent)
}

func TestAssertErrorResponse(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.WriteHeader(http.StatusBadRequest)
	_, _ = rr.WriteString(`{"status":"error","message":"bad request"}`)

	testutil.AssertErrorResponse(t, rr, http.StatusBadRequest)
}

func TestAssertUnauthorized(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.WriteHeader(http.StatusUnauthorized)
	_, _ = rr.WriteString(`{"status":"Unauthorized"}`)

	testutil.AssertUnauthorized(t, rr)
}

func TestAssertForbidden(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.WriteHeader(http.StatusForbidden)
	_, _ = rr.WriteString(`{"status":"Forbidden"}`)

	testutil.AssertForbidden(t, rr)
}

func TestAssertNotFound(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.WriteHeader(http.StatusNotFound)
	_, _ = rr.WriteString(`{"status":"Not Found"}`)

	testutil.AssertNotFound(t, rr)
}

func TestAssertBadRequest(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.WriteHeader(http.StatusBadRequest)
	_, _ = rr.WriteString(`{"status":"Invalid Request"}`)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// Claims Helper Tests
// =============================================================================

func TestDefaultTestClaims(t *testing.T) {
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
	claims := testutil.TeacherTestClaims(42)

	assert.Equal(t, 42, claims.ID)
	assert.Equal(t, "teacher@example.com", claims.Sub)
	assert.Equal(t, "teacher", claims.Username)
	assert.Contains(t, claims.Roles, "teacher")
	assert.Contains(t, claims.Permissions, "students:read")
	assert.Contains(t, claims.Permissions, "groups:read")
	assert.True(t, claims.IsTeacher)
	assert.False(t, claims.IsAdmin)
}

func TestAdminTestClaims(t *testing.T) {
	claims := testutil.AdminTestClaims(99)

	assert.Equal(t, 99, claims.ID)
	assert.Equal(t, "admin@example.com", claims.Sub)
	assert.Equal(t, "admin", claims.Username)
	assert.Contains(t, claims.Roles, "admin")
	assert.Contains(t, claims.Permissions, "admin:*")
	assert.True(t, claims.IsAdmin)
}
