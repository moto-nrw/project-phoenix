package settings_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/modules/settings/records"
	api "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestPublicSettingsContractDistinguishesOverrideFromRegistryDefault(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	var resolver settings.Resolver = module.Settings
	ctx := testpkg.Ctx(t)
	key := settings.KeyParentNotesEnabled
	fallback, err := testpkg.TenantQuery1(t, ctx, db, resolver.ResolveBool, key)
	require.NoError(t, err)
	overridden, err := testpkg.TenantQuery1(t, ctx, db, resolver.HasTenantOverride, key)
	require.NoError(t, err)
	require.False(t, overridden)
	require.NoError(t, module.Settings.SetValue(ctx, key, !fallback, nil, nil))
	actual, err := testpkg.TenantQuery1(t, ctx, db, resolver.ResolveBool, key)
	require.NoError(t, err)
	require.Equal(t, !fallback, actual)
	overridden, err = testpkg.TenantQuery1(t, ctx, db, resolver.HasTenantOverride, key)
	require.NoError(t, err)
	require.True(t, overridden)
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	var reader settings.TenantReader = module.Settings
	other, err := reader.ResolveBoolForTenant(testpkg.WithPackageTenantRuntime(context.Background()), otherTenant, key)
	require.NoError(t, err)
	require.Equal(t, fallback, other, "an override never becomes another school's default")
}

func TestSettingsHelperUsesRegistryDefaultWithoutOverride(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	key := records.KeyGuardianInvitationTokenExpiryHours
	got, err := testpkg.TenantQuery0(t, ctx, db, func(ctx context.Context) (int, error) {
		return settings.ResolveIntOrDefault(ctx, module.Settings, key, 0, nil), nil
	})
	require.NoError(t, err)
	require.Equal(t, 48, got, "the registry default must win over the caller's obsolete literal")
}
