// Router-level tests for the code step in the running Träger session
// (#7068): a Träger session whose factor only a remembered device proved
// redeems a code without signing out, rotates into a code-proven family and
// then opens a school that accepts no remembered device.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

const (
	traegerStepUpPath       = "/traeger/auth/mfa/step-up"
	traegerStepUpVerifyPath = "/traeger/auth/mfa/step-up/verify"
)

func TestTraegerStepUpOpensASchoolWithoutRememberedDevices(t *testing.T) {
	t.Parallel()
	db, portal, identity, unit := newTraegerSchoolSessionPortal(t)
	ctx := tenant.WithUnitOfWork(context.Background(), unit)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Code-Schritt", true)
	allowing := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Code-Schritt erlaubt")
	refusing := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Code-Schritt aus")
	unreadable := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Code-Schritt unlesbar")
	setSchoolTrustedDevices(t, db, refusing, `false`)
	setSchoolTrustedDevices(t, db, unreadable, `"unreadable"`)
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleSchoolAccess)
	require.NoError(t, enrollMFA(t.Context(), t, db, identity, account.ID))

	// Sign in once with the code and remember the device, then sign in by
	// the device alone: the Träger session carries the device mark.
	_, login := traegerLogin(t, portal, account.Email, password, organization)
	require.Equal(t, "mfa_required", login.Status)
	rec := traegerRequest(t, portal, "/traeger/auth/mfa/verify", map[string]any{
		"challenge_token": login.ChallengeToken, "code": lastTraegerCode(t, portal, account.Email),
		"organization_id": organization, "remember_device": true,
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	_, byDevice := traegerLoginWithDevice(t, portal, account.Email, password, organization, traegerDeviceCookie(rec))
	require.Equal(t, "authenticated", byDevice.Status)

	// A school session the device-proven family opened before the step.
	code, answer := startTraegerSchoolSession(t, portal, byDevice.AccessToken, allowing)
	require.Equal(t, http.StatusCreated, code, answer.Code)
	earlierSchool, _, err := identity.RefreshTokenWithAudit(ctx, answer.Data.HandoffToken, "tab-1", "", "")
	require.NoError(t, err)

	for _, school := range []int64{refusing, unreadable} {
		code, answer = startTraegerSchoolSession(t, portal, byDevice.AccessToken, school)
		require.Equal(t, http.StatusForbidden, code)
		require.Equal(t, "traeger.school_requires_code", answer.Code, "the device-proven session needs the code, also where the setting is unreadable")
	}

	code, start := traegerPost(t, portal, traegerStepUpPath, byDevice.AccessToken, map[string]any{})
	require.Equal(t, http.StatusOK, code, start.Code)
	require.NotEmpty(t, start.ChallengeToken)
	assert.NotEmpty(t, start.MaskedEmail)
	sent := lastTraegerCode(t, portal, account.Email)

	// A wrong code keeps the session as it is.
	wrong := "000000"
	if sent == wrong {
		wrong = "111111"
	}
	code, refused := traegerPost(t, portal, traegerStepUpVerifyPath, byDevice.AccessToken, map[string]any{"challenge_token": start.ChallengeToken, "code": wrong})
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Equal(t, "identity.mfa_code_invalid", refused.Code)
	assert.Empty(t, refused.AccessToken)
	code, answer = startTraegerSchoolSession(t, portal, byDevice.AccessToken, refusing)
	assert.Equal(t, http.StatusForbidden, code, "a wrong code leaves the Absprung refused")
	assert.Equal(t, "traeger.school_requires_code", answer.Code)

	rec = traegerRequestWithBearer(t, portal, traegerStepUpVerifyPath, byDevice.AccessToken, map[string]any{"challenge_token": start.ChallengeToken, "code": sent})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var stepped traegerLoginAnswer
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &stepped))
	require.NotEmpty(t, stepped.AccessToken)
	require.NotEmpty(t, stepped.RefreshToken)

	oldFamily := traegerAccessSession(t, byDevice.AccessToken).FamilyID
	newFamily := traegerAccessSession(t, stepped.AccessToken).FamilyID
	require.NotEqual(t, oldFamily, newFamily, "the step rotates the session into a new family")
	// The old family is retired like the family a switch replaces (#2952):
	// it ends within the recovery grace, so a refresh racing the step
	// still lands.
	var retiredUntil time.Time
	require.NoError(t, db.NewRaw(`SELECT MAX(LEAST(expiry, family_expiry_cap)) FROM auth.tokens WHERE family_id = ?`, oldFamily).
		Scan(context.Background(), &retiredUntil))
	assert.WithinDuration(t, time.Now().Add(5*time.Minute), retiredUntil, time.Minute, "the old family ends with the recovery grace of five minutes")
	require.NoError(t, identity.VerifyAccessSession(ctx, traegerAccessSession(t, stepped.AccessToken)))
	var byDeviceMark bool
	require.NoError(t, db.NewRaw(`SELECT mfa_by_device FROM auth.tokens WHERE family_id = ? ORDER BY id DESC LIMIT 1`, newFamily).
		Scan(context.Background(), &byDeviceMark))
	assert.False(t, byDeviceMark, "the code proved the new family's factor")

	for _, school := range []int64{refusing, unreadable} {
		code, answer = startTraegerSchoolSession(t, portal, stepped.AccessToken, school)
		assert.Equal(t, http.StatusCreated, code, answer.Code)
		assert.NotEmpty(t, answer.Data.HandoffToken, "the code-proven session opens the school without signing out")
	}

	// The codes of a step share the sign-in's cap of three per window: the
	// sign-in sent one, the step one; the third goes out, the fourth not.
	code, start = traegerPost(t, portal, traegerStepUpPath, stepped.AccessToken, map[string]any{})
	require.Equal(t, http.StatusOK, code, start.Code)
	code, start = traegerPost(t, portal, traegerStepUpPath, stepped.AccessToken, map[string]any{})
	assert.Equal(t, http.StatusTooManyRequests, code)
	assert.Equal(t, "identity.mfa_blocked", start.Code)

	// The new family keeps the lineage: its logout ends the school session
	// the old family opened.
	require.NoError(t, identity.LogoutWithAudit(ctx, stepped.RefreshToken, "", ""))
	require.ErrorIs(t, identity.VerifyAccessSession(ctx, traegerAccessSession(t, earlierSchool)), identityaccess.ErrInvalidToken,
		"the Träger logout after the step still ends the earlier Absprung")
}

