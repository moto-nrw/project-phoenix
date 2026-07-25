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

// mockStaffLookup implements StaffByIDGetter for resolveStaffContext tests.
type mockStaffLookup struct {
	staff *users.Staff
	err   error
}

func (m *mockStaffLookup) GetStaffByID(_ context.Context, _ int64) (*users.Staff, error) {
	return m.staff, m.err
}

func testDeviceForTenant(tenantID int64) *iot.Device {
	dev := &iot.Device{DeviceID: "test-device"}
	dev.TenantID = tenantID
	return dev
}

func tenantStaff(id, tenantID int64) *users.Staff {
	staff := &users.Staff{}
	staff.ID = id
	staff.SetTenantID(tenantID)
	return staff
}

func TestResolveStaffContext_ValidStaff(t *testing.T) {
	staff := tenantStaff(42, 7)
	lookup := &mockStaffLookup{staff: staff}

	result := resolveStaffContext(context.Background(), lookup, testDeviceForTenant(7), "42")
	assert.Equal(t, staff, result)
}

func TestResolveStaffContext_EmptyHeader(t *testing.T) {
	lookup := &mockStaffLookup{staff: tenantStaff(42, 7)}
	assert.Nil(t, resolveStaffContext(context.Background(), lookup, testDeviceForTenant(7), ""))
}

func TestResolveStaffContext_NilLookup(t *testing.T) {
	assert.Nil(t, resolveStaffContext(context.Background(), nil, testDeviceForTenant(7), "42"))
}

func TestResolveStaffContext_InvalidHeader(t *testing.T) {
	lookup := &mockStaffLookup{staff: tenantStaff(42, 7)}
	assert.Nil(t, resolveStaffContext(context.Background(), lookup, testDeviceForTenant(7), "abc"))
	assert.Nil(t, resolveStaffContext(context.Background(), lookup, testDeviceForTenant(7), "-1"))
	assert.Nil(t, resolveStaffContext(context.Background(), lookup, testDeviceForTenant(7), "0"))
}

func TestResolveStaffContext_LookupError(t *testing.T) {
	lookup := &mockStaffLookup{err: errors.New("not found")}
	assert.Nil(t, resolveStaffContext(context.Background(), lookup, testDeviceForTenant(7), "42"))
}

func TestResolveStaffContext_CrossTenantRejected(t *testing.T) {
	lookup := &mockStaffLookup{staff: tenantStaff(42, 99)}
	assert.Nil(t, resolveStaffContext(context.Background(), lookup, testDeviceForTenant(7), "42"))
}

func TestDeviceAuthenticator_AddsValidStaffToContext(t *testing.T) {
	lastSeenWriteCache = sync.Map{}
	t.Setenv("OGS_DEVICE_PIN", "test-pin")

	mockIoT := newMockIoTService()
	apiKey := "staff-context-api-key"
	dev := testDeviceForTenant(7)
	dev.Status = iot.DeviceStatusActive
	mockIoT.addDevice(apiKey, dev)

	staff := tenantStaff(42, 7)
	router := chi.NewRouter()
	router.Use(DeviceAuthenticator(mockIoT, nil, &mockStaffLookup{staff: staff}, nil))
	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		require.Same(t, staff, StaffFromCtx(r.Context()))
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Staff-PIN", "test-pin")
	req.Header.Set("X-Staff-ID", "42")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusOK, recorder.Code)
}
