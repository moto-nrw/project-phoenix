// Notifications in the school portal (#2208): a Lehrkraft manages her own
// decisions and devices through /school/notifications with a school token,
// sees only the catalogue the school portal offers, and her devices are
// recorded as school-portal devices.
package school_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	notificationsService "github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestSchoolNotificationPreferences(t *testing.T) {
	t.Parallel()
	db, factory, tenantID, _ := setupSchoolTest(t)
	schoolRouter := newSchoolChiRouter(db, factory)

	require.NoError(t, factory.Settings.SetValue(
		testpkg.TenantContext(tenantID), configModel.KeyStaffMessagingEnabled, true, nil, nil,
	))

	_, teacherAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "Pref", fmt.Sprintf("Lehrkraft-%d", time.Now().UnixNano()))
	testpkg.EnsureAccountTenant(t, db, teacherAccount.ID, tenantID)
	testpkg.AssignLehrkraftSystemRole(t, db, teacherAccount.ID, tenantID)
	claims := jwt.AppClaims{
		ID: int(teacherAccount.ID), Sub: teacherAccount.Email,
		Roles: []string{"lehrkraft"}, TenantID: tenantID, Scope: tenant.ScopeSchool,
	}

	// The school catalogue: only what a Lehrkraft can actually receive. The
	// OGS supervision reminders never address her, so they are not offered.
	req := httptest.NewRequest(http.MethodGet, "/notifications/preferences/", nil)
	rec := testutil.ExecuteWithAuth(t, schoolRouter, req, claims)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	overview := decodeData[notificationsService.PreferenceOverview](t, rec.Body.Bytes())
	keys := make([]string, 0, len(overview.Types))
	for _, entry := range overview.Types {
		keys = append(keys, entry.Key)
	}
	assert.Equal(t, []string{notificationsService.TypeStaffMessage}, keys)
	assert.True(t, overview.Types[0].Available, "the school switched the chat on")
	assert.False(t, overview.Types[0].Enabled, "opt-in: nothing is on until the person decides")

	// Opting in writes the shared (account, type) row ...
	req = httptest.NewRequest(http.MethodPut, "/notifications/preferences/"+notificationsService.TypeStaffMessage,
		strings.NewReader(`{"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec = testutil.ExecuteWithAuth(t, schoolRouter, req, claims)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	req = httptest.NewRequest(http.MethodGet, "/notifications/preferences/", nil)
	rec = testutil.ExecuteWithAuth(t, schoolRouter, req, claims)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	overview = decodeData[notificationsService.PreferenceOverview](t, rec.Body.Bytes())
	assert.True(t, overview.Types[0].Enabled)

	// ... and only the offered types can be decided from here.
	req = httptest.NewRequest(http.MethodPut, "/notifications/preferences/"+notificationsService.TypePickupUpcoming,
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
	overview = decodeData[notificationsService.PreferenceOverview](t, rec.Body.Bytes())
	assert.False(t, overview.Types[0].Enabled)

	// Scope isolation, symmetric to every other school surface.
	req = httptest.NewRequest(http.MethodGet, "/notifications/preferences/", nil)
	rec = testutil.ExecuteWithAuth(t, schoolRouter, req, jwt.AppClaims{
		ID: int(teacherAccount.ID), Sub: teacherAccount.Email, TenantID: tenantID, Scope: tenant.ScopeTenant,
	})
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}

func TestSchoolPushSubscriptionIsRecordedAsSchoolDevice(t *testing.T) {
	t.Parallel()
	db, factory, tenantID, _ := setupSchoolTest(t)
	schoolRouter := newSchoolChiRouter(db, factory)

	_, teacherAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "Push", fmt.Sprintf("Lehrkraft-%d", time.Now().UnixNano()))
	testpkg.EnsureAccountTenant(t, db, teacherAccount.ID, tenantID)
	testpkg.AssignLehrkraftSystemRole(t, db, teacherAccount.ID, tenantID)
	claims := jwt.AppClaims{
		ID: int(teacherAccount.ID), Sub: teacherAccount.Email,
		Roles: []string{"lehrkraft"}, TenantID: tenantID, Scope: tenant.ScopeSchool,
	}

	endpoint := fmt.Sprintf("https://fcm.googleapis.com/fcm/send/school-%d", time.Now().UnixNano())
	body := fmt.Sprintf(`{"endpoint":%q,"keys":{"p256dh":"BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_iDb9sByR6yeA-QtbErWx5yZJ-U5bAYzpL5y-Z-9YBI_S2xKDs","auth":"tBHItJI5svbpez7KI4CCXg"}}`, endpoint)
	req := httptest.NewRequest(http.MethodPost, "/notifications/push/subscriptions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := testutil.ExecuteWithAuth(t, schoolRouter, req, claims)
	if rec.Code == http.StatusConflict {
		// Without VAPID keys in the test environment the service refuses; the
		// portal wiring above is still exercised up to that point.
		t.Skip("web push is not configured in this test environment")
	}
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	assert.Equal(t, "school", pushSubscriptionPortal(t, db, tenantID, endpoint))
}
