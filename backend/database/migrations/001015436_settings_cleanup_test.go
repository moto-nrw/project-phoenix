package migrations

import (
	"context"
	"sync"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Every test runs the whole migration, which scans every school of the
// package's database. Serialized so one test's delete step cannot remove a
// removed-key fixture another test planted before its own carry step ran.
var settingsCleanupMigrationTests sync.Mutex

func settingsCleanupValue(t *testing.T, db bun.IDB, tenantID int64, key string) (string, bool) {
	t.Helper()
	var values []string
	require.NoError(t, db.NewRaw(`
		SELECT value::text
		FROM config.setting_values
		WHERE tenant_id = ?
			AND setting_key = ?
	`, tenantID, key).Scan(context.Background(), &values))
	if len(values) == 0 {
		return "", false
	}
	return values[0], true
}

func settingsCleanupStore(t *testing.T, db bun.IDB, tenantID int64, key, jsonValue string) {
	t.Helper()
	_, err := db.NewRaw(`
		INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, ?, ?::jsonb)
	`, tenantID, key, jsonValue).Exec(context.Background())
	require.NoError(t, err)
	_, err = db.NewRaw(`
		INSERT INTO config.setting_audit (tenant_id, setting_key, old_value, new_value, action, changed_by)
		VALUES (?, ?, NULL, ?::jsonb, 'set', NULL)
	`, tenantID, key, jsonValue).Exec(context.Background())
	require.NoError(t, err)
}

func TestSettingsCleanupCarriesCareConceptIntoSpontaneousActivities(t *testing.T) {
	t.Parallel()
	settingsCleanupMigrationTests.Lock()
	defer settingsCleanupMigrationTests.Unlock()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	fixed, _ := testpkg.CreateTestTenant(t, db)
	fixedAlreadyOff, _ := testpkg.CreateTestTenant(t, db)
	openRooms, _ := testpkg.CreateTestTenant(t, db)
	openRoomsOff, _ := testpkg.CreateTestTenant(t, db)
	untouched, _ := testpkg.CreateTestTenant(t, db)

	settingsCleanupStore(t, db, fixed, settingsCleanupCareConceptKey, `"fixed_schedule"`)
	settingsCleanupStore(t, db, fixed, settingsCleanupSpontaneousKey, `true`)
	settingsCleanupStore(t, db, fixedAlreadyOff, settingsCleanupCareConceptKey, `"fixed_schedule"`)
	settingsCleanupStore(t, db, fixedAlreadyOff, settingsCleanupSpontaneousKey, `false`)
	settingsCleanupStore(t, db, openRooms, settingsCleanupCareConceptKey, `"open_rooms"`)
	settingsCleanupStore(t, db, openRoomsOff, settingsCleanupCareConceptKey, `"open_rooms"`)
	settingsCleanupStore(t, db, openRoomsOff, settingsCleanupSpontaneousKey, `false`)

	require.NoError(t, settingsCleanupUp(ctx, db))

	value, ok := settingsCleanupValue(t, db, fixed, settingsCleanupSpontaneousKey)
	require.True(t, ok)
	assert.Equal(t, "false", value, "a fixed schedule never allowed spontaneous activities")
	value, _ = settingsCleanupValue(t, db, fixedAlreadyOff, settingsCleanupSpontaneousKey)
	assert.Equal(t, "false", value)
	_, ok = settingsCleanupValue(t, db, openRooms, settingsCleanupSpontaneousKey)
	assert.False(t, ok, "open rooms without an own value keep the default (on)")
	value, _ = settingsCleanupValue(t, db, openRoomsOff, settingsCleanupSpontaneousKey)
	assert.Equal(t, "false", value, "a switched-off school stays off")
	_, ok = settingsCleanupValue(t, db, untouched, settingsCleanupSpontaneousKey)
	assert.False(t, ok, "a school on both defaults keeps the default (on)")

	var audits int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM config.setting_audit
		WHERE tenant_id = ? AND setting_key = ? AND new_value = 'false'::jsonb
	`, fixed, settingsCleanupSpontaneousKey).Scan(ctx, &audits))
	assert.Equal(t, 1, audits, "the carried value is audited")
}

func TestSettingsCleanupPinsOnDutyOnlyForExistingSchools(t *testing.T) {
	t.Parallel()
	settingsCleanupMigrationTests.Lock()
	defer settingsCleanupMigrationTests.Unlock()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	onDefault, _ := testpkg.CreateTestTenant(t, db)
	switchedOff, _ := testpkg.CreateTestTenant(t, db)
	settingsCleanupStore(t, db, switchedOff, settingsCleanupOnDutyOnlyKey, `false`)

	require.NoError(t, settingsCleanupUp(ctx, db))

	value, ok := settingsCleanupValue(t, db, onDefault, settingsCleanupOnDutyOnlyKey)
	require.True(t, ok, "a school on the old default gets it pinned")
	assert.Equal(t, "true", value)
	value, _ = settingsCleanupValue(t, db, switchedOff, settingsCleanupOnDutyOnlyKey)
	assert.Equal(t, "false", value, "an own choice stays")
}

func TestSettingsCleanupPinsEmptyIndicatorSlotsForSchoolsUsingThem(t *testing.T) {
	t.Parallel()
	settingsCleanupMigrationTests.Lock()
	defer settingsCleanupMigrationTests.Unlock()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	using, _ := testpkg.CreateTestTenant(t, db)
	settingsCleanupStore(t, db, using, settingsCleanupIndicatorsKey, `true`)
	settingsCleanupStore(t, db, using, settingsCleanupIndicator1Key, `"Hausaufgaben"`)
	notUsing, _ := testpkg.CreateTestTenant(t, db)
	settingsCleanupStore(t, db, notUsing, settingsCleanupIndicatorsKey, `false`)
	neverTouched, _ := testpkg.CreateTestTenant(t, db)

	require.NoError(t, settingsCleanupUp(ctx, db))

	value, _ := settingsCleanupValue(t, db, using, settingsCleanupIndicator1Key)
	assert.Equal(t, `"Hausaufgaben"`, value, "an own label stays")
	value, ok := settingsCleanupValue(t, db, using, settingsCleanupIndicator2Key)
	require.True(t, ok, "an empty slot of a school showing indicators is pinned")
	assert.Equal(t, `""`, value)
	for _, tenantID := range []int64{notUsing, neverTouched} {
		for _, key := range []string{settingsCleanupIndicator1Key, settingsCleanupIndicator2Key} {
			_, ok := settingsCleanupValue(t, db, tenantID, key)
			assert.False(t, ok, "tenant %d gets the new default for %s when switching on", tenantID, key)
		}
	}
}

func TestSettingsCleanupDeletesRemovedSettings(t *testing.T) {
	t.Parallel()
	settingsCleanupMigrationTests.Lock()
	defer settingsCleanupMigrationTests.Unlock()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID, _ := testpkg.CreateTestTenant(t, db)
	for _, key := range settingsCleanupRemovedKeys {
		settingsCleanupStore(t, db, tenantID, key, `"x"`)
	}

	require.NoError(t, settingsCleanupUp(ctx, db))

	var remaining int
	require.NoError(t, db.NewRaw(`
		SELECT (SELECT COUNT(*) FROM config.setting_values WHERE tenant_id = ? AND setting_key IN (?))
			+ (SELECT COUNT(*) FROM config.setting_audit WHERE tenant_id = ? AND setting_key IN (?))
	`, tenantID, bun.List(settingsCleanupRemovedKeys), tenantID, bun.List(settingsCleanupRemovedKeys)).Scan(ctx, &remaining))
	assert.Zero(t, remaining)
}

func TestSettingsCleanupRollbackIsNoop(t *testing.T) {
	t.Parallel()
	require.NoError(t, settingsCleanupDown(context.Background(), nil))
}
