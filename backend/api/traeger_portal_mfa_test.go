// Router-level tests for the second factor of the Träger-Portal (ADR 0084,
// #4651): every Träger session needs it, for every role; it is set up on the
// first sign-in, redeemed on every later one, checked again at mint and at
// refresh, and the moto team can reset it so the person sets it up again.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	provisioningAPI "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestTraegerLoginSetsUpTheSecondFactorBeforeTheSession(t *testing.T) {
	t.Parallel()
	db, portal, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Einrichten", true)
	testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Einrichten")
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleReadOnly)

	code, login := traegerLogin(t, portal, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "mfa_enrollment_required", login.Status, "a Träger account without a factor sets it up first")
	assert.Empty(t, login.RefreshToken, "no session before the second factor")
	assert.Equal(t, organization, login.OrganizationID, "the Träger the login resolved travels back")
	assert.NotEmpty(t, login.MaskedEmail)
	rec, _ := traegerSchools(t, portal, login.AccessToken)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "the enrollment token opens no Träger route")

	started, start := traegerPost(t, portal, "/traeger/auth/mfa/enroll/start", login.AccessToken, map[string]any{})
	require.Equal(t, http.StatusOK, started)
	require.NotEmpty(t, start.ChallengeToken)
	sent := lastTraegerCode(t, portal, account.Email)
	wrong := "000000"
	if sent == wrong {
		wrong = "111111"
	}
	code, refused := traegerPost(t, portal, "/traeger/auth/mfa/enroll/confirm", login.AccessToken, map[string]any{
		"challenge_token": start.ChallengeToken, "code": wrong, "organization_id": organization,
	})
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Equal(t, "identity.mfa_code_invalid", refused.Code)

	code, session := traegerPost(t, portal, "/traeger/auth/mfa/enroll/confirm", login.AccessToken, map[string]any{
		"challenge_token": start.ChallengeToken, "code": sent, "organization_id": organization,
	})
	require.Equal(t, http.StatusOK, code)
	require.NotEmpty(t, session.RefreshToken)
	claims := traegerTokenClaims(t, session.AccessToken)
	assert.Equal(t, "org", claims["scope"])
	assert.Equal(t, float64(organization), claims["org_id"])
	rec, schools := traegerSchools(t, portal, session.AccessToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Len(t, schools, 1)
	assert.True(t, traegerSessionVerified(t, db, account.ID), "the session records its second factor")

	code, again := traegerLogin(t, portal, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "mfa_required", again.Status, "an enrolled account is asked for the mailed code")
	assert.Empty(t, again.AccessToken, "no session before the code")
	assert.Equal(t, organization, again.OrganizationID)
	code, verified := traegerPost(t, portal, "/traeger/auth/mfa/verify", "", map[string]any{
		"challenge_token": again.ChallengeToken, "code": lastTraegerCode(t, portal, account.Email), "organization_id": organization,
	})
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(organization), traegerTokenClaims(t, verified.AccessToken)["org_id"])
}

func TestTraegerSecondFactorRefusesForeignChallengesAndCodes(t *testing.T) {
	t.Parallel()
	db, portal, identity, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Fremdcode", true)
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleAdministration)
	require.NoError(t, identity.EnrollMFA(t.Context(), account.ID))
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Fremdcode")

	schoolChallenge, err := identity.StartMFAChallenge(t.Context(), account.ID, school, identityaccess.MFAChallengeScopeSchool, nil)
	require.NoError(t, err)
	code, answer := traegerPost(t, portal, "/traeger/auth/mfa/verify", "", map[string]any{
		"challenge_token": schoolChallenge, "code": lastTraegerCode(t, portal, account.Email),
	})
	assert.Equal(t, http.StatusUnauthorized, code, "a school challenge mints no Träger session")
	assert.Equal(t, "identity.mfa_code_invalid", answer.Code)

	schoolEnrollment := testpkg.MintTestJWT(t, sessiontoken.AppClaims{ID: int(account.ID), Sub: account.Email})
	code, _ = traegerPost(t, portal, "/traeger/auth/mfa/enroll/start", schoolEnrollment, map[string]any{})
	assert.Equal(t, http.StatusUnauthorized, code, "a session token is no enrollment token")

	code, login := traegerLogin(t, portal, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "mfa_required", login.Status)
	foreign := traegerOrganization(t, db, "Träger Fremd", true)
	code, answer = traegerPost(t, portal, "/traeger/auth/mfa/verify", "", map[string]any{
		"challenge_token": login.ChallengeToken, "code": lastTraegerCode(t, portal, account.Email), "organization_id": foreign,
	})
	assert.Equal(t, http.StatusForbidden, code, "the code opens only a Träger of the account")
	assert.Equal(t, "traeger.no_membership", answer.Code)
}

