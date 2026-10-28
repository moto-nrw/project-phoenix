package api

import (
	"context"
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
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

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

	t.Run("every surface locked elsewhere is a rule", func(t *testing.T) {
		rules := map[string]settings.ModuleKey{}
		for _, rule := range moduleRouteRules {
			rules[rule.prefix] = rule.module
		}
		for prefix := range moduleRoutesLockedElsewhere {
			module, ok := rules[prefix]
			require.Truef(t, ok, "moduleRoutesLockedElsewhere names %s, which is no rule", prefix)
			require.NotEqualf(t, moduleRouteBasis, module, "%s is Basis and needs no lock", prefix)
		}
	})

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
			// The school portal binds its school like the team portal and
			// passes the same lock in SchoolScopeMiddleware (#4895).
			// Kiosk routes carry no session; their device authenticator
			// passes the lock in gateDeviceAuthentication (#4896).
			if !slices.Contains(route.chain, "api.TenantScopeMiddleware") && !slices.Contains(route.chain, "api.SchoolScopeMiddleware") &&
				!slices.ContainsFunc(route.chain, isDeviceModuleGate) {
				unlocked = append(unlocked, route.route)
			}
		}
		require.Emptyf(t, unlocked,
			"these routes of an additional module pass neither api.TenantScopeMiddleware, api.SchoolScopeMiddleware nor a device authenticator behind gateDeviceAuthentication, so a school without the module still reaches them. Mount them in a tenant chain or name the issue that locks their surface in moduleRoutesLockedElsewhere:\n%s",
			strings.Join(unlocked, "\n"))
	})

	var table strings.Builder
	byModule := map[settings.ModuleKey][]string{}
	for _, route := range routes {
		byModule[route.module] = append(byModule[route.module], route.route)
	}
	table.WriteString("# Each heading is a module key (basis = moto Basis); the routes below it belong to it.\n")
	keys := []settings.ModuleKey{moduleRouteBasis}
	for _, module := range configModels.Modules() {
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
			_, pattern, _ := strings.Cut(entry, " ")
			rule, _ := moduleRouteFor(pattern)
			if reason, ok := moduleRoutesLockedElsewhere[rule.prefix]; ok {
				entry += "  (" + reason + ")"
			}
			table.WriteString("\t" + entry + "\n")
		}
	}
	compareGolden(t, filepath.Join("testdata", "route_modules.golden"), table.String(),
		"the module of a route changed — if intentional, regenerate with -update-goldens and justify the assignment in the PR description")
}

func isDeviceModuleGate(name string) bool {
	return strings.HasPrefix(name, "api.gateDeviceAuthentication.")
}

// lockedElsewhere reports whether the rule a route pattern resolves to
// belongs to a surface another place locks.
func lockedElsewhere(route string) bool {
	_, pattern, _ := strings.Cut(route, " ")
	rule, _ := moduleRouteFor(pattern)
	_, ok := moduleRoutesLockedElsewhere[rule.prefix]
	return ok
}

