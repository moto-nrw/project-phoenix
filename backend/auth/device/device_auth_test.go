package device

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The HTTP status codes the wire contract promises. Spelled out so this
// package's tests stay free of net/http.
const (
	statusUnauthorized = 401
	statusForbidden    = 403
)

// =============================================================================
// Port doubles
// =============================================================================

type mockDeviceDirectory struct {
	mu             sync.Mutex
	devices        map[string]*AuthenticatedDevice
	findErr        error
	updateCalled   bool
	updateError    error
	updateCount    int
	lastSeenWrites []time.Time
	updateStarted  chan struct{}
	updateBlock    chan struct{}
}

func newMockDeviceDirectory() *mockDeviceDirectory {
	return &mockDeviceDirectory{devices: make(map[string]*AuthenticatedDevice)}
}

// testTenantID is the tenant every device without an explicit tenant belongs
// to in these tests.
const testTenantID int64 = 7

func (m *mockDeviceDirectory) addDevice(apiKey string, device *AuthenticatedDevice) {
	if device.TenantID == 0 {
		device.TenantID = testTenantID
	}
	m.devices[apiKey] = device
}

func (m *mockDeviceDirectory) FindByAPIKey(_ context.Context, apiKey string) (*AuthenticatedDevice, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	device, ok := m.devices[apiKey]
	if !ok {
		return nil, errors.New("device not found")
	}
	return device, nil
}

func (m *mockDeviceDirectory) RecordLastSeen(_ context.Context, _ int64, lastSeen time.Time) error {
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

func (m *mockDeviceDirectory) wasUpdated() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateCalled
}

func (m *mockDeviceDirectory) resetUpdated() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCalled = false
}

// nilDeviceDirectory returns a nil device without an error.
type nilDeviceDirectory struct{ *mockDeviceDirectory }

func (nilDeviceDirectory) FindByAPIKey(_ context.Context, _ string) (*AuthenticatedDevice, error) {
	return nil, nil
}

func newNilDeviceDirectory() nilDeviceDirectory {
	return nilDeviceDirectory{mockDeviceDirectory: newMockDeviceDirectory()}
}

type stubSchoolLookup struct {
	deleted bool
	err     error
	calls   int
}

func (s *stubSchoolLookup) IsSchoolDeleted(_ context.Context, _ int64) (bool, error) {
	s.calls++
	return s.deleted, s.err
}

type stubStaffPINAuthenticator struct {
	staff    *AuthenticatedStaff
	err      error
	tenantID int64
	staffID  int64
	pin      string
	calls    int
}

func (s *stubStaffPINAuthenticator) AuthenticateStaffPIN(_ context.Context, tenantID, staffID int64, pin string) (*AuthenticatedStaff, error) {
	s.calls++
	s.tenantID = tenantID
	s.staffID = staffID
	s.pin = pin
	return s.staff, s.err
}

type tenantKey struct{}

// bindTenant is the test tenant binder: it records the tenant on the context
// and rejects non-positive ids like the runtime does.
func bindTenant(ctx context.Context, tenantID int64) (context.Context, error) {
	if tenantID <= 0 {
		return nil, fmt.Errorf("invalid tenant id %d", tenantID)
	}
	return context.WithValue(ctx, tenantKey{}, tenantID), nil
}

func boundTenant(ctx context.Context) int64 {
	id, _ := ctx.Value(tenantKey{}).(int64)
	return id
}

func activeDevice(deviceID string) *AuthenticatedDevice {
	return &AuthenticatedDevice{DeviceID: deviceID, DeviceType: "terminal", Status: activeStatus}
}

func newTestAuthenticator(directory DeviceDirectory, options ...func(*Dependencies)) *Authenticator {
	deps := Dependencies{Devices: directory, BindTenant: bindTenant}
	for _, option := range options {
		option(&deps)
	}
	return NewAuthenticator(deps)
}

func withSchools(schools SchoolLookup) func(*Dependencies) {
	return func(deps *Dependencies) { deps.Schools = schools }
}

