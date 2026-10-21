package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDropEnrollmentCaptchaKeySettingsKeepsOtherSettings(t *testing.T) {
	t.Parallel()
	settingsCleanupMigrationTests.Lock()
	defer settingsCleanupMigrationTests.Unlock()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	school, _ := testpkg.CreateTestTenant(t, db)
	settingsCleanupStore(t, db, school, "enrollment.captcha_site_key", `"site"`)
	settingsCleanupStore(t, db, school, "enrollment.captcha_secret_key", `"secret"`)
	settingsCleanupStore(t, db, school, "enrollment.require_captcha", `true`)

	require.NoError(t, dropEnrollmentCaptchaKeySettingsUp(ctx, db))
	require.NoError(t, dropEnrollmentCaptchaKeySettingsUp(ctx, db), "a second run changes nothing")

	for _, key := range dropEnrollmentCaptchaKeySettingsKeys {
		_, ok := settingsCleanupValue(t, db, school, key)
		assert.False(t, ok, key)
		var audits int
		require.NoError(t, db.NewRaw(`
			SELECT COUNT(*) FROM config.setting_audit WHERE tenant_id = ? AND setting_key = ?
		`, school, key).Scan(ctx, &audits))
		assert.Zero(t, audits, key)
	}
	value, ok := settingsCleanupValue(t, db, school, "enrollment.require_captcha")
	require.True(t, ok, "the school's captcha switch stays")
	assert.Equal(t, `true`, value)
}
