package device

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// =============================================================================
// Mock IoT Service - implements iot.Service interface
// =============================================================================

type mockIoTService struct {
	devices      map[string]*iot.Device
	updateCalled bool
	updateError  error
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
func (m *mockIoTService) WithTx(_ bun.Tx) interface{} { return m }
func (m *mockIoTService) CreateDevice(_ context.Context, _ *iot.Device) error {
	return nil
}
func (m *mockIoTService) GetDeviceByID(_ context.Context, _ int64) (*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) GetDeviceByDeviceID(_ context.Context, _ string) (*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) DeleteDevice(_ context.Context, _ int64) error       { return nil }
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
func (m *mockIoTService) DetectNewDevices(_ context.Context) ([]*iot.Device, error) { return nil, nil }
func (m *mockIoTService) ScanNetwork(_ context.Context) (map[string]string, error)  { return nil, nil }

// =============================================================================
// Mock Person Service - not actually used by DeviceAuthenticator
// =============================================================================

// mockPersonService is not needed since DeviceAuthenticator doesn't use it
// The PersonService parameter is unused in the current implementation

// =============================================================================
// Context Helpers Tests
// =============================================================================

func TestDeviceFromCtx_ValidDevice(t *testing.T) {
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "rfid_reader",
		Status:     iot.DeviceStatusActive,
	}

	ctx := context.WithValue(context.Background(), CtxDevice, device)

	result := DeviceFromCtx(ctx)
	require.NotNil(t, result)
	assert.Equal(t, "device-001", result.DeviceID)
	assert.Equal(t, "rfid_reader", result.DeviceType)
}

func TestDeviceFromCtx_NoDevice(t *testing.T) {
	ctx := context.Background()
	result := DeviceFromCtx(ctx)
	assert.Nil(t, result)
}

func TestDeviceFromCtx_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), CtxDevice, "not a device")
	result := DeviceFromCtx(ctx)
	assert.Nil(t, result)
}

func TestStaffFromCtx_ValidStaff(t *testing.T) {
	staff := &users.Staff{
		StaffNotes: "Test staff member",
	}

	ctx := context.WithValue(context.Background(), CtxStaff, staff)

	result := StaffFromCtx(ctx)
	require.NotNil(t, result)
	assert.Equal(t, "Test staff member", result.StaffNotes)
}

func TestStaffFromCtx_NoStaff(t *testing.T) {
	ctx := context.Background()
	result := StaffFromCtx(ctx)
	assert.Nil(t, result)
}

func TestStaffFromCtx_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), CtxStaff, "not a staff")
	result := StaffFromCtx(ctx)
	assert.Nil(t, result)
}

func TestIsIoTDeviceRequest_True(t *testing.T) {
	ctx := context.WithValue(context.Background(), CtxIsIoTDevice, true)
	result := IsIoTDeviceRequest(ctx)
	assert.True(t, result)
}

func TestIsIoTDeviceRequest_False(t *testing.T) {
	ctx := context.WithValue(context.Background(), CtxIsIoTDevice, false)
	result := IsIoTDeviceRequest(ctx)
	assert.False(t, result)
}

func TestIsIoTDeviceRequest_NotSet(t *testing.T) {
	ctx := context.Background()
	result := IsIoTDeviceRequest(ctx)
	assert.False(t, result)
}

func TestIsIoTDeviceRequest_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), CtxIsIoTDevice, "true")
	result := IsIoTDeviceRequest(ctx)
	assert.False(t, result)
}

// =============================================================================
// SecureCompareStrings Tests
// =============================================================================

func TestSecureCompareStrings_Equal(t *testing.T) {
	assert.True(t, SecureCompareStrings("password", "password"))
	assert.True(t, SecureCompareStrings("", ""))
	assert.True(t, SecureCompareStrings("a very long string with special chars!@#$%", "a very long string with special chars!@#$%"))
}

func TestSecureCompareStrings_NotEqual(t *testing.T) {
	assert.False(t, SecureCompareStrings("password", "different"))
	assert.False(t, SecureCompareStrings("password", "Password")) // Case sensitive
	assert.False(t, SecureCompareStrings("password", "password "))
	assert.False(t, SecureCompareStrings("", "notempty"))
}

func TestSecureCompareStrings_DifferentLengths(t *testing.T) {
	assert.False(t, SecureCompareStrings("short", "muchlongerstring"))
	assert.False(t, SecureCompareStrings("muchlongerstring", "short"))
}

// =============================================================================
// DeviceOnlyAuthenticator Tests
// =============================================================================

func TestDeviceOnlyAuthenticator_ValidAPIKey(t *testing.T) {
	mockService := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "rfid_reader",
		Status:     iot.DeviceStatusActive,
	}
	mockService.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceOnlyAuthenticator(mockService))
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
	mockService := newMockIoTService()

	r := chi.NewRouter()
	r.Use(DeviceOnlyAuthenticator(mockService))
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
			r.Use(DeviceOnlyAuthenticator(mockService))
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
	mockService := newMockIoTService()
	// No devices added

	r := chi.NewRouter()
	r.Use(DeviceOnlyAuthenticator(mockService))
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
	mockService := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "rfid_reader",
		Status:     iot.DeviceStatusInactive, // Not active
	}
	mockService.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceOnlyAuthenticator(mockService))
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
	mockService := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "rfid_reader",
		Status:     iot.DeviceStatusOffline, // Offline
	}
	mockService.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceOnlyAuthenticator(mockService))
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
	mockService := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "rfid_reader",
		Status:     iot.DeviceStatusMaintenance, // In maintenance
	}
	mockService.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceOnlyAuthenticator(mockService))
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

