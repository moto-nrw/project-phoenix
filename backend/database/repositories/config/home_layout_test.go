package config

import (
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func homeLayoutAccount(tb testing.TB) int64 {
	tb.Helper()
	db := testpkg.SetupTestDB(tb)
	email := fmt.Sprintf("home-layout-%d@example.com", testpkg.UniqueSuffix())
	return testpkg.CreateTestAccount(tb, db, email).ID
}

func TestHomeLayoutRepository_FindByAccount_NoRow(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewHomeLayoutRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	// Never having customized is the normal state and must not read as an
	// error: the start page falls back to what the role recommends.
	found, err := repo.FindByAccount(ctx, homeLayoutAccount(t))
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestHomeLayoutRepository_UpsertForAccount_InsertThenReplace(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewHomeLayoutRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)
	accountID := homeLayoutAccount(t)

	require.NoError(t, repo.UpsertForAccount(ctx, &config.HomeLayout{
		TenantID:  testpkg.Tenant(t),
		AccountID: accountID,
		Overrides: map[string]bool{"tile.students_sick": false, "section.birthdays": true},
	}))

	found, err := repo.FindByAccount(ctx, accountID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, map[string]bool{"tile.students_sick": false, "section.birthdays": true}, found.Overrides)

	// The map is always written whole, so a second write replaces it rather
	// than merging — otherwise a block a person just re-enabled could never
	// lose its stored entry again.
	require.NoError(t, repo.UpsertForAccount(ctx, &config.HomeLayout{
		TenantID:  testpkg.Tenant(t),
		AccountID: accountID,
		Overrides: map[string]bool{"tile.students_home": true},
	}))

	found, err = repo.FindByAccount(ctx, accountID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, map[string]bool{"tile.students_home": true}, found.Overrides)

	t.Cleanup(func() { _ = repo.DeleteForAccount(ctx, accountID) })
}

func TestHomeLayoutRepository_UpsertForAccount_RejectsUnknownKeyShape(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewHomeLayoutRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	err := repo.UpsertForAccount(ctx, &config.HomeLayout{
		TenantID:  testpkg.Tenant(t),
		AccountID: homeLayoutAccount(t),
		Overrides: map[string]bool{"DROP TABLE": true},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid start page block key")
}

func TestHomeLayoutRepository_DeleteForAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewHomeLayoutRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)
	accountID := homeLayoutAccount(t)

	require.NoError(t, repo.UpsertForAccount(ctx, &config.HomeLayout{
		TenantID:  testpkg.Tenant(t),
		AccountID: accountID,
		Overrides: map[string]bool{"section.birthdays": false},
	}))
	require.NoError(t, repo.DeleteForAccount(ctx, accountID))

	found, err := repo.FindByAccount(ctx, accountID)
	require.NoError(t, err)
	assert.Nil(t, found, "resetting drops the row so the recommended start page applies again")

	// Deleting again is not an error: an account without a row already sees
	// the recommendation, which is what the caller asked for.
	require.NoError(t, repo.DeleteForAccount(ctx, accountID))
}

func TestHomeLayoutRepository_SeparateAccountsKeepSeparateLayouts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewHomeLayoutRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)
	first := homeLayoutAccount(t)
	second := homeLayoutAccount(t)

	require.NoError(t, repo.UpsertForAccount(ctx, &config.HomeLayout{
		TenantID: testpkg.Tenant(t), AccountID: first,
		Overrides: map[string]bool{"section.birthdays": false},
	}))
	require.NoError(t, repo.UpsertForAccount(ctx, &config.HomeLayout{
		TenantID: testpkg.Tenant(t), AccountID: second,
		Overrides: map[string]bool{"section.birthdays": true},
	}))

	firstLayout, err := repo.FindByAccount(ctx, first)
	require.NoError(t, err)
	require.NotNil(t, firstLayout)
	assert.Equal(t, map[string]bool{"section.birthdays": false}, firstLayout.Overrides)

	secondLayout, err := repo.FindByAccount(ctx, second)
	require.NoError(t, err)
	require.NotNil(t, secondLayout)
	assert.Equal(t, map[string]bool{"section.birthdays": true}, secondLayout.Overrides)

	t.Cleanup(func() {
		_ = repo.DeleteForAccount(ctx, first)
		_ = repo.DeleteForAccount(ctx, second)
	})
}

func TestHomeLayoutRepository_Policies_UpsertAndReplace(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewHomeLayoutRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	policies, err := repo.FindPolicies(ctx)
	require.NoError(t, err)
	assert.Nil(t, policies, "a school that never prescribed anything has no row")

	require.NoError(t, repo.UpsertPolicies(ctx, &config.HomeBlockPolicySet{
		TenantID: testpkg.Tenant(t),
		Policies: map[string]config.BlockPolicy{
			"section.birthdays":  config.BlockDisabled,
			"tile.students_sick": config.BlockRequired,
		},
	}))

	policies, err = repo.FindPolicies(ctx)
	require.NoError(t, err)
	require.NotNil(t, policies)
	assert.Equal(t, config.BlockDisabled, policies.Policies["section.birthdays"])
	assert.Equal(t, config.BlockRequired, policies.Policies["tile.students_sick"])

	require.NoError(t, repo.UpsertPolicies(ctx, &config.HomeBlockPolicySet{
		TenantID: testpkg.Tenant(t),
		Policies: map[string]config.BlockPolicy{"section.birthdays": config.BlockRequired},
	}))

	policies, err = repo.FindPolicies(ctx)
	require.NoError(t, err)
	require.NotNil(t, policies)
	assert.Equal(t, map[string]config.BlockPolicy{"section.birthdays": config.BlockRequired}, policies.Policies,
		"the prescription is replaced whole, so a released block leaves no trace")
}

func TestHomeLayoutRepository_Policies_RejectUnknownPolicy(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewHomeLayoutRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	err := repo.UpsertPolicies(ctx, &config.HomeBlockPolicySet{
		TenantID: testpkg.Tenant(t),
		Policies: map[string]config.BlockPolicy{"section.birthdays": "mandatory"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown block policy")
}
