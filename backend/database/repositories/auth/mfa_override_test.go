package auth_test

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

	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	"github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func newOverrideRepoTestFixtures(t *testing.T) (
	ctx context.Context,
	repo auth.MFAOverrideRepository,
	accountID, tenantID int64,
	cleanup func(),
) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	ctx = context.Background()

	account := testpkg.CreateTestAccount(t, db, "mfa-override")
	tenantID = testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	repo = authRepo.NewMFAOverrideRepository(db)

	cleanup = func() {
		// DeleteAllByAccount drops every override row this test wrote,
		// regardless of which test path created them — keeps the fixture
		// teardown branch-independent.
		if _, err := repo.DeleteAllByAccount(ctx, account.ID); err != nil {
			t.Logf("teardown: delete all overrides: %v", err)
		}
		testpkg.CleanupTenantTestData(t, db, tenantID)
		testpkg.CleanupAccount(t, db, account.ID)
	}
	return ctx, repo, account.ID, tenantID, cleanup
}

// --- UpsertGlobal -----------------------------------------------------

func TestMFAOverrideRepository_UpsertGlobal_HappyPath(t *testing.T) {
	ctx, repo, accountID, _, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	require.NoError(t, repo.UpsertGlobal(ctx, &auth.MFAOverride{
		AccountID: accountID,
		Override:  auth.MFAAdminOverrideForceOff,
		SetBy:     42,
		SetByType: auth.MFAOverrideSetByTypeOperator,
		Reason:    "user has no email access",
	}))

	got, err := repo.FindGlobal(ctx, accountID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Nil(t, got.TenantID, "global row must have tenant_id NULL")
	assert.Equal(t, auth.MFAAdminOverrideForceOff, got.Override)
	assert.True(t, got.IsGlobal())
}

func TestMFAOverrideRepository_UpsertGlobal_UpdatesExistingRow(t *testing.T) {
	ctx, repo, accountID, _, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	require.NoError(t, repo.UpsertGlobal(ctx, &auth.MFAOverride{
		AccountID: accountID,
		Override:  auth.MFAAdminOverrideForceOff,
		SetBy:     42,
		SetByType: auth.MFAOverrideSetByTypeOperator,
		Reason:    "initial",
	}))
	require.NoError(t, repo.UpsertGlobal(ctx, &auth.MFAOverride{
		AccountID: accountID,
		Override:  auth.MFAAdminOverrideForceOn,
		SetBy:     77,
		SetByType: auth.MFAOverrideSetByTypeOperator,
		Reason:    "policy reversal",
	}))

	rows, err := repo.ListByAccount(ctx, accountID)
	require.NoError(t, err)
	require.Len(t, rows, 1, "ON CONFLICT must replace, not append")
	assert.Equal(t, auth.MFAAdminOverrideForceOn, rows[0].Override)
	assert.EqualValues(t, 77, rows[0].SetBy)
	assert.Equal(t, "policy reversal", rows[0].Reason)
}

func TestMFAOverrideRepository_UpsertGlobal_NilRejected(t *testing.T) {
	ctx, repo, _, _, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	err := repo.UpsertGlobal(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestMFAOverrideRepository_UpsertGlobal_ValidationBranches(t *testing.T) {
	ctx, repo, accountID, tenantID, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	tid := tenantID
	cases := map[string]*auth.MFAOverride{
		"account_id zero": {
			Override:  auth.MFAAdminOverrideForceOff,
			SetByType: auth.MFAOverrideSetByTypeOperator,
		},
		"tenant_id must be nil for global": {
			AccountID: accountID,
			TenantID:  &tid,
			Override:  auth.MFAAdminOverrideForceOff,
			SetByType: auth.MFAOverrideSetByTypeOperator,
		},
		"override must be force_off or force_on": {
			AccountID: accountID,
			Override:  "garbage",
			SetByType: auth.MFAOverrideSetByTypeOperator,
		},
		"set_by_type must be operator": {
			AccountID: accountID,
			Override:  auth.MFAAdminOverrideForceOff,
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
	ctx, repo, accountID, tenantID, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	require.NoError(t, repo.UpsertTenant(ctx, &auth.MFAOverride{
		AccountID: accountID,
		TenantID:  &tenantID,
		Override:  auth.MFAAdminOverrideForceOn,
		SetBy:     5,
		SetByType: "account",
		Reason:    "tenant policy",
	}))

	got, err := repo.FindByAccountAndTenant(ctx, accountID, tenantID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.TenantID)
	assert.Equal(t, tenantID, *got.TenantID)
	assert.Equal(t, auth.MFAAdminOverrideForceOn, got.Override)
	assert.False(t, got.IsGlobal())
}

func TestMFAOverrideRepository_UpsertTenant_UpdatesExistingRow(t *testing.T) {
	ctx, repo, accountID, tenantID, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	require.NoError(t, repo.UpsertTenant(ctx, &auth.MFAOverride{
		AccountID: accountID, TenantID: &tenantID,
		Override: auth.MFAAdminOverrideForceOn, SetByType: "account",
		Reason: "first",
	}))
	require.NoError(t, repo.UpsertTenant(ctx, &auth.MFAOverride{
		AccountID: accountID, TenantID: &tenantID,
		Override: auth.MFAAdminOverrideForceOff, SetByType: "account",
		Reason: "second",
	}))

	got, err := repo.FindByAccountAndTenant(ctx, accountID, tenantID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, auth.MFAAdminOverrideForceOff, got.Override)
	assert.Equal(t, "second", got.Reason)
}

func TestMFAOverrideRepository_UpsertTenant_NilRejected(t *testing.T) {
	ctx, repo, _, _, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	require.Error(t, repo.UpsertTenant(ctx, nil))
}

func TestMFAOverrideRepository_UpsertTenant_ValidationBranches(t *testing.T) {
	ctx, repo, accountID, tenantID, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	zero := int64(0)
	tid := tenantID
	cases := map[string]*auth.MFAOverride{
		"account_id zero": {
			TenantID:  &tid,
			Override:  auth.MFAAdminOverrideForceOn,
			SetByType: "account",
		},
		"tenant_id nil": {
			AccountID: accountID,
			Override:  auth.MFAAdminOverrideForceOn,
			SetByType: "account",
		},
		"tenant_id zero": {
			AccountID: accountID,
			TenantID:  &zero,
			Override:  auth.MFAAdminOverrideForceOn,
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
	ctx, repo, accountID, _, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	got, err := repo.FindGlobal(ctx, accountID)
	require.NoError(t, err)
	assert.Nil(t, got, "no row should map to (nil, nil), not an error")
}

func TestMFAOverrideRepository_FindByAccountAndTenant_ReturnsNilWhenAbsent(t *testing.T) {
	ctx, repo, accountID, tenantID, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	got, err := repo.FindByAccountAndTenant(ctx, accountID, tenantID)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMFAOverrideRepository_FindByAccountAndTenant_TenantIDZeroIsBlocked(t *testing.T) {
	ctx, repo, accountID, _, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	// Write a global row so a buggy implementation would surface it
	// through the zero-tenant path. Repository must short-circuit
	// before the query so the global row stays invisible here.
	require.NoError(t, repo.UpsertGlobal(ctx, &auth.MFAOverride{
		AccountID: accountID,
		Override:  auth.MFAAdminOverrideForceOff,
		SetByType: auth.MFAOverrideSetByTypeOperator,
	}))

	got, err := repo.FindByAccountAndTenant(ctx, accountID, 0)
	require.NoError(t, err)
	assert.Nil(t, got, "tenant_id=0 sentinel must not leak the global row")
}

// --- DeleteGlobal / DeleteTenant --------------------------------------

func TestMFAOverrideRepository_DeleteGlobal_RemovesRow(t *testing.T) {
	ctx, repo, accountID, _, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	require.NoError(t, repo.UpsertGlobal(ctx, &auth.MFAOverride{
		AccountID: accountID,
		Override:  auth.MFAAdminOverrideForceOff,
		SetByType: auth.MFAOverrideSetByTypeOperator,
	}))
	require.NoError(t, repo.DeleteGlobal(ctx, accountID))

	got, err := repo.FindGlobal(ctx, accountID)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMFAOverrideRepository_DeleteGlobal_NoopWhenAbsent(t *testing.T) {
	ctx, repo, accountID, _, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	// Idempotent: delete-without-row is not an error.
	require.NoError(t, repo.DeleteGlobal(ctx, accountID))
}

func TestMFAOverrideRepository_DeleteTenant_RemovesRow(t *testing.T) {
	ctx, repo, accountID, tenantID, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	require.NoError(t, repo.UpsertTenant(ctx, &auth.MFAOverride{
		AccountID: accountID, TenantID: &tenantID,
		Override: auth.MFAAdminOverrideForceOn, SetByType: "account",
	}))
	require.NoError(t, repo.DeleteTenant(ctx, accountID, tenantID))

	got, err := repo.FindByAccountAndTenant(ctx, accountID, tenantID)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMFAOverrideRepository_DeleteTenant_TenantIDZeroRejected(t *testing.T) {
	ctx, repo, accountID, _, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	require.Error(t, repo.DeleteTenant(ctx, accountID, 0))
}

// --- ListByAccount / DeleteAllByAccount -------------------------------

func TestMFAOverrideRepository_ListByAccount_GlobalRowComesFirst(t *testing.T) {
	ctx, repo, accountID, tenantID, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	require.NoError(t, repo.UpsertTenant(ctx, &auth.MFAOverride{
		AccountID: accountID, TenantID: &tenantID,
		Override: auth.MFAAdminOverrideForceOn, SetByType: "account",
	}))
	require.NoError(t, repo.UpsertGlobal(ctx, &auth.MFAOverride{
		AccountID: accountID,
		Override:  auth.MFAAdminOverrideForceOff,
		SetByType: auth.MFAOverrideSetByTypeOperator,
	}))

	rows, err := repo.ListByAccount(ctx, accountID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.True(t, rows[0].IsGlobal(), "ORDER BY tenant_id NULLS FIRST → global row leads")
	require.NotNil(t, rows[1].TenantID)
	assert.Equal(t, tenantID, *rows[1].TenantID)
}

func TestMFAOverrideRepository_ListByAccount_EmptyWhenNone(t *testing.T) {
	ctx, repo, accountID, _, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	rows, err := repo.ListByAccount(ctx, accountID)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestMFAOverrideRepository_DeleteAllByAccount_ReturnsCount(t *testing.T) {
	ctx, repo, accountID, tenantID, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	require.NoError(t, repo.UpsertGlobal(ctx, &auth.MFAOverride{
		AccountID: accountID,
		Override:  auth.MFAAdminOverrideForceOff,
		SetByType: auth.MFAOverrideSetByTypeOperator,
	}))
	require.NoError(t, repo.UpsertTenant(ctx, &auth.MFAOverride{
		AccountID: accountID, TenantID: &tenantID,
		Override: auth.MFAAdminOverrideForceOn, SetByType: "account",
	}))

	n, err := repo.DeleteAllByAccount(ctx, accountID)
	require.NoError(t, err)
	assert.EqualValues(t, 2, n)

	rows, err := repo.ListByAccount(ctx, accountID)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestMFAOverrideRepository_DeleteAllByAccount_ReturnsZeroWhenNothingToDelete(t *testing.T) {
	ctx, repo, accountID, _, cleanup := newOverrideRepoTestFixtures(t)
	defer cleanup()

	n, err := repo.DeleteAllByAccount(ctx, accountID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
}
