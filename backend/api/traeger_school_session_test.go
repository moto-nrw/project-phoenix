// Router-level tests for the Absprung of the Träger-Büro into a school (ADR
// 0084 section 5, #4654): the hand-off through the Träger-Portal, the OGS
// session it opens, its re-checks at every refresh and validation, the
// refusals across Träger and rights, the audit entry, and two schools in two
// tabs.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// newTraegerSchoolSessionPortal mounts /traeger with its second-factor
// mailbox over the shared package database like newTraegerPortal and hands
// back the Identity & Access owner whose refresh, switch and validation the
// school's host would call.
func newTraegerSchoolSessionPortal(t *testing.T) (*bun.DB, *traegerPortalHandler, identityaccess.Owner, tenant.UnitOfWork) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	unit := testpkg.TenantRuntime(t, db)
	portal, identity := traegerPortalOn(t, db, unit)
	return db, portal, identity, unit
}

// traegerSchoolSessionAnswer reads both answer shapes of the Absprung.
type traegerSchoolSessionAnswer struct {
	Data struct {
		HandoffToken string `json:"handoff_token"`
	} `json:"data"`
	Code string `json:"code"`
}

func startTraegerSchoolSession(t *testing.T, router http.Handler, token string, schoolID int64) (int, traegerSchoolSessionAnswer) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"school_id": schoolID})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/traeger/school-sessions/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var answer traegerSchoolSessionAnswer
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &answer), rec.Body.String())
	return rec.Code, answer
}

// signInTraegerWithSecondFactor signs the account in to the Träger-Portal
// with the second factor its login requires (#4651).
func signInTraegerWithSecondFactor(t *testing.T, portal *traegerPortalHandler, email, password string) traegerLoginAnswer {
	t.Helper()
	code, answer := traegerSignIn(t, portal, email, password, 0)
	require.Equal(t, http.StatusOK, code, answer.Code)
	require.NotEmpty(t, answer.AccessToken, "the second factor opens a Träger session")
	return answer
}

func schoolSubdomain(t *testing.T, db *bun.DB, schoolID int64) string {
	t.Helper()
	var subdomain string
	require.NoError(t, db.NewRaw(`SELECT subdomain FROM platform.schools WHERE id = ?`, schoolID).Scan(context.Background(), &subdomain))
	return subdomain
}