func TestTraegerSessionWithoutSecondFactorEndsAtRefresh(t *testing.T) {
	t.Parallel()
	db, portal, identity, unit := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Altsitzung", true)
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleReadOnly)
	code, session := traegerSignIn(t, portal, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	ctx := tenant.WithUnitOfWork(context.Background(), unit)

	_, refresh, err := identity.RefreshTokenWithAudit(ctx, session.RefreshToken, "", "", "")
	require.NoError(t, err, "a session with its second factor refreshes")

	// A Träger session minted before the duty carries no proof.
	_, err = db.ExecContext(context.Background(), `UPDATE auth.tokens SET mfa_verified = FALSE WHERE account_id = ?`, account.ID)
	require.NoError(t, err)
	_, _, err = identity.RefreshTokenWithAudit(ctx, refresh, "", "", "")
	assert.ErrorIs(t, err, identityaccess.ErrInvalidToken, "refresh refuses a Träger session without its second factor")
}

func TestTraegerLockoutIsLiftedByTheOperatorReset(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, _ := buildOperatorProvisioningWithIdentity(t, db)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, _ := provisionTestSchool(t, db, service, operatorID, "traeger-mfa-reset")
	unit := testpkg.TenantRuntime(t, db)
	portal, identity := traegerPortalOn(t, db, unit)
	password := testpkg.NewTestPassword()
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organizationID, account.ID, identityaccess.TraegerRoleAdministration)
	stranger := traegerAccount(t, db, password)

	code, session := traegerSignIn(t, portal, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)

	// Wrong codes until the second factor locks.
	code, login := traegerLogin(t, portal, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "mfa_required", login.Status)
	sent := lastTraegerCode(t, portal, account.Email)
	wrong := "000000"
	if sent == wrong {
		wrong = "111111"
	}
	locked := false
	for attempt := 0; attempt < 10 && !locked; attempt++ {
		code, _ = traegerPost(t, portal, "/traeger/auth/mfa/verify", "", map[string]any{"challenge_token": login.ChallengeToken, "code": wrong})
		locked = code == http.StatusTooManyRequests
	}
	require.True(t, locked, "repeated wrong codes lock the second factor")
	code, answer := traegerPost(t, portal, "/traeger/auth/mfa/verify", "", map[string]any{"challenge_token": login.ChallengeToken, "code": sent})
	assert.Equal(t, http.StatusTooManyRequests, code, "a locked factor refuses even the right code")
	assert.Equal(t, "identity.mfa_blocked", answer.Code)

	resource := provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service})
	operator := chi.NewRouter()
	mountOperatorProvisioningRoutes(operator, resource, nil)
	reset := func(accountID int64, reason string) *httptest.ResponseRecorder {
		var payload bytes.Buffer
		require.NoError(t, json.NewEncoder(&payload).Encode(map[string]string{"reason": reason}))
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/organizations/%d/traeger-members/%d/mfa", organizationID, accountID), &payload)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		rec := httptest.NewRecorder()
		operator.ServeHTTP(rec, req)
		return rec
	}
	rec := reset(account.ID, "x")
	assert.Equal(t, http.StatusBadRequest, rec.Code, "the audit needs a reason: %s", rec.Body.String())
	rec = reset(stranger.ID, "Postfach verloren")
	assert.Equal(t, http.StatusNotFound, rec.Code, "only a member of this Träger-Büro: %s", rec.Body.String())
	rec = reset(account.ID, "Gesperrt nach Fehlversuchen")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var audited int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log
		WHERE operator_id = ? AND action = 'mfa_reset' AND resource_type = 'traeger_member' AND resource_id = ?`, operatorID, account.ID).Scan(ctx, &audited))
	assert.Equal(t, 1, audited, "the reset is in the operator audit")

	_, _, err := identity.RefreshTokenWithAudit(tenant.WithUnitOfWork(context.Background(), unit), session.RefreshToken, "", "", "")
	assert.ErrorIs(t, err, identityaccess.ErrInvalidToken, "the reset ends the running Träger session at its refresh")

	code, login = traegerLogin(t, portal, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "mfa_enrollment_required", login.Status, "after the reset the person sets the factor up again")
	code, session = traegerSignIn(t, portal, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	assert.NotEmpty(t, session.RefreshToken, "the lock is lifted: the new factor opens the Träger-Portal")
}

// traegerSessionVerified reports whether the account has Träger sessions and
// every one of them records a second factor.
func traegerSessionVerified(t *testing.T, db *bun.DB, accountID int64) bool {
	t.Helper()
	var total, verified int
	require.NoError(t, db.NewRaw(`SELECT count(*), count(*) FILTER (WHERE mfa_verified) FROM auth.tokens
		WHERE account_id = ? AND organization_id IS NOT NULL`, accountID).Scan(context.Background(), &total, &verified))
	return total > 0 && total == verified
}
