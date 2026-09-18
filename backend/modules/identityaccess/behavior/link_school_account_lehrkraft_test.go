package behavior_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// POST /auth/link-to-tenant is the fourth path that can put the Lehrkraft role
// on an account, and the one that was still open (#1772/#2222). Unlike
// /auth/register the account already exists and may already carry an identity at
// this school — a revoke deliberately leaves person → staff → teacher behind —
// so linking it as Lehrkraft would revive a live caregiver profile and its group
// supervisions under a JWT that only holds class_day permissions.
func TestLinkSchoolAccount_RejectsLehrkraftForCaregiverProfile(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	provisioning := setupAuthFactory(t, db).AccountAuthentication()

	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Link", "Betreuung")

	roleID := lehrkraftRoleID(t, db)
	_, err := provisioning.LinkSchoolAccount(context.Background(), identityaccess.SchoolAccountLink{
		TenantID: testpkg.Tenant(t), Email: account.Email, RoleID: &roleID,
		Identity: &identityaccess.SchoolAccountIdentity{FirstName: "Link", LastName: "Betreuung"},
	})
	require.ErrorIs(t, err, identityaccess.ErrRoleLehrkraftCaregiverProfile)

	// The guard runs inside the link transaction, so the role is not assigned.
	var assigned int
	require.NoError(t, db.NewSelect().
		ColumnExpr("count(*)").
		TableExpr("auth.account_roles").
		Where("account_id = ?", account.ID).
		Scan(testpkg.Ctx(t), &assigned))
	assert.Zero(t, assigned, "a refused link must not assign the role")
}

// The same path with no caregiver profile in the way is the ordinary case and
// must keep working — the guard is about the profile, not about the role.
func TestLinkSchoolAccount_AllowsLehrkraftWithoutCaregiverProfile(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	provisioning := setupAuthFactory(t, db).AccountAuthentication()

	account := testpkg.CreateTestAccount(t, db, "link-lehrkraft-plain")

	roleID := lehrkraftRoleID(t, db)
	provisioned, err := provisioning.LinkSchoolAccount(context.Background(), identityaccess.SchoolAccountLink{
		TenantID: testpkg.Tenant(t), Email: account.Email, RoleID: &roleID,
		Identity: &identityaccess.SchoolAccountIdentity{FirstName: "Link", LastName: "Lehrkraft"},
	})
	require.NoError(t, err)

	// Staff, because Lehrkraft is personnel; no caregiver profile, because it is
	// class_day-read-only by design.
	require.NotNil(t, provisioned.Identity)
	require.NotZero(t, provisioned.Identity.StaffID)
	assert.Zero(t, provisioned.Identity.TeacherID, "the Lehrkraft role never earns a caregiver profile")
}
