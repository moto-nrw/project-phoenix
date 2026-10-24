package common

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ProblemResponseMiddleware logs every body it has to repair (#2507). The
// check must flag a status other than the HTTP status number, an `error` or
// `message` text and a missing code, and leave the shared envelope alone
// (#2520).
func TestDepartsFromEnvelope(t *testing.T) {
	t.Parallel()

	for body, want := range map[string]bool{
		`{"status":409,"detail":"taken","code":"general.business_rejection"}`:                   false,
		`{"status":409,"detail":"taken","code":"care.announcement_stale","details":{"id":"1"}}`: false,
		`{"status":"error","error":"taken","code":"general.business_rejection"}`:                true,
		`{"status":409,"error":"taken","detail":"taken","code":"general.business_rejection"}`:   true,
		`{"status":404,"detail":"taken","code":"general.business_rejection"}`:                   true,
		`{"status":409,"detail":"taken"}`:                                                       true,
		`{"status":"Conflict","error":"taken"}`:                                                 true,
		`{"status":409,"message":"taken","code":"general.business_rejection"}`:                  true,
		`{"conflicts":[],"message":"taken"}`:                                                    true,
		`{"status":"success","data":{},"message":"conflict"}`:                                   true,
	} {
		var decoded map[string]json.RawMessage
		assert.NoError(t, json.Unmarshal([]byte(body), &decoded))
		assert.Equal(t, want, departsFromEnvelope(decoded, 409), body)
	}
}

// A repaired body is a regression of the one error path, so it is logged at
// error level with route, status and the code the answer now carries. A
// status without a body is how a handler lets the middleware answer; it is
// not logged.
func TestProblemResponseLogsEveryRepairAtErrorLevel(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	router := chi.NewRouter()
	router.Use(problemResponse(func() *slog.Logger { return logger }))
	router.Get("/legacy", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"status":"error","error":"taken"}`))
	})
	router.Get("/bodiless", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/legacy", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/bodiless", nil))

	var records []map[string]any
	for line := range strings.Lines(logs.String()) {
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		if record["msg"] == "error body outside the shared envelope" {
			records = append(records, record)
		}
	}
	require.Len(t, records, 1, logs.String())
	assert.Equal(t, "ERROR", records[0]["level"])
	assert.Equal(t, "/legacy", records[0]["route"])
	assert.Equal(t, float64(http.StatusConflict), records[0]["status"])
	assert.Equal(t, CodeGeneralBusinessRejection, records[0]["code"])
}
