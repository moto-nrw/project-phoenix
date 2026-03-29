package config_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock repositories ---

type mockValueRepo struct {
	values map[string]*config.SettingValue // key: "tenantID:settingKey"
	err    error
}

func newMockValueRepo() *mockValueRepo {
	return &mockValueRepo{values: make(map[string]*config.SettingValue)}
}

func (m *mockValueRepo) key(tenantID int64, settingKey string) string {
	return settingKey // simplified for tests
}

func (m *mockValueRepo) FindByTenantAndKey(_ context.Context, tenantID int64, settingKey string) (*config.SettingValue, error) {
	if m.err != nil {
		return nil, m.err
	}
	sv, ok := m.values[m.key(tenantID, settingKey)]
	if !ok {
		return nil, nil
	}
	return sv, nil
}

func (m *mockValueRepo) FindByTenant(_ context.Context, _ int64) ([]*config.SettingValue, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []*config.SettingValue
	for _, v := range m.values {
		result = append(result, v)
	}
	return result, nil
}

func (m *mockValueRepo) Upsert(_ context.Context, sv *config.SettingValue) error {
	if m.err != nil {
		return m.err
	}
	m.values[m.key(sv.TenantID, sv.SettingKey)] = sv
	return nil
}

func (m *mockValueRepo) Delete(_ context.Context, _ int64, settingKey string) error {
	if m.err != nil {
		return m.err
	}
	delete(m.values, settingKey)
	return nil
}

type mockAuditRepo struct {
	entries []*config.SettingAuditEntry
	err     error
}

func (m *mockAuditRepo) Create(_ context.Context, entry *config.SettingAuditEntry) error {
	if m.err != nil {
		return m.err
	}
	m.entries = append(m.entries, entry)
	return nil
}

// --- Helpers ---

func setupTest(t *testing.T) {
	t.Helper()
	config.ResetRegistry()
	t.Cleanup(func() { config.ResetRegistry() })
}

func registerTestSetting(key string, fieldType config.FieldType, defaultVal any) {
	def := config.Definition{
		Key:      key,
		Label:    "Test " + key,
		Type:     fieldType,
		Default:  defaultVal,
		Tab:      "test",
		Category: "test",
	}
	if fieldType == config.FieldSelect {
		def.Options = &config.SelectOptions{
			Static: []config.SelectOption{{Label: "A", Value: "a"}},
		}
	}
	config.Register(def)
}

func tenantCtx(tenantID int64) context.Context {
	return tenant.WithTenantID(context.Background(), tenantID)
}

func createService(valueRepo config.SettingValueRepository, auditRepo config.SettingAuditRepository) configSvc.SettingsService {
	return configSvc.NewSettingsService(valueRepo, auditRepo, nil, slog.Default())
}

// --- Tests ---

func TestResolve_ReturnsDefault_WhenNoOverride(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.timeout", config.FieldNumber, 30)

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	val, err := svc.Resolve(tenantCtx(1), "test.timeout")
	require.NoError(t, err)
	assert.Equal(t, 30, val)
}

func TestResolve_ReturnsTenantOverride(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.timeout", config.FieldNumber, 30)

	repo := newMockValueRepo()
	repo.values["test.timeout"] = &config.SettingValue{
		SettingKey: "test.timeout",
		Value:      json.RawMessage(`60`),
	}
	repo.values["test.timeout"].TenantID = 1

	svc := createService(repo, &mockAuditRepo{})

	val, err := svc.Resolve(tenantCtx(1), "test.timeout")
	require.NoError(t, err)
	assert.Equal(t, float64(60), val) // JSON numbers unmarshal as float64
}

func TestResolve_UnknownKey_ReturnsError(t *testing.T) {
	setupTest(t)
	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	_, err := svc.Resolve(tenantCtx(1), "nonexistent.key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolve_NoTenantContext_ReturnsDefault(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.flag", config.FieldBoolean, true)

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	val, err := svc.Resolve(context.Background(), "test.flag")
	require.NoError(t, err)
	assert.Equal(t, true, val)
}

func TestResolveString(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.name", config.FieldText, "default")

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	val, err := svc.ResolveString(tenantCtx(1), "test.name")
	require.NoError(t, err)
	assert.Equal(t, "default", val)
}

func TestResolveBool(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.enabled", config.FieldBoolean, true)

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	val, err := svc.ResolveBool(tenantCtx(1), "test.enabled")
	require.NoError(t, err)
	assert.True(t, val)
}

