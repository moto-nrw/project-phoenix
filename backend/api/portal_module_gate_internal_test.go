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
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	settings "github.com/moto-nrw/project-phoenix/modules/settings"
)

// guardianSchoolList answers the guardian schools of every account alike.
type guardianSchoolList struct {
	schoolIDs []int64
	err       error
}

func (l guardianSchoolList) ListGuardianSchoolIDs(context.Context, int64) ([]int64, error) {
	return l.schoolIDs, l.err
}

func serveThroughParentPortalLock(t *testing.T, switches moduleSwitches, schools guardianSchoolList, path string) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	booking, err := settings.NewModuleBooking(switches)
	require.NoError(t, err)
	reached := false
	handler := moduleRoutesMiddleware(booking, schools)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: 4895, Scope: "parent"}))
		requireParentPortal(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			reached = true
			w.WriteHeader(http.StatusNoContent)
		})).ServeHTTP(w, r)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	return response, reached
}

func TestParentPortalLockRefusesAnAccountWithoutAnyBookedSchool(t *testing.T) {
	t.Parallel()
	switches := moduleSwitches{cancelled: map[int64]settings.ModuleKey{41: settings.ModuleElternApp}}

	response, reached := serveThroughParentPortalLock(t, switches, guardianSchoolList{schoolIDs: []int64{41}}, "/parent/me/children")

	require.False(t, reached)
	require.Equal(t, http.StatusForbidden, response.Code)
	var body struct {
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, common.CodeModuleNotBooked, body.Code)
	assert.Equal(t, "eltern_app", body.Details["module"])
	assert.Equal(t, "Eltern-App", body.Details["module_label"])
}

func TestParentPortalLockOpensWithOneBookedSchool(t *testing.T) {
	t.Parallel()
	switches := moduleSwitches{cancelled: map[int64]settings.ModuleKey{41: settings.ModuleElternApp}}

	_, reached := serveThroughParentPortalLock(t, switches, guardianSchoolList{schoolIDs: []int64{41, 42}}, "/parent/me/children")

	assert.True(t, reached, "a guardian with one school that booked Eltern-App keeps the portal")
}

func TestParentPortalLockIgnoresOtherModules(t *testing.T) {
	t.Parallel()
	switches := moduleSwitches{cancelled: map[int64]settings.ModuleKey{41: settings.ModuleUnterrichtVormittag}}

	_, reached := serveThroughParentPortalLock(t, switches, guardianSchoolList{schoolIDs: []int64{41}}, "/parent/me/children")

	assert.True(t, reached)
}

func TestParentPortalLockFailsClosedOnReadErrors(t *testing.T) {
	t.Parallel()

	response, reached := serveThroughParentPortalLock(t, moduleSwitches{err: errors.New("settings down")}, guardianSchoolList{schoolIDs: []int64{41}}, "/parent/me/children")
	assert.False(t, reached)
	assert.Equal(t, http.StatusInternalServerError, response.Code)

	response, reached = serveThroughParentPortalLock(t, moduleSwitches{}, guardianSchoolList{err: errors.New("identity down")}, "/parent/me/children")
	assert.False(t, reached)
	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestSchoolPortalLockRefusesASchoolWithoutTheModule(t *testing.T) {
	t.Parallel()
	const cancelled, booked int64 = 4895, 4896
	switches := moduleSwitches{cancelled: map[int64]settings.ModuleKey{cancelled: settings.ModuleUnterrichtVormittag}}

	response, reached := serveThroughModuleLock(t, switches, cancelled, "/school/class-day/today")
	require.False(t, reached)
	require.Equal(t, http.StatusForbidden, response.Code)

	_, reached = serveThroughModuleLock(t, switches, booked, "/school/class-day/today")
	assert.True(t, reached, "another school's booking does not leak")

	_, reached = serveThroughModuleLock(t, switches, cancelled, "/school/auth/login")
	assert.True(t, reached, "the login route carries no school; the login itself refuses")
}
