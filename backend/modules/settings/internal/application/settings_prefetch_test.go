package config

import (
	"encoding/json"
	"errors"
	"testing"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrefetchAttachesOneSnapshotForLaterReads(t *testing.T) {
	t.Parallel()
	registry := setupTest(t)
	registerTestSetting(registry, "test.prefetch_text", config.FieldText, "default")
	registerTestSetting(registry, "test.prefetch_bool", config.FieldBoolean, false)

	repo := newMockValueRepo()
	textValue := &config.SettingValue{SettingKey: "test.prefetch_text", Value: json.RawMessage(`"tenant"`)}
	textValue.TenantID = 41
	repo.values[repo.key(41, textValue.SettingKey)] = textValue
	service := createService(registry, repo, &mockAuditRepo{}).(config.BatchSettingsService)

	ctx, err := service.Prefetch(tenantCtx(41), []string{"test.prefetch_text", "test.prefetch_bool"})
	require.NoError(t, err)
	assert.Equal(t, 1, repo.findManyCalls)

	text, err := service.ResolveString(ctx, "test.prefetch_text")
	require.NoError(t, err)
	assert.Equal(t, "tenant", text)
	enabled, err := service.ResolveBool(ctx, "test.prefetch_bool")
	require.NoError(t, err)
	assert.False(t, enabled)
	assert.Equal(t, 1, repo.findManyCalls, "reads of prefetched keys must reuse the snapshot")
}

func TestPrefetchReturnsTheReadErrorAndTheOriginalContext(t *testing.T) {
	t.Parallel()
	registry := setupTest(t)
	registerTestSetting(registry, "test.prefetch_fail", config.FieldText, "default")

	repo := newMockValueRepo()
	repo.err = errors.New("settings down")
	service := createService(registry, repo, &mockAuditRepo{}).(config.BatchSettingsService)

	ctx := tenantCtx(41)
	prepared, err := service.Prefetch(ctx, []string{"test.prefetch_fail"})
	require.ErrorContains(t, err, "settings down")
	assert.Equal(t, ctx, prepared)
}
