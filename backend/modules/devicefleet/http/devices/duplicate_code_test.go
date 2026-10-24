package devices

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
)

// TestRenderErrorNamesTakenDeviceID pins the code and field the device form
// marks for a device ID that is already in use (#2517).
func TestRenderErrorNamesTakenDeviceID(t *testing.T) {
	t.Parallel()

	rs := &Resource{runtime: testRuntime()}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rs.renderError(rr, req, &devicefleet.AdministrationError{
		Op:  "CreateDevice",
		Err: &devicefleet.AdministrationDuplicateDeviceIDError{DeviceID: "tablet-1"},
	})

	assert.Equal(t, http.StatusConflict, rr.Code)
	var body struct {
		Code   string `json:"code"`
		Errors []struct {
			Field string `json:"field"`
		} `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, codeDeviceIDTaken, body.Code)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "device_id", body.Errors[0].Field)
}
