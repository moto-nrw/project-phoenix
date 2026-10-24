package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	settings "github.com/moto-nrw/project-phoenix/modules/settings"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// moduleRoutesLockedElsewhere are the path prefixes whose module routes run
// without the tenant scope, with the place that locks them: a later issue, or
// the owner itself where the route resolves its school from the path. Every
// other route of an additional module must pass api.TenantScopeMiddleware,
// which is where the module lock sits.
var moduleRoutesLockedElsewhere = map[string]string{
	"/parent":                            "#4895 Elternportal",
	"/parent-sse":                        "#4895 Elternportal",
	"/parent-news-attachments":           "#4895 Elternportal",
	"/auth/guardian-invitations":         "#4895 Elternportal: Einladung annehmen stellt die Eltern-Sitzung aus",
	"/school":                            "#4895 Schulportal",
	"/school-sse":                        "#4895 Schulportal",
	"/api/iot":                           "#4896 Geräte und PyrePortal",
	"/api/students/*/rfid":               "#4896 Geräte: Armband am Tablet zuordnen, Geräte-Schlüssel statt Sitzung",
	"/public/enrollment-legal-documents": "öffentliches Anmeldeformular, prüft die Anmeldung über die Schule im Pfad",
	"/public/enrollment-form-legal-documents": "öffentliches Anmeldeformular, prüft die Anmeldung über die Schule im Pfad",
	// Public enrollment resolves its school from the slug. Phase list,
	// form bootstrap, uploads and submit refuse with enrollment.disabled
	// unless the school booked Online-Anmeldung (EnrollmentIntakeValues).
	"/api/enrollment/*/submit":              "öffentliche Anmeldung, Modul über enrollment.enabled",
	"/api/enrollment/*/uploads":             "öffentliche Anmeldung, Modul über enrollment.enabled",
	"/api/enrollment/phases/public":         "öffentliche Anmeldung, Modul über enrollment.enabled",
	"/api/enrollment/form-bootstrap/public": "öffentliche Anmeldung, Modul über enrollment.enabled",
	"/api/enrollment/schema/public":         "öffentliche Anmeldung: Formularaufbau einer Phase ohne Personendaten",
	"/api/enrollment/care-offerings/public": "öffentliche Anmeldung: Angebotsliste einer Phase ohne Personendaten",
	"/api/enrollment/legal":                 "öffentliche Anmeldung: Rechtstexte der Schule",
	"/api/enrollment/captcha-config":        "öffentliche Anmeldung: Captcha-Schlüssel",
	// Status links of submitted applications stay usable: the family keeps
	// access to its own data and can withdraw (decision in #4893).
	"/api/enrollment/requests": "Statuslink einer abgeschickten Anmeldung, bleibt bewusst erreichbar",
}

