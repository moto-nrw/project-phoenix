package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func roleHasPermissionCatalogPrivilege(t *testing.T, db *bun.DB, role, privilege string) bool {
	t.Helper()

	var granted bool
	require.NoError(t, db.NewRaw(`SELECT has_table_privilege(?, 'auth.permissions', ?)`, role, privilege).
		Scan(context.Background(), &granted))
	return granted
}

func TestPermissionCatalogPrivileges(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	assert.True(t, roleHasPermissionCatalogPrivilege(t, db, "phoenix_tenant", "SELECT"))
	for _, privilege := range []string{"INSERT", "UPDATE", "DELETE"} {
		assert.Falsef(t, roleHasPermissionCatalogPrivilege(t, db, "phoenix_tenant", privilege),
			"phoenix_tenant must not hold %s on the global permission catalog", privilege)
		assert.Truef(t, roleHasPermissionCatalogPrivilege(t, db, "phoenix_admin", privilege),
			"phoenix_admin must retain %s on the global permission catalog", privilege)
	}
}
