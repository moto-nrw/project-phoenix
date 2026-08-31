package common

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

func executeReadOnlyPreview(t *testing.T, method, path string, claims jwt.AppClaims) *httptest.ResponseRecorder {
	t.Helper()
	handlerCalled := false
	handler := ReadOnlyPreviewMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(method, path, nil)
	req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, claims))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		require.True(t, handlerCalled)
	} else {
		require.False(t, handlerCalled, "blocked request must never reach the handler")
	}
	return rec
}

func TestReadOnlyPreviewMiddleware(t *testing.T) {
	t.Parallel()

	preview := jwt.AppClaims{ID: 42, ReadOnly: true, ActingAdminID: 7}
	regular := jwt.AppClaims{ID: 42}

	t.Run("regular tokens pass every method untouched", func(t *testing.T) {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			rec := executeReadOnlyPreview(t, method, "/api/students", regular)
			assert.Equalf(t, http.StatusOK, rec.Code, "method %s", method)
		}
	})

	t.Run("missing claims pass (device-authenticated routes)", func(t *testing.T) {
		handler := ReadOnlyPreviewMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/iot/checkin", nil))
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("preview tokens read freely", func(t *testing.T) {
		for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
			rec := executeReadOnlyPreview(t, method, "/api/students", preview)
			assert.Equalf(t, http.StatusOK, rec.Code, "method %s", method)
		}
	})

	t.Run("state-changing GETs stay blocked", func(t *testing.T) {
		// GET /api/calendar/feed persists a subscription token on first call.
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			rec := executeReadOnlyPreview(t, method, "/api/calendar/feed", preview)
			assert.Equalf(t, http.StatusForbidden, rec.Code, "%s /api/calendar/feed", method)
		}
		// Neighbouring reads are unaffected.
		rec := executeReadOnlyPreview(t, http.MethodGet, "/api/calendar/events", preview)
		assert.Equal(t, http.StatusOK, rec.Code)
		// A regular token keeps full access.
		rec = executeReadOnlyPreview(t, http.MethodGet, "/api/calendar/feed", regular)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("preview tokens are blocked on writes", func(t *testing.T) {
		cases := []struct{ method, path string }{
			{http.MethodPost, "/api/students"},
			{http.MethodPut, "/api/students/5"},
			{http.MethodPatch, "/api/settings/values/foo"},
			{http.MethodDelete, "/api/students/5"},
			{http.MethodPost, "/auth/switch-tenant"},
			{http.MethodPost, "/auth/staff-preview"},
			{http.MethodPost, "/auth/password"},
			// reads with a plaintext-reveal side effect stay blocked
			{http.MethodPost, "/api/staff/5/stammdaten/bank-steuer/reveal"},
			{http.MethodPost, "/api/guardians/5/payment/reveal"},
			// import dry-runs persist a GDPR audit row in the target's name
			{http.MethodPost, "/api/import/students/preview"},
			{http.MethodPost, "/api/import/teachers/preview"},
			{http.MethodPost, "/api/import/class-list-entries/preview"},
			{http.MethodPost, "/api/import/opening-balances/preview"},
			// bulk WRITES that sit next to allowlisted bulk reads
			{http.MethodPost, "/api/students/pickup-schedules/bulk"},
			{http.MethodPost, "/api/students/arrival-schedules/bulk"},
			{http.MethodPost, "/api/students/status-days/bulk"},
		}
		for _, tc := range cases {
			rec := executeReadOnlyPreview(t, tc.method, tc.path, preview)
			assert.Equalf(t, http.StatusForbidden, rec.Code, "%s %s", tc.method, tc.path)
			assert.Containsf(t, rec.Body.String(), CodeReadOnlyPreview, "%s %s", tc.method, tc.path)
		}
	})

	t.Run("preview tokens may call allowlisted read-only POSTs", func(t *testing.T) {
		cases := []string{
			"/api/students/export",
			"/api/rooms/export",
			"/api/students/123/pickup-schedules/preview",
			"/api/enrollment/phases/9/export",
			"/api/students/pickup-times/bulk",
			"/api/schedules/check-conflict",
			"/api/timetable/lists/options",
		}
		for _, path := range cases {
			rec := executeReadOnlyPreview(t, http.MethodPost, path, preview)
			assert.Equalf(t, http.StatusOK, rec.Code, "POST %s", path)
		}
	})

	t.Run("pattern segments never match across slashes or prefixes", func(t *testing.T) {
		cases := []string{
			"/api/students/export/extra",                 // longer than the pattern
			"/api/students",                              // shorter
			"/api/students/1/2/pickup-schedules/preview", // {id} must be ONE segment
		}
		for _, path := range cases {
			rec := executeReadOnlyPreview(t, http.MethodPost, path, preview)
			assert.Equalf(t, http.StatusForbidden, rec.Code, "POST %s", path)
		}
	})
}

// goldenRoutesForMethod returns every route pattern the production route
// table (api/testdata/route_table.golden) registers for one HTTP method.
func goldenRoutesForMethod(t *testing.T, method string) []string {
	t.Helper()

	golden, err := os.Open(filepath.Join("..", "testdata", "route_table.golden"))
	require.NoError(t, err, "route table golden must exist (regenerated by TestRouteTableGolden)")
	defer func() { require.NoError(t, golden.Close()) }()

	var routes []string
	scanner := bufio.NewScanner(golden)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if pattern, ok := strings.CutPrefix(line, method+" "); ok {
			routes = append(routes, pattern)
		}
	}
	require.NoError(t, scanner.Err())
	require.NotEmpty(t, routes)
	return routes
}

