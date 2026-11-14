//go:build testhooks

package parentauth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/inbound/parentauth"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

// The parents portal's list of remembered devices (#7051) loads set-based
// and lists only the account's devices of that portal: never the caterer
// portal's, a school's, another account's or an expired one.
func TestParentTrustedDevicesQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, err := api.NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	access, ok := module.Auth.(identityaccess.ParentPortalAccess)
	require.True(t, ok)
	parents := access.Parents()
	require.NotNil(t, parents)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	other := testpkg.CreateTestAccount(t, db, "parent-device-decoy")
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), "DELETE FROM auth.mfa_trusted_devices WHERE account_id IN (?, ?)", chain.AccountID, other.ID)
		require.NoError(t, err)
	})
	parent := chi.NewRouter()
	parent.Use(render.SetContentType(render.ContentTypeJSON))
	parent.Group(func(r chi.Router) {
		api.ParentSessionChain(r)
		parentauth.NewResource(parents).MountSession(r)
	})
	router := testutil.NewTenantRouter(db)
	router.Use(api.RequestSettingsCacheMiddleware, api.RequestIdentityCacheMiddleware)
	router.Mount("/parent", parent)
	claims := sessiontoken.AppClaims{ID: int(chain.AccountID), Sub: fmt.Sprint(chain.AccountID), Roles: []string{"parent"}, Scope: "parent"}
	token := testutil.MintTestJWT(t, claims)

	created := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	expires := time.Date(2099, 1, 2, 12, 0, 0, 0, time.UTC)
	addDevice := func(owner int64, school any, parentPortal bool, name string) int64 {
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO auth.mfa_trusted_devices (account_id, tenant_id, parent_portal, token_hash, user_agent, ip_address, created_at, expires_at)
			VALUES (?, ?, ?, ?, ?, '127.0.0.1', ?, ?) RETURNING id`, owner, school, parentPortal, fmt.Sprintf("example-device-%d-%s", owner, name), name, created, expires).Scan(context.Background(), &id))
		return id
	}
	addDevice(other.ID, nil, true, "other-account")
	addDevice(chain.AccountID, testpkg.Tenant(t), false, "school-device")
	addDevice(chain.AccountID, nil, false, "caterer-device")
	expired := addDevice(chain.AccountID, nil, true, "expired")
	_, err = db.ExecContext(context.Background(), "UPDATE auth.mfa_trusted_devices SET expires_at = ? WHERE id = ?", created.Add(time.Hour), expired)
	require.NoError(t, err)

	counter := testpkg.CaptureQueries(t, db)
	counter.Stop()
	devices := []map[string]any{}
	for _, n := range []int{3, 8} {
		for len(devices) < n {
			name := fmt.Sprintf("Parent browser %d", len(devices))
			id := addDevice(chain.AccountID, nil, true, name)
			devices = append(devices, map[string]any{"id": float64(id), "user_agent": name, "ip_address": "127.0.0.1", "created_at": created.Format(time.RFC3339), "expires_at": expires.Format(time.RFC3339)})
		}
		counter.Reset()
		counter.Start()
		rr := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, http.MethodGet, "/parent/me/mfa/trusted-devices", nil, testutil.WithJWTBearer(token)))
		counter.Stop()
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		require.Zero(t, counter.WriteRows())
		var got []map[string]any
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
		require.ElementsMatch(t, devices, got)
		querybudgets.AssertQueryBudget(t, "api.parent.me.mfa.trusted_devices.positive_growth", counter.Measurement(counter.Queries()))
	}
	wrong := claims
	wrong.Scope = "caterer"
	rr := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, http.MethodGet, "/parent/me/mfa/trusted-devices", nil, testutil.WithJWTBearer(testutil.MintTestJWT(t, wrong))))
	require.Equal(t, http.StatusUnauthorized, rr.Code, "a caterer token lists no parent devices")
}
