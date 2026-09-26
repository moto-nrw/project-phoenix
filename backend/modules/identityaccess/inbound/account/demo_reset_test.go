package account_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// „Demo neu anfangen" (#3470), driven through the wired router against the
// real test database: a reset orders a fresh demo school for the same demo
// access, hides the old one and ends its sessions. The mailed link keeps
// working and now leads into the new school.

func (e demoEnv) reset(t *testing.T, token string) *httptest.ResponseRecorder {
	t.Helper()
	return e.post(t, "/demo/access/reset", map[string]string{"token": token})
}

func TestDemoResetOrdersANewSchoolForTheSameLink(t *testing.T) {
	t.Parallel()
	env := newOwnSchoolDemoEnv(t)
	school, _ := testpkg.CreateTestTenant(t, env.db)
	_, visitor := testpkg.CreateTestStaffWithAccountForTenant(t, env.db, school, "Kim", "Beispiel")
	testpkg.EnsureAccountTenant(t, env.db, visitor.ID, school)
	token, slug := env.requestOwnSchool(t, env.address())
	t.Cleanup(func() {
		// The access moves on to its new school; the old order stays behind.
		_, err := env.db.NewRaw(`DELETE FROM platform.demo_school_states WHERE name = ?`, slug).Exec(context.Background())
		require.NoError(t, err)
	})

	rr := env.reset(t, token)
	assert.Equal(t, http.StatusConflict, rr.Code, "a school that is still being prepared cannot be restarted")
	assert.Contains(t, rr.Body.String(), "demo_school_preparing")

	seedDemoSchool(t, env.db, slug, school, visitor.ID)
	session := env.enterAs(t, token, "lead")

	rr = env.reset(t, token)
	require.Equal(t, http.StatusAccepted, rr.Code, rr.Body.String())
	var body struct {
		Status   string `json:"status"`
		EntryURL string `json:"entry_url"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "preparing", body.Status)
	assert.Equal(t, demoEntryPrefix+token, body.EntryURL, "the same link leads into the new school, through the waiting room")

	// The access enters a new school that waits for its seed.
	var access struct {
		SchoolSlug string `bun:"school_slug"`
		AccountID  *int64 `bun:"account_id"`
	}
	require.NoError(t, env.db.NewRaw(`SELECT school_slug, account_id FROM auth.demo_accesses WHERE token_hash = ?`, fingerprint(token)).Scan(context.Background(), &access))
	assert.NotEqual(t, slug, access.SchoolSlug)
	assert.Regexp(t, `^ogs-beispiel-[a-z0-9]{6}$`, access.SchoolSlug, "the new school carries the OGS name again")
	assert.Nil(t, access.AccountID, "the account of the old school no longer belongs to the access")
	var order struct {
		Status     string `bun:"status"`
		SchoolName string `bun:"school_name"`
		FirstName  string `bun:"first_name"`
		LastName   string `bun:"last_name"`
	}
	require.NoError(t, env.db.NewRaw(`SELECT status, school_name, first_name, last_name FROM platform.demo_school_states WHERE name = ?`, access.SchoolSlug).Scan(context.Background(), &order))
	assert.Equal(t, "preparing", order.Status)
	assert.Equal(t, "OGS Beispiel", order.SchoolName)
	assert.Equal(t, []string{"Kim", "Beispiel"}, []string{order.FirstName, order.LastName}, "the restart reuses the stored names")
	assert.JSONEq(t, `{"status":"preparing","school_name":"OGS Beispiel"}`, env.status(token).Body.String(), "the entry shows the setup screen again")
	assert.Equal(t, http.StatusConflict, env.post(t, "/demo/access/sessions", map[string]string{"token": token}).Code)

	// The old school is hidden and its sessions are over.
	var hidden bool
	require.NoError(t, env.db.NewRaw(`SELECT deleted_at IS NOT NULL FROM platform.schools WHERE id = ?`, school).Scan(context.Background(), &hidden))
	assert.True(t, hidden, "the old demo school is soft-deleted")
	req := testutil.NewJSONRequest(t, http.MethodPost, "/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+session.RefreshToken)
	assert.Equal(t, http.StatusUnauthorized, testutil.ExecuteRequest(env.router, req).Code, "the session of the old school cannot be refreshed")
}

// Every access of the address moves along: an earlier link of the same
// address must not lead into the hidden school.
func TestDemoResetMovesEveryLinkOfTheAddress(t *testing.T) {
	t.Parallel()
	env := newOwnSchoolDemoEnv(t)
	school, _ := testpkg.CreateTestTenant(t, env.db)
	_, visitor := testpkg.CreateTestStaffWithAccountForTenant(t, env.db, school, "Kim", "Beispiel")
	testpkg.EnsureAccountTenant(t, env.db, visitor.ID, school)
	first, slug := env.requestOwnSchool(t, env.address())
	env.endCooldown(t)
	second, again := env.requestOwnSchool(t, env.address())
	require.Equal(t, slug, again)
	t.Cleanup(func() {
		_, err := env.db.NewRaw(`DELETE FROM platform.demo_school_states WHERE name = ?`, slug).Exec(context.Background())
		require.NoError(t, err)
	})
	seedDemoSchool(t, env.db, slug, school, visitor.ID)

	require.Equal(t, http.StatusAccepted, env.reset(t, second).Code)

	var slugs []string
	require.NoError(t, env.db.NewRaw(`SELECT DISTINCT school_slug FROM auth.demo_accesses WHERE token_hash IN (?, ?)`, fingerprint(first), fingerprint(second)).Scan(context.Background(), &slugs))
	require.Len(t, slugs, 1, "both links lead into the same new school")
	assert.NotEqual(t, slug, slugs[0])
}

// The standing school is shared by every visitor: nobody may restart it.
func TestDemoResetRefusesTheStandingSchool(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	env.provisionDemoAdmin(t)
	token := env.requestToken(t)

	rr := env.reset(t, token)
	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "demo_access_invalid")
	var slug string
	require.NoError(t, env.db.NewRaw(`SELECT school_slug FROM auth.demo_accesses WHERE token_hash = ?`, fingerprint(token)).Scan(context.Background(), &slug))
	assert.Equal(t, env.slug, slug)
}

func TestDemoResetRejectsUnknownAndExpiredTokens(t *testing.T) {
	t.Parallel()
	env := newOwnSchoolDemoEnv(t)

	rr := env.reset(t, "no-such-token")
	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Contains(t, rr.Body.String(), "demo_access_unknown")

	token, _ := env.requestOwnSchool(t, env.address())
	_, err := env.db.NewRaw(`UPDATE auth.demo_accesses SET expires_at = NOW() - INTERVAL '1 minute' WHERE token_hash = ?`, fingerprint(token)).Exec(context.Background())
	require.NoError(t, err)
	rr = env.reset(t, token)
	assert.Equal(t, http.StatusGone, rr.Code)
	assert.Contains(t, rr.Body.String(), "demo_access_expired")
}

// A restart is a seed job, so it counts against the address's window like a
// request does (#3466): three in an hour, the request included, then 429.
func TestDemoResetCountsAgainstTheAddressLimit(t *testing.T) {
	t.Parallel()
	env := newOwnSchoolDemoEnv(t)
	token, slug := env.requestOwnSchool(t, env.address())
	for round := range 2 {
		school, _ := testpkg.CreateTestTenant(t, env.db)
		_, visitor := testpkg.CreateTestStaffWithAccountForTenant(t, env.db, school, "Kim", "Beispiel")
		testpkg.EnsureAccountTenant(t, env.db, visitor.ID, school)
		seedDemoSchool(t, env.db, slug, school, visitor.ID)
		old := slug
		t.Cleanup(func() {
			_, err := env.db.NewRaw(`DELETE FROM platform.demo_school_states WHERE name = ?`, old).Exec(context.Background())
			require.NoError(t, err)
		})
		rr := env.reset(t, token)
		require.Equal(t, http.StatusAccepted, rr.Code, "restart %d: %s", round+1, rr.Body.String())
		require.NoError(t, env.db.NewRaw(`SELECT school_slug FROM auth.demo_accesses WHERE token_hash = ?`, fingerprint(token)).Scan(context.Background(), &slug))
	}

	rr := env.reset(t, token)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "demo_access_rate_limited")
	assert.NotEmpty(t, rr.Header().Get("Retry-After"))
}

// Every use extends the link (#3470): the demo expires 14 days after the
// last entry, not 14 days after the request.
func TestDemoAccessUseExtendsItsLifetime(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	env.provisionDemoAdmin(t)
	token := env.requestToken(t)
	_, err := env.db.NewRaw(`UPDATE auth.demo_accesses SET expires_at = NOW() + INTERVAL '1 day' WHERE token_hash = ?`, fingerprint(token)).Exec(context.Background())
	require.NoError(t, err)

	env.enterAs(t, token, "")

	var validDays int
	require.NoError(t, env.db.NewRaw(`SELECT ROUND(EXTRACT(EPOCH FROM expires_at - last_used_at) / 86400)::int FROM auth.demo_accesses WHERE token_hash = ?`, fingerprint(token)).Scan(context.Background(), &validDays))
	assert.Equal(t, 14, validDays)
}
