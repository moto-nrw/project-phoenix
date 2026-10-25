package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestReadinessReflectsDatabaseAvailability(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	handler := common.NewReadinessHandler(db.PingContext)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "OK", rec.Body.String())
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

	require.NoError(t, db.Close())
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
	assertUnavailable(t, rec)
}

func TestReadinessTimesOutWhenPoolIsExhausted(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	conn, err := db.DB.Conn(context.Background())
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	// Holding the only connection makes PingContext wait for the probe's
	// deadline rather than a database response. No server is stopped.
	rec := httptest.NewRecorder()
	start := time.Now()
	common.NewReadinessHandler(db.PingContext).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
	require.Less(t, time.Since(start), 5*time.Second, "probe must not wait indefinitely for a connection")
	assertUnavailable(t, rec)
}

func TestReadinessHonoursRequestCancellation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec := httptest.NewRecorder()
	common.NewReadinessHandler(db.PingContext).ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/ready", nil))
	assertUnavailable(t, rec)
}

func assertUnavailable(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, common.CodeGeneralUnavailable, body["code"])
	require.Equal(t, "service unavailable", body["detail"], "do not expose database or connection errors")
}
