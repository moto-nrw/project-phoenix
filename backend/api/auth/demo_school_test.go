package auth_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// One demo school per demo access (#3463), driven through the wired router
// against the real test database. The demo process is a separate program;
// these tests stand in for it by writing the progress it would write.

func newOwnSchoolDemoEnv(t *testing.T) demoEnv {
	t.Helper()
	return mountDemoEnv(t, testpkg.SetupTestDB(t), "")
}

// requestOwnSchool returns the mailed token and the slug of its demo school.
// The school does not exist yet, so the link leads to the waiting room, not
// to the school's subdomain.
func (e demoEnv) requestOwnSchool(t *testing.T, address string) (token, slug string) {
	t.Helper()
	token = e.requestTokenWith(t, e.requestBodyFor(t, address))
	require.NotEmpty(t, token)
	require.NoError(t, e.db.NewRaw(`SELECT school_slug FROM auth.demo_accesses WHERE token_hash = ?`, fingerprint(token)).Scan(context.Background(), &slug))
	return token, slug
}

// seedDemoSchool does what the demo process does once its seed and first tick
// succeeded: it names the school and the visitor's account and opens it.
func seedDemoSchool(t *testing.T, db *bun.DB, slug string, schoolID, visitorAccountID int64) {
	t.Helper()
	_, err := db.NewRaw(`UPDATE platform.demo_school_states SET tenant_id = ?, seed_state = '{}'::jsonb, status = 'ready', visitor_account_id = ? WHERE name = ?`,
		schoolID, visitorAccountID, slug).Exec(context.Background())
	require.NoError(t, err)
}

func sessionClaims(t *testing.T, e demoEnv, token string) jwt.AppClaims {
	t.Helper()
	rr := e.post(t, "/demo/access/sessions", map[string]string{"token": token})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var tokens authAPI.TokenResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &tokens))
	segments := strings.Split(tokens.AccessToken, ".")
	require.Len(t, segments, 3)
	payload, err := base64.RawURLEncoding.DecodeString(segments[1])
	require.NoError(t, err)
	var claims jwt.AppClaims
	require.NoError(t, json.Unmarshal(payload, &claims))
	return claims
}

func TestDemoAccessRequestQueuesASchoolOfItsOwn(t *testing.T) {
	t.Parallel()
	env := newOwnSchoolDemoEnv(t)
	token, slug := env.requestOwnSchool(t, demoAddress(t))

	assert.Regexp(t, `^ogs-beispiel-[a-z0-9]{6}$`, slug, "the slug is the OGS name plus a random suffix")
	var order struct {
		Status     string `bun:"status"`
		SchoolName string `bun:"school_name"`
		PersonName string `bun:"person_name"`
		Seeded     bool   `bun:"seeded"`
	}
	require.NoError(t, env.db.NewRaw(`SELECT status, school_name, person_name, seed_state IS NOT NULL AS seeded
		FROM platform.demo_school_states WHERE name = ?`, slug).Scan(context.Background(), &order))
	assert.Equal(t, "preparing", order.Status)
	assert.Equal(t, "OGS Beispiel", order.SchoolName)
	assert.Equal(t, "Kim Beispiel", order.PersonName)
	assert.False(t, order.Seeded)

	assert.JSONEq(t, `{"status":"preparing","school_name":"OGS Beispiel"}`, env.status(token).Body.String())
	rr := env.post(t, "/demo/access/sessions", map[string]string{"token": token})
	assert.Equal(t, http.StatusConflict, rr.Code, "a school that is not seeded cannot be entered")

	_, err := env.db.NewRaw(`UPDATE platform.demo_school_states SET status = 'failed' WHERE name = ?`, slug).Exec(context.Background())
	require.NoError(t, err)
	assert.JSONEq(t, `{"status":"failed","school_name":"OGS Beispiel"}`, env.status(token).Body.String())
}