// checkModuleRouteMap pins the module of every production route (#4893):
// each route belongs to exactly one module or to Basis, a request path
// resolves like its route pattern, and every route of an additional module
// carries the module lock unless a later issue owns its surface.
func checkModuleRouteMap(t *testing.T, api *goldenRuntime) {
	t.Parallel()

	type walked struct {
		route  string
		chain  []string
		module settings.ModuleKey
	}
	var routes []walked
	var unassigned []string
	walkErr := chi.Walk(api.router, func(method, route string, _ http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		module, ok := moduleForRoute(route)
		if !ok {
			unassigned = append(unassigned, method+" "+route)
			return nil
		}
		names := make([]string, 0, len(middlewares))
		for _, middleware := range middlewares {
			name := runtime.FuncForPC(reflect.ValueOf(middleware).Pointer()).Name()
			names = append(names, strings.TrimPrefix(name, "github.com/moto-nrw/project-phoenix/"))
		}
		routes = append(routes, walked{route: method + " " + route, chain: names, module: module})
		return nil
	})
	require.NoError(t, walkErr)
	sort.Strings(unassigned)
	require.Emptyf(t, unassigned,
		"these routes belong to no module and not to Basis. Add a rule to moduleRouteRules (api/module_routes.go) and justify a borderline case in docs/reference/modulschalter.md:\n%s",
		strings.Join(unassigned, "\n"))

	t.Run("a request path resolves like its route pattern", func(t *testing.T) {
		for _, route := range routes {
			_, pattern, _ := strings.Cut(route.route, " ")
			concrete := chiParamPattern.ReplaceAllString(pattern, "4893")
			concrete = strings.ReplaceAll(concrete, "/*", "/rest")
			module, ok := moduleForRoute(concrete)
			assert.Truef(t, ok, "%s: %s matches no rule", route.route, concrete)
			assert.Equalf(t, route.module, module, "%s: %s resolves to another module than its pattern", route.route, concrete)
		}
	})

	t.Run("every route of a module passes the module lock", func(t *testing.T) {
		var unlocked []string
		for _, route := range routes {
			if route.module == moduleRouteBasis || lockedElsewhere(route.route) {
				continue
			}
			if !slices.Contains(route.chain, "api.TenantScopeMiddleware") {
				unlocked = append(unlocked, route.route)
			}
		}
		require.Emptyf(t, unlocked,
			"these routes of an additional module do not pass api.TenantScopeMiddleware, so a school without the module still reaches them. Mount them in a tenant chain or name the issue that locks their surface in moduleRoutesLockedElsewhere:\n%s",
			strings.Join(unlocked, "\n"))
	})

	var table strings.Builder
	byModule := map[settings.ModuleKey][]string{}
	for _, route := range routes {
		byModule[route.module] = append(byModule[route.module], route.route)
	}
	table.WriteString("# Each heading is a module key (basis = moto Basis); the routes below it belong to it.\n")
	keys := []settings.ModuleKey{moduleRouteBasis}
	for _, module := range settings.Modules() {
		keys = append(keys, module.Key)
	}
	for _, key := range keys {
		heading := string(key)
		if key == moduleRouteBasis {
			heading = "basis"
		}
		entries := byModule[key]
		if len(entries) == 0 {
			continue
		}
		sort.Strings(entries)
		table.WriteString("\n" + heading + "\n")
		for _, entry := range entries {
			table.WriteString("\t" + entry + "\n")
		}
	}
	compareGolden(t, filepath.Join("testdata", "route_modules.golden"), table.String(),
		"the module of a route changed — if intentional, regenerate with -update-goldens and justify the assignment in the PR description")
}

func lockedElsewhere(route string) bool {
	_, pattern, _ := strings.Cut(route, " ")
	segments := routeSegments(pattern)
	for prefix := range moduleRoutesLockedElsewhere {
		if _, ok := matchRoutePrefix(routeSegments(prefix), segments); ok {
			return true
		}
	}
	return false
}

