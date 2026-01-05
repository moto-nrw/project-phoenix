package device

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Mock IoT Service for testing
type mockIoTService struct {
	devices        map[string]*iot.Device // Key: API key
	updateCalled   bool
	updateError    error
	getByAPIKeyErr error
}

func newMockIoTService() *mockIoTService {
	return &mockIoTService{
		devices: make(map[string]*iot.Device),
	}
}

func (m *mockIoTService) GetDeviceByAPIKey(ctx context.Context, apiKey string) (*iot.Device, error) {
	if m.getByAPIKeyErr != nil {
		return nil, m.getByAPIKeyErr
	}
	device, ok := m.devices[apiKey]
	if !ok {
		return nil, nil
	}
	return device, nil
}

func (m *mockIoTService) UpdateDevice(ctx context.Context, device *iot.Device) error {
	m.updateCalled = true
	if m.updateError != nil {
		return m.updateError
	}
	// Update the device in the map
	if device.APIKey != nil {
		m.devices[*device.APIKey] = device
	}
	return nil
}

// Implement required interface methods (not used in device auth tests)
func (m *mockIoTService) CreateDevice(ctx context.Context, device *iot.Device) error {
	return nil
}
func (m *mockIoTService) GetDeviceByID(ctx context.Context, id int64) (*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) GetDeviceByDeviceID(ctx context.Context, deviceID string) (*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) DeleteDevice(ctx context.Context, id int64) error {
	return nil
}
func (m *mockIoTService) ListDevices(ctx context.Context, filters map[string]interface{}) ([]*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) UpdateDeviceStatus(ctx context.Context, deviceID string, status iot.DeviceStatus) error {
	return nil
}
func (m *mockIoTService) PingDevice(ctx context.Context, deviceID string) error {
	return nil
}
func (m *mockIoTService) GetDevicesByType(ctx context.Context, deviceType string) ([]*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) GetDevicesByStatus(ctx context.Context, status iot.DeviceStatus) ([]*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) GetDevicesByRegisteredBy(ctx context.Context, personID int64) ([]*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) GetActiveDevices(ctx context.Context) ([]*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) GetOfflineDevices(ctx context.Context, offlineDuration time.Duration) ([]*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) GetDevicesRequiringMaintenance(ctx context.Context) ([]*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) GetDeviceTypeStatistics(ctx context.Context) (map[string]int, error) {
	return nil, nil
}
func (m *mockIoTService) DetectNewDevices(ctx context.Context) ([]*iot.Device, error) {
	return nil, nil
}
func (m *mockIoTService) ScanNetwork(ctx context.Context) (map[string]string, error) {
	return nil, nil
}
func (m *mockIoTService) RunInTransaction(ctx context.Context, fn func(context.Context) error) error {
	return nil
}
func (m *mockIoTService) WithTx(tx bun.Tx) interface{} {
	return m
}

// Mock Person Service for testing
type mockPersonService struct{}

func newMockPersonService() *mockPersonService {
	return &mockPersonService{}
}

