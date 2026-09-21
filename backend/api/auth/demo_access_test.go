package auth_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The public demo routes (#3462), driven through the wired router against
// the real test database, as the password reset tests are.

const demoEntryOrigin = "https://messe-demo.demo.example"

type demoEnv struct {
	db     *bun.DB
	router chi.Router
	slug   string
	// mails are the messages that left the demo environment's mail lock.
	mails *testpkg.CapturingMailer
}

// newDemoEnv mounts the demo routes for this test's own school and the auth
// routes with the demo tenant-switch guard, as the demo root does. The
// transport carries the demo environment's mail lock (#3465).
func newDemoEnv(t *testing.T) demoEnv {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	mails := testpkg.NewCapturingMailer()
	svc, err := services.NewAuthTestModule(db, testpkg.TenantRuntime(t, db),
		services.WithAuthTestMailer(mails.InDemoEnvironment()))
	require.NoError(t, err)
	var slug string
	require.NoError(t, db.NewRaw(`SELECT slug FROM platform.schools WHERE id = ?`, testpkg.Tenant(t)).Scan(context.Background(), &slug))
	resource, err := authAPI.NewDemoResource(svc.DemoAccess, slug, demoEntryOrigin+"/")
	require.NoError(t, err)
	auth := authAPI.NewResource(svc.Auth, svc.Invitation, testSchoolDirectory{schools: svc.Schools, db: db, runtime: testpkg.TenantRuntime(t, db)}, svc.AccountAuthentication, db)
	router := testutil.NewTenantRouter(db)
	router.Mount("/demo", resource.Router())
	router.Mount("/auth", auth.Router())
	return demoEnv{db: db, router: router, slug: slug, mails: mails}
}

// address is this test's own prospect: an address waits between two links,
// so tests sharing one address would swallow each other's request.
func (e demoEnv) address() string { return "leitung@" + e.slug + ".example" }

func (e demoEnv) requestBody(t *testing.T) map[string]any {
	t.Helper()
	// A demo access carries no tenant, so it is shared state the leftover
	// gate would count; forget it like the reset tests forget their windows.
	t.Cleanup(func() {
		_, err := e.db.NewRaw(`DELETE FROM auth.demo_accesses WHERE email = ?`, e.address()).Exec(context.Background())
		require.NoError(t, err)
	})
	return map[string]any{
		"email": " " + strings.ToUpper(e.address()) + " ", "school_name": "OGS Beispiel", "person_name": "Kim Beispiel", "contact_opt_in": true, "src": "messe",
	}
}

// provisionDemoAdmin gives the school the administrator a demo session signs in.
func (e demoEnv) provisionDemoAdmin(t *testing.T) int64 {
	t.Helper()
	_, account := testpkg.CreateTestStaffWithAccount(t, e.db, "Demo", "Leitung")
	testpkg.EnsureAccountTenant(t, e.db, account.ID, testpkg.Tenant(t))
	_, err := e.db.NewRaw(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id)
		SELECT ?, id, ? FROM auth.roles WHERE name = 'admin'`, account.ID, testpkg.Tenant(t)).Exec(context.Background())
	require.NoError(t, err)
	return account.ID
}

func (e demoEnv) post(t *testing.T, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return testutil.ExecuteRequest(e.router, testutil.NewJSONRequest(t, http.MethodPost, path, body))
}

// requestToken asks for a demo access and reads the token from the mailed
// link: the answer never carries it (#3465).
func (e demoEnv) requestToken(t *testing.T) string {
	t.Helper()
	return e.requestTokenWith(t, e.requestBody(t))
}

func (e demoEnv) requestTokenWith(t *testing.T, body map[string]any) string {
	t.Helper()
	links := len(e.mailedLinks())
	rr := e.post(t, "/demo/access-requests", body)
	require.Equal(t, http.StatusAccepted, rr.Code, rr.Body.String())
	require.JSONEq(t, `{"link_sent":true}`, rr.Body.String(), "the answer is the same for every address")
	require.Eventually(t, func() bool { return len(e.mailedLinks()) == links+1 },
		2*time.Second, 10*time.Millisecond, "the link mail: %v", e.mails.Templates())
	entryURL := e.mailedLinks()[links]
	prefix := demoEntryOrigin + "/demo#token="
	require.True(t, strings.HasPrefix(entryURL, prefix), entryURL)
	return strings.TrimPrefix(entryURL, prefix)
}

// mailedLinks are the entry URLs of the link mails that left so far.
func (e demoEnv) mailedLinks() []string {
	var links []string
	for _, message := range e.mails.Messages() {
		if content, ok := message.Content.(map[string]any); ok && message.Template == "demo-access.html" {
			link, _ := content["EntryURL"].(string)
			links = append(links, link)
		}
	}
	return links
}

// endCooldown ages this test's accesses past the wait between two links.
func (e demoEnv) endCooldown(t *testing.T) {
	t.Helper()
	_, err := e.db.NewRaw(`UPDATE auth.demo_accesses SET created_at = created_at - INTERVAL '11 minutes' WHERE email = ?`, e.address()).Exec(context.Background())
	require.NoError(t, err)
}

func fingerprint(token string) string { return jwt.OpaqueCapabilityFingerprint(token) }

func (e demoEnv) status(token string) *httptest.ResponseRecorder {
	req := testutil.NewRequest(http.MethodGet, "/demo/access/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return testutil.ExecuteRequest(e.router, req)
}

func TestDemoAccessRequestStoresOnlyTheTokenFingerprint(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	token := env.requestToken(t)

	var row struct {
		Email     string `bun:"email"`
		Source    string `bun:"source"`
		OptIn     bool   `bun:"contact_opt_in"`
		ValidDays int    `bun:"valid_days"`
		Raw       int    `bun:"raw"`
	}
	require.NoError(t, env.db.NewRaw(`SELECT email, source, contact_opt_in,
			ROUND(EXTRACT(EPOCH FROM expires_at - created_at) / 86400)::int AS valid_days,
			(SELECT COUNT(*) FROM auth.demo_accesses WHERE token_hash = ?)::int AS raw
		FROM auth.demo_accesses WHERE token_hash = ?`, token, fingerprint(token)).Scan(context.Background(), &row))
	assert.Equal(t, env.address(), row.Email, "trimmed and lower-cased")
	assert.Equal(t, "messe", row.Source)
	assert.True(t, row.OptIn)
	assert.Equal(t, 14, row.ValidDays)
	assert.Zero(t, row.Raw, "the token itself must not be stored")
}

func TestDemoAccessRequestRejectsInvalidFields(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	for name, body := range map[string]map[string]any{
		"no address":     {"email": "", "school_name": "OGS", "person_name": "Kim"},
		"broken address": {"email": "kim(at)ogs", "school_name": "OGS", "person_name": "Kim"},
		"no school name": {"email": "kim@ogs.de", "school_name": " ", "person_name": "Kim"},
		"no person name": {"email": "kim@ogs.de", "school_name": "OGS", "person_name": ""},
	} {
		rr := env.post(t, "/demo/access-requests", body)
		assert.Equal(t, http.StatusUnprocessableEntity, rr.Code, name)
		assert.Contains(t, rr.Body.String(), "demo_access_invalid", name)
	}
}

func TestDemoAccessStatusFollowsTheDemoSchool(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	token := env.requestToken(t)

	rr := env.status(token)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.JSONEq(t, `{"status":"preparing"}`, rr.Body.String())
	assert.Equal(t, http.StatusConflict, env.post(t, "/demo/access/sessions", map[string]string{"token": token}).Code)

	env.provisionDemoAdmin(t)
	rr = env.status(token)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.JSONEq(t, `{"status":"ready"}`, rr.Body.String())
}

func TestDemoAccessRedeemsRepeatedlyIntoALockedTenantSession(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	adminID := env.provisionDemoAdmin(t)
	token := env.requestToken(t)

	var tokens authAPI.TokenResponse
	for range 2 {
		rr := env.post(t, "/demo/access/sessions", map[string]string{"token": token})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &tokens))
		require.NotEmpty(t, tokens.RefreshToken)
	}

	segments := strings.Split(tokens.AccessToken, ".")
	require.Len(t, segments, 3)
	payload, err := base64.RawURLEncoding.DecodeString(segments[1])
	require.NoError(t, err)
	var claims jwt.AppClaims
	require.NoError(t, json.Unmarshal(payload, &claims))
	assert.EqualValues(t, adminID, claims.ID)
	assert.Equal(t, testpkg.Tenant(t), claims.TenantID)
	assert.Empty(t, claims.Scope, "a demo session is a tenant session")

	var uses int
	require.NoError(t, env.db.NewRaw(`SELECT use_count FROM auth.demo_accesses WHERE account_id = ? AND last_used_at IS NOT NULL`, adminID).Scan(context.Background(), &uses))
	assert.Equal(t, 2, uses, "every use is noted")

	// Even a membership in a second school does not let the demo session out.
	otherID, other := testpkg.CreateTestTenant(t, env.db)
	testpkg.MapAccountToTenant(t, env.db, adminID, otherID)
	req := testutil.NewJSONRequest(t, http.MethodPost, "/auth/switch-tenant", map[string]string{"tenant_slug": other})
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rr := testutil.ExecuteRequest(env.router, req)
	assert.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "demo_session")
}

// Every visitor signs in as the same administrator, so the session cap of a
// single account must not end the demo of an earlier visitor.
func TestDemoAccessKeepsEarlierVisitorsSignedIn(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	env.provisionDemoAdmin(t)
	token := env.requestToken(t)

	var first authAPI.TokenResponse
	for visitor := range 7 {
		rr := env.post(t, "/demo/access/sessions", map[string]string{"token": token})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		if visitor == 0 {
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &first))
		}
	}

	req := testutil.NewJSONRequest(t, http.MethodPost, "/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+first.RefreshToken)
	rr := testutil.ExecuteRequest(env.router, req)
	assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestDemoAccessRejectsUnknownAndExpiredTokens(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	env.provisionDemoAdmin(t)

	assert.Equal(t, http.StatusNotFound, env.status("no-such-token").Code)
	rr := env.post(t, "/demo/access/sessions", map[string]string{"token": "no-such-token"})
	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Contains(t, rr.Body.String(), "demo_access_unknown")

	token := env.requestToken(t)
	_, err := env.db.NewRaw(`UPDATE auth.demo_accesses SET expires_at = NOW() - INTERVAL '1 minute' WHERE token_hash = ?`, fingerprint(token)).Exec(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusGone, env.status(token).Code)
	rr = env.post(t, "/demo/access/sessions", map[string]string{"token": token})
	assert.Equal(t, http.StatusGone, rr.Code)
	assert.Contains(t, rr.Body.String(), "demo_access_expired")
}

// The demo routes are a public, unauthenticated surface: outside the demo
// environment they must not exist, and nothing behind them is composed.
func TestDemoRoutesExistOnlyInTheDemoEnvironment(t *testing.T) {
	t.Parallel()
	routes := []struct{ method, path string }{
		{http.MethodPost, "/demo/access-requests"},
		{http.MethodGet, "/demo/access/status"},
		{http.MethodPost, "/demo/access/sessions"},
	}
	composed := 0
	compose := func() (authAPI.DemoAccesses, error) {
		composed++
		var accesses authAPI.DemoAccesses = (*identityaccess.DemoAccess)(nil)
		return accesses, nil
	}
	for _, appEnv := range []string{"", "development", "test", "staging", "production"} {
		router := chi.NewRouter()
		require.NoError(t, authAPI.MountDemoRoutes(router, appEnv, compose, "messe-demo", demoEntryOrigin))
		assert.Zero(t, composed, "nothing is composed under APP_ENV=%q", appEnv)
		for _, route := range routes {
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, httptest.NewRequest(route.method, route.path, strings.NewReader("{}")))
			assert.Equal(t, http.StatusNotFound, rr.Code, "%s %s under APP_ENV=%q", route.method, route.path, appEnv)
		}
	}

	router := chi.NewRouter()
	require.NoError(t, authAPI.MountDemoRoutes(router, "demo", compose, "messe-demo", demoEntryOrigin))
	for _, route := range routes {
		assert.True(t, router.Match(chi.NewRouteContext(), route.method, route.path), "%s %s", route.method, route.path)
	}
}
