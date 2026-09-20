package behavior_test

// Native persistence tests for auth.mfa_overrides.
//
// The repository covers two row shapes that share the same table:
//
//   - Platform-wide ("global") row: tenant_id IS NULL, set_by_type = "operator".
//     Wins over every tenant override and applies on every login. Operator-only.
//   - Tenant-scoped row: tenant_id = N. Set either by a tenant admin
//     (set_by_type = "account") or by an operator working on behalf of a
//     single school (set_by_type = "operator").
//
// These tests exercise both paths and their validation branches. They
// run as the postgres superuser (BYPASSRLS) so RLS policies are exercised
// at the application layer via the service tests in services/auth/,
// not here.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func newOverrideRepoTestFixtures(t *testing.T) (
	ctx context.Context,
	repo services.AccountMFARecords,
	accountID, tenantID int64,
) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	ctx = context.Background()

	account := testpkg.CreateTestAccount(t, db, "mfa-override")
	tenantID = testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	repo = nativeMFARecords(t, db)

	return ctx, repo, account.ID, tenantID
}

// --- UpsertGlobal -----------------------------------------------------

func TestNativeMFAOverride_UpsertGlobal_HappyPath(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, _ := newOverrideRepoTestFixtures(t)

	require.NoError(t, repo.UpsertGlobalOverride(ctx, identityaccess.AccountMFAOverride{
		AccountID: accountID,
		Override:  identityaccess.MFAAdminOverrideForceOff,
		SetBy:     42,
		SetByType: "operator",
		Reason:    "user has no email access",
	}))

	got, found, err := repo.FindGlobalOverride(ctx, accountID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Nil(t, got.TenantID, "global row must have tenant_id NULL")
	assert.Equal(t, identityaccess.MFAAdminOverrideForceOff, got.Override)
	assert.Nil(t, got.TenantID)
}

func TestNativeMFAOverride_UpsertGlobal_UpdatesExistingRow(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, _ := newOverrideRepoTestFixtures(t)

	require.NoError(t, repo.UpsertGlobalOverride(ctx, identityaccess.AccountMFAOverride{
		AccountID: accountID,
		Override:  identityaccess.MFAAdminOverrideForceOff,
		SetBy:     42,
		SetByType: "operator",
		Reason:    "initial",
	}))
	require.NoError(t, repo.UpsertGlobalOverride(ctx, identityaccess.AccountMFAOverride{
		AccountID: accountID,
		Override:  identityaccess.MFAAdminOverrideForceOn,
		SetBy:     77,
		SetByType: "operator",
		Reason:    "policy reversal",
	}))

	var count int
	err := testpkg.SetupTestDB(t).NewRaw("SELECT COUNT(*) FROM auth.mfa_overrides WHERE account_id = ?", accountID).Scan(ctx, &count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "ON CONFLICT must replace, not append")
	got, found, err := repo.FindGlobalOverride(ctx, accountID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, identityaccess.MFAAdminOverrideForceOn, got.Override)
	assert.EqualValues(t, 77, got.SetBy)
	assert.Equal(t, "policy reversal", got.Reason)
}

func TestNativeMFAOverride_UpsertGlobal_ValidationBranches(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, tenantID := newOverrideRepoTestFixtures(t)

	tid := tenantID
	cases := map[string]identityaccess.AccountMFAOverride{
		"account_id zero": {
			Override:  identityaccess.MFAAdminOverrideForceOff,
			SetByType: "operator",
		},
		"tenant_id must be nil for global": {
			AccountID: accountID,
			TenantID:  &tid,
			Override:  identityaccess.MFAAdminOverrideForceOff,
			SetByType: "operator",
		},
		"override must be force_off or force_on": {
			AccountID: accountID,
			Override:  "garbage",
			SetByType: "operator",
		},
		"set_by_type must be operator": {
			AccountID: accountID,
			Override:  identityaccess.MFAAdminOverrideForceOff,
			SetByType: "account",
		},
	}
	for name, row := range cases {
		t.Run(name, func(t *testing.T) {
			require.Error(t, repo.UpsertGlobalOverride(ctx, row))
		})
	}
}

// --- UpsertTenant -----------------------------------------------------

func TestNativeMFAOverride_UpsertTenant_HappyPath(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, tenantID := newOverrideRepoTestFixtures(t)

	require.NoError(t, repo.UpsertTenantOverride(ctx, identityaccess.AccountMFAOverride{
		AccountID: accountID,
		TenantID:  &tenantID,
		Override:  identityaccess.MFAAdminOverrideForceOn,
		SetBy:     5,
		SetByType: "account",
		Reason:    "tenant policy",
	}))

	got, found, err := repo.FindTenantOverride(ctx, accountID, tenantID)
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, got.TenantID)
	assert.Equal(t, tenantID, *got.TenantID)
	assert.Equal(t, identityaccess.MFAAdminOverrideForceOn, got.Override)
	assert.NotNil(t, got.TenantID)
}