func withStaffPIN(authenticator StaffPINAuthenticator) func(*Dependencies) {
	return func(deps *Dependencies) { deps.StaffPIN = authenticator }
}

func withPIN(resolver PINResolver, fallback string) func(*Dependencies) {
	return func(deps *Dependencies) {
		deps.PIN = resolver
		deps.FallbackPIN = fallback
	}
}

func bearer(apiKey string) credentials { return credentials{authorization: "Bearer " + apiKey} }

func pinCredentials(apiKey, pin string) credentials {
	c := bearer(apiKey)
	c.devicePIN = pin
	return c
}

// =============================================================================
// Context helpers
// =============================================================================

func TestDeviceFromCtx(t *testing.T) {
	t.Parallel()

	device := &AuthenticatedDevice{DeviceID: "device-001", DeviceType: "terminal", Status: activeStatus}
	result := DeviceFromCtx(context.WithValue(context.Background(), CtxDevice, device))
	require.NotNil(t, result)
	assert.Equal(t, "device-001", result.DeviceID)
	assert.Equal(t, "terminal", result.DeviceType)
	assert.True(t, result.IsActive())

	assert.Nil(t, DeviceFromCtx(context.Background()))
	assert.Nil(t, DeviceFromCtx(context.WithValue(context.Background(), CtxDevice, "not a device")))
}

func TestStaffFromCtx(t *testing.T) {
	t.Parallel()

	staff := &AuthenticatedStaff{ID: 42, TenantID: 7}
	result := StaffFromCtx(context.WithValue(context.Background(), CtxStaff, staff))
	require.NotNil(t, result)
	assert.Equal(t, int64(42), result.ID)

	assert.Nil(t, StaffFromCtx(context.Background()))
	assert.Nil(t, StaffFromCtx(context.WithValue(context.Background(), CtxStaff, "not a staff")))
}

func TestIsIoTDeviceRequest(t *testing.T) {
	t.Parallel()

	assert.True(t, IsIoTDeviceRequest(context.WithValue(context.Background(), CtxIsIoTDevice, true)))
	assert.False(t, IsIoTDeviceRequest(context.WithValue(context.Background(), CtxIsIoTDevice, false)))
	assert.False(t, IsIoTDeviceRequest(context.Background()))
	assert.False(t, IsIoTDeviceRequest(context.WithValue(context.Background(), CtxIsIoTDevice, "true")))
}

func TestCtxKey_DistinctValues(t *testing.T) {
	t.Parallel()

	assert.NotEqual(t, CtxDevice, CtxStaff)
	assert.NotEqual(t, CtxDevice, CtxIsIoTDevice)
	assert.NotEqual(t, CtxStaff, CtxIsIoTDevice)
}

func TestAuthenticatedDevice_IsActive(t *testing.T) {
	t.Parallel()

	assert.True(t, (&AuthenticatedDevice{Status: "active"}).IsActive())
	assert.False(t, (&AuthenticatedDevice{Status: "inactive"}).IsActive())
	assert.False(t, (&AuthenticatedDevice{Status: "offline"}).IsActive())
	assert.False(t, (&AuthenticatedDevice{Status: "maintenance"}).IsActive())
	assert.False(t, (*AuthenticatedDevice)(nil).IsActive())
}

// =============================================================================
// SecureCompareStrings
// =============================================================================

func TestSecureCompareStrings(t *testing.T) {
	t.Parallel()

	assert.True(t, SecureCompareStrings("password", "password"))
	assert.True(t, SecureCompareStrings("", ""))
	assert.True(t, SecureCompareStrings("a very long string with special chars!@#$%", "a very long string with special chars!@#$%"))

	assert.False(t, SecureCompareStrings("password", "different"))
	assert.False(t, SecureCompareStrings("password", "Password"))
	assert.False(t, SecureCompareStrings("password", "password "))
	assert.False(t, SecureCompareStrings("", "notempty"))
	assert.False(t, SecureCompareStrings("short", "muchlongerstring"))
	assert.False(t, SecureCompareStrings("muchlongerstring", "short"))
	assert.False(t, SecureCompareStrings("correct-pin-12345", "correct-pin-12346"))
}

