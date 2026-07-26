package device

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubStaffPINAuthenticator struct {
	staff    *users.Staff
	err      error
	tenantID int64
	staffID  int64
	pin      string
	calls    int
}

func (s *stubStaffPINAuthenticator) AuthenticateStaffPIN(
	_ context.Context,
	tenantID, staffID int64,
	pin string,
) (*users.Staff, error) {
	s.calls++
	s.tenantID = tenantID
	s.staffID = staffID
	s.pin = pin
	return s.staff, s.err
}

func newStaffPINTestRouter(
	t *testing.T,
	authenticator StaffPINAuthenticator,
	handler http.HandlerFunc,
) (*chi.Mux, string) {
	t.Helper()
	lastSeenWriteCache = sync.Map{}
	t.Setenv("OGS_DEVICE_PIN", "device-pin")

	mockIoT := newMockIoTService()
	apiKey := "staff-pin-auth-api-key"
	dev := &iot.Device{
		DeviceID: "test-device",
		Status:   iot.DeviceStatusActive,
	}
	dev.TenantID = 7
	mockIoT.addDevice(apiKey, dev)

	router := chi.NewRouter()
	router.Use(DeviceAuthenticator(mockIoT, nil, authenticator, nil))
	router.Get("/test", handler)
	return router, apiKey
}

func newStaffPINTestRequest(apiKey, staffPIN string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Staff-PIN", "device-pin")
	req.Header.Set("X-Staff-ID", "42")
	if staffPIN != "" {
		req.Header.Set("X-Staff-Auth-PIN", staffPIN)
	}
	return req
}

func TestDeviceAuthenticatorSetsCredentialBoundStaffContext(t *testing.T) {
	staff := &users.Staff{}
	staff.ID = 42
	staff.TenantID = 7
	authenticator := &stubStaffPINAuthenticator{staff: staff}

	router, apiKey := newStaffPINTestRouter(t, authenticator, func(w http.ResponseWriter, r *http.Request) {
		require.Same(t, staff, StaffFromCtx(r.Context()))
		w.WriteHeader(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, newStaffPINTestRequest(apiKey, "personal-pin"))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, 1, authenticator.calls)
	assert.Equal(t, staff.TenantID, authenticator.tenantID)
	assert.Equal(t, staff.ID, authenticator.staffID)
	assert.Equal(t, "personal-pin", authenticator.pin)
}

func TestDeviceAuthenticatorIgnoresLegacyStaffIDWithoutCredential(t *testing.T) {
	authenticator := &stubStaffPINAuthenticator{}
	handlerCalled := false
	router, apiKey := newStaffPINTestRouter(t, authenticator, func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		assert.Nil(t, StaffFromCtx(r.Context()))
		w.WriteHeader(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, newStaffPINTestRequest(apiKey, ""))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, handlerCalled)
	assert.Zero(t, authenticator.calls)
}

func TestDeviceAuthenticatorRejectsInvalidStaffCredential(t *testing.T) {
	authenticator := &stubStaffPINAuthenticator{err: errors.New("invalid credential")}
	handlerCalled := false
	router, apiKey := newStaffPINTestRouter(t, authenticator, func(w http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, newStaffPINTestRequest(apiKey, "wrong-pin"))

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.False(t, handlerCalled)
	assert.Equal(t, 1, authenticator.calls)
}

func TestDeviceAuthenticatorRejectsCrossTenantStaff(t *testing.T) {
	staff := &users.Staff{}
	staff.ID = 42
	staff.TenantID = 8
	authenticator := &stubStaffPINAuthenticator{staff: staff}
	handlerCalled := false
	router, apiKey := newStaffPINTestRouter(t, authenticator, func(w http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, newStaffPINTestRequest(apiKey, "personal-pin"))

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.False(t, handlerCalled)
}
