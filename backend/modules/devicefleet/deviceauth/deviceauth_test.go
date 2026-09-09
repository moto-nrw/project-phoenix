package deviceauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/driver/pgdriver"
)

// =============================================================================
// Doubles
// =============================================================================

type fakeFleet struct {
	mu       sync.Mutex
	devices  map[string]devicefleet.Device
	findErr  error
	lastSeen map[int64]time.Time
	writeErr error
}

func newFakeFleet() *fakeFleet {
	return &fakeFleet{devices: map[string]devicefleet.Device{}, lastSeen: map[int64]time.Time{}}
}

// testTenantID is the tenant every device without an explicit tenant belongs
// to in these tests.
const testTenantID int64 = 7

func (f *fakeFleet) add(apiKey string, d devicefleet.Device) {
	if d.TenantID == 0 {
		d.TenantID = testTenantID
	}
	f.devices[apiKey] = d
}

func (f *fakeFleet) FindDeviceByAPIKey(_ context.Context, apiKey string) (devicefleet.Device, error) {
	if f.findErr != nil {
		return devicefleet.Device{}, f.findErr
	}
	d, ok := f.devices[apiKey]
	if !ok {
		return devicefleet.Device{}, devicefleet.ErrDeviceNotFound
	}
	return d, nil
}

func (f *fakeFleet) UpdateDeviceLastSeen(_ context.Context, id int64, seenAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writeErr != nil {
		return f.writeErr
	}
	f.lastSeen[id] = seenAt
	return nil
}

func (f *fakeFleet) writes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.lastSeen)
}

type fakeSchools struct {
	school *platform.School
	err    error
}

func (s fakeSchools) GetSchoolByID(_ context.Context, _ int64) (*platform.School, error) {
	return s.school, s.err
}

type fakeSettings struct {
	pins map[int64]string
	err  error
}

func (s fakeSettings) ResolveStringForTenant(_ context.Context, tenantID int64, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.pins[tenantID], nil
}

// staffRow stands in for the retained people-directory staff row.
type staffRow struct {
	id       int64
	tenantID int64
}

func (r *staffRow) GetID() any         { return r.id }
func (r *staffRow) GetTenantID() int64 { return r.tenantID }

type stubNetError struct{}

func (stubNetError) Error() string   { return "network error" }
func (stubNetError) Timeout() bool   { return true }
func (stubNetError) Temporary() bool { return true }

func activeDevice(id int64, deviceID string) devicefleet.Device {
	return devicefleet.Device{ID: id, DeviceID: deviceID, DeviceType: "terminal", Status: devicefleet.DeviceStatusActive}
}

func router(middleware Middleware, handler http.HandlerFunc) http.Handler {
	if handler == nil {
		handler = func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	}
	return middleware(handler)
}

func request(method, apiKey string, headers map[string]string) *http.Request {
	req := httptest.NewRequest(method, "/test", nil)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return req
}

func serve(handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

// =============================================================================
// Composition
// =============================================================================

func TestNew_RequiresFleet(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() { New(Dependencies{}) })
	assert.NotPanics(t, func() { New(Dependencies{Devices: newFakeFleet()}) })
}

