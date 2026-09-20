package users_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	authModels "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authmodels"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// moveGuardianAccessToOtherSchool gives the chain's account an ACTIVE mapping
// and the guardian role at a second school, then applies change at the
// chain's own school. The owner queries list both schools, so only the
// (account_id, tenant_id) pairing keeps the second school's facts from
// authorizing reads at the first.
func moveGuardianAccessToOtherSchool(t *testing.T, db *bun.DB, chain testpkg.ParentChain, change string) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	other, _ := testpkg.CreateTestTenant(t, db)
	testpkg.EnsureAccountTenant(t, db, chain.AccountID, other)
	_, err := db.ExecContext(ctx, `
		INSERT INTO auth.account_roles (account_id, role_id, tenant_id)
		SELECT ?, id, ? FROM auth.roles WHERE name = ? AND tenant_id IS NULL`,
		chain.AccountID, other, authModels.BaseRoleGuardian)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, change, chain.AccountID, chain.TenantID)
	require.NoError(t, err)
}

const (
	deactivateMapping = `UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?`
	dropGuardianRole  = `DELETE FROM auth.account_roles WHERE account_id = ? AND tenant_id = ?`
)

// TestIdentityMembership_PairsTheRelationshipSchool pins the #2721 cutover:
// the People Directory reads filter through the Identity & Access owner
// queries, and a membership or guardian role at another school never stands
// in for the one at the relationship's school.
func TestIdentityMembership_PairsTheRelationshipSchool(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	guardians := repositories.NewStudentGuardianRepository(db)
	profiles := repositories.NewGuardianProfileRepository(db)
	recipients := repositories.NewMessageableGuardianRepository(db)
	perm := authorize.GuardianPermissionPortalAccess

	t.Run("an active mapping at the relationship's school grants access", func(t *testing.T) {
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		granted, err := guardians.AccountHasStudentPermission(ctx, chain.AccountID, chain.StudentID, chain.TenantID, perm)
		require.NoError(t, err)
		assert.True(t, granted)

		byEmail, err := guardians.GuardianEmailHasStudentPermission(ctx, chain.Email, chain.StudentID, chain.TenantID, perm)
		require.NoError(t, err)
		assert.True(t, byEmail)

		permitted, err := guardians.FilterAccountsWithStudentAccess(ctx, []int64{chain.AccountID}, []int64{chain.StudentID}, chain.TenantID, perm)
		require.NoError(t, err)
		assert.Equal(t, []int64{chain.AccountID}, permitted)

		reachable, err := profiles.FindActivePortalProfilesByIDs(ctx, []int64{chain.GuardianProfileID})
		require.NoError(t, err)
		assert.Contains(t, reachable, chain.GuardianProfileID)

		listed, err := recipients.ListGuardiansForStudent(ctx, chain.StudentID)
		require.NoError(t, err)
		require.Len(t, listed, 1)
		assert.Equal(t, chain.AccountID, listed[0].AccountID)
	})

	t.Run("a mapping at another school does not grant access here", func(t *testing.T) {
		chain := testpkg.CreateTestParentGuardianChain(t, db)
		moveGuardianAccessToOtherSchool(t, db, chain, deactivateMapping)

		granted, err := guardians.AccountHasStudentPermission(ctx, chain.AccountID, chain.StudentID, chain.TenantID, perm)
		require.NoError(t, err)
		assert.False(t, granted)

		byEmail, err := guardians.GuardianEmailHasStudentPermission(ctx, chain.Email, chain.StudentID, chain.TenantID, perm)
		require.NoError(t, err)
		assert.False(t, byEmail, "a profile with an account needs the active mapping at its own school")

		permitted, err := guardians.FilterAccountsWithStudentAccess(ctx, []int64{chain.AccountID}, []int64{chain.StudentID}, chain.TenantID, perm)
		require.NoError(t, err)
		assert.Empty(t, permitted)

		reachable, err := profiles.FindActivePortalProfilesByIDs(ctx, []int64{chain.GuardianProfileID})
		require.NoError(t, err)
		assert.NotContains(t, reachable, chain.GuardianProfileID)

		listed, err := recipients.ListGuardiansForStudent(ctx, chain.StudentID)
		require.NoError(t, err)
		assert.Empty(t, listed)
	})

	t.Run("a guardian role at another school does not make the profile reachable here", func(t *testing.T) {
		chain := testpkg.CreateTestParentGuardianChain(t, db)
		moveGuardianAccessToOtherSchool(t, db, chain, dropGuardianRole)

		reachable, err := profiles.FindActivePortalProfilesByIDs(ctx, []int64{chain.GuardianProfileID})
		require.NoError(t, err)
		assert.NotContains(t, reachable, chain.GuardianProfileID)
	})
}

