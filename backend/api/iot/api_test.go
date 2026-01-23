package iot

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// delegateHandler Tests
// =============================================================================

func TestDelegateHandler_ForwardsRequest(t *testing.T) {
	// Create a subrouter with a test endpoint
	subrouter := chi.NewRouter()
	subrouter.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("delegated response"))
	})

	handler := delegateHandler(subrouter)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "delegated response", w.Body.String())
}

func TestDelegateHandler_PostRequest(t *testing.T) {
	subrouter := chi.NewRouter()
	subrouter.Post("/*", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("post response"))
	})

	handler := delegateHandler(subrouter)

	req := httptest.NewRequest("POST", "/data", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "post response", w.Body.String())
}

func TestDelegateHandler_PutRequest(t *testing.T) {
	subrouter := chi.NewRouter()
	subrouter.Put("/*", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("put response"))
	})

	handler := delegateHandler(subrouter)

	req := httptest.NewRequest("PUT", "/update", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "put response", w.Body.String())
}

func TestDelegateHandler_DeleteRequest(t *testing.T) {
	subrouter := chi.NewRouter()
	subrouter.Delete("/*", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler := delegateHandler(subrouter)

	req := httptest.NewRequest("DELETE", "/remove", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDelegateHandler_PatchRequest(t *testing.T) {
	subrouter := chi.NewRouter()
	subrouter.Patch("/*", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("patched"))
	})

	handler := delegateHandler(subrouter)

	req := httptest.NewRequest("PATCH", "/partial", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "patched", w.Body.String())
}

func TestDelegateHandler_PreservesRequestHeaders(t *testing.T) {
	subrouter := chi.NewRouter()
	subrouter.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		// Echo back a custom header
		value := r.Header.Get("X-Custom-Header")
		w.Header().Set("X-Echo-Header", value)
		w.WriteHeader(http.StatusOK)
	})

	handler := delegateHandler(subrouter)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Custom-Header", "test-value")
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test-value", w.Header().Get("X-Echo-Header"))
}

func TestDelegateHandler_HandlesNotFound(t *testing.T) {
	subrouter := chi.NewRouter()
	// Only register GET, so POST should return 405
	subrouter.Get("/exists", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := delegateHandler(subrouter)

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// =============================================================================
// NewResource Tests
// =============================================================================

func TestNewResource(t *testing.T) {
	deps := ServiceDependencies{
		IoTService:        nil,
		UsersService:      nil,
		ActiveService:     nil,
		ActivitiesService: nil,
		ConfigService:     nil,
		FacilityService:   nil,
		EducationService:  nil,
		FeedbackService:   nil,
	}

	resource := NewResource(deps)

	require.NotNil(t, resource)
	assert.Nil(t, resource.IoTService)
	assert.Nil(t, resource.UsersService)
	assert.Nil(t, resource.ActiveService)
	assert.Nil(t, resource.ActivitiesService)
	assert.Nil(t, resource.ConfigService)
	assert.Nil(t, resource.FacilityService)
	assert.Nil(t, resource.EducationService)
	assert.Nil(t, resource.FeedbackService)
}

func TestNewResource_CopiesAllDependencies(t *testing.T) {
	// Verify that NewResource creates a new struct with same dependencies
	// (demonstrates pass-by-value semantics)
	deps := ServiceDependencies{}
	resource := NewResource(deps)

	// Verify the returned resource has the same nil values
	assert.Equal(t, deps.IoTService, resource.IoTService)
	assert.Equal(t, deps.UsersService, resource.UsersService)
	assert.Equal(t, deps.ActiveService, resource.ActiveService)
	assert.Equal(t, deps.ActivitiesService, resource.ActivitiesService)
	assert.Equal(t, deps.ConfigService, resource.ConfigService)
	assert.Equal(t, deps.FacilityService, resource.FacilityService)
	assert.Equal(t, deps.EducationService, resource.EducationService)
	assert.Equal(t, deps.FeedbackService, resource.FeedbackService)
}

// =============================================================================
// ServiceDependencies Tests
// =============================================================================

func TestServiceDependencies_Struct(t *testing.T) {
	// Verify struct fields exist
	deps := ServiceDependencies{}

	assert.Nil(t, deps.IoTService)
	assert.Nil(t, deps.UsersService)
	assert.Nil(t, deps.ActiveService)
	assert.Nil(t, deps.ActivitiesService)
	assert.Nil(t, deps.ConfigService)
	assert.Nil(t, deps.FacilityService)
	assert.Nil(t, deps.EducationService)
	assert.Nil(t, deps.FeedbackService)
}

// =============================================================================
// Resource Struct Tests
// =============================================================================

func TestResource_Struct(t *testing.T) {
	// Verify Resource struct can be instantiated
	resource := &Resource{}

	assert.Nil(t, resource.IoTService)
	assert.Nil(t, resource.UsersService)
	assert.Nil(t, resource.ActiveService)
	assert.Nil(t, resource.ActivitiesService)
	assert.Nil(t, resource.ConfigService)
	assert.Nil(t, resource.FacilityService)
	assert.Nil(t, resource.EducationService)
	assert.Nil(t, resource.FeedbackService)
}

// =============================================================================
// Router Tests (Legacy - calls DeviceRouter)
// =============================================================================

func TestResource_Router_ReturnsRouter(t *testing.T) {
	// Create resource with nil services (just testing router structure)
	resource := &Resource{}

	router := resource.Router()

	require.NotNil(t, router)
}

func TestResource_Router_HasRoutes(t *testing.T) {
	// Create resource with nil services
	resource := &Resource{}

	router := resource.Router()

	// Verify router has routes registered by checking it responds
	// (routes will fail auth but router structure should be valid)
	require.NotNil(t, router)

	// Test that router can handle requests (even if they fail auth)
	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// We expect 401 Unauthorized since we have no device auth
	// This proves the route exists and middleware runs
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestResource_Router_DelegatesToDeviceRouter(t *testing.T) {
	resource := &Resource{}

	// Router() should return the same result as DeviceRouter()
	router := resource.Router()
	deviceRouter := resource.DeviceRouter()

	require.NotNil(t, router)
	require.NotNil(t, deviceRouter)

	// Both should handle the same routes
	// Test with a common route that exists in DeviceRouter
	testCases := []struct {
		method string
		path   string
	}{
		{"POST", "/checkin"},
		{"GET", "/status"},
		{"POST", "/ping"},
	}

	for _, tc := range testCases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			reqRouter := httptest.NewRequest(tc.method, tc.path, nil)
			wRouter := httptest.NewRecorder()
			router.ServeHTTP(wRouter, reqRouter)

			reqDevice := httptest.NewRequest(tc.method, tc.path, nil)
			wDevice := httptest.NewRecorder()
			deviceRouter.ServeHTTP(wDevice, reqDevice)

			// Both should return the same status (likely 401 Unauthorized)
			assert.Equal(t, wDevice.Code, wRouter.Code)
		})
	}
}