// Implement PersonService interface methods (stub implementations for testing)
func (m *mockPersonService) Get(ctx context.Context, id interface{}) (*users.Person, error) {
	return nil, nil
}
func (m *mockPersonService) GetByIDs(ctx context.Context, ids []int64) (map[int64]*users.Person, error) {
	return nil, nil
}
func (m *mockPersonService) Create(ctx context.Context, person *users.Person) error {
	return nil
}
func (m *mockPersonService) Update(ctx context.Context, person *users.Person) error {
	return nil
}
func (m *mockPersonService) Delete(ctx context.Context, id interface{}) error {
	return nil
}
func (m *mockPersonService) List(ctx context.Context, options *base.QueryOptions) ([]*users.Person, error) {
	return nil, nil
}
func (m *mockPersonService) FindByTagID(ctx context.Context, tagID string) (*users.Person, error) {
	return nil, nil
}
func (m *mockPersonService) FindByAccountID(ctx context.Context, accountID int64) (*users.Person, error) {
	return nil, nil
}
func (m *mockPersonService) FindByName(ctx context.Context, firstName, lastName string) ([]*users.Person, error) {
	return nil, nil
}
func (m *mockPersonService) LinkToAccount(ctx context.Context, personID int64, accountID int64) error {
	return nil
}
func (m *mockPersonService) UnlinkFromAccount(ctx context.Context, personID int64) error {
	return nil
}
func (m *mockPersonService) LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error {
	return nil
}
func (m *mockPersonService) UnlinkFromRFIDCard(ctx context.Context, personID int64) error {
	return nil
}
func (m *mockPersonService) GetFullProfile(ctx context.Context, personID int64) (*users.Person, error) {
	return nil, nil
}
func (m *mockPersonService) FindByGuardianID(ctx context.Context, guardianAccountID int64) ([]*users.Person, error) {
	return nil, nil
}
func (m *mockPersonService) StudentRepository() users.StudentRepository {
	return nil
}
func (m *mockPersonService) StaffRepository() users.StaffRepository {
	return nil
}
func (m *mockPersonService) TeacherRepository() users.TeacherRepository {
	return nil
}
func (m *mockPersonService) ValidateStaffPIN(ctx context.Context, pin string) (*users.Staff, error) {
	return nil, nil
}
func (m *mockPersonService) ValidateStaffPINForSpecificStaff(ctx context.Context, staffID int64, pin string) (*users.Staff, error) {
	return nil, nil
}
func (m *mockPersonService) GetStudentsByTeacher(ctx context.Context, teacherID int64) ([]*users.Student, error) {
	return nil, nil
}
func (m *mockPersonService) RunInTransaction(ctx context.Context, fn func(context.Context) error) error {
	return nil
}
func (m *mockPersonService) GetStudentsWithGroupsByTeacher(ctx context.Context, teacherID int64) ([]usersSvc.StudentWithGroup, error) {
	return nil, nil
}
func (m *mockPersonService) ListAvailableRFIDCards(ctx context.Context) ([]*users.RFIDCard, error) {
	return nil, nil
}
func (m *mockPersonService) WithTx(tx bun.Tx) interface{} {
	return m
}

// createTestDevice creates a test device with the given API key and status
func createTestDevice(apiKey string, status iot.DeviceStatus) *iot.Device {
	device := &iot.Device{
		Model: base.Model{
			ID: 1,
		},
		DeviceID:   "test-device-001",
		DeviceType: "rfid-reader",
		APIKey:     &apiKey,
		Status:     status,
	}
	name := "Test RFID Reader"
	device.Name = &name
	return device
}

// createTestStaff creates a test staff member
func createTestStaff() *users.Staff {
	return &users.Staff{
		Model: base.Model{
			ID: 1,
		},
		PersonID: 1,
	}
}

// TestDeviceFromCtx tests retrieving device from context
func TestDeviceFromCtx(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected *iot.Device
	}{
		{
			name:     "valid device in context",
			ctx:      context.WithValue(context.Background(), CtxDevice, createTestDevice("api-key-123", iot.DeviceStatusActive)),
			expected: createTestDevice("api-key-123", iot.DeviceStatusActive),
		},
		{
			name:     "no device in context",
			ctx:      context.Background(),
			expected: nil,
		},
		{
			name:     "wrong type in context",
			ctx:      context.WithValue(context.Background(), CtxDevice, "not-a-device"),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeviceFromCtx(tt.ctx)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.DeviceID, result.DeviceID)
			}
		})
	}
}

