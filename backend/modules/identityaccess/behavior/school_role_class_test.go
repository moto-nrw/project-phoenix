package behavior_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestSchoolRoleClassesRespectTenantAndRoleKind(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	home := testpkg.Tenant(t)
	other, _ := testpkg.CreateTestTenant(t, db)
	member := testpkg.CreateTestAccount(t, db, "classified-member")
	customAdmin := testpkg.CreateTestAccount(t, db, "classified-custom-admin")
	customTeacher := testpkg.CreateTestAccount(t, db, "classified-custom-teacher")
	unassigned := testpkg.CreateTestAccount(t, db, "classified-unassigned")
	admin := testpkg.CreateTestRole(t, db, "classified-admin")
	teacher := testpkg.CreateTestRole(t, db, "classified-teacher")
	_, err = db.ExecContext(ctx, "UPDATE auth.roles SET base_role = 'admin' WHERE id = ?", admin.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "UPDATE auth.roles SET name = 'lehrkraft' WHERE id = ?", teacher.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?), (?, ?, ?)",
		customAdmin.ID, admin.ID, home, customTeacher.ID, teacher.ID, home)
	require.NoError(t, err)
	testpkg.AssignLehrkraftSystemRole(t, db, member.ID, home)
	_, err = db.ExecContext(ctx, "UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?",
		customTeacher.ID, home)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO auth.account_roles (account_id, role_id, tenant_id)
		SELECT ?, id, ? FROM auth.roles WHERE name = 'admin' AND tenant_id IS NULL`, member.ID, other)
	require.NoError(t, err)

	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, home, func(txCtx context.Context) error {
		classes, queryErr := access.ClassifySchoolRoles(txCtx, home, []int64{member.ID, customAdmin.ID, customTeacher.ID, unassigned.ID})
		require.NoError(t, queryErr)
		require.ElementsMatch(t, []identityaccess.SchoolRoleClass{
			{AccountID: member.ID, IsLehrkraft: true},
			{AccountID: customAdmin.ID, IsAdmin: true},
			{AccountID: customTeacher.ID},
		}, classes)
		foreign, queryErr := access.ClassifySchoolRoles(txCtx, other, []int64{member.ID})
		require.NoError(t, queryErr)
		require.Empty(t, foreign, "ambient RLS prevents reading another school's admin assignment")
		return nil
	}))
	require.NoError(t, testpkg.WithinTenantContext(t, tenant.WithTenantID(ctx, other), db, other, func(txCtx context.Context) error {
		classes, queryErr := access.ClassifySchoolRoles(txCtx, other, []int64{member.ID})
		require.NoError(t, queryErr)
		require.Equal(t, []identityaccess.SchoolRoleClass{{AccountID: member.ID, IsAdmin: true}}, classes)
		return nil
	}))
	classes, err := access.ClassifySchoolRoles(ctx, home, nil)
	require.NoError(t, err)
	require.Empty(t, classes)

	rollback := errors.New("roll back classification fixture")
	err = tenant.WithAdminTx(ctx, db, func(txCtx context.Context, tx bun.Tx) error {
		_, insertErr := tx.ExecContext(txCtx, "INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
			member.ID, admin.ID, home)
		require.NoError(t, insertErr)
		classes, queryErr := access.ClassifySchoolRoles(txCtx, home, []int64{member.ID})
		require.NoError(t, queryErr)
		require.Equal(t, []identityaccess.SchoolRoleClass{{AccountID: member.ID, IsAdmin: true, IsLehrkraft: true}}, classes)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	classes, err = access.ClassifySchoolRoles(ctx, home, []int64{member.ID})
	require.NoError(t, err)
	require.Equal(t, []identityaccess.SchoolRoleClass{{AccountID: member.ID, IsLehrkraft: true}}, classes)
}

func TestSchoolRoleClassesDatabaseFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	access, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	school := testpkg.Tenant(t)
	require.NoError(t, db.Close())
	_, err = access.ClassifySchoolRoles(testpkg.Ctx(t), school, []int64{0})
	require.ErrorContains(t, err, "database is closed")
}
