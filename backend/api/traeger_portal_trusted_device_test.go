// Router-level tests for the remembered device of the Träger-Portal (#6955):
// the code step remembers the browser only when the person chooses it, the
// device skips the code only at the Träger it was remembered at, and the
// person sees and removes it in the Träger-Portal.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

type traegerChallengeAnswer struct {
	Status               string `json:"status"`
	ChallengeToken       string `json:"challenge_token"`
	OrganizationID       int64  `json:"organization_id"`
	TrustedDeviceEnabled bool   `json:"trusted_device_enabled"`
	TrustedDeviceDays    int    `json:"trusted_device_days"`
	AccessToken          string `json:"access_token"`
	RefreshToken         string `json:"refresh_token"`
}

// traegerLoginWithDevice signs in to one Träger with the browser's
// remember-device cookie, when there is one.
func traegerLoginWithDevice(t *testing.T, portal http.Handler, email, password string, organizationID int64, device string) traegerChallengeAnswer {
	t.Helper()
	body, err := json.Marshal(map[string]any{"email": email, "password": password, "organization_id": organizationID})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/traeger/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if device != "" {
		req.AddCookie(&http.Cookie{Name: common.TrustedDeviceCookie, Value: device})
	}
	rec := httptest.NewRecorder()
	portal.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var answer traegerChallengeAnswer
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &answer))
	return answer
}

// traegerRedeem answers the code step and hands back the session and the
// remember-device cookie the answer set, if any.
func traegerRedeem(t *testing.T, portal *traegerPortalHandler, email string, login traegerChallengeAnswer, remember bool) (traegerChallengeAnswer, *http.Cookie) {
	t.Helper()
	require.Equal(t, "mfa_required", login.Status)
	rec := traegerRequest(t, portal, "/traeger/auth/mfa/verify", map[string]any{
		"challenge_token": login.ChallengeToken, "code": lastTraegerCode(t, portal, email),
		"organization_id": login.OrganizationID, "remember_device": remember,
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var session traegerChallengeAnswer
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &session))
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == common.TrustedDeviceCookie {
			return session, cookie
		}
	}
	return session, nil
}

type traegerDevice struct {
	ID        int64  `json:"id"`
	UserAgent string `json:"user_agent"`
}

func traegerDevices(t *testing.T, portal http.Handler, token string) []traegerDevice {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/traeger/auth/mfa/trusted-devices", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	portal.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var devices []traegerDevice
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &devices))
	return devices
}

func traegerRevokeDevice(t *testing.T, portal http.Handler, token string, deviceID int64) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/traeger/auth/mfa/trusted-devices/%d", deviceID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	portal.ServeHTTP(rec, req)
	return rec.Code
}

func trustedDeviceMails(portal *traegerPortalHandler, address string) int {
	count := 0
	for _, message := range portal.mail.Messages() {
		if message.To.Address == address && strings.Contains(message.Subject, "vertrautes Gerät") {
			count++
		}
	}
	return count
}

func TestTraegerRemembersADeviceOnlyWhenChosenAndOnlyAtItsTraeger(t *testing.T) {
	t.Parallel()
	db, portal, identity, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	traegerA := traegerOrganization(t, db, "Träger Gerät A", true)
	traegerB := traegerOrganization(t, db, "Träger Gerät B", true)
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, traegerA, account.ID, identityaccess.TraegerRoleReadOnly)
	testpkg.CreateTestTraegerMember(t, db, traegerB, account.ID, identityaccess.TraegerRoleReadOnly)
	require.NoError(t, enrollMFA(t.Context(), t, db, identity, account.ID))

	login := traegerLoginWithDevice(t, portal, account.Email, password, traegerA, "")
	require.Equal(t, "mfa_required", login.Status)
	assert.True(t, login.TrustedDeviceEnabled, "the code step offers to remember the device")
	assert.Equal(t, identityaccess.MFATrustedDeviceCookieDefaultDays, login.TrustedDeviceDays, "the Träger-Portal remembers for the registry default")

	_, cookie := traegerRedeem(t, portal, account.Email, login, false)
	assert.Nil(t, cookie, "without the choice no device is remembered")
	assert.Zero(t, trustedDeviceMails(portal, account.Email))

	sessionA, cookie := traegerRedeem(t, portal, account.Email, traegerLoginWithDevice(t, portal, account.Email, password, traegerA, ""), true)
	require.NotNil(t, cookie, "the chosen device is remembered")
	assert.True(t, cookie.HttpOnly)
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assert.Equal(t, 1, trustedDeviceMails(portal, account.Email), "remembering a device mails the holder")

	skipped := traegerLoginWithDevice(t, portal, account.Email, password, traegerA, cookie.Value)
	assert.Equal(t, "authenticated", skipped.Status, "the remembered device skips the code at its Träger")
	assert.NotEmpty(t, skipped.RefreshToken)
	assert.Empty(t, skipped.ChallengeToken)
	assert.True(t, traegerSessionVerified(t, db, account.ID), "a remembered device proves the second factor")

	other := traegerLoginWithDevice(t, portal, account.Email, password, traegerB, cookie.Value)
	assert.Equal(t, "mfa_required", other.Status, "a device of Träger A does not open Träger B")
	sessionB, _ := traegerRedeem(t, portal, account.Email, other, false)

	devicesA := traegerDevices(t, portal, sessionA.AccessToken)
	require.Len(t, devicesA, 1)
	assert.Empty(t, traegerDevices(t, portal, sessionB.AccessToken), "Träger B lists no device of Träger A")
	assert.Equal(t, http.StatusForbidden, traegerRevokeDevice(t, portal, sessionB.AccessToken, devicesA[0].ID),
		"Träger B cannot remove a device of Träger A")
	assert.Equal(t, http.StatusNoContent, traegerRevokeDevice(t, portal, sessionA.AccessToken, devicesA[0].ID))
	assert.Empty(t, traegerDevices(t, portal, sessionA.AccessToken))
	// A fourth code within the quarter hour would hit the code limit; the
	// sign-in consults the same check.
	trusted, err := identity.VerifyTrustedDevice(t.Context(), account.ID, identityaccess.TraegerPortalDevice(traegerA), cookie.Value)
	require.NoError(t, err)
	assert.False(t, trusted, "a removed device asks for the code again")
}

func TestTraegerDeviceIsNotTheCaterersAndNeedsTheTraegerNamed(t *testing.T) {
	t.Parallel()
	db, portal, identity, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	traeger := traegerOrganization(t, db, "Träger Gerät Caterer", true)
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, traeger, account.ID, identityaccess.TraegerRoleReadOnly)
	require.NoError(t, enrollMFA(t.Context(), t, db, identity, account.ID))

	catererDevice, _, err := identity.IssueTrustedDevice(t.Context(), account.ID, identityaccess.CatererPortalDevice(), "Caterer", nil)
	require.NoError(t, err)
	require.NotEmpty(t, catererDevice)
	login := traegerLoginWithDevice(t, portal, account.Email, password, traeger, catererDevice)
	assert.Equal(t, "mfa_required", login.Status, "a device of the caterer portal does not open the Träger-Portal")

	rec := traegerRequest(t, portal, "/traeger/auth/mfa/verify", map[string]any{
		"challenge_token": login.ChallengeToken, "code": lastTraegerCode(t, portal, account.Email), "remember_device": true,
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, cookie := range rec.Result().Cookies() {
		assert.NotEqual(t, common.TrustedDeviceCookie, cookie.Name, "without the Träger named no device is remembered")
	}
}
