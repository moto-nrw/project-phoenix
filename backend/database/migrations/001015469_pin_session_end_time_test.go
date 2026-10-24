package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPinSessionEndTimeKeepsDeployedTimeAndOwnChoices(t *testing.T) {
	t.Parallel()
	settingsCleanupMigrationTests.Lock()
	defer settingsCleanupMigrationTests.Unlock()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	onDefault, _ := testpkg.CreateTestTenant(t, db)
	ownChoice, _ := testpkg.CreateTestTenant(t, db)
	settingsCleanupStore(t, db, ownChoice, pinSessionEndTimeKey, `"16:15"`)

	require.NoError(t, pinSessionEndTimeUp(ctx, db))
	require.NoError(t, pinSessionEndTimeUp(ctx, db), "a second run changes nothing")

	value, ok := settingsCleanupValue(t, db, onDefault, pinSessionEndTimeKey)
	require.True(t, ok, "a school without its own time gets the deployed time pinned")
	assert.Equal(t, `"17:00"`, value)
	value, _ = settingsCleanupValue(t, db, ownChoice, pinSessionEndTimeKey)
	assert.Equal(t, `"16:15"`, value, "an own choice stays")

	var audits int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM config.setting_audit
		WHERE tenant_id = ? AND setting_key = ? AND new_value = '"17:00"'::jsonb
	`, onDefault, pinSessionEndTimeKey).Scan(ctx, &audits))
	assert.Equal(t, 1, audits, "the pinned value is audited once")
}
