package config_test

import (
	"context"
	"encoding/json"
	"fmt"
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
	return fmt.Sprintf("%d:%s", tenantID, settingKey)
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

func (m *mockValueRepo) Delete(_ context.Context, tenantID int64, settingKey string) error {
	if m.err != nil {
		return m.err
	}
	delete(m.values, m.key(tenantID, settingKey))
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
	repo.values["1:test.timeout"] = &config.SettingValue{
		SettingKey: "test.timeout",
		Value:      json.RawMessage(`60`),
	}
	repo.values["1:test.timeout"].TenantID = 1

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
	repo.values["1:test.count"] = &config.SettingValue{
		SettingKey: "test.count",
		Value:      json.RawMessage(`99`),
	}
	repo.values["1:test.count"].TenantID = 1

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
	err := svc.SetValue(tenantCtx(1), "test.timeout", 60, &changedBy, nil)
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

	err := svc.SetValue(tenantCtx(1), "nonexistent", "val", nil, nil)
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

	err := svc.SetValue(tenantCtx(1), "test.min", 5, nil, nil)
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

	err := svc.SetValue(tenantCtx(1), "test.max", 200, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

func TestResetValue_DeletesOverrideAndAudits(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.timeout", config.FieldNumber, 30)

	valueRepo := newMockValueRepo()
	valueRepo.values["1:test.timeout"] = &config.SettingValue{
		SettingKey: "test.timeout",
		Value:      json.RawMessage(`60`),
	}
	valueRepo.values["1:test.timeout"].TenantID = 1

	auditRepo := &mockAuditRepo{}
	svc := createService(valueRepo, auditRepo)

	changedBy := int64(42)
	err := svc.ResetValue(tenantCtx(1), "test.timeout", &changedBy, nil)
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

	err := svc.ResetValue(tenantCtx(1), "nonexistent", nil, nil)
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
	valueRepo.values["1:security.pin"] = &config.SettingValue{
		SettingKey: "security.pin",
		Value:      json.RawMessage(`"1234"`),
	}
	valueRepo.values["1:security.pin"].TenantID = 1

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

func TestPermissionDeniedError(t *testing.T) {
	err := &configSvc.PermissionDeniedError{Key: "admin.setting", RequiredPermission: "config:manage"}
	assert.Contains(t, err.Error(), "admin.setting")
	assert.Contains(t, err.Error(), "config:manage")
	assert.ErrorIs(t, err, configSvc.ErrPermissionDenied)
}

// --- Additional coverage tests ---

func TestResolve_RepoError_ReturnsError(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.val", config.FieldText, "default")

	repo := newMockValueRepo()
	repo.err = fmt.Errorf("db connection failed")
	svc := createService(repo, &mockAuditRepo{})

	_, err := svc.Resolve(tenantCtx(1), "test.val")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db connection failed")
}

func TestResolveString_FromOverride(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.name", config.FieldText, "default")

	repo := newMockValueRepo()
	repo.values["1:test.name"] = &config.SettingValue{
		SettingKey: "test.name",
		Value:      json.RawMessage(`"custom"`),
	}
	repo.values["1:test.name"].TenantID = 1

	svc := createService(repo, &mockAuditRepo{})

	val, err := svc.ResolveString(tenantCtx(1), "test.name")
	require.NoError(t, err)
	assert.Equal(t, "custom", val)
}

func TestResolveString_NilDefault(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.optional", config.FieldText, nil)

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	val, err := svc.ResolveString(tenantCtx(1), "test.optional")
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestResolveBool_FromOverride(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.flag", config.FieldBoolean, true)

	repo := newMockValueRepo()
	repo.values["1:test.flag"] = &config.SettingValue{
		SettingKey: "test.flag",
		Value:      json.RawMessage(`false`),
	}
	repo.values["1:test.flag"].TenantID = 1

	svc := createService(repo, &mockAuditRepo{})

	val, err := svc.ResolveBool(tenantCtx(1), "test.flag")
	require.NoError(t, err)
	assert.False(t, val)
}

func TestResolveBool_NilDefault(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.nilbool", config.FieldBoolean, nil)

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	val, err := svc.ResolveBool(tenantCtx(1), "test.nilbool")
	require.NoError(t, err)
	assert.False(t, val)
}

func TestResolveInt_NilDefault(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.nilint", config.FieldNumber, nil)

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	val, err := svc.ResolveInt(tenantCtx(1), "test.nilint")
	require.NoError(t, err)
	assert.Equal(t, 0, val)
}

func TestSetValue_WithExistingOverride_RecordsOldValue(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.val", config.FieldNumber, 10)

	repo := newMockValueRepo()
	repo.values["1:test.val"] = &config.SettingValue{
		SettingKey: "test.val",
		Value:      json.RawMessage(`20`),
	}
	repo.values["1:test.val"].TenantID = 1

	auditRepo := &mockAuditRepo{}
	svc := createService(repo, auditRepo)

	err := svc.SetValue(tenantCtx(1), "test.val", 30, nil, nil)
	require.NoError(t, err)

	// Audit should have old_value
	require.Len(t, auditRepo.entries, 1)
	assert.Equal(t, json.RawMessage(`20`), auditRepo.entries[0].OldValue)
	assert.Equal(t, json.RawMessage(`30`), auditRepo.entries[0].NewValue)
}

func TestSetValue_AuditError_DoesNotFail(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.val", config.FieldText, "default")

	auditRepo := &mockAuditRepo{err: fmt.Errorf("audit write failed")}
	svc := createService(newMockValueRepo(), auditRepo)

	// Should succeed even if audit fails (audit is best-effort)
	err := svc.SetValue(tenantCtx(1), "test.val", "new", nil, nil)
	require.NoError(t, err)
}

func TestResetValue_NilChangedBy(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.val", config.FieldText, "default")

	repo := newMockValueRepo()
	repo.values["1:test.val"] = &config.SettingValue{
		SettingKey: "test.val",
		Value:      json.RawMessage(`"custom"`),
	}
	repo.values["1:test.val"].TenantID = 1

	auditRepo := &mockAuditRepo{}
	svc := createService(repo, auditRepo)

	err := svc.ResetValue(tenantCtx(1), "test.val", nil, nil)
	require.NoError(t, err)
	assert.Nil(t, auditRepo.entries[0].ChangedBy)
}

func TestGetSchema_WritableFlag(t *testing.T) {
	setupTest(t)

	config.Register(config.Definition{
		Key:             "test.readonly",
		Label:           "ReadOnly",
		Type:            config.FieldText,
		Default:         "val",
		Tab:             "test",
		Category:        "test",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	// User with read but not update
	schema, err := svc.GetSchema(tenantCtx(1), []string{"config:read"})
	require.NoError(t, err)
	require.Len(t, schema.Tabs, 1)
	item := schema.Tabs[0].Categories[0].Items[0]
	assert.False(t, item.Writable, "should not be writable without config:update")

	// User with both
	schema2, err := svc.GetSchema(tenantCtx(1), []string{"config:read", "config:update"})
	require.NoError(t, err)
	item2 := schema2.Tabs[0].Categories[0].Items[0]
	assert.True(t, item2.Writable, "should be writable with config:update")
}

func TestGetSchema_DependsOn_NeqCondition(t *testing.T) {
	setupTest(t)

	config.Register(config.Definition{
		Key:      "p.mode",
		Label:    "Mode",
		Type:     config.FieldText,
		Default:  "auto",
		Tab:      "test",
		Category: "deps",
	})
	config.Register(config.Definition{
		Key:      "p.manual_config",
		Label:    "Manual Config",
		Type:     config.FieldText,
		Default:  "",
		Tab:      "test",
		Category: "deps",
		DependsOn: &config.Dependency{
			Key:       "p.mode",
			Condition: "neq",
			Value:     "auto",
		},
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	schema, err := svc.GetSchema(tenantCtx(1), []string{})
	require.NoError(t, err)

	items := schema.Tabs[0].Categories[0].Items
	for _, item := range items {
		if item.Key == "p.manual_config" {
			assert.False(t, item.Visible, "should be hidden when mode=auto (neq auto is false)")
		}
	}
}

func TestSetValue_ValidationRequired(t *testing.T) {
	setupTest(t)
	config.Register(config.Definition{
		Key:        "test.required",
		Type:       config.FieldText,
		Default:    "default",
		Tab:        "test",
		Category:   "test",
		Validation: &config.ValidationRules{Required: true},
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "test.required", nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestSetValue_NonNumberForNumberField(t *testing.T) {
	setupTest(t)
	min := float64(1)
	config.Register(config.Definition{
		Key:        "test.num",
		Type:       config.FieldNumber,
		Default:    5,
		Tab:        "test",
		Category:   "test",
		Validation: &config.ValidationRules{Min: &min},
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "test.num", "not_a_number", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected a number")
}

// --- Additional coverage: typed resolver edge cases ---

func TestResolveString_NonStringValue(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.numstr", config.FieldNumber, 42)

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	// Default is int 42, ResolveString should convert it
	val, err := svc.ResolveString(tenantCtx(1), "test.numstr")
	require.NoError(t, err)
	assert.Equal(t, "42", val)
}

func TestResolveString_ErrorFromResolve(t *testing.T) {
	setupTest(t)
	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	_, err := svc.ResolveString(tenantCtx(1), "nonexistent")
	require.Error(t, err)
}

func TestResolveBool_ErrorFromResolve(t *testing.T) {
	setupTest(t)
	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	_, err := svc.ResolveBool(tenantCtx(1), "nonexistent")
	require.Error(t, err)
}

func TestResolveBool_NonBoolValue(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.notbool", config.FieldText, "hello")

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	val, err := svc.ResolveBool(tenantCtx(1), "test.notbool")
	require.Error(t, err) // type mismatch now returns an error
	assert.False(t, val)
}

func TestResolveInt_ErrorFromResolve(t *testing.T) {
	setupTest(t)
	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	_, err := svc.ResolveInt(tenantCtx(1), "nonexistent")
	require.Error(t, err)
}

func TestResolveInt_NonNumericValue(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.notint", config.FieldText, "hello")

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	val, err := svc.ResolveInt(tenantCtx(1), "test.notint")
	require.Error(t, err) // type mismatch now returns an error
	assert.Equal(t, 0, val)
}

func TestResolveInt_IntValue(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.intval", config.FieldNumber, 0)

	repo := newMockValueRepo()
	repo.values["1:test.intval"] = &config.SettingValue{
		SettingKey: "test.intval",
		Value:      json.RawMessage(`42`),
	}
	repo.values["1:test.intval"].TenantID = 1

	svc := createService(repo, &mockAuditRepo{})

	val, err := svc.ResolveInt(tenantCtx(1), "test.intval")
	require.NoError(t, err)
	assert.Equal(t, 42, val)
}

func TestSetValue_RepoUpsertError(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.val", config.FieldText, "default")

	repo := newMockValueRepo()
	repo.err = fmt.Errorf("upsert failed")
	svc := createService(repo, &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "test.val", "new", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upsert failed")
}

func TestResetValue_RepoDeleteError(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.val", config.FieldText, "default")

	repo := newMockValueRepo()
	repo.err = fmt.Errorf("delete failed")
	svc := createService(repo, &mockAuditRepo{})

	err := svc.ResetValue(tenantCtx(1), "test.val", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete failed")
}

func TestSetValue_NilChangedBy(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.val", config.FieldText, "default")

	auditRepo := &mockAuditRepo{}
	svc := createService(newMockValueRepo(), auditRepo)

	err := svc.SetValue(tenantCtx(1), "test.val", "new", nil, nil)
	require.NoError(t, err)
	assert.Len(t, auditRepo.entries, 1)
	assert.Nil(t, auditRepo.entries[0].ChangedBy)
}

func TestResetValue_AuditError_DoesNotFail(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.val", config.FieldText, "default")

	auditRepo := &mockAuditRepo{err: fmt.Errorf("audit write failed")}
	svc := createService(newMockValueRepo(), auditRepo)

	// Should succeed even if audit fails
	err := svc.ResetValue(tenantCtx(1), "test.val", nil, nil)
	require.NoError(t, err)
}

// =============================================================================
// Type validation tests
// =============================================================================

func TestSetValue_BooleanRejectsString(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.flag", config.FieldBoolean, true)

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "test.flag", "yes", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected a boolean")
}

func TestSetValue_BooleanAcceptsBool(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.flag", config.FieldBoolean, true)

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "test.flag", false, nil, nil)
	require.NoError(t, err)
}

func TestSetValue_TimeRejectsInvalidFormat(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.time", config.FieldTime, "18:00")

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "test.time", "25:99", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected time in HH:MM format")
}

func TestSetValue_TimeRejectsNonString(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.time", config.FieldTime, "18:00")

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "test.time", 1800, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected a time string")
}

func TestSetValue_TimeAcceptsValidTime(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.time", config.FieldTime, "18:00")

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "test.time", "14:30", nil, nil)
	require.NoError(t, err)
}

func TestSetValue_SelectRejectsInvalidOption(t *testing.T) {
	setupTest(t)
	config.Register(config.Definition{
		Key:      "test.sel",
		Type:     config.FieldSelect,
		Default:  "a",
		Tab:      "test",
		Category: "test",
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "A", Value: "a"},
				{Label: "B", Value: "b"},
			},
		},
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "test.sel", "c", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid option")
}

func TestSetValue_SelectAcceptsValidOption(t *testing.T) {
	setupTest(t)
	config.Register(config.Definition{
		Key:      "test.sel",
		Type:     config.FieldSelect,
		Default:  "a",
		Tab:      "test",
		Category: "test",
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "A", Value: "a"},
				{Label: "B", Value: "b"},
			},
		},
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "test.sel", "b", nil, nil)
	require.NoError(t, err)
}

func TestSetValue_TextRejectsNonString(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.text", config.FieldText, "default")

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "test.text", 123, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected a string")
}

// =============================================================================
// PIN validation tests
// =============================================================================

func TestSetValue_PINRejectsMoreThan4Digits(t *testing.T) {
	setupTest(t)
	pinPattern := `^\d{4}$`
	config.Register(config.Definition{
		Key:        "security.ogs_device_pin",
		Label:      "PIN",
		Type:       config.FieldPassword,
		Default:    "",
		Tab:        "security",
		Category:   "auth",
		Validation: &config.ValidationRules{Pattern: &pinPattern},
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "security.ogs_device_pin", "123456", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match required pattern")
}

func TestSetValue_PINRejectsNonNumeric(t *testing.T) {
	setupTest(t)
	pinPattern := `^\d{4}$`
	config.Register(config.Definition{
		Key:        "security.ogs_device_pin",
		Label:      "PIN",
		Type:       config.FieldPassword,
		Default:    "",
		Tab:        "security",
		Category:   "auth",
		Validation: &config.ValidationRules{Pattern: &pinPattern},
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "security.ogs_device_pin", "abcd", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match required pattern")
}

func TestSetValue_PINAccepts4Digits(t *testing.T) {
	setupTest(t)
	pinPattern := `^\d{4}$`
	config.Register(config.Definition{
		Key:        "security.ogs_device_pin",
		Label:      "PIN",
		Type:       config.FieldPassword,
		Default:    "",
		Tab:        "security",
		Category:   "auth",
		Validation: &config.ValidationRules{Pattern: &pinPattern},
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "security.ogs_device_pin", "1234", nil, nil)
	require.NoError(t, err)
}

func TestSetValue_PINAcceptsEmpty(t *testing.T) {
	setupTest(t)
	pinPattern := `^\d{4}$`
	config.Register(config.Definition{
		Key:        "security.ogs_device_pin",
		Label:      "PIN",
		Type:       config.FieldPassword,
		Default:    "",
		Tab:        "security",
		Category:   "auth",
		Validation: &config.ValidationRules{Pattern: &pinPattern},
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "security.ogs_device_pin", "", nil, nil)
	require.NoError(t, err)
}

// =============================================================================
// Write permission enforcement tests
// =============================================================================

func TestSetValue_PermissionDenied(t *testing.T) {
	setupTest(t)
	config.Register(config.Definition{
		Key:             "admin.setting",
		Label:           "Admin",
		Type:            config.FieldText,
		Default:         "default",
		Tab:             "test",
		Category:        "test",
		WritePermission: "config:manage",
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	// User with config:update but NOT config:manage
	err := svc.SetValue(tenantCtx(1), "admin.setting", "new", nil, []string{"config:update"})
	require.Error(t, err)

	var permErr *configSvc.PermissionDeniedError
	assert.ErrorAs(t, err, &permErr)
	assert.Equal(t, "config:manage", permErr.RequiredPermission)
}

func TestSetValue_PermissionGranted(t *testing.T) {
	setupTest(t)
	config.Register(config.Definition{
		Key:             "admin.setting",
		Label:           "Admin",
		Type:            config.FieldText,
		Default:         "default",
		Tab:             "test",
		Category:        "test",
		WritePermission: "config:manage",
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	// User WITH config:manage
	err := svc.SetValue(tenantCtx(1), "admin.setting", "new", nil, []string{"config:update", "config:manage"})
	require.NoError(t, err)
}

func TestSetValue_NilPermissions_SkipsCheck(t *testing.T) {
	setupTest(t)
	config.Register(config.Definition{
		Key:             "admin.setting",
		Label:           "Admin",
		Type:            config.FieldText,
		Default:         "default",
		Tab:             "test",
		Category:        "test",
		WritePermission: "config:manage",
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	// nil permissions = system caller, skips check
	err := svc.SetValue(tenantCtx(1), "admin.setting", "new", nil, nil)
	require.NoError(t, err)
}

func TestResetValue_PermissionDenied(t *testing.T) {
	setupTest(t)
	config.Register(config.Definition{
		Key:             "admin.setting",
		Label:           "Admin",
		Type:            config.FieldText,
		Default:         "default",
		Tab:             "test",
		Category:        "test",
		WritePermission: "config:manage",
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.ResetValue(tenantCtx(1), "admin.setting", nil, []string{"config:update"})
	require.Error(t, err)

	var permErr *configSvc.PermissionDeniedError
	assert.ErrorAs(t, err, &permErr)
}

// =============================================================================
// HasTenantOverride tests
// =============================================================================

func TestHasTenantOverride_ReturnsTrueWhenExists(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.val", config.FieldText, "default")

	repo := newMockValueRepo()
	repo.values["1:test.val"] = &config.SettingValue{
		SettingKey: "test.val",
		Value:      json.RawMessage(`"custom"`),
	}
	repo.values["1:test.val"].TenantID = 1

	svc := createService(repo, &mockAuditRepo{})

	has, err := svc.HasTenantOverride(tenantCtx(1), "test.val")
	require.NoError(t, err)
	assert.True(t, has)
}

func TestHasTenantOverride_ReturnsFalseWhenNotExists(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.val", config.FieldText, "default")

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	has, err := svc.HasTenantOverride(tenantCtx(1), "test.val")
	require.NoError(t, err)
	assert.False(t, has)
}

func TestHasTenantOverride_ReturnsFalseWithNoTenant(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.val", config.FieldText, "default")

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	has, err := svc.HasTenantOverride(context.Background(), "test.val")
	require.NoError(t, err)
	assert.False(t, has)
}

// =============================================================================
// Password masking tests
// =============================================================================

func TestGetSchema_EmptyPasswordNotMasked(t *testing.T) {
	setupTest(t)

	config.Register(config.Definition{
		Key:      "security.pin",
		Label:    "PIN",
		Type:     config.FieldPassword,
		Default:  "",
		Tab:      "security",
		Category: "auth",
	})

	// No override — empty default should NOT be masked
	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	schema, err := svc.GetSchema(tenantCtx(1), []string{})
	require.NoError(t, err)
	require.Len(t, schema.Tabs, 1)

	item := schema.Tabs[0].Categories[0].Items[0]
	assert.Equal(t, "", item.Value, "empty password should show empty, not masked")
}

// =============================================================================
// Tenant context guard tests
// =============================================================================

func TestSetValue_NoTenantContext(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.guard", config.FieldText, "default")

	svc := createService(newMockValueRepo(), &mockAuditRepo{})
	// context.Background() has no tenant
	err := svc.SetValue(context.Background(), "test.guard", "value", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tenant context")
}

func TestResetValue_NoTenantContext(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.guard", config.FieldText, "default")

	svc := createService(newMockValueRepo(), &mockAuditRepo{})
	err := svc.ResetValue(context.Background(), "test.guard", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tenant context")
}

// =============================================================================
// ResolveInt edge cases
// =============================================================================

func TestResolveInt_FractionalFloat(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.frac", config.FieldNumber, 30.5)

	svc := createService(newMockValueRepo(), &mockAuditRepo{})
	_, err := svc.ResolveInt(tenantCtx(1), "test.frac")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fractional part")
}

func TestResolveInt_WholeFloat(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.whole", config.FieldNumber, 30.0)

	svc := createService(newMockValueRepo(), &mockAuditRepo{})
	val, err := svc.ResolveInt(tenantCtx(1), "test.whole")
	require.NoError(t, err)
	assert.Equal(t, 30, val)
}

// =============================================================================
// HasTenantOverride no-tenant path
// =============================================================================

func TestHasTenantOverride_NoTenantContext(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.override", config.FieldText, "default")

	svc := createService(newMockValueRepo(), &mockAuditRepo{})
	has, err := svc.HasTenantOverride(context.Background(), "test.override")
	require.NoError(t, err)
	assert.False(t, has)
}
