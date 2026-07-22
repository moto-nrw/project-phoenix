package config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
	"time"

	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/platform"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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
	return configSvc.NewSettingsService(valueRepo, auditRepo, nil, nil, slog.Default())
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

// findSchemaItem returns the resolved setting with the given key, or nil.
func findSchemaItem(schema *configSvc.SettingsSchema, key string) *configSvc.ResolvedSetting {
	for _, tab := range schema.Tabs {
		for _, cat := range tab.Categories {
			for _, item := range cat.Items {
				if item.Key == key {
					return item
				}
			}
		}
	}
	return nil
}

// is_default must be value-based, not just override-existence-based (issue
// #1680). Boolean toggles have no reset button, so an override row equal to the
// registry default has to still report is_default=true.
func TestGetSchema_IsDefault_NoOverride(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.flag", config.FieldBoolean, false)

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	schema, err := svc.GetSchema(tenantCtx(1), nil)
	require.NoError(t, err)
	item := findSchemaItem(schema, "test.flag")
	require.NotNil(t, item)
	assert.True(t, item.IsDefault, "no override should be default")
}

func TestGetSchema_IsDefault_BooleanOverrideEqualsDefault(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.flag", config.FieldBoolean, false)

	repo := newMockValueRepo()
	repo.values["1:test.flag"] = &config.SettingValue{
		SettingKey: "test.flag",
		Value:      json.RawMessage(`false`),
	}
	repo.values["1:test.flag"].TenantID = 1

	svc := createService(repo, &mockAuditRepo{})

	schema, err := svc.GetSchema(tenantCtx(1), nil)
	require.NoError(t, err)
	item := findSchemaItem(schema, "test.flag")
	require.NotNil(t, item)
	assert.True(t, item.IsDefault, "override equal to default must report is_default")
}

func TestGetSchema_IsDefault_BooleanOverrideDiffersFromDefault(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.flag", config.FieldBoolean, false)

	repo := newMockValueRepo()
	repo.values["1:test.flag"] = &config.SettingValue{
		SettingKey: "test.flag",
		Value:      json.RawMessage(`true`),
	}
	repo.values["1:test.flag"].TenantID = 1

	svc := createService(repo, &mockAuditRepo{})

	schema, err := svc.GetSchema(tenantCtx(1), nil)
	require.NoError(t, err)
	item := findSchemaItem(schema, "test.flag")
	require.NotNil(t, item)
	assert.False(t, item.IsDefault, "override differing from default must not report is_default")
}

