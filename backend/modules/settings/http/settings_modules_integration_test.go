package settingshttp_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// School admins see their booked modules read-only (#4892, ADR 0048).
func TestSettingsSchemaShowsModuleSwitchesReadOnly(t *testing.T) {
	t.Parallel()
	items := schemaItems(t, setupSettingsModule(t))

	for _, key := range settings.ModuleSettingKeys() {
		item, ok := items[key]
		require.True(t, ok, "the school sees its module switch %s", key)
		assert.Equal(t, false, item["writable"], "%s is read-only for the school", key)
		assert.Equal(t, "operator_managed", item["access_policy"], key)
		assert.Equal(t, true, item["value"], "%s defaults to booked", key)
	}
}

// Only the moto team books modules: the school's own write and reset are
// refused and store nothing (#4892).
func TestSettingsSchoolCannotSwitchModules(t *testing.T) {
	t.Parallel()
	ctx := setupSettingsModule(t)
	router := ctx.resource.SettingsRouter()

	for _, key := range settings.ModuleSettingKeys() {
		put := testutil.NewAuthenticatedRequest(t, "PUT", "/values/"+key, map[string]any{"value": false}, testutil.WithTestTenant(t))
		testutil.AssertErrorResponse(t, testutil.ExecuteRequest(router, put), 403)
		reset := testutil.NewAuthenticatedRequest(t, "DELETE", "/values/"+key, nil, testutil.WithTestTenant(t))
		testutil.AssertErrorResponse(t, testutil.ExecuteRequest(router, reset), 403)
	}

	count, err := ctx.db.NewSelect().Table("config.setting_values").
		Where("tenant_id = ?", testpkg.Tenant(t)).Where("setting_key LIKE 'module.%'").
		Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count, "a refused write must not store a booking")
}
