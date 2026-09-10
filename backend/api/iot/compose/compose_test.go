package compose

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// delegateHandler Tests
// =============================================================================

func TestDelegateHandler_ForwardsRequest(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	deps := ServiceDependencies{
		Administration:  nil,
		Rooms:           nil,
		Directory:       nil,
		FeedbackService: nil,
	}

	resource := NewResource(deps)

	require.NotNil(t, resource)
	assert.Nil(t, resource.Administration)
	assert.Nil(t, resource.Rooms)
	assert.Nil(t, resource.Directory)
	assert.Nil(t, resource.FeedbackService)
}

// =============================================================================
// ServiceDependencies Tests
// =============================================================================

func TestServiceDependencies_Struct(t *testing.T) {
	t.Parallel()

	// Verify struct fields exist
	deps := ServiceDependencies{}

	assert.Nil(t, deps.Administration)
	assert.Nil(t, deps.Rooms)
	assert.Nil(t, deps.Directory)
	assert.Nil(t, deps.FeedbackService)
}

// =============================================================================
// Resource Struct Tests
// =============================================================================

func TestResource_Struct(t *testing.T) {
	t.Parallel()

	// Verify Resource struct can be instantiated
	resource := &Resource{}

	assert.Nil(t, resource.Administration)
	assert.Nil(t, resource.Rooms)
	assert.Nil(t, resource.Directory)
	assert.Nil(t, resource.FeedbackService)
}

// =============================================================================
// Router Tests
// =============================================================================

type routerFeedback struct{}

func (routerFeedback) Available(context.Context) (bool, error) { return true, nil }
func (routerFeedback) Submit(context.Context, feedbackModule.CreateEntry) (feedbackModule.Entry, error) {
	return feedbackModule.Entry{}, nil
}

// routerStaffClock satisfies the kiosk staff clock contract during route
// composition; the routes are never exercised here.
type routerStaffClock struct{}

func (routerStaffClock) StaffClockState(context.Context, string) (*devicescan.StaffClockState, error) {
	return &devicescan.StaffClockState{}, nil
}

func (routerStaffClock) ExecuteStaffClock(context.Context, devicescan.StaffClockCommand) (*devicescan.StaffClockState, error) {
	return &devicescan.StaffClockState{}, nil
}

// routerDeviceScan satisfies the kiosk scan contract during route
// composition; every call answers as an unauthenticated device.
type routerDeviceScan struct{}

func (routerDeviceScan) LockFeedbackStudent(context.Context, int64) (devicescan.FeedbackStudent, error) {
	return devicescan.FeedbackStudent{}, devicescan.ErrDeviceUnauthorized
}

func (routerDeviceScan) Device(context.Context) (devicescan.Device, error) {
	return devicescan.Device{}, devicescan.ErrDeviceUnauthorized
}
func (routerDeviceScan) Scan(context.Context, devicescan.ScanCommand) (*devicescan.ScanResult, error) {
	return nil, devicescan.ErrDeviceUnauthorized
}
func (routerDeviceScan) PickupInfo(context.Context, string) (*devicescan.PickupInfo, error) {
	return nil, devicescan.ErrDeviceUnauthorized
}
func (routerDeviceScan) Ping(context.Context) (*devicescan.DevicePing, error) {
	return nil, devicescan.ErrDeviceUnauthorized
}
func (routerDeviceScan) Status(context.Context) (*devicescan.DeviceStatus, error) {
	return nil, devicescan.ErrDeviceUnauthorized
}
func (routerDeviceScan) AttendanceStatus(context.Context, string) (*devicescan.AttendanceStatus, error) {
	return nil, devicescan.ErrDeviceUnauthorized
}
func (routerDeviceScan) ToggleAttendance(context.Context, devicescan.AttendanceToggleCommand) (*devicescan.AttendanceToggleResult, error) {
	return nil, devicescan.ErrDeviceUnauthorized
}

func newRouterTestResource() *Resource {
	return &Resource{ServiceDependencies: ServiceDependencies{
		DeviceScan:               routerDeviceScan{},
		StaffClock:               routerStaffClock{},
		FeedbackStudents:         routerDeviceScan{},
		FeedbackService:          routerFeedback{},
		FeedbackResponseObserver: func(int, string) {},
	}}
}

func TestResourceRouterFailsFastWithoutFeedbackGraph(t *testing.T) {
	t.Parallel()
	assert.PanicsWithValue(t, "IoT feedback: all dependencies are required", func() {
		(&Resource{}).Router()
	})
}

func TestResource_Router_ReturnsRouter(t *testing.T) {
	t.Parallel()

	// Wire only the dependencies required during route composition.
	resource := newRouterTestResource()

	router := resource.Router()

	require.NotNil(t, router)
}

func TestResource_Router_HasRoutes(t *testing.T) {
	t.Parallel()

	// Wire only the dependencies required during route composition.
	resource := newRouterTestResource()

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
	t.Parallel()

	resource := newRouterTestResource()
	router := resource.Router()

	req := httptest.NewRequest("POST", "/checkin", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Route exists (not 404)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestResource_Router_PingRoute(t *testing.T) {
	t.Parallel()

	resource := newRouterTestResource()
	router := resource.Router()

	req := httptest.NewRequest("POST", "/ping", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Route exists (not 404)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestResource_Router_AttendanceRoute(t *testing.T) {
	t.Parallel()

	resource := newRouterTestResource()
	router := resource.Router()

	req := httptest.NewRequest("GET", "/attendance/daily", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Route exists (not 404)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestResource_Router_SessionRoute(t *testing.T) {
	t.Parallel()

	resource := newRouterTestResource()
	router := resource.Router()

	req := httptest.NewRequest("POST", "/session/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Route exists (not 404)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestResource_Router_FeedbackRoute(t *testing.T) {
	t.Parallel()

	resource := newRouterTestResource()
	router := resource.Router()

	req := httptest.NewRequest("POST", "/feedback", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Route exists (not 404)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestResource_Router_DataRoutes(t *testing.T) {
	t.Parallel()

	resource := newRouterTestResource()
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
	t.Parallel()

	resource := newRouterTestResource()
	router := resource.Router()

	req := httptest.NewRequest("POST", "/staff/rfid/assign", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Route exists (not 404)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestResource_Router_SchoolNameRoute(t *testing.T) {
	t.Parallel()

	resource := newRouterTestResource()
	router := resource.Router()

	req := httptest.NewRequest("GET", "/school-name", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Route exists (not 404)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}
