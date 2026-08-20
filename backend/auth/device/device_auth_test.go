package device

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/models/users"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock IoT Service - implements iot.Service interface
// =============================================================================

type mockIoTService struct {
	mu             sync.Mutex
	devices        map[string]*iot.Device
	updateCalled   bool
	updateError    error
	updateCount    int
	lastSeenWrites []time.Time
	updateStarted  chan struct{}
	updateBlock    chan struct{}
}

func newMockIoTService() *mockIoTService {
	return &mockIoTService{
		devices: make(map[string]*iot.Device),
	}
}

func (m *mockIoTService) addDevice(apiKey string, device *iot.Device) {
	m.devices[apiKey] = device
}

// The only methods actually used by DeviceAuthenticator
func (m *mockIoTService) GetDeviceByAPIKey(_ context.Context, apiKey string) (*iot.Device, error) {
	device, ok := m.devices[apiKey]
	if !ok {
		return nil, errors.New("device not found")
	}
	return device, nil
}

func (m *mockIoTService) UpdateDevice(_ context.Context, _ *iot.Device) error {
	m.updateCalled = true
	return m.updateError
}

// Required interface methods (not used in device auth tests)
func (m *mockIoTService) CreateDevice(_ context.Context, _ *iot.Device) error {
	return nil
}
func (m *mockIoTService) GetDeviceByID(_ context.Context, _ int64) (*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) GetDeviceByDeviceID(_ context.Context, _ string) (*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) DeleteDevice(_ context.Context, _ int64) error { return nil }
func (m *mockIoTService) ListDevices(_ context.Context, _ map[string]interface{}) ([]*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) UpdateDeviceStatus(_ context.Context, _ string, _ iot.DeviceStatus) error {
	return nil
}
func (m *mockIoTService) PingDevice(_ context.Context, _ string) error { return nil }
func (m *mockIoTService) GetDevicesByType(_ context.Context, _ string) ([]*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) GetDevicesByStatus(_ context.Context, _ iot.DeviceStatus) ([]*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) GetDevicesByRegisteredBy(_ context.Context, _ int64) ([]*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) GetActiveDevices(_ context.Context) ([]*iot.Device, error) { return nil, nil }
func (m *mockIoTService) GetDevicesRequiringMaintenance(_ context.Context) ([]*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) GetOfflineDevices(_ context.Context, _ time.Duration) ([]*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) GetDeviceTypeStatistics(_ context.Context) (map[string]int, error) {
	return nil, nil
}
func (m *mockIoTService) DeviceOnlineWindow(_ context.Context) time.Duration   { return 5 * time.Minute }
func (m *mockIoTService) IsDeviceOnline(_ context.Context, _ *iot.Device) bool { return false }
func (m *mockIoTService) IsDeviceOnlineAt(_ context.Context, _ *iot.Device, _ time.Time) bool {
	return false
}
func (m *mockIoTService) SetSettingsService(_ iotSvc.SettingsResolver)              {}
func (m *mockIoTService) DetectNewDevices(_ context.Context) ([]*iot.Device, error) { return nil, nil }
func (m *mockIoTService) ScanNetwork(_ context.Context) (map[string]string, error)  { return nil, nil }
func (m *mockIoTService) UpdateDeviceLastSeenAt(_ context.Context, _ int64, lastSeen time.Time) error {
	if m.updateStarted != nil {
		select {
		case m.updateStarted <- struct{}{}:
		default:
		}
	}
	if m.updateBlock != nil {
		<-m.updateBlock
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCalled = true
	m.updateCount++
	m.lastSeenWrites = append(m.lastSeenWrites, lastSeen)
	return m.updateError
}

// =============================================================================
// Mock Person Service - not actually used by DeviceAuthenticator
// =============================================================================

// mockPersonService is not needed since DeviceAuthenticator doesn't use it
// The PersonService parameter is unused in the current implementation

// =============================================================================
// Context Helpers Tests
// =============================================================================

func TestDeviceFromCtx_ValidDevice(t *testing.T) {
	t.Parallel()

	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "terminal",
		Status:     iot.DeviceStatusActive,
	}

	ctx := context.WithValue(context.Background(), CtxDevice, device)

	result := DeviceFromCtx(ctx)
	require.NotNil(t, result)
	assert.Equal(t, "device-001", result.DeviceID)
	assert.Equal(t, "terminal", result.DeviceType)
}