// =============================================================================
// Construction
// =============================================================================

func TestNewAuthenticator_RequiresPorts(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() { NewAuthenticator(Dependencies{BindTenant: bindTenant}) })
	assert.Panics(t, func() { NewAuthenticator(Dependencies{Devices: newMockDeviceDirectory()}) })
	assert.NotPanics(t, func() { newTestAuthenticator(newMockDeviceDirectory()) })
}

// =============================================================================
// Device-only authentication
// =============================================================================

func TestDeviceOnly_ValidAPIKey(t *testing.T) {
	t.Parallel()

	directory := newMockDeviceDirectory()
	device := activeDevice("device-001")
	device.TenantID = 42
	directory.addDevice("valid-api-key-123", device)

	ctx, errResp := newTestAuthenticator(directory).authenticateDeviceOnly(context.Background(), bearer("valid-api-key-123"))
	require.Nil(t, errResp)

	ctxDevice := DeviceFromCtx(ctx)
	require.NotNil(t, ctxDevice)
	assert.Equal(t, "device-001", ctxDevice.DeviceID)
	assert.Equal(t, int64(42), boundTenant(ctx), "device tenant becomes the ambient tenant")
	assert.False(t, IsIoTDeviceRequest(ctx), "device-only requests are not IoT-PIN requests")
	assert.Nil(t, StaffFromCtx(ctx))
	assert.True(t, directory.wasUpdated(), "should update device last seen")
	assert.NotNil(t, ctxDevice.LastSeen)
}

func TestDeviceOnly_MissingAuthHeader(t *testing.T) {
	t.Parallel()

	_, errResp := newTestAuthenticator(newMockDeviceDirectory()).authenticateDeviceOnly(context.Background(), credentials{})
	require.NotNil(t, errResp)
	assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, ErrMissingAPIKey.Error(), errResp.ErrorText)
}

func TestDeviceOnly_InvalidAuthFormat(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		header  string
		wantErr error
	}{
		{"No Bearer prefix", "api-key-123", ErrInvalidAPIKeyFormat},
		{"Basic instead of Bearer", "Basic api-key-123", ErrInvalidAPIKeyFormat},
		{"Empty Bearer", "Bearer ", ErrMissingAPIKey},
		{"Lowercase bearer", "bearer api-key-123", ErrInvalidAPIKeyFormat},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, errResp := newTestAuthenticator(newMockDeviceDirectory()).authenticateDeviceOnly(context.Background(), credentials{authorization: tc.header})
			require.NotNil(t, errResp)
			assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)
			assert.Equal(t, tc.wantErr.Error(), errResp.ErrorText)
		})
	}
}

func TestDeviceOnly_InvalidAPIKey(t *testing.T) {
	t.Parallel()

	_, errResp := newTestAuthenticator(newMockDeviceDirectory()).authenticateDeviceOnly(context.Background(), bearer("invalid-api-key"))
	require.NotNil(t, errResp)
	assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, ErrInvalidAPIKey.Error(), errResp.ErrorText)
}

func TestDeviceOnly_DirectoryErrorIsInvalidAPIKey(t *testing.T) {
	t.Parallel()

	directory := newMockDeviceDirectory()
	directory.findErr = errors.New("database unavailable")

	_, errResp := newTestAuthenticator(directory).authenticateDeviceOnly(context.Background(), bearer("any-key"))
	require.NotNil(t, errResp)
	assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, ErrInvalidAPIKey.Error(), errResp.ErrorText, "read failures are not distinguishable from unknown keys on the wire")
}

func TestDeviceOnly_NilDeviceReturn(t *testing.T) {
	t.Parallel()

	_, errResp := newTestAuthenticator(newNilDeviceDirectory()).authenticateDeviceOnly(context.Background(), bearer("some-key"))
	require.NotNil(t, errResp)
	assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)
}

