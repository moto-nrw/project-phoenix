package config_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"

	configRepository "github.com/moto-nrw/project-phoenix/database/repositories/config"
	"github.com/moto-nrw/project-phoenix/models/config"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type settingValuesSelectCounter struct {
	count atomic.Int32
}

func (c *settingValuesSelectCounter) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	query := strings.ToLower(event.Query)
	if strings.HasPrefix(strings.TrimSpace(query), "select") &&
		strings.Contains(query, "config.setting_values") {
		c.count.Add(1)
	}
	return ctx
}

func (*settingValuesSelectCounter) AfterQuery(context.Context, *bun.QueryEvent) {}

func TestResolveManyForTenantsUsesOneCrossTenantQuery(t *testing.T) {
	config.ResetRegistry()
	t.Cleanup(config.ResetRegistry)
	registerTestSetting("test.multi_tenant", config.FieldText, "default")

	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)

	repository := configRepository.NewSettingValueRepository(db)
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
	require.NoError(t, repository.Upsert(tenant.WithTenantID(context.Background(), tenantA), valueA))
	require.NoError(t, repository.Upsert(tenant.WithTenantID(context.Background(), tenantB), valueB))
	t.Cleanup(func() {
		_ = repository.Delete(tenant.WithTenantID(context.Background(), tenantA), tenantA, valueA.SettingKey)
		_ = repository.Delete(tenant.WithTenantID(context.Background(), tenantB), tenantB, valueB.SettingKey)
	})

	counter := &settingValuesSelectCounter{}
	db.AddQueryHook(counter)
	service := configService.NewSettingsService(repository, nil, nil, db, slog.Default())
	testpkg.SetTenantRuntime(t, service, db)
	batch, ok := service.(configService.BatchSettingsService)
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
	assert.Equal(t, int32(1), counter.count.Load(),
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

	runtime, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, bun.Tx{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error { return fn(ctx, bun.Tx{}) },
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)
	service := configService.NewSettingsService(repository, nil, nil, nil, slog.Default())
	service.(interface{ SetTenantRuntime(tenant.UnitOfWork) }).SetTenantRuntime(runtime)
	values, err := service.EnrollmentEnabledForTenants(context.Background(), []int64{tenantA, tenantB})
	require.NoError(t, err)
	assert.Equal(t, map[int64]bool{tenantA: true, tenantB: false}, values)
}

func TestEnrollmentEnabledForTenantsFailsWhenDefinitionIsMissing(t *testing.T) {
	config.ResetRegistry()
	t.Cleanup(config.ResetRegistry)

	runtime, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, bun.Tx{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error { return fn(ctx, bun.Tx{}) },
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)
	service := configService.NewSettingsService(newInMemoryValueRepo(), nil, nil, nil, slog.Default())
	service.(interface{ SetTenantRuntime(tenant.UnitOfWork) }).SetTenantRuntime(runtime)

	_, err = service.EnrollmentEnabledForTenants(context.Background(), []int64{41})
	var missing *configService.DefinitionNotFoundError
	require.ErrorAs(t, err, &missing)
	assert.Equal(t, config.KeyEnrollmentEnabled, missing.Key)
}