func TestDeviceFromCtx_NoDevice(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	result := DeviceFromCtx(ctx)
	assert.Nil(t, result)
}

func TestDeviceFromCtx_WrongType(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), CtxDevice, "not a device")
	result := DeviceFromCtx(ctx)
	assert.Nil(t, result)
}

func TestStaffFromCtx_ValidStaff(t *testing.T) {
	t.Parallel()

	staff := &users.Staff{
		StaffNotes: "Test staff member",
	}

	ctx := context.WithValue(context.Background(), CtxStaff, staff)

	result := StaffFromCtx(ctx)
	require.NotNil(t, result)
	assert.Equal(t, "Test staff member", result.StaffNotes)
}

func TestStaffFromCtx_NoStaff(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	result := StaffFromCtx(ctx)
	assert.Nil(t, result)
}

func TestStaffFromCtx_WrongType(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), CtxStaff, "not a staff")
	result := StaffFromCtx(ctx)
	assert.Nil(t, result)
}

func TestIsIoTDeviceRequest_True(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), CtxIsIoTDevice, true)
	result := IsIoTDeviceRequest(ctx)
	assert.True(t, result)
}

func TestIsIoTDeviceRequest_False(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), CtxIsIoTDevice, false)
	result := IsIoTDeviceRequest(ctx)
	assert.False(t, result)
}

func TestIsIoTDeviceRequest_NotSet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	result := IsIoTDeviceRequest(ctx)
	assert.False(t, result)
}

func TestIsIoTDeviceRequest_WrongType(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), CtxIsIoTDevice, "true")
	result := IsIoTDeviceRequest(ctx)
	assert.False(t, result)
}

// =============================================================================
// SecureCompareStrings Tests
// =============================================================================

func TestSecureCompareStrings_Equal(t *testing.T) {
	t.Parallel()

	assert.True(t, SecureCompareStrings("password", "password"))
	assert.True(t, SecureCompareStrings("", ""))
	assert.True(t, SecureCompareStrings("a very long string with special chars!@#$%", "a very long string with special chars!@#$%"))
}

func TestSecureCompareStrings_NotEqual(t *testing.T) {
	t.Parallel()

	assert.False(t, SecureCompareStrings("password", "different"))
	assert.False(t, SecureCompareStrings("password", "Password")) // Case sensitive
	assert.False(t, SecureCompareStrings("password", "password "))
	assert.False(t, SecureCompareStrings("", "notempty"))
}

func TestSecureCompareStrings_DifferentLengths(t *testing.T) {
	t.Parallel()

	assert.False(t, SecureCompareStrings("short", "muchlongerstring"))
	assert.False(t, SecureCompareStrings("muchlongerstring", "short"))
}

// =============================================================================
// DeviceOnlyAuthenticator Tests
// =============================================================================

// Deliberately NOT parallel: the test resets lastSeenWriteCache, the
// package-level debounce map every device authentication shares.
func TestDeviceOnlyAuthenticator_ValidAPIKey(t *testing.T) {
	lastSeenWriteCache = sync.Map{}

	mockService := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "terminal",
		Status:     iot.DeviceStatusActive,
	}
	mockService.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceOnlyAuthenticator(mockService, nil))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		// Verify device is in context
		ctxDevice := DeviceFromCtx(r.Context())
		require.NotNil(t, ctxDevice)
		assert.Equal(t, "device-001", ctxDevice.DeviceID)

		// Verify IsIoTDevice is NOT set (only set by DeviceAuthenticator)
		assert.False(t, IsIoTDeviceRequest(r.Context()))

		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, mockService.updateCalled, "Should update device last seen")
}