func TestDeviceOnly_InactiveDeviceStates(t *testing.T) {
	t.Parallel()

	for _, status := range []string{"inactive", "offline", "maintenance"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			directory := newMockDeviceDirectory()
			device := activeDevice("device-001")
			device.Status = status
			directory.addDevice("valid-api-key-123", device)

			_, errResp := newTestAuthenticator(directory).authenticateDeviceOnly(context.Background(), bearer("valid-api-key-123"))
			require.NotNil(t, errResp)
			assert.Equal(t, statusForbidden, errResp.HTTPStatusCode)
			assert.Equal(t, ErrDeviceInactive.Error(), errResp.ErrorText)
			assert.False(t, directory.wasUpdated(), "rejected devices are not marked as seen")
		})
	}
}

func TestAuthenticators_ZeroTenantID_IsRejected(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		run  func(*Authenticator, context.Context, credentials) (context.Context, *ErrResponse)
	}{
		{"device only", (*Authenticator).authenticateDeviceOnly},
		{"device and PIN", (*Authenticator).authenticateDevice},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := newMockDeviceDirectory()
			directory.devices["zero-tenant-api-key"] = activeDevice("device-zero-tenant")
			var observedTenant int64 = -1
			authenticator := newTestAuthenticator(directory, func(deps *Dependencies) {
				deps.BindTenant = func(ctx context.Context, tenantID int64) (context.Context, error) {
					observedTenant = tenantID
					return bindTenant(ctx, tenantID)
				}
			})

			ctx, errResp := tc.run(authenticator, context.Background(), pinCredentials("zero-tenant-api-key", "any"))
			require.NotNil(t, errResp)
			assert.Nil(t, ctx)
			assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)
			assert.Equal(t, ErrInvalidAPIKey.Error(), errResp.ErrorText)
			assert.Equal(t, int64(0), observedTenant, "the binder sees the zero tenant and reports it")
			assert.False(t, directory.wasUpdated())
		})
	}
}

// =============================================================================
// Device + PIN authentication
// =============================================================================

func TestDevice_ValidAPIKeyAndPIN(t *testing.T) {
	t.Parallel()

	directory := newMockDeviceDirectory()
	directory.addDevice("valid-api-key-123", activeDevice("device-001"))
	authenticator := newTestAuthenticator(directory, withPIN(nil, "test-device-pin-123"))

	ctx, errResp := authenticator.authenticateDevice(context.Background(), pinCredentials("valid-api-key-123", "test-device-pin-123"))
	require.Nil(t, errResp)

	ctxDevice := DeviceFromCtx(ctx)
	require.NotNil(t, ctxDevice)
	assert.Equal(t, "device-001", ctxDevice.DeviceID)
	assert.True(t, IsIoTDeviceRequest(ctx))
	assert.Nil(t, StaffFromCtx(ctx))
	assert.Equal(t, testTenantID, boundTenant(ctx))
	assert.True(t, directory.wasUpdated(), "should update device last seen")
}

func TestDevice_MissingPIN(t *testing.T) {
	t.Parallel()

	directory := newMockDeviceDirectory()
	directory.addDevice("valid-api-key-123", activeDevice("device-001"))
	authenticator := newTestAuthenticator(directory, withPIN(nil, "test-pin"))

	_, errResp := authenticator.authenticateDevice(context.Background(), bearer("valid-api-key-123"))
	require.NotNil(t, errResp)
	assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, ErrMissingPIN.Error(), errResp.ErrorText)
}

func TestDevice_InvalidPIN(t *testing.T) {
	t.Parallel()

	directory := newMockDeviceDirectory()
	directory.addDevice("valid-api-key-123", activeDevice("device-001"))
	authenticator := newTestAuthenticator(directory, withPIN(nil, "correct-pin"))

	_, errResp := authenticator.authenticateDevice(context.Background(), pinCredentials("valid-api-key-123", "wrong-pin"))
	require.NotNil(t, errResp)
	assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, ErrInvalidPIN.Error(), errResp.ErrorText)
	assert.False(t, directory.wasUpdated(), "a rejected PIN must not mark the device as seen")
}

