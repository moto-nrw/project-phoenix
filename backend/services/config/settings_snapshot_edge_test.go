package config

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type noisyValueRepo struct {
	*mockValueRepo
	stored []*config.SettingValue
}

func (r *noisyValueRepo) FindByTenantAndKeys(
	context.Context,
	int64,
	[]string,
) ([]*config.SettingValue, error) {
	r.findManyCalls++
	return r.stored, nil
}

func TestSettingsSnapshotRejectsNilAndUnknownKeys(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.known", config.FieldText, "known")

	service := createService(newMockValueRepo(), &mockAuditRepo{})
	batch := service.(BatchSettingsService)
	snapshot, err := batch.ResolveMany(tenantCtx(9), []string{"test.known"})
	require.NoError(t, err)

	ctx := context.Background()
	assert.Equal(t, ctx, WithSettingsSnapshot(ctx, nil))

	var nilSnapshot *SettingsSnapshot
	_, err = nilSnapshot.Value("test.known")
	require.Error(t, err)
	_, err = nilSnapshot.HasOverride("test.known")
	require.Error(t, err)
	_, err = nilSnapshot.String("test.known")
	require.Error(t, err)
	_, err = nilSnapshot.Int("test.known")
	require.Error(t, err)

	_, err = snapshot.Value("test.unknown")
	require.Error(t, err)
	_, err = snapshot.HasOverride("test.unknown")
	require.Error(t, err)
	_, err = snapshot.String("test.unknown")
	require.Error(t, err)
	_, err = snapshot.Int("test.unknown")
	require.Error(t, err)
}

func TestSettingsSnapshotContextMustContainEveryRequestedKey(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.first", config.FieldText, "first")
	registerTestSetting("test.second", config.FieldText, "second")

	repo := newMockValueRepo()
	service := createService(repo, &mockAuditRepo{})
	batch := service.(BatchSettingsService)
	snapshot, err := batch.ResolveMany(tenantCtx(23), []string{"test.first"})
	require.NoError(t, err)
	require.Equal(t, 1, repo.findManyCalls)

	ctx := WithSettingsSnapshot(tenantCtx(23), snapshot)
	value, err := service.ResolveString(ctx, "test.second")
	require.NoError(t, err)
	assert.Equal(t, "second", value)
	assert.Equal(t, 2, repo.findManyCalls, "an incomplete snapshot must not serve an unrequested key")
}

func TestSettingsSnapshotIgnoresRowsOutsideRequestedTenantAndKeys(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.requested", config.FieldText, "default")

	wrongTenant := &config.SettingValue{
		SettingKey: "test.requested",
		Value:      json.RawMessage(`"wrong tenant"`),
	}
	wrongTenant.TenantID = 72
	unrequested := &config.SettingValue{
		SettingKey: "test.unrequested",
		Value:      json.RawMessage(`"wrong key"`),
	}
	unrequested.TenantID = 71
	requested := &config.SettingValue{
		SettingKey: "test.requested",
		Value:      json.RawMessage(`"tenant value"`),
	}
	requested.TenantID = 71

	repo := &noisyValueRepo{
		mockValueRepo: newMockValueRepo(),
		stored:        []*config.SettingValue{nil, wrongTenant, unrequested, requested},
	}
	service := createService(repo, &mockAuditRepo{})
	snapshot, err := service.(BatchSettingsService).ResolveMany(
		tenantCtx(71),
		[]string{"test.requested"},
	)
	require.NoError(t, err)

	value, err := snapshot.String("test.requested")
	require.NoError(t, err)
	assert.Equal(t, "tenant value", value)
}

func TestSettingsSnapshotIntConversionsRejectInvalidNumbers(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.json_number", config.FieldNumber, json.Number("12"))
	registerTestSetting("test.invalid_json_number", config.FieldNumber, json.Number("invalid"))
	registerTestSetting("test.out_of_range", config.FieldNumber, 0)

	repo := newMockValueRepo()
	outOfRange := &config.SettingValue{
		SettingKey: "test.out_of_range",
		Value:      json.RawMessage(`1e100`),
	}
	outOfRange.TenantID = 31
	repo.values[repo.key(outOfRange.TenantID, outOfRange.SettingKey)] = outOfRange

	service := createService(repo, &mockAuditRepo{})
	snapshot, err := service.(BatchSettingsService).ResolveMany(
		tenantCtx(31),
		[]string{"test.json_number", "test.invalid_json_number", "test.out_of_range"},
	)
	require.NoError(t, err)

	value, err := snapshot.Int("test.json_number")
	require.NoError(t, err)
	assert.Equal(t, 12, value)

	_, err = snapshot.Int("test.invalid_json_number")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid number")

	_, err = snapshot.Int("test.out_of_range")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of int range")
}

func TestGetSchemaPropagatesBatchResolutionError(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.schema_error", config.FieldText, "default")

	repo := newMockValueRepo()
	repo.err = errors.New("database unavailable")
	service := createService(repo, &mockAuditRepo{})

	schema, err := service.GetSchema(tenantCtx(11), nil)
	require.Error(t, err)
	assert.Nil(t, schema)
	assert.Contains(t, err.Error(), "database unavailable")
}