func TestDeviceOnlyAuthenticator_MissingAuthHeader(t *testing.T) {
	t.Parallel()

	mockService := newMockIoTService()

	r := chi.NewRouter()
	r.Use(DeviceOnlyAuthenticator(mockService, nil))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No Authorization header
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestDeviceOnlyAuthenticator_InvalidAuthFormat(t *testing.T) {
	t.Parallel()

	mockService := newMockIoTService()

	testCases := []struct {
		name   string
		header string
	}{
		{"No Bearer prefix", "api-key-123"},
		{"Basic instead of Bearer", "Basic api-key-123"},
		{"Empty Bearer", "Bearer "},
		{"Lowercase bearer", "bearer api-key-123"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Use(DeviceOnlyAuthenticator(mockService, nil))
			r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", tc.header)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	}
}

func TestDeviceOnlyAuthenticator_InvalidAPIKey(t *testing.T) {
	t.Parallel()

	mockService := newMockIoTService()
	// No devices added

	r := chi.NewRouter()
	r.Use(DeviceOnlyAuthenticator(mockService, nil))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-api-key")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestDeviceOnlyAuthenticator_InactiveDevice(t *testing.T) {
	t.Parallel()

	mockService := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "terminal",
		Status:     iot.DeviceStatusInactive, // Not active
	}
	mockService.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceOnlyAuthenticator(mockService, nil))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestDeviceOnlyAuthenticator_OfflineDevice(t *testing.T) {
	t.Parallel()

	mockService := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "terminal",
		Status:     iot.DeviceStatusOffline, // Offline
	}
	mockService.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceOnlyAuthenticator(mockService, nil))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestDeviceOnlyAuthenticator_MaintenanceDevice(t *testing.T) {
	t.Parallel()

	mockService := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "terminal",
		Status:     iot.DeviceStatusMaintenance, // In maintenance
	}
	mockService.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceOnlyAuthenticator(mockService, nil))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

// =============================================================================
// DeviceAuthenticator Tests (API Key + PIN)
// =============================================================================

