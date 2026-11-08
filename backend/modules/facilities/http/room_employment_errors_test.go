//go:build testhooks

package httpadapter_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func withRoomEmploymentReadDenied(t *testing.T, db *bun.DB, run func()) {
	t.Helper()
	// Refuse a shared package pool: the existing N=3 budget fixtures already
	// selected their test-owned disposable database before composing roots.
	require.Same(t, testpkg.SetupIsolatedTestDB(t), db)
	canRead := func() bool {
		t.Helper()
		var allowed bool
		require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', 'users.staff_employment_profiles', 'SELECT')`).Scan(context.Background(), &allowed))
		return allowed
	}
	require.True(t, canRead())
	defer func() {
		_, err := db.ExecContext(context.Background(), `GRANT SELECT ON TABLE users.staff_employment_profiles TO phoenix_tenant`)
		require.NoError(t, err)
		require.True(t, canRead())
	}()
	_, err := db.ExecContext(context.Background(), `REVOKE SELECT ON TABLE users.staff_employment_profiles FROM phoenix_tenant`)
	require.NoError(t, err)
	require.False(t, canRead(), "the effective SELECT must actually be absent")
	run()
}

func assertRoomEmploymentReadIndependent(t *testing.T, counter *testpkg.QueryCounter, response *httptest.ResponseRecorder) {
	t.Helper()
	// E1 (#5570): unused Employment failures no longer reject room reads.
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var body struct {
		Status string            `json:"status"`
		Data   []json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, "success", body.Status)
	require.Len(t, body.Data, 3)
	require.Empty(t, counter.Selects("staff_employment_profiles"))
	for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
		require.Empty(t, counter.Operation(operation))
	}
	require.Zero(t, counter.WriteRows())
}