func TestTraegerSchoolSessionOpensTheSchoolAsItsAdmin(t *testing.T) {
	t.Parallel()
	db, router, identity, unit := newTraegerSchoolSessionPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Absprung", true)
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Am Markt")
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleAdministration)
	traeger := signInTraegerWithSecondFactor(t, router, account.Email, password)

	code, answer := startTraegerSchoolSession(t, router, traeger.AccessToken, school)
	require.Equal(t, http.StatusCreated, code, answer.Code)
	handoff := traegerTokenClaims(t, answer.Data.HandoffToken)
	assert.Equal(t, float64(school), handoff["tenant_id"], "the hand-off names exactly one school")
	assert.Equal(t, float64(organization), handoff["org_id"], "and the Träger it came through")
	assert.LessOrEqual(t, handoff["exp"].(float64), float64(time.Now().Add(3*time.Minute).Unix()), "the hand-off token is short-lived")

	ctx := tenant.WithUnitOfWork(context.Background(), unit)
	access, refresh, err := identity.RefreshTokenWithAudit(ctx, answer.Data.HandoffToken, "proof-of-the-school-host", "192.0.2.10", "test")
	require.NoError(t, err, "the school's host redeems the hand-off")
	claims := traegerTokenClaims(t, access)
	assert.Equal(t, float64(school), claims["tenant_id"])
	assert.NotContains(t, claims, "scope", "an OGS session, not a Träger session")
	assert.Equal(t, true, claims["via_traeger"], "the token says the access comes through the Träger")
	assert.Equal(t, []any{"admin"}, claims["roles"], "the Träger role Verwaltung acts as the school's admin")
	assert.Equal(t, true, claims["is_admin"])
	assert.NotEmpty(t, claims["permissions"])
	assert.Greater(t, traegerTokenClaims(t, refresh)["exp"].(float64), float64(time.Now().Add(10*time.Minute).Unix()), "the redeemed session has the normal lifetime")

	validated, err := identity.ValidateSessionTokens(ctx, access, refresh, "tenant")
	require.NoError(t, err, "the OGS portal accepts the session")
	assert.True(t, validated.ViaTraeger)
	_, err = identity.ValidateSessionTokens(ctx, access, refresh, "org")
	assert.ErrorIs(t, err, identityaccess.ErrInvalidToken, "the Träger portal refuses the school session")

	_, _, err = identity.RefreshTokenWithAudit(ctx, answer.Data.HandoffToken, "proof-of-someone-else", "192.0.2.11", "test")
	assert.Error(t, err, "the hand-off token works once")

	var mappings int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?`, account.ID, school).Scan(context.Background(), &mappings))
	assert.Zero(t, mappings, "the Absprung creates no school mapping")

	var events []struct {
		IPAddress string `bun:"ip_address"`
		Traeger   int64  `bun:"organization_id"`
	}
	require.NoError(t, db.NewRaw(`SELECT ip_address, (metadata->>'organization_id')::bigint AS organization_id FROM audit.auth_events
		WHERE tenant_id = ? AND account_id = ? AND event_type = 'traeger_school_switch'`, school, account.ID).Scan(context.Background(), &events))
	require.Len(t, events, 1, "the Absprung is in the school's audit")
	assert.Equal(t, organization, events[0].Traeger)
}

func TestTraegerSchoolSessionRefusesForeignSchoolsAndMissingRights(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerSchoolSessionPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Eigen", true)
	foreign := traegerOrganization(t, db, "Träger Fremd", true)
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Eigen")
	foreignSchool := testpkg.CreateTestSchoolInOrganization(t, db, foreign, "OGS Fremd")
	admin := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, admin.ID, identityaccess.TraegerRoleAdministration)
	traeger := signInTraegerWithSecondFactor(t, router, admin.Email, password)

	code, answer := startTraegerSchoolSession(t, router, traeger.AccessToken, foreignSchool)
	assert.Equal(t, http.StatusNotFound, code, "the school of another Träger is refused")
	assert.Equal(t, "traeger.school_not_found", answer.Code)
	code, answer = startTraegerSchoolSession(t, router, traeger.AccessToken, foreignSchool+1_000_000)
	assert.Equal(t, http.StatusNotFound, code, "an unknown school reads the same")
	assert.Equal(t, "traeger.school_not_found", answer.Code)

	billing := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, billing.ID, identityaccess.TraegerRoleBilling)
	billingSession := signInTraegerWithSecondFactor(t, router, billing.Email, password)
	code, _ = startTraegerSchoolSession(t, router, billingSession.AccessToken, school)
	assert.Equal(t, http.StatusForbidden, code, "a role without the right gets 403")

	// The token claims a right the database no longer grants: the owner
	// re-checks the Träger roles.
	_, err := db.ExecContext(context.Background(), `DELETE FROM auth.organization_account_roles WHERE organization_id = ? AND account_id = ?`, organization, admin.ID)
	require.NoError(t, err)
	code, _ = startTraegerSchoolSession(t, router, traeger.AccessToken, school)
	assert.Equal(t, http.StatusForbidden, code, "a right withdrawn since sign-in gets 403")

	ogs := testpkg.MintTestJWT(t, sessiontoken.AppClaims{ID: int(admin.ID), Sub: admin.Email, TenantID: school, OrgID: organization, Roles: []string{"admin"},
		Permissions: []string{"admin:*", identityaccess.TraegerRightSchoolSwitch}, IsAdmin: true})
	req := httptest.NewRequest(http.MethodPost, "/traeger/school-sessions/", bytes.NewReader([]byte(`{"school_id":1}`)))
	req.Header.Set("Authorization", "Bearer "+ogs)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "an OGS token never reaches the Absprung")
}

func TestTraegerSchoolSessionNeedsTheSecondFactorOfTheTraegerSession(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerSchoolSessionPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Zweiter Faktor", true)
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Faktor")
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleAdministration)
	traeger := signInTraegerWithSecondFactor(t, router, account.Email, password)
	code, answer := startTraegerSchoolSession(t, router, traeger.AccessToken, school)
	require.Equal(t, http.StatusCreated, code, "the signed-in Träger session carries the second factor: %s", answer.Code)

	// The operator reset revokes the proof of the running Träger session
	// (#4651); its access token lives on until the next refresh.
	_, err := db.ExecContext(context.Background(), `UPDATE auth.tokens SET mfa_verified = FALSE WHERE account_id = ?`, account.ID)
	require.NoError(t, err)
	code, answer = startTraegerSchoolSession(t, router, traeger.AccessToken, school)
	assert.Equal(t, http.StatusForbidden, code, "the school's admin rights need the second factor")
	assert.Equal(t, "traeger.second_factor_required", answer.Code)
}

func TestTraegerSchoolSessionFollowsTheSchoolsOfTheTraeger(t *testing.T) {
	t.Parallel()
	db, router, identity, unit := newTraegerSchoolSessionPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Zuordnung", true)
	other := traegerOrganization(t, db, "Träger Übernahme", true)
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleAdministration)
	traeger := signInTraegerWithSecondFactor(t, router, account.Email, password)
	ctx := tenant.WithUnitOfWork(context.Background(), unit)

	added := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Neu dabei")
	code, answer := startTraegerSchoolSession(t, router, traeger.AccessToken, added)
	require.Equal(t, http.StatusCreated, code, "a school the Träger gained is reachable without a grant of its own")
	access, refresh, err := identity.RefreshTokenWithAudit(ctx, answer.Data.HandoffToken, "proof", "", "")
	require.NoError(t, err)

	_, err = db.ExecContext(context.Background(), `UPDATE platform.schools SET organization_id = ? WHERE id = ?`, other, added)
	require.NoError(t, err)
	_, err = identity.ValidateSessionTokens(ctx, access, refresh, "tenant")
	assert.ErrorIs(t, err, identityaccess.ErrInvalidToken, "a school that left the Träger refuses the session")
	_, _, err = identity.RefreshTokenWithAudit(ctx, refresh, "proof-2", "", "")
	assert.ErrorIs(t, err, identityaccess.ErrTraegerSchoolNotFound, "and ends it at the next refresh")
	code, answer = startTraegerSchoolSession(t, router, traeger.AccessToken, added)
	assert.Equal(t, http.StatusNotFound, code, "a school the Träger lost is no longer reachable")
	assert.Equal(t, "traeger.school_not_found", answer.Code)

	kept := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Bleibt")
	code, answer = startTraegerSchoolSession(t, router, traeger.AccessToken, kept)
	require.Equal(t, http.StatusCreated, code)
	_, refresh, err = identity.RefreshTokenWithAudit(ctx, answer.Data.HandoffToken, "proof-3", "", "")
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), `UPDATE platform.organization_accounts SET status = 'inactive', deactivated_at = NOW() WHERE account_id = ?`, account.ID)
	require.NoError(t, err)
	_, _, err = identity.RefreshTokenWithAudit(ctx, refresh, "proof-4", "", "")
	assert.ErrorIs(t, err, identityaccess.ErrNoTraegerMembership, "an ended membership ends the school session at its refresh")
}

func TestTraegerSchoolSessionsInTwoTabsStaySignedIn(t *testing.T) {
	t.Parallel()
	db, router, identity, unit := newTraegerSchoolSessionPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Zwei Tabs", true)
	foreign := traegerOrganization(t, db, "Träger Anderswo", true)
	first := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Erster Tab")
	second := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Zweiter Tab")
	foreignSchool := testpkg.CreateTestSchoolInOrganization(t, db, foreign, "OGS Fremder Tab")
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleAdministration)
	traeger := signInTraegerWithSecondFactor(t, router, account.Email, password)
	ctx := tenant.WithUnitOfWork(context.Background(), unit)

	open := func(schoolID int64, proof string) (string, string) {
		code, answer := startTraegerSchoolSession(t, router, traeger.AccessToken, schoolID)
		require.Equal(t, http.StatusCreated, code, answer.Code)
		access, refresh, err := identity.RefreshTokenWithAudit(ctx, answer.Data.HandoffToken, proof, "", "")
		require.NoError(t, err)
		return access, refresh
	}
	firstAccess, firstRefresh := open(first, "tab-1")
	secondAccess, secondRefresh := open(second, "tab-2")

	_, err := identity.ValidateSessionTokens(ctx, firstAccess, firstRefresh, "tenant")
	require.NoError(t, err, "the first school stays signed in after the second one opened")
	_, err = identity.ValidateSessionTokens(ctx, secondAccess, secondRefresh, "tenant")
	require.NoError(t, err)
	firstAccess, _, err = identity.RefreshTokenWithAudit(ctx, firstRefresh, "tab-1", "", "")
	require.NoError(t, err)
	assert.Equal(t, float64(first), traegerTokenClaims(t, firstAccess)["tenant_id"], "each session refreshes in its own school")
	_, _, err = identity.RefreshTokenWithAudit(ctx, secondRefresh, "tab-2", "", "")
	require.NoError(t, err)

	// A tab whose shared OGS cookie now holds the other school switches back
	// within the Träger, without a mapping and without a new sign-in.
	family, _ := traegerTokenClaims(t, firstAccess)["family_id"].(string)
	switched, _, err := identity.SwitchTenant(ctx, account.ID, schoolSubdomain(t, db, second), family)
	require.NoError(t, err)
	switchedClaims := traegerTokenClaims(t, switched)
	assert.Equal(t, float64(second), switchedClaims["tenant_id"])
	assert.Equal(t, true, switchedClaims["via_traeger"])

	_, _, err = identity.SwitchTenant(ctx, account.ID, schoolSubdomain(t, db, foreignSchool), switchedClaims["family_id"].(string))
	assert.ErrorIs(t, err, identityaccess.ErrTenantAccessDenied, "a switch leaves the Träger only through the account's own mapping")

	var switches int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM audit.auth_events
		WHERE tenant_id = ? AND account_id = ? AND event_type = 'traeger_school_switch' AND (metadata->>'organization_id')::bigint = ?`,
		second, account.ID, organization).Scan(context.Background(), &switches))
	assert.Equal(t, 2, switches, "the tab switch into the second school is in its audit like the Absprung")
}