// checkModuleBookingGate drives the lock through the production router: a
// school that cancelled a module gets module.not_booked on its
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
		{settings.ModuleMittagessen, http.MethodGet, "/api/meal-plan/?week_start=2026-10-19"},
		{settings.ModuleMittagessen, http.MethodGet, "/api/meal-caterer/"},
		{settings.ModuleElternApp, http.MethodGet, "/api/messages/unread-count"},
		{settings.ModuleElternApp, http.MethodGet, "/api/guardians/invitations/pending"},
		{settings.ModuleOnlineAnmeldung, http.MethodGet, "/api/enrollment/phases/"},
		{settings.ModuleDienstplan, http.MethodGet, "/api/shift-types/"},
		{settings.ModuleDienstplan, http.MethodGet, "/api/time-tracking/shifts?from=2026-10-19&to=2026-10-25"},
		{settings.ModuleZeiterfassung, http.MethodGet, "/api/work-time-models/"},
		{settings.ModuleZeiterfassung, http.MethodGet, "/api/time-tracking/current"},
		{settings.ModulePersonalakte, http.MethodGet, "/api/staff/documents-directory"},
	}
	for _, tc := range cases {
		setBooked(cancelled, tc.module, false)
	}
	t.Cleanup(func() {
		for _, module := range configModels.Modules() {
			setBooked(cancelled, module.Key, true)
		}
	})

	for _, tc := range cases {
		status, body := request(cancelledToken, tc.method, tc.path, "{}")
		require.Equalf(t, http.StatusForbidden, status, "%s %s without %s: %v", tc.method, tc.path, tc.module, body)
		require.Equalf(t, common.CodeModuleNotBooked, body["code"], "%s %s: %v", tc.method, tc.path, body)
		details, _ := body["details"].(map[string]any)
		require.Equalf(t, string(tc.module), details["module"], "%s %s names its module: %v", tc.method, tc.path, body)
		require.NotEmptyf(t, details["module_label"], "%s %s names the module for people: %v", tc.method, tc.path, body)

		status, body = request(bookedToken, tc.method, tc.path, "{}")
		require.Equalf(t, http.StatusOK, status, "%s %s for the school that booked %s: %v", tc.method, tc.path, tc.module, body)
	}

	t.Run("the other school cancels independently", func(t *testing.T) {
		setBooked(booked, settings.ModuleMittagessen, false)
		t.Cleanup(func() { setBooked(booked, settings.ModuleMittagessen, true) })
		setBooked(cancelled, settings.ModuleMittagessen, true)
		t.Cleanup(func() { setBooked(cancelled, settings.ModuleMittagessen, false) })

		status, body := request(bookedToken, http.MethodGet, "/api/meal-plan/?week_start=2026-10-19", "")
		require.Equalf(t, http.StatusForbidden, status, "school B cancelled: %v", body)
		require.Equal(t, common.CodeModuleNotBooked, body["code"])
		status, body = request(cancelledToken, http.MethodGet, "/api/meal-plan/?week_start=2026-10-19", "")
		require.Equalf(t, http.StatusOK, status, "school A booked again: %v", body)
	})

	t.Run("Basis stays reachable", func(t *testing.T) {
		for _, path := range []string{
			"/api/students/", fmt.Sprintf("/api/students/%d", child.ID), "/api/staff/", "/api/settings/schema",
			// Basis inside /api/time-tracking while Zeiterfassung is cancelled.
			"/api/time-tracking/assignments?from=2026-10-19&to=2026-10-25",
			"/api/time-tracking/closing-days?from=2026-10-01&to=2026-10-31",
		} {
			status, body := request(cancelledToken, http.MethodGet, path, "")
			require.Equalf(t, http.StatusOK, status, "GET %s: %v", path, body)
		}
	})

	t.Run("device administration follows NFC-Anwesenheit", func(t *testing.T) {
		setBooked(cancelled, settings.ModuleNFCAnwesenheit, false)
		t.Cleanup(func() { setBooked(cancelled, settings.ModuleNFCAnwesenheit, true) })
		status, body := request(cancelledToken, http.MethodGet, "/api/iot/?device_type=terminal", "")
		require.Equalf(t, http.StatusForbidden, status, "device administration without NFC: %v", body)
		require.Equal(t, common.CodeModuleNotBooked, body["code"])
		status, body = request(bookedToken, http.MethodGet, "/api/iot/?device_type=terminal", "")
		require.Equalf(t, http.StatusOK, status, "the other school keeps its devices: %v", body)
	})

	t.Run("the kiosk follows NFC-Anwesenheit and the staff clock also Zeiterfassung", func(t *testing.T) {
		device := testpkg.CreateTestDeviceForTenant(t, db, cancelled, "module-4896")
		kiosk := func(method, path string) (int, map[string]any) {
			// The PyrePortal contract: device key and the school's device PIN
			// (the fresh-tenant default of security.ogs_device_pin).
			response := checkpointHeaderRequest(api, checkpointScenario{Method: method, Path: path, Body: "{}"}, "", 0,
				map[string]string{"Authorization": "Bearer " + *device.APIKey, "X-Staff-PIN": "1234"})
			var payload map[string]any
			_ = json.Unmarshal(response.Body.Bytes(), &payload)
			return response.Code, payload
		}
		refused := func(method, path string, module settings.ModuleKey) {
			t.Helper()
			status, body := kiosk(method, path)
			require.Equalf(t, http.StatusForbidden, status, "%s %s without %s: %v", method, path, module, body)
			require.Equalf(t, common.CodeModuleNotBooked, body["code"], "%s %s: %v", method, path, body)
			details, _ := body["details"].(map[string]any)
			require.Equalf(t, string(module), details["module"], "%s %s names its module: %v", method, path, body)
		}

		status, body := kiosk(http.MethodPost, "/api/iot/ping")
		require.Equalf(t, http.StatusOK, status, "both booked: %v", body)

		setBooked(cancelled, settings.ModuleNFCAnwesenheit, false)
		refused(http.MethodPost, "/api/iot/ping", settings.ModuleNFCAnwesenheit)
		refused(http.MethodGet, "/api/iot/config", settings.ModuleNFCAnwesenheit)
		refused(http.MethodPost, "/api/iot/staff-clock/state", settings.ModuleNFCAnwesenheit)
		setBooked(cancelled, settings.ModuleNFCAnwesenheit, true)

		setBooked(cancelled, settings.ModuleZeiterfassung, false)
		t.Cleanup(func() { setBooked(cancelled, settings.ModuleZeiterfassung, true) })
		refused(http.MethodPost, "/api/iot/staff-clock/state", settings.ModuleZeiterfassung)
		status, body = kiosk(http.MethodPost, "/api/iot/ping")
		require.Equalf(t, http.StatusOK, status, "without Zeiterfassung the kiosk keeps attendance: %v", body)
	})

	t.Run("status links close with Online-Anmeldung", func(t *testing.T) {
		phase := testpkg.CreateTestEnrollmentPhase(t, db)
		token := fmt.Sprintf("status-4893-%d", phase.ID)
		_, err := db.NewRaw(`INSERT INTO enrollment.requests (
			tenant_id, phase_id, guardian_first_name, guardian_last_name,
			guardian_email, consent_flags, legal_blocks_snapshot, custom_data,
			source_metadata, status_token, submitted_at
		) VALUES (?, ?, 'Status', 'Link', 'status-4893@example.test', '{}'::jsonb, '[]'::jsonb,
			'{}'::jsonb, '{}'::jsonb, ?, NOW())`, cancelled, phase.ID, token).Exec(context.Background())
		require.NoError(t, err)
		path := "/api/enrollment/requests/" + token
		public := func() int {
			return checkpointRequest(api, checkpointScenario{Method: http.MethodGet, Path: path}, "").Code
		}

		setBooked(cancelled, settings.ModuleOnlineAnmeldung, true)
		require.Equal(t, http.StatusOK, public(), "booked: the family reads its application")
		setBooked(cancelled, settings.ModuleOnlineAnmeldung, false)
		require.Equal(t, http.StatusNotFound, public(), "cancelled: the link answers like an expired one")
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
