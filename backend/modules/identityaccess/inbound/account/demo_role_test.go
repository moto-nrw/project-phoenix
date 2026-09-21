package account_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Demo roles in the banner (#3467): the visitor's one account changes its
// role and a new session is issued, driven through the wired router against
// the real test database.

type demoSession struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Demo         struct {
		AccessID  string `json:"access_id"`
		Role      string `json:"role"`
		Source    string `json:"src"`
		FixedRole bool   `json:"fixed_role"`
	} `json:"demo"`
}

func (s demoSession) claims(t *testing.T) testutil.Claims {
	t.Helper()
	segments := strings.Split(s.AccessToken, ".")
	require.Len(t, segments, 3)
	payload, err := base64.RawURLEncoding.DecodeString(segments[1])
	require.NoError(t, err)
	var claims testutil.Claims
	require.NoError(t, json.Unmarshal(payload, &claims))
	return claims
}

func (e demoEnv) enterAs(t *testing.T, token, role string) demoSession {
	t.Helper()
	rr := e.post(t, "/demo/access/sessions", map[string]string{"token": token, "role": role})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var session demoSession
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &session))
	return session
}

// grantRole gives the account a system role in the test's school.
func (e demoEnv) grantRole(t *testing.T, accountID int64, role string) {
	t.Helper()
	_, err := e.db.NewRaw(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id)
		SELECT ?, id, ? FROM auth.roles WHERE name = ? AND tenant_id IS NULL`, accountID, testpkg.Tenant(t), role).Exec(context.Background())
	require.NoError(t, err)
}

func (e demoEnv) schoolRoles(t *testing.T, accountID int64) []string {
	t.Helper()
	var roles []string
	require.NoError(t, e.db.NewRaw(`SELECT role.name FROM auth.account_roles AS account_role
		JOIN auth.roles AS role ON role.id = account_role.role_id
		WHERE account_role.account_id = ? AND account_role.tenant_id = ? ORDER BY role.name`, accountID, testpkg.Tenant(t)).Scan(context.Background(), &roles))
	return roles
}

func TestDemoRoleSwitchChangesTheVisitorsRoleAndIssuesANewSession(t *testing.T) {
	t.Parallel()
	env := newOwnSchoolDemoEnv(t)
	_, visitor := testpkg.CreateTestStaffWithAccount(t, env.db, "Kim", "Beispiel")
	testpkg.EnsureAccountTenant(t, env.db, visitor.ID, testpkg.Tenant(t))
	env.grantRole(t, visitor.ID, "user")
	token, slug := env.requestOwnSchool(t, env.address())
	seedDemoSchool(t, env.db, slug, testpkg.Tenant(t), visitor.ID)
	var accessID string
	require.NoError(t, env.db.NewRaw(`SELECT id::text FROM auth.demo_accesses WHERE token_hash = ?`, fingerprint(token)).Scan(context.Background(), &accessID))

	lead := env.enterAs(t, token, "lead")
	assert.Equal(t, accessID, lead.Demo.AccessID, "the banner's analytics identify the demo access, not the person")
	assert.Equal(t, "lead", lead.Demo.Role)
	assert.Equal(t, "messe", lead.Demo.Source)
	assert.False(t, lead.Demo.FixedRole, "the visitor's own school lets the banner switch roles")
	claims := lead.claims(t)
	assert.EqualValues(t, visitor.ID, claims.ID, "every demo role is the same account")
	assert.Contains(t, claims.Roles, "admin", "until reduced roles exist, the OGS lead uses the administrator role")
	assert.Equal(t, []string{"admin"}, env.schoolRoles(t, visitor.ID))

	// A role of the school's own adds permissions a caregiver must not keep.
	_, err := env.db.NewRaw(`WITH role AS (
			INSERT INTO auth.roles (name, tenant_id, is_system) VALUES ('Hortleitung', ?, false) RETURNING id)
		INSERT INTO auth.account_roles (account_id, role_id, tenant_id) SELECT ?, id, ? FROM role`,
		testpkg.Tenant(t), visitor.ID, testpkg.Tenant(t)).Exec(context.Background())
	require.NoError(t, err)

	caregiver := env.enterAs(t, token, "caregiver")
	assert.Equal(t, "caregiver", caregiver.Demo.Role)
	claims = caregiver.claims(t)
	assert.EqualValues(t, visitor.ID, claims.ID)
	assert.Equal(t, []string{"user"}, claims.Roles, "the caregiver uses the standard staff role")
	assert.False(t, claims.IsAdmin)
	assert.Equal(t, []string{"user"}, env.schoolRoles(t, visitor.ID), "the name stays once in the staff list: no second account")

	all := env.enterAs(t, token, "all")
	assert.Equal(t, "all", all.Demo.Role)
	assert.True(t, all.claims(t).IsAdmin)
}

func TestDemoRoleSwitchRejectsAnUnknownRole(t *testing.T) {
	t.Parallel()
	env := newOwnSchoolDemoEnv(t)
	_, visitor := testpkg.CreateTestStaffWithAccount(t, env.db, "Kim", "Beispiel")
	testpkg.EnsureAccountTenant(t, env.db, visitor.ID, testpkg.Tenant(t))
	env.grantRole(t, visitor.ID, "user")
	token, slug := env.requestOwnSchool(t, env.address())
	seedDemoSchool(t, env.db, slug, testpkg.Tenant(t), visitor.ID)

	rr := env.post(t, "/demo/access/sessions", map[string]string{"token": token, "role": "operator"})
	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "demo_access_invalid")
	assert.Equal(t, []string{"user"}, env.schoolRoles(t, visitor.ID), "a rejected switch changes no role")
}

// The standing demo school is shared by every visitor, who all sign in as its
// administrator. A switch there would change the role for everybody, so the
// session keeps all functions.
func TestDemoRoleSwitchLeavesTheSharedAdministratorAlone(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	adminID := env.provisionDemoAdmin(t)
	token := env.requestToken(t)

	session := env.enterAs(t, token, "caregiver")
	assert.Equal(t, "all", session.Demo.Role, "the answer names the role the session really has")
	assert.True(t, session.Demo.FixedRole, "the banner offers no switch that would do nothing")
	assert.EqualValues(t, adminID, session.claims(t).ID)
	assert.True(t, session.claims(t).IsAdmin)
	assert.Equal(t, []string{"admin"}, env.schoolRoles(t, adminID))
}

// A role chosen on the website or behind a fair QR code rides in the mailed
// link, so the entry page can skip the role cards.
func TestDemoAccessRequestCarriesAPreselectedRoleIntoTheLink(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	body := env.requestBody(t)
	body["role"] = "caregiver"
	link := env.requestTokenWith(t, body)
	token, role, found := strings.Cut(link, "&role=")
	require.True(t, found, link)
	assert.Equal(t, "caregiver", role)
	assert.NotEmpty(t, token)
	assert.Equal(t, http.StatusOK, env.status(token).Code, "the token before the role is the whole token")

	env.endCooldown(t)
	body["role"] = "operator"
	rr := env.post(t, "/demo/access-requests", body)
	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code, rr.Body.String())
}

// The demo role parent (#3468): the same token, redeemed on the parents host,
// signs the visitor in as the school's parent of the visitor's name. The
// caregiver keeps its role, and the next switch back is an ordinary entry.
func TestDemoRoleParentIssuesAParentSessionForTheVisitorsParent(t *testing.T) {
	t.Parallel()
	env := newOwnSchoolDemoEnv(t)
	_, visitor := testpkg.CreateTestStaffWithAccount(t, env.db, "Kim", "Beispiel")
	testpkg.EnsureAccountTenant(t, env.db, visitor.ID, testpkg.Tenant(t))
	env.grantRole(t, visitor.ID, "user")
	parent := testpkg.CreateTestParentGuardianChain(t, env.db)
	token, slug := env.requestOwnSchool(t, env.address())
	seedDemoSchoolWithParent(t, env.db, slug, testpkg.Tenant(t), visitor.ID, parent.AccountID)

	session := env.enterAs(t, token, "parent")
	assert.Equal(t, "parent", session.Demo.Role, "PostHog reports the role parent")
	assert.False(t, session.Demo.FixedRole)
	claims := session.claims(t)
	assert.Equal(t, "parent", claims.Scope, "a parents portal session, not a tenant session")
	assert.EqualValues(t, parent.AccountID, claims.ID, "the visitor is the parent of the visitor's name")
	assert.Zero(t, claims.TenantID, "a parent session is bound to no school")
	assert.Equal(t, []string{"user"}, env.schoolRoles(t, visitor.ID), "the parent role leaves the caregiver's role alone")

	back := env.enterAs(t, token, "lead")
	assert.EqualValues(t, visitor.ID, back.claims(t).ID, "the way back into the OGS app is the same token")
	assert.Empty(t, back.claims(t).Scope)
}

// A demo school the demo process opened before the role parent existed
// names no parent; the role is refused instead of signing in somebody else.
func TestDemoRoleParentNeedsTheSchoolsVisitorParent(t *testing.T) {
	t.Parallel()
	env := newOwnSchoolDemoEnv(t)
	_, visitor := testpkg.CreateTestStaffWithAccount(t, env.db, "Kim", "Beispiel")
	testpkg.EnsureAccountTenant(t, env.db, visitor.ID, testpkg.Tenant(t))
	env.grantRole(t, visitor.ID, "user")
	token, slug := env.requestOwnSchool(t, env.address())
	seedDemoSchool(t, env.db, slug, testpkg.Tenant(t), visitor.ID)

	rr := env.post(t, "/demo/access/sessions", map[string]string{"token": token, "role": "parent"})
	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "demo_access_invalid")
}

// The standing school is shared and has no parent of its own; a parent
// choice there keeps the administrator, and the answer says so.
func TestDemoRoleParentInTheStandingSchoolKeepsTheSharedAdministrator(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	adminID := env.provisionDemoAdmin(t)
	token := env.requestToken(t)

	session := env.enterAs(t, token, "parent")
	assert.Equal(t, "all", session.Demo.Role)
	assert.True(t, session.Demo.FixedRole)
	assert.EqualValues(t, adminID, session.claims(t).ID)
	assert.Empty(t, session.claims(t).Scope)
}