func TestDevice_MissingOGSPINConfig(t *testing.T) {
	t.Parallel()

	directory := newMockDeviceDirectory()
	directory.addDevice("valid-api-key-123", activeDevice("device-001"))
	authenticator := newTestAuthenticator(directory, withPIN(nil, ""))

	_, errResp := authenticator.authenticateDevice(context.Background(), pinCredentials("valid-api-key-123", "any-pin"))
	require.NotNil(t, errResp)
	assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, ErrInvalidPIN.Error(), errResp.ErrorText)
}

func TestDevice_MissingAPIKey(t *testing.T) {
	t.Parallel()

	authenticator := newTestAuthenticator(newMockDeviceDirectory(), withPIN(nil, "test-pin"))
	_, errResp := authenticator.authenticateDevice(context.Background(), credentials{devicePIN: "test-pin"})
	require.NotNil(t, errResp)
	assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, ErrMissingAPIKey.Error(), errResp.ErrorText)
}

func TestDevice_InvalidAPIKey(t *testing.T) {
	t.Parallel()

	authenticator := newTestAuthenticator(newMockDeviceDirectory(), withPIN(nil, "test-pin"))
	_, errResp := authenticator.authenticateDevice(context.Background(), pinCredentials("invalid-key", "test-pin"))
	require.NotNil(t, errResp)
	assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, ErrInvalidAPIKey.Error(), errResp.ErrorText)
}

func TestDevice_InactiveDevice(t *testing.T) {
	t.Parallel()

	directory := newMockDeviceDirectory()
	device := activeDevice("device-001")
	device.Status = "inactive"
	directory.addDevice("valid-api-key-123", device)
	authenticator := newTestAuthenticator(directory, withPIN(nil, "test-pin"))

	_, errResp := authenticator.authenticateDevice(context.Background(), pinCredentials("valid-api-key-123", "test-pin"))
	require.NotNil(t, errResp)
	assert.Equal(t, statusForbidden, errResp.HTTPStatusCode)
	assert.Equal(t, ErrDeviceInactive.Error(), errResp.ErrorText)
}

func TestDevice_NilDeviceReturn(t *testing.T) {
	t.Parallel()

	authenticator := newTestAuthenticator(newNilDeviceDirectory(), withPIN(nil, "test-pin"))
	_, errResp := authenticator.authenticateDevice(context.Background(), pinCredentials("some-key", "test-pin"))
	require.NotNil(t, errResp)
	assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)
}

// =============================================================================
// PIN resolver wiring
// =============================================================================

func TestDevice_UsesPINResolver(t *testing.T) {
	t.Parallel()

	directory := newMockDeviceDirectory()
	device := activeDevice("device-resolver")
	device.TenantID = 42
	directory.addDevice("valid-api-key-resolver", device)

	resolverCalled := false
	resolver := func(_ context.Context, tenantID int64) string {
		resolverCalled = true
		if tenantID == 42 {
			return "9999"
		}
		return ""
	}
	authenticator := newTestAuthenticator(directory, withPIN(resolver, ""))

	_, errResp := authenticator.authenticateDevice(context.Background(), pinCredentials("valid-api-key-resolver", "9999"))
	assert.Nil(t, errResp, "should authenticate using PIN from resolver")
	assert.True(t, resolverCalled, "PIN resolver should have been called")
}

func TestDevice_PINResolverFallsBackToConfiguredPIN(t *testing.T) {
	t.Parallel()

	directory := newMockDeviceDirectory()
	device := activeDevice("device-fallback")
	device.TenantID = 42
	directory.addDevice("valid-api-key-fallback", device)
	resolver := func(_ context.Context, _ int64) string { return "" }
	authenticator := newTestAuthenticator(directory, withPIN(resolver, "env-pin"))

	_, errResp := authenticator.authenticateDevice(context.Background(), pinCredentials("valid-api-key-fallback", "env-pin"))
	assert.Nil(t, errResp, "should fall back to OGS_DEVICE_PIN env var")
}

