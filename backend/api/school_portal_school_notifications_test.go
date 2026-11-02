// Notifications in the school portal (#2208): a Lehrkraft manages her own
// decisions and devices through /school/notifications with a school token
// and sees only the catalogue the school portal offers.
package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	timetable "github.com/moto-nrw/project-phoenix/modules/timetable/http"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// schoolNotificationPreferences is the school portal's observed wire response.
type schoolNotificationPreferences struct {
	Types []schoolNotificationPreference `json:"types"`
}
type schoolNotificationPreference struct {
	Key       string `json:"key"`
	Available bool   `json:"available"`
	Enabled   bool   `json:"enabled"`
}

func schoolPreference(t *testing.T, overview schoolNotificationPreferences, key string) schoolNotificationPreference {
	t.Helper()
	for _, entry := range overview.Types {
		if entry.Key == key {
			return entry
		}
	}
	t.Fatalf("school catalogue has no %s", key)
	return schoolNotificationPreference{}
}

func TestSchoolNotificationPreferences(t *testing.T) {
	t.Parallel()
	db, resource, tenantID, _ := setupSchoolRoute(t)
	schoolRouter := newSchoolChiRouter(resource)

	require.NoError(t, resource.Timetable.(*timetable.Resource).SettingsService.(schoolPortalSettingsWriter).SetValue(
		testpkg.TenantContext(tenantID), configModel.KeyStaffMessagingEnabled, true, nil, nil,
	))

	_, teacherAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "Pref", fmt.Sprintf("Lehrkraft-%d", time.Now().UnixNano()))
	testpkg.EnsureAccountTenant(t, db, teacherAccount.ID, tenantID)
	testpkg.AssignLehrkraftSystemRole(t, db, teacherAccount.ID, tenantID)
	claims := sessiontoken.AppClaims{
		ID: int(teacherAccount.ID), Sub: teacherAccount.Email,
		Roles: []string{"lehrkraft"}, TenantID: tenantID, Scope: tenant.ScopeSchool,
	}

	// The school catalogue: only what a Lehrkraft can actually receive. The
	// OGS supervision reminders never address her, so they are not offered.
	req := httptest.NewRequest(http.MethodGet, "/notifications/preferences/", nil)
	rec := testutil.ExecuteWithAuth(t, schoolRouter, req, claims)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	overview := decodeSchoolPortalData[schoolNotificationPreferences](t, rec.Body.Bytes())
	keys := make([]string, 0, len(overview.Types))
	for _, entry := range overview.Types {
		keys = append(keys, entry.Key)
	}
	assert.Equal(t, []string{"school_class_absence_reported", "staff_message", "school_parent_conference_booking"}, keys)
	assert.True(t, schoolPreference(t, overview, "staff_message").Available, "the school switched the chat on")
	assert.False(t, schoolPreference(t, overview, "staff_message").Enabled, "opt-in: nothing is on until the person decides")
	// The class sick-note notice (#4792) is offered and starts off as well.
	assert.True(t, schoolPreference(t, overview, "school_class_absence_reported").Available)
	assert.False(t, schoolPreference(t, overview, "school_class_absence_reported").Enabled)
	// The Elternsprechtag booking notice (#4801) is offered and starts off.
	assert.True(t, schoolPreference(t, overview, "school_parent_conference_booking").Available)
	assert.False(t, schoolPreference(t, overview, "school_parent_conference_booking").Enabled)

	// Opting in writes the shared (account, type) row ...
	req = httptest.NewRequest(http.MethodPut, "/notifications/preferences/"+"staff_message",
		strings.NewReader(`{"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec = testutil.ExecuteWithAuth(t, schoolRouter, req, claims)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	req = httptest.NewRequest(http.MethodGet, "/notifications/preferences/", nil)
	rec = testutil.ExecuteWithAuth(t, schoolRouter, req, claims)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	overview = decodeSchoolPortalData[schoolNotificationPreferences](t, rec.Body.Bytes())
	assert.True(t, schoolPreference(t, overview, "staff_message").Enabled)

	// ... and only the offered types can be decided from here.
	req = httptest.NewRequest(http.MethodPut, "/notifications/preferences/"+"pickup_upcoming",
		strings.NewReader(`{"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec = testutil.ExecuteWithAuth(t, schoolRouter, req, claims)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	// "Alle deaktivieren" switches the school catalogue off.
	req = httptest.NewRequest(http.MethodDelete, "/notifications/preferences/", nil)
	rec = testutil.ExecuteWithAuth(t, schoolRouter, req, claims)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	req = httptest.NewRequest(http.MethodGet, "/notifications/preferences/", nil)
	rec = testutil.ExecuteWithAuth(t, schoolRouter, req, claims)
	overview = decodeSchoolPortalData[schoolNotificationPreferences](t, rec.Body.Bytes())
	assert.False(t, schoolPreference(t, overview, "staff_message").Enabled)

	// Scope isolation, symmetric to every other school surface.
	req = httptest.NewRequest(http.MethodGet, "/notifications/preferences/", nil)
	rec = testutil.ExecuteWithAuth(t, schoolRouter, req, sessiontoken.AppClaims{
		ID: int(teacherAccount.ID), Sub: teacherAccount.Email, TenantID: tenantID, Scope: tenant.ScopeTenant,
	})
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}
