package auth

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// lehrkraftSystemRoleID resolves the platform Lehrkraft role the migrations
// seed. Shared by the guard tests in both directions.
func lehrkraftSystemRoleID(t *testing.T, db *bun.DB) int64 {
	t.Helper()
	var roleID int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("LOWER(name) = 'lehrkraft'").
		Where("is_system = true").
		Where("tenant_id IS NULL").
		Scan(context.Background(), &roleID),
		"lehrkraft system role must exist in the test schema")
	return roleID
}

// POST /auth/link-to-tenant is the fourth path that can put the Lehrkraft role
// on an account, and the one that was still open (#1772/#2222). Unlike
// /auth/register the account already exists and may already carry an identity at
// this school — a revoke deliberately leaves person → staff → teacher behind —
// so linking it as Lehrkraft would revive a live caregiver profile and its group
// supervisions under a JWT that only holds class_day permissions.
func TestLinkSchoolAccount_RejectsLehrkraftForCaregiverProfile(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupInternalAuthService(t, db)

	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Link", "Betreuung")

	roleID := lehrkraftSystemRoleID(t, db)
	_, _, err := service.LinkSchoolAccount(context.Background(), account.Email, &roleID, testpkg.Tenant(t),
		&SchoolAccountIdentity{FirstName: "Link", LastName: "Betreuung"})
	require.ErrorIs(t, err, ErrRoleLehrkraftCaregiverProfile)

	// The guard runs inside the link transaction, so the role is not assigned.
	roles, err := service.repos.Role.FindByAccountID(testpkg.Ctx(t), account.ID)
	require.NoError(t, err)
	assert.Empty(t, roles, "a refused link must not assign the role")
}

// The same path with no caregiver profile in the way is the ordinary case and
// must keep working — the guard is about the profile, not about the role.
func TestLinkSchoolAccount_AllowsLehrkraftWithoutCaregiverProfile(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupInternalAuthService(t, db)

	account := testpkg.CreateTestAccount(t, db, "link-lehrkraft-plain")

	roleID := lehrkraftSystemRoleID(t, db)
	_, identity, err := service.LinkSchoolAccount(context.Background(), account.Email, &roleID, testpkg.Tenant(t),
		&SchoolAccountIdentity{FirstName: "Link", LastName: "Lehrkraft"})
	require.NoError(t, err)

	// Staff, because Lehrkraft is personnel; no caregiver profile, because it is
	// class_day-read-only by design.
	require.NotNil(t, identity)
	require.NotZero(t, identity.StaffID)
	assert.Zero(t, identity.TeacherID, "the Lehrkraft role never earns a caregiver profile")
}