// The prospect's address is contact data of the demo access only. It never
// reaches the queue the seeder reads, so it cannot become an account or a
// guardian address of a demo school, where attachExistingAccountByEmail would
// give an existing account of that address access to another tenant.
func TestDemoSchoolOrderCarriesNoAddress(t *testing.T) {
	t.Parallel()
	env := newOwnSchoolDemoEnv(t)
	_, slug := env.requestOwnSchool(t, demoAddress(t))

	var row string
	require.NoError(t, env.db.NewRaw(`SELECT to_jsonb(state)::text FROM platform.demo_school_states AS state WHERE name = ?`, slug).Scan(context.Background(), &row))
	assert.NotContains(t, strings.ToLower(row), demoAddress(t))
	assert.NotContains(t, row, "@")
}

// A known address gets a further link after the cooldown (#3465), but no
// second school: the new link leads into the school it already has.
func TestDemoAccessOfAnActiveAddressGetsNoSecondSchool(t *testing.T) {
	t.Parallel()
	env := newOwnSchoolDemoEnv(t)
	address := env.address()
	first, slug := env.requestOwnSchool(t, address)
	env.endCooldown(t)

	second, again := env.requestOwnSchool(t, strings.ToUpper(address))
	assert.NotEqual(t, first, second, "every request stores its own access")
	assert.Equal(t, slug, again, "an active address gets no second school")
	var accesses, orders int
	require.NoError(t, env.db.NewRaw(`SELECT COUNT(*) FROM auth.demo_accesses WHERE email = ?`, address).Scan(context.Background(), &accesses))
	require.NoError(t, env.db.NewRaw(`SELECT COUNT(*) FROM platform.demo_school_states
		WHERE name IN (SELECT school_slug FROM auth.demo_accesses WHERE email = ?)`, address).Scan(context.Background(), &orders))
	assert.Equal(t, 2, accesses)
	assert.Equal(t, 1, orders)

	// An access whose school failed for good is not active: the prospect may try again.
	_, err := env.db.NewRaw(`UPDATE platform.demo_school_states SET status = 'failed' WHERE name = ?`, slug).Exec(context.Background())
	require.NoError(t, err)
	env.endCooldown(t)
	_, third := env.requestOwnSchool(t, address)
	assert.NotEqual(t, slug, third)
}

// Two visitors, two demo schools: each session belongs to its own school and
// its own caregiver, cannot leave it, and sees nothing the other one changed.
func TestDemoVisitorsDoNotSeeEachOthersSchool(t *testing.T) {
	t.Parallel()
	env := newOwnSchoolDemoEnv(t)
	firstSchool := testpkg.Tenant(t)
	secondSchool, secondSlug := testpkg.CreateTestTenant(t, env.db)
	_, firstVisitor := testpkg.CreateTestStaffWithAccount(t, env.db, "Kim", "Beispiel")
	testpkg.EnsureAccountTenant(t, env.db, firstVisitor.ID, firstSchool)
	_, secondVisitor := testpkg.CreateTestStaffWithAccountForTenant(t, env.db, secondSchool, "Alex", "Muster")
	testpkg.EnsureAccountTenant(t, env.db, secondVisitor.ID, secondSchool)

	firstToken, firstOrder := env.requestOwnSchool(t, env.address())
	secondToken, secondOrder := env.requestOwnSchool(t, "zweite-"+env.address())
	require.NotEqual(t, firstOrder, secondOrder)
	seedDemoSchool(t, env.db, firstOrder, firstSchool, firstVisitor.ID)
	seedDemoSchool(t, env.db, secondOrder, secondSchool, secondVisitor.ID)

	assert.JSONEq(t, `{"status":"ready","school_name":"OGS Beispiel","school_url":"https://`+firstOrder+`.demo.example"}`, env.status(firstToken).Body.String())
	assert.JSONEq(t, `{"status":"ready","school_name":"OGS Beispiel","school_url":"https://`+secondOrder+`.demo.example"}`, env.status(secondToken).Body.String())

	first, second := sessionClaims(t, env, firstToken), sessionClaims(t, env, secondToken)
	assert.Equal(t, firstSchool, first.TenantID)
	assert.EqualValues(t, firstVisitor.ID, first.ID, "the visitor signs in as the own caregiver, not as a shared administrator")
	assert.Equal(t, secondSchool, second.TenantID)
	assert.EqualValues(t, secondVisitor.ID, second.ID)

	// Even with a membership in the other demo school the session stays put.
	testpkg.MapAccountToTenant(t, env.db, firstVisitor.ID, secondSchool)
	rr := env.post(t, "/demo/access/sessions", map[string]string{"token": firstToken})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var tokens authAPI.TokenResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &tokens))
	req := testutil.NewJSONRequest(t, http.MethodPost, "/auth/switch-tenant", map[string]string{"tenant_slug": secondSlug})
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	assert.Equal(t, http.StatusForbidden, testutil.ExecuteRequest(env.router, req).Code)

	// What the first visitor changes stays in the first school.
	room := testpkg.CreateTestRoomForTenant(t, env.db, firstSchool, "Raum des ersten Besuchers")
	visible := func(schoolID int64) int {
		var count int
		require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), env.db, schoolID, func(ctx context.Context) error {
			transaction, ok := tenant.TransactionFromContext(ctx)
			require.True(t, ok)
			return transaction.(bun.Tx).NewRaw(`SELECT COUNT(*) FROM facilities.rooms WHERE id = ?`, room.ID).Scan(ctx, &count)
		}))
		return count
	}
	assert.Equal(t, 1, visible(first.TenantID))
	assert.Zero(t, visible(second.TenantID), "the second visitor's school does not contain the first visitor's room")
}

