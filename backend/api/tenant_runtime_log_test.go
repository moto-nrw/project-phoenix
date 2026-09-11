package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runtimeEventLogs(t *testing.T, status int, event tenant.RuntimeEvent) []map[string]any {
	t.Helper()
	var output bytes.Buffer
	tracer := newRuntimeTracer(slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelInfo})))

	request := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)

	recordHTTPRuntimeEvent(tracer, httpRuntimeObservation{Request: request, Route: "/auth/refresh", Status: status, Event: event})

	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record), line)
		records = append(records, record)
	}
	return records
}

// #2953: a rolled-back transaction behind a 401 (expired refresh token) is
// expected behaviour and must not produce an ERROR line.
func TestRecordHTTPRuntimeEventStaysSilentForRollbackBehindClientError(t *testing.T) {
	t.Parallel()
	event := tenant.RuntimeEvent{Kind: tenant.RuntimeTransaction, Result: tenant.UnitOfWorkRolledBack, Err: errors.New("token expired")}

	for _, status := range []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusGone, http.StatusOK} {
		assert.Empty(t, runtimeEventLogs(t, status, event), "status %d", status)
	}
}

func TestRecordHTTPRuntimeEventLogsRollbackBehindServerErrorWithRequestFields(t *testing.T) {
	t.Parallel()
	event := tenant.RuntimeEvent{Kind: tenant.RuntimeTransaction, Result: tenant.UnitOfWorkRolledBack, Err: errors.New("permission denied for schema active")}

	records := runtimeEventLogs(t, http.StatusInternalServerError, event)

	require.Len(t, records, 1)
	for key, want := range map[string]any{
		"level":       "ERROR",
		"msg":         "runtime operation failed",
		"entry_point": "http",
		"operation":   "transaction",
		"outcome":     "transaction_failure",
		"result":      "rollback",
		"method":      http.MethodPost,
		"route":       "/auth/refresh",
		"path":        "/auth/refresh",
		"status":      float64(http.StatusInternalServerError),
	} {
		assert.Equal(t, want, records[0][key], key)
	}
	_, leaked := records[0]["error"]
	assert.False(t, leaked, "error text must stay at Debug")
}

func TestRecordHTTPRuntimeEventKeepsMissingTenantAndWriteFailuresAsErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		event   tenant.RuntimeEvent
		outcome string
	}{
		{event: tenant.RuntimeEvent{Kind: tenant.RuntimeMissingTenant, Err: tenant.ErrTenantRequired}, outcome: "missing_tenant"},
		{event: tenant.RuntimeEvent{Kind: "response_write", Err: errors.New("broken pipe")}, outcome: "response_write_failure"},
	} {
		records := runtimeEventLogs(t, http.StatusNoContent, tc.event)
		require.Len(t, records, 1, tc.outcome)
		assert.Equal(t, "ERROR", records[0]["level"])
		assert.Equal(t, tc.outcome, records[0]["outcome"])
		assert.Equal(t, float64(http.StatusNoContent), records[0]["status"])
	}
}