// Deliberately NOT parallel: mutates process-global configuration.
func TestDeviceAuthenticator_ValidAPIKeyAndPIN(t *testing.T) {
	lastSeenWriteCache = sync.Map{}

	// Set up environment
	ogsPin := "test-device-pin-123"
	require.NoError(t, os.Setenv("OGS_DEVICE_PIN", ogsPin))
	defer func() { _ = os.Unsetenv("OGS_DEVICE_PIN") }()

	mockIoT := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "terminal",
		Status:     iot.DeviceStatusActive,
	}
	mockIoT.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceAuthenticator(mockIoT, nil, nil, nil))
	r.Post("/checkin", func(w http.ResponseWriter, r *http.Request) {
		// Verify device is in context
		ctxDevice := DeviceFromCtx(r.Context())
		require.NotNil(t, ctxDevice)
		assert.Equal(t, "device-001", ctxDevice.DeviceID)

		// Verify IsIoTDevice is set
		assert.True(t, IsIoTDeviceRequest(r.Context()))

		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/checkin", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Staff-PIN", ogsPin)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, mockIoT.updateCalled, "Should update device last seen")
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestDeviceAuthenticator_MissingPIN(t *testing.T) {
	require.NoError(t, os.Setenv("OGS_DEVICE_PIN", "test-pin"))
	defer func() { _ = os.Unsetenv("OGS_DEVICE_PIN") }()

	mockIoT := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "terminal",
		Status:     iot.DeviceStatusActive,
	}
	mockIoT.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceAuthenticator(mockIoT, nil, nil, nil))
	r.Post("/checkin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/checkin", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	// No X-Staff-PIN header
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestDeviceAuthenticator_InvalidPIN(t *testing.T) {
	require.NoError(t, os.Setenv("OGS_DEVICE_PIN", "correct-pin"))
	defer func() { _ = os.Unsetenv("OGS_DEVICE_PIN") }()

	mockIoT := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "terminal",
		Status:     iot.DeviceStatusActive,
	}
	mockIoT.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceAuthenticator(mockIoT, nil, nil, nil))
	r.Post("/checkin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/checkin", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Staff-PIN", "wrong-pin")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestDeviceAuthenticator_MissingOGSPINConfig(t *testing.T) {
	// Ensure OGS_DEVICE_PIN is not set
	_ = os.Unsetenv("OGS_DEVICE_PIN")

	mockIoT := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "terminal",
		Status:     iot.DeviceStatusActive,
	}
	mockIoT.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceAuthenticator(mockIoT, nil, nil, nil))
	r.Post("/checkin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/checkin", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Staff-PIN", "any-pin")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestDeviceAuthenticator_MissingAPIKey(t *testing.T) {
	require.NoError(t, os.Setenv("OGS_DEVICE_PIN", "test-pin"))
	defer func() { _ = os.Unsetenv("OGS_DEVICE_PIN") }()

	mockIoT := newMockIoTService()

	r := chi.NewRouter()
	r.Use(DeviceAuthenticator(mockIoT, nil, nil, nil))
	r.Post("/checkin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/checkin", nil)
	// No Authorization header
	req.Header.Set("X-Staff-PIN", "test-pin")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestDeviceAuthenticator_InvalidAPIKey(t *testing.T) {
	require.NoError(t, os.Setenv("OGS_DEVICE_PIN", "test-pin"))
	defer func() { _ = os.Unsetenv("OGS_DEVICE_PIN") }()

	mockIoT := newMockIoTService()
	// No devices added

	r := chi.NewRouter()
	r.Use(DeviceAuthenticator(mockIoT, nil, nil, nil))
	r.Post("/checkin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/checkin", nil)
	req.Header.Set("Authorization", "Bearer invalid-key")
	req.Header.Set("X-Staff-PIN", "test-pin")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestDeviceAuthenticator_InactiveDevice(t *testing.T) {
	require.NoError(t, os.Setenv("OGS_DEVICE_PIN", "test-pin"))
	defer func() { _ = os.Unsetenv("OGS_DEVICE_PIN") }()

	mockIoT := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "terminal",
		Status:     iot.DeviceStatusInactive,
	}
	mockIoT.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceAuthenticator(mockIoT, nil, nil, nil))
	r.Post("/checkin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/checkin", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Staff-PIN", "test-pin")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

// =============================================================================
// PINResolver Wiring Tests
// =============================================================================

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestDeviceAuthenticator_UsesPINResolver(t *testing.T) {
	lastSeenWriteCache = sync.Map{}

	// Do NOT set OGS_DEVICE_PIN env var — PIN only available via resolver
	_ = os.Unsetenv("OGS_DEVICE_PIN")

	mockIoT := newMockIoTService()
	apiKey := "valid-api-key-resolver"
	device := &iot.Device{
		DeviceID:   "device-resolver",
		DeviceType: "terminal",
		Status:     iot.DeviceStatusActive,
	}
	device.TenantID = 42
	mockIoT.addDevice(apiKey, device)

	// PIN resolver returns a tenant-specific PIN
	resolverCalled := false
	resolver := func(_ context.Context, tenantID int64) string {
		resolverCalled = true
		if tenantID == 42 {
			return "9999"
		}
		return ""
	}

	r := chi.NewRouter()
	r.Use(DeviceAuthenticator(mockIoT, nil, nil, resolver))
	r.Post("/checkin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/checkin", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Staff-PIN", "9999")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code, "should authenticate using PIN from resolver")
	assert.True(t, resolverCalled, "PIN resolver should have been called")
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestDeviceAuthenticator_PINResolverFallsBackToEnv(t *testing.T) {
	lastSeenWriteCache = sync.Map{}

	// Set env var as fallback
	require.NoError(t, os.Setenv("OGS_DEVICE_PIN", "env-pin"))
	defer func() { _ = os.Unsetenv("OGS_DEVICE_PIN") }()

	mockIoT := newMockIoTService()
	apiKey := "valid-api-key-fallback"
	device := &iot.Device{
		DeviceID:   "device-fallback",
		DeviceType: "terminal",
		Status:     iot.DeviceStatusActive,
	}
	device.TenantID = 42
	mockIoT.addDevice(apiKey, device)

	// Resolver returns empty — should fall back to env var
	resolver := func(_ context.Context, _ int64) string {
		return ""
	}

	r := chi.NewRouter()
	r.Use(DeviceAuthenticator(mockIoT, nil, nil, resolver))
	r.Post("/checkin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/checkin", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Staff-PIN", "env-pin")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code, "should fall back to OGS_DEVICE_PIN env var")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestDeviceAuthenticator_PINResolverWrongPIN(t *testing.T) {
	lastSeenWriteCache = sync.Map{}
	_ = os.Unsetenv("OGS_DEVICE_PIN")

	mockIoT := newMockIoTService()
	apiKey := "valid-api-key-wrong"
	device := &iot.Device{
		DeviceID:   "device-wrong",
		DeviceType: "terminal",
		Status:     iot.DeviceStatusActive,
	}
	device.TenantID = 42
	mockIoT.addDevice(apiKey, device)

	resolver := func(_ context.Context, _ int64) string {
		return "9999"
	}

	r := chi.NewRouter()
	r.Use(DeviceAuthenticator(mockIoT, nil, nil, resolver))
	r.Post("/checkin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/checkin", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Staff-PIN", "0000") // wrong PIN
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "wrong PIN should be rejected")
}

// =============================================================================
// Error Response Tests
// =============================================================================

func TestErrDeviceUnauthorized(t *testing.T) {
	t.Parallel()

	renderer := ErrDeviceUnauthorized(ErrInvalidAPIKey)
	assert.NotNil(t, renderer)

	errResp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.StatusText)
	assert.Equal(t, "invalid device API key", errResp.ErrorText)
}

func TestErrDeviceForbidden(t *testing.T) {
	t.Parallel()

	renderer := ErrDeviceForbidden(ErrDeviceInactive)
	assert.NotNil(t, renderer)

	errResp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusForbidden, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.StatusText)
	assert.Equal(t, "device is not active", errResp.ErrorText)
}

func TestErrResponse_Render(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	errResp := &ErrResponse{
		Err:            ErrMissingAPIKey,
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "error",
		ErrorText:      "device API key is required",
	}

	err := errResp.Render(rr, req)
	assert.NoError(t, err)
}

// =============================================================================
// Error Types Tests
// =============================================================================

func TestErrorTypes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		err      error
		expected string
	}{
		{ErrMissingAPIKey, "device API key is required"},
		{ErrInvalidAPIKey, "invalid device API key"},
		{ErrInvalidAPIKeyFormat, "invalid API key format - use Bearer token"},
		{ErrMissingPIN, "staff PIN is required"},
		{ErrInvalidPIN, "invalid staff PIN"},
		{ErrDeviceInactive, "device is not active"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.err.Error())
		})
	}
}

