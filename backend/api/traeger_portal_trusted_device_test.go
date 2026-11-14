// Router-level tests for the remembered device of the Träger-Portal (#6955):
// it is remembered only when the person asks at the code step, signs the next
// sign-in in without a code, is listed and revoked behind the Träger session,
// and opens only the Träger-Portal at its own Träger.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

const traegerTrustedDevicesPath = "/traeger/auth/mfa/trusted-devices"

// traegerLoginWithDevice is traegerLogin with the browser's remember-device
// cookie.
func traegerLoginWithDevice(t *testing.T, router http.Handler, email, password string, organizationID int64, cookie string) (int, traegerLoginAnswer) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"email": email, "password": password, "organization_id": organizationID})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/traeger/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: common.TrustedDeviceCookie, Value: cookie})
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var answer traegerLoginAnswer
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &answer), rec.Body.String())
	return rec.Code, answer
}

// traegerDeviceCookie is the remember-device cookie a response sets, or empty.
func traegerDeviceCookie(rec *httptest.ResponseRecorder) string {
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == common.TrustedDeviceCookie {
			return cookie.Value
		}
	}
	return ""
}

// traegerDevices reads the session's remembered devices.
func traegerDevices(t *testing.T, router http.Handler, bearer string, counter *testpkg.QueryCounter) (int, []common.TrustedDeviceDTO) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, traegerTrustedDevicesPath, nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
	if counter != nil {
		req = req.WithContext(counter.Context(req.Context()))
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		return rec.Code, nil
	}
	var devices []common.TrustedDeviceDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &devices), rec.Body.String())
	return rec.Code, devices
}