// Resettable settings (here: a number) keep override-existence semantics even
// when the override value equals the registry default. The value-based check is
// boolean-only (issue #1680): for everything else the reset button must stay
// available to clear an explicit override, so is_default has to report false
// while a row exists — regardless of whether the value matches the default.
func TestGetSchema_IsDefault_NumberOverrideEqualsDefault_StaysResettable(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.timeout", config.FieldNumber, 30)

	repo := newMockValueRepo()
	repo.values["1:test.timeout"] = &config.SettingValue{
		SettingKey: "test.timeout",
		Value:      json.RawMessage(`30`),
	}
	repo.values["1:test.timeout"].TenantID = 1

	svc := createService(repo, &mockAuditRepo{})

	schema, err := svc.GetSchema(tenantCtx(1), nil)
	require.NoError(t, err)
	item := findSchemaItem(schema, "test.timeout")
	require.NotNil(t, item)
	assert.False(t, item.IsDefault, "a number override must stay resettable even when it equals the default")
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

func TestGetSchema_DependsOn_UsesHiddenOperatorOnlyParent(t *testing.T) {
	setupTest(t)

	config.Register(config.Definition{
		Key:             "attendance.nfc_enabled",
		Label:           "NFC",
		Type:            config.FieldBoolean,
		Default:         false,
		Tab:             "operations",
		Category:        "attendance",
		AccessPolicy:    config.AccessOperatorOnly,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
	})
	config.Register(config.Definition{
		Key:             "security.ogs_device_pin",
		Label:           "PIN",
		Type:            config.FieldPassword,
		Default:         "1234",
		Tab:             "security",
		Category:        "devices",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		DependsOn: &config.Dependency{
			Key:       "attendance.nfc_enabled",
			Condition: "eq",
			Value:     true,
		},
	})
	config.Register(config.Definition{
		Key:             "checkout.raumwechsel_enabled",
		Label:           "Raumwechsel",
		Type:            config.FieldBoolean,
		Default:         true,
		Tab:             "devices",
		Category:        "checkout",
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		AccessPolicy:    config.AccessAdminOnly,
		DependsOn: &config.Dependency{
			Key:       "attendance.nfc_enabled",
			Condition: "eq",
			Value:     true,
		},
	})

	tenantID := int64(42)
	valueRepo := newMockValueRepo()
	valueRepo.values[valueRepo.key(tenantID, "attendance.nfc_enabled")] = &config.SettingValue{
		SettingKey: "attendance.nfc_enabled",
		Value:      json.RawMessage(`true`),
	}
	valueRepo.values[valueRepo.key(tenantID, "attendance.nfc_enabled")].TenantID = tenantID
	svc := createService(valueRepo, &mockAuditRepo{})

	schema, err := svc.GetSchema(tenantCtx(tenantID), []string{"config:read"})
	require.NoError(t, err)

	visibility := schemaVisibility(schema)
	_, parentIncluded := visibility["attendance.nfc_enabled"]
	assert.False(t, parentIncluded, "operator-only parent must not be serialized to tenant admins")
	assert.True(t, visibility["security.ogs_device_pin"], "shared child should use hidden parent for visibility")
	assert.True(t, visibility["checkout.raumwechsel_enabled"], "admin-only child should use hidden parent for visibility")
}

func TestGetSchema_DependsOn_HidesChildWhenHiddenOperatorOnlyParentFalse(t *testing.T) {
	setupTest(t)

	config.Register(config.Definition{
		Key:          "attendance.nfc_enabled",
		Label:        "NFC",
		Type:         config.FieldBoolean,
		Default:      false,
		Tab:          "operations",
		Category:     "attendance",
		AccessPolicy: config.AccessOperatorOnly,
	})
	config.Register(config.Definition{
		Key:      "operations.student_daily_checkout_time",
		Label:    "Checkout time",
		Type:     config.FieldTime,
		Default:  "",
		Tab:      "operations",
		Category: "checkout",
		DependsOn: &config.Dependency{
			Key:       "attendance.nfc_enabled",
			Condition: "eq",
			Value:     true,
		},
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	schema, err := svc.GetSchema(tenantCtx(42), []string{})
	require.NoError(t, err)

	visibility := schemaVisibility(schema)
	_, parentIncluded := visibility["attendance.nfc_enabled"]
	assert.False(t, parentIncluded, "operator-only parent must not be serialized to tenant admins")
	assert.False(t, visibility["operations.student_daily_checkout_time"], "child should hide when hidden parent is false")
}

func TestGetSchema_DependsOn_HidesGrandchildWhenParentHidden(t *testing.T) {
	setupTest(t)

	config.Register(config.Definition{
		Key:      "root.enabled",
		Label:    "Root",
		Type:     config.FieldBoolean,
		Default:  false,
		Tab:      "test",
		Category: "deps",
	})
	config.Register(config.Definition{
		Key:      "child.enabled",
		Label:    "Child",
		Type:     config.FieldBoolean,
		Default:  true,
		Tab:      "test",
		Category: "deps",
		DependsOn: &config.Dependency{
			Key:       "root.enabled",
			Condition: "eq",
			Value:     true,
		},
	})
	config.Register(config.Definition{
		Key:      "grandchild.minutes",
		Label:    "Grandchild",
		Type:     config.FieldNumber,
		Default:  15,
		Tab:      "test",
		Category: "deps",
		DependsOn: &config.Dependency{
			Key:       "child.enabled",
			Condition: "eq",
			Value:     true,
		},
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	schema, err := svc.GetSchema(tenantCtx(1), []string{})
	require.NoError(t, err)

	items := schema.Tabs[0].Categories[0].Items
	visibility := make(map[string]bool, len(items))
	for _, item := range items {
		visibility[item.Key] = item.Visible
	}

	assert.True(t, visibility["root.enabled"], "root should stay visible")
	assert.False(t, visibility["child.enabled"], "child should hide when root is false")
	assert.False(t, visibility["grandchild.minutes"], "grandchild should hide when its parent is hidden")
}

func schemaVisibility(schema *configSvc.SettingsSchema) map[string]bool {
	visibility := make(map[string]bool)
	for _, tab := range schema.Tabs {
		for _, category := range tab.Categories {
			for _, item := range category.Items {
				visibility[item.Key] = item.Visible
			}
		}
	}
	return visibility
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

func TestSetValue_DateRejectsInvalidFormat(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.date", config.FieldDate, "")

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "test.date", "01.08.2026", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected date in YYYY-MM-DD format")
}

func TestSetValue_DateAcceptsValidDate(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.date", config.FieldDate, "")

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "test.date", "2026-08-01", nil, nil)
	require.NoError(t, err)
}

func TestSetValue_DateAcceptsEmptyDate(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.date", config.FieldDate, "")

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	err := svc.SetValue(tenantCtx(1), "test.date", "", nil, nil)
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

func TestResolveString_NonStringCoercion(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.coerce", config.FieldNumber, 42)

	svc := createService(newMockValueRepo(), &mockAuditRepo{})
	val, err := svc.ResolveString(tenantCtx(1), "test.coerce")
	require.NoError(t, err)
	assert.Equal(t, "42", val)
}

func TestResolveString_NilValue(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.nilstr", config.FieldText, nil)

	svc := createService(newMockValueRepo(), &mockAuditRepo{})
	val, err := svc.ResolveString(tenantCtx(1), "test.nilstr")
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestSetValue_SelectRejectsInvalidWithMarshalable(t *testing.T) {
	setupTest(t)
	config.Register(config.Definition{
		Key:      "test.sel2",
		Label:    "Sel2",
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
	err := svc.SetValue(tenantCtx(1), "test.sel2", "c", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid option")
}

func TestHasTenantOverride_NoTenantContext(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.override", config.FieldText, "default")

	svc := createService(newMockValueRepo(), &mockAuditRepo{})
	has, err := svc.HasTenantOverride(context.Background(), "test.override")
	require.NoError(t, err)
	assert.False(t, has)
}

func TestSetValue_RequiredFieldRejectsNil(t *testing.T) {
	setupTest(t)
	config.Register(config.Definition{
		Key:        "test.required",
		Label:      "Required",
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

func TestSetValue_BoolRejectsNonBool(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.boolval", config.FieldBoolean, true)

	svc := createService(newMockValueRepo(), &mockAuditRepo{})
	err := svc.SetValue(tenantCtx(1), "test.boolval", "not a bool", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected a boolean")
}

func TestSetValue_TimeRejectsNumericValue(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.timeval", config.FieldTime, "12:00")

	svc := createService(newMockValueRepo(), &mockAuditRepo{})
	err := svc.SetValue(tenantCtx(1), "test.timeval", 123, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected a time string")
}

func TestSetValue_NumberRejectsString(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.numval", config.FieldNumber, 10)

	svc := createService(newMockValueRepo(), &mockAuditRepo{})
	err := svc.SetValue(tenantCtx(1), "test.numval", "not a number", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected a number")
}

func TestSetValue_PasswordRejectsNumeric(t *testing.T) {
	setupTest(t)
	config.Register(config.Definition{
		Key:      "test.pwd",
		Label:    "Password",
		Type:     config.FieldPassword,
		Default:  "",
		Tab:      "test",
		Category: "test",
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})
	err := svc.SetValue(tenantCtx(1), "test.pwd", 12345, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected a string")
}

func TestResolveString_UnknownKey(t *testing.T) {
	setupTest(t)

	svc := createService(newMockValueRepo(), &mockAuditRepo{})
	_, err := svc.ResolveString(tenantCtx(1), "nonexistent.key")
	require.Error(t, err)
}

// --- Mock school repository for login image tests ---

// newMockSchoolRepo wires a testpkg.SchoolRepoMock to reproduce the exact
// behavior of the old hand-rolled mockSchoolRepo: FindByID (and its
// ForShare/ForUpdate aliases) return the stored school when its ID matches,
// "school not found" otherwise, or err when set; Update stores the new
// school (mutating what subsequent FindByID calls return); Create returns
// err. All other methods keep the SchoolRepoMock zero-value defaults
// (nil, nil / 0, nil), matching the old mock's always-nil behavior.
func newMockSchoolRepo(school *platform.School, err error) *testpkg.SchoolRepoMock {
	m := &testpkg.SchoolRepoMock{}
	current := school

	findByID := func(_ context.Context, id int64) (*platform.School, error) {
		if err != nil {
			return nil, err
		}
		if current == nil || current.ID != id {
			return nil, fmt.Errorf("school not found")
		}
		return current, nil
	}

	m.FindByIDFn = findByID
	m.FindByIDForShareFn = findByID
	m.FindByIDForUpdateFn = findByID
	m.CreateFn = func(_ context.Context, _ *platform.School) error {
		return err
	}
	m.UpdateFn = func(_ context.Context, s *platform.School) error {
		if err != nil {
			return err
		}
		current = s
		return nil
	}

	return m
}

func createServiceWithSchoolRepo(
	valueRepo config.SettingValueRepository,
	auditRepo config.SettingAuditRepository,
	schoolRepo platform.SchoolRepository,
) configSvc.SettingsService {
	return configSvc.NewSettingsService(valueRepo, auditRepo, schoolRepo, nil, slog.Default())
}

func newSchool(id int64, settings string) *platform.School {
	s := &platform.School{Settings: settings}
	s.ID = id
	return s
}

// --- Login image tests ---

func TestGetLoginImageURL_NoImage(t *testing.T) {
	setupTest(t)

	schoolRepo := newMockSchoolRepo(newSchool(42, ""), nil)
	svc := createServiceWithSchoolRepo(newMockValueRepo(), &mockAuditRepo{}, schoolRepo)

	url, err := svc.GetLoginImageURL(context.Background(), 42)
	require.NoError(t, err)
	assert.Equal(t, "", url)
}

func TestGetLoginImageURL_EmptyObject(t *testing.T) {
	setupTest(t)

	schoolRepo := newMockSchoolRepo(newSchool(42, "{}"), nil)
	svc := createServiceWithSchoolRepo(newMockValueRepo(), &mockAuditRepo{}, schoolRepo)

	url, err := svc.GetLoginImageURL(context.Background(), 42)
	require.NoError(t, err)
	assert.Equal(t, "", url)
}

func TestGetLoginImageURL_NullLiteral(t *testing.T) {
	setupTest(t)

	schoolRepo := newMockSchoolRepo(newSchool(42, "null"), nil)
	svc := createServiceWithSchoolRepo(newMockValueRepo(), &mockAuditRepo{}, schoolRepo)

	url, err := svc.GetLoginImageURL(context.Background(), 42)
	require.NoError(t, err)
	assert.Equal(t, "", url)
}

func TestGetLoginImageURL_WithImage(t *testing.T) {
	setupTest(t)

	settings := `{"loginImageUrl":"/uploads/tenant/42/login.jpg","other":"value"}`
	schoolRepo := newMockSchoolRepo(newSchool(42, settings), nil)
	svc := createServiceWithSchoolRepo(newMockValueRepo(), &mockAuditRepo{}, schoolRepo)

	url, err := svc.GetLoginImageURL(context.Background(), 42)
	require.NoError(t, err)
	assert.Equal(t, "/uploads/tenant/42/login.jpg", url)
}

func TestGetLoginImageURL_InvalidTenantID(t *testing.T) {
	setupTest(t)

	schoolRepo := newMockSchoolRepo(newSchool(42, `{"loginImageUrl":"/img.jpg"}`), nil)
	svc := createServiceWithSchoolRepo(newMockValueRepo(), &mockAuditRepo{}, schoolRepo)

	url, err := svc.GetLoginImageURL(context.Background(), 0)
	require.NoError(t, err)
	assert.Equal(t, "", url)
}

func TestGetLoginImageURL_NegativeTenantID(t *testing.T) {
	setupTest(t)

	schoolRepo := newMockSchoolRepo(newSchool(42, `{"loginImageUrl":"/img.jpg"}`), nil)
	svc := createServiceWithSchoolRepo(newMockValueRepo(), &mockAuditRepo{}, schoolRepo)

	url, err := svc.GetLoginImageURL(context.Background(), -1)
	require.NoError(t, err)
	assert.Equal(t, "", url)
}

func TestGetLoginImageURL_CorruptJSON(t *testing.T) {
	setupTest(t)

	schoolRepo := newMockSchoolRepo(newSchool(42, "not json"), nil)
	svc := createServiceWithSchoolRepo(newMockValueRepo(), &mockAuditRepo{}, schoolRepo)

	_, err := svc.GetLoginImageURL(context.Background(), 42)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "corrupt school settings JSON")
}

func TestGetLoginImageURL_RepoError(t *testing.T) {
	setupTest(t)

	schoolRepo := newMockSchoolRepo(nil, fmt.Errorf("database connection lost"))
	svc := createServiceWithSchoolRepo(newMockValueRepo(), &mockAuditRepo{}, schoolRepo)

	_, err := svc.GetLoginImageURL(context.Background(), 42)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "find school")
}

func TestGetLoginImageURL_SettingsWithOtherKeysButNoImage(t *testing.T) {
	setupTest(t)

	schoolRepo := newMockSchoolRepo(newSchool(42, `{"theme":"dark","lang":"de"}`), nil)
	svc := createServiceWithSchoolRepo(newMockValueRepo(), &mockAuditRepo{}, schoolRepo)

	url, err := svc.GetLoginImageURL(context.Background(), 42)
	require.NoError(t, err)
	assert.Equal(t, "", url)
}

// --- SetLoginImageURL / ClearLoginImageURL integration tests ---
// These require a real DB because updateSchoolSetting uses tenant.WithAdminTx.

func setupLoginImageIntegrationTest(t *testing.T) (configSvc.SettingsService, *platform.School, func()) {
	t.Helper()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.TenantContext(1)

	// Create a dedicated org + school to avoid polluting shared test state.
	now := time.Now().UnixNano()
	orgRepo := platformRepo.NewOrganizationRepository(db)
	org := &platform.Organization{
		Model:  modelBase.Model{ID: now},
		Name:   fmt.Sprintf("LoginImgOrg %d", now),
		Slug:   fmt.Sprintf("loginimg-org-%d", now),
		Active: true,
	}
	require.NoError(t, orgRepo.Create(ctx, org))

	schoolRepo := platformRepo.NewSchoolRepository(db)
	school := &platform.School{
		Model:          modelBase.Model{ID: now + 1},
		OrganizationID: org.ID,
		Name:           fmt.Sprintf("LoginImgSchool %d", now),
		Slug:           fmt.Sprintf("loginimg-%d", now),
		Subdomain:      fmt.Sprintf("loginimg-%d", now),
		Active:         true,
	}
	require.NoError(t, schoolRepo.Create(ctx, school))

	svc := configSvc.NewSettingsService(
		newMockValueRepo(), &mockAuditRepo{}, schoolRepo, db, slog.Default(),
	)

	cleanup := func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, school.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, org.ID)
		_ = db.Close()
	}

	return svc, school, cleanup
}

func TestSetLoginImageURL_Success(t *testing.T) {
	svc, school, cleanup := setupLoginImageIntegrationTest(t)
	defer cleanup()

	imageURL := "/uploads/login-images/test_abc.jpg"
	oldURL, err := svc.SetLoginImageURL(context.Background(), school.ID, imageURL)
	require.NoError(t, err)
	assert.Equal(t, "", oldURL, "first set should return empty old URL")

	// Verify it was persisted
	got, err := svc.GetLoginImageURL(context.Background(), school.ID)
	require.NoError(t, err)
	assert.Equal(t, imageURL, got)
}

func TestSetLoginImageURL_ReplacesExisting(t *testing.T) {
	svc, school, cleanup := setupLoginImageIntegrationTest(t)
	defer cleanup()

	first := "/uploads/login-images/first.jpg"
	second := "/uploads/login-images/second.jpg"

	_, err := svc.SetLoginImageURL(context.Background(), school.ID, first)
	require.NoError(t, err)

	oldURL, err := svc.SetLoginImageURL(context.Background(), school.ID, second)
	require.NoError(t, err)
	assert.Equal(t, first, oldURL, "should return previous URL for cleanup")

	got, err := svc.GetLoginImageURL(context.Background(), school.ID)
	require.NoError(t, err)
	assert.Equal(t, second, got)
}

func TestSetLoginImageURL_PreservesOtherSettings(t *testing.T) {
	svc, school, cleanup := setupLoginImageIntegrationTest(t)
	defer cleanup()

	// Pre-populate the school with other settings via direct SQL
	ctx := testpkg.TenantContext(1)
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_, err := db.ExecContext(ctx,
		`UPDATE platform.schools SET settings = '{"theme":"dark","lang":"de"}' WHERE id = ?`,
		school.ID)
	require.NoError(t, err)

	imageURL := "/uploads/login-images/preserve.jpg"
	_, err = svc.SetLoginImageURL(context.Background(), school.ID, imageURL)
	require.NoError(t, err)

	// Verify other keys survived the update
	got, err := svc.GetLoginImageURL(context.Background(), school.ID)
	require.NoError(t, err)
	assert.Equal(t, imageURL, got)
}

func TestClearLoginImageURL_Success(t *testing.T) {
	svc, school, cleanup := setupLoginImageIntegrationTest(t)
	defer cleanup()

	imageURL := "/uploads/login-images/to-clear.jpg"
	_, err := svc.SetLoginImageURL(context.Background(), school.ID, imageURL)
	require.NoError(t, err)

	oldURL, err := svc.ClearLoginImageURL(context.Background(), school.ID)
	require.NoError(t, err)
	assert.Equal(t, imageURL, oldURL, "should return removed URL for cleanup")

	got, err := svc.GetLoginImageURL(context.Background(), school.ID)
	require.NoError(t, err)
	assert.Equal(t, "", got, "image should be removed")
}

func TestClearLoginImageURL_NoExistingImage(t *testing.T) {
	svc, school, cleanup := setupLoginImageIntegrationTest(t)
	defer cleanup()

	oldURL, err := svc.ClearLoginImageURL(context.Background(), school.ID)
	require.NoError(t, err)
	assert.Equal(t, "", oldURL, "clearing when no image exists should return empty")
}

func TestSetLoginImageURL_NonexistentSchool(t *testing.T) {
	svc, _, cleanup := setupLoginImageIntegrationTest(t)
	defer cleanup()

	_, err := svc.SetLoginImageURL(context.Background(), 999999999, "/uploads/test.jpg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "find school")
}

// TestSetValue_SlotListCutoffPair_CrossFieldValidation covers the #1565-review
// fix: an inverted or equal Ganztag cutoff pair passes the per-field FieldTime
// validation but must be rejected here so it can never be persisted (and then
// 500 every pickup list). A valid pair, and any pair reached in a consistent
// edit order, is accepted.
func TestSetValue_SlotListCutoffPair_CrossFieldValidation(t *testing.T) {
	registerCutoffs := func() {
		registerTestSetting(config.KeySlotListShortDayCutoff, config.FieldTime, "14:30")
		registerTestSetting(config.KeySlotListLongDayCutoff, config.FieldTime, "16:00")
	}

	t.Run("long cutoff not after short is rejected", func(t *testing.T) {
		setupTest(t)
		registerCutoffs()
		svc := createService(newMockValueRepo(), &mockAuditRepo{})

		err := svc.SetValue(tenantCtx(1), config.KeySlotListLongDayCutoff, "14:00", nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "muss nach dem kurzen Ganztag")
	})

	t.Run("equal cutoffs are rejected", func(t *testing.T) {
		setupTest(t)
		registerCutoffs()
		svc := createService(newMockValueRepo(), &mockAuditRepo{})

		err := svc.SetValue(tenantCtx(1), config.KeySlotListShortDayCutoff, "16:00", nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "muss nach dem kurzen Ganztag")
	})

	t.Run("valid pair is accepted", func(t *testing.T) {
		setupTest(t)
		registerCutoffs()
		svc := createService(newMockValueRepo(), &mockAuditRepo{})

		require.NoError(t, svc.SetValue(tenantCtx(1), config.KeySlotListLongDayCutoff, "17:00", nil, nil))
	})

	t.Run("window can be shifted later via a consistent edit order", func(t *testing.T) {
		setupTest(t)
		registerCutoffs()
		repo := newMockValueRepo()
		svc := createService(repo, &mockAuditRepo{})

		// Setting the short cutoff past the current long cutoff first is refused,
		// but setting the long cutoff first, then the short cutoff, succeeds.
		require.Error(t, svc.SetValue(tenantCtx(1), config.KeySlotListShortDayCutoff, "18:00", nil, nil))
		require.NoError(t, svc.SetValue(tenantCtx(1), config.KeySlotListLongDayCutoff, "19:00", nil, nil))
		require.NoError(t, svc.SetValue(tenantCtx(1), config.KeySlotListShortDayCutoff, "18:00", nil, nil))
	})
}
