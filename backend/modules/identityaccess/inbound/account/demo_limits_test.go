package account_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The limits of the public request (#3466): per address, per IP, and the
// number of demo schools that may exist at once.

// postFrom sends the request as the client with this IP address.
func (e demoEnv) postFrom(t *testing.T, ip, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	req := testutil.NewJSONRequest(t, http.MethodPost, path, body)
	req.RemoteAddr = ip + ":40000"
	return testutil.ExecuteRequest(e.router, req)
}

// requireRateLimited asserts a 429 whose Retry-After lies within the hour.
func requireRateLimited(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusTooManyRequests, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "demo_access_rate_limited")
	seconds, err := strconv.Atoi(rr.Header().Get("Retry-After"))
	require.NoError(t, err, "Retry-After is a number of seconds: %q", rr.Header().Get("Retry-After"))
	assert.Positive(t, seconds)
	assert.LessOrEqual(t, seconds, 3600)
}

func TestDemoAccessRequestAllowsThreeRequestsPerAddressAndHour(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	body := env.requestBody(t)
	for request := range 3 {
		rr := env.post(t, "/demo/access-requests", body)
		require.Equal(t, http.StatusAccepted, rr.Code, "request %d: %s", request+1, rr.Body.String())
	}

	requireRateLimited(t, env.post(t, "/demo/access-requests", body))

	other := env.requestBodyFor(t, fmt.Sprintf("kollegin-%d@ogs-beispiel.de", testpkg.Tenant(t)))
	assert.Equal(t, http.StatusAccepted, env.post(t, "/demo/access-requests", other).Code,
		"the limit of one address does not stop another")
}

func TestDemoAccessAddressLimitCountsOnlyValidRequests(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	invalid := env.requestBody(t)
	invalid["last_name"] = " "
	for range 5 {
		assert.Equal(t, http.StatusUnprocessableEntity, env.post(t, "/demo/access-requests", invalid).Code)
	}

	body := env.requestBody(t)
	for request := range 3 {
		rr := env.post(t, "/demo/access-requests", body)
		require.Equal(t, http.StatusAccepted, rr.Code, "request %d after invalid ones: %s", request+1, rr.Body.String())
	}
	requireRateLimited(t, env.post(t, "/demo/access-requests", body))
}

// Visitors of a fair share one WLAN address: sixty of them get in within an
// hour, the next request from that address waits.
func TestDemoAccessRequestAllowsSixtyRequestsPerIPAndHour(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	const fairWLAN = "198.51.100.23"
	for visitor := range 60 {
		body := env.requestBodyFor(t, fmt.Sprintf("besuch-%d-%d@ogs-beispiel.de", testpkg.Tenant(t), visitor))
		rr := env.postFrom(t, fairWLAN, "/demo/access-requests", body)
		require.Equal(t, http.StatusAccepted, rr.Code, "visitor %d: %s", visitor+1, rr.Body.String())
	}

	requireRateLimited(t, env.postFrom(t, fairWLAN, "/demo/access-requests", env.requestBody(t)))
	assert.Equal(t, http.StatusAccepted, env.postFrom(t, "198.51.100.24", "/demo/access-requests", env.requestBody(t)).Code,
		"the limit of one IP address does not stop another")
}

// Typing errors of fair visitors must not use up the shared WLAN address.
func TestDemoAccessIPLimitCountsOnlyValidRequests(t *testing.T) {
	t.Parallel()
	env := newDemoEnv(t)
	const client = "198.51.100.42"
	invalid := map[string]any{"email": "kein-postfach", "school_name": "OGS", "first_name": "Kim", "last_name": "Beispiel"}
	for range 60 {
		require.Equal(t, http.StatusUnprocessableEntity, env.postFrom(t, client, "/demo/access-requests", invalid).Code)
	}

	rr := env.postFrom(t, client, "/demo/access-requests", env.requestBody(t))
	assert.Equal(t, http.StatusAccepted, rr.Code, rr.Body.String())
}

// activeDemoSchools counts the demo schools that hold a place: queued ones
// and ready ones whose school still exists.
func activeDemoSchools(t *testing.T, db *bun.DB) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM platform.demo_school_states AS state
		LEFT JOIN platform.schools AS school ON school.id = state.tenant_id
		WHERE state.status = 'preparing' OR (state.status = 'ready' AND school.deleted_at IS NULL)`).Scan(context.Background(), &count))
	return count
}

// The capacity counts every demo school of the database, so this test owns
// its database: the other tests of the package queue schools concurrently.
func TestDemoAccessRequestStopsAtTheDemoSchoolCapacity(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	capacity := activeDemoSchools(t, db) + 1
	env := mountDemoEnv(t, db, "", testutil.WithDemoMaxActiveSchools(capacity))

	_, slug := env.requestOwnSchool(t, env.address())
	second := env.requestBodyFor(t, fmt.Sprintf("zweite-%d@ogs-beispiel.de", testpkg.Tenant(t)))
	rr := env.post(t, "/demo/access-requests", second)
	require.Equal(t, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "demo_capacity_reached")
	var orders int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM auth.demo_accesses WHERE email = ?`, second["email"]).Scan(context.Background(), &orders))
	assert.Zero(t, orders, "a request over the capacity stores nothing")
	for range 3 {
		require.Equal(t, http.StatusServiceUnavailable, env.post(t, "/demo/access-requests", second).Code,
			"a request refused for capacity does not use up the address's requests")
	}

	env.endCooldown(t)
	assert.Equal(t, http.StatusAccepted, env.post(t, "/demo/access-requests", env.requestBody(t)).Code,
		"an address with a school gets its link again: it needs no new school")

	_, err := db.NewRaw(`UPDATE platform.demo_school_states SET status = 'failed' WHERE name = ?`, slug).Exec(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, env.post(t, "/demo/access-requests", second).Code,
		"a failed demo school frees its place")
}