func TestDevice_PINResolverWrongPIN(t *testing.T) {
	t.Parallel()

	directory := newMockDeviceDirectory()
	device := activeDevice("device-wrong")
	device.TenantID = 42
	directory.addDevice("valid-api-key-wrong", device)
	resolver := func(_ context.Context, _ int64) string { return "9999" }
	authenticator := newTestAuthenticator(directory, withPIN(resolver, "env-pin"))

	_, errResp := authenticator.authenticateDevice(context.Background(), pinCredentials("valid-api-key-wrong", "env-pin"))
	require.NotNil(t, errResp, "the env PIN is not accepted once the tenant PIN resolves")
	assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)

	_, errResp = authenticator.authenticateDevice(context.Background(), pinCredentials("valid-api-key-wrong", "0000"))
	require.NotNil(t, errResp, "wrong PIN should be rejected")
	assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)
}

// =============================================================================
// Staff PIN binding
// =============================================================================

func staffCredentials(apiKey, staffID, staffPIN string) credentials {
	c := pinCredentials(apiKey, "device-pin")
	c.staffID = staffID
	c.staffPIN = staffPIN
	return c
}

func newStaffPINAuthenticator(authenticator StaffPINAuthenticator) (*Authenticator, string) {
	directory := newMockDeviceDirectory()
	const apiKey = "staff-pin-auth-api-key"
	directory.addDevice(apiKey, activeDevice("test-device"))
	return newTestAuthenticator(directory, withPIN(nil, "device-pin"), withStaffPIN(authenticator)), apiKey
}

func TestDevice_SetsCredentialBoundStaffContext(t *testing.T) {
	t.Parallel()

	staff := &AuthenticatedStaff{ID: 42, TenantID: testTenantID}
	stub := &stubStaffPINAuthenticator{staff: staff}
	authenticator, apiKey := newStaffPINAuthenticator(stub)

	ctx, errResp := authenticator.authenticateDevice(context.Background(), staffCredentials(apiKey, "42", "personal-pin"))
	require.Nil(t, errResp)
	assert.Same(t, staff, StaffFromCtx(ctx))
	assert.Equal(t, 1, stub.calls)
	assert.Equal(t, testTenantID, stub.tenantID)
	assert.Equal(t, int64(42), stub.staffID)
	assert.Equal(t, "personal-pin", stub.pin)
}

func TestDevice_IgnoresLegacyStaffIDWithoutCredential(t *testing.T) {
	t.Parallel()

	stub := &stubStaffPINAuthenticator{}
	authenticator, apiKey := newStaffPINAuthenticator(stub)

	ctx, errResp := authenticator.authenticateDevice(context.Background(), staffCredentials(apiKey, "42", ""))
	require.Nil(t, errResp)
	assert.Nil(t, StaffFromCtx(ctx))
	assert.Zero(t, stub.calls)
}

func TestDevice_RejectsInvalidStaffCredential(t *testing.T) {
	t.Parallel()

	stub := &stubStaffPINAuthenticator{err: errors.New("invalid credential")}
	authenticator, apiKey := newStaffPINAuthenticator(stub)

	_, errResp := authenticator.authenticateDevice(context.Background(), staffCredentials(apiKey, "42", "wrong-pin"))
	require.NotNil(t, errResp)
	assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, ErrInvalidPIN.Error(), errResp.ErrorText)
	assert.Equal(t, 1, stub.calls)
}

func TestDevice_RejectsCrossTenantStaff(t *testing.T) {
	t.Parallel()

	stub := &stubStaffPINAuthenticator{staff: &AuthenticatedStaff{ID: 42, TenantID: 8}}
	authenticator, apiKey := newStaffPINAuthenticator(stub)

	_, errResp := authenticator.authenticateDevice(context.Background(), staffCredentials(apiKey, "42", "personal-pin"))
	require.NotNil(t, errResp)
	assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)
}

