package operator_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
)

// renderOperatorError renders an error body the way the routes do and
// returns its status code and message. The cases below moved with
// AuthErrorRenderer from api/operator (#3231); they read the rendered body
// because this adapter's tests do not name api/common.
func renderOperatorError(t *testing.T, renderer render.Renderer) (int, string) {
	t.Helper()
	rr := httptest.NewRecorder()
	require.NoError(t, render.Render(rr, httptest.NewRequest(http.MethodGet, "/", nil), renderer))
	var body struct {
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), rr.Body.String())
	return rr.Code, body.Message
}

func TestAuthErrorRenderer_InvalidCredentials(t *testing.T) {
	t.Parallel()

	status, errorText := renderOperatorError(t, operator.AuthErrorRenderer(operator.ErrOperatorInvalidCredentials))
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "Invalid email or password", errorText)
}

func TestAuthErrorRenderer_OperatorInactive(t *testing.T) {
	t.Parallel()

	status, errorText := renderOperatorError(t, operator.AuthErrorRenderer(operator.ErrOperatorInactive))
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "Operator account is inactive", errorText)
}

func TestAuthErrorRenderer_OperatorNotFound(t *testing.T) {
	t.Parallel()

	status, errorText := renderOperatorError(t, operator.AuthErrorRenderer(operator.ErrOperatorNotFound))
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "Invalid email or password", errorText)
}

func TestAuthErrorRenderer_GenericError(t *testing.T) {
	t.Parallel()

	status, errorText := renderOperatorError(t, operator.AuthErrorRenderer(errors.New("database error")))
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "Authentication failed", errorText)
}

// TestAuthErrorRenderer_MFAStatusUnavailable (Item #3) — an operator MFA
// gate that hit a non-not-found infra error during HasEnrollment must surface
// as 503 so it fails-closed for THIS caller without locking everyone else
// out (operator MFA is mandatory; silently treating it as "not enrolled"
// would issue an enrollment-token bypass during a credentials-table outage).
func TestAuthErrorRenderer_MFAStatusUnavailable(t *testing.T) {
	t.Parallel()

	status, errorText := renderOperatorError(t, operator.AuthErrorRenderer(operator.ErrMFAStatusUnavailable))
	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Equal(t, "MFA status temporarily unavailable, please retry", errorText)
}
