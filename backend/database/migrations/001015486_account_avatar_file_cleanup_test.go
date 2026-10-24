package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestAccountAvatarCleanupIsAdministrativeOnly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, role := range []string{"phoenix_auth", "phoenix_tenant"} {
		var accessible bool
		require.NoError(t, db.NewRaw(`SELECT has_table_privilege(?, 'auth.account_avatar_file_cleanup', 'SELECT,INSERT,UPDATE,DELETE')`, role).Scan(t.Context(), &accessible))
		require.False(t, accessible, "global cleanup is not accessible to %s", role)
	}
	var admin, enabled, forced bool
	require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_admin', 'auth.account_avatar_file_cleanup', 'SELECT,INSERT,UPDATE,DELETE'),
		relrowsecurity, relforcerowsecurity FROM pg_class WHERE oid = 'auth.account_avatar_file_cleanup'::regclass`).Scan(t.Context(), &admin, &enabled, &forced))
	require.True(t, admin)
	require.True(t, enabled)
	require.True(t, forced)
}