func TestDeviceOnly_BindsPrincipalAndTenant(t *testing.T) {
	t.Parallel()

	fleet := newFakeFleet()
	name := "Eingang"
	d := activeDevice(11, "device-001")
	d.TenantID = 42
	d.Name = &name
	fleet.add("valid-api-key", d)

	var seen *device.AuthenticatedDevice
	var boundTenant int64
	handler := router(New(Dependencies{Devices: fleet}).DeviceOnly(), func(w http.ResponseWriter, r *http.Request) {
		seen = device.DeviceFromCtx(r.Context())
		boundTenant = tenant.FromContext(r.Context())
		assert.False(t, device.IsIoTDeviceRequest(r.Context()))
		w.WriteHeader(http.StatusOK)
	})

	rr := serve(handler, request(http.MethodGet, "valid-api-key", nil))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.NotNil(t, seen)
	assert.Equal(t, int64(11), seen.ID)
	assert.Equal(t, int64(42), seen.TenantID)
	assert.Equal(t, "device-001", seen.DeviceID)
	assert.Equal(t, "terminal", seen.DeviceType)
	assert.Equal(t, &name, seen.Name)
	assert.Equal(t, string(devicefleet.DeviceStatusActive), seen.Status)
	assert.True(t, seen.IsActive())
	assert.NotNil(t, seen.LastSeen, "the principal carries the observed instant")
	assert.Equal(t, int64(42), boundTenant, "the device tenant is the ambient tenant")
	assert.Equal(t, 1, fleet.writes(), "the owner records last_seen")
}

func TestDeviceOnly_WireContract(t *testing.T) {
	t.Parallel()

	fleet := newFakeFleet()
	fleet.add("active-key", activeDevice(1, "active"))
	inactive := activeDevice(2, "inactive")
	inactive.Status = devicefleet.DeviceStatusInactive
	fleet.add("inactive-key", inactive)
	handler := router(New(Dependencies{Devices: fleet}).DeviceOnly(), nil)

	tests := []struct {
		name     string
		header   string
		wantCode int
		wantBody string
	}{
		{"missing header", "", http.StatusUnauthorized, `"error":"device API key is required"`},
		{"basic scheme", "Basic active-key", http.StatusUnauthorized, `"error":"invalid API key format - use Bearer token"`},
		{"lowercase bearer", "bearer active-key", http.StatusUnauthorized, `"error":"invalid API key format - use Bearer token"`},
		{"empty bearer", "Bearer ", http.StatusUnauthorized, `"error":"device API key is required"`},
		{"unknown key", "Bearer unknown", http.StatusUnauthorized, `"error":"invalid device API key"`},
		{"inactive device", "Bearer inactive-key", http.StatusForbidden, `"error":"device is not active"`},
		{"active device", "Bearer active-key", http.StatusOK, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rr := serve(handler, req)
			assert.Equal(t, tc.wantCode, rr.Code)
			if tc.wantBody != "" {
				assert.Contains(t, rr.Body.String(), tc.wantBody)
				assert.Contains(t, rr.Body.String(), `"status":"error"`)
			}
		})
	}
}

func TestDeviceOnly_ReadFailureIsInvalidAPIKey(t *testing.T) {
	t.Parallel()

	fleet := newFakeFleet()
	fleet.findErr = errors.New("connection reset")
	rr := serve(router(New(Dependencies{Devices: fleet}).DeviceOnly(), nil), request(http.MethodGet, "any", nil))
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), `"error":"invalid device API key"`)
}

func TestDeviceOnly_ZeroTenantIsObservedAndRejected(t *testing.T) {
	t.Parallel()

	fleet := newFakeFleet()
	fleet.devices["zero-tenant"] = activeDevice(3, "zero-tenant")
	handler := router(New(Dependencies{Devices: fleet}).DeviceOnly(), func(http.ResponseWriter, *http.Request) {
		t.Fatal("zero-tenant device must not reach the wrapped handler")
	})

	rr := serve(handler, request(http.MethodGet, "zero-tenant", nil))
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), `"error":"invalid device API key"`)
	assert.Zero(t, fleet.writes())
}