func TestNativeMFAOverride_UpsertTenant_UpdatesExistingRow(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, tenantID := newOverrideRepoTestFixtures(t)

	require.NoError(t, repo.UpsertTenantOverride(ctx, identityaccess.AccountMFAOverride{
		AccountID: accountID, TenantID: &tenantID,
		Override: identityaccess.MFAAdminOverrideForceOn, SetByType: "account",
		Reason: "first",
	}))
	require.NoError(t, repo.UpsertTenantOverride(ctx, identityaccess.AccountMFAOverride{
		AccountID: accountID, TenantID: &tenantID,
		Override: identityaccess.MFAAdminOverrideForceOff, SetByType: "account",
		Reason: "second",
	}))

	got, found, err := repo.FindTenantOverride(ctx, accountID, tenantID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, identityaccess.MFAAdminOverrideForceOff, got.Override)
	assert.Equal(t, "second", got.Reason)
}

func TestNativeMFAOverride_UpsertTenant_ValidationBranches(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, tenantID := newOverrideRepoTestFixtures(t)

	zero := int64(0)
	tid := tenantID
	cases := map[string]identityaccess.AccountMFAOverride{
		"account_id zero": {
			TenantID:  &tid,
			Override:  identityaccess.MFAAdminOverrideForceOn,
			SetByType: "account",
		},
		"tenant_id nil": {
			AccountID: accountID,
			Override:  identityaccess.MFAAdminOverrideForceOn,
			SetByType: "account",
		},
		"tenant_id zero": {
			AccountID: accountID,
			TenantID:  &zero,
			Override:  identityaccess.MFAAdminOverrideForceOn,
			SetByType: "account",
		},
		"invalid override": {
			AccountID: accountID,
			TenantID:  &tid,
			Override:  "force_maybe",
			SetByType: "account",
		},
	}
	for name, row := range cases {
		t.Run(name, func(t *testing.T) {
			require.Error(t, repo.UpsertTenantOverride(ctx, row))
		})
	}
}

// --- FindGlobal / FindByAccountAndTenant ------------------------------

func TestNativeMFAOverride_FindGlobal_ReturnsNilWhenAbsent(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, _ := newOverrideRepoTestFixtures(t)

	_, found, err := repo.FindGlobalOverride(ctx, accountID)
	require.NoError(t, err)
	assert.False(t, found, "no row should map to (nil, nil), not an error")
}

func TestNativeMFAOverride_FindByAccountAndTenant_ReturnsNilWhenAbsent(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, tenantID := newOverrideRepoTestFixtures(t)

	_, found, err := repo.FindTenantOverride(ctx, accountID, tenantID)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestNativeMFAOverride_FindByAccountAndTenant_TenantIDZeroIsBlocked(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, _ := newOverrideRepoTestFixtures(t)

	// Write a global row so a buggy implementation would surface it
	// through the zero-tenant path. Repository must short-circuit
	// before the query so the global row stays invisible here.
	require.NoError(t, repo.UpsertGlobalOverride(ctx, identityaccess.AccountMFAOverride{
		AccountID: accountID,
		Override:  identityaccess.MFAAdminOverrideForceOff,
		SetByType: "operator",
	}))

	_, found, err := repo.FindTenantOverride(ctx, accountID, 0)
	require.NoError(t, err)
	assert.False(t, found, "tenant_id=0 sentinel must not leak the global row")
}

// --- DeleteGlobal / DeleteTenant --------------------------------------

func TestNativeMFAOverride_DeleteGlobal_RemovesRow(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, _ := newOverrideRepoTestFixtures(t)

	require.NoError(t, repo.UpsertGlobalOverride(ctx, identityaccess.AccountMFAOverride{
		AccountID: accountID,
		Override:  identityaccess.MFAAdminOverrideForceOff,
		SetByType: "operator",
	}))
	require.NoError(t, repo.DeleteGlobalOverride(ctx, accountID))

	_, found, err := repo.FindGlobalOverride(ctx, accountID)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestNativeMFAOverride_DeleteGlobal_NoopWhenAbsent(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, _ := newOverrideRepoTestFixtures(t)

	// Idempotent: delete-without-row is not an error.
	require.NoError(t, repo.DeleteGlobalOverride(ctx, accountID))
}

func TestNativeMFAOverride_DeleteTenant_RemovesRow(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, tenantID := newOverrideRepoTestFixtures(t)

	require.NoError(t, repo.UpsertTenantOverride(ctx, identityaccess.AccountMFAOverride{
		AccountID: accountID, TenantID: &tenantID,
		Override: identityaccess.MFAAdminOverrideForceOn, SetByType: "account",
	}))
	require.NoError(t, repo.DeleteTenantOverride(ctx, accountID, tenantID))

	_, found, err := repo.FindTenantOverride(ctx, accountID, tenantID)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestNativeMFAOverride_DeleteTenant_TenantIDZeroRejected(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, _ := newOverrideRepoTestFixtures(t)

	require.Error(t, repo.DeleteTenantOverride(ctx, accountID, 0))
}