// checkModuleBookingGate drives the lock through the production router: a
// school that cancelled a module gets settings.module_not_booked on its
// routes, a second school with the module keeps them, Basis routes stay
// reachable, and booking again finds the data unchanged (#4893).
func checkModuleBookingGate(t *testing.T, api *goldenRuntime) {
	t.Helper()
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	cancelled := testpkg.Tenant(t)
	booked, _ := testpkg.CreateTestTenant(t, db)
	_, cancelledAdmin := testpkg.CreateTestStaffWithAccountForTenant(t, db, cancelled, "Modul", "Abbestellt")
	_, bookedAdmin := testpkg.CreateTestStaffWithAccountForTenant(t, db, booked, "Modul", "Gebucht")
	cancelledToken := testpkg.MintTestJWT(t, testpkg.AdminTestClaimsForTenant(int(cancelledAdmin.ID), cancelled))
	bookedToken := testpkg.MintTestJWT(t, testpkg.AdminTestClaimsForTenant(int(bookedAdmin.ID), booked))
	child := testpkg.CreateTestStudent(t, db, "Mila", "Modul", "2a")

	request := func(token, method, path, body string) (int, map[string]any) {
		response := checkpointRequest(api, checkpointScenario{Method: method, Path: path, Authenticated: true, Body: body}, token)
		var payload map[string]any
		_ = json.Unmarshal(response.Body.Bytes(), &payload)
		return response.Code, payload
	}
	setBooked := func(tenantID int64, module settings.ModuleKey, value bool) {
		definition, ok := settings.LookupModule(module)
		require.True(t, ok)
		testpkg.SetTestTenantSetting(t, db, tenantID, definition.SettingKey, fmt.Sprint(value))
	}

	// One route of every module this PR locks. Basis data the routes show
	// stays readable through its own route.
	cases := []struct {
		module settings.ModuleKey
		method string
		path   string
	}{
		{settings.ModuleMittagessen, http.MethodGet, "/api/meal-plan/"},
		{settings.ModuleElternApp, http.MethodGet, "/api/messages/unread-count"},
		{settings.ModuleElternApp, http.MethodGet, "/api/guardians/invitations/pending"},
		{settings.ModuleOnlineAnmeldung, http.MethodGet, "/api/enrollment/phases/"},
		{settings.ModuleDienstplan, http.MethodGet, "/api/shift-types/"},
		{settings.ModuleZeiterfassung, http.MethodGet, "/api/work-time-models/"},
		{settings.ModuleZeiterfassung, http.MethodGet, "/api/time-tracking/current"},
		{settings.ModulePersonalakte, http.MethodGet, "/api/staff/documents-directory"},
	}
	for _, tc := range cases {
		setBooked(cancelled, tc.module, false)
	}
	t.Cleanup(func() {
		for _, module := range settings.Modules() {
			setBooked(cancelled, module.Key, true)
		}
	})

	for _, tc := range cases {
		status, body := request(cancelledToken, tc.method, tc.path, "{}")
		require.Equalf(t, http.StatusForbidden, status, "%s %s without %s: %v", tc.method, tc.path, tc.module, body)
		require.Equalf(t, common.CodeSettingsModuleNotBooked, body["code"], "%s %s: %v", tc.method, tc.path, body)
		details, _ := body["details"].(map[string]any)
		require.Equalf(t, string(tc.module), details["module"], "%s %s names its module: %v", tc.method, tc.path, body)
		require.NotEmptyf(t, details["module_label"], "%s %s names the module for people: %v", tc.method, tc.path, body)

		status, body = request(bookedToken, tc.method, tc.path, "{}")
		require.NotEqualf(t, common.CodeSettingsModuleNotBooked, body["code"], "%s %s for the school that booked %s: %d %v", tc.method, tc.path, tc.module, status, body)
	}

	t.Run("Basis stays reachable", func(t *testing.T) {
		for _, path := range []string{"/api/students/", fmt.Sprintf("/api/students/%d", child.ID), "/api/staff/", "/api/settings/schema"} {
			status, body := request(cancelledToken, http.MethodGet, path, "")
			require.Equalf(t, http.StatusOK, status, "GET %s: %v", path, body)
		}
	})

	t.Run("cancelling keeps the data", func(t *testing.T) {
		setBooked(cancelled, settings.ModuleDienstplan, true)
		status, body := request(cancelledToken, http.MethodPost, "/api/shift-types/", `{"name":"Frühdienst 4893","color":"#83CD2D"}`)
		require.Equalf(t, http.StatusCreated, status, "create shift type: %v", body)

		setBooked(cancelled, settings.ModuleDienstplan, false)
		status, body = request(cancelledToken, http.MethodGet, "/api/shift-types/", "")
		require.Equalf(t, http.StatusForbidden, status, "list while cancelled: %v", body)

		setBooked(cancelled, settings.ModuleDienstplan, true)
		response := checkpointRequest(api, checkpointScenario{Method: http.MethodGet, Path: "/api/shift-types/", Authenticated: true}, cancelledToken)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		require.Contains(t, response.Body.String(), "Frühdienst 4893")
	})
}
