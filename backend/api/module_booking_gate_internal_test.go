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
	handler := moduleRoutesMiddleware(booking)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

// A platform token binds no school; the operator portal is not locked.
func TestModuleLockLetsRequestsWithoutSchoolPass(t *testing.T) {
	t.Parallel()
	switches := moduleSwitches{err: errors.New("must not be read")}
	_, reached := serveThroughModuleLock(t, switches, 0, "/api/meal-plan/")
	assert.True(t, reached)
}
