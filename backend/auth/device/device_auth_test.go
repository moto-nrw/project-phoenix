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
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// MockIoTService is a mock implementation of iotSvc.Service for testing
type MockIoTService struct {
	GetDeviceByAPIKeyFunc func(ctx context.Context, apiKey string) (*iot.Device, error)
	UpdateDeviceFunc      func(ctx context.Context, device *iot.Device) error
}

func (m *MockIoTService) GetDeviceByAPIKey(ctx context.Context, apiKey string) (*iot.Device, error) {
	if m.GetDeviceByAPIKeyFunc != nil {
		return m.GetDeviceByAPIKeyFunc(ctx, apiKey)
	}
	return nil, nil
}

func (m *MockIoTService) UpdateDevice(ctx context.Context, device *iot.Device) error {
	if m.UpdateDeviceFunc != nil {
		return m.UpdateDeviceFunc(ctx, device)
	}
	return nil
}

// Implement other required interface methods with no-ops
func (m *MockIoTService) CreateDevice(_ context.Context, _ *iot.Device) error          { return nil }
func (m *MockIoTService) GetDeviceByID(_ context.Context, _ int64) (*iot.Device, error) { return nil, nil }
func (m *MockIoTService) GetDeviceByDeviceID(_ context.Context, _ string) (*iot.Device, error) {
	return nil, nil
}
func (m *MockIoTService) DeleteDevice(_ context.Context, _ int64) error { return nil }
func (m *MockIoTService) ListDevices(_ context.Context, _ map[string]interface{}) ([]*iot.Device, error) {
	return nil, nil
}
func (m *MockIoTService) UpdateDeviceStatus(_ context.Context, _ string, _ iot.DeviceStatus) error {
	return nil
}
func (m *MockIoTService) PingDevice(_ context.Context, _ string) error { return nil }
func (m *MockIoTService) GetDevicesByType(_ context.Context, _ string) ([]*iot.Device, error) {
	return nil, nil
}
func (m *MockIoTService) GetDevicesByStatus(_ context.Context, _ iot.DeviceStatus) ([]*iot.Device, error) {
	return nil, nil
}
func (m *MockIoTService) GetDevicesByRegisteredBy(_ context.Context, _ int64) ([]*iot.Device, error) {
	return nil, nil
}
func (m *MockIoTService) GetActiveDevices(_ context.Context) ([]*iot.Device, error) { return nil, nil }
func (m *MockIoTService) GetDevicesRequiringMaintenance(_ context.Context) ([]*iot.Device, error) {
	return nil, nil
}
func (m *MockIoTService) GetOfflineDevices(_ context.Context, _ time.Duration) ([]*iot.Device, error) {
	return nil, nil
}
func (m *MockIoTService) GetDeviceTypeStatistics(_ context.Context) (map[string]int, error) {
	return nil, nil
}
func (m *MockIoTService) DetectNewDevices(_ context.Context) ([]*iot.Device, error) { return nil, nil }
func (m *MockIoTService) ScanNetwork(_ context.Context) (map[string]string, error)  { return nil, nil }
func (m *MockIoTService) WithTx(_ bun.Tx) interface{}                               { return nil }

// MockPersonService is a mock implementation of usersSvc.PersonService for testing
type MockPersonService struct{}