// TestIdentityMembership_UnboundQueriesFailClosed pins that a composition
// without the Identity & Access owner queries reports an error instead of
// widening or silently emptying a result.
func TestIdentityMembership_UnboundQueriesFailClosed(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	perm := authorize.GuardianPermissionPortalAccess

	unbound := usersRepo.NewStudentGuardianRepository(db)
	_, err := unbound.AccountHasStudentPermission(ctx, chain.AccountID, chain.StudentID, chain.TenantID, perm)
	require.Error(t, err)
	_, err = unbound.FilterAccountsWithStudentAccess(ctx, []int64{chain.AccountID}, []int64{chain.StudentID}, chain.TenantID, perm)
	require.Error(t, err)
	_, err = unbound.GuardianEmailHasStudentPermission(ctx, chain.Email, chain.StudentID, chain.TenantID, perm)
	require.Error(t, err)

	_, err = usersRepo.NewMessageableGuardianRepository(db, nil).ListGuardiansForStudent(ctx, chain.StudentID)
	require.Error(t, err)

	_, err = usersRepo.NewGuardianProfileRepository(db).
		FindActivePortalProfilesByIDs(ctx, []int64{chain.GuardianProfileID})
	require.ErrorContains(t, err, "portal membership query is required")
	projectionFailure := errors.New("portal membership lookup failed")
	failingProfiles := usersRepo.NewGuardianProfileRepository(db, usersRepo.WithPortalMemberships(
		func(_ context.Context, accountIDs []int64) (map[int64][]int64, error) {
			require.Equal(t, []int64{chain.AccountID}, accountIDs)
			return nil, projectionFailure
		}))
	profiles, err := failingProfiles.FindActivePortalProfilesByIDs(ctx, []int64{chain.GuardianProfileID})
	require.ErrorIs(t, err, projectionFailure)
	require.Nil(t, profiles, "failed account reachability must not return candidates")

	activeAccounts := func(context.Context) *bun.SelectQuery {
		return db.NewSelect().TableExpr(`auth.accounts AS "account"`).ColumnExpr(`"account".id`).Where(`"account".active = TRUE`)
	}

	staffAccounts := func(context.Context) ([]int64, error) { return []int64{chain.AccountID}, nil }
	reads := usersRepo.NewMessageableStaffRepository(db, staffAccounts, usersRepo.StaffMessageIdentity{ActiveAccounts: activeAccounts})
	_, err = reads.ListMessageableStaff(ctx, chain.AccountID)
	require.ErrorContains(t, err, "membership queries are required")
	_, err = reads.IsMessageableStaff(ctx, chain.AccountID)
	require.ErrorContains(t, err, "membership queries are required")
	_, err = reads.StaffRoleKinds(ctx, []int64{chain.AccountID})
	require.ErrorContains(t, err, "role class query is required")
}

// TestIdentityMembership_RoleClassFailureIsNotSwallowed pins that a failing
// owner read surfaces through the staff role classification instead of
// degrading every account to plain staff.
func TestIdentityMembership_RoleClassFailureIsNotSwallowed(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "role-class-failure")
	ownerFailure := errors.New("identity access unavailable")

	reads := usersRepo.NewMessageableStaffRepository(db, nil, usersRepo.StaffMessageIdentity{
		RoleClasses: func(context.Context, int64, []int64) ([]usersRepo.SchoolRoleClass, error) {
			return nil, ownerFailure
		},
	})
	kinds, err := reads.StaffRoleKinds(ctx, []int64{account.ID})
	require.ErrorIs(t, err, ownerFailure)
	assert.Nil(t, kinds)
}
