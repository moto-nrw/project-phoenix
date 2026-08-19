package iot

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/auth/device"
	iotModel "github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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
		FacilityService:   nil,
		EducationService:  nil,
	}

	resource := NewResource(deps)

	require.NotNil(t, resource)
	assert.Nil(t, resource.IoTService)
	assert.Nil(t, resource.UsersService)
	assert.Nil(t, resource.ActiveService)
	assert.Nil(t, resource.ActivitiesService)
	assert.Nil(t, resource.FacilityService)
	assert.Nil(t, resource.EducationService)
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
	assert.Nil(t, deps.FacilityService)
	assert.Nil(t, deps.EducationService)
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
	assert.Nil(t, resource.FacilityService)
	assert.Nil(t, resource.EducationService)
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

func TestResource_Router_SchoolNameRoute(t *testing.T) {
	resource := &Resource{}
	router := resource.Router()

	req := httptest.NewRequest("GET", "/school-name", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Route exists (not 404)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

// =============================================================================
// getSchoolName Handler Tests
// =============================================================================

// withDeviceCtx injects a device model into the request context.
func withDeviceCtx(req *http.Request, d *iotModel.Device) *http.Request {
	ctx := context.WithValue(req.Context(), device.CtxDevice, d)
	return req.WithContext(ctx)
}

func TestGetSchoolName_Success(t *testing.T) {
	repo := &testpkg.SchoolRepoMock{
		FindByIDFn: func(_ context.Context, id int64) (*platform.School, error) {
			assert.Equal(t, int64(42), id)
			return &platform.School{Name: "OGS Musterstadt"}, nil
		},
	}
	rs := &Resource{ServiceDependencies: ServiceDependencies{SchoolService: platformSvc.NewSchoolService(repo)}}

	req := httptest.NewRequest("GET", "/school-name", nil)
	dev := &iotModel.Device{}
	dev.TenantID = 42
	req = withDeviceCtx(req, dev)

	w := httptest.NewRecorder()
	rs.getSchoolName(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "success", body["status"])
	assert.Equal(t, "School name retrieved", body["message"])

	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "OGS Musterstadt", data["name"])
}

func TestGetSchoolName_NoDeviceContext(t *testing.T) {
	rs := &Resource{}

	req := httptest.NewRequest("GET", "/school-name", nil)
	w := httptest.NewRecorder()
	rs.getSchoolName(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "error", body["status"])
	assert.Equal(t, device.ErrMissingAPIKey.Error(), body["error"])
}

func TestGetSchoolName_SchoolNotFound(t *testing.T) {
	repo := &testpkg.SchoolRepoMock{
		FindByIDFn: func(_ context.Context, _ int64) (*platform.School, error) {
			return nil, errors.New("sql: no rows in result set")
		},
	}
	rs := &Resource{ServiceDependencies: ServiceDependencies{SchoolService: platformSvc.NewSchoolService(repo)}}

	req := httptest.NewRequest("GET", "/school-name", nil)
	dev := &iotModel.Device{}
	dev.TenantID = 999
	req = withDeviceCtx(req, dev)

	w := httptest.NewRecorder()
	rs.getSchoolName(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "error", body["status"])
}

func TestGetSchoolName_DatabaseError(t *testing.T) {
	repo := &testpkg.SchoolRepoMock{
		FindByIDFn: func(_ context.Context, _ int64) (*platform.School, error) {
			return nil, errors.New("connection refused")
		},
	}
	rs := &Resource{ServiceDependencies: ServiceDependencies{SchoolService: platformSvc.NewSchoolService(repo)}}

	req := httptest.NewRequest("GET", "/school-name", nil)
	dev := &iotModel.Device{}
	dev.TenantID = 10
	req = withDeviceCtx(req, dev)

	w := httptest.NewRecorder()
	rs.getSchoolName(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetSchoolName_EmptySchoolName(t *testing.T) {
	repo := &testpkg.SchoolRepoMock{
		FindByIDFn: func(_ context.Context, _ int64) (*platform.School, error) {
			return &platform.School{Name: ""}, nil
		},
	}
	rs := &Resource{ServiceDependencies: ServiceDependencies{SchoolService: platformSvc.NewSchoolService(repo)}}

	req := httptest.NewRequest("GET", "/school-name", nil)
	dev := &iotModel.Device{}
	dev.TenantID = 10
	req = withDeviceCtx(req, dev)

	w := httptest.NewRecorder()
	rs.getSchoolName(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)

	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "", data["name"])
}

func TestGetSchoolName_UsesTenantIDFromDevice(t *testing.T) {
	var receivedID int64
	repo := &testpkg.SchoolRepoMock{
		FindByIDFn: func(_ context.Context, id int64) (*platform.School, error) {
			receivedID = id
			return &platform.School{Name: "Test School"}, nil
		},
	}
	rs := &Resource{ServiceDependencies: ServiceDependencies{SchoolService: platformSvc.NewSchoolService(repo)}}

	req := httptest.NewRequest("GET", "/school-name", nil)
	dev := &iotModel.Device{}
	dev.TenantID = 777
	req = withDeviceCtx(req, dev)

	w := httptest.NewRecorder()
	rs.getSchoolName(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(777), receivedID)
}

func TestGetSchoolName_ResponseStructure(t *testing.T) {
	repo := &testpkg.SchoolRepoMock{
		FindByIDFn: func(_ context.Context, _ int64) (*platform.School, error) {
			return &platform.School{Name: "Grundschule am Park"}, nil
		},
	}
	rs := &Resource{ServiceDependencies: ServiceDependencies{SchoolService: platformSvc.NewSchoolService(repo)}}

	req := httptest.NewRequest("GET", "/school-name", nil)
	dev := &iotModel.Device{}
	dev.TenantID = 10
	req = withDeviceCtx(req, dev)

	w := httptest.NewRecorder()
	rs.getSchoolName(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)

	// Verify only expected top-level keys
	assert.Contains(t, body, "status")
	assert.Contains(t, body, "data")
	assert.Contains(t, body, "message")

	// Verify data only contains "name"
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Len(t, data, 1)
	assert.Contains(t, data, "name")
	assert.Equal(t, "Grundschule am Park", data["name"])
}

func TestSchoolNameResponse_JSONSerialization(t *testing.T) {
	resp := schoolNameResponse{Name: "Test Schule"}
	b, err := json.Marshal(resp)
	require.NoError(t, err)

	var parsed map[string]string
	err = json.Unmarshal(b, &parsed)
	require.NoError(t, err)
	assert.Equal(t, "Test Schule", parsed["name"])
	assert.Len(t, parsed, 1)
}
