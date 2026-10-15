package compose_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The settings CLI lists stored overrides only: a key the school stored comes
// back as its JSON value, a key without a row is absent rather than its
// registry default.
func TestStoredOverrideReaderReportsOnlyStoredRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.SetTestTenantSetting(t, db, tenantID, settings.KeyGroupMode, `"open_care"`)
	reader := settingsCompose.NewStoredOverrideReader(db)

	value, found, err := reader.StoredOverride(context.Background(), tenantID, settings.KeyGroupMode)
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `"open_care"`, string(value))

	value, found, err = reader.StoredOverride(context.Background(), tenantID, settings.KeyAttendanceWebEnabled)
	require.NoError(t, err)
	assert.False(t, found, "a key without a stored row is not reported")
	assert.Nil(t, value)
}

// A read failure surfaces instead of reading as "no override".
func TestStoredOverrideReaderReportsReadFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	require.NoError(t, db.Close())

	_, found, err := settingsCompose.NewStoredOverrideReader(db).StoredOverride(context.Background(), testpkg.Tenant(t), settings.KeyGroupMode)
	require.Error(t, err)
	assert.False(t, found)
}
