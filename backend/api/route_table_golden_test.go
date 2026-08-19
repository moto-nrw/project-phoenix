// Route-table and IoT auth-matrix goldens — issue #575 batch B0.
//
// The upcoming API-layer restructuring (god-object splits, IoT sub-package
// consolidation) must not move, drop, or re-auth a single route. Two goldens
// pin that:
//
//  1. TestRouteTableGolden walks the fully-assembled production router
//     (api.New — the same constructor `serve` and `gendoc` use) and compares
//     the sorted METHOD+pattern list against testdata/route_table.golden.
//     Any added, removed, or moved route fails the diff.
//
//  2. TestIoTAuthMatrixGolden fires an UNAUTHENTICATED request at every
//     /api/iot/* route and pins status + body. The IoT router mounts the
//     JWT-authenticated devices sub-router at "/" as a catch-all, so a route
//     accidentally dropped from a device-auth group during the consolidation
//     would fall through to the JWT group and start rejecting kiosks with a
//     JWT-401 instead of the device-auth error string. That failure mode is
//     invisible to the route-table golden (the path still exists) but changes
//     the rejection signature pinned here.
//
// Regenerate after an INTENTIONAL route change:
//
//	go test ./api/ -run 'TestRouteTableGolden|TestIoTAuthMatrixGolden' -update-goldens
//
// and justify the diff in the PR description.
package api

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

var updateGoldens = flag.Bool("update-goldens", false, "rewrite the route-table and IoT auth-matrix golden files")

var (
	goldenAPIOnce sync.Once
	goldenAPI     *API
	goldenAPIErr  error
)

// newGoldenAPI builds the production API exactly once per test binary.
// api.New registers Prometheus collectors and DB stats providers on global
// registries — a second construction in the same process would panic on
// duplicate registration.
func newGoldenAPI(t *testing.T) *API {
	t.Helper()

	// SetupTestDB loads the root .env (TEST_DB_DSN, PHOENIX_AUTH_PASSWORD),
	// forces APP_ENV=test, and points viper's test_db_dsn at the
	// package-isolated clone — api.New's DBConnForServe resolves through the
	// same viper key, so the whole router builds against the clone.
	_ = testpkg.SetupTestDB(t)

	t.Setenv("METRICS_BEARER_TOKEN", "route-golden-test-token")

	goldenAPIOnce.Do(func() {
		goldenAPI, goldenAPIErr = New(false, slog.Default())
	})
	require.NoError(t, goldenAPIErr, "api.New failed — route goldens need the assembled production router")
	return goldenAPI
}

func TestRouteTableGolden(t *testing.T) {
	apiInstance := newGoldenAPI(t)

	var routes []string
	walkErr := chi.Walk(apiInstance.Router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+route)
		return nil
	})
	require.NoError(t, walkErr)
	sort.Strings(routes)
	got := strings.Join(routes, "\n") + "\n"

	compareGolden(t, filepath.Join("testdata", "route_table.golden"), got,
		"the route table changed — if intentional, regenerate with -update-goldens and call the change out in the PR description")
}

// chiParamPattern matches one {param} placeholder (incl. regex-constrained
// ones like {id:[0-9]+}) for probe-URL substitution.
var chiParamPattern = regexp.MustCompile(`\{[^}]+\}`)

func TestIoTAuthMatrixGolden(t *testing.T) {
	apiInstance := newGoldenAPI(t)

	var iotRoutes []string
	walkErr := chi.Walk(apiInstance.Router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.HasPrefix(route, "/api/iot/") {
			iotRoutes = append(iotRoutes, method+" "+route)
		}
		return nil
	})
	require.NoError(t, walkErr)
	sort.Strings(iotRoutes)

	var lines []string
	for _, mr := range iotRoutes {
		parts := strings.SplitN(mr, " ", 2)
		method, pattern := parts[0], parts[1]

		// Substitute path params with a plausible literal; auth middleware
		// rejects the request before any handler parses them.
		probePath := chiParamPattern.ReplaceAllString(pattern, "1")
		probePath = strings.ReplaceAll(probePath, "*", "x")

		req := httptest.NewRequest(method, probePath, nil)
		rec := httptest.NewRecorder()
		apiInstance.Router.ServeHTTP(rec, req)

		body := strings.TrimSpace(rec.Body.String())
		lines = append(lines, fmt.Sprintf("%s %s -> %d %s", method, pattern, rec.Code, body))
	}
	got := strings.Join(lines, "\n") + "\n"

	compareGolden(t, filepath.Join("testdata", "iot_auth_matrix.golden"), got,
		"an unauthenticated /api/iot request now gets a different rejection — a route may have slipped between the device-auth and JWT groups; kiosks would see raw 401s. If intentional, regenerate with -update-goldens")
}

func compareGolden(t *testing.T, path, got, hint string) {
	t.Helper()
	if *updateGoldens {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
		t.Logf("golden rewritten: %s", path)
		return
	}
	want, err := os.ReadFile(path) // #nosec G304 -- repo-local testdata
	require.NoErrorf(t, err, "golden file %s missing — generate it with -update-goldens", path)
	if string(want) != got {
		t.Errorf("%s\n\ngolden diff for %s:\n%s", hint, path, unifiedDiff(string(want), got))
	}
}

// unifiedDiff renders a minimal line diff (no external deps) — enough to see
// which routes appeared/disappeared without dumping both full tables.
func unifiedDiff(want, got string) string {
	wantLines := strings.Split(strings.TrimRight(want, "\n"), "\n")
	gotLines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	wantSet := make(map[string]bool, len(wantLines))
	for _, l := range wantLines {
		wantSet[l] = true
	}
	gotSet := make(map[string]bool, len(gotLines))
	for _, l := range gotLines {
		gotSet[l] = true
	}
	var b strings.Builder
	for _, l := range wantLines {
		if !gotSet[l] {
			b.WriteString("- " + l + "\n")
		}
	}
	for _, l := range gotLines {
		if !wantSet[l] {
			b.WriteString("+ " + l + "\n")
		}
	}
	if b.Len() == 0 {
		return "(same line sets — ordering or duplicate-count changed)"
	}
	return b.String()
}
