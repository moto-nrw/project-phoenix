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

	// Since Cutover #2756 the child links live in the three owner tables; the
	// Contract (#2757) removed users.students_guardians.
	for _, relation := range []string{"users.student_guardian_relationships", "users.student_guardian_pickup_permissions", "auth.guardian_student_access"} {
		assert.True(t, roleHasTablePrivilege(t, db, "phoenix_auth", relation, "SELECT"),
			"guardian invitation e-mail rendering reads child links from the phoenix_auth worker connection: %s", relation)
	}
	assert.True(t, roleHasTablePrivilege(t, db, "phoenix_auth", "auth.guardian_invitations", "SELECT"),
		"guardian invitation e-mail rendering loads invitation rows from the phoenix_auth worker connection")
	assert.True(t, roleHasTablePrivilege(t, db, "phoenix_auth", "auth.guardian_invitations", "UPDATE"),
		"guardian invitation e-mail dispatch records send state from the phoenix_auth worker connection")
}
