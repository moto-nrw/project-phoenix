package config_test

// Coverage for ResolveBoolForTenant / ResolveIntForTenant which both run a
// tenant.WithTenantTx round-trip. The unit-level tests in
// settings_service_test.go pass nil DB, so the tenant-tx wrappers go
// completely uncovered. These tests wire a real test-DB so the wrappers
// execute end-to-end.

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inMemoryValueRepo mirrors mockValueRepo but is package-local to this file
// so we don't depend on test files belonging to the same compilation unit
// being visible across builds.
type inMemoryValueRepo struct {
	rows map[string]*config.SettingValue // tenantID:key
}

func newInMemoryValueRepo() *inMemoryValueRepo {
	return &inMemoryValueRepo{rows: make(map[string]*config.SettingValue)}
}

func (r *inMemoryValueRepo) key(tenantID int64, settingKey string) string {
	return settingKey + "@" + itoa(tenantID)
}

func itoa(i int64) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	b := make([]byte, 0, 20)
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func (r *inMemoryValueRepo) FindByTenantAndKey(_ context.Context, tenantID int64, settingKey string) (*config.SettingValue, error) {
	return r.rows[r.key(tenantID, settingKey)], nil
}

func (r *inMemoryValueRepo) FindByTenantAndKeys(_ context.Context, tenantID int64, settingKeys []string) ([]*config.SettingValue, error) {
	requested := make(map[string]struct{}, len(settingKeys))
	for _, key := range settingKeys {
		requested[key] = struct{}{}
	}
	out := []*config.SettingValue{}
	for _, v := range r.rows {
		if v.GetTenantID() != tenantID {
			continue
		}
		if _, ok := requested[v.SettingKey]; !ok {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

func (r *inMemoryValueRepo) FindByTenantsAndKeys(_ context.Context, tenantIDs []int64, settingKeys []string) ([]*config.SettingValue, error) {
	tenants := make(map[int64]struct{}, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		tenants[tenantID] = struct{}{}
	}
	requested := make(map[string]struct{}, len(settingKeys))
	for _, key := range settingKeys {
		requested[key] = struct{}{}
	}
	var out []*config.SettingValue
	for _, value := range r.rows {
		if _, ok := tenants[value.GetTenantID()]; !ok {
			continue
		}
		if _, ok := requested[value.SettingKey]; ok {
			out = append(out, value)
		}
	}
	return out, nil
}

func (r *inMemoryValueRepo) Upsert(_ context.Context, sv *config.SettingValue) error {
	r.rows[r.key(sv.GetTenantID(), sv.SettingKey)] = sv
	return nil
}

func (r *inMemoryValueRepo) Delete(_ context.Context, tenantID int64, settingKey string) error {
	delete(r.rows, r.key(tenantID, settingKey))
	return nil
}

type noopAuditRepo struct{}

func (noopAuditRepo) Create(_ context.Context, _ *config.SettingAuditEntry) error { return nil }

// --- Tests ---

func TestResolveBoolForTenant_RegistryDefault_NoOverride(t *testing.T) {
	config.ResetRegistry()
	t.Cleanup(func() { config.ResetRegistry() })

	config.Register(config.Definition{
		Key:      "test.tenant_bool",
		Type:     config.FieldBoolean,
		Default:  true,
		Tab:      "test",
		Category: "test",
	})

	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	svc := configSvc.NewSettingsService(newInMemoryValueRepo(), noopAuditRepo{}, nil, db, slog.Default())
	testpkg.SetTenantRuntime(t, svc, db)
	val, err := svc.ResolveBoolForTenant(context.Background(), tenantID, "test.tenant_bool")
	require.NoError(t, err)
	assert.True(t, val, "registry default must be returned when no tenant override exists")
}

func TestResolveBoolForTenant_TenantOverrideWins(t *testing.T) {
	config.ResetRegistry()
	t.Cleanup(func() { config.ResetRegistry() })
	config.Register(config.Definition{
		Key: "test.tenant_bool_override", Type: config.FieldBoolean,
		Default: true, Tab: "test", Category: "test",
	})

	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	repo := newInMemoryValueRepo()
	sv := &config.SettingValue{
		Model:      modelBase.Model{},
		SettingKey: "test.tenant_bool_override",
		Value:      json.RawMessage(`false`),
	}
	sv.SetTenantID(tenantID)
	require.NoError(t, repo.Upsert(context.Background(), sv))

	svc := configSvc.NewSettingsService(repo, noopAuditRepo{}, nil, db, slog.Default())
	testpkg.SetTenantRuntime(t, svc, db)
	val, err := svc.ResolveBoolForTenant(context.Background(), tenantID, "test.tenant_bool_override")
	require.NoError(t, err)
	assert.False(t, val, "tenant override (false) must beat the registry default (true)")
}

func TestResolveIntForTenant_RegistryDefault_NoOverride(t *testing.T) {
	config.ResetRegistry()
	t.Cleanup(func() { config.ResetRegistry() })
	config.Register(config.Definition{
		Key: "test.tenant_int", Type: config.FieldNumber,
		Default: 17, Tab: "test", Category: "test",
	})

	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	svc := configSvc.NewSettingsService(newInMemoryValueRepo(), noopAuditRepo{}, nil, db, slog.Default())
	testpkg.SetTenantRuntime(t, svc, db)
	val, err := svc.ResolveIntForTenant(context.Background(), tenantID, "test.tenant_int")
	require.NoError(t, err)
	assert.Equal(t, 17, val)
}

func TestResolveIntForTenant_TenantOverrideWins(t *testing.T) {
	config.ResetRegistry()
	t.Cleanup(func() { config.ResetRegistry() })
	config.Register(config.Definition{
		Key: "test.tenant_int_override", Type: config.FieldNumber,
		Default: 17, Tab: "test", Category: "test",
	})

	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	repo := newInMemoryValueRepo()
	sv := &config.SettingValue{
		SettingKey: "test.tenant_int_override",
		Value:      json.RawMessage(`42`),
	}
	sv.SetTenantID(tenantID)
	require.NoError(t, repo.Upsert(context.Background(), sv))

	svc := configSvc.NewSettingsService(repo, noopAuditRepo{}, nil, db, slog.Default())
	testpkg.SetTenantRuntime(t, svc, db)
	val, err := svc.ResolveIntForTenant(context.Background(), tenantID, "test.tenant_int_override")
	require.NoError(t, err)
	assert.Equal(t, 42, val, "tenant override (42) must beat the registry default (17)")
}

func TestResolveIntForTenant_UnknownKey_ReturnsError(t *testing.T) {
	config.ResetRegistry()
	t.Cleanup(func() { config.ResetRegistry() })

	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	svc := configSvc.NewSettingsService(newInMemoryValueRepo(), noopAuditRepo{}, nil, db, slog.Default())
	testpkg.SetTenantRuntime(t, svc, db)
	_, err := svc.ResolveIntForTenant(context.Background(), tenantID, "unregistered.key")
	require.Error(t, err, "an unknown setting must propagate as an error even through the tenant-tx wrapper")
}