func TestDeviceOnly_LastSeenWriteFailureDoesNotBlock(t *testing.T) {
	t.Parallel()

	fleet := newFakeFleet()
	fleet.writeErr = errors.New("database error")
	fleet.add("key", activeDevice(4, "device"))
	rr := serve(router(New(Dependencies{Devices: fleet}).DeviceOnly(), nil), request(http.MethodGet, "key", nil))
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthenticators_ShareLastSeenDebouncer(t *testing.T) {
	t.Parallel()

	fleet := newFakeFleet()
	fleet.add("shared", activeDevice(5, "shared"))
	authenticators := New(Dependencies{Devices: fleet, FallbackPIN: "1234"})

	rr := serve(router(authenticators.DeviceOnly(), nil), request(http.MethodGet, "shared", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 1, fleet.writes())

	rr = serve(router(authenticators.Device(), nil), request(http.MethodGet, "shared", map[string]string{"X-Staff-PIN": "1234"}))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 1, fleet.writes(), "the second request inside the window reuses the shared debounce state")
}

// =============================================================================
// Device + PIN
// =============================================================================

func TestDevice_PINResolution(t *testing.T) {
	t.Parallel()

	fleet := newFakeFleet()
	d := activeDevice(6, "kiosk")
	d.TenantID = 42
	fleet.add("kiosk-key", d)

	tests := []struct {
		name     string
		settings Settings
		fallback string
		pin      string
		wantCode int
	}{
		{"tenant pin from settings", fakeSettings{pins: map[int64]string{42: "9999"}}, "1234", "9999", http.StatusOK},
		{"fallback rejected once tenant pin resolves", fakeSettings{pins: map[int64]string{42: "9999"}}, "1234", "1234", http.StatusUnauthorized},
		{"empty tenant pin falls back", fakeSettings{pins: map[int64]string{}}, "1234", "1234", http.StatusOK},
		{"settings failure falls back", fakeSettings{err: errors.New("settings unavailable")}, "1234", "1234", http.StatusOK},
		{"no settings service uses fallback", nil, "1234", "1234", http.StatusOK},
		{"no pin configured anywhere", nil, "", "1234", http.StatusUnauthorized},
		{"wrong pin", fakeSettings{pins: map[int64]string{42: "9999"}}, "", "0000", http.StatusUnauthorized},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handler := router(New(Dependencies{Devices: fleet, Settings: tc.settings, FallbackPIN: tc.fallback}).Device(), func(w http.ResponseWriter, r *http.Request) {
				assert.True(t, device.IsIoTDeviceRequest(r.Context()))
				assert.Equal(t, int64(42), tenant.FromContext(r.Context()))
				w.WriteHeader(http.StatusOK)
			})
			rr := serve(handler, request(http.MethodPost, "kiosk-key", map[string]string{"X-Staff-PIN": tc.pin}))
			assert.Equal(t, tc.wantCode, rr.Code, rr.Body.String())
			if tc.wantCode != http.StatusOK {
				assert.Contains(t, rr.Body.String(), `"error":"invalid staff PIN"`)
			}
		})
	}
}

func TestDevice_MissingPIN(t *testing.T) {
	t.Parallel()

	fleet := newFakeFleet()
	fleet.add("kiosk-key", activeDevice(7, "kiosk"))
	rr := serve(router(New(Dependencies{Devices: fleet, FallbackPIN: "1234"}).Device(), nil), request(http.MethodPost, "kiosk-key", nil))
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), `"error":"staff PIN is required"`)
}

// =============================================================================
// Staff PIN adapter
// =============================================================================

