package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	settings "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// moduleForRoute returns the module of a request path or a route pattern
// and whether any rule covers it.
func moduleForRoute(path string) (settings.ModuleKey, bool) {
	rule, ok := moduleRouteFor(path)
	return rule.module, ok
}

func TestModuleForRoutePrefersTheMostSpecificRule(t *testing.T) {
	t.Parallel()
	cases := map[string]settings.ModuleKey{
		"/":                                      moduleRouteBasis,
		"/api/staff/":                            moduleRouteBasis,
		"/api/staff/{id}":                        moduleRouteBasis,
		"/api/staff/17/groups":                   moduleRouteBasis,
		"/api/staff/17/documents":                settings.ModulePersonalakte,
		"/api/staff/{id}/documents/{documentId}": settings.ModulePersonalakte,
		"/api/staff/documents-directory":         settings.ModulePersonalakte,
		"/api/staff/17/time-tracking/history":    settings.ModuleZeiterfassung,
		"/api/staff/time-tracking/overview":      settings.ModuleZeiterfassung,
		"/api/timetable/instances":               moduleRouteBasis,
		"/api/timetable/shift-coverage":          settings.ModuleDienstplan,
		"/api/guardians/17":                      moduleRouteBasis,
		"/api/guardians/17/invite":               settings.ModuleElternApp,
		"/api/guardians/students/17/invite":      settings.ModuleElternApp,
		"/api/guardians/students/17/guardians":   moduleRouteBasis,
		"/api/iot/session/start":                 settings.ModuleNFCAnwesenheit,
		"/api/iot/staff-clock/check-in":          settings.ModuleZeiterfassung,
		"/parent/enrollments/":                   settings.ModuleOnlineAnmeldung,
		"/parent/me/children":                    settings.ModuleElternApp,
	}
	for path, want := range cases {
		got, ok := moduleForRoute(path)
		require.Truef(t, ok, "%s matches no rule", path)
		assert.Equalf(t, want, got, "%s", path)
	}
}

// The root rule covers only the root: a path under a new prefix belongs to
// no module until a rule names it.
func TestModuleForRouteHasNoCatchAll(t *testing.T) {
	t.Parallel()
	_, ok := moduleForRoute("/api/unknown-area/list")
	assert.False(t, ok)
	_, ok = moduleForRoute("/nowhere")
	assert.False(t, ok)
}

func TestModuleRouteRulesAreUnique(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, rule := range moduleRouteRules {
		require.Falsef(t, seen[rule.prefix], "rule %s is declared twice", rule.prefix)
		seen[rule.prefix] = true
		if rule.module != moduleRouteBasis {
			_, ok := settings.LookupModule(rule.module)
			require.Truef(t, ok, "rule %s names an unknown module %q", rule.prefix, rule.module)
		}
	}
}

// moduleSwitches answers the switches of each school; an unset switch is
// booked, as the registry default.
type moduleSwitches struct {
	cancelled map[int64]settings.ModuleKey
	err       error
}

func (s moduleSwitches) ResolveBoolForTenant(_ context.Context, tenantID int64, key string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	module, ok := s.cancelled[tenantID]
	if !ok {
		return true, nil
	}
	definition, _ := settings.LookupModule(module)
	return definition.SettingKey != key, nil
}

func serveThroughModuleLock(t *testing.T, switches moduleSwitches, tenantID int64, path string) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	booking, err := settings.NewModuleBooking(switches)
	require.NoError(t, err)
	reached := false
	handler := moduleRoutesMiddleware(booking, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tenantID > 0 {
			r = r.WithContext(tenant.WithTenantID(r.Context(), tenantID))
		}
		requireBookedModule(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			reached = true
			w.WriteHeader(http.StatusNoContent)
		})).ServeHTTP(w, r)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	return response, reached
}

func TestModuleLockRefusesACancelledModuleOfItsSchoolOnly(t *testing.T) {
	t.Parallel()
	const cancelled, booked int64 = 4893, 4894
	switches := moduleSwitches{cancelled: map[int64]settings.ModuleKey{cancelled: settings.ModuleZeiterfassung}}

	response, reached := serveThroughModuleLock(t, switches, cancelled, "/api/time-tracking/current")
	require.False(t, reached)
	require.Equal(t, http.StatusForbidden, response.Code)
	var body struct {
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, common.CodeModuleNotBooked, body.Code)
	assert.Equal(t, "zeiterfassung", body.Details["module"])
	assert.Equal(t, "Zeiterfassung", body.Details["module_label"])

	_, reached = serveThroughModuleLock(t, switches, booked, "/api/time-tracking/current")
	assert.True(t, reached, "the other school keeps its module")
	_, reached = serveThroughModuleLock(t, switches, cancelled, "/api/students/")
	assert.True(t, reached, "Basis stays reachable")
	_, reached = serveThroughModuleLock(t, switches, cancelled, "/api/staff-shifts/")
	assert.True(t, reached, "another booked module stays reachable")
}

