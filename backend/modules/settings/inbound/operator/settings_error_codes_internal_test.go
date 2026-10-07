package operator

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderSettingsErrorBody(t *testing.T, err error) (int, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	renderOperatorSettingsError(w, httptest.NewRequest(http.MethodPut, "/test", nil), err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return w.Code, body
}

// #2519: the settings outcomes name their reason by code.
func TestRenderOperatorSettingsErrorAnswersWithCodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"unknown setting", &settings.SettingsError{Op: "set_value", Err: &settings.DefinitionNotFoundError{Key: "bad.key"}}, http.StatusNotFound, "settings.not_found"},
		{"invalid value", &settings.SettingsError{Op: "set_value", Err: &settings.InvalidValueError{Key: "a.b", Reason: "below minimum 5"}}, http.StatusBadRequest, "settings.invalid_value"},
		{"permission", &settings.SettingsError{Op: "set_value", Err: &settings.PermissionDeniedError{Key: "a.b"}}, http.StatusForbidden, "general.permission"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			status, body := renderSettingsErrorBody(t, tc.err)
			assert.Equal(t, tc.status, status)
			assert.Equal(t, tc.code, body["code"])
		})
	}
}

// A failure the mapper does not classify is logged, never sent: the client
// sees a fixed text, not the cause (#2519).
func TestRenderOperatorSettingsErrorKeepsCauseOutOfTheAnswer(t *testing.T) {
	t.Parallel()

	for name, err := range map[string]error{
		"settings error": &settings.SettingsError{Op: "set_value", Err: errors.New("pq: relation config.secret")},
		"other error":    errors.New("pq: relation config.secret"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			status, body := renderSettingsErrorBody(t, err)
			assert.Equal(t, http.StatusInternalServerError, status)
			assert.Equal(t, "general.server", body["code"])
			assert.NotContains(t, body["error"], "pq:")
			assert.NotContains(t, body["detail"], "pq:")
		})
	}
}
