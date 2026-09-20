package test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/require"
)

func TestStudentFixturesDoNotUseRollbackStorage(t *testing.T) {
	t.Parallel()
	db := SetupIsolatedTestDB(t)
	_, err := db.ExecContext(t.Context(), `SELECT nextval('users.student_school_memberships_id_seq')`)
	require.NoError(t, err)
	student := CreateTestStudent(t, db, "Owner", "Fixture", "3a")
	withAccount, _ := CreateTestStudentWithAccount(t, db, "Account", "Fixture", "4b")
	forTenant := CreateTestStudentForTenant(t, db, Tenant(t), "Tenant", "Fixture", "2c")
	group := CreateTestEducationGroup(t, db, "Contract fixture group")
	AssignStudentToGroup(t, db, student.ID, group.ID)
	SetStudentGroup(t, db, withAccount.ID, &group.ID)
	SetStudentStatus(t, db, student.ID, "inactive")
	AssignStudentGroup(t, db, forTenant.ID, group.ID)
	SetStudentLifecycle(t, db, forTenant.ID, users.StudentStatusAlumnus, nil, nil)
	var membershipID, groupID int64
	var status string
	require.NoError(t, db.NewRaw(`SELECT id, group_id, status FROM users.student_school_memberships
		WHERE student_profile_id=?`, student.ID).Scan(t.Context(), &membershipID, &groupID, &status))
	require.NotEqual(t, student.ID, membershipID)
	require.Equal(t, group.ID, groupID)
	require.Equal(t, "inactive", status)
	require.NoError(t, db.NewRaw(`SELECT status FROM users.student_school_memberships
		WHERE student_profile_id=?`, forTenant.ID).Scan(t.Context(), &status))
	require.Equal(t, "alumnus", status)
	for _, id := range []int64{student.ID, withAccount.ID, forTenant.ID} {
		var complete bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT FROM users.student_profiles p
			JOIN users.student_school_memberships m ON m.tenant_id=p.tenant_id AND m.student_profile_id=p.id
			JOIN users.student_care_profiles c ON c.tenant_id=m.tenant_id AND c.membership_id=m.id
			WHERE p.id=?)`, id).Scan(t.Context(), &complete))
		require.True(t, complete)
	}
	var absent bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('users.students') IS NULL
		AND to_regclass('users.students_legacy') IS NULL
		AND to_regclass('users.student_compatibility_reads') IS NULL
		AND to_regclass('users.student_compatibility_writes') IS NULL`).Scan(t.Context(), &absent))
	require.True(t, absent, "fixtures must work without recreating any rollback storage")
}
