package auth

import (
	"context"
	"errors"
	"testing"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStaffOffboardingAccessRequiresCommitHooksForDurableCleanup(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	_, account := testpkg.CreateTestCalendarStaff(t, db, "Durable", "Offboarding")
	service := setupInternalAuthService(t, db)
	preview, err := service.PreviewStaffOffboarding(ctx, account.ID)
	require.NoError(t, err)
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, executeErr := service.ExecuteStaffOffboarding(tenant.ContextWithoutAfterCommitHooks(txCtx), account.ID, preview.Revision)
		return executeErr
	})
	require.ErrorContains(t, err, "commit hooks are required")
	active, err := service.VerifyAccountTenantMembership(ctx, account.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	require.True(t, active)
}

func TestStaffOffboardingAccessRevokesOnlyThisTenantsTokens(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	_, account := testpkg.CreateTestCalendarStaff(t, db, "Session", "Offboarding")
	service := setupInternalAuthService(t, db)
	otherID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherID)
	testpkg.EnsureAccountTenant(t, db, account.ID, otherID)
	testpkg.CreateTestToken(t, db, account.ID, "refresh")
	otherToken := testpkg.CreateTestTokenForTenant(t, db, otherID, account.ID)
	preview, err := service.PreviewStaffOffboarding(ctx, account.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, preview.Tokens)
	result, err := service.ExecuteStaffOffboarding(ctx, account.ID, preview.Revision)
	require.NoError(t, err)
	require.EqualValues(t, 1, result.TokensRevoked)
	tokens, err := service.GetActiveTokens(ctx, int(account.ID))
	require.NoError(t, err)
	require.Empty(t, tokens)
	tokens, err = service.GetActiveTokens(testpkg.TenantContext(otherID), int(account.ID))
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, otherToken.ID, tokens[0].ID)
}

func TestStaffOffboardingAccessPreservesGuardianAndOtherTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	_, account := testpkg.CreateTestCalendarStaff(t, db, "Guardian", "Offboarding")
	service := setupInternalAuthService(t, db)
	roles, err := service.ListRoles(ctx, map[string]interface{}{"name": authModels.BaseRoleGuardian})
	require.NoError(t, err)
	require.Len(t, roles, 1)
	require.NoError(t, service.AssignRoleToAccount(ctx, int(account.ID), int(roles[0].ID)))
	preview, err := service.PreviewStaffOffboarding(ctx, account.ID)
	require.NoError(t, err)
	require.True(t, preview.PreserveGuardian)
	require.False(t, preview.DeactivateAccount)
	result, err := service.ExecuteStaffOffboarding(ctx, account.ID, preview.Revision)
	require.NoError(t, err)
	require.True(t, result.GuardianAccessPreserved)
	remaining, err := service.GetAccountRoles(ctx, int(account.ID))
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	require.Equal(t, authModels.BaseRoleGuardian, remaining[0].Name)
	active, err := service.VerifyAccountTenantMembership(ctx, account.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	require.True(t, active)
	_, err = service.ExecuteStaffOffboarding(ctx, account.ID, preview.Revision)
	require.NoError(t, err)

	_, multiSchool := testpkg.CreateTestCalendarStaff(t, db, "MultiSchool", "Offboarding")
	otherID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherID)
	testpkg.EnsureAccountTenant(t, db, multiSchool.ID, otherID)
	preview, err = service.PreviewStaffOffboarding(ctx, multiSchool.ID)
	require.NoError(t, err)
	require.False(t, preview.DeactivateAccount)
	_, err = service.ExecuteStaffOffboarding(ctx, multiSchool.ID, preview.Revision)
	require.NoError(t, err)
	active, err = service.VerifyAccountTenantMembership(ctx, multiSchool.ID, otherID)
	require.NoError(t, err)
	require.True(t, active)
}

func TestStaffOffboardingAccessRollsBackWithWorkflow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	_, account := testpkg.CreateTestCalendarStaff(t, db, "Identity", "Offboarding")
	service := setupInternalAuthService(t, db)
	preview, err := service.PreviewStaffOffboarding(ctx, account.ID)
	require.NoError(t, err)
	require.True(t, preview.DeactivateAccount)
	require.NotEmpty(t, preview.RoleIDs)
	failure := errors.New("later workflow command failed")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, executeErr := service.ExecuteStaffOffboarding(txCtx, account.ID, preview.Revision)
		require.NoError(t, executeErr)
		return failure
	})
	require.ErrorIs(t, err, failure)
	active, err := service.VerifyAccountTenantMembership(ctx, account.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	require.True(t, active)
	roles, err := service.GetAccountRoles(ctx, int(account.ID))
	require.NoError(t, err)
	require.Len(t, roles, len(preview.RoleIDs))
	_, err = service.ExecuteStaffOffboarding(ctx, account.ID, preview.Revision)
	require.NoError(t, err)
	active, err = service.VerifyAccountTenantMembership(ctx, account.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	require.False(t, active)
	_, err = service.ExecuteStaffOffboarding(ctx, account.ID, preview.Revision)
	require.NoError(t, err)
}