func TestResolveInt(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.count", config.FieldNumber, 42)

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	val, err := svc.ResolveInt(tenantCtx(1), "test.count")
	require.NoError(t, err)
	assert.Equal(t, 42, val)
}

func TestResolveInt_FromOverride(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.count", config.FieldNumber, 42)

	repo := newMockValueRepo()
	repo.values["test.count"] = &config.SettingValue{
		SettingKey: "test.count",
		Value:      json.RawMessage(`99`),
	}
	repo.values["test.count"].TenantID = 1

	svc := createService(repo, &mockAuditRepo{})

	val, err := svc.ResolveInt(tenantCtx(1), "test.count")
	require.NoError(t, err)
	assert.Equal(t, 99, val)
}

func TestSetValue_StoresValueAndAudit(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.timeout", config.FieldNumber, 30)

	valueRepo := newMockValueRepo()
	auditRepo := &mockAuditRepo{}
	svc := createService(valueRepo, auditRepo)

	changedBy := int64(42)
	err := svc.SetValue(tenantCtx(1), "test.timeout", 60, &changedBy)
	require.NoError(t, err)

	// Value stored
	assert.Len(t, valueRepo.values, 1)

	// Audit entry created
	assert.Len(t, auditRepo.entries, 1)
	assert.Equal(t, "set", auditRepo.entries[0].Action)
	assert.Equal(t, "test.timeout", auditRepo.entries[0].SettingKey)
}

