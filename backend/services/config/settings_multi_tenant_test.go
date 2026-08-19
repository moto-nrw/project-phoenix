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
	t.Cleanup(func() { testpkg.CleanupTenantTestData(t, db, tenantA, tenantB) })

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