func traegerRevokeDevice(t *testing.T, router http.Handler, bearer string, deviceID int64) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("%s/%d", traegerTrustedDevicesPath, deviceID), nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// traegerDeviceRows counts the account's remembered devices at the Träger,
// active or not.
func traegerDeviceRows(t *testing.T, db *bun.DB, accountID, organizationID int64) int {
	t.Helper()
	var rows int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.mfa_trusted_devices
		WHERE account_id = ? AND organization_id = ? AND tenant_id IS NULL`, accountID, organizationID).Scan(context.Background(), &rows))
	return rows
}

// enrolledTraegerMember is an account with the e-mail factor set up and a
// membership at each Träger.
func enrolledTraegerMember(t *testing.T, db *bun.DB, identity identityaccess.Owner, password string, organizations ...int64) *testpkg.AccountFixture {
	t.Helper()
	account := traegerAccount(t, db, password)
	for _, organization := range organizations {
		testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleReadOnly)
	}
	require.NoError(t, enrollMFA(t.Context(), t, db, identity, account.ID))
	return account
}

func TestTraegerRemembersTheDeviceOnlyWhenAsked(t *testing.T) {
	t.Parallel()
	db, portal, identity, unit := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Gerät", true)
	account := enrolledTraegerMember(t, db, identity, password, organization)

	code, login := traegerLogin(t, portal, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "mfa_required", login.Status)
	require.NotNil(t, login.TrustedDeviceEnabled, "the code step offers remembering the device")
	assert.True(t, *login.TrustedDeviceEnabled, "the registry default allows it")
	require.NotNil(t, login.TrustedDeviceDays)
	assert.Equal(t, 90, *login.TrustedDeviceDays, "the registry default lifetime")

	rec := traegerRequest(t, portal, "/traeger/auth/mfa/verify", map[string]any{
		"challenge_token": login.ChallengeToken, "code": lastTraegerCode(t, portal, account.Email), "organization_id": organization,
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, traegerDeviceCookie(rec), "without the choice no device is remembered")
	assert.Zero(t, traegerDeviceRows(t, db, account.ID, organization))

	_, login = traegerLogin(t, portal, account.Email, password, 0)
	require.Equal(t, "mfa_required", login.Status)
	rec = traegerRequest(t, portal, "/traeger/auth/mfa/verify", map[string]any{
		"challenge_token": login.ChallengeToken, "code": lastTraegerCode(t, portal, account.Email), "remember_device": true,
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	cookie := traegerDeviceCookie(rec)
	require.NotEmpty(t, cookie, "the chosen device is remembered")
	assert.Equal(t, 1, traegerDeviceRows(t, db, account.ID, organization), "at the only Träger of the account, without a school")
	var session traegerLoginAnswer
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &session))
	assert.Equal(t, float64(organization), traegerTokenClaims(t, session.AccessToken)["org_id"])
	added := portal.mail.Messages()
	require.NotEmpty(t, added)
	notice := added[len(added)-1]
	assert.Equal(t, "trusted-device-added.html", notice.Template, "the holder hears of the new device")
	content, err := json.Marshal(notice.Content)
	require.NoError(t, err)
	var portalOf struct{ Traeger, Caterer bool }
	require.NoError(t, json.Unmarshal(content, &portalOf))
	assert.True(t, portalOf.Traeger, "the notice names the Träger-Portal")
	assert.False(t, portalOf.Caterer)

	code, remembered := traegerLoginWithDevice(t, portal, account.Email, password, 0, cookie)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "authenticated", remembered.Status, "the remembered device replaces the code")
	require.NotEmpty(t, remembered.RefreshToken)
	assert.Equal(t, float64(organization), traegerTokenClaims(t, remembered.AccessToken)["org_id"])
	assert.True(t, traegerSessionVerified(t, db, account.ID), "a device-proven session records its second factor")
	_, _, err = identity.RefreshTokenWithAudit(tenant.WithUnitOfWork(context.Background(), unit), remembered.RefreshToken, "", "", "")
	require.NoError(t, err, "a device-proven Träger session refreshes")

	code, devices := traegerDevices(t, portal, remembered.AccessToken, nil)
	require.Equal(t, http.StatusOK, code)
	require.Len(t, devices, 1)
	assert.NotEmpty(t, devices[0].ExpiresAt)
	assert.NotNil(t, devices[0].LastUsedAt, "the sign-in on it counts as a use")

	rec = traegerRevokeDevice(t, portal, remembered.AccessToken, devices[0].ID)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	_, devices = traegerDevices(t, portal, remembered.AccessToken, nil)
	assert.Empty(t, devices)
	code, again := traegerLoginWithDevice(t, portal, account.Email, password, 0, cookie)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "mfa_required", again.Status, "a revoked device asks for the code again")
}

func TestTraegerDeviceOpensOnlyItsPortalAndItsTraeger(t *testing.T) {
	t.Parallel()
	db, portal, identity, _ := newTraegerPortal(t)
	ctx := t.Context()
	password := testpkg.NewTestPassword()
	first := traegerOrganization(t, db, "Träger Erstgerät", true)
	second := traegerOrganization(t, db, "Träger Zweitgerät", true)
	school := testpkg.CreateTestSchoolInOrganization(t, db, first, "OGS Erstgerät")
	account := enrolledTraegerMember(t, db, identity, password, first, second)
	cookie, _, err := identity.IssueTrustedDevice(identityaccess.WithTraegerDevice(ctx, first), account.ID, 0, "Firefox", nil)
	require.NoError(t, err)
	require.NotEmpty(t, cookie)

	verified := func(ctx context.Context, tenantID int64) bool {
		ok, err := identity.VerifyTrustedDevice(ctx, account.ID, tenantID, cookie)
		require.NoError(t, err)
		return ok
	}
	assert.True(t, verified(identityaccess.WithTraegerDevice(ctx, first), 0), "the device proves the factor at its Träger")
	assert.False(t, verified(identityaccess.WithTraegerDevice(ctx, second), 0), "and at no other Träger")
	assert.False(t, verified(ctx, 0), "a Träger device does not open the caterer portal")
	assert.False(t, verified(ctx, school), "nor a school of the Träger")

	code, other := traegerLoginWithDevice(t, portal, account.Email, password, second, cookie)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "mfa_required", other.Status, "the device of Träger A asks for the code at Träger B")
	code, own := traegerLoginWithDevice(t, portal, account.Email, password, first, cookie)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "authenticated", own.Status)

	code, secondSession := traegerPost(t, portal, "/traeger/auth/mfa/verify", "", map[string]any{
		"challenge_token": other.ChallengeToken, "code": lastTraegerCode(t, portal, account.Email), "organization_id": second,
	})
	require.Equal(t, http.StatusOK, code)
	code, devices := traegerDevices(t, portal, secondSession.AccessToken, nil)
	require.Equal(t, http.StatusOK, code)
	assert.Empty(t, devices, "Träger B lists none of Träger A's devices")
	_, firstDevices := traegerDevices(t, portal, own.AccessToken, nil)
	require.Len(t, firstDevices, 1)
	rec := traegerRevokeDevice(t, portal, secondSession.AccessToken, firstDevices[0].ID)
	assert.Equal(t, http.StatusForbidden, rec.Code, "a device of another Träger answers like a foreign one: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"general.permission"`)
	assert.True(t, verified(identityaccess.WithTraegerDevice(ctx, first), 0), "and stays remembered")

	tenantToken := testpkg.MintTestJWT(t, sessiontoken.AppClaims{ID: int(account.ID), Sub: account.Email, TenantID: school})
	code, _ = traegerDevices(t, portal, tenantToken, nil)
	assert.Equal(t, http.StatusUnauthorized, code, "a school token lists no Träger devices")

	// The same row without its Träger is a device of the caterer portal: it
	// opens that portal and not the Träger-Portal. Without its Träger the row
	// no longer goes with the test's Träger, so the test deletes it.
	t.Cleanup(func() {
		_, err := db.NewRaw(`DELETE FROM auth.mfa_trusted_devices WHERE id = ?`, firstDevices[0].ID).Exec(context.Background())
		assert.NoError(t, err)
	})
	_, err = db.NewRaw(`UPDATE auth.mfa_trusted_devices SET organization_id = NULL WHERE id = ?`, firstDevices[0].ID).Exec(ctx)
	require.NoError(t, err)
	assert.True(t, verified(ctx, 0), "the caterer portal's check accepts its own device")
	code, caterer := traegerLoginWithDevice(t, portal, account.Email, password, first, cookie)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "mfa_required", caterer.Status, "a caterer device does not open the Träger-Portal")
}