func TestSetValue_UnknownKey_ReturnsError(t *testing.T) {
	setupTest(t)
	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "nonexistent", "val", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSetValue_ValidationError_NumberBelowMin(t *testing.T) {
	setupTest(t)
	min := float64(10)
	config.Register(config.Definition{
		Key:        "test.min",
		Type:       config.FieldNumber,
		Default:    15,
		Tab:        "test",
		Category:   "test",
		Validation: &config.ValidationRules{Min: &min},
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "test.min", 5, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "below minimum")
}

func TestSetValue_ValidationError_NumberAboveMax(t *testing.T) {
	setupTest(t)
	max := float64(100)
	config.Register(config.Definition{
		Key:        "test.max",
		Type:       config.FieldNumber,
		Default:    50,
		Tab:        "test",
		Category:   "test",
		Validation: &config.ValidationRules{Max: &max},
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "test.max", 200, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

func TestResetValue_DeletesOverrideAndAudits(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.timeout", config.FieldNumber, 30)

	valueRepo := newMockValueRepo()
	valueRepo.values["test.timeout"] = &config.SettingValue{
		SettingKey: "test.timeout",
		Value:      json.RawMessage(`60`),
	}
	valueRepo.values["test.timeout"].TenantID = 1

	auditRepo := &mockAuditRepo{}
	svc := createService(valueRepo, auditRepo)

	changedBy := int64(42)
	err := svc.ResetValue(tenantCtx(1), "test.timeout", &changedBy)
	require.NoError(t, err)

	// Value deleted
	assert.Empty(t, valueRepo.values)

	// Audit entry created with action=reset
	assert.Len(t, auditRepo.entries, 1)
	assert.Equal(t, "reset", auditRepo.entries[0].Action)
}

func TestResetValue_UnknownKey_ReturnsError(t *testing.T) {
	setupTest(t)
	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.ResetValue(tenantCtx(1), "nonexistent", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetSchema_ReturnsGroupedSettings(t *testing.T) {
	setupTest(t)

	config.Register(config.Definition{
		Key:            "ops.enabled",
		Label:          "Enabled",
		Type:           config.FieldBoolean,
		Default:        true,
		Tab:            "operations",
		Category:       "sessions",
		SortOrder:      1,
		ReadPermission: "config:read",
	})
	config.Register(config.Definition{
		Key:            "ops.time",
		Label:          "Time",
		Type:           config.FieldTime,
		Default:        "18:00",
		Tab:            "operations",
		Category:       "sessions",
		SortOrder:      2,
		ReadPermission: "config:read",
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	schema, err := svc.GetSchema(tenantCtx(1), []string{"config:read"})
	require.NoError(t, err)
	require.NotNil(t, schema)
	require.Len(t, schema.Tabs, 1)
	assert.Equal(t, "operations", schema.Tabs[0].Key)
	require.Len(t, schema.Tabs[0].Categories, 1)
	assert.Equal(t, "sessions", schema.Tabs[0].Categories[0].Key)
	assert.Len(t, schema.Tabs[0].Categories[0].Items, 2)
}

func TestGetSchema_FiltersByPermission(t *testing.T) {
	setupTest(t)

	config.Register(config.Definition{
		Key:            "admin.secret",
		Label:          "Secret",
		Type:           config.FieldText,
		Default:        "hidden",
		Tab:            "admin",
		Category:       "security",
		ReadPermission: "config:manage",
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	// User without config:manage should not see the setting
	schema, err := svc.GetSchema(tenantCtx(1), []string{"config:read"})
	require.NoError(t, err)
	assert.Empty(t, schema.Tabs)
}

func TestGetSchema_PasswordMasked(t *testing.T) {
	setupTest(t)

	config.Register(config.Definition{
		Key:      "security.pin",
		Label:    "PIN",
		Type:     config.FieldPassword,
		Default:  "",
		Tab:      "security",
		Category: "auth",
	})

	valueRepo := newMockValueRepo()
	valueRepo.values["security.pin"] = &config.SettingValue{
		SettingKey: "security.pin",
		Value:      json.RawMessage(`"1234"`),
	}
	valueRepo.values["security.pin"].TenantID = 1

	svc := createService(valueRepo, &mockAuditRepo{})

	schema, err := svc.GetSchema(tenantCtx(1), []string{})
	require.NoError(t, err)
	require.Len(t, schema.Tabs, 1)

	item := schema.Tabs[0].Categories[0].Items[0]
	assert.Equal(t, "••••••", item.Value)
}

func TestGetSchema_DependsOn_HidesChild(t *testing.T) {
	setupTest(t)

	config.Register(config.Definition{
		Key:      "parent.enabled",
		Label:    "Enabled",
		Type:     config.FieldBoolean,
		Default:  false, // parent is OFF
		Tab:      "test",
		Category: "deps",
	})
	config.Register(config.Definition{
		Key:      "parent.time",
		Label:    "Time",
		Type:     config.FieldTime,
		Default:  "18:00",
		Tab:      "test",
		Category: "deps",
		DependsOn: &config.Dependency{
			Key:       "parent.enabled",
			Condition: "eq",
			Value:     true,
		},
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	schema, err := svc.GetSchema(tenantCtx(1), []string{})
	require.NoError(t, err)

	items := schema.Tabs[0].Categories[0].Items
	for _, item := range items {
		if item.Key == "parent.time" {
			assert.False(t, item.Visible, "child should be hidden when parent is false")
		}
		if item.Key == "parent.enabled" {
			assert.True(t, item.Visible, "parent should be visible")
		}
	}
}

func TestGetSchema_DependsOn_ShowsChild(t *testing.T) {
	setupTest(t)

	config.Register(config.Definition{
		Key:      "parent2.enabled",
		Label:    "Enabled",
		Type:     config.FieldBoolean,
		Default:  true, // parent is ON
		Tab:      "test",
		Category: "deps",
	})
	config.Register(config.Definition{
		Key:      "parent2.time",
		Label:    "Time",
		Type:     config.FieldTime,
		Default:  "18:00",
		Tab:      "test",
		Category: "deps",
		DependsOn: &config.Dependency{
			Key:       "parent2.enabled",
			Condition: "eq",
			Value:     true,
		},
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	schema, err := svc.GetSchema(tenantCtx(1), []string{})
	require.NoError(t, err)

	items := schema.Tabs[0].Categories[0].Items
	for _, item := range items {
		if item.Key == "parent2.time" {
			assert.True(t, item.Visible, "child should be visible when parent is true")
		}
	}
}

func TestSettingsError_Unwrap(t *testing.T) {
	inner := &configSvc.DefinitionNotFoundError{Key: "test"}
	err := &configSvc.SettingsError{Op: "resolve", Err: inner}

	assert.Contains(t, err.Error(), "resolve")
	assert.Contains(t, err.Error(), "test")
	assert.Equal(t, inner, err.Unwrap())
}

func TestDefinitionNotFoundError(t *testing.T) {
	err := &configSvc.DefinitionNotFoundError{Key: "missing.key"}
	assert.Contains(t, err.Error(), "missing.key")
	assert.ErrorIs(t, err, configSvc.ErrDefinitionNotFound)
}

func TestInvalidValueError(t *testing.T) {
	err := &configSvc.InvalidValueError{Key: "test.key", Reason: "too small"}
	assert.Contains(t, err.Error(), "test.key")
	assert.Contains(t, err.Error(), "too small")
	assert.ErrorIs(t, err, configSvc.ErrInvalidValue)
}