func TestTraegerStepUpNeedsTheSchoolSwitchRightAndTheOwnCode(t *testing.T) {
	t.Parallel()
	db, portal, identity, _ := newTraegerSchoolSessionPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Code-Schritt Recht", true)
	member := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, member.ID, identityaccess.TraegerRoleSchoolAccess)
	reader := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, reader.ID, identityaccess.TraegerRoleReadOnly)
	require.NoError(t, enrollMFA(t.Context(), t, db, identity, member.ID))
	require.NoError(t, enrollMFA(t.Context(), t, db, identity, reader.ID))
	memberSession := signInTraegerWithSecondFactor(t, portal, member.Email, password)
	readerSession := signInTraegerWithSecondFactor(t, portal, reader.Email, password)

	code, _ := traegerPost(t, portal, traegerStepUpPath, readerSession.AccessToken, map[string]any{})
	assert.Equal(t, http.StatusForbidden, code, "without „In OGS wechseln“ there is nothing to step up for")
	code, _ = traegerPost(t, portal, traegerStepUpPath, "", map[string]any{})
	assert.Equal(t, http.StatusUnauthorized, code, "the step needs a Träger session")

	// A challenge of another account is refused before its code is
	// compared, and the code stays usable for its owner's own sign-in.
	_, foreign := traegerLogin(t, portal, reader.Email, password, organization)
	require.Equal(t, "mfa_required", foreign.Status)
	foreignCode := lastTraegerCode(t, portal, reader.Email)
	code, refused := traegerPost(t, portal, traegerStepUpVerifyPath, memberSession.AccessToken, map[string]any{"challenge_token": foreign.ChallengeToken, "code": foreignCode})
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Equal(t, "identity.mfa_code_invalid", refused.Code)
	assert.Empty(t, refused.AccessToken)
	rec := traegerRequest(t, portal, "/traeger/auth/mfa/verify", map[string]any{
		"challenge_token": foreign.ChallengeToken, "code": foreignCode, "organization_id": organization,
	})
	assert.Equal(t, http.StatusOK, rec.Code, "the refused step consumed nothing: %s", rec.Body.String())

	// „Neuen Code anfordern“ in the dialog renews the step's challenge
	// through the sign-in's resend; the renewed code completes the step.
	code, start := traegerPost(t, portal, traegerStepUpPath, memberSession.AccessToken, map[string]any{})
	require.Equal(t, http.StatusOK, code, start.Code)
	rec = traegerRequest(t, portal, "/traeger/auth/mfa/resend", map[string]any{"challenge_token": start.ChallengeToken})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var renewed struct {
		ChallengeToken string `json:"challenge_token"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &renewed))
	require.NotEmpty(t, renewed.ChallengeToken)
	code, stepped := traegerPost(t, portal, traegerStepUpVerifyPath, memberSession.AccessToken, map[string]any{
		"challenge_token": renewed.ChallengeToken, "code": lastTraegerCode(t, portal, member.Email),
	})
	assert.Equal(t, http.StatusOK, code, stepped.Code)
	assert.NotEmpty(t, stepped.AccessToken, "the resent code completes the step")
}
