package auth_test

import (
	"context"
	"testing"

	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type membershipPair struct {
	AccountID int64 `bun:"account_id"`
	TenantID  int64 `bun:"tenant_id"`
}

// scanPairs runs an owner query on the transaction it was built on and keeps
// only the rows of the given accounts, so parallel fixtures stay out. Only
// the mapping and assignment tables carry an account_id column, so the
// unqualified filter is unambiguous.
func scanPairs(ctx context.Context, query *bun.SelectQuery, accountIDs ...int64) ([]membershipPair, error) {
	var rows []membershipPair
	err := query.
		Where(`account_id IN (?)`, bun.List(accountIDs)).
		OrderExpr(`1, 2`).
		Scan(ctx, &rows)
	return rows, err
}

func assignSystemRole(t *testing.T, db *bun.DB, accountID, tenantID int64, roleName string) {
	t.Helper()
	_, err := db.ExecContext(testpkg.Ctx(t), `
		INSERT INTO auth.account_roles (account_id, role_id, tenant_id)
		SELECT ?, id, ? FROM auth.roles WHERE name = ? AND tenant_id IS NULL`,
		accountID, tenantID, roleName)
	require.NoError(t, err)
}

// TestMembershipQueries_TwoTenantIsolation pins the Identity & Access owner
// queries other owners join instead of reading auth.account_tenants,
// auth.account_roles and auth.roles (#2721). Role assignments are protected
// by row-level security, so a school transaction sees only its own. The
// school mapping is not: the membership query names the school of every
// ACTIVE mapping, and consumers pair it with their own tenant column.
func TestMembershipQueries_TwoTenantIsolation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	home := testpkg.Tenant(t)
	other, _ := testpkg.CreateTestTenant(t, db)

	member := testpkg.CreateTestAccount(t, db, "membership-owner")
	testpkg.EnsureAccountTenant(t, db, member.ID, other)
	departed := testpkg.CreateTestAccount(t, db, "membership-departed")
	_, err := db.ExecContext(ctx,
		`UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?`,
		departed.ID, home)
	require.NoError(t, err)

	assignSystemRole(t, db, member.ID, home, authModels.BaseRoleGuardian)
	assignSystemRole(t, db, member.ID, other, authModels.BaseRoleGuardian)
	assignSystemRole(t, db, departed.ID, home, authModels.BaseRoleGuardian)
	testpkg.AssignLehrkraftSystemRole(t, db, member.ID, home)
	assignSystemRole(t, db, member.ID, other, authModels.BaseRoleAdmin)

	tenants, ok := authRepo.NewAccountTenantRepository(db).(*authRepo.AccountTenantRepository)
	require.True(t, ok)
	roles, ok := authRepo.NewAccountRoleRepository(db).(*authRepo.AccountRoleRepository)
	require.True(t, ok)

	withinSchool := func(t *testing.T, tenantID int64, fn func(context.Context)) {
		t.Helper()
		require.NoError(t, testpkg.WithinTenantContext(t, testpkg.TenantContext(tenantID), db, tenantID, func(txCtx context.Context) error {
			fn(txCtx)
			return nil
		}))
	}

	t.Run("active memberships name the school of every active mapping", func(t *testing.T) {
		want := []membershipPair{
			{AccountID: member.ID, TenantID: home},
			{AccountID: member.ID, TenantID: other},
		}
		withinSchool(t, home, func(txCtx context.Context) {
			rows, err := scanPairs(txCtx, tenants.ActiveMemberships(txCtx), member.ID, departed.ID)
			require.NoError(t, err)
			assert.ElementsMatch(t, want, rows, "the inactive mapping is never a membership")
		})
		require.NoError(t, testpkg.WithinAdminContext(t, ctx, db, func(txCtx context.Context) error {
			rows, err := scanPairs(txCtx, tenants.ActiveMemberships(txCtx), member.ID, departed.ID)
			require.NoError(t, err)
			assert.ElementsMatch(t, want, rows)
			return nil
		}))
	})

	t.Run("guardian role holders are scoped to the school in context", func(t *testing.T) {
		withinSchool(t, home, func(txCtx context.Context) {
			rows, err := scanPairs(txCtx, roles.GuardianRoleHolders(txCtx), member.ID, departed.ID)
			require.NoError(t, err)
			assert.ElementsMatch(t, []membershipPair{
				{AccountID: member.ID, TenantID: home},
				{AccountID: departed.ID, TenantID: home},
			}, rows, "the role assignment is its own fact; the mapping status is filtered by the consumer")
		})
		withinSchool(t, other, func(txCtx context.Context) {
			rows, err := scanPairs(txCtx, roles.GuardianRoleHolders(txCtx), member.ID, departed.ID)
			require.NoError(t, err)
			assert.Equal(t, []membershipPair{{AccountID: member.ID, TenantID: other}}, rows)
		})
	})

	t.Run("role classes follow the school's own assignments", func(t *testing.T) {
		withinSchool(t, home, func(txCtx context.Context) {
			classes, err := roles.ClassifySchoolRoles(txCtx, home, []int64{member.ID, departed.ID})
			require.NoError(t, err)
			assert.ElementsMatch(t, []authRepo.SchoolRoleClass{
				{AccountID: member.ID, IsLehrkraft: true},
				{AccountID: departed.ID},
			}, classes)

			foreign, err := roles.ClassifySchoolRoles(txCtx, other, []int64{member.ID})
			require.NoError(t, err)
			assert.Empty(t, foreign, "a school transaction must not classify another school's admin role")
		})
		withinSchool(t, other, func(txCtx context.Context) {
			classes, err := roles.ClassifySchoolRoles(txCtx, other, []int64{member.ID})
			require.NoError(t, err)
			assert.Equal(t, []authRepo.SchoolRoleClass{{AccountID: member.ID, IsAdmin: true}}, classes)
		})
	})

	t.Run("an empty account list classifies nothing", func(t *testing.T) {
		classes, err := roles.ClassifySchoolRoles(ctx, home, nil)
		require.NoError(t, err)
		assert.Empty(t, classes)
	})
}
