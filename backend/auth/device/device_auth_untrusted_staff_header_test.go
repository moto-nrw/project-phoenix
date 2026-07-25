package device

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/stretchr/testify/assert"
)

func TestDeviceAuthenticatorDoesNotTrustStaffIDHeader(t *testing.T) {
	lastSeenWriteCache = sync.Map{}
	t.Setenv("OGS_DEVICE_PIN", "test-pin")

	mockIoT := newMockIoTService()
	apiKey := "untrusted-staff-header-api-key"
	dev := &iot.Device{
		DeviceID: "test-device",
		Status:   iot.DeviceStatusActive,
	}
	dev.TenantID = 7
	mockIoT.addDevice(apiKey, dev)

	router := chi.NewRouter()
	router.Use(DeviceAuthenticator(mockIoT, nil, nil))
	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		assert.Nil(t, StaffFromCtx(r.Context()))
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