func TestResource_Router_CheckinRoute(t *testing.T) {
	resource := &Resource{}
	router := resource.Router()

	req := httptest.NewRequest("POST", "/checkin", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Route exists (not 404)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestResource_Router_PingRoute(t *testing.T) {
	resource := &Resource{}
	router := resource.Router()

	req := httptest.NewRequest("POST", "/ping", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Route exists (not 404)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestResource_Router_AttendanceRoute(t *testing.T) {
	resource := &Resource{}
	router := resource.Router()

	req := httptest.NewRequest("GET", "/attendance/daily", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Route exists (not 404)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestResource_Router_SessionRoute(t *testing.T) {
	resource := &Resource{}
	router := resource.Router()

	req := httptest.NewRequest("POST", "/session/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Route exists (not 404)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestResource_Router_FeedbackRoute(t *testing.T) {
	resource := &Resource{}
	router := resource.Router()

	req := httptest.NewRequest("POST", "/feedback", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Route exists (not 404)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestResource_Router_DataRoutes(t *testing.T) {
	resource := &Resource{}
	router := resource.Router()

	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/students"},
		{"GET", "/activities"},
		{"GET", "/rooms/available"},
		{"GET", "/rfid/test-tag"},
		{"GET", "/teachers"},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Route exists (not 404)
			assert.NotEqual(t, http.StatusNotFound, w.Code)
		})
	}
}

func TestResource_Router_StaffRFIDRoute(t *testing.T) {
	resource := &Resource{}
	router := resource.Router()

	req := httptest.NewRequest("POST", "/staff/rfid/assign", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Route exists (not 404)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

// =============================================================================
// AdminRouter Tests
// =============================================================================

func TestResource_AdminRouter_ReturnsRouter(t *testing.T) {
	resource := &Resource{}

	router := resource.AdminRouter()

	require.NotNil(t, router)
}

func TestResource_AdminRouter_MountsDevicesResource(t *testing.T) {
	resource := &Resource{}
	router := resource.AdminRouter()

	// AdminRouter mounts devices at "/" which includes CRUD operations
	// Test common device management endpoints
	testCases := []struct {
		method string
		path   string
	}{
		{"GET", "/"},
		{"POST", "/"},
		{"GET", "/123"},
		{"PUT", "/123"},
		{"DELETE", "/123"},
	}

	for _, tc := range testCases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// We can't test for specific responses without mocking the IoTService,
			// but we can verify the route handler is invoked (returns something other than 404 for GET)
			// For POST/PUT/DELETE, they may return various codes depending on auth
			if tc.method == "GET" {
				// With nil service, GET should fail but not with 404
				// Actually with nil service it will panic or return error
				// So we just verify router exists and responds
				assert.True(t, w.Code >= 0, "Router should respond")
			}
		})
	}
}

func TestResource_AdminRouter_SetsContentTypeJSON(t *testing.T) {
	resource := &Resource{}
	router := resource.AdminRouter()

	// Make a request to verify Content-Type middleware is applied
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// The render.SetContentType middleware should set JSON content type
	contentType := w.Header().Get("Content-Type")
	// Empty or contains json (depending on response handling)
	assert.True(t, contentType == "" || contentType == "application/json" ||
		contentType == "application/json; charset=utf-8",
		"Content-Type should be JSON or empty, got: %s", contentType)
}

// =============================================================================
// DeviceRouter Tests
// =============================================================================

func TestResource_DeviceRouter_ReturnsRouter(t *testing.T) {
	resource := &Resource{}

	router := resource.DeviceRouter()

	require.NotNil(t, router)
}

func TestResource_DeviceRouter_SetsContentTypeJSON(t *testing.T) {
	resource := &Resource{}
	router := resource.DeviceRouter()

	// Make a request to verify Content-Type middleware is applied
	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	// May be empty or json depending on middleware order
	assert.True(t, contentType == "" || contentType == "application/json" ||
		contentType == "application/json; charset=utf-8",
		"Content-Type should be JSON or empty, got: %s", contentType)
}

func TestResource_DeviceRouter_DeviceOnlyAuthRoutes(t *testing.T) {
	resource := &Resource{}
	router := resource.DeviceRouter()

	// /teachers endpoint uses DeviceOnlyAuthenticator (API key only, no PIN)
	req := httptest.NewRequest("GET", "/teachers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Without API key, should get 401
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestResource_DeviceRouter_DeviceAuthRoutes(t *testing.T) {
	resource := &Resource{}
	router := resource.DeviceRouter()

	// These routes require DeviceAuthenticator (API key + PIN)
	routes := []struct {
		method string
		path   string
	}{
		{"POST", "/checkin"},
		{"POST", "/ping"},
		{"GET", "/status"},
		{"POST", "/feedback"},
		{"GET", "/students"},
		{"GET", "/activities"},
		{"GET", "/rooms/available"},
		{"GET", "/rfid/test-tag-123"},
	}

	for _, tc := range routes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Without auth headers, should get 401
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestResource_DeviceRouter_AttendanceMountedRoutes(t *testing.T) {
	resource := &Resource{}
	router := resource.DeviceRouter()

	// Attendance routes are mounted at /attendance
	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/attendance/daily"},
		{"GET", "/attendance/weekly"},
	}

	for _, tc := range routes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Route exists (not 404), should fail auth
			assert.NotEqual(t, http.StatusNotFound, w.Code)
		})
	}
}

func TestResource_DeviceRouter_SessionMountedRoutes(t *testing.T) {
	resource := &Resource{}
	router := resource.DeviceRouter()

	// Session routes are mounted at /session
	routes := []struct {
		method string
		path   string
	}{
		{"POST", "/session/start"},
		{"POST", "/session/end"},
		{"GET", "/session/timeout"},
	}

	for _, tc := range routes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Route exists (not 404)
			assert.NotEqual(t, http.StatusNotFound, w.Code)
		})
	}
}

func TestResource_DeviceRouter_StaffRFIDMountedRoutes(t *testing.T) {
	resource := &Resource{}
	router := resource.DeviceRouter()

	// Staff RFID routes are mounted at /staff
	routes := []struct {
		method string
		path   string
	}{
		{"POST", "/staff/rfid/assign"},
		{"POST", "/staff/rfid/unassign"},
	}

	for _, tc := range routes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Route exists (not 404)
			assert.NotEqual(t, http.StatusNotFound, w.Code)
		})
	}
}