func TestDeviceAuthenticator_ValidAPIKeyAndPIN(t *testing.T) {
	// Set up environment
	ogsPin := "test-device-pin-123"
	os.Setenv("OGS_DEVICE_PIN", ogsPin)
	defer os.Unsetenv("OGS_DEVICE_PIN")

	mockIoT := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "rfid_reader",
		Status:     iot.DeviceStatusActive,
	}
	mockIoT.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceAuthenticator(mockIoT, nil)) // PersonService not used
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

func TestDeviceAuthenticator_MissingPIN(t *testing.T) {
	os.Setenv("OGS_DEVICE_PIN", "test-pin")
	defer os.Unsetenv("OGS_DEVICE_PIN")

	mockIoT := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "rfid_reader",
		Status:     iot.DeviceStatusActive,
	}
	mockIoT.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceAuthenticator(mockIoT, nil))
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

func TestDeviceAuthenticator_InvalidPIN(t *testing.T) {
	os.Setenv("OGS_DEVICE_PIN", "correct-pin")
	defer os.Unsetenv("OGS_DEVICE_PIN")

	mockIoT := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "rfid_reader",
		Status:     iot.DeviceStatusActive,
	}
	mockIoT.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceAuthenticator(mockIoT, nil))
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

func TestDeviceAuthenticator_MissingOGSPINConfig(t *testing.T) {
	// Ensure OGS_DEVICE_PIN is not set
	os.Unsetenv("OGS_DEVICE_PIN")

	mockIoT := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "rfid_reader",
		Status:     iot.DeviceStatusActive,
	}
	mockIoT.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceAuthenticator(mockIoT, nil))
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

func TestDeviceAuthenticator_MissingAPIKey(t *testing.T) {
	os.Setenv("OGS_DEVICE_PIN", "test-pin")
	defer os.Unsetenv("OGS_DEVICE_PIN")

	mockIoT := newMockIoTService()

	r := chi.NewRouter()
	r.Use(DeviceAuthenticator(mockIoT, nil))
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

func TestDeviceAuthenticator_InvalidAPIKey(t *testing.T) {
	os.Setenv("OGS_DEVICE_PIN", "test-pin")
	defer os.Unsetenv("OGS_DEVICE_PIN")

	mockIoT := newMockIoTService()
	// No devices added

	r := chi.NewRouter()
	r.Use(DeviceAuthenticator(mockIoT, nil))
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

func TestDeviceAuthenticator_InactiveDevice(t *testing.T) {
	os.Setenv("OGS_DEVICE_PIN", "test-pin")
	defer os.Unsetenv("OGS_DEVICE_PIN")

	mockIoT := newMockIoTService()
	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "rfid_reader",
		Status:     iot.DeviceStatusInactive,
	}
	mockIoT.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceAuthenticator(mockIoT, nil))
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
// Error Response Tests
// =============================================================================

func TestErrDeviceUnauthorized(t *testing.T) {
	renderer := ErrDeviceUnauthorized(ErrInvalidAPIKey)
	assert.NotNil(t, renderer)

	errResp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.StatusText)
	assert.Equal(t, "invalid device API key", errResp.ErrorText)
}

func TestErrDeviceForbidden(t *testing.T) {
	renderer := ErrDeviceForbidden(ErrDeviceInactive)
	assert.NotNil(t, renderer)

	errResp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusForbidden, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.StatusText)
	assert.Equal(t, "device is not active", errResp.ErrorText)
}

func TestErrResponse_Render(t *testing.T) {
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
	testCases := []struct {
		err      error
		expected string
	}{
		{ErrMissingAPIKey, "device API key is required"},
		{ErrInvalidAPIKey, "invalid device API key"},
		{ErrInvalidAPIKeyFormat, "invalid API key format - use Bearer token"},
		{ErrMissingPIN, "staff PIN is required"},
		{ErrInvalidPIN, "invalid staff PIN"},
		{ErrMissingStaffID, "staff ID is required"},
		{ErrInvalidStaffID, "invalid staff ID format"},
		{ErrDeviceInactive, "device is not active"},
		{ErrStaffAccountLocked, "staff account is locked due to failed PIN attempts"},
		{ErrDeviceOffline, "device is offline"},
		{ErrPINAttemptsExceeded, "maximum PIN attempts exceeded"},
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
	assert.NotEqual(t, CtxDevice, CtxStaff)
	assert.NotEqual(t, CtxDevice, CtxIsIoTDevice)
	assert.NotEqual(t, CtxStaff, CtxIsIoTDevice)
}

// =============================================================================
// Update Last Seen Tests
// =============================================================================

func TestDeviceOnlyAuthenticator_UpdateLastSeenError(t *testing.T) {
	mockService := newMockIoTService()
	mockService.updateError = errors.New("database error")

	apiKey := "valid-api-key-123"
	device := &iot.Device{
		DeviceID:   "device-001",
		DeviceType: "rfid_reader",
		Status:     iot.DeviceStatusActive,
	}
	mockService.addDevice(apiKey, device)

	r := chi.NewRouter()
	r.Use(DeviceOnlyAuthenticator(mockService))
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

// =============================================================================
// PIN Timing Attack Resistance Tests
// =============================================================================

func TestSecureCompareStrings_TimingResistance(t *testing.T) {
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