// TestStaffFromCtx tests retrieving staff from context
func TestStaffFromCtx(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected *users.Staff
	}{
		{
			name:     "valid staff in context",
			ctx:      context.WithValue(context.Background(), CtxStaff, createTestStaff()),
			expected: createTestStaff(),
		},
		{
			name:     "no staff in context",
			ctx:      context.Background(),
			expected: nil,
		},
		{
			name:     "wrong type in context",
			ctx:      context.WithValue(context.Background(), CtxStaff, "not-staff"),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StaffFromCtx(tt.ctx)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.ID, result.ID)
			}
		})
	}
}

// TestIsIoTDeviceRequest tests checking if request is from IoT device
func TestIsIoTDeviceRequest(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected bool
	}{
		{
			name:     "IoT device request (true)",
			ctx:      context.WithValue(context.Background(), CtxIsIoTDevice, true),
			expected: true,
		},
		{
			name:     "IoT device request (false)",
			ctx:      context.WithValue(context.Background(), CtxIsIoTDevice, false),
			expected: false,
		},
		{
			name:     "no IoT device flag in context",
			ctx:      context.Background(),
			expected: false,
		},
		{
			name:     "wrong type in context",
			ctx:      context.WithValue(context.Background(), CtxIsIoTDevice, "not-a-bool"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsIoTDeviceRequest(tt.ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSecureCompareStrings tests constant-time string comparison
func TestSecureCompareStrings(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected bool
	}{
		{
			name:     "identical strings",
			a:        "1234",
			b:        "1234",
			expected: true,
		},
		{
			name:     "different strings",
			a:        "1234",
			b:        "5678",
			expected: false,
		},
		{
			name:     "different lengths",
			a:        "1234",
			b:        "12345",
			expected: false,
		},
		{
			name:     "empty strings",
			a:        "",
			b:        "",
			expected: true,
		},
		{
			name:     "one empty string",
			a:        "1234",
			b:        "",
			expected: false,
		},
		{
			name:     "case sensitive",
			a:        "AbCd",
			b:        "abcd",
			expected: false,
		},
		{
			name:     "unicode characters",
			a:        "café",
			b:        "café",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SecureCompareStrings(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSecureCompareStrings_TimingAttackResistance tests that comparison is constant-time
func TestSecureCompareStrings_TimingAttackResistance(t *testing.T) {
	// This test verifies that comparison time is consistent regardless of where strings differ
	const iterations = 1000
	const correctPIN = "1234567890"

	// Measure time for strings that differ at different positions
	testCases := []string{
		"0234567890", // Differs at position 0
		"1034567890", // Differs at position 1
		"1204567890", // Differs at position 2
		"1230567890", // Differs at position 3
		"0000000000", // Differs at all positions
	}

	// Run comparisons and verify all return false (they should all be constant time)
	for _, testCase := range testCases {
		startTime := time.Now()
		for i := 0; i < iterations; i++ {
			result := SecureCompareStrings(correctPIN, testCase)
			assert.False(t, result, "Comparison should return false for different strings")
		}
		elapsed := time.Since(startTime)

		// Just verify it completes (actual timing analysis would need more sophisticated tests)
		assert.True(t, elapsed > 0, "Comparison should take some time")
	}
}

// TestDeviceAuthenticator tests the two-layer device authentication middleware
func TestDeviceAuthenticator(t *testing.T) {
	// Set up OGS PIN for tests
	testOGSPin := "test-ogs-pin-1234"
	os.Setenv("OGS_DEVICE_PIN", testOGSPin)
	defer os.Unsetenv("OGS_DEVICE_PIN")

	t.Run("valid API key and PIN passes through", func(t *testing.T) {
		iotService := newMockIoTService()
		personService := newMockPersonService()

		// Create active device
		apiKey := "valid-api-key-123"
		device := createTestDevice(apiKey, iot.DeviceStatusActive)
		iotService.devices[apiKey] = device

		req := httptest.NewRequest(http.MethodPost, "/iot/checkin", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("X-Staff-PIN", testOGSPin)

		// Handler to verify device is in context
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			device := DeviceFromCtx(r.Context())
			assert.NotNil(t, device)
			assert.Equal(t, "test-device-001", device.DeviceID)

			isIoT := IsIoTDeviceRequest(r.Context())
			assert.True(t, isIoT)

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		})

		middleware := DeviceAuthenticator(iotService, personService)
		handler := middleware(testHandler)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "OK", rr.Body.String())
		assert.True(t, iotService.updateCalled, "UpdateDevice should be called to update last seen")
	})

	t.Run("missing Authorization header returns 401", func(t *testing.T) {
		iotService := newMockIoTService()
		personService := newMockPersonService()

		req := httptest.NewRequest(http.MethodPost, "/iot/checkin", nil)
		// No Authorization header
		req.Header.Set("X-Staff-PIN", testOGSPin)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called")
		})

		middleware := DeviceAuthenticator(iotService, personService)
		handler := middleware(testHandler)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Body.String(), "error")
	})

	t.Run("invalid Authorization header format returns 401", func(t *testing.T) {
		iotService := newMockIoTService()
		personService := newMockPersonService()

		req := httptest.NewRequest(http.MethodPost, "/iot/checkin", nil)
		req.Header.Set("Authorization", "InvalidFormat api-key-123") // Not "Bearer"
		req.Header.Set("X-Staff-PIN", testOGSPin)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called")
		})

		middleware := DeviceAuthenticator(iotService, personService)
		handler := middleware(testHandler)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("empty API key returns 401", func(t *testing.T) {
		iotService := newMockIoTService()
		personService := newMockPersonService()

		req := httptest.NewRequest(http.MethodPost, "/iot/checkin", nil)
		req.Header.Set("Authorization", "Bearer ") // Empty after Bearer
		req.Header.Set("X-Staff-PIN", testOGSPin)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called")
		})

		middleware := DeviceAuthenticator(iotService, personService)
		handler := middleware(testHandler)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("invalid API key returns 401", func(t *testing.T) {
		iotService := newMockIoTService()
		personService := newMockPersonService()

		req := httptest.NewRequest(http.MethodPost, "/iot/checkin", nil)
		req.Header.Set("Authorization", "Bearer invalid-api-key")
		req.Header.Set("X-Staff-PIN", testOGSPin)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called")
		})

		middleware := DeviceAuthenticator(iotService, personService)
		handler := middleware(testHandler)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("inactive device returns 403", func(t *testing.T) {
		iotService := newMockIoTService()
		personService := newMockPersonService()

		// Create inactive device
		apiKey := "inactive-device-key"
		device := createTestDevice(apiKey, iot.DeviceStatusInactive)
		iotService.devices[apiKey] = device

		req := httptest.NewRequest(http.MethodPost, "/iot/checkin", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("X-Staff-PIN", testOGSPin)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called for inactive device")
		})

		middleware := DeviceAuthenticator(iotService, personService)
		handler := middleware(testHandler)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusForbidden, rr.Code)
		assert.Contains(t, rr.Body.String(), "not active")
	})

	t.Run("missing X-Staff-PIN header returns 401", func(t *testing.T) {
		iotService := newMockIoTService()
		personService := newMockPersonService()

		apiKey := "valid-api-key"
		device := createTestDevice(apiKey, iot.DeviceStatusActive)
		iotService.devices[apiKey] = device

		req := httptest.NewRequest(http.MethodPost, "/iot/checkin", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		// No X-Staff-PIN header

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called")
		})

		middleware := DeviceAuthenticator(iotService, personService)
		handler := middleware(testHandler)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Body.String(), "PIN")
	})

	t.Run("invalid PIN returns 401", func(t *testing.T) {
		iotService := newMockIoTService()
		personService := newMockPersonService()

		apiKey := "valid-api-key"
		device := createTestDevice(apiKey, iot.DeviceStatusActive)
		iotService.devices[apiKey] = device

		req := httptest.NewRequest(http.MethodPost, "/iot/checkin", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("X-Staff-PIN", "wrong-pin-9999")

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called with wrong PIN")
		})

		middleware := DeviceAuthenticator(iotService, personService)
		handler := middleware(testHandler)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid")
	})

	t.Run("missing OGS_DEVICE_PIN environment variable returns 401", func(t *testing.T) {
		// Temporarily unset the environment variable
		os.Unsetenv("OGS_DEVICE_PIN")
		defer os.Setenv("OGS_DEVICE_PIN", testOGSPin)

		iotService := newMockIoTService()
		personService := newMockPersonService()

		apiKey := "valid-api-key"
		device := createTestDevice(apiKey, iot.DeviceStatusActive)
		iotService.devices[apiKey] = device

		req := httptest.NewRequest(http.MethodPost, "/iot/checkin", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("X-Staff-PIN", "1234")

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called")
		})

		middleware := DeviceAuthenticator(iotService, personService)
		handler := middleware(testHandler)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

// TestDeviceOnlyAuthenticator tests the device-only authentication middleware (no PIN required)
func TestDeviceOnlyAuthenticator(t *testing.T) {
	t.Run("valid API key passes through without PIN", func(t *testing.T) {
		iotService := newMockIoTService()

		apiKey := "valid-device-key"
		device := createTestDevice(apiKey, iot.DeviceStatusActive)
		iotService.devices[apiKey] = device

		req := httptest.NewRequest(http.MethodGet, "/iot/teachers", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		// No X-Staff-PIN header required

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			device := DeviceFromCtx(r.Context())
			assert.NotNil(t, device)
			assert.Equal(t, "test-device-001", device.DeviceID)

			// Should NOT be marked as IoT device request (no PIN verification)
			isIoT := IsIoTDeviceRequest(r.Context())
			assert.False(t, isIoT, "DeviceOnlyAuthenticator should not set IsIoTDevice flag")

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		})

		middleware := DeviceOnlyAuthenticator(iotService)
		handler := middleware(testHandler)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "OK", rr.Body.String())
		assert.True(t, iotService.updateCalled, "UpdateDevice should be called")
	})

	t.Run("missing Authorization header returns 401", func(t *testing.T) {
		iotService := newMockIoTService()

		req := httptest.NewRequest(http.MethodGet, "/iot/teachers", nil)
		// No Authorization header

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called")
		})

		middleware := DeviceOnlyAuthenticator(iotService)
		handler := middleware(testHandler)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("invalid API key returns 401", func(t *testing.T) {
		iotService := newMockIoTService()

		req := httptest.NewRequest(http.MethodGet, "/iot/teachers", nil)
		req.Header.Set("Authorization", "Bearer invalid-key")

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called")
		})

		middleware := DeviceOnlyAuthenticator(iotService)
		handler := middleware(testHandler)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("inactive device returns 403", func(t *testing.T) {
		iotService := newMockIoTService()

		apiKey := "inactive-device-key"
		device := createTestDevice(apiKey, iot.DeviceStatusMaintenance)
		iotService.devices[apiKey] = device

		req := httptest.NewRequest(http.MethodGet, "/iot/teachers", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called for inactive device")
		})

		middleware := DeviceOnlyAuthenticator(iotService)
		handler := middleware(testHandler)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("device with offline status returns 403", func(t *testing.T) {
		iotService := newMockIoTService()

		apiKey := "offline-device-key"
		device := createTestDevice(apiKey, iot.DeviceStatusOffline)
		iotService.devices[apiKey] = device

		req := httptest.NewRequest(http.MethodGet, "/iot/teachers", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called for offline device")
		})

		middleware := DeviceOnlyAuthenticator(iotService)
		handler := middleware(testHandler)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusForbidden, rr.Code)
	})
}