// =============================================================================
// Context Key Tests
// =============================================================================

func TestCtxKey_DistinctValues(t *testing.T) {
	t.Parallel()

	assert.NotEqual(t, CtxDevice, CtxStaff)
	assert.NotEqual(t, CtxDevice, CtxIsIoTDevice)
	assert.NotEqual(t, CtxStaff, CtxIsIoTDevice)
}

// =============================================================================
// Update Last Seen Tests
// =============================================================================

// Deliberately NOT parallel: the test resets lastSeenWriteCache, the
// package-level debounce map every device authentication shares.
func TestDeviceOnlyAuthenticator_UpdateLastSeenError(t *testing.T) {
	lastSeenWriteCache = sync.Map{}

	mockService := newMockIoTService()
	mockService.updateError = errors.New("database error")

	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "terminal",
		Status:     iot.DeviceStatusActive,
	}
	mockService.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceOnlyAuthenticator(mockService, nil))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		// Request should still succeed even if update fails
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	// Should still succeed - update error is logged but not blocking
	assert.Equal(t, http.StatusOK, rr.Code)
}

// Deliberately NOT parallel: the test resets lastSeenWriteCache, the
// package-level debounce map every device authentication shares.
func TestDeviceOnlyAuthenticator_DebouncesLastSeenWrites(t *testing.T) {
	lastSeenWriteCache = sync.Map{}

	mockService := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "terminal",
		Status:     iot.DeviceStatusActive,
	}
	mockService.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceOnlyAuthenticator(mockService, nil))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.Header.Set("Authorization", "Bearer "+apiKey)
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code)
	assert.True(t, mockService.updateCalled, "first request should update last seen")

	mockService.updateCalled = false

	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("Authorization", "Bearer "+apiKey)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)
	assert.False(t, mockService.updateCalled, "second request inside debounce window should skip last seen write")
}

