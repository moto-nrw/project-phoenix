package timetablehttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTimetableResourceRejectsMissingSettings(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t, "timetable API: settings service is required", func() {
		NewResource(Dependencies{})
	})
}

func TestOperationCapabilitiesRejectsUnreadableFeaturePolicy(t *testing.T) {
	t.Parallel()
	resource := NewResource(Dependencies{SettingsService: &settingsDouble{ResolveBoolFn: func(context.Context, string) (bool, error) {
		return false, errors.New("private spontaneous activity policy failure")
	}}})
	response := httptest.NewRecorder()
	resource.operationsCapabilities(response, httptest.NewRequest(http.MethodGet, "/operations/capabilities", nil))
	require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
	require.NotContains(t, response.Body.String(), "private spontaneous activity policy failure")
}
