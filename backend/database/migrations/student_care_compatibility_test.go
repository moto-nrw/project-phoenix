package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStudentCareAbsenceOwnerAndRollbackImageShareState(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Care", "Compatibility", "2a")
	_, err := db.NewRaw(`UPDATE users.student_care_profiles AS c SET sick = true
		FROM users.student_school_memberships AS m
		WHERE m.tenant_id = ? AND m.student_profile_id = ?
		AND c.tenant_id = m.tenant_id AND c.membership_id = m.id`, testpkg.Tenant(t), student.ID).Exec(ctx)
	require.NoError(t, err)
	var sick bool
	require.NoError(t, db.NewRaw(`SELECT sick FROM users.students WHERE tenant_id = ? AND id = ?`, testpkg.Tenant(t), student.ID).Scan(ctx, &sick))
	require.True(t, sick, "the previous image must see the owner command")
	_, err = db.NewRaw(`UPDATE users.students SET sick = false, excused = true WHERE tenant_id = ? AND id = ?`, testpkg.Tenant(t), student.ID).Exec(ctx)
	require.NoError(t, err)
	var excused bool
	require.NoError(t, db.NewRaw(`SELECT c.sick, c.excused FROM users.student_care_profiles AS c
		JOIN users.student_school_memberships AS m ON m.tenant_id = c.tenant_id AND m.id = c.membership_id
		WHERE m.tenant_id = ? AND m.student_profile_id = ?`, testpkg.Tenant(t), student.ID).Scan(ctx, &sick, &excused))
	require.False(t, sick)
	require.True(t, excused, "rollback writes must reach the same care owner")
}
