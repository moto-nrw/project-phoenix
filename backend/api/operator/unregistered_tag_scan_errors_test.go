package operator_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/operator"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

// The review routes of unregistered RFID scans live in Device Fleet (#3232);
// the operator surface still decides how a failed resolution answers.

func renderResolveError(t *testing.T, err error) (int, string, string, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/unregistered-tag-scans/123/resolve", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, render.Render(rr, req, operator.UnregisteredTagScanResolveError(err)))
	var body struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	return rr.Code, body.Status, body.Message, rr.Body.String()
}

func TestResolveUnregisteredTagScanMapsDatabaseErrorToInternal(t *testing.T) {
	t.Parallel()

	rawErr := errors.New("pq: permission denied for audit.unregistered_tag_scans")

	code, status, message, raw := renderResolveError(t, &modelBase.DatabaseError{Op: "resolve unregistered tag scan", Err: rawErr})

	require.Equal(t, http.StatusInternalServerError, code)
	require.Equal(t, "error", status)
	require.Equal(t, "Failed to resolve unregistered RFID scan", message)
	require.NotContains(t, raw, rawErr.Error())
}

func TestResolveUnregisteredTagScanMapsWrappedAdapterErrorToInternal(t *testing.T) {
	t.Parallel()

	rawErr := errors.New("pq: permission denied for audit.unregistered_tag_scans")
	err := fmt.Errorf("devicefleet postgres: resolve unregistered tag scan: %w", rawErr)

	code, status, message, raw := renderResolveError(t, err)

	require.Equal(t, http.StatusInternalServerError, code)
	require.Equal(t, "error", status)
	require.Equal(t, "Failed to resolve unregistered RFID scan", message)
	require.NotContains(t, raw, rawErr.Error())
	require.NotContains(t, raw, "devicefleet postgres:")
}

func TestResolveUnregisteredTagScanKeepsValidationErrorsAsBadRequest(t *testing.T) {
	t.Parallel()

	for _, message := range []string{
		"operator ID is required",
		"scan ID is required",
		"unregistered tag scan not found",
		"unregistered tag scan already resolved",
		"invalid unregistered tag scan",
	} {
		code, status, got, _ := renderResolveError(t, errors.New(message))

		require.Equal(t, http.StatusBadRequest, code, message)
		require.Equal(t, "error", status, message)
		require.Equal(t, message, got)
	}
}
