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
	ctx := testpkg.ContextForTenant(context.Background(), tenantID)
	testpkg.SetTestTenantSetting(t, db, tenantID, settings.KeyGroupMode, `"open_care"`)
	reader := settingsCompose.NewStoredOverrideReader(settingsCompose.NewTenantSettingsRuntime(db, nil))
	_, found, err := reader.StoredOverride(ctx, tenantID, settings.KeyGroupMode)
	require.ErrorContains(t, err, "transaction is required")
	assert.False(t, found)
	err = testpkg.WithinTenantContext(t, ctx, db, tenantID, func(ctx context.Context) error {
		value, found, err := reader.StoredOverride(ctx, tenantID, settings.KeyGroupMode)
		require.NoError(t, err)
		assert.True(t, found)
		assert.JSONEq(t, `"open_care"`, string(value))
		value, found, err = reader.StoredOverride(ctx, tenantID, settings.KeyAttendanceWebEnabled)
		require.NoError(t, err)
		assert.False(t, found, "a key without a stored row is not reported")
		assert.Nil(t, value)
		return nil
	})
	require.NoError(t, err)
}

// An operational read failure is never interpreted as an absent override.
func TestStoredOverrideReaderReportsReadFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	reader := settingsCompose.NewStoredOverrideReader(settingsCompose.NewTenantSettingsRuntime(db, nil))
	err := testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(txCtx context.Context) error {
		ctx, cancel := context.WithCancel(txCtx)
		cancel()
		_, found, err := reader.StoredOverride(ctx, testpkg.Tenant(t), settings.KeyGroupMode)
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		assert.False(t, found)
		return nil
	})
	require.NoError(t, err)
}
