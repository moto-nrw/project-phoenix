package config

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	configRepository "github.com/moto-nrw/project-phoenix/database/repositories/config"
	"github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveManyForTenantsUsesOneCrossTenantQuery(t *testing.T) {
	config.ResetRegistry()
	t.Cleanup(config.ResetRegistry)
	registerTestSetting("test.multi_tenant", config.FieldText, "default")

	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)

	repository := configRepository.NewSettingValueRepository(testpkg.ConfigRuntime(db))
	valueA := &config.SettingValue{
		SettingKey: "test.multi_tenant",
		Value:      json.RawMessage(`"tenant-a"`),
	}
	valueA.SetTenantID(tenantA)
	valueB := &config.SettingValue{
		SettingKey: "test.multi_tenant",
		Value:      json.RawMessage(`"tenant-b"`),
	}
	valueB.SetTenantID(tenantB)
	require.NoError(t, repository.Upsert(testpkg.ContextForTenant(context.Background(), tenantA), valueA))
	require.NoError(t, repository.Upsert(testpkg.ContextForTenant(context.Background(), tenantB), valueB))
	t.Cleanup(func() {
		_ = repository.Delete(testpkg.ContextForTenant(context.Background(), tenantA), tenantA, valueA.SettingKey)
		_ = repository.Delete(testpkg.ContextForTenant(context.Background(), tenantB), tenantB, valueB.SettingKey)
	})

	selectCount := testpkg.CaptureSettingValueSelects(db)
	service := NewSettingsService(repository, nil, nil, testpkg.SettingsRuntime(t, db), slog.Default())
	testpkg.SetTenantRuntime(t, service, db)
	batch, ok := service.(BatchSettingsService)
	require.True(t, ok)

	snapshots, err := batch.ResolveManyForTenants(
		context.Background(),
		[]int64{tenantA, tenantB, tenantA},
		[]string{"test.multi_tenant"},
	)
	require.NoError(t, err)
	require.Len(t, snapshots, 2)
	resolvedA, err := snapshots[tenantA].String("test.multi_tenant")
	require.NoError(t, err)
	resolvedB, err := snapshots[tenantB].String("test.multi_tenant")
	require.NoError(t, err)
	assert.Equal(t, "tenant-a", resolvedA)
	assert.Equal(t, "tenant-b", resolvedB)
	assert.Equal(t, int32(1), selectCount(),
		"tenant count must not affect config.setting_values query count")
}

func TestEnrollmentEnabledForTenantsPreservesOverrideAndDefault(t *testing.T) {
	config.ResetRegistry()
	t.Cleanup(config.ResetRegistry)
	registerTestSetting(config.KeyEnrollmentEnabled, config.FieldBoolean, false)

	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)

	repository := newInMemoryValueRepo()
	override := &config.SettingValue{
		SettingKey: config.KeyEnrollmentEnabled,
		Value:      json.RawMessage(`true`),
	}
	override.SetTenantID(tenantA)
	require.NoError(t, repository.Upsert(context.Background(), override))

	service := NewSettingsService(repository, nil, nil, testSettingsRuntime{}, slog.Default())
	values, err := service.EnrollmentEnabledForTenants(context.Background(), []int64{tenantA, tenantB})
	require.NoError(t, err)
	assert.Equal(t, map[int64]bool{tenantA: true, tenantB: false}, values)
}

func TestEnrollmentEnabledForTenantsFailsWhenDefinitionIsMissing(t *testing.T) {
	config.ResetRegistry()
	t.Cleanup(config.ResetRegistry)

	service := NewSettingsService(newInMemoryValueRepo(), nil, nil, testSettingsRuntime{}, slog.Default())

	_, err := service.EnrollmentEnabledForTenants(context.Background(), []int64{41})
	var missing *DefinitionNotFoundError
	require.ErrorAs(t, err, &missing)
	assert.Equal(t, config.KeyEnrollmentEnabled, missing.Key)
}
