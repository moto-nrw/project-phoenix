package config

import (
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveManyUsesOneQueryAndContextSnapshot(t *testing.T) {
	t.Parallel()
	registry := setupTest(t)
	registerTestSetting(registry, "test.batch_text", config.FieldText, "default")
	registerTestSetting(registry, "test.batch_bool", config.FieldBoolean, false)
	registerTestSetting(registry, "test.batch_int", config.FieldNumber, 7)

	repo := newMockValueRepo()
	textValue := &config.SettingValue{
		SettingKey: "test.batch_text",
		Value:      json.RawMessage(`"tenant"`),
	}
	textValue.TenantID = 41
	repo.values[repo.key(41, textValue.SettingKey)] = textValue
	boolValue := &config.SettingValue{
		SettingKey: "test.batch_bool",
		Value:      json.RawMessage(`true`),
	}
	boolValue.TenantID = 41
	repo.values[repo.key(41, boolValue.SettingKey)] = boolValue

	service := createService(registry, repo, &mockAuditRepo{})
	batch, ok := service.(BatchSettingsService)
	require.True(t, ok)

	snapshot, err := batch.ResolveMany(tenantCtx(41), []string{
		"test.batch_text",
		"test.batch_bool",
		"test.batch_int",
		"test.batch_text",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, repo.findManyCalls)

	text, err := snapshot.String("test.batch_text")
	require.NoError(t, err)
	assert.Equal(t, "tenant", text)
	boolean, err := snapshot.Bool("test.batch_bool")
	require.NoError(t, err)
	assert.True(t, boolean)
	integer, err := snapshot.Int("test.batch_int")
	require.NoError(t, err)
	assert.Equal(t, 7, integer)
	hasOverride, err := snapshot.HasOverride("test.batch_text")
	require.NoError(t, err)
	assert.True(t, hasOverride)
	hasOverride, err = snapshot.HasOverride("test.batch_int")
	require.NoError(t, err)
	assert.False(t, hasOverride)

	snapshotCtx := WithSettingsSnapshot(tenantCtx(41), snapshot)
	_, err = service.ResolveString(snapshotCtx, "test.batch_text")
	require.NoError(t, err)
	_, err = service.ResolveBool(snapshotCtx, "test.batch_bool")
	require.NoError(t, err)
	_, err = service.ResolveInt(snapshotCtx, "test.batch_int")
	require.NoError(t, err)
	_, err = service.HasTenantOverride(snapshotCtx, "test.batch_text")
	require.NoError(t, err)
	assert.Equal(t, 1, repo.findManyCalls, "typed accessors must reuse the attached snapshot")

	_, err = service.ResolveString(WithSettingsSnapshot(tenantCtx(42), snapshot), "test.batch_text")
	require.NoError(t, err)
	assert.Equal(t, 2, repo.findManyCalls, "a snapshot from another tenant must never be reused")
}

func TestResolveManyKeepsMalformedOverrideIsolated(t *testing.T) {
	t.Parallel()
	registry := setupTest(t)
	registerTestSetting(registry, "test.valid", config.FieldBoolean, false)
	registerTestSetting(registry, "test.malformed", config.FieldBoolean, false)

	repo := newMockValueRepo()
	valid := &config.SettingValue{SettingKey: "test.valid", Value: json.RawMessage(`true`)}
	valid.TenantID = 8
	repo.values[repo.key(8, valid.SettingKey)] = valid
	malformed := &config.SettingValue{SettingKey: "test.malformed", Value: json.RawMessage(`{`)}
	malformed.TenantID = 8
	repo.values[repo.key(8, malformed.SettingKey)] = malformed

	service := createService(registry, repo, &mockAuditRepo{})
	batch := service.(BatchSettingsService)
	snapshot, err := batch.ResolveMany(tenantCtx(8), []string{"test.valid", "test.malformed"})
	require.NoError(t, err)

	value, err := snapshot.Bool("test.valid")
	require.NoError(t, err)
	assert.True(t, value)
	_, err = snapshot.Bool("test.malformed")
	require.Error(t, err)
	hasOverride, err := snapshot.HasOverride("test.malformed")
	require.NoError(t, err)
	assert.True(t, hasOverride, "malformed JSON must not erase knowledge that an override row exists")
	assert.Equal(t, 1, repo.findManyCalls)
}

func TestGetSchemaLoadsAllSettingsWithOneQuery(t *testing.T) {
	t.Parallel()
	registry := setupTest(t)
	registerTestSetting(registry, "test.schema_one", config.FieldText, "one")
	registerTestSetting(registry, "test.schema_two", config.FieldBoolean, true)
	registerTestSetting(registry, "test.schema_three", config.FieldNumber, 3)

	repo := newMockValueRepo()
	service := createService(registry, repo, &mockAuditRepo{})

	schema, err := service.GetSchema(tenantCtx(5), nil)
	require.NoError(t, err)
	require.NotNil(t, schema)
	assert.Equal(t, 1, repo.findManyCalls, "schema size must not affect settings query count")
}