// The simulation serves only the demo schools a visitor entered in the last
// minutes (#3464). Entering is redeeming the token, which notes the entry on
// the school; a returning visitor redeems it again and brings the school back.
func TestDemoSimulationServesOnlySchoolsInUse(t *testing.T) {
	t.Parallel()
	env := newOwnSchoolDemoEnv(t)
	usedSchool := testpkg.Tenant(t)
	idleSchool, _ := testpkg.CreateTestTenant(t, env.db)
	_, usedVisitor := testpkg.CreateTestStaffWithAccount(t, env.db, "Kim", "Beispiel")
	testpkg.EnsureAccountTenant(t, env.db, usedVisitor.ID, usedSchool)
	_, idleVisitor := testpkg.CreateTestStaffWithAccountForTenant(t, env.db, idleSchool, "Alex", "Muster")
	testpkg.EnsureAccountTenant(t, env.db, idleVisitor.ID, idleSchool)

	usedToken, used := env.requestOwnSchool(t, env.address())
	_, idle := env.requestOwnSchool(t, "zweite-"+env.address())
	seedDemoSchool(t, env.db, used, usedSchool, usedVisitor.ID)
	seedDemoSchool(t, env.db, idle, idleSchool, idleVisitor.ID)

	// The demo process ticks the schools whose entry is recent; the selection
	// itself is Organisation & Tenancy's (TestActiveDemoSchoolsAreTheReadyOnesEnteredSince).
	lastEntry := func(slug string) *time.Time {
		var at *time.Time
		require.NoError(t, env.db.NewRaw(`SELECT last_used_at FROM platform.demo_school_states WHERE name = ?`, slug).Scan(context.Background(), &at))
		return at
	}
	assert.Nil(t, lastEntry(used), "a ready school nobody entered gets no ticks")

	before := time.Now().Add(-time.Second)
	sessionClaims(t, env, usedToken)
	require.NotNil(t, lastEntry(used), "entering the school starts its simulation")
	assert.True(t, lastEntry(used).After(before))
	assert.Nil(t, lastEntry(idle), "entering one school does not wake another")

	_, err := env.db.NewRaw(`UPDATE platform.demo_school_states SET last_used_at = NOW() - INTERVAL '1 hour' WHERE name = ?`, used).Exec(context.Background())
	require.NoError(t, err)
	before = time.Now().Add(-time.Second)
	sessionClaims(t, env, usedToken)
	assert.True(t, lastEntry(used).After(before), "a returning visitor brings the simulation back")
}
