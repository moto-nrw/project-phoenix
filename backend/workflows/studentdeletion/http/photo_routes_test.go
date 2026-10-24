package studentdeletionhttp_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func graduateStudent(t *testing.T, tc *testContext, studentID int64) {
	t.Helper()
	_, err := tc.db.ExecContext(testpkg.Ctx(t), `
		UPDATE users.student_school_memberships SET status = ? WHERE student_profile_id = ? AND deleted_at IS NULL
	`, peopledirectory.StudentStatusAlumnus, studentID)
	require.NoError(t, err)
}
