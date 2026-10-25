// Router-level tests for the Träger-Portal (ADR 0084, #4649): the Träger
// sign-in with its own session, the school list through the Träger
// projection, the isolation between Träger and between portals, the right
// check, and the refresh and session validation of a Träger session.
package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// newTraegerPortal mounts /traeger as the serving root does, behind the
// tenant runtime and the session verifier of the production chain, and hands
// back the Identity & Access owner and runtime it was composed over.
func newTraegerPortal(t *testing.T) (*bun.DB, http.Handler, identityaccess.Owner, tenant.UnitOfWork) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	unit := testpkg.TenantRuntime(t, db)
	auth, err := NewAuthTestModule(testpkg.NewDatabase(t, db), unit)
	require.NoError(t, err)
	return db, traegerPortalRouter(t, db, auth.Auth, unit), auth.Auth, unit
}

// traegerPortalRouter builds the /traeger router over identity.
func traegerPortalRouter(t *testing.T, db *bun.DB, identity TraegerPortalIdentity, unit tenant.UnitOfWork) http.Handler {
	t.Helper()
	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	settings, err := owners.BuildTraegerSettings(testpkg.NewDatabase(t, db), unit, slog.Default(), organizations, nil)
	require.NoError(t, err)
	portal, err := NewTraegerPortalHTTP(identity, organizations, settings)
	require.NoError(t, err)
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithUnitOfWork(r.Context(), unit)))
		})
	})
	router.Mount("/traeger", portal.RouterWithAuthRateLimiter(nil, ProtectedTraegerRoutes))
	return testpkg.SessionVerifier(router)
}

func traegerOrganization(t *testing.T, db *bun.DB, name string, active bool) int64 {
	t.Helper()
	slug := fmt.Sprintf("traeger-%d", testpkg.UniqueTestTenantID(t))
	id := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug, Active: true}).ID
	if !active {
		// The fixture's column default turns a zero Active into true.
		_, err := db.ExecContext(context.Background(), `UPDATE platform.organizations SET active = false WHERE id = ?`, id)
		require.NoError(t, err)
	}
	return id
}

func traegerAccount(t *testing.T, db *bun.DB, password string) *testpkg.AccountFixture {
	t.Helper()
	return testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("traeger-%d@test.local", testpkg.UniqueTestTenantID(t)), password)
}

