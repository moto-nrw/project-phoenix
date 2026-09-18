package authpostgres_test

// Tests for MFAOverrideRepository (auth.mfa_overrides).
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

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authmodels"
	authRepo "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authpostgres"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func newOverrideRepoTestFixtures(t *testing.T) (
	ctx context.Context,
	repo authmodels.MFAOverrideRepository,
	accountID, tenantID int64,
) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	ctx = context.Background()

	account := testpkg.CreateTestAccount(t, db, "mfa-override")
	tenantID = testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	repo = authRepo.NewMFAOverrideRepository(db)

	t.Cleanup(func() {
		rows, err := repo.ListByAccount(ctx, account.ID)
		require.NoError(t, err)
		for _, row := range rows {
			if row.IsGlobal() {
				err = repo.DeleteGlobal(ctx, account.ID)
			} else {
				err = repo.DeleteTenant(ctx, account.ID, *row.TenantID)
			}
			require.NoError(t, err)
		}
	})
	return ctx, repo, account.ID, tenantID
}

// --- UpsertGlobal -----------------------------------------------------

func TestMFAOverrideRepository_UpsertGlobal_HappyPath(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, _ := newOverrideRepoTestFixtures(t)

	require.NoError(t, repo.UpsertGlobal(ctx, &authmodels.MFAOverride{
		AccountID: accountID,
		Override:  authmodels.MFAAdminOverrideForceOff,
		SetBy:     42,
		SetByType: authmodels.MFAOverrideSetByTypeOperator,
		Reason:    "user has no email access",
	}))

	got, err := repo.FindGlobal(ctx, accountID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Nil(t, got.TenantID, "global row must have tenant_id NULL")
	assert.Equal(t, authmodels.MFAAdminOverrideForceOff, got.Override)
	assert.True(t, got.IsGlobal())
}

func TestMFAOverrideRepository_UpsertGlobal_UpdatesExistingRow(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, _ := newOverrideRepoTestFixtures(t)

	require.NoError(t, repo.UpsertGlobal(ctx, &authmodels.MFAOverride{
		AccountID: accountID,
		Override:  authmodels.MFAAdminOverrideForceOff,
		SetBy:     42,
		SetByType: authmodels.MFAOverrideSetByTypeOperator,
		Reason:    "initial",
	}))
	require.NoError(t, repo.UpsertGlobal(ctx, &authmodels.MFAOverride{
		AccountID: accountID,
		Override:  authmodels.MFAAdminOverrideForceOn,
		SetBy:     77,
		SetByType: authmodels.MFAOverrideSetByTypeOperator,
		Reason:    "policy reversal",
	}))

	rows, err := repo.ListByAccount(ctx, accountID)
	require.NoError(t, err)
	require.Len(t, rows, 1, "ON CONFLICT must replace, not append")
	assert.Equal(t, authmodels.MFAAdminOverrideForceOn, rows[0].Override)
	assert.EqualValues(t, 77, rows[0].SetBy)
	assert.Equal(t, "policy reversal", rows[0].Reason)
}

func TestMFAOverrideRepository_UpsertGlobal_NilRejected(t *testing.T) {
	t.Parallel()

	ctx, repo, _, _ := newOverrideRepoTestFixtures(t)

	err := repo.UpsertGlobal(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestMFAOverrideRepository_UpsertGlobal_ValidationBranches(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, tenantID := newOverrideRepoTestFixtures(t)

	tid := tenantID
	cases := map[string]*authmodels.MFAOverride{
		"account_id zero": {
			Override:  authmodels.MFAAdminOverrideForceOff,
			SetByType: authmodels.MFAOverrideSetByTypeOperator,
		},
		"tenant_id must be nil for global": {
			AccountID: accountID,
			TenantID:  &tid,
			Override:  authmodels.MFAAdminOverrideForceOff,
			SetByType: authmodels.MFAOverrideSetByTypeOperator,
		},
		"override must be force_off or force_on": {
			AccountID: accountID,
			Override:  "garbage",
			SetByType: authmodels.MFAOverrideSetByTypeOperator,
		},
		"set_by_type must be operator": {
			AccountID: accountID,
			Override:  authmodels.MFAAdminOverrideForceOff,
			SetByType: "account",
		},
	}
	for name, row := range cases {
		t.Run(name, func(t *testing.T) {
			require.Error(t, repo.UpsertGlobal(ctx, row))
		})
	}
}

// --- UpsertTenant -----------------------------------------------------

func TestMFAOverrideRepository_UpsertTenant_HappyPath(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, tenantID := newOverrideRepoTestFixtures(t)

	require.NoError(t, repo.UpsertTenant(ctx, &authmodels.MFAOverride{
		AccountID: accountID,
		TenantID:  &tenantID,
		Override:  authmodels.MFAAdminOverrideForceOn,
		SetBy:     5,
		SetByType: "account",
		Reason:    "tenant policy",
	}))

	got, err := repo.FindByAccountAndTenant(ctx, accountID, tenantID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.TenantID)
	assert.Equal(t, tenantID, *got.TenantID)
	assert.Equal(t, authmodels.MFAAdminOverrideForceOn, got.Override)
	assert.False(t, got.IsGlobal())
}

func TestMFAOverrideRepository_UpsertTenant_UpdatesExistingRow(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, tenantID := newOverrideRepoTestFixtures(t)

	require.NoError(t, repo.UpsertTenant(ctx, &authmodels.MFAOverride{
		AccountID: accountID, TenantID: &tenantID,
		Override: authmodels.MFAAdminOverrideForceOn, SetByType: "account",
		Reason: "first",
	}))
	require.NoError(t, repo.UpsertTenant(ctx, &authmodels.MFAOverride{
		AccountID: accountID, TenantID: &tenantID,
		Override: authmodels.MFAAdminOverrideForceOff, SetByType: "account",
		Reason: "second",
	}))

	got, err := repo.FindByAccountAndTenant(ctx, accountID, tenantID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, authmodels.MFAAdminOverrideForceOff, got.Override)
	assert.Equal(t, "second", got.Reason)
}

func TestMFAOverrideRepository_UpsertTenant_NilRejected(t *testing.T) {
	t.Parallel()

	ctx, repo, _, _ := newOverrideRepoTestFixtures(t)

	require.Error(t, repo.UpsertTenant(ctx, nil))
}

func TestMFAOverrideRepository_UpsertTenant_ValidationBranches(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, tenantID := newOverrideRepoTestFixtures(t)

	zero := int64(0)
	tid := tenantID
	cases := map[string]*authmodels.MFAOverride{
		"account_id zero": {
			TenantID:  &tid,
			Override:  authmodels.MFAAdminOverrideForceOn,
			SetByType: "account",
		},
		"tenant_id nil": {
			AccountID: accountID,
			Override:  authmodels.MFAAdminOverrideForceOn,
			SetByType: "account",
		},
		"tenant_id zero": {
			AccountID: accountID,
			TenantID:  &zero,
			Override:  authmodels.MFAAdminOverrideForceOn,
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
			require.Error(t, repo.UpsertTenant(ctx, row))
		})
	}
}

// --- FindGlobal / FindByAccountAndTenant ------------------------------

func TestMFAOverrideRepository_FindGlobal_ReturnsNilWhenAbsent(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, _ := newOverrideRepoTestFixtures(t)

	got, err := repo.FindGlobal(ctx, accountID)
	require.NoError(t, err)
	assert.Nil(t, got, "no row should map to (nil, nil), not an error")
}

func TestMFAOverrideRepository_FindByAccountAndTenant_ReturnsNilWhenAbsent(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, tenantID := newOverrideRepoTestFixtures(t)

	got, err := repo.FindByAccountAndTenant(ctx, accountID, tenantID)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMFAOverrideRepository_FindByAccountAndTenant_TenantIDZeroIsBlocked(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, _ := newOverrideRepoTestFixtures(t)

	// Write a global row so a buggy implementation would surface it
	// through the zero-tenant path. Repository must short-circuit
	// before the query so the global row stays invisible here.
	require.NoError(t, repo.UpsertGlobal(ctx, &authmodels.MFAOverride{
		AccountID: accountID,
		Override:  authmodels.MFAAdminOverrideForceOff,
		SetByType: authmodels.MFAOverrideSetByTypeOperator,
	}))

	got, err := repo.FindByAccountAndTenant(ctx, accountID, 0)
	require.NoError(t, err)
	assert.Nil(t, got, "tenant_id=0 sentinel must not leak the global row")
}

// --- DeleteGlobal / DeleteTenant --------------------------------------

func TestMFAOverrideRepository_DeleteGlobal_RemovesRow(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, _ := newOverrideRepoTestFixtures(t)

	require.NoError(t, repo.UpsertGlobal(ctx, &authmodels.MFAOverride{
		AccountID: accountID,
		Override:  authmodels.MFAAdminOverrideForceOff,
		SetByType: authmodels.MFAOverrideSetByTypeOperator,
	}))
	require.NoError(t, repo.DeleteGlobal(ctx, accountID))

	got, err := repo.FindGlobal(ctx, accountID)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMFAOverrideRepository_DeleteGlobal_NoopWhenAbsent(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, _ := newOverrideRepoTestFixtures(t)

	// Idempotent: delete-without-row is not an error.
	require.NoError(t, repo.DeleteGlobal(ctx, accountID))
}

func TestMFAOverrideRepository_DeleteTenant_RemovesRow(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, tenantID := newOverrideRepoTestFixtures(t)

	require.NoError(t, repo.UpsertTenant(ctx, &authmodels.MFAOverride{
		AccountID: accountID, TenantID: &tenantID,
		Override: authmodels.MFAAdminOverrideForceOn, SetByType: "account",
	}))
	require.NoError(t, repo.DeleteTenant(ctx, accountID, tenantID))

	got, err := repo.FindByAccountAndTenant(ctx, accountID, tenantID)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMFAOverrideRepository_DeleteTenant_TenantIDZeroRejected(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, _ := newOverrideRepoTestFixtures(t)

	require.Error(t, repo.DeleteTenant(ctx, accountID, 0))
}

// --- ListByAccount ----------------------------------------------------

func TestMFAOverrideRepository_ListByAccount_GlobalRowComesFirst(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, tenantID := newOverrideRepoTestFixtures(t)

	require.NoError(t, repo.UpsertTenant(ctx, &authmodels.MFAOverride{
		AccountID: accountID, TenantID: &tenantID,
		Override: authmodels.MFAAdminOverrideForceOn, SetByType: "account",
	}))
	require.NoError(t, repo.UpsertGlobal(ctx, &authmodels.MFAOverride{
		AccountID: accountID,
		Override:  authmodels.MFAAdminOverrideForceOff,
		SetByType: authmodels.MFAOverrideSetByTypeOperator,
	}))

	rows, err := repo.ListByAccount(ctx, accountID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.True(t, rows[0].IsGlobal(), "ORDER BY tenant_id NULLS FIRST → global row leads")
	require.NotNil(t, rows[1].TenantID)
	assert.Equal(t, tenantID, *rows[1].TenantID)
}

func TestMFAOverrideRepository_ListByAccount_EmptyWhenNone(t *testing.T) {
	t.Parallel()

	ctx, repo, accountID, _ := newOverrideRepoTestFixtures(t)

	rows, err := repo.ListByAccount(ctx, accountID)
	require.NoError(t, err)
	assert.Empty(t, rows)
}