// TestReadOnlyGETDenylistMatchesRouteTable pins the state-changing-GET
// denylist against the route table: every entry must name an existing GET
// route, and the matcher may accept exactly those routes — a renamed feed
// route fails here instead of silently reopening the write hole.
func TestReadOnlyGETDenylistMatchesRouteTable(t *testing.T) {
	t.Parallel()

	getRoutes := goldenRoutesForMethod(t, http.MethodGet)

	routeSet := make(map[string]bool, len(getRoutes))
	var matched []string
	for _, pattern := range getRoutes {
		routeSet[pattern] = true
		if matchesPatterns(pattern, readOnlyGETDenylist) {
			matched = append(matched, pattern)
		}
	}

	for _, entry := range readOnlyGETDenylist {
		assert.Truef(t, routeSet[entry], "denylist entry %q names no existing GET route — stale?", entry)
	}

	want := append([]string(nil), readOnlyGETDenylist...)
	sort.Strings(want)
	sort.Strings(matched)
	assert.Equal(t, want, matched,
		"the set of GET routes the read-only denylist blocks drifted from the list")
}

// TestReadOnlyPOSTAllowlistMatchesRouteTable pins the allowlist against the
// production route table (api/testdata/route_table.golden): every entry must
// name an existing POST route, and — computed the other way around — the set
// of golden POST routes the matcher accepts must be exactly the allowlist.
// A new write route can therefore never silently match, and a removed or
// renamed route fails here instead of rotting in the list.
func TestReadOnlyPOSTAllowlistMatchesRouteTable(t *testing.T) {
	t.Parallel()

	postRoutes := goldenRoutesForMethod(t, http.MethodPost)

	routeSet := make(map[string]bool, len(postRoutes))
	var matched []string
	for _, pattern := range postRoutes {
		routeSet[pattern] = true
		// The golden holds chi patterns; the matcher sees concrete paths.
		// A pattern-vs-pattern comparison works because {x} matches the
		// literal "{x}" segment as any-one-segment.
		if matchesPatterns(pattern, readOnlyPOSTAllowlist) {
			matched = append(matched, pattern)
		}
	}

	for _, entry := range readOnlyPOSTAllowlist {
		assert.Truef(t, routeSet[entry], "allowlist entry %q names no existing POST route — stale?", entry)
	}

	want := append([]string(nil), readOnlyPOSTAllowlist...)
	sort.Strings(want)
	sort.Strings(matched)
	assert.Equal(t, want, matched,
		"the set of POST routes the read-only matcher accepts drifted from the allowlist — check whether a new route accidentally matches or an entry went stale")
}