// The list of the Träger's remembered devices loads set-based: the statement
// count stays flat in the number of devices (#2940).
func TestTraegerTrustedDevicesQueryBudget(t *testing.T) {
	t.Parallel()
	db, portal, identity, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Gerätebudget", true)
	other := traegerOrganization(t, db, "Träger Fremdbudget", true)
	account := enrolledTraegerMember(t, db, identity, password, organization, other)
	remember := func(organizationID int64, n int) {
		for i := 0; i < n; i++ {
			_, _, err := identity.IssueTrustedDevice(identityaccess.WithTraegerDevice(t.Context(), organizationID), account.ID, 0, fmt.Sprintf("Browser %d", i), nil)
			require.NoError(t, err)
		}
	}
	remember(other, 2)
	remember(organization, 3)
	cookie, _, err := identity.IssueTrustedDevice(identityaccess.WithTraegerDevice(t.Context(), organization), account.ID, 0, "Anmeldung", nil)
	require.NoError(t, err)
	code, session := traegerLoginWithDevice(t, portal, account.Email, password, organization, cookie)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "authenticated", session.Status)

	measure := func(want int) []string {
		counter := testpkg.CaptureQueriesForContext(t, db)
		code, devices := traegerDevices(t, portal, session.AccessToken, counter)
		require.Equal(t, http.StatusOK, code)
		require.Len(t, devices, want, "only the devices at the session's Träger")
		counter.Stop()
		querybudgets.AssertQueryBudget(t, "api.traeger.auth.mfa.trusted_devices.positive_growth", counter.Measurement(counter.Queries()))
		return counter.Queries()
	}
	small := measure(4)
	remember(organization, 5)
	assert.Len(t, measure(9), len(small), "flat in the number of devices")
}
