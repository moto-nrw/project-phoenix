//go:build testhooks

package catererauth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/catererauth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestIdentityCatererSecurityPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, err := api.NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	caterers := module.Auth.Caterers()
	require.NotNil(t, caterers)
	account := testpkg.CreateTestAccount(t, db, "caterer-security-budget")
	other := testpkg.CreateTestAccount(t, db, "caterer-security-decoy")
	var catererID int64
	require.NoError(t, db.NewRaw("INSERT INTO auth.caterers (name) VALUES ('Security budget kitchen') RETURNING id").Scan(context.Background(), &catererID))
	_, err = db.ExecContext(context.Background(), "INSERT INTO auth.caterer_accounts (account_id,caterer_id) VALUES (?,?)", account.ID, catererID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), "DELETE FROM auth.caterers WHERE id=?", catererID)
		require.NoError(t, err)
	})
	// The session routes as the caterer router places them: in a caterer
	// session, with no meal-order workflow or renderer on the measured path.
	routes := catererauth.NewResource(caterers, caterers)
	caterer := chi.NewRouter()
	caterer.Use(render.SetContentType(render.ContentTypeJSON))
	caterer.Group(func(r chi.Router) {
		api.CatererSessionChain(r)
		routes.MountSession(r)
	})
	router := testutil.NewTenantRouter(db)
	router.Use(api.RequestSettingsCacheMiddleware, api.RequestIdentityCacheMiddleware)
	router.Mount("/caterer", caterer)
	claims := testutil.TenantUserTestClaims(int(account.ID), testpkg.Tenant(t))
	claims.Scope = "caterer"
	claims.TenantID = 0
	claims.Roles = []string{"caterer"}
	token := testutil.MintTestJWT(t, claims)
	counter := testpkg.CaptureQueries(t, db)
	counter.Stop()
	created := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	expires := time.Date(2099, 1, 2, 12, 0, 0, 0, time.UTC)
	devices := []map[string]any{}
	keys := []map[string]any{}
	addDevice := func(owner int64, school any, name string) int64 {
		var id int64
		require.NoError(t, db.NewRaw("INSERT INTO auth.mfa_trusted_devices (account_id,tenant_id,token_hash,user_agent,ip_address,created_at,expires_at) VALUES (?,?,?,?, '127.0.0.1',?,?) RETURNING id", owner, school, fmt.Sprintf("example-device-%d-%s", owner, name), name, created, expires).Scan(context.Background(), &id))
		return id
	}
	addKey := func(owner int64, name string) int64 {
		var id int64
		require.NoError(t, db.NewRaw("INSERT INTO auth.passkey_credentials (account_id,user_handle,credential_id,credential_json,name,created_at) VALUES (?,?,?,?::jsonb,?,?) RETURNING id", owner, []byte(fmt.Sprint(owner)), []byte(fmt.Sprintf("key-%d-%s", owner, name)), `{"id":"fixture"}`, name, created).Scan(context.Background(), &id))
		return id
	}
	addDevice(other.ID, nil, "other-account")
	addDevice(account.ID, testpkg.Tenant(t), "school-device")
	addKey(other.ID, "other-account")
	revoked := addKey(account.ID, "revoked")
	_, err = db.ExecContext(context.Background(), "UPDATE auth.passkey_credentials SET revoked_at=? WHERE id=?", created, revoked)
	require.NoError(t, err)
	expired := addDevice(account.ID, nil, "expired")
	_, err = db.ExecContext(context.Background(), "UPDATE auth.mfa_trusted_devices SET expires_at=? WHERE id=?", created.Add(time.Hour), expired)
	require.NoError(t, err)
	for _, n := range []int{3, 8} {
		for len(devices) < n {
			name := fmt.Sprintf("Caterer browser %d", len(devices))
			id := addDevice(account.ID, nil, name)
			devices = append(devices, map[string]any{"id": float64(id), "user_agent": name, "ip_address": "127.0.0.1", "created_at": created.Format(time.RFC3339), "expires_at": expires.Format(time.RFC3339)})
			name = fmt.Sprintf("Caterer key %d", len(keys))
			id = addKey(account.ID, name)
			var stored time.Time
			require.NoError(t, db.NewRaw("SELECT created_at FROM auth.passkey_credentials WHERE id=?", id).Scan(context.Background(), &stored))
			keys = append(keys, map[string]any{"id": fmt.Sprint(id), "name": name, "created_at": stored.Format(time.RFC3339Nano)})
		}
		for _, path := range []string{"/caterer/auth/mfa/trusted-devices", "/caterer/auth/passkeys"} {
			counter.Reset()
			counter.Start()
			rr := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(token)))
			counter.Stop()
			require.Equal(t, 200, rr.Code, rr.Body.String())
			require.Zero(t, counter.WriteRows())
			for _, op := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
				require.Empty(t, counter.Operation(op))
			}
			var got []map[string]any
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
			t.Logf("CATERER %s N%d count=%d body=%s", path, n, counter.Total(), rr.Body.String())
			if path == "/caterer/auth/passkeys" {
				require.Equal(t, keys, got)
				querybudgets.AssertQueryBudget(t, "api.caterer.auth.passkeys.positive_growth", counter.Measurement(counter.Queries()))
			} else {
				require.ElementsMatch(t, devices, got)
				querybudgets.AssertQueryBudget(t, "api.caterer.auth.mfa.trusted_devices.positive_growth", counter.Measurement(counter.Queries()))
			}
			require.Equal(t, 401, testutil.ExecuteRequest(router, testutil.NewRequest(http.MethodGet, path, nil)).Code)
			wrong := claims
			wrong.Scope = "tenant"
			wrong.TenantID = testpkg.Tenant(t)
			require.Equal(t, 401, testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(testutil.MintTestJWT(t, wrong)))).Code)
		}
	}
}