func TestDevice_RejectsStaffCredentialWithoutAuthenticator(t *testing.T) {
	t.Parallel()

	authenticator, apiKey := newStaffPINAuthenticator(nil)

	_, errResp := authenticator.authenticateDevice(context.Background(), staffCredentials(apiKey, "42", "personal-pin"))
	require.NotNil(t, errResp, "a personal credential cannot be verified without an authenticator")
	assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)
}

func TestDevice_RejectsMalformedStaffID(t *testing.T) {
	t.Parallel()

	stub := &stubStaffPINAuthenticator{staff: &AuthenticatedStaff{ID: 42, TenantID: 7}}
	authenticator, apiKey := newStaffPINAuthenticator(stub)

	for _, staffID := range []string{"", "abc", "0", "-1"} {
		_, errResp := authenticator.authenticateDevice(context.Background(), staffCredentials(apiKey, staffID, "personal-pin"))
		require.NotNil(t, errResp, "staff id %q", staffID)
		assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)
	}
	assert.Zero(t, stub.calls, "malformed ids never reach the verifier")
}

// =============================================================================
// School guard
// =============================================================================

func TestRejectDeletedSchool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schools    SchoolLookup
		wantReject bool
	}{
		{"active school", &stubSchoolLookup{}, false},
		{"soft-deleted school", &stubSchoolLookup{deleted: true}, true},
		{"nil lookup fails open", nil, false},
		{"school not found", &stubSchoolLookup{err: ErrSchoolNotFound}, true},
		{"wrapped not found", &stubSchoolLookup{err: fmt.Errorf("find: %w", ErrSchoolNotFound)}, true},
		{"transient lookup failure fails open", &stubSchoolLookup{err: ErrSchoolLookupUnavailable}, false},
		{"wrapped transient failure fails open", &stubSchoolLookup{err: fmt.Errorf("%w: dial tcp", ErrSchoolLookupUnavailable)}, false},
		{"other failure fails closed", &stubSchoolLookup{err: errors.New("permission denied")}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			device := &AuthenticatedDevice{DeviceID: "device-001", TenantID: 100}
			authenticator := newTestAuthenticator(newMockDeviceDirectory(), withSchools(tc.schools))

			result := authenticator.rejectDeletedSchool(context.Background(), device)
			if !tc.wantReject {
				assert.Nil(t, result)
				return
			}
			require.NotNil(t, result)
			assert.Equal(t, statusForbidden, result.HTTPStatusCode)
			assert.Equal(t, ErrDeviceInactive.Error(), result.ErrorText)
		})
	}
}

func TestRejectDeletedSchool_SkipsLookupWithoutTenant(t *testing.T) {
	t.Parallel()

	schools := &stubSchoolLookup{deleted: true}
	authenticator := newTestAuthenticator(newMockDeviceDirectory(), withSchools(schools))

	assert.Nil(t, authenticator.rejectDeletedSchool(context.Background(), &AuthenticatedDevice{DeviceID: "device-001"}))
	assert.Zero(t, schools.calls)
}

func TestDeviceOnly_DeletedSchool_Forbidden(t *testing.T) {
	t.Parallel()

	directory := newMockDeviceDirectory()
	device := activeDevice("device-deleted-school")
	device.TenantID = 100
	directory.addDevice("valid-api-key-deleted", device)
	authenticator := newTestAuthenticator(directory, withSchools(&stubSchoolLookup{deleted: true}))

	_, errResp := authenticator.authenticateDeviceOnly(context.Background(), bearer("valid-api-key-deleted"))
	require.NotNil(t, errResp, "devices belonging to deleted schools must be rejected")
	assert.Equal(t, statusForbidden, errResp.HTTPStatusCode)
	assert.False(t, directory.wasUpdated())
}

// =============================================================================
// Error responses
// =============================================================================

