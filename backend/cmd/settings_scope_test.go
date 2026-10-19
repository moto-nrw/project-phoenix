package cmd

import (
	"context"
	"encoding/json"
	"testing"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	settingsRecords "github.com/moto-nrw/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestSettingOverrideCLIProvidesEachSchoolUnit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	schoolID := testpkg.Tenant(t)
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, other)
	runtime := settingsCompose.NewTenantSettingsRuntime(db, nil)
	values := settingsCompose.NewSettingValueRepository(runtime)
	key := "feedback.enabled"
	for id, value := range map[int64]string{schoolID: "true", other: "false"} {
		require.NoError(t, testpkg.TenantCommand1(t, testpkg.ContextForTenant(ctx, id), db, values.Upsert, &settingsRecords.SettingValue{TenantID: id, SettingKey: key, Value: json.RawMessage(value)}))
	}
	raw := settingsCompose.NewStoredOverrideReader(runtime)
	_, _, err := raw.StoredOverride(ctx, schoolID, key)
	require.ErrorContains(t, err, "transaction is required")
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
		value, found, err := raw.StoredOverride(txCtx, schoolID, key)
		require.NoError(t, err)
		require.True(t, found)
		require.JSONEq(t, "true", string(value))
		value, found, err = raw.StoredOverride(txCtx, other, key)
		require.NoError(t, err)
		require.False(t, found)
		require.Empty(t, value)
		return nil
	}))
	root := settingOverrideRoot{raw}
	cliCtx := testpkg.WithTenantRuntime(t, context.Background(), db)
	value, found, err := root.StoredOverride(cliCtx, schoolID, key)
	require.NoError(t, err)
	require.True(t, found)
	require.JSONEq(t, "true", string(value))
	value, found, err = root.StoredOverride(cliCtx, other, key)
	require.NoError(t, err)
	require.True(t, found)
	require.JSONEq(t, "false", string(value))
	_, _, err = root.StoredOverride(cliCtx, 0, key)
	require.Error(t, err)
}