func (m *MockPersonService) Get(_ context.Context, _ interface{}) (*users.Person, error) { return nil, nil }
func (m *MockPersonService) GetByIDs(_ context.Context, _ []int64) (map[int64]*users.Person, error) { return nil, nil }
func (m *MockPersonService) Create(_ context.Context, _ *users.Person) error { return nil }
func (m *MockPersonService) Update(_ context.Context, _ *users.Person) error { return nil }
func (m *MockPersonService) Delete(_ context.Context, _ interface{}) error   { return nil }
func (m *MockPersonService) List(_ context.Context, _ *base.QueryOptions) ([]*users.Person, error) { return nil, nil }
func (m *MockPersonService) FindByTagID(_ context.Context, _ string) (*users.Person, error) { return nil, nil }
func (m *MockPersonService) FindByAccountID(_ context.Context, _ int64) (*users.Person, error) { return nil, nil }
func (m *MockPersonService) FindByName(_ context.Context, _, _ string) ([]*users.Person, error) { return nil, nil }
func (m *MockPersonService) LinkToAccount(_ context.Context, _, _ int64) error { return nil }
func (m *MockPersonService) UnlinkFromAccount(_ context.Context, _ int64) error { return nil }
func (m *MockPersonService) LinkToRFIDCard(_ context.Context, _ int64, _ string) error { return nil }
func (m *MockPersonService) UnlinkFromRFIDCard(_ context.Context, _ int64) error { return nil }
func (m *MockPersonService) GetFullProfile(_ context.Context, _ int64) (*users.Person, error) { return nil, nil }
func (m *MockPersonService) FindByGuardianID(_ context.Context, _ int64) ([]*users.Person, error) { return nil, nil }
func (m *MockPersonService) StudentRepository() users.StudentRepository { return nil }
func (m *MockPersonService) StaffRepository() users.StaffRepository { return nil }
func (m *MockPersonService) TeacherRepository() users.TeacherRepository { return nil }
func (m *MockPersonService) ListAvailableRFIDCards(_ context.Context) ([]*users.RFIDCard, error) { return nil, nil }
func (m *MockPersonService) ValidateStaffPIN(_ context.Context, _ string) (*users.Staff, error) { return nil, nil }
func (m *MockPersonService) ValidateStaffPINForSpecificStaff(_ context.Context, _ int64, _ string) (*users.Staff, error) { return nil, nil }
func (m *MockPersonService) GetStudentsByTeacher(_ context.Context, _ int64) ([]*users.Student, error) { return nil, nil }
func (m *MockPersonService) GetStudentsWithGroupsByTeacher(_ context.Context, _ int64) ([]usersSvc.StudentWithGroup, error) { return nil, nil }
func (m *MockPersonService) WithTx(_ bun.Tx) interface{} { return nil }

func createTestDevice(status iot.DeviceStatus) *iot.Device {
	apiKey := "test-api-key-123"
	return &iot.Device{
		DeviceID:   "DEVICE-001",
		DeviceType: "rfid_reader",
		Status:     status,
		APIKey:     &apiKey,
	}
}

func TestDeviceFromCtx(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() context.Context
		expected *iot.Device
	}{
		{
			name: "device present in context",
			setup: func() context.Context {
				device := createTestDevice(iot.DeviceStatusActive)
				return context.WithValue(context.Background(), CtxDevice, device)
			},
			expected: createTestDevice(iot.DeviceStatusActive),
		},
		{
			name: "no device in context",
			setup: func() context.Context {
				return context.Background()
			},
			expected: nil,
		},
		{
			name: "wrong type in context",
			setup: func() context.Context {
				return context.WithValue(context.Background(), CtxDevice, "not-a-device")
			},
			expected: nil,
		},
		{
			name: "nil value in context",
			setup: func() context.Context {
				var device *iot.Device = nil
				return context.WithValue(context.Background(), CtxDevice, device)
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			result := DeviceFromCtx(ctx)

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.DeviceID, result.DeviceID)
			}
		})
	}
}

func TestStaffFromCtx(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() context.Context
		expected *users.Staff
	}{
		{
			name: "staff present in context",
			setup: func() context.Context {
				staff := &users.Staff{PersonID: 42}
				return context.WithValue(context.Background(), CtxStaff, staff)
			},
			expected: &users.Staff{PersonID: 42},
		},
		{
			name: "no staff in context",
			setup: func() context.Context {
				return context.Background()
			},
			expected: nil,
		},
		{
			name: "wrong type in context",
			setup: func() context.Context {
				return context.WithValue(context.Background(), CtxStaff, "not-a-staff")
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			result := StaffFromCtx(ctx)

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.PersonID, result.PersonID)
			}
		})
	}
}