func TestErrDeviceUnauthorized(t *testing.T) {
	t.Parallel()

	errResp := ErrDeviceUnauthorized(ErrInvalidAPIKey)
	require.NotNil(t, errResp)
	assert.Equal(t, statusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.StatusText)
	assert.Equal(t, "invalid device API key", errResp.ErrorText)
	assert.Equal(t, ErrInvalidAPIKey, errResp.Err)
}

func TestErrDeviceForbidden(t *testing.T) {
	t.Parallel()

	errResp := ErrDeviceForbidden(ErrDeviceInactive)
	require.NotNil(t, errResp)
	assert.Equal(t, statusForbidden, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.StatusText)
	assert.Equal(t, "device is not active", errResp.ErrorText)
}

// TestErrorTypes pins the error strings PyrePortal maps to German UI text.
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
			t.Parallel()
			assert.Equal(t, tc.expected, tc.err.Error())
		})
	}
}

// =============================================================================
// Last-seen recording through the middlewares
// =============================================================================

func TestDeviceOnly_UpdateLastSeenError(t *testing.T) {
	t.Parallel()

	directory := newMockDeviceDirectory()
	directory.updateError = errors.New("database error")
	directory.addDevice("valid-api-key-123", activeDevice("device-001"))

	_, errResp := newTestAuthenticator(directory).authenticateDeviceOnly(context.Background(), bearer("valid-api-key-123"))
	assert.Nil(t, errResp, "a failed last-seen write is logged, not blocking")
}

func TestDeviceOnly_DebouncesLastSeenWrites(t *testing.T) {
	t.Parallel()

	directory := newMockDeviceDirectory()
	directory.addDevice("valid-api-key-123", activeDevice("device-001"))
	authenticator := newTestAuthenticator(directory)

	_, errResp := authenticator.authenticateDeviceOnly(context.Background(), bearer("valid-api-key-123"))
	require.Nil(t, errResp)
	assert.True(t, directory.wasUpdated(), "first request should update last seen")

	directory.resetUpdated()
	_, errResp = authenticator.authenticateDeviceOnly(context.Background(), bearer("valid-api-key-123"))
	require.Nil(t, errResp)
	assert.False(t, directory.wasUpdated(), "second request inside debounce window should skip last seen write")
}

func TestAuthenticators_ShareLastSeenDebouncer(t *testing.T) {
	t.Parallel()

	directory := newMockDeviceDirectory()
	const apiKey = "shared-debouncer-key"
	device := activeDevice("shared-debouncer-device")
	device.ID = 1
	directory.addDevice(apiKey, device)
	authenticator := newTestAuthenticator(directory, withPIN(nil, "pin"))

	_, errResp := authenticator.authenticateDeviceOnly(context.Background(), bearer(apiKey))
	require.Nil(t, errResp)
	assert.True(t, directory.wasUpdated())

	directory.resetUpdated()
	_, errResp = authenticator.authenticateDevice(context.Background(), pinCredentials(apiKey, "pin"))
	require.Nil(t, errResp)
	assert.False(t, directory.wasUpdated(), "both middlewares share one debouncer")
}

func TestNewAuthenticator_UsesSuppliedDebouncer(t *testing.T) {
	t.Parallel()

	directory := newMockDeviceDirectory()
	directory.addDevice("key", activeDevice("device"))
	debouncer := NewLastSeenDebouncer()

	first := NewAuthenticator(Dependencies{Devices: directory, BindTenant: bindTenant, LastSeen: debouncer})
	second := NewAuthenticator(Dependencies{Devices: directory, BindTenant: bindTenant, LastSeen: debouncer})

	_, errResp := first.authenticateDeviceOnly(context.Background(), bearer("key"))
	require.Nil(t, errResp)
	directory.resetUpdated()
	_, errResp = second.authenticateDeviceOnly(context.Background(), bearer("key"))
	require.Nil(t, errResp)
	assert.False(t, directory.wasUpdated(), "authenticators built over one debouncer share its state")
}
