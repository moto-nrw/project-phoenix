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
// Router Tests
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