// traegerLoginAnswer reads both answer shapes: status is the login state of
// a success and the HTTP status number of a refusal.
type traegerLoginAnswer struct {
	Status        any    `json:"status"`
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token"`
	Organizations []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"organizations"`
	Code string `json:"code"`
}

func traegerLogin(t *testing.T, router http.Handler, email, password string, organizationID int64) (int, traegerLoginAnswer) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"email": email, "password": password, "organization_id": organizationID})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/traeger/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var answer traegerLoginAnswer
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &answer), rec.Body.String())
	return rec.Code, answer
}

type traegerSchool struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func traegerSchools(t *testing.T, router http.Handler, token string) (*httptest.ResponseRecorder, []traegerSchool) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/traeger/schools/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		return rec, nil
	}
	return rec, decodeSchoolPortalData[[]traegerSchool](t, rec.Body.Bytes())
}

func traegerTokenClaims(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(payload, &claims))
	return claims
}

func TestTraegerPortalLoginShowsOnlyTheSchoolsOfTheTraeger(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	own := traegerOrganization(t, db, "Träger Bissendorf", true)
	foreign := traegerOrganization(t, db, "Träger Hardt", true)
	firstSchool := testpkg.CreateTestSchoolInOrganization(t, db, own, "OGS Am Bach")
	secondSchool := testpkg.CreateTestSchoolInOrganization(t, db, own, "OGS Waldweg")
	foreignSchool := testpkg.CreateTestSchoolInOrganization(t, db, foreign, "OGS Fremd")
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, own, account.ID, identityaccess.TraegerRoleReadOnly)

	code, answer := traegerLogin(t, router, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "authenticated", answer.Status)
	require.NotEmpty(t, answer.RefreshToken)
	claims := traegerTokenClaims(t, answer.AccessToken)
	assert.Equal(t, "org", claims["scope"], "the Träger session has its own scope")
	assert.Equal(t, float64(own), claims["org_id"])
	assert.NotContains(t, claims, "tenant_id", "a Träger token never names a school")
	assert.ElementsMatch(t, []any{identityaccess.TraegerRightOverviewRead, identityaccess.TraegerRightReportsRead}, claims["permissions"])

	rec, schools := traegerSchools(t, router, answer.AccessToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var ids []int64
	for _, school := range schools {
		ids = append(ids, school.ID)
	}
	assert.Equal(t, []int64{firstSchool, secondSchool}, ids, "both schools of the Träger, ordered by name")
	assert.NotContains(t, ids, foreignSchool, "a Träger sees no school of another Träger")
}

func TestTraegerPortalLoginRefusesWithoutActiveMembership(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	active := traegerOrganization(t, db, "Träger aktiv", true)
	inactive := traegerOrganization(t, db, "Träger stillgelegt", false)

	stranger := traegerAccount(t, db, password)
	code, answer := traegerLogin(t, router, stranger.Email, password, 0)
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, "traeger.no_membership", answer.Code, "a correct password without Träger-Büro access")

	code, answer = traegerLogin(t, router, stranger.Email, "wrong-"+password, 0)
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Equal(t, "identity.invalid_credentials", answer.Code)

	member := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, inactive, member.ID, identityaccess.TraegerRoleAdministration)
	code, answer = traegerLogin(t, router, member.Email, password, 0)
	assert.Equal(t, http.StatusForbidden, code, "a deactivated Träger lets nobody in")
	assert.Equal(t, "traeger.no_membership", answer.Code)

	former := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, active, former.ID, identityaccess.TraegerRoleAdministration)
	_, err := db.ExecContext(context.Background(), `UPDATE platform.organization_accounts SET status = 'inactive', deactivated_at = NOW() WHERE account_id = ?`, former.ID)
	require.NoError(t, err)
	code, answer = traegerLogin(t, router, former.Email, password, 0)
	assert.Equal(t, http.StatusForbidden, code, "an ended membership lets nobody in")
	assert.Equal(t, "traeger.no_membership", answer.Code)
}

func TestTraegerPortalLoginAsksAMemberOfSeveralTraegerToChoose(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	first := traegerOrganization(t, db, "Träger A", true)
	second := traegerOrganization(t, db, "Träger B", true)
	unrelated := traegerOrganization(t, db, "Träger C", true)
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, first, account.ID, identityaccess.TraegerRoleReadOnly)
	testpkg.CreateTestTraegerMember(t, db, second, account.ID, identityaccess.TraegerRoleReadOnly)

	code, answer := traegerLogin(t, router, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "organization_selection_required", answer.Status)
	assert.Empty(t, answer.AccessToken, "no session before the choice")
	require.Len(t, answer.Organizations, 2)
	assert.Equal(t, first, answer.Organizations[0].ID)
	assert.Equal(t, second, answer.Organizations[1].ID)

	code, answer = traegerLogin(t, router, account.Email, password, second)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(second), traegerTokenClaims(t, answer.AccessToken)["org_id"], "one session belongs to exactly one Träger")

	code, answer = traegerLogin(t, router, account.Email, password, unrelated)
	assert.Equal(t, http.StatusForbidden, code, "a choice outside the memberships is refused")
	assert.Equal(t, "traeger.no_membership", answer.Code)
}

func TestTraegerSchoolListRequiresTheOverviewRight(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger ohne Rolle", true)
	testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Ohne Recht")
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID)

	code, answer := traegerLogin(t, router, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	assert.Empty(t, traegerTokenClaims(t, answer.AccessToken)["permissions"], "a member without a role holds no right")
	rec, _ := traegerSchools(t, router, answer.AccessToken)
	assert.Equal(t, http.StatusForbidden, rec.Code, "a role without the right gets 403")

	minted := testpkg.MintTestJWT(t, sessiontoken.AppClaims{ID: int(account.ID), Sub: account.Email, Scope: "org", OrgID: organization, Roles: []string{identityaccess.TraegerRoleBilling}})
	rec, _ = traegerSchools(t, router, minted)
	assert.Equal(t, http.StatusForbidden, rec.Code, "the route checks the right, never the role name")
}

func TestTraegerSchoolListRejectsTokensOfOtherPortals(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	organization := traegerOrganization(t, db, "Träger Portaltrennung", true)
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Trennung")
	everyRight := []string{"admin:*", identityaccess.TraegerRightOverviewRead}
	for name, claims := range map[string]sessiontoken.AppClaims{
		"OGS portal":      {ID: 7, Sub: "ogs@test.local", TenantID: school, OrgID: organization, Permissions: everyRight, IsAdmin: true},
		"school portal":   {ID: 7, Sub: "lehrkraft@test.local", Scope: "school", TenantID: school, OrgID: organization, Permissions: everyRight},
		"parents":         {ID: 7, Sub: "eltern@test.local", Scope: "parent", Permissions: everyRight},
		"operator":        {ID: 7, Sub: "operator@test.local", Scope: "platform", Permissions: everyRight},
		"org with school": {ID: 7, Sub: "gemischt@test.local", Scope: "org", OrgID: organization, TenantID: school, Permissions: everyRight},
	} {
		t.Run(name, func(t *testing.T) {
			rec, _ := traegerSchools(t, router, testpkg.MintTestJWT(t, claims))
			assert.Equal(t, http.StatusUnauthorized, rec.Code, "only a Träger session without a school passes the Träger route group")
		})
	}
}

func TestTraegerSessionRefreshesAndEndsWithTheMembership(t *testing.T) {
	t.Parallel()
	db, router, identity, unit := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Refresh", true)
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleAdministration)
	code, answer := traegerLogin(t, router, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	ctx := tenant.WithUnitOfWork(context.Background(), unit)

	claims, err := identity.ValidateSessionTokens(ctx, answer.AccessToken, answer.RefreshToken, "org")
	require.NoError(t, err, "the Träger portal validates its own session")
	assert.Equal(t, organization, claims.OrgID)
	_, err = identity.ValidateSessionTokens(ctx, answer.AccessToken, answer.RefreshToken, "tenant")
	assert.ErrorIs(t, err, identityaccess.ErrInvalidToken, "the OGS portal refuses a Träger session")

	access, refresh, err := identity.RefreshTokenWithAudit(ctx, answer.RefreshToken, "", "", "")
	require.NoError(t, err)
	refreshed := traegerTokenClaims(t, access)
	assert.Equal(t, "org", refreshed["scope"], "refresh keeps the Träger portal")
	assert.Equal(t, float64(organization), refreshed["org_id"])
	assert.NotContains(t, refreshed, "tenant_id")

	_, err = db.ExecContext(context.Background(), `UPDATE platform.organization_accounts SET status = 'inactive', deactivated_at = NOW() WHERE account_id = ?`, account.ID)
	require.NoError(t, err)
	_, _, err = identity.RefreshTokenWithAudit(ctx, refresh, "", "", "")
	assert.ErrorIs(t, err, identityaccess.ErrNoTraegerMembership, "refresh re-checks the membership")
}

func TestTraegerSchoolListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Budget", true)
	for _, name := range []string{"OGS Eins", "OGS Zwei", "OGS Drei"} {
		testpkg.CreateTestSchoolInOrganization(t, db, organization, name)
	}
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleReadOnly)
	code, answer := traegerLogin(t, router, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	list := func() []string {
		counter := testpkg.CaptureQueriesForContext(t, db)
		req := httptest.NewRequest(http.MethodGet, "/traeger/schools/", nil)
		req.Header.Set("Authorization", "Bearer "+answer.AccessToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	three := list()
	testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Vier")
	testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Fünf")
	assert.Len(t, list(), len(three), "the school list reads the same statements for any number of schools")
	testpkg.AssertQueryBudget(t, "api.traeger.schools", three)
}
