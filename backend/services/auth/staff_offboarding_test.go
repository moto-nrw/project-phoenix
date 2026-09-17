package auth_test

import (
	"context"
	"errors"
	"testing"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The staff offboarding access step lives in Identity & Access (#3225); these
// behaviour tests drive it through the composed module and read the retained
// account facts back through the auth service the factory composes with it.

func TestStaffOffboardingAccessRequiresCommitHooksForDurableCleanup(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	_, account := testpkg.CreateTestCalendarStaff(t, db, "Durable", "Offboarding")
	factory := setupAuthFactory(t, db)
	access := factory.AccountAuthentication()
	preview, err := access.PreviewStaffOffboarding(ctx, account.ID)
	require.NoError(t, err)
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, executeErr := access.ExecuteStaffOffboarding(tenant.ContextWithoutAfterCommitHooks(txCtx), account.ID, preview.Revision)
		return executeErr
	})
	require.ErrorContains(t, err, "commit hooks are required")
	active, err := factory.Auth.VerifyAccountTenantMembership(ctx, account.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	require.True(t, active)
}

func TestStaffOffboardingAccessRevokesOnlyThisTenantsTokens(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	_, account := testpkg.CreateTestCalendarStaff(t, db, "Session", "Offboarding")
	factory := setupAuthFactory(t, db)
	access := factory.AccountAuthentication()
	otherID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherID)
	testpkg.EnsureAccountTenant(t, db, account.ID, otherID)
	testpkg.CreateTestToken(t, db, account.ID, "refresh")
	otherToken := testpkg.CreateTestTokenForTenant(t, db, otherID, account.ID)
	preview, err := access.PreviewStaffOffboarding(ctx, account.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, preview.Tokens)
	result, err := access.ExecuteStaffOffboarding(ctx, account.ID, preview.Revision)
	require.NoError(t, err)
	require.EqualValues(t, 1, result.TokensRevoked)
	tokens, err := factory.Auth.GetActiveTokens(ctx, int(account.ID))
	require.NoError(t, err)
	require.Empty(t, tokens)
	tokens, err = factory.Auth.GetActiveTokens(testpkg.TenantContext(otherID), int(account.ID))
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, otherToken.ID, tokens[0].ID)
}

func TestStaffOffboardingAccessPreservesGuardianAndOtherTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	_, account := testpkg.CreateTestCalendarStaff(t, db, "Guardian", "Offboarding")
	factory := setupAuthFactory(t, db)
	access := factory.AccountAuthentication()
	roles, err := access.ListRoles(ctx, identityaccess.RoleFilter{Name: authModels.BaseRoleGuardian})
	require.NoError(t, err)
	require.Len(t, roles, 1)
	require.NoError(t, access.AssignRoleToAccount(ctx, account.ID, roles[0].ID))
	preview, err := access.PreviewStaffOffboarding(ctx, account.ID)
	require.NoError(t, err)
	require.True(t, preview.PreserveGuardian)
	require.False(t, preview.DeactivateAccount)
	result, err := access.ExecuteStaffOffboarding(ctx, account.ID, preview.Revision)
	require.NoError(t, err)
	require.True(t, result.GuardianAccessPreserved)
	remaining, err := access.GetAccountRoles(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	require.Equal(t, authModels.BaseRoleGuardian, remaining[0].Name)
	active, err := factory.Auth.VerifyAccountTenantMembership(ctx, account.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	require.True(t, active)
	_, err = access.ExecuteStaffOffboarding(ctx, account.ID, preview.Revision)
	require.NoError(t, err)

	_, multiSchool := testpkg.CreateTestCalendarStaff(t, db, "MultiSchool", "Offboarding")
	otherID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherID)
	testpkg.EnsureAccountTenant(t, db, multiSchool.ID, otherID)
	preview, err = access.PreviewStaffOffboarding(ctx, multiSchool.ID)
	require.NoError(t, err)
	require.False(t, preview.DeactivateAccount)
	_, err = access.ExecuteStaffOffboarding(ctx, multiSchool.ID, preview.Revision)
	require.NoError(t, err)
	active, err = factory.Auth.VerifyAccountTenantMembership(ctx, multiSchool.ID, otherID)
	require.NoError(t, err)
	require.True(t, active)
}

func TestStaffOffboardingAccessRollsBackWithWorkflow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	_, account := testpkg.CreateTestCalendarStaff(t, db, "Identity", "Offboarding")
	factory := setupAuthFactory(t, db)
	access := factory.AccountAuthentication()
	preview, err := access.PreviewStaffOffboarding(ctx, account.ID)
	require.NoError(t, err)
	require.True(t, preview.DeactivateAccount)
	require.NotEmpty(t, preview.RoleIDs)
	failure := errors.New("later workflow command failed")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, executeErr := access.ExecuteStaffOffboarding(txCtx, account.ID, preview.Revision)
		require.NoError(t, executeErr)
		return failure
	})
	require.ErrorIs(t, err, failure)
	active, err := factory.Auth.VerifyAccountTenantMembership(ctx, account.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	require.True(t, active)
	roles, err := access.GetAccountRoles(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, roles, len(preview.RoleIDs))
	_, err = access.ExecuteStaffOffboarding(ctx, account.ID, preview.Revision)
	require.NoError(t, err)
	active, err = factory.Auth.VerifyAccountTenantMembership(ctx, account.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	require.False(t, active)
	_, err = access.ExecuteStaffOffboarding(ctx, account.ID, preview.Revision)
	require.NoError(t, err)
}
