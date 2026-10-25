package account

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The parent demo of a school without Eltern-App answers like the parents
// login (#4895): 403 module.not_booked, not a server error.
func TestDemoAccessPortalNotBookedAnswersModuleNotBooked(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	err := &identityaccess.AuthenticationError{Op: "parent login", Err: &identityaccess.PortalNotBookedError{Module: "eltern_app", ModuleLabel: "Eltern-App"}}
	(&DemoResource{}).renderError(recorder, httptest.NewRequest(http.MethodPost, "/access/sessions", nil), err)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	var body struct {
		Code    string            `json:"code"`
		Details map[string]string `json:"details"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "module.not_booked", body.Code)
	assert.Equal(t, "eltern_app", body.Details["module"])
}