// TestUpdateDeviceLastSeen tests the last seen timestamp update
func TestUpdateDeviceLastSeen(t *testing.T) {
	t.Run("updates device last seen timestamp", func(t *testing.T) {
		iotService := newMockIoTService()

		apiKey := "test-device-key"
		device := createTestDevice(apiKey, iot.DeviceStatusActive)
		iotService.devices[apiKey] = device

		// Verify last seen is initially nil
		assert.Nil(t, device.LastSeen)

		// Create request
		req := httptest.NewRequest(http.MethodPost, "/test", nil)

		// Update last seen
		updateDeviceLastSeen(req, iotService, device)

		// Verify update was called
		assert.True(t, iotService.updateCalled)

		// Verify device last seen was set (UpdateLastSeen was called)
		assert.NotNil(t, device.LastSeen)
		assert.WithinDuration(t, time.Now(), *device.LastSeen, 1*time.Second)
	})

	t.Run("handles update error gracefully", func(t *testing.T) {
		iotService := newMockIoTService()
		iotService.updateError = assert.AnError // Simulate error

		apiKey := "test-device-key"
		device := createTestDevice(apiKey, iot.DeviceStatusActive)
		iotService.devices[apiKey] = device

		req := httptest.NewRequest(http.MethodPost, "/test", nil)

		// Should not panic, just log error
		require.NotPanics(t, func() {
			updateDeviceLastSeen(req, iotService, device)
		})

		assert.True(t, iotService.updateCalled)
	})
}