// =============================================================================
// PIN Timing Attack Resistance Tests
// =============================================================================

func TestExtractAndValidateAPIKey_NilDeviceReturn(t *testing.T) {
	t.Parallel()

	// Test the path where GetDeviceByAPIKey returns nil device with no error
	mockService := &mockIoTServiceNilDevice{}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-key-nil-device")

	device, errResp := extractAndValidateAPIKey(req, mockService)

	assert.Nil(t, device)
	assert.NotNil(t, errResp)
}

// mockIoTServiceNilDevice returns nil device without error
type mockIoTServiceNilDevice struct {
	mockIoTService
}

func (m *mockIoTServiceNilDevice) GetDeviceByAPIKey(_ context.Context, _ string) (*iot.Device, error) {
	return nil, nil // nil device, no error
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestDeviceAuthenticator_NilDeviceReturn(t *testing.T) {
	// Test the full middleware path where device is nil
	require.NoError(t, os.Setenv("OGS_DEVICE_PIN", "test-pin"))
	defer func() { _ = os.Unsetenv("OGS_DEVICE_PIN") }()

	mockService := &mockIoTServiceNilDevice{mockIoTService: *newMockIoTService()}

	r := chi.NewRouter()
	r.Use(DeviceAuthenticator(mockService, nil, nil, nil))
	r.Post("/checkin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/checkin", nil)
	req.Header.Set("Authorization", "Bearer some-key")
	req.Header.Set("X-Staff-PIN", "test-pin")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestDeviceOnlyAuthenticator_NilDeviceReturn(t *testing.T) {
	t.Parallel()

	mockService := &mockIoTServiceNilDevice{mockIoTService: *newMockIoTService()}

	r := chi.NewRouter()
	r.Use(DeviceOnlyAuthenticator(mockService, nil))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer some-key")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// =============================================================================
// rejectDeletedSchool Tests
// =============================================================================

// mockSchoolRepo implements the SchoolLookup interface used by rejectDeletedSchool.
type mockSchoolRepo struct {
	school *platform.School
	err    error
}

func (m *mockSchoolRepo) GetSchoolByID(_ context.Context, _ int64) (*platform.School, error) {
	return m.school, m.err
}

func TestRejectDeletedSchool_ActiveSchool_ReturnsNil(t *testing.T) {
	t.Parallel()

	repo := &mockSchoolRepo{school: &platform.School{Active: true}}
	device := &iot.Device{DeviceID: "device-001", TenantModel: modelBase.TenantModel{TenantID: 100}}

	result := rejectDeletedSchool(context.Background(), repo, device)
	assert.Nil(t, result, "active school should not be rejected")
}

func TestRejectDeletedSchool_DeletedSchool_ReturnsForbidden(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &mockSchoolRepo{school: &platform.School{DeletedAt: &now, Active: true}}
	device := &iot.Device{DeviceID: "device-001", TenantModel: modelBase.TenantModel{TenantID: 100}}

	result := rejectDeletedSchool(context.Background(), repo, device)
	assert.NotNil(t, result, "deleted school should be rejected")
}

func TestRejectDeletedSchool_NilRepo_ReturnsNil(t *testing.T) {
	t.Parallel()

	device := &iot.Device{DeviceID: "device-001", TenantModel: modelBase.TenantModel{TenantID: 100}}

	result := rejectDeletedSchool(context.Background(), nil, device)
	assert.Nil(t, result, "nil repo should fail open")
}

func TestRejectDeletedSchool_NilSchool_ReturnsForbidden(t *testing.T) {
	t.Parallel()

	repo := &mockSchoolRepo{school: nil, err: nil}
	device := &iot.Device{DeviceID: "device-001", TenantModel: modelBase.TenantModel{TenantID: 100}}

	result := rejectDeletedSchool(context.Background(), repo, device)
	assert.NotNil(t, result, "non-existent school should be rejected")
}

func TestRejectDeletedSchool_NonTransientDBError_RejectsDevice(t *testing.T) {
	t.Parallel()

	// Non-transient errors (bad query, permission issue, etc.) must fail closed
	// to prevent bypassing the soft-delete guard.
	repo := &mockSchoolRepo{err: errors.New("connection refused")}
	device := &iot.Device{DeviceID: "device-001", TenantModel: modelBase.TenantModel{TenantID: 100}}

	result := rejectDeletedSchool(context.Background(), repo, device)
	assert.NotNil(t, result, "non-transient DB errors should reject device")
}

func TestRejectDeletedSchool_TransientDBError_FailsOpen(t *testing.T) {
	t.Parallel()

	// Genuine transient connectivity errors should fail open so IoT devices
	// keep working during brief outages.
	repo := &mockSchoolRepo{err: &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: errors.New("connection refused"),
	}}
	device := &iot.Device{DeviceID: "device-001", TenantModel: modelBase.TenantModel{TenantID: 100}}

	result := rejectDeletedSchool(context.Background(), repo, device)
	assert.Nil(t, result, "transient DB errors should fail open")
}

func TestRejectDeletedSchool_ContextTimeout_FailsOpen(t *testing.T) {
	t.Parallel()

	repo := &mockSchoolRepo{err: context.DeadlineExceeded}
	device := &iot.Device{DeviceID: "device-001", TenantModel: modelBase.TenantModel{TenantID: 100}}

	result := rejectDeletedSchool(context.Background(), repo, device)
	assert.Nil(t, result, "context deadline errors should fail open")
}

// Deliberately NOT parallel: the test resets lastSeenWriteCache, the
// package-level debounce map every device authentication shares.
func TestDeviceOnlyAuthenticator_DeletedSchool_Forbidden(t *testing.T) {
	lastSeenWriteCache = sync.Map{}

	mockService := newMockIoTService()
	apiKey := "valid-api-key-deleted"
	device := &iot.Device{
		TenantModel: modelBase.TenantModel{TenantID: 100},
		DeviceID:    "device-deleted-school",
		DeviceType:  "terminal",
		Status:      iot.DeviceStatusActive,
	}
	mockService.addDevice(apiKey, device)

	now := time.Now()
	repo := &mockSchoolRepo{school: &platform.School{DeletedAt: &now}}

	r := chi.NewRouter()
	r.Use(DeviceOnlyAuthenticator(mockService, repo))
	r.Get("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code, "devices belonging to deleted schools must be rejected")
}

func TestSecureCompareStrings_TimingResistance(t *testing.T) {
	t.Parallel()

	// This test verifies the constant-time comparison is used
	// We can't easily test timing, but we can verify behavior

	correctPIN := "correct-pin-12345"
	wrongPIN := "wrong-pin-67890"
	partialMatchPIN := "correct-pin-12346" // Differs only in last char

	// All comparisons should return consistent results
	assert.True(t, SecureCompareStrings(correctPIN, correctPIN))
	assert.False(t, SecureCompareStrings(correctPIN, wrongPIN))
	assert.False(t, SecureCompareStrings(correctPIN, partialMatchPIN))
	assert.False(t, SecureCompareStrings(correctPIN, ""))
	assert.False(t, SecureCompareStrings("", correctPIN))
}

// =============================================================================
// isNotFoundErr Tests
// =============================================================================

func TestIsNotFoundErr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "sql.ErrNoRows directly",
			err:  sql.ErrNoRows,
			want: true,
		},
		{
			name: "DatabaseError wrapping sql.ErrNoRows",
			err:  &modelBase.DatabaseError{Op: "find", Err: sql.ErrNoRows},
			want: true,
		},
		{
			name: "DatabaseError wrapping a different error",
			err:  &modelBase.DatabaseError{Op: "find", Err: errors.New("permission denied")},
			want: false,
		},
		{
			name: "random error",
			err:  errors.New("something went wrong"),
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isNotFoundErr(tc.err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// =============================================================================
// isTransientDBErr Tests
// =============================================================================

// stubNetError implements net.Error for testing transient error detection.
type stubNetError struct{}

func (stubNetError) Error() string   { return "network error" }
func (stubNetError) Timeout() bool   { return true }
func (stubNetError) Temporary() bool { return true }

func TestIsTransientDBErr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "context.DeadlineExceeded",
			err:  context.DeadlineExceeded,
			want: true,
		},
		{
			name: "context.Canceled",
			err:  context.Canceled,
			want: true,
		},
		{
			name: "net.Error stub",
			err:  stubNetError{},
			want: true,
		},
		{
			name: "DatabaseError wrapping context.DeadlineExceeded (recursive unwrap)",
			err:  &modelBase.DatabaseError{Op: "find", Err: context.DeadlineExceeded},
			want: true,
		},
		{
			name: "random error",
			err:  errors.New("some random error"),
			want: false,
		},
		{
			name: "sql.ErrNoRows is not transient",
			err:  sql.ErrNoRows,
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isTransientDBErr(tc.err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// =============================================================================
// rejectDeletedSchool error-path Tests
// =============================================================================

func TestRejectDeletedSchool_SchoolNotFound(t *testing.T) {
	t.Parallel()

	// FindByID returns sql.ErrNoRows wrapped in DatabaseError — should reject.
	repo := &mockSchoolRepo{
		err: &modelBase.DatabaseError{Op: "find", Err: sql.ErrNoRows},
	}
	device := &iot.Device{
		DeviceID:    "device-notfound",
		TenantModel: modelBase.TenantModel{TenantID: 100},
	}

	result := rejectDeletedSchool(context.Background(), repo, device)
	assert.NotNil(t, result, "school not found (DatabaseError wrapping sql.ErrNoRows) should reject device")
}

func TestRejectDeletedSchool_TransientError_FailsOpen(t *testing.T) {
	t.Parallel()

	// FindByID returns context.DeadlineExceeded wrapped in DatabaseError — fail open.
	repo := &mockSchoolRepo{
		err: &modelBase.DatabaseError{Op: "find", Err: context.DeadlineExceeded},
	}
	device := &iot.Device{
		DeviceID:    "device-timeout",
		TenantModel: modelBase.TenantModel{TenantID: 100},
	}

	result := rejectDeletedSchool(context.Background(), repo, device)
	assert.Nil(t, result, "transient DB error (wrapped DeadlineExceeded) should fail open")
}

func TestRejectDeletedSchool_NonTransientError_FailsClosed(t *testing.T) {
	t.Parallel()

	// FindByID returns a permission error wrapped in DatabaseError — fail closed.
	repo := &mockSchoolRepo{
		err: &modelBase.DatabaseError{Op: "find", Err: errors.New("permission denied")},
	}
	device := &iot.Device{
		DeviceID:    "device-permission",
		TenantModel: modelBase.TenantModel{TenantID: 100},
	}

	result := rejectDeletedSchool(context.Background(), repo, device)
	assert.NotNil(t, result, "non-transient DB error should reject device (fail closed)")
}

func TestRejectDeletedSchool_NilSchool(t *testing.T) {
	t.Parallel()

	// FindByID returns (nil, nil) — school doesn't exist, reject.
	repo := &mockSchoolRepo{school: nil, err: nil}
	device := &iot.Device{
		DeviceID:    "device-nil-school",
		TenantModel: modelBase.TenantModel{TenantID: 100},
	}

	result := rejectDeletedSchool(context.Background(), repo, device)
	assert.NotNil(t, result, "nil school (no error) should reject device")
}
