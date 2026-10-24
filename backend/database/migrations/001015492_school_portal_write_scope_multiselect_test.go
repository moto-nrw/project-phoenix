package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchoolPortalWriteScopeMultiSelectKeepsEachSchoolsChoice(t *testing.T) {
	t.Parallel()
	settingsCleanupMigrationTests.Lock()
	defer settingsCleanupMigrationTests.Unlock()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	readOnly, _ := testpkg.CreateTestTenant(t, db)
	arrivalExceptions, _ := testpkg.CreateTestTenant(t, db)
	onDefault, _ := testpkg.CreateTestTenant(t, db)
	alreadyList, _ := testpkg.CreateTestTenant(t, db)
	settingsCleanupStore(t, db, readOnly, schoolPortalWriteScopeMultiSelectKey, `"none"`)
	settingsCleanupStore(t, db, arrivalExceptions, schoolPortalWriteScopeMultiSelectKey, `"class_arrival_exceptions"`)
	settingsCleanupStore(t, db, alreadyList, schoolPortalWriteScopeMultiSelectKey, `["class_arrival_exceptions", "class_trips"]`)

	require.NoError(t, schoolPortalWriteScopeMultiSelectUp(ctx, db))
	require.NoError(t, schoolPortalWriteScopeMultiSelectUp(ctx, db), "a second run changes nothing")

	value, ok := settingsCleanupValue(t, db, readOnly, schoolPortalWriteScopeMultiSelectKey)
	require.True(t, ok)
	assert.JSONEq(t, `[]`, value, "a read-only school stays read-only")
	value, ok = settingsCleanupValue(t, db, arrivalExceptions, schoolPortalWriteScopeMultiSelectKey)
	require.True(t, ok)
	assert.JSONEq(t, `["class_arrival_exceptions"]`, value, "an opened write stays open")
	value, ok = settingsCleanupValue(t, db, alreadyList, schoolPortalWriteScopeMultiSelectKey)
	require.True(t, ok)
	assert.JSONEq(t, `["class_arrival_exceptions", "class_trips"]`, value, "a list stays as it is")
	_, ok = settingsCleanupValue(t, db, onDefault, schoolPortalWriteScopeMultiSelectKey)
	assert.False(t, ok, "a school on the default gets no row")

	var audits int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM config.setting_audit
		WHERE tenant_id = ? AND setting_key = ? AND new_value = '"class_arrival_exceptions"'::jsonb
	`, arrivalExceptions, schoolPortalWriteScopeMultiSelectKey).Scan(ctx, &audits))
	assert.Equal(t, 1, audits, "the audit trail keeps the value as it was written")

	require.NoError(t, schoolPortalWriteScopeMultiSelectDown(ctx, db))
	value, _ = settingsCleanupValue(t, db, readOnly, schoolPortalWriteScopeMultiSelectKey)
	assert.JSONEq(t, `"none"`, value)
	value, _ = settingsCleanupValue(t, db, arrivalExceptions, schoolPortalWriteScopeMultiSelectKey)
	assert.JSONEq(t, `"class_arrival_exceptions"`, value)
	value, _ = settingsCleanupValue(t, db, alreadyList, schoolPortalWriteScopeMultiSelectKey)
	assert.JSONEq(t, `"class_arrival_exceptions"`, value, "the class arrival exceptions survive the way back")
}