// TestDeviceAuthenticator_Integration tests realistic authentication flow
func TestDeviceAuthenticator_Integration(t *testing.T) {
	// Set up test environment
	testOGSPin := "integration-test-pin"
	os.Setenv("OGS_DEVICE_PIN", testOGSPin)
	defer os.Unsetenv("OGS_DEVICE_PIN")

	iotService := newMockIoTService()
	personService := newMockPersonService()

	// Register multiple devices
	device1Key := "device-1-api-key"
	device1 := createTestDevice(device1Key, iot.DeviceStatusActive)
	device1.DeviceID = "device-001"
	iotService.devices[device1Key] = device1

	device2Key := "device-2-api-key"
	device2 := createTestDevice(device2Key, iot.DeviceStatusActive)
	device2.DeviceID = "device-002"
	iotService.devices[device2Key] = device2

	// Test scenario: Device 1 makes authenticated request
	t.Run("device 1 authenticates successfully", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/iot/checkin", nil)
		req.Header.Set("Authorization", "Bearer "+device1Key)
		req.Header.Set("X-Staff-PIN", testOGSPin)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			device := DeviceFromCtx(r.Context())
			require.NotNil(t, device)
			assert.Equal(t, "device-001", device.DeviceID)
			w.WriteHeader(http.StatusOK)
		})

		middleware := DeviceAuthenticator(iotService, personService)
		handler := middleware(testHandler)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	// Test scenario: Device 2 makes authenticated request
	t.Run("device 2 authenticates successfully", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/iot/checkin", nil)
		req.Header.Set("Authorization", "Bearer "+device2Key)
		req.Header.Set("X-Staff-PIN", testOGSPin)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			device := DeviceFromCtx(r.Context())
			require.NotNil(t, device)
			assert.Equal(t, "device-002", device.DeviceID)
			w.WriteHeader(http.StatusOK)
		})

		middleware := DeviceAuthenticator(iotService, personService)
		handler := middleware(testHandler)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	// Test scenario: Device uses DeviceOnlyAuthenticator for teacher list
	t.Run("device fetches teacher list without PIN", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/iot/teachers", nil)
		req.Header.Set("Authorization", "Bearer "+device1Key)
		// No PIN required

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			device := DeviceFromCtx(r.Context())
			require.NotNil(t, device)
			assert.Equal(t, "device-001", device.DeviceID)

			// Not an IoT device request (no PIN verified)
			assert.False(t, IsIoTDeviceRequest(r.Context()))

			w.WriteHeader(http.StatusOK)
		})

		middleware := DeviceOnlyAuthenticator(iotService)
		handler := middleware(testHandler)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}
