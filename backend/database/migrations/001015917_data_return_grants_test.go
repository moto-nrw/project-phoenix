package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The HTTP role reads a grant by its token lookup and counts a download; it
// never issues, revokes or rewrites a grant (ADR 0203).
func TestDataReturnGrantsGiveTheHTTPRoleOnlyTheRetrieval(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	checks := []struct {
		query string
		want  bool
	}{
		{`SELECT has_table_privilege('phoenix_auth', 'platform.data_return_grants', 'SELECT')`, true},
		{`SELECT has_column_privilege('phoenix_auth', 'platform.data_return_grants', 'downloads', 'UPDATE')`, true},
		{`SELECT has_column_privilege('phoenix_auth', 'platform.data_return_grants', 'wrapped_key', 'UPDATE')`, false},
		{`SELECT has_column_privilege('phoenix_auth', 'platform.data_return_grants', 'expires_at', 'UPDATE')`, false},
		{`SELECT has_table_privilege('phoenix_auth', 'platform.data_return_grants', 'INSERT')`, false},
		{`SELECT has_table_privilege('phoenix_auth', 'platform.data_return_grants', 'DELETE')`, false},
		{`SELECT has_table_privilege('phoenix_auth', 'platform.data_return_downloads', 'INSERT')`, true},
		{`SELECT has_table_privilege('phoenix_auth', 'platform.data_return_downloads', 'DELETE')`, false},
		{`SELECT has_column_privilege('phoenix_auth', 'platform.data_return_downloads', 'grant_id', 'UPDATE')`, false},
		{`SELECT has_table_privilege('phoenix_tenant', 'platform.data_return_grants', 'SELECT')`, false},
	}
	for _, check := range checks {
		var got bool
		require.NoError(t, db.NewRaw(check.query).Scan(t.Context(), &got))
		require.Equal(t, check.want, got, check.query)
	}
}