func TestModuleLockTreatsAFailedReadAsAServerError(t *testing.T) {
	t.Parallel()
	response, reached := serveThroughModuleLock(t, moduleSwitches{err: errors.New("settings unavailable")}, 4893, "/api/meal-plan/")
	assert.False(t, reached)
	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

// The parents portal and the public enrollment are locked by their sessions
// and owners (#4895), not here.
func TestModuleLockLeavesSurfacesLockedElsewhere(t *testing.T) {
	t.Parallel()
	switches := moduleSwitches{cancelled: map[int64]settings.ModuleKey{4893: settings.ModuleElternApp}}
	for _, path := range []string{"/parent/me/children", "/api/enrollment/requests/token"} {
		_, reached := serveThroughModuleLock(t, switches, 4893, path)
		assert.Truef(t, reached, "%s", path)
	}
}

// The device administration of the team portal follows NFC-Anwesenheit in
// the tenant chain like every other module route (#4896).
func TestModuleLockRefusesDeviceAdministrationWithoutNFC(t *testing.T) {
	t.Parallel()
	switches := moduleSwitches{cancelled: map[int64]settings.ModuleKey{4896: settings.ModuleNFCAnwesenheit}}
	response, reached := serveThroughModuleLock(t, switches, 4896, "/api/iot/")
	require.False(t, reached)
	assertModuleNotBooked(t, response, settings.ModuleNFCAnwesenheit)
}

// serveThroughDeviceLock runs a request through the router-wide
// classification and a device authenticator wrapped by
// gateDeviceAuthentication; the stand-in authenticator binds the device's
// school, as Device Fleet does.
func serveThroughDeviceLock(t *testing.T, switches moduleSwitches, tenantID int64, path string) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	booking, err := settings.NewModuleBooking(switches)
	require.NoError(t, err)
	authenticate := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithTenantID(r.Context(), tenantID)))
		})
	}
	reached := false
	handler := moduleRoutesMiddleware(booking, nil)(gateDeviceAuthentication(authenticate)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	})))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, nil))
	return response, reached
}

func assertModuleNotBooked(t *testing.T, response *httptest.ResponseRecorder, module settings.ModuleKey) {
	t.Helper()
	require.Equal(t, http.StatusForbidden, response.Code)
	var body struct {
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, common.CodeModuleNotBooked, body.Code)
	assert.Equal(t, string(module), body.Details["module"])
	assert.NotEmpty(t, body.Details["module_label"])
}

// Every kiosk route needs NFC-Anwesenheit: scans, sessions, the device's
// configuration and the armband assignment at the tablet (#4896).
func TestDeviceLockRefusesKioskRoutesWithoutNFC(t *testing.T) {
	t.Parallel()
	const cancelled, booked int64 = 4896, 4897
	switches := moduleSwitches{cancelled: map[int64]settings.ModuleKey{cancelled: settings.ModuleNFCAnwesenheit}}
	for _, path := range []string{"/api/iot/checkin", "/api/iot/ping", "/api/iot/session/start", "/api/iot/config", "/api/iot/attendance/toggle", "/api/students/17/rfid"} {
		response, reached := serveThroughDeviceLock(t, switches, cancelled, path)
		assert.Falsef(t, reached, "%s", path)
		assertModuleNotBooked(t, response, settings.ModuleNFCAnwesenheit)

		_, reached = serveThroughDeviceLock(t, switches, booked, path)
		assert.Truef(t, reached, "%s: the other school keeps its kiosk", path)
	}
}

// Stamping at the kiosk needs Zeiterfassung and NFC-Anwesenheit; missing
// either one refuses with that module (#4812, decision 3).
func TestDeviceLockStaffClockNeedsZeiterfassungAndNFC(t *testing.T) {
	t.Parallel()
	for _, module := range []settings.ModuleKey{settings.ModuleZeiterfassung, settings.ModuleNFCAnwesenheit} {
		switches := moduleSwitches{cancelled: map[int64]settings.ModuleKey{4896: module}}
		for _, path := range []string{"/api/iot/staff-clock", "/api/iot/staff-clock/state"} {
			response, reached := serveThroughDeviceLock(t, switches, 4896, path)
			assert.Falsef(t, reached, "%s without %s", path, module)
			assertModuleNotBooked(t, response, module)
		}
	}
	// Without Zeiterfassung the rest of the kiosk stays open.
	switches := moduleSwitches{cancelled: map[int64]settings.ModuleKey{4896: settings.ModuleZeiterfassung}}
	_, reached := serveThroughDeviceLock(t, switches, 4896, "/api/iot/checkin")
	assert.True(t, reached)
	_, reached = serveThroughDeviceLock(t, moduleSwitches{}, 4896, "/api/iot/staff-clock")
	assert.True(t, reached, "both booked")
}

func TestDeviceLockTreatsAFailedReadAsAServerError(t *testing.T) {
	t.Parallel()
	response, reached := serveThroughDeviceLock(t, moduleSwitches{err: errors.New("settings unavailable")}, 4896, "/api/iot/checkin")
	assert.False(t, reached)
	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

// A platform token binds no school; the operator portal is not locked.
func TestModuleLockLetsRequestsWithoutSchoolPass(t *testing.T) {
	t.Parallel()
	switches := moduleSwitches{err: errors.New("must not be read")}
	_, reached := serveThroughModuleLock(t, switches, 0, "/api/meal-plan/")
	assert.True(t, reached)
}
