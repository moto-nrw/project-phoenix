package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock repositories ---

type mockValueRepo struct {
	values        map[string]*config.SettingValue // key: "tenantID:settingKey"
	err           error
	findManyCalls int
	findManyKeys  [][]string // per FindByTenantAndKeys call, the requested keys
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

func (m *mockValueRepo) FindByTenantAndKeys(_ context.Context, tenantID int64, settingKeys []string) ([]*config.SettingValue, error) {
	m.findManyCalls++
	m.findManyKeys = append(m.findManyKeys, append([]string(nil), settingKeys...))
	if m.err != nil {
		return nil, m.err
	}
	requested := make(map[string]struct{}, len(settingKeys))
	for _, key := range settingKeys {
		requested[key] = struct{}{}
	}
	var result []*config.SettingValue
	for _, v := range m.values {
		if v.TenantID != tenantID {
			continue
		}
		if _, ok := requested[v.SettingKey]; !ok {
			continue
		}
		result = append(result, v)
	}
	return result, nil
}

func (m *mockValueRepo) FindByTenantsAndKeys(_ context.Context, tenantIDs []int64, settingKeys []string) ([]*config.SettingValue, error) {
	m.findManyCalls++
	if m.err != nil {
		return nil, m.err
	}
	tenants := make(map[int64]struct{}, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		tenants[tenantID] = struct{}{}
	}
	requested := make(map[string]struct{}, len(settingKeys))
	for _, key := range settingKeys {
		requested[key] = struct{}{}
	}
	var result []*config.SettingValue
	for _, value := range m.values {
		if _, ok := tenants[value.GetTenantID()]; !ok {
			continue
		}
		if _, ok := requested[value.SettingKey]; ok {
			result = append(result, value)
		}
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
	return withTestTenantID(context.Background(), tenantID)
}

type testSettingsRuntime struct{}

type testTenantIDKey struct{}

func withTestTenantID(ctx context.Context, tenantID int64) context.Context {
	return context.WithValue(ctx, testTenantIDKey{}, tenantID)
}

func (testSettingsRuntime) TenantID(ctx context.Context) int64 {
	tenantID, _ := ctx.Value(testTenantIDKey{}).(int64)
	return tenantID
}
func (testSettingsRuntime) HasTransaction(context.Context) bool { return false }
func (testSettingsRuntime) WithinTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	return fn(withTestTenantID(ctx, tenantID))
}
func (testSettingsRuntime) WithinAdmin(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (testSettingsRuntime) AcquireLock(context.Context, string, bool) error { return nil }

func createService(valueRepo config.SettingValueRepository, auditRepo config.SettingAuditRepository) SettingsService {
	return NewSettingsService(valueRepo, auditRepo, nil, testSettingsRuntime{}, slog.Default())
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

func TestResolveBoolsLoadsOverridesOnceAndFillsDefaults(t *testing.T) {
	setupTest(t)
	registerTestSetting("test.enabled", config.FieldBoolean, true)
	registerTestSetting("test.disabled", config.FieldBoolean, false)

	repo := newMockValueRepo()
	repo.values["1:test.enabled"] = &config.SettingValue{
		SettingKey: "test.enabled",
		Value:      json.RawMessage(`false`),
	}
	repo.values["1:test.enabled"].TenantID = 1
	repo.values["2:test.disabled"] = &config.SettingValue{
		SettingKey: "test.disabled",
		Value:      json.RawMessage(`true`),
	}
	repo.values["2:test.disabled"].TenantID = 2

	svc := createService(repo, &mockAuditRepo{})
	values, err := svc.ResolveBools(tenantCtx(1), []string{
		"test.enabled",
		"test.disabled",
		"test.enabled",
	})
	require.NoError(t, err)

	assert.Equal(t, map[string]bool{
		"test.enabled":  false,
		"test.disabled": false,
	}, values)
	assert.Equal(t, 1, repo.findManyCalls, "all keys must share one repository query")
}

func TestResolveBoolsFailsClosedForInvalidDefinitionsAndValues(t *testing.T) {
	t.Run("unknown key", func(t *testing.T) {
		setupTest(t)
		svc := createService(newMockValueRepo(), &mockAuditRepo{})

		_, err := svc.ResolveBools(tenantCtx(1), []string{"test.unknown"})
		require.Error(t, err)
	})

	t.Run("non-boolean default", func(t *testing.T) {
		setupTest(t)
		registerTestSetting("test.text", config.FieldText, "value")
		svc := createService(newMockValueRepo(), &mockAuditRepo{})

		_, err := svc.ResolveBools(tenantCtx(1), []string{"test.text"})
		require.Error(t, err)
	})

	t.Run("non-boolean override", func(t *testing.T) {
		setupTest(t)
		registerTestSetting("test.enabled", config.FieldBoolean, true)
		repo := newMockValueRepo()
		repo.values["1:test.enabled"] = &config.SettingValue{
			SettingKey: "test.enabled",
			Value:      json.RawMessage(`"yes"`),
		}
		repo.values["1:test.enabled"].TenantID = 1
		svc := createService(repo, &mockAuditRepo{})

		_, err := svc.ResolveBools(tenantCtx(1), []string{"test.enabled"})
		require.Error(t, err)
	})
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
func findSchemaItem(schema *SettingsSchema, key string) *ResolvedSetting {
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

func schemaVisibility(schema *SettingsSchema) map[string]bool {
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
	t.Parallel()

	inner := &DefinitionNotFoundError{Key: "test"}
	err := &SettingsError{Op: "resolve", Err: inner}

	assert.Contains(t, err.Error(), "resolve")
	assert.Contains(t, err.Error(), "test")
	assert.Equal(t, inner, err.Unwrap())
}

func TestDefinitionNotFoundError(t *testing.T) {
	t.Parallel()

	err := &DefinitionNotFoundError{Key: "missing.key"}
	assert.Contains(t, err.Error(), "missing.key")
	assert.ErrorIs(t, err, ErrDefinitionNotFound)
}

func TestInvalidValueError(t *testing.T) {
	t.Parallel()

	err := &InvalidValueError{Key: "test.key", Reason: "too small"}
	assert.Contains(t, err.Error(), "test.key")
	assert.Contains(t, err.Error(), "too small")
	assert.ErrorIs(t, err, ErrInvalidValue)
}

func TestPermissionDeniedError(t *testing.T) {
	t.Parallel()

	err := &PermissionDeniedError{Key: "admin.setting", RequiredPermission: "config:manage"}
	assert.Contains(t, err.Error(), "admin.setting")
	assert.Contains(t, err.Error(), "config:manage")
	assert.ErrorIs(t, err, ErrPermissionDenied)
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

func TestSetValue_TextPatternValidation(t *testing.T) {
	setupTest(t)
	pattern := `^\d{1,4}$`
	config.Register(config.Definition{
		Key:        "payroll.lohnart_test",
		Label:      "Lohnart",
		Type:       config.FieldText,
		Default:    "",
		Tab:        "abrechnung",
		Category:   "lohnarten",
		Validation: &config.ValidationRules{Pattern: &pattern, AllowEmpty: true},
	})

	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	for _, tc := range []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "empty remains valid", value: ""},
		{name: "numeric value matches", value: "1234"},
		{name: "letters are rejected", value: "12a", wantErr: true},
		{name: "too many digits are rejected", value: "12345", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.SetValue(tenantCtx(1), "payroll.lohnart_test", tc.value, nil, nil)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "does not match required pattern")
				return
			}
			require.NoError(t, err)
		})
	}
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

func TestSetValue_PINRejectsEmpty(t *testing.T) {
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
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match required pattern")
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

	var permErr *PermissionDeniedError
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

	var permErr *PermissionDeniedError
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

// --- Mock school-settings store for login image tests ---

type mockSchoolSettingsStore struct {
	id       int64
	settings string
	err      error
}

func newMockSchoolRepo(school *mockSchoolSettingsStore, err error) *mockSchoolSettingsStore {
	if school == nil {
		school = &mockSchoolSettingsStore{}
	}
	school.err = err
	return school
}

func (s *mockSchoolSettingsStore) FindSettings(_ context.Context, schoolID int64) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if s.id != schoolID {
		return "", fmt.Errorf("school not found")
	}
	return s.settings, nil
}

func (s *mockSchoolSettingsStore) UpdateSettings(_ context.Context, schoolID int64, update func(string) (string, error)) error {
	settings, err := s.FindSettings(context.Background(), schoolID)
	if err != nil {
		return err
	}
	s.settings, err = update(settings)
	return err
}

func createServiceWithSchoolRepo(
	valueRepo config.SettingValueRepository,
	auditRepo config.SettingAuditRepository,
	schoolRepo SchoolSettingsStore,
) SettingsService {
	return NewSettingsService(valueRepo, auditRepo, schoolRepo, testSettingsRuntime{}, slog.Default())
}

func newSchool(id int64, settings string) *mockSchoolSettingsStore {
	return &mockSchoolSettingsStore{id: id, settings: settings}
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

// TestResetValue_SlotListCutoffPair_CrossFieldValidation covers the #1565-review
// fix: resetting a Ganztag cutoff restores its registry default, which can
// invert the effective pair even though SetValue guards it. The reset must be
// validated the same way, or it 500s every pickup list until repaired.
func TestResetValue_SlotListCutoffPair_CrossFieldValidation(t *testing.T) {
	registerCutoffs := func() {
		registerTestSetting(config.KeySlotListShortDayCutoff, config.FieldTime, "14:30")
		registerTestSetting(config.KeySlotListLongDayCutoff, config.FieldTime, "16:00")
	}

	t.Run("reset that inverts the effective pair is rejected", func(t *testing.T) {
		setupTest(t)
		registerCutoffs()
		repo := newMockValueRepo()
		svc := createService(repo, &mockAuditRepo{})

		// Valid overrides: short 18:00, long 19:00.
		require.NoError(t, svc.SetValue(tenantCtx(1), config.KeySlotListLongDayCutoff, "19:00", nil, nil))
		require.NoError(t, svc.SetValue(tenantCtx(1), config.KeySlotListShortDayCutoff, "18:00", nil, nil))

		// Resetting the long cutoff would restore its 16:00 default, leaving the
		// short cutoff at 18:00 — an inverted pair. It must be refused, and the
		// override must survive.
		err := svc.ResetValue(tenantCtx(1), config.KeySlotListLongDayCutoff, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "muss nach dem kurzen Ganztag")

		long, err := svc.ResolveString(tenantCtx(1), config.KeySlotListLongDayCutoff)
		require.NoError(t, err)
		assert.Equal(t, "19:00", long, "rejected reset must not delete the override")
	})

	t.Run("reset that keeps a valid pair is accepted", func(t *testing.T) {
		setupTest(t)
		registerCutoffs()
		repo := newMockValueRepo()
		svc := createService(repo, &mockAuditRepo{})

		// Valid overrides: short 18:00, long 19:00. Resetting the short cutoff
		// restores 14:30, which is still before the long 19:00 → allowed.
		require.NoError(t, svc.SetValue(tenantCtx(1), config.KeySlotListLongDayCutoff, "19:00", nil, nil))
		require.NoError(t, svc.SetValue(tenantCtx(1), config.KeySlotListShortDayCutoff, "18:00", nil, nil))

		require.NoError(t, svc.ResetValue(tenantCtx(1), config.KeySlotListShortDayCutoff, nil, nil))

		short, err := svc.ResolveString(tenantCtx(1), config.KeySlotListShortDayCutoff)
		require.NoError(t, err)
		assert.Equal(t, "14:30", short, "accepted reset falls back to the registry default")
	})
}

// setClassRestrictionGuard wires the enrollment class-restriction probe on a
// settings service via the exported setter on the concrete type (reachable
// from this external test package through an interface assertion, the same
// way the factory wires it).
func setClassRestrictionGuard(t *testing.T, svc SettingsService, restricted bool) {
	t.Helper()
	guarded, ok := svc.(interface {
		SetClassRestrictionGuard(func(context.Context) (bool, error))
	})
	require.True(t, ok, "settings service must expose SetClassRestrictionGuard")
	guarded.SetClassRestrictionGuard(func(context.Context) (bool, error) { return restricted, nil })
}

// TestSetValue_ClassCollectionGuard_CrossFieldValidation covers the #1663 fix:
// disabling either concrete-class collection toggle must be refused while the
// tenant has an active phase that restricts eligibility to specific classes —
// otherwise validateAndNormalizeSchoolClasses erases every child's class and
// the eligibility gate rejects every submission with class_not_eligible. This
// is the inverse of the phase-side validateEligibleClassesCollectable guard.
func TestSetValue_ClassCollectionGuard_CrossFieldValidation(t *testing.T) {
	registerCollectionKeys := func() {
		registerTestSetting(config.KeyEnrollmentCollectGradeLevel, config.FieldBoolean, true)
		registerTestSetting(config.KeyEnrollmentCollectSchoolClass, config.FieldBoolean, true)
	}

	t.Run("disabling concrete-class collection is rejected while a restricted phase exists", func(t *testing.T) {
		setupTest(t)
		registerCollectionKeys()
		svc := createService(newMockValueRepo(), &mockAuditRepo{})
		setClassRestrictionGuard(t, svc, true)

		err := svc.SetValue(tenantCtx(1), config.KeyEnrollmentCollectSchoolClass, false, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Klassen-Abfrage")
	})

	t.Run("disabling grade-level collection is likewise rejected", func(t *testing.T) {
		setupTest(t)
		registerCollectionKeys()
		svc := createService(newMockValueRepo(), &mockAuditRepo{})
		setClassRestrictionGuard(t, svc, true)

		err := svc.SetValue(tenantCtx(1), config.KeyEnrollmentCollectGradeLevel, false, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Klassen-Abfrage")
	})

	t.Run("keeping collection effective is allowed even with a restricted phase", func(t *testing.T) {
		setupTest(t)
		registerCollectionKeys()
		svc := createService(newMockValueRepo(), &mockAuditRepo{})
		setClassRestrictionGuard(t, svc, true)

		require.NoError(t, svc.SetValue(tenantCtx(1), config.KeyEnrollmentCollectSchoolClass, true, nil, nil))
	})

	t.Run("disabling is allowed when no restricted phase exists", func(t *testing.T) {
		setupTest(t)
		registerCollectionKeys()
		svc := createService(newMockValueRepo(), &mockAuditRepo{})
		setClassRestrictionGuard(t, svc, false)

		require.NoError(t, svc.SetValue(tenantCtx(1), config.KeyEnrollmentCollectSchoolClass, false, nil, nil))
	})

	t.Run("no guard wired skips the check", func(t *testing.T) {
		setupTest(t)
		registerCollectionKeys()
		svc := createService(newMockValueRepo(), &mockAuditRepo{})

		require.NoError(t, svc.SetValue(tenantCtx(1), config.KeyEnrollmentCollectSchoolClass, false, nil, nil))
	})
}

// TestResetValue_ClassCollectionGuard_CrossFieldValidation covers the reset
// direction of the #1663 guard: resetting a collection toggle restores its
// registry default, which can disable collection even though SetValue guards
// it. The reset must be validated the same way and the override must survive.
func TestResetValue_ClassCollectionGuard_CrossFieldValidation(t *testing.T) {
	setupTest(t)
	// grade default true, class default false: resetting class restores false,
	// which disables concrete-class collection.
	registerTestSetting(config.KeyEnrollmentCollectGradeLevel, config.FieldBoolean, true)
	registerTestSetting(config.KeyEnrollmentCollectSchoolClass, config.FieldBoolean, false)
	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	// Enable the class override first (no restricted phase yet, so allowed).
	setClassRestrictionGuard(t, svc, false)
	require.NoError(t, svc.SetValue(tenantCtx(1), config.KeyEnrollmentCollectSchoolClass, true, nil, nil))

	// A restricted phase now exists; resetting class back to its false default
	// would disable collection and must be refused, leaving the override intact.
	setClassRestrictionGuard(t, svc, true)
	err := svc.ResetValue(tenantCtx(1), config.KeyEnrollmentCollectSchoolClass, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Klassen-Abfrage")

	got, err := svc.ResolveBool(tenantCtx(1), config.KeyEnrollmentCollectSchoolClass)
	require.NoError(t, err)
	assert.True(t, got, "rejected reset must not delete the override")
}

// setGradeRestrictionGuard wires the enrollment grade-restriction probe, the
// counterpart of setClassRestrictionGuard above (#1663).
func setGradeRestrictionGuard(t *testing.T, svc SettingsService, restricted bool) {
	t.Helper()
	guarded, ok := svc.(interface {
		SetGradeRestrictionGuard(func(context.Context) (bool, error))
	})
	require.True(t, ok, "settings service must expose SetGradeRestrictionGuard")
	guarded.SetGradeRestrictionGuard(func(context.Context) (bool, error) { return restricted, nil })
}

// TestSetValue_GradeCollectionGuard_CrossFieldValidation covers the grade-level
// half of the #1663 guard. A phase restricted to whole grades depends on
// collect_grade_level ALONE — it needs no concrete classes — so disabling that
// toggle must be refused, while disabling concrete-class collection stays
// allowed for such a phase.
func TestSetValue_GradeCollectionGuard_CrossFieldValidation(t *testing.T) {
	registerCollectionKeys := func() {
		registerTestSetting(config.KeyEnrollmentCollectGradeLevel, config.FieldBoolean, true)
		registerTestSetting(config.KeyEnrollmentCollectSchoolClass, config.FieldBoolean, true)
	}

	t.Run("disabling grade-level collection is rejected while a grade-restricted phase exists", func(t *testing.T) {
		setupTest(t)
		registerCollectionKeys()
		svc := createService(newMockValueRepo(), &mockAuditRepo{})
		setGradeRestrictionGuard(t, svc, true)

		err := svc.SetValue(tenantCtx(1), config.KeyEnrollmentCollectGradeLevel, false, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Klassenstufen-Abfrage")
	})

	t.Run("disabling concrete-class collection stays allowed for a grade-restricted phase", func(t *testing.T) {
		setupTest(t)
		registerCollectionKeys()
		svc := createService(newMockValueRepo(), &mockAuditRepo{})
		setGradeRestrictionGuard(t, svc, true)

		require.NoError(t, svc.SetValue(tenantCtx(1), config.KeyEnrollmentCollectSchoolClass, false, nil, nil),
			"a whole-grade phase does not depend on concrete classes")
	})

	t.Run("enabling grade-level collection is never blocked", func(t *testing.T) {
		setupTest(t)
		registerCollectionKeys()
		svc := createService(newMockValueRepo(), &mockAuditRepo{})
		setGradeRestrictionGuard(t, svc, true)

		require.NoError(t, svc.SetValue(tenantCtx(1), config.KeyEnrollmentCollectGradeLevel, true, nil, nil))
	})

	t.Run("disabling is allowed when no grade-restricted phase exists", func(t *testing.T) {
		setupTest(t)
		registerCollectionKeys()
		svc := createService(newMockValueRepo(), &mockAuditRepo{})
		setGradeRestrictionGuard(t, svc, false)

		require.NoError(t, svc.SetValue(tenantCtx(1), config.KeyEnrollmentCollectGradeLevel, false, nil, nil))
	})
}

// setGradeCapGuard wires the highest-restricted-grade probe, the third #1663
// enrollment probe next to setClassRestrictionGuard / setGradeRestrictionGuard.
func setGradeCapGuard(t *testing.T, svc SettingsService, highest int) {
	t.Helper()
	guarded, ok := svc.(interface {
		SetGradeCapGuard(func(context.Context) (int, error))
	})
	require.True(t, ok, "settings service must expose SetGradeCapGuard")
	guarded.SetGradeCapGuard(func(context.Context) (int, error) { return highest, nil })
}

// TestSetValue_GradeLevelCapGuard_CrossFieldValidation covers lowering
// enrollment.grade_level_max below a grade an active phase already restricts
// itself to. The form offers grades 1..cap and submit re-checks the cap, so
// such a phase would accept no submission at all (#1663).
func TestSetValue_GradeLevelCapGuard_CrossFieldValidation(t *testing.T) {
	registerCap := func() {
		registerTestSetting(config.KeyEnrollmentGradeLevelMax, config.FieldNumber, 4)
	}

	t.Run("lowering the cap below a live grade restriction is rejected", func(t *testing.T) {
		setupTest(t)
		registerCap()
		svc := createService(newMockValueRepo(), &mockAuditRepo{})
		setGradeCapGuard(t, svc, 6)

		err := svc.SetValue(tenantCtx(1), config.KeyEnrollmentGradeLevelMax, 4, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Klassenstufe 6")
	})

	t.Run("lowering to exactly the restricted grade is allowed", func(t *testing.T) {
		setupTest(t)
		registerCap()
		svc := createService(newMockValueRepo(), &mockAuditRepo{})
		setGradeCapGuard(t, svc, 6)

		require.NoError(t, svc.SetValue(tenantCtx(1), config.KeyEnrollmentGradeLevelMax, 6, nil, nil),
			"the restricted grade itself is still offered at cap == grade")
	})

	t.Run("raising the cap is never blocked", func(t *testing.T) {
		setupTest(t)
		registerCap()
		svc := createService(newMockValueRepo(), &mockAuditRepo{})
		setGradeCapGuard(t, svc, 6)

		require.NoError(t, svc.SetValue(tenantCtx(1), config.KeyEnrollmentGradeLevelMax, 10, nil, nil))
	})

	t.Run("no grade-restricted phase leaves the cap free", func(t *testing.T) {
		setupTest(t)
		registerCap()
		svc := createService(newMockValueRepo(), &mockAuditRepo{})
		setGradeCapGuard(t, svc, 0)

		require.NoError(t, svc.SetValue(tenantCtx(1), config.KeyEnrollmentGradeLevelMax, 1, nil, nil))
	})
}

// A reset restores the registry default (4), which lowers the cap just as a
// SetValue would — so it must be validated the same way and the override must
// survive the rejection.
func TestResetValue_GradeLevelCapGuard_CrossFieldValidation(t *testing.T) {
	setupTest(t)
	registerTestSetting(config.KeyEnrollmentGradeLevelMax, config.FieldNumber, 4)
	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	// Raise the cap to 6 first (no restricted phase yet, so allowed).
	setGradeCapGuard(t, svc, 0)
	require.NoError(t, svc.SetValue(tenantCtx(1), config.KeyEnrollmentGradeLevelMax, 6, nil, nil))

	// A phase restricted to grade 6 now exists; resetting back to the default 4
	// must be refused, leaving the override intact.
	setGradeCapGuard(t, svc, 6)
	err := svc.ResetValue(tenantCtx(1), config.KeyEnrollmentGradeLevelMax, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Klassenstufe 6")

	got, err := svc.ResolveInt(tenantCtx(1), config.KeyEnrollmentGradeLevelMax)
	require.NoError(t, err)
	assert.Equal(t, 6, got, "rejected reset must not delete the override")
}
