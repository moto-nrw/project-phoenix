package api

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Issue #2105: slog-chi writes slog.String("query", r.URL.RawQuery) into every
// access line at Info level. Staff-UI search endpoints pass student names and
// e-mail addresses as query parameters (?search=, ?first_name=, ?email=, ?q=),
// so every search wrote personal data into Loki. The request logger is wrapped
// in NewQueryValueRedactor; this test drives the real setupBasicMiddleware
// chain and pins that query VALUES never reach the log while method, path,
// status, and duration survive.
func TestAccessLogOmitsQueryValues(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))

	router := chi.NewRouter()
	setupBasicMiddleware(router, logger, nil)
	router.Get("/api/students", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet,
		"/api/students?search=Mustermann&first_name=Erika&email=erika%40example.com", nil)
	// The browser sends the staff-UI page URL — including its search box query —
	// as the Referer on same-origin API calls.
	req.Header.Set("Referer", "https://schule.example.com/students/search?search=Mustermann")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	out := logs.String()
	require.NotEmpty(t, out, "expected the request to emit an access log line")

	for _, forbidden := range []string{"Mustermann", "Erika", "erika%40example.com", "erika@example.com"} {
		assert.NotContainsf(t, out, forbidden, "access log must not carry query value %q:\n%s", forbidden, out)
	}

	// Diagnostic value stays: method, path, status, duration, and the query
	// parameter NAMES.
	assert.Contains(t, out, `"method":"GET"`)
	assert.Contains(t, out, `"path":"/api/students"`)
	assert.Contains(t, out, `"status":200`)
	assert.Contains(t, out, `"latency"`)
	assert.Contains(t, out, `"query":"search&first_name&email"`)
}
