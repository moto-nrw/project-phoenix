// Route-table and IoT auth-matrix goldens — issue #575 batch B0.
//
// The upcoming API-layer restructuring (god-object splits, IoT sub-package
// consolidation) must not move, drop, or re-auth a single route. Two goldens
// pin that:
//
//  1. TestRouteTableGolden walks the fully-assembled production Serve root
//     (api.WithRuntime, including the embedded worker wiring) and pins both the
//     sorted METHOD+pattern list and each route's ordered middleware chain.
//     Any added, removed, moved, or re-wrapped route fails the diff.
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
//	go test ./api/ -run TestFullProductionRouterGolden -update-goldens
//
// and justify the diff in the PR description.
package api

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

var updateGoldens = flag.Bool("update-goldens", false, "rewrite the route-table and IoT auth-matrix golden files")

func checkRouteTableGolden(t *testing.T, apiInstance *API) {
	t.Parallel()

	var routes, middlewareRoutes []string
	walkErr := chi.Walk(apiInstance.Router, func(method, route string, _ http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+route)
		names := make([]string, 0, len(middlewares))
		for _, middleware := range middlewares {
			name := runtime.FuncForPC(reflect.ValueOf(middleware).Pointer()).Name()
			name = strings.TrimPrefix(name, "github.com/moto-nrw/project-phoenix/")
			names = append(names, compilerGeneratedNamePart.ReplaceAllString(name, "#"))
		}
		middlewareRoutes = append(middlewareRoutes, fmt.Sprintf("%s %s -> %s", method, route, strings.Join(names, " > ")))
		return nil
	})
	require.NoError(t, walkErr)
	sort.Strings(routes)
	sort.Strings(middlewareRoutes)

	compareGolden(t, filepath.Join("testdata", "route_table.golden"), strings.Join(routes, "\n")+"\n",
		"the route table changed — if intentional, regenerate with -update-goldens and call the change out in the PR description")
	compareGolden(t, filepath.Join("testdata", "middleware_table.golden"), strings.Join(middlewareRoutes, "\n")+"\n",
		"a route's middleware chain changed — if intentional, regenerate with -update-goldens and call out the auth, scope, transaction, and observability impact",
		stableMiddlewareTable)

	// Der Frontend-Client ruft Sammlungen ohne Schrägstrich am Ende auf
	// (/api/staff-notices), das Backend registriert den Subrouter mit "/".
	// chi.Mount bedient beide Schreibweisen mit demselben Handler, die
	// Walk-Tabelle oben zeigt aber nur eine davon. Der Probe-Lauf pinnt, dass
	// ein Umbau der Mount-Stelle das nicht stumm zu einem 404 macht.
	t.Run("mounted collections answer without a trailing slash", func(t *testing.T) {
		cases := []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/api/staff-notices"},
			{http.MethodGet, "/api/staff-notices/"},
			{http.MethodPost, "/api/staff-notices"},
			{http.MethodPost, "/api/staff-notices/"},
			{http.MethodGet, "/api/staff-notices/today"},
		}
		for _, tc := range cases {
			rec := httptest.NewRecorder()
			apiInstance.Router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			// Ohne Token endet die Anfrage in der Auth-Kette (401), nicht im
			// Router-Fallback (404): der Pfad ist also gebunden.
			require.Equalf(t, http.StatusUnauthorized, rec.Code, "%s %s must be routed to the staff-notice resource", tc.method, tc.path)
		}
	})

	// Pins the write block for admin staff-view preview tokens (#2893) to
	// EVERY authenticated WRITE route, not just the ones the feature was built
	// against. Each api sub-package assembles its own JWT chain, so a group
	// that authenticates without ReadOnlyPreviewMiddleware would let a preview
	// token write through calendar, import, enrollment, or any future router —
	// silently, because the route still answers 200. Walking the assembled
	// production router is the only place that sees all of them at once.
	//
	// Read-only methods are exempt: the middleware lets GET/HEAD/OPTIONS
	// through anyway (minus its own denylist), so a purely reading group (the
	// SSE streams, which may not import api/common under the architecture
	// policy) needs nothing.
	t.Run("every authenticated write route carries the read-only preview guard", func(t *testing.T) {
		const (
			authenticator = "auth/jwt.Authenticator"
			readOnlyGuard = "api/common.ReadOnlyPreviewMiddleware"
		)
		safeMethods := map[string]bool{
			http.MethodGet:     true,
			http.MethodHead:    true,
			http.MethodOptions: true,
		}

		var unguarded []string
		walkErr := chi.Walk(apiInstance.Router, func(method, route string, _ http.Handler, middlewares ...func(http.Handler) http.Handler) error {
			if safeMethods[method] {
				return nil
			}
			var authenticated, guarded bool
			for _, middleware := range middlewares {
				name := runtime.FuncForPC(reflect.ValueOf(middleware).Pointer()).Name()
				name = strings.TrimPrefix(name, "github.com/moto-nrw/project-phoenix/")
				switch {
				case strings.HasPrefix(name, authenticator):
					authenticated = true
				case strings.HasPrefix(name, readOnlyGuard):
					guarded = true
				}
			}
			if authenticated && !guarded {
				unguarded = append(unguarded, method+" "+route)
			}
			return nil
		})
		require.NoError(t, walkErr)
		sort.Strings(unguarded)

		require.Emptyf(t, unguarded,
			"these routes authenticate a JWT without api/common.ReadOnlyPreviewMiddleware — a read-only staff-preview token could write through them. Add the middleware right after jwt.Authenticator in the group that mounts them:\n%s",
			strings.Join(unguarded, "\n"))
	})
}

