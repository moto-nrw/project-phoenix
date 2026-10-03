package compose

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// A session conflict answered 409 inside the success envelope until #2507.
// It is a refusal like any other now: the shared error envelope, the
// conflict as details. The kiosk detects it by the 409 alone.
func TestSessionConflictAnswersInTheSharedErrorEnvelope(t *testing.T) {
	t.Parallel()

	device := int64(4711)
	recorder := httptest.NewRecorder()
	renderSessionConflict(recorder, httptest.NewRequest(http.MethodPost, "/api/iot/session/start", nil), "Session conflict detected", devicescan.ConflictInfoResponse{
		HasConflict: true, ConflictingDevice: &device, ConflictMessage: "Activity is already running", CanOverride: true,
	})

	require.Equal(t, http.StatusConflict, recorder.Code)
	assert.Empty(t, routetest.ProblemEnvelopeViolations(recorder.Body.Bytes(), false), recorder.Body.String())
	var body struct {
		Error   string         `json:"error"`
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "Session conflict detected", body.Error)
	assert.Equal(t, "general.business_rejection", body.Code)
	assert.Equal(t, map[string]any{
		"has_conflict": true, "conflicting_device": float64(4711),
		"conflict_message": "Activity is already running", "can_override": true,
	}, body.Details)
}