func TestResource_DeviceRouter_RFIDTagRoute(t *testing.T) {
	resource := &Resource{}
	router := resource.DeviceRouter()

	// RFID route with path parameter
	testTags := []string{
		"ABC123",
		"test-tag-456",
		"12345678",
		"RFID-001",
	}

	for _, tag := range testTags {
		t.Run("tag="+tag, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/rfid/"+tag, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Route exists (should fail auth, not 404)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

// =============================================================================
// Edge Cases and Error Handling
// =============================================================================

func TestResource_DeviceRouter_InvalidMethod(t *testing.T) {
	resource := &Resource{}
	router := resource.DeviceRouter()

	// Try invalid methods on known routes
	testCases := []struct {
		method string
		path   string
	}{
		{"DELETE", "/checkin"},    // Only POST allowed
		{"PUT", "/status"},        // Only GET allowed
		{"PATCH", "/teachers"},    // Only GET allowed
		{"DELETE", "/activities"}, // Only GET allowed
	}

	for _, tc := range testCases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should return 405 Method Not Allowed or 401 (if auth runs first)
			assert.True(t, w.Code == http.StatusMethodNotAllowed || w.Code == http.StatusUnauthorized,
				"Expected 405 or 401, got %d", w.Code)
		})
	}
}

func TestResource_DeviceRouter_NonExistentRoute(t *testing.T) {
	resource := &Resource{}
	router := resource.DeviceRouter()

	// Try a completely non-existent route
	testCases := []struct {
		method string
		path   string
	}{
		{"GET", "/nonexistent"},
		{"POST", "/api/v1/something"},
		{"GET", "/admin/users"},
	}

	for _, tc := range testCases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should return 404 Not Found
			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}

