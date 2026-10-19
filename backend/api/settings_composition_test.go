package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderSettingsInputError(t *testing.T, err error) (int, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	apiCommon.RenderError(w, httptest.NewRequest(http.MethodPut, "/values/key", nil), settingsInputError(err))
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return w.Code, body
}

// A rejected attendance window names the window that blocks it (#3957), so
// the school reads how many days it may choose.
func TestSettingsInputErrorNamesTheBlockingAttendanceWindow(t *testing.T) {
	t.Parallel()

	visible := &settingsModule.SettingsError{Op: "set_value", Err: &settingsModule.AttendanceWindowError{
		Key: settingsModule.KeyAttendanceVisibleDays, VisibleDays: 90, RetentionDays: 60,
	}}
	status, body := renderSettingsInputError(t, visible)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, apiCommon.CodeSettingsAttendanceVisibleExceedsRetention, body["code"])
	assert.Equal(t, map[string]any{"retention_days": float64(60)}, body["details"])

	retention := &settingsModule.SettingsError{Op: "set_value", Err: &settingsModule.AttendanceWindowError{
		Key: settingsModule.KeyGDPRAttendanceRetentionDays, VisibleDays: 90, RetentionDays: 60,
	}}
	status, body = renderSettingsInputError(t, retention)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, apiCommon.CodeSettingsAttendanceRetentionBelowVisible, body["code"])
	assert.Equal(t, map[string]any{"visible_days": float64(90)}, body["details"])

	status, body = renderSettingsInputError(t, errors.New("value below minimum"))
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, apiCommon.CodeGeneralInput, body["code"], "any other rejected value keeps the general input code")
}
