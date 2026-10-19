package pwa_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/pwa"
	api "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPWAUsageRetentionKeyMatchesTheRegistry(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "gdpr.pwa_usage_retention_days", pwa.RestatedRetentionSettingKey)
	db := testpkg.SetupTestDB(t)
	module, err := api.NewSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	_, err = testpkg.TenantQuery1(t, testpkg.Ctx(t), db, module.Settings.ResolveInt, pwa.RestatedRetentionSettingKey)
	require.NoError(t, err, "PWA retention must use a registered integer setting")
}
