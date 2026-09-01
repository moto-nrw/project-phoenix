package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func roleHasTablePrivilege(t *testing.T, db *testpkg.DB, role, relation, privilege string) bool {
	t.Helper()

	var granted bool
	require.NoError(t, db.NewRaw(`SELECT has_table_privilege(?, ?, ?)`, role, relation, privilege).
		Scan(context.Background(), &granted))
	return granted
}

func TestGuardianInvitationWorkerPrivileges(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	assert.True(t, roleHasTablePrivilege(t, db, "phoenix_auth", "users.students_guardians", "SELECT"),
		"guardian invitation e-mail rendering reads child links from the phoenix_auth worker connection")
	assert.True(t, roleHasTablePrivilege(t, db, "phoenix_auth", "auth.guardian_invitations", "SELECT"),
		"guardian invitation e-mail rendering loads invitation rows from the phoenix_auth worker connection")
	assert.True(t, roleHasTablePrivilege(t, db, "phoenix_auth", "auth.guardian_invitations", "UPDATE"),
		"guardian invitation e-mail dispatch records send state from the phoenix_auth worker connection")
}
