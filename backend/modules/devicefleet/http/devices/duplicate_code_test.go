package devices

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/modules/devicefleet"
)

// TestRenderErrorNamesTakenDeviceID pins the code and field the device form
// marks for a device ID that is already in use (#2517).
func TestRenderErrorNamesTakenDeviceID(t *testing.T) {
	t.Parallel()

	var gotCode, gotField string
	rs := &Resource{runtime: Runtime{ConflictOnField: func(w http.ResponseWriter, _ *http.Request, _ error, code, field string) {
		gotCode, gotField = code, field
		w.WriteHeader(http.StatusConflict)
	}}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rs.renderError(rr, req, &devicefleet.AdministrationError{
		Op:  "CreateDevice",
		Err: &devicefleet.AdministrationDuplicateDeviceIDError{DeviceID: "tablet-1"},
	})

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Equal(t, codeDeviceIDTaken, gotCode)
	assert.Equal(t, "device_id", gotField)
}