func TestStaffPIN_BindsVerifiedStaff(t *testing.T) {
	t.Parallel()

	fleet := newFakeFleet()
	fleet.add("kiosk-key", activeDevice(8, "kiosk"))
	var verified struct {
		tenantID, staffID int64
		pin               string
	}
	verify := func(_ context.Context, tenantID, staffID int64, pin string) (*staffRow, error) {
		verified.tenantID, verified.staffID, verified.pin = tenantID, staffID, pin
		return &staffRow{id: staffID, tenantID: tenantID}, nil
	}
	var staff *device.AuthenticatedStaff
	handler := router(New(Dependencies{Devices: fleet, FallbackPIN: "1234", StaffPIN: StaffPIN(verify)}).Device(), func(w http.ResponseWriter, r *http.Request) {
		staff = device.StaffFromCtx(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	rr := serve(handler, request(http.MethodPost, "kiosk-key", map[string]string{
		"X-Staff-PIN": "1234", "X-Staff-ID": "42", "X-Staff-Auth-PIN": "personal",
	}))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.NotNil(t, staff)
	assert.Equal(t, int64(42), staff.ID)
	assert.Equal(t, testTenantID, staff.TenantID)
	assert.Equal(t, testTenantID, verified.tenantID)
	assert.Equal(t, int64(42), verified.staffID)
	assert.Equal(t, "personal", verified.pin)
}

func TestStaffPIN_Rejections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		verify func(context.Context, int64, int64, string) (*staffRow, error)
	}{
		{"verification error", func(context.Context, int64, int64, string) (*staffRow, error) {
			return nil, errors.New("invalid credential")
		}},
		{"nil row", func(context.Context, int64, int64, string) (*staffRow, error) { return nil, nil }},
		{"cross-tenant row", func(_ context.Context, _, staffID int64, _ string) (*staffRow, error) {
			return &staffRow{id: staffID, tenantID: 8}, nil
		}},
		{"unusable id", func(context.Context, int64, int64, string) (*staffRow, error) {
			return &staffRow{id: 0, tenantID: 7}, nil
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fleet := newFakeFleet()
			fleet.add("kiosk-key", activeDevice(9, "kiosk"))
			handler := router(New(Dependencies{Devices: fleet, FallbackPIN: "1234", StaffPIN: StaffPIN(tc.verify)}).Device(), func(http.ResponseWriter, *http.Request) {
				t.Fatal("unverified staff must not reach the handler")
			})
			rr := serve(handler, request(http.MethodPost, "kiosk-key", map[string]string{
				"X-Staff-PIN": "1234", "X-Staff-ID": "42", "X-Staff-Auth-PIN": "personal",
			}))
			assert.Equal(t, http.StatusUnauthorized, rr.Code)
			assert.Contains(t, rr.Body.String(), `"error":"invalid staff PIN"`)
		})
	}
}

func TestStaffPIN_NilVerifier(t *testing.T) {
	t.Parallel()

	var verify func(context.Context, int64, int64, string) (*staffRow, error)
	assert.Nil(t, StaffPIN(verify))
}

func TestDevice_LegacyStaffIDWithoutCredentialIsIgnored(t *testing.T) {
	t.Parallel()

	fleet := newFakeFleet()
	fleet.add("kiosk-key", activeDevice(10, "kiosk"))
	calls := 0
	verify := func(context.Context, int64, int64, string) (*staffRow, error) {
		calls++
		return &staffRow{id: 42, tenantID: 7}, nil
	}
	handler := router(New(Dependencies{Devices: fleet, FallbackPIN: "1234", StaffPIN: StaffPIN(verify)}).Device(), func(w http.ResponseWriter, r *http.Request) {
		assert.Nil(t, device.StaffFromCtx(r.Context()))
		w.WriteHeader(http.StatusOK)
	})
	rr := serve(handler, request(http.MethodPost, "kiosk-key", map[string]string{"X-Staff-PIN": "1234", "X-Staff-ID": "42"}))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Zero(t, calls)
}

// =============================================================================
// School guard
// =============================================================================

func TestSchoolLookup_Classification(t *testing.T) {
	t.Parallel()

	now := time.Now()
	tests := []struct {
		name        string
		schools     fakeSchools
		wantDeleted bool
		wantErr     error
		wantOther   bool
	}{
		{"active school", fakeSchools{school: &platform.School{Active: true}}, false, nil, false},
		{"soft-deleted school", fakeSchools{school: &platform.School{DeletedAt: &now}}, true, nil, false},
		{"nil school", fakeSchools{}, false, device.ErrSchoolNotFound, false},
		{"sql.ErrNoRows", fakeSchools{err: sql.ErrNoRows}, false, device.ErrSchoolNotFound, false},
		{"wrapped sql.ErrNoRows", fakeSchools{err: fmt.Errorf("find: %w", sql.ErrNoRows)}, false, device.ErrSchoolNotFound, false},
		{"context deadline", fakeSchools{err: context.DeadlineExceeded}, false, device.ErrSchoolLookupUnavailable, false},
		{"wrapped context cancel", fakeSchools{err: fmt.Errorf("find: %w", context.Canceled)}, false, device.ErrSchoolLookupUnavailable, false},
		{"net error", fakeSchools{err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}}, false, device.ErrSchoolLookupUnavailable, false},
		{"net error stub", fakeSchools{err: stubNetError{}}, false, device.ErrSchoolLookupUnavailable, false},
		{"driver error without a connection-class sqlstate", fakeSchools{err: pgdriver.Error{}}, false, nil, true},
		{"permission error", fakeSchools{err: errors.New("permission denied")}, false, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deleted, err := schoolLookup{schools: tc.schools}.IsSchoolDeleted(context.Background(), 100)
			assert.Equal(t, tc.wantDeleted, deleted)
			switch {
			case tc.wantErr != nil:
				require.ErrorIs(t, err, tc.wantErr)
			case tc.wantOther:
				require.Error(t, err)
				assert.NotErrorIs(t, err, device.ErrSchoolNotFound)
				assert.NotErrorIs(t, err, device.ErrSchoolLookupUnavailable)
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestIsTransientDBErr(t *testing.T) {
	t.Parallel()

	assert.False(t, isTransientDBErr(nil))
	assert.False(t, isTransientDBErr(sql.ErrNoRows))
	assert.False(t, isTransientDBErr(errors.New("permission denied")))
	// pgdriver.Error carries the SQLSTATE in field 'C'; without one it is not
	// a connection-class failure.
	assert.False(t, isTransientDBErr(pgdriver.Error{}))
	assert.True(t, isTransientDBErr(context.DeadlineExceeded))
	assert.True(t, isTransientDBErr(context.Canceled))
	assert.True(t, isTransientDBErr(stubNetError{}))
	assert.True(t, isTransientDBErr(fmt.Errorf("find: %w", stubNetError{})))
}

func TestDeviceOnly_SchoolGuard(t *testing.T) {
	t.Parallel()

	now := time.Now()
	tests := []struct {
		name     string
		schools  SchoolDirectory
		wantCode int
	}{
		{"active school", fakeSchools{school: &platform.School{Active: true}}, http.StatusOK},
		{"deleted school", fakeSchools{school: &platform.School{DeletedAt: &now}}, http.StatusForbidden},
		{"missing school", fakeSchools{err: sql.ErrNoRows}, http.StatusForbidden},
		{"transient failure fails open", fakeSchools{err: context.DeadlineExceeded}, http.StatusOK},
		{"other failure fails closed", fakeSchools{err: errors.New("permission denied")}, http.StatusForbidden},
		{"no directory skips the guard", nil, http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fleet := newFakeFleet()
			d := activeDevice(12, "kiosk")
			d.TenantID = 100
			fleet.add("kiosk-key", d)
			rr := serve(router(New(Dependencies{Devices: fleet, Schools: tc.schools}).DeviceOnly(), nil), request(http.MethodGet, "kiosk-key", nil))
			assert.Equal(t, tc.wantCode, rr.Code, rr.Body.String())
			if tc.wantCode == http.StatusForbidden {
				assert.Contains(t, rr.Body.String(), `"error":"device is not active"`)
			}
		})
	}
}

// =============================================================================
// Tenant binding
// =============================================================================

func TestBindTenant(t *testing.T) {
	t.Parallel()

	ctx, err := bindTenant(context.Background(), 42)
	require.NoError(t, err)
	assert.Equal(t, int64(42), tenant.FromContext(ctx))

	_, err = bindTenant(context.Background(), 0)
	require.Error(t, err)
}
