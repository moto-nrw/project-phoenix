package auth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestIdentityPersistenceRuntimeEvidence measures the school account flows whose
// persistence #3226 rewrote natively, through the auth router: login, refresh
// rotation, the own-account reads, the administrative account, role and
// permission reads, and one role assignment round trip. The harness only uses
// the router and fixtures that exist before and after the cutover, so the same
// file ran against the merge base; the numbers are recorded in
// docs/runtime-checkpoints/identity-persistence-3226.md. Fixtures are outside
// the timer.
func TestIdentityPersistenceRuntimeEvidence(t *testing.T) {
	t.Parallel()

	tc, router := setupProtectedRouter(t)
	db := tc.db
	ctx := testpkg.Ctx(t)
	testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))

	email := fmt.Sprintf("identity-runtime-%d@example.com", time.Now().UnixNano())
	const password = "RuntimePass123!"
	account := testpkg.CreateTestAccountWithPassword(t, db, email, password)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	adminRole := testpkg.GetOrCreateTestRole(t, db, "admin")
	_, err := db.NewRaw("INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
		account.ID, adminRole.ID, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	assignedRole := testpkg.GetOrCreateTestRole(t, db, "user")
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = db.NewDelete().TableExpr("auth.account_roles").Where("account_id = ?", account.ID).Exec(cleanupCtx)
		_, _ = db.NewDelete().TableExpr("auth.tokens").Where("account_id = ?", account.ID).Exec(cleanupCtx)
	})

	var postgres string
	require.NoError(t, db.NewRaw("SHOW server_version").Scan(ctx, &postgres))
	// Scoped to this test's requests: the package pool is shared with the
	// parallel tests, whose statements must not land in these counts.
	counter := testpkg.CaptureQueriesForContext(t, db)
	deadlocks := func() int64 {
		var n int64
		require.NoError(t, db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(ctx, &n))
		return n
	}

	var accessToken, refreshToken string
	tokens := func(t *testing.T, body []byte) {
		t.Helper()
		var pair struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		require.NoError(t, json.Unmarshal(body, &pair))
		require.NotEmpty(t, pair.AccessToken)
		require.NotEmpty(t, pair.RefreshToken)
		accessToken, refreshToken = pair.AccessToken, pair.RefreshToken
	}
	keep := func(*testing.T, []byte) {}
	accountPath := fmt.Sprintf("/auth/accounts/%d", account.ID)

	type operation struct {
		name   string
		method string
		path   string
		body   any
		// bearer picks the token the request carries; nil sends none.
		bearer func() string
		status int
		after  func(*testing.T, []byte)
	}
	access := func() string { return accessToken }
	operations := []operation{
		{"login", http.MethodPost, "/auth/login", map[string]string{"email": email, "password": password}, nil, http.StatusOK, tokens},
		{"refresh", http.MethodPost, "/auth/refresh", nil, func() string { return refreshToken }, http.StatusOK, tokens},
		{"own_account", http.MethodGet, "/auth/account", nil, access, http.StatusOK, keep},
		{"own_account_tenants", http.MethodGet, "/auth/account/tenants", nil, access, http.StatusOK, keep},
		{"accounts_list", http.MethodGet, "/auth/accounts?email=identity-runtime", nil, access, http.StatusOK, keep},
		{"account_roles", http.MethodGet, accountPath + "/roles", nil, access, http.StatusOK, keep},
		{"account_permissions", http.MethodGet, accountPath + "/permissions", nil, access, http.StatusOK, keep},
		{"account_direct_permissions", http.MethodGet, accountPath + "/permissions/direct", nil, access, http.StatusOK, keep},
		{"role_assign", http.MethodPost, fmt.Sprintf("%s/roles/%d", accountPath, assignedRole.ID), nil, access, 0, keep},
		{"role_unassign", http.MethodDelete, fmt.Sprintf("%s/roles/%d", accountPath, assignedRole.ID), nil, access, 0, keep},
	}

	samples := make(map[string][]testpkg.RuntimeCheckpointSample, len(operations))
	durations := make(map[string][]float64, len(operations))
	statuses := make(map[string]int, len(operations))
	beforeDeadlocks := deadlocks()
	stopLocks := testpkg.SampleCheckpointLocks(func(sampleCtx context.Context) (int, error) {
		var n int
		err := db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(sampleCtx, &n)
		return n, err
	})
	// Interleaved rounds: every round logs in, rotates the refresh token and
	// then reads and writes with the freshly issued access token.
	for i := range 35 {
		for _, op := range operations {
			req := testutil.NewJSONRequest(t, op.method, op.path, op.body)
			if op.bearer != nil {
				req.Header.Set("Authorization", "Bearer "+op.bearer())
			}
			req = req.WithContext(counter.Context(req.Context()))
			counter.Reset()
			before := db.Stats()
			start := time.Now()
			rr := testutil.ExecuteRequest(router, req)
			elapsed := float64(time.Since(start)) / float64(time.Millisecond)
			after := db.Stats()
			want := op.status
			if want == 0 {
				// The first round fixes the write status; later rounds must repeat it.
				if _, seen := statuses[op.name]; !seen {
					require.Less(t, rr.Code, http.StatusBadRequest, "%s: %s", op.name, rr.Body.String())
					statuses[op.name] = rr.Code
				}
				want = statuses[op.name]
			}
			require.Equal(t, want, rr.Code, "%s: %s", op.name, rr.Body.String())
			statuses[op.name] = rr.Code
			op.after(t, rr.Body.Bytes())
			if i < 5 {
				continue
			}
			affected, statements := counter.Rows()
			writes := counter.WriteRows()
			samples[op.name] = append(samples[op.name], testpkg.RuntimeCheckpointSample{
				DurationMS: elapsed, Queries: counter.Total(), Status: rr.Code,
				RowsAffected: affected, StatementsWithRows: statements, WriteRowsAffected: &writes,
				PoolWaitCount: after.WaitCount - before.WaitCount,
				PoolWaitMS:    float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond),
			})
			durations[op.name] = append(durations[op.name], elapsed)
		}
	}
	locks := stopLocks()
	require.Empty(t, locks.Error)

	results := make([]map[string]any, 0, len(operations))
	for _, op := range operations {
		sorted := slices.Clone(durations[op.name])
		slices.Sort(sorted)
		queries := make([]int, 0, len(samples[op.name]))
		for _, sample := range samples[op.name] {
			if !slices.Contains(queries, sample.Queries) {
				queries = append(queries, sample.Queries)
			}
		}
		slices.Sort(queries)
		results = append(results, map[string]any{
			"operation": op.name, "method": op.method, "status": statuses[op.name],
			"samples": samples[op.name], "p50_ms": sorted[14], "p95_ms": sorted[28], "max_ms": sorted[29],
			"distinct_query_counts": queries,
		})
	}
	report, err := json.Marshal(map[string]any{
		"operations": results, "go": runtime.Version(), "os_arch": runtime.GOOS + "/" + runtime.GOARCH,
		"postgres": postgres, "warmup": 5, "measured": 30, "concurrency": 1, "unexpected_errors": 0,
		"lock_samples": locks, "deadlocks": deadlocks() - beforeDeadlocks,
	})
	require.NoError(t, err)
	t.Logf("identity-persistence-runtime: %s", report)
}
