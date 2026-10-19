package settings_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/settings"
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
