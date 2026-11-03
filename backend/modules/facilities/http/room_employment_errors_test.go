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

func assertRoomEmploymentReadFailure(t *testing.T, counter *testpkg.QueryCounter, response *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, "general.server", body["code"])
	require.Equal(t, float64(http.StatusInternalServerError), body["status"])
	require.NotContains(t, body, "data")
	require.Len(t, counter.Selects("staff_employment_profiles"), 1)
	for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
		require.Empty(t, counter.Operation(operation))
	}
	require.Zero(t, counter.WriteRows())
}