var compilerGeneratedNamePart = regexp.MustCompile(`func[0-9]+|\.[0-9]+`)

// stableMiddlewareTable removes compiler-dependent wrapper provenance while
// retaining each route's middleware order and semantic function names. Go may
// report the same closure as either Resource.Router.SetContentType or the
// underlying render.SetContentType when an unrelated dependency changes its
// inlining budget; that is not an HTTP middleware change.
func stableMiddlewareTable(table string) string {
	lines := strings.Split(strings.TrimSuffix(table, "\n"), "\n")
	for i, line := range lines {
		prefix, chain, ok := strings.Cut(line, " -> ")
		if !ok {
			continue
		}
		names := strings.Split(chain, " > ")
		for j, name := range names {
			if !strings.HasSuffix(name, "#") {
				continue
			}
			name = strings.TrimSuffix(strings.TrimSuffix(name, "#"), ".")
			if lastDot := strings.LastIndexByte(name, '.'); lastDot >= 0 {
				name = name[lastDot+1:]
			}
			names[j] = name + "#"
		}
		lines[i] = prefix + " -> " + strings.Join(names, " > ")
	}
	return strings.Join(lines, "\n") + "\n"
}

// chiParamPattern matches one {param} placeholder (incl. regex-constrained
// ones like {id:[0-9]+}) for probe-URL substitution.
var chiParamPattern = regexp.MustCompile(`\{[^}]+\}`)

func checkIoTAuthMatrixGolden(t *testing.T, apiInstance *API) {
	t.Parallel()

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

func compareGolden(t *testing.T, path, got, hint string, normalizers ...func(string) string) {
	t.Helper()
	if *updateGoldens {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
		t.Logf("golden rewritten: %s", path)
		return
	}
	wantBytes, err := os.ReadFile(path) // #nosec G304 -- repo-local testdata
	require.NoErrorf(t, err, "golden file %s missing — generate it with -update-goldens", path)
	want := string(wantBytes)
	for _, normalize := range normalizers {
		want = normalize(want)
		got = normalize(got)
	}
	if want != got {
		t.Errorf("%s\n\ngolden diff for %s:\n%s", hint, path, unifiedDiff(want, got))
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

// TestFullProductionRouterGolden is the only API test that composes the complete
// production graph. Its subtests cover contracts of the assembled router.
func TestFullProductionRouterGolden(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	called := false
	err := WithRuntime(context.Background(), ServeConfig{Port: "127.0.0.1:0", Logger: slog.Default()}, func(runtime *Runtime) error {
		called = true
		require.NotNil(t, runtime.worker)
		api, ok := runtime.Handler().(*API)
		require.True(t, ok)
		if *runtimeCheckpointOutput != "" {
			measureRuntimeCheckpoint(t, runtime)
		}
		// Wait for parallel contract subtests before WithRuntime closes its resources.
		t.Run("contracts", func(t *testing.T) {
			t.Run("route table", func(t *testing.T) { checkRouteTableGolden(t, api) })
			t.Run("IoT auth matrix", func(t *testing.T) { checkIoTAuthMatrixGolden(t, api) })
			t.Run("school scope matrix", func(t *testing.T) { checkSchoolScopeMatrix(t, api) })
			t.Run("caregiver wiring", func(t *testing.T) { checkCaregiverWiring(t, api) })
			t.Run("enrollment submission", func(t *testing.T) { checkEnrollmentSubmissionGolden(t, api) })
			t.Run("rate limited operator invitations", func(t *testing.T) { checkOperatorInvitationMount(t, api) })
		})
		return nil
	})
	require.NoError(t, err)
	require.True(t, called)
	t.Run("invalid runtime dependencies", checkRuntimeRejectsMissingDependencies)
}