func TestIsIoTDeviceRequest(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() context.Context
		expected bool
	}{
		{
			name: "is IoT device request",
			setup: func() context.Context {
				return context.WithValue(context.Background(), CtxIsIoTDevice, true)
			},
			expected: true,
		},
		{
			name: "not IoT device request",
			setup: func() context.Context {
				return context.WithValue(context.Background(), CtxIsIoTDevice, false)
			},
			expected: false,
		},
		{
			name: "no IoT flag in context",
			setup: func() context.Context {
				return context.Background()
			},
			expected: false,
		},
		{
			name: "wrong type in context",
			setup: func() context.Context {
				return context.WithValue(context.Background(), CtxIsIoTDevice, "true")
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			result := IsIoTDeviceRequest(ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSecureCompareStrings(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected bool
	}{
		{
			name:     "equal strings",
			a:        "password123",
			b:        "password123",
			expected: true,
		},
		{
			name:     "different strings",
			a:        "password123",
			b:        "password456",
			expected: false,
		},
		{
			name:     "same length different content",
			a:        "abcdefgh",
			b:        "12345678",
			expected: false,
		},
		{
			name:     "different lengths",
			a:        "short",
			b:        "longer string",
			expected: false,
		},
		{
			name:     "both empty",
			a:        "",
			b:        "",
			expected: true,
		},
		{
			name:     "one empty",
			a:        "",
			b:        "nonempty",
			expected: false,
		},
		{
			name:     "special characters",
			a:        "p@$$w0rd!#$%",
			b:        "p@$$w0rd!#$%",
			expected: true,
		},
		{
			name:     "unicode strings",
			a:        "пароль密码",
			b:        "пароль密码",
			expected: true,
		},
		{
			name:     "case sensitive",
			a:        "Password",
			b:        "password",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SecureCompareStrings(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeviceAuthenticator(t *testing.T) {
	// Save and restore original env var
	originalPIN := os.Getenv("OGS_DEVICE_PIN")
	defer func() {
		if originalPIN != "" {
			os.Setenv("OGS_DEVICE_PIN", originalPIN)
		} else {
			os.Unsetenv("OGS_DEVICE_PIN")
		}
	}()

	tests := []struct {
		name           string
		setupEnv       func()
		setupRequest   func(req *http.Request)
		mockIoT        *MockIoTService
		expectedStatus int
		checkContext   func(t *testing.T, device *iot.Device, isIoT bool)
	}{
		{
			name: "successful authentication",
			setupEnv: func() {
				os.Setenv("OGS_DEVICE_PIN", "1234")
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer valid-api-key")
				req.Header.Set("X-Staff-PIN", "1234")
			},
			mockIoT: &MockIoTService{
				GetDeviceByAPIKeyFunc: func(ctx context.Context, apiKey string) (*iot.Device, error) {
					return createTestDevice(iot.DeviceStatusActive), nil
				},
			},
			expectedStatus: http.StatusOK,
			checkContext: func(t *testing.T, device *iot.Device, isIoT bool) {
				assert.NotNil(t, device)
				assert.True(t, isIoT)
			},
		},
		{
			name: "missing Authorization header",
			setupEnv: func() {
				os.Setenv("OGS_DEVICE_PIN", "1234")
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("X-Staff-PIN", "1234")
			},
			mockIoT:        &MockIoTService{},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "invalid Authorization format",
			setupEnv: func() {
				os.Setenv("OGS_DEVICE_PIN", "1234")
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Basic some-credentials")
				req.Header.Set("X-Staff-PIN", "1234")
			},
			mockIoT:        &MockIoTService{},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "empty API key in Bearer",
			setupEnv: func() {
				os.Setenv("OGS_DEVICE_PIN", "1234")
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer ")
				req.Header.Set("X-Staff-PIN", "1234")
			},
			mockIoT:        &MockIoTService{},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "missing X-Staff-PIN header",
			setupEnv: func() {
				os.Setenv("OGS_DEVICE_PIN", "1234")
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer valid-api-key")
			},
			mockIoT:        &MockIoTService{},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "OGS_DEVICE_PIN not configured",
			setupEnv: func() {
				os.Unsetenv("OGS_DEVICE_PIN")
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer valid-api-key")
				req.Header.Set("X-Staff-PIN", "1234")
			},
			mockIoT:        &MockIoTService{},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "invalid PIN",
			setupEnv: func() {
				os.Setenv("OGS_DEVICE_PIN", "1234")
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer valid-api-key")
				req.Header.Set("X-Staff-PIN", "wrong-pin")
			},
			mockIoT:        &MockIoTService{},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "invalid API key (service error)",
			setupEnv: func() {
				os.Setenv("OGS_DEVICE_PIN", "1234")
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer invalid-api-key")
				req.Header.Set("X-Staff-PIN", "1234")
			},
			mockIoT: &MockIoTService{
				GetDeviceByAPIKeyFunc: func(ctx context.Context, apiKey string) (*iot.Device, error) {
					return nil, errors.New("device not found")
				},
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "device not found (nil device)",
			setupEnv: func() {
				os.Setenv("OGS_DEVICE_PIN", "1234")
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer unknown-key")
				req.Header.Set("X-Staff-PIN", "1234")
			},
			mockIoT: &MockIoTService{
				GetDeviceByAPIKeyFunc: func(ctx context.Context, apiKey string) (*iot.Device, error) {
					return nil, nil
				},
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "inactive device",
			setupEnv: func() {
				os.Setenv("OGS_DEVICE_PIN", "1234")
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer valid-api-key")
				req.Header.Set("X-Staff-PIN", "1234")
			},
			mockIoT: &MockIoTService{
				GetDeviceByAPIKeyFunc: func(ctx context.Context, apiKey string) (*iot.Device, error) {
					return createTestDevice(iot.DeviceStatusInactive), nil
				},
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "device update fails (should not block request)",
			setupEnv: func() {
				os.Setenv("OGS_DEVICE_PIN", "1234")
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer valid-api-key")
				req.Header.Set("X-Staff-PIN", "1234")
			},
			mockIoT: &MockIoTService{
				GetDeviceByAPIKeyFunc: func(ctx context.Context, apiKey string) (*iot.Device, error) {
					return createTestDevice(iot.DeviceStatusActive), nil
				},
				UpdateDeviceFunc: func(ctx context.Context, device *iot.Device) error {
					return errors.New("update failed")
				},
			},
			expectedStatus: http.StatusOK, // Should still succeed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()

			var capturedDevice *iot.Device
			var capturedIsIoT bool

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedDevice = DeviceFromCtx(r.Context())
				capturedIsIoT = IsIoTDeviceRequest(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			r := chi.NewRouter()
			r.Use(DeviceAuthenticator(tt.mockIoT, &MockPersonService{}))
			r.Get("/", handler)

			req := httptest.NewRequest("GET", "/", nil)
			tt.setupRequest(req)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.checkContext != nil && rr.Code == http.StatusOK {
				tt.checkContext(t, capturedDevice, capturedIsIoT)
			}
		})
	}
}

func TestDeviceOnlyAuthenticator(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func(req *http.Request)
		mockIoT        *MockIoTService
		expectedStatus int
		checkContext   func(t *testing.T, device *iot.Device)
	}{
		{
			name: "successful authentication",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer valid-api-key")
			},
			mockIoT: &MockIoTService{
				GetDeviceByAPIKeyFunc: func(ctx context.Context, apiKey string) (*iot.Device, error) {
					return createTestDevice(iot.DeviceStatusActive), nil
				},
			},
			expectedStatus: http.StatusOK,
			checkContext: func(t *testing.T, device *iot.Device) {
				assert.NotNil(t, device)
				assert.Equal(t, "DEVICE-001", device.DeviceID)
			},
		},
		{
			name: "missing Authorization header",
			setupRequest: func(req *http.Request) {
				// No header
			},
			mockIoT:        &MockIoTService{},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "invalid Authorization format",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "ApiKey some-key")
			},
			mockIoT:        &MockIoTService{},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "empty API key",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer ")
			},
			mockIoT:        &MockIoTService{},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "API key not found",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer unknown-key")
			},
			mockIoT: &MockIoTService{
				GetDeviceByAPIKeyFunc: func(ctx context.Context, apiKey string) (*iot.Device, error) {
					return nil, nil
				},
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "service error",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer some-key")
			},
			mockIoT: &MockIoTService{
				GetDeviceByAPIKeyFunc: func(ctx context.Context, apiKey string) (*iot.Device, error) {
					return nil, errors.New("database error")
				},
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "inactive device",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer valid-api-key")
			},
			mockIoT: &MockIoTService{
				GetDeviceByAPIKeyFunc: func(ctx context.Context, apiKey string) (*iot.Device, error) {
					return createTestDevice(iot.DeviceStatusInactive), nil
				},
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "device update fails (should not block request)",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer valid-api-key")
			},
			mockIoT: &MockIoTService{
				GetDeviceByAPIKeyFunc: func(ctx context.Context, apiKey string) (*iot.Device, error) {
					return createTestDevice(iot.DeviceStatusActive), nil
				},
				UpdateDeviceFunc: func(ctx context.Context, device *iot.Device) error {
					return errors.New("update failed")
				},
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedDevice *iot.Device

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedDevice = DeviceFromCtx(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			r := chi.NewRouter()
			r.Use(DeviceOnlyAuthenticator(tt.mockIoT))
			r.Get("/", handler)

			req := httptest.NewRequest("GET", "/", nil)
			tt.setupRequest(req)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.checkContext != nil && rr.Code == http.StatusOK {
				tt.checkContext(t, capturedDevice)
			}
		})
	}
}

func TestCtxKeyConstants(t *testing.T) {
	// Verify context key constants are distinct
	assert.NotEqual(t, CtxDevice, CtxStaff)
	assert.NotEqual(t, CtxDevice, CtxIsIoTDevice)
	assert.NotEqual(t, CtxStaff, CtxIsIoTDevice)
}