// traegerAccessSession reads the family an access token rests on, as the
// per-request check sees it.
func traegerAccessSession(t *testing.T, accessToken string) identityaccess.AccessSession {
	t.Helper()
	claims := traegerTokenClaims(t, accessToken)
	session := identityaccess.AccessSession{AccountID: int64(claims["id"].(float64))}
	session.FamilyID, _ = claims["family_id"].(string)
	session.Scope, _ = claims["scope"].(string)
	return session
}

// TestTraegerLogoutEndsTheSchoolSessionsOfItsAbsprung pins the Absprung
// side of #5422: a school session the Träger session opened before its
// logout ends with it, also when the school switched on within the Träger.
// The other direction stays: ending the school session leaves the Träger
// session signed in.
func TestTraegerLogoutEndsTheSchoolSessionsOfItsAbsprung(t *testing.T) {
	t.Parallel()
	db, router, identity, unit := newTraegerSchoolSessionPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Abmeldung", true)
	first := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Abmeldung Eins")
	second := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Abmeldung Zwei")
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleAdministration)
	traeger := signInTraegerWithSecondFactor(t, router, account.Email, password)
	ctx := tenant.WithUnitOfWork(context.Background(), unit)
	open := func(schoolID int64, proof string) (string, string) {
		code, answer := startTraegerSchoolSession(t, router, traeger.AccessToken, schoolID)
		require.Equal(t, http.StatusCreated, code, answer.Code)
		access, refresh, err := identity.RefreshTokenWithAudit(ctx, answer.Data.HandoffToken, proof, "", "")
		require.NoError(t, err)
		return access, refresh
	}

	schoolAccess, schoolRefresh := open(first, "tab-1")
	require.NoError(t, identity.LogoutWithAudit(ctx, schoolRefresh, "", ""))
	require.ErrorIs(t, identity.VerifyAccessSession(ctx, traegerAccessSession(t, schoolAccess)), identityaccess.ErrInvalidToken)
	require.NoError(t, identity.VerifyAccessSession(ctx, traegerAccessSession(t, traeger.AccessToken)), "ending the school session leaves the Träger session signed in")

	schoolAccess, _ = open(first, "tab-2")
	switched, _, err := identity.SwitchTenant(ctx, account.ID, schoolSubdomain(t, db, second), traegerAccessSession(t, schoolAccess).FamilyID)
	require.NoError(t, err)
	require.NoError(t, identity.VerifyAccessSession(ctx, traegerAccessSession(t, switched)))

	require.NoError(t, identity.LogoutWithAudit(ctx, traeger.RefreshToken, "", ""))

	require.ErrorIs(t, identity.VerifyAccessSession(ctx, traegerAccessSession(t, switched)), identityaccess.ErrInvalidToken,
		"the school session the Träger session opened ends with its logout")
	var live int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM auth.tokens WHERE account_id = ? AND expiry > NOW()`, account.ID).Scan(context.Background(), &live))
	assert.Zero(t, live, "no session of the Träger lineage survives")
}