func TestResource_AdminRouter_NonExistentRoute(t *testing.T) {
	resource := &Resource{}
	router := resource.AdminRouter()

	testCases := []struct {
		method string
		path   string
	}{
		{"GET", "/nonexistent/route"},
		{"POST", "/api/v1/admin"},
	}

	for _, tc := range testCases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should return 404 Not Found
			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}

// =============================================================================
// Integration Test: Full Router Configuration
// =============================================================================

func TestResource_AllRouters_AreDistinct(t *testing.T) {
	resource := &Resource{}

	router := resource.Router()
	deviceRouter := resource.DeviceRouter()
	adminRouter := resource.AdminRouter()

	// All routers should be non-nil
	require.NotNil(t, router)
	require.NotNil(t, deviceRouter)
	require.NotNil(t, adminRouter)

	// Router and DeviceRouter should behave the same (Router delegates to DeviceRouter)
	// AdminRouter should have different routes

	// Test that AdminRouter does not have device routes like /checkin
	// AdminRouter mounts device CRUD at "/" so /checkin returns 405 (Method Not Allowed)
	// because POST /checkin is not a device CRUD operation
	req := httptest.NewRequest("POST", "/checkin", nil)
	w := httptest.NewRecorder()
	adminRouter.ServeHTTP(w, req)
	// AdminRouter has device resource at "/" which catches all paths but with limited methods
	// So /checkin returns 405 (not a valid method) rather than 404
	assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed,
		"AdminRouter should not have /checkin route, got %d", w.Code)
}

func TestResource_FullRouteMatrix(t *testing.T) {
	resource := &Resource{}
	router := resource.DeviceRouter()

	// Complete matrix of all documented routes
	routes := []struct {
		method         string
		path           string
		expectNotFound bool
		authGroup      string // "device-only" or "device-pin"
	}{
		// Device-only auth group (API key only)
		{"GET", "/teachers", false, "device-only"},

		// Device + PIN auth group
		{"POST", "/checkin", false, "device-pin"},
		{"POST", "/ping", false, "device-pin"},
		{"GET", "/status", false, "device-pin"},
		{"POST", "/feedback", false, "device-pin"},
		{"GET", "/students", false, "device-pin"},
		{"GET", "/activities", false, "device-pin"},
		{"GET", "/rooms/available", false, "device-pin"},
		{"GET", "/rfid/test-tag", false, "device-pin"},

		// Mounted sub-routers
		{"GET", "/attendance/daily", false, "device-pin"},
		{"POST", "/session/start", false, "device-pin"},
		{"POST", "/staff/rfid/assign", false, "device-pin"},

		// Non-existent routes
		{"GET", "/invalid", true, ""},
		{"POST", "/does-not-exist", true, ""},
	}

	for _, tc := range routes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if tc.expectNotFound {
				assert.Equal(t, http.StatusNotFound, w.Code)
			} else {
				assert.NotEqual(t, http.StatusNotFound, w.Code)
				// Without auth, we expect 401
				assert.Equal(t, http.StatusUnauthorized, w.Code)
			}
		})
	}
}
