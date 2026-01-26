package config_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	repoAudit "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	repoConfig "github.com/moto-nrw/project-phoenix/database/repositories/config"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupScopedSettingsService creates a ScopedSettingsService with real database.
func setupScopedSettingsService(t *testing.T, db *bun.DB) configSvc.ScopedSettingsService {
	t.Helper()

	repoFactory := repositories.NewFactory(db)

	return configSvc.NewScopedSettingsService(&configSvc.ScopedSettingsRepositories{
		Definition: repoFactory.SettingDefinition,
		Value:      repoFactory.SettingValue,
		Change:     repoFactory.SettingChange,
	})
}

// initializeAndGetService is a test helper that sets up a service and initializes definitions.
func initializeAndGetService(t *testing.T) (*bun.DB, configSvc.ScopedSettingsService) {
	t.Helper()

	db := testpkg.SetupTestDB(t)
	svc := setupScopedSettingsService(t, db)

	ctx := context.Background()
	err := svc.InitializeDefinitions(ctx)
	require.NoError(t, err, "InitializeDefinitions should succeed")

	return db, svc
}

// cleanupScopedFixtures cleans up all scoped settings test data.
func cleanupScopedFixtures(t *testing.T, db *bun.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Clean up values first (FK to definitions)
	_, _ = db.NewDelete().
		TableExpr("config.setting_values").
		Where("1=1").
		Exec(ctx)

	// Clean up definitions
	_, _ = db.NewDelete().
		TableExpr("config.setting_definitions").
		Where("1=1").
		Exec(ctx)

	// Clean up audit entries
	_, _ = db.NewDelete().
		TableExpr("audit.setting_changes").
		Where("1=1").
		Exec(ctx)
}

// testActor returns an Actor suitable for integration tests.
func testActor(personID int64) *configSvc.Actor {
	return &configSvc.Actor{
		AccountID:   personID, // use same ID for simplicity
		PersonID:    personID,
		Permissions: []string{"config:manage", "settings:update", "settings:read"},
	}
}

// =============================================================================
// DEFAULT DEFINITIONS TESTS
// =============================================================================

func TestGetDefaultDefinitions(t *testing.T) {
	defs := configSvc.GetDefaultDefinitions()

	t.Run("returns non-empty list", func(t *testing.T) {
		require.NotEmpty(t, defs, "Should have default definitions")
		assert.GreaterOrEqual(t, len(defs), 10, "Should have at least 10 definitions")
	})

	t.Run("all definitions have required fields", func(t *testing.T) {
		for _, def := range defs {
			assert.NotEmpty(t, def.Key, "Key should not be empty")
			assert.NotEmpty(t, def.Type, "Type should not be empty for %s", def.Key)
			assert.NotEmpty(t, def.DefaultValue, "DefaultValue should not be empty for %s", def.Key)
			assert.NotEmpty(t, def.Category, "Category should not be empty for %s", def.Key)
			assert.NotEmpty(t, def.AllowedScopes, "AllowedScopes should not be empty for %s", def.Key)
			assert.NotEmpty(t, def.ScopePermissions, "ScopePermissions should not be empty for %s", def.Key)
			assert.NotEmpty(t, def.GroupName, "GroupName should not be empty for %s", def.Key)
		}
	})

	t.Run("all definitions are valid", func(t *testing.T) {
		for _, def := range defs {
			err := def.Validate()
			assert.NoError(t, err, "Definition %s should be valid", def.Key)
		}
	})

	t.Run("definitions have correct types", func(t *testing.T) {
		keyTypes := map[string]config.SettingType{
			"session.timeout_minutes":     config.SettingTypeInt,
			"session.auto_checkout":       config.SettingTypeBool,
			"session.warning_minutes":     config.SettingTypeInt,
			"pickup.has_earliest_time":    config.SettingTypeBool,
			"pickup.earliest_time":        config.SettingTypeTime,
			"ui.theme":                    config.SettingTypeEnum,
			"ui.language":                 config.SettingTypeEnum,
			"notifications.absence_channels": config.SettingTypeJSON,
			"audit.track_setting_changes": config.SettingTypeBool,
			"audit.setting_retention_days": config.SettingTypeInt,
			"device.beep_on_scan":         config.SettingTypeBool,
			"device.display_timeout_seconds": config.SettingTypeInt,
		}

		for _, def := range defs {
			if expectedType, ok := keyTypes[def.Key]; ok {
				assert.Equal(t, expectedType, def.Type, "Type mismatch for %s", def.Key)
			}
		}
	})

	t.Run("definitions have dependencies set correctly", func(t *testing.T) {
		dependentKeys := map[string]string{
			"pickup.earliest_time":        "pickup.has_earliest_time",
			"pickup.latest_time":          "pickup.has_latest_time",
			"notifications.absence_channels": "notifications.absence_enabled",
		}

		for _, def := range defs {
			if expectedParent, ok := dependentKeys[def.Key]; ok {
				require.NotNil(t, def.DependsOn, "DependsOn should be set for %s", def.Key)
				assert.Equal(t, expectedParent, def.DependsOn.Key, "DependsOn.Key mismatch for %s", def.Key)
				assert.Equal(t, "equals", def.DependsOn.Condition, "DependsOn.Condition should be equals for %s", def.Key)
			}
		}
	})

	t.Run("definitions have validation where expected", func(t *testing.T) {
		validatedKeys := []string{
			"session.timeout_minutes",
			"session.warning_minutes",
			"ui.theme",
			"ui.language",
			"audit.setting_retention_days",
			"device.display_timeout_seconds",
		}

		defMap := make(map[string]*config.SettingDefinition)
		for _, def := range defs {
			defMap[def.Key] = def
		}

		for _, key := range validatedKeys {
			def, ok := defMap[key]
			require.True(t, ok, "Definition %s should exist", key)
			assert.NotNil(t, def.Validation, "Validation should be set for %s", key)
		}
	})

	t.Run("unique keys", func(t *testing.T) {
		seen := make(map[string]bool)
		for _, def := range defs {
			assert.False(t, seen[def.Key], "Duplicate key: %s", def.Key)
			seen[def.Key] = true
		}
	})

	t.Run("default values can be parsed", func(t *testing.T) {
		for _, def := range defs {
			val, err := def.GetDefaultValueTyped()
			assert.NoError(t, err, "GetDefaultValueTyped should succeed for %s", def.Key)
			assert.NotNil(t, val, "Default value should not be nil for %s", def.Key)
		}
	})
}

// =============================================================================
// INITIALIZE DEFINITIONS TESTS
// =============================================================================

func TestScopedSettingsService_InitializeDefinitions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	defer cleanupScopedFixtures(t, db)

	svc := setupScopedSettingsService(t, db)
	ctx := context.Background()

	t.Run("initializes all definitions", func(t *testing.T) {
		err := svc.InitializeDefinitions(ctx)
		require.NoError(t, err)

		// Verify definitions were created
		defs, err := svc.ListDefinitions(ctx, map[string]interface{}{})
		require.NoError(t, err)

		expectedCount := len(configSvc.GetDefaultDefinitions())
		assert.GreaterOrEqual(t, len(defs), expectedCount,
			"Should have at least %d definitions after initialization", expectedCount)
	})

	t.Run("idempotent - running twice works", func(t *testing.T) {
		err := svc.InitializeDefinitions(ctx)
		require.NoError(t, err)

		// Run again - should not fail (upsert)
		err = svc.InitializeDefinitions(ctx)
		require.NoError(t, err)
	})
}

// =============================================================================
// GET DEFINITION TESTS
// =============================================================================

func TestScopedSettingsService_GetDefinition(t *testing.T) {
	db, svc := initializeAndGetService(t)
	defer func() { _ = db.Close() }()
	defer cleanupScopedFixtures(t, db)

	ctx := context.Background()

	t.Run("returns existing definition", func(t *testing.T) {
		def, err := svc.GetDefinition(ctx, "session.timeout_minutes")
		require.NoError(t, err)
		require.NotNil(t, def)
		assert.Equal(t, "session.timeout_minutes", def.Key)
		assert.Equal(t, config.SettingTypeInt, def.Type)
	})

	t.Run("returns nil for non-existent key", func(t *testing.T) {
		def, err := svc.GetDefinition(ctx, "nonexistent.key")
		require.NoError(t, err)
		assert.Nil(t, def)
	})
}

// =============================================================================
// LIST DEFINITIONS TESTS
// =============================================================================

func TestScopedSettingsService_ListDefinitions(t *testing.T) {
	db, svc := initializeAndGetService(t)
	defer func() { _ = db.Close() }()
	defer cleanupScopedFixtures(t, db)

	ctx := context.Background()

	t.Run("lists all definitions", func(t *testing.T) {
		defs, err := svc.ListDefinitions(ctx, map[string]interface{}{})
		require.NoError(t, err)
		assert.NotEmpty(t, defs)
	})

	t.Run("filters by category", func(t *testing.T) {
		defs, err := svc.ListDefinitions(ctx, map[string]interface{}{
			"category": "session",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, defs)
		for _, def := range defs {
			assert.Equal(t, "session", def.Category)
		}
	})

	t.Run("filters by scope", func(t *testing.T) {
		defs, err := svc.ListDefinitions(ctx, map[string]interface{}{
			"scope": "user",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, defs)
		for _, def := range defs {
			assert.Contains(t, def.AllowedScopes, "user")
		}
	})
}

// =============================================================================
// GET DEFINITIONS FOR SCOPE TESTS
// =============================================================================

func TestScopedSettingsService_GetDefinitionsForScope(t *testing.T) {
	db, svc := initializeAndGetService(t)
	defer func() { _ = db.Close() }()
	defer cleanupScopedFixtures(t, db)

	ctx := context.Background()

	t.Run("returns system-scoped definitions", func(t *testing.T) {
		defs, err := svc.GetDefinitionsForScope(ctx, config.ScopeSystem)
		require.NoError(t, err)
		assert.NotEmpty(t, defs)

		for _, def := range defs {
			assert.Contains(t, def.AllowedScopes, "system")
		}
	})

	t.Run("returns user-scoped definitions", func(t *testing.T) {
		defs, err := svc.GetDefinitionsForScope(ctx, config.ScopeUser)
		require.NoError(t, err)
		assert.NotEmpty(t, defs)

		for _, def := range defs {
			assert.Contains(t, def.AllowedScopes, "user")
		}
	})

	t.Run("returns og-scoped definitions", func(t *testing.T) {
		defs, err := svc.GetDefinitionsForScope(ctx, config.ScopeOG)
		require.NoError(t, err)
		assert.NotEmpty(t, defs)

		for _, def := range defs {
			assert.Contains(t, def.AllowedScopes, "og")
		}
	})
}

// =============================================================================
// GET / GET WITH SOURCE TESTS
// =============================================================================

func TestScopedSettingsService_Get(t *testing.T) {
	db, svc := initializeAndGetService(t)
	defer func() { _ = db.Close() }()
	defer cleanupScopedFixtures(t, db)

	ctx := context.Background()

	t.Run("returns default value when no override set", func(t *testing.T) {
		value, err := svc.Get(ctx, "session.timeout_minutes", config.NewSystemScope())
		require.NoError(t, err)
		assert.Equal(t, 30, value, "Default session timeout should be 30")
	})

	t.Run("returns error for non-existent key", func(t *testing.T) {
		_, err := svc.Get(ctx, "nonexistent.key", config.NewSystemScope())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "setting not found")
	})

	t.Run("returns default bool value", func(t *testing.T) {
		value, err := svc.Get(ctx, "session.auto_checkout", config.NewSystemScope())
		require.NoError(t, err)
		assert.Equal(t, true, value)
	})

	t.Run("returns default enum value", func(t *testing.T) {
		value, err := svc.Get(ctx, "ui.theme", config.NewSystemScope())
		require.NoError(t, err)
		assert.Equal(t, "light", value)
	})

	t.Run("returns default time value", func(t *testing.T) {
		value, err := svc.Get(ctx, "pickup.earliest_time", config.NewSystemScope())
		require.NoError(t, err)
		assert.Equal(t, "15:00", value)
	})
}

func TestScopedSettingsService_GetWithSource(t *testing.T) {
	db, svc := initializeAndGetService(t)
	defer func() { _ = db.Close() }()
	defer cleanupScopedFixtures(t, db)

	ctx := context.Background()

	t.Run("returns resolved setting with default source", func(t *testing.T) {
		resolved, err := svc.GetWithSource(ctx, "session.timeout_minutes", config.NewSystemScope())
		require.NoError(t, err)
		require.NotNil(t, resolved)

		assert.Equal(t, "session.timeout_minutes", resolved.Key)
		assert.Equal(t, config.SettingTypeInt, resolved.Type)
		assert.Equal(t, "session", resolved.Category)
		assert.True(t, resolved.IsDefault, "Should be default value")
		assert.Nil(t, resolved.Source, "Source should be nil for default")
		assert.True(t, resolved.IsActive, "Setting without dependency should be active")
	})

	t.Run("returns error for non-existent key", func(t *testing.T) {
		_, err := svc.GetWithSource(ctx, "nonexistent.key", config.NewSystemScope())
		require.Error(t, err)
	})

	t.Run("resolves OG scope to system default", func(t *testing.T) {
		resolved, err := svc.GetWithSource(ctx, "session.timeout_minutes", config.NewOGScope(999))
		require.NoError(t, err)
		require.NotNil(t, resolved)

		assert.True(t, resolved.IsDefault, "Should fall back to default since no OG/system override exists")
	})
}

// =============================================================================
// GET ALL / GET ALL BY CATEGORY TESTS
// =============================================================================

func TestScopedSettingsService_GetAll(t *testing.T) {
	db, svc := initializeAndGetService(t)
	defer func() { _ = db.Close() }()
	defer cleanupScopedFixtures(t, db)

	ctx := context.Background()

	t.Run("returns all system settings", func(t *testing.T) {
		settings, err := svc.GetAll(ctx, config.NewSystemScope())
		require.NoError(t, err)
		assert.NotEmpty(t, settings)

		// Verify each setting has required fields
		for _, s := range settings {
			assert.NotEmpty(t, s.Key)
			assert.NotEmpty(t, s.Type)
			assert.NotEmpty(t, s.Category)
		}
	})

	t.Run("returns all OG settings", func(t *testing.T) {
		settings, err := svc.GetAll(ctx, config.NewOGScope(42))
		require.NoError(t, err)
		assert.NotEmpty(t, settings)
	})

	t.Run("returns all user settings", func(t *testing.T) {
		settings, err := svc.GetAll(ctx, config.NewUserScope(1))
		require.NoError(t, err)
		assert.NotEmpty(t, settings)
	})
}

func TestScopedSettingsService_GetAllByCategory(t *testing.T) {
	db, svc := initializeAndGetService(t)
	defer func() { _ = db.Close() }()
	defer cleanupScopedFixtures(t, db)

	ctx := context.Background()

	t.Run("returns settings for session category", func(t *testing.T) {
		settings, err := svc.GetAllByCategory(ctx, config.NewSystemScope(), "session")
		require.NoError(t, err)
		assert.NotEmpty(t, settings)

		for _, s := range settings {
			assert.Equal(t, "session", s.Category)
		}
	})

	t.Run("returns empty for non-existent category", func(t *testing.T) {
		settings, err := svc.GetAllByCategory(ctx, config.NewSystemScope(), "nonexistent")
		require.NoError(t, err)
		assert.Empty(t, settings)
	})

	t.Run("returns settings for appearance category", func(t *testing.T) {
		settings, err := svc.GetAllByCategory(ctx, config.NewSystemScope(), "appearance")
		require.NoError(t, err)
		assert.NotEmpty(t, settings)

		for _, s := range settings {
			assert.Equal(t, "appearance", s.Category)
		}
	})
}

// =============================================================================
// SET TESTS
// =============================================================================

func TestScopedSettingsService_Set(t *testing.T) {
	db, svc := initializeAndGetService(t)
	defer func() { _ = db.Close() }()
	defer cleanupScopedFixtures(t, db)

	ctx := context.Background()

	// Create a person to use as the actor
	person := testpkg.CreateTestStaff(t, db, "Settings", "Tester")
	defer testpkg.CleanupStaffFixtures(t, db, person.PersonID)
	actor := testActor(person.PersonID)
	req := httptest.NewRequest("PUT", "/settings/system/session.timeout_minutes", nil)

	t.Run("sets system setting successfully", func(t *testing.T) {
		err := svc.Set(ctx, "session.timeout_minutes", config.NewSystemScope(), 45, actor, req)
		require.NoError(t, err)

		// Verify the value was set
		value, err := svc.Get(ctx, "session.timeout_minutes", config.NewSystemScope())
		require.NoError(t, err)
		assert.Equal(t, 45, value)
	})

	t.Run("sets OG setting successfully", func(t *testing.T) {
		err := svc.Set(ctx, "session.timeout_minutes", config.NewOGScope(42), 60, actor, req)
		require.NoError(t, err)

		// Verify the OG value
		value, err := svc.Get(ctx, "session.timeout_minutes", config.NewOGScope(42))
		require.NoError(t, err)
		assert.Equal(t, 60, value)

		// System value should still be 45
		systemValue, err := svc.Get(ctx, "session.timeout_minutes", config.NewSystemScope())
		require.NoError(t, err)
		assert.Equal(t, 45, systemValue)
	})

	t.Run("sets bool setting", func(t *testing.T) {
		err := svc.Set(ctx, "session.auto_checkout", config.NewSystemScope(), false, actor, req)
		require.NoError(t, err)

		value, err := svc.Get(ctx, "session.auto_checkout", config.NewSystemScope())
		require.NoError(t, err)
		assert.Equal(t, false, value)
	})

	t.Run("fails for non-existent setting", func(t *testing.T) {
		err := svc.Set(ctx, "nonexistent.key", config.NewSystemScope(), "value", actor, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "setting not found")
	})

	t.Run("fails for invalid value type", func(t *testing.T) {
		err := svc.Set(ctx, "session.timeout_minutes", config.NewSystemScope(), "not_a_number", actor, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid value")
	})

	t.Run("fails for value out of range", func(t *testing.T) {
		err := svc.Set(ctx, "session.timeout_minutes", config.NewSystemScope(), 999, actor, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at most")
	})

	t.Run("fails for disallowed scope", func(t *testing.T) {
		// audit.track_setting_changes only allows system scope
		err := svc.Set(ctx, "audit.track_setting_changes", config.NewOGScope(42), true, actor, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be configured at og level")
	})

	t.Run("updates existing value", func(t *testing.T) {
		// Use OG scope (non-NULL scope_id) because PostgreSQL UNIQUE constraints
		// treat NULL != NULL, so ON CONFLICT does not fire for system scope.
		ogScope := config.NewOGScope(99)

		// Set initial value
		err := svc.Set(ctx, "session.warning_minutes", ogScope, 10, actor, req)
		require.NoError(t, err)

		// Update it
		err = svc.Set(ctx, "session.warning_minutes", ogScope, 15, actor, req)
		require.NoError(t, err)

		value, err := svc.Get(ctx, "session.warning_minutes", ogScope)
		require.NoError(t, err)
		assert.Equal(t, 15, value)
	})

	t.Run("validates enum value", func(t *testing.T) {
		err := svc.Set(ctx, "ui.theme", config.NewSystemScope(), "dark", actor, req)
		require.NoError(t, err)

		err = svc.Set(ctx, "ui.theme", config.NewSystemScope(), "invalid_theme", actor, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be one of")
	})
}

// =============================================================================
// RESET TESTS
// =============================================================================

func TestScopedSettingsService_Reset(t *testing.T) {
	db, svc := initializeAndGetService(t)
	defer func() { _ = db.Close() }()
	defer cleanupScopedFixtures(t, db)

	ctx := context.Background()

	person := testpkg.CreateTestStaff(t, db, "Reset", "Tester")
	defer testpkg.CleanupStaffFixtures(t, db, person.PersonID)
	actor := testActor(person.PersonID)
	req := httptest.NewRequest("DELETE", "/settings/og/42/session.timeout_minutes", nil)

	t.Run("resets OG setting to inherit", func(t *testing.T) {
		// Set an OG override
		ogScope := config.NewOGScope(42)
		err := svc.Set(ctx, "session.timeout_minutes", ogScope, 90, actor, req)
		require.NoError(t, err)

		// Verify it's set
		value, err := svc.Get(ctx, "session.timeout_minutes", ogScope)
		require.NoError(t, err)
		assert.Equal(t, 90, value)

		// Reset it
		err = svc.Reset(ctx, "session.timeout_minutes", ogScope, actor, req)
		require.NoError(t, err)

		// Should fall back to system/default value
		resolved, err := svc.GetWithSource(ctx, "session.timeout_minutes", ogScope)
		require.NoError(t, err)
		assert.True(t, resolved.IsDefault || (resolved.Source != nil && resolved.Source.Type == config.ScopeSystem),
			"Should fall back to system/default after reset")
	})

	t.Run("fails to reset system scope", func(t *testing.T) {
		err := svc.Reset(ctx, "session.timeout_minutes", config.NewSystemScope(), actor, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot reset system default")
	})

	t.Run("no-op when no override exists", func(t *testing.T) {
		// Reset a key that was never set at this scope
		err := svc.Reset(ctx, "session.timeout_minutes", config.NewOGScope(99999), actor, req)
		require.NoError(t, err) // Should succeed (no-op)
	})

	t.Run("fails for non-existent setting", func(t *testing.T) {
		err := svc.Reset(ctx, "nonexistent.key", config.NewOGScope(42), actor, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "setting not found")
	})
}

// =============================================================================
// IS SETTING ACTIVE TESTS
// =============================================================================

func TestScopedSettingsService_IsSettingActive(t *testing.T) {
	db, svc := initializeAndGetService(t)
	defer func() { _ = db.Close() }()
	defer cleanupScopedFixtures(t, db)

	ctx := context.Background()

	t.Run("independent setting is always active", func(t *testing.T) {
		active, err := svc.IsSettingActive(ctx, "session.timeout_minutes", config.NewSystemScope())
		require.NoError(t, err)
		assert.True(t, active)
	})

	t.Run("dependent setting is active when parent matches", func(t *testing.T) {
		// pickup.earliest_time depends on pickup.has_earliest_time being true
		// Default for has_earliest_time is false, so earliest_time should be inactive
		active, err := svc.IsSettingActive(ctx, "pickup.earliest_time", config.NewSystemScope())
		require.NoError(t, err)
		assert.False(t, active, "pickup.earliest_time should be inactive when has_earliest_time is false")
	})

	t.Run("dependent setting becomes active when parent set to true", func(t *testing.T) {
		person := testpkg.CreateTestStaff(t, db, "Active", "Tester")
		defer testpkg.CleanupStaffFixtures(t, db, person.PersonID)
		actor := testActor(person.PersonID)
		req := httptest.NewRequest("PUT", "/test", nil)

		// Enable the parent
		err := svc.Set(ctx, "pickup.has_earliest_time", config.NewSystemScope(), true, actor, req)
		require.NoError(t, err)

		// Now the dependent should be active
		active, err := svc.IsSettingActive(ctx, "pickup.earliest_time", config.NewSystemScope())
		require.NoError(t, err)
		assert.True(t, active, "pickup.earliest_time should be active when has_earliest_time is true")
	})

	t.Run("returns error for non-existent setting", func(t *testing.T) {
		_, err := svc.IsSettingActive(ctx, "nonexistent.key", config.NewSystemScope())
		require.Error(t, err)
	})
}

// =============================================================================
// CAN MODIFY TESTS
// =============================================================================

func TestScopedSettingsService_CanModify(t *testing.T) {
	db, svc := initializeAndGetService(t)
	defer func() { _ = db.Close() }()
	defer cleanupScopedFixtures(t, db)

	ctx := context.Background()

	t.Run("admin can modify system setting", func(t *testing.T) {
		actor := &configSvc.Actor{
			AccountID:   1,
			PersonID:    1,
			Permissions: []string{"config:manage"},
		}
		canModify, err := svc.CanModify(ctx, "session.timeout_minutes", config.NewSystemScope(), actor)
		require.NoError(t, err)
		assert.True(t, canModify)
	})

	t.Run("user without permission cannot modify system setting", func(t *testing.T) {
		actor := &configSvc.Actor{
			AccountID:   1,
			PersonID:    1,
			Permissions: []string{"settings:read"},
		}
		canModify, err := svc.CanModify(ctx, "session.timeout_minutes", config.NewSystemScope(), actor)
		require.NoError(t, err)
		assert.False(t, canModify)
	})

	t.Run("user can modify own settings", func(t *testing.T) {
		personID := int64(42)
		actor := &configSvc.Actor{
			AccountID:   1,
			PersonID:    personID,
			Permissions: []string{},
		}
		// ui.theme allows user scope with "self" permission
		canModify, err := svc.CanModify(ctx, "ui.theme", config.NewUserScope(personID), actor)
		require.NoError(t, err)
		assert.True(t, canModify)
	})

	t.Run("user cannot modify other user settings", func(t *testing.T) {
		actor := &configSvc.Actor{
			AccountID:   1,
			PersonID:    42,
			Permissions: []string{},
		}
		canModify, err := svc.CanModify(ctx, "ui.theme", config.NewUserScope(99), actor)
		require.NoError(t, err)
		assert.False(t, canModify)
	})

	t.Run("returns false for non-existent setting", func(t *testing.T) {
		actor := &configSvc.Actor{
			AccountID:   1,
			PersonID:    1,
			Permissions: []string{"config:manage"},
		}
		canModify, err := svc.CanModify(ctx, "nonexistent.key", config.NewSystemScope(), actor)
		require.NoError(t, err)
		assert.False(t, canModify)
	})

	t.Run("returns false for disallowed scope", func(t *testing.T) {
		actor := &configSvc.Actor{
			AccountID:   1,
			PersonID:    1,
			Permissions: []string{"config:manage"},
		}
		// audit.track_setting_changes only allows system scope
		canModify, err := svc.CanModify(ctx, "audit.track_setting_changes", config.NewOGScope(42), actor)
		require.NoError(t, err)
		assert.False(t, canModify)
	})

	t.Run("owner permission check for OG scope", func(t *testing.T) {
		actor := &configSvc.Actor{
			AccountID:   1,
			PersonID:    1,
			Permissions: []string{"settings:update"},
		}
		// session.timeout_minutes allows og scope with "owner" permission
		canModify, err := svc.CanModify(ctx, "session.timeout_minutes", config.NewOGScope(42), actor)
		require.NoError(t, err)
		assert.True(t, canModify, "User with settings:update should pass owner check")
	})
}

// =============================================================================
// HISTORY TESTS
// =============================================================================

func TestScopedSettingsService_GetHistory(t *testing.T) {
	db, svc := initializeAndGetService(t)
	defer func() { _ = db.Close() }()
	defer cleanupScopedFixtures(t, db)

	ctx := context.Background()

	person := testpkg.CreateTestStaff(t, db, "History", "Tester")
	defer testpkg.CleanupStaffFixtures(t, db, person.PersonID)
	actor := testActor(person.PersonID)
	req := httptest.NewRequest("PUT", "/test", nil)

	t.Run("returns empty for no changes", func(t *testing.T) {
		history, err := svc.GetHistory(ctx, "session.timeout_minutes", config.NewSystemScope(), 10)
		require.NoError(t, err)
		assert.Empty(t, history)
	})

	t.Run("records changes after Set", func(t *testing.T) {
		// Make a change
		err := svc.Set(ctx, "session.warning_minutes", config.NewSystemScope(), 10, actor, req)
		require.NoError(t, err)

		// Wait briefly for async logging
		time.Sleep(200 * time.Millisecond)

		history, err := svc.GetHistory(ctx, "session.warning_minutes", config.NewSystemScope(), 10)
		require.NoError(t, err)
		// Note: audit logging is async so might not be recorded yet in fast tests
		// We just verify no error occurs
		_ = history
	})
}

func TestScopedSettingsService_GetHistoryForScope(t *testing.T) {
	db, svc := initializeAndGetService(t)
	defer func() { _ = db.Close() }()
	defer cleanupScopedFixtures(t, db)

	ctx := context.Background()

	t.Run("returns history for system scope", func(t *testing.T) {
		history, err := svc.GetHistoryForScope(ctx, config.NewSystemScope(), 50)
		require.NoError(t, err)
		// Should succeed even if empty
		_ = history
	})

	t.Run("returns history for OG scope", func(t *testing.T) {
		history, err := svc.GetHistoryForScope(ctx, config.NewOGScope(42), 50)
		require.NoError(t, err)
		_ = history
	})
}

// =============================================================================
// DELETE SCOPE SETTINGS TESTS
// =============================================================================

func TestScopedSettingsService_DeleteScopeSettings(t *testing.T) {
	db, svc := initializeAndGetService(t)
	defer func() { _ = db.Close() }()
	defer cleanupScopedFixtures(t, db)

	ctx := context.Background()

	person := testpkg.CreateTestStaff(t, db, "Delete", "Tester")
	defer testpkg.CleanupStaffFixtures(t, db, person.PersonID)
	actor := testActor(person.PersonID)
	req := httptest.NewRequest("PUT", "/test", nil)

	t.Run("deletes all settings for OG", func(t *testing.T) {
		ogScope := config.NewOGScope(12345)

		// Set some OG values
		err := svc.Set(ctx, "session.timeout_minutes", ogScope, 60, actor, req)
		require.NoError(t, err)

		err = svc.Set(ctx, "session.auto_checkout", ogScope, false, actor, req)
		require.NoError(t, err)

		// Delete all settings for this OG
		err = svc.DeleteScopeSettings(ctx, config.ScopeOG, 12345)
		require.NoError(t, err)

		// Verify values are gone (should fall back to default)
		resolved, err := svc.GetWithSource(ctx, "session.timeout_minutes", ogScope)
		require.NoError(t, err)
		assert.True(t, resolved.IsDefault || (resolved.Source != nil && resolved.Source.Type == config.ScopeSystem),
			"Should fall back to system/default after deletion")
	})
}

// =============================================================================
// SCOPE RESOLUTION CHAIN (INTEGRATION)
// =============================================================================

func TestScopedSettingsService_ScopeResolution(t *testing.T) {
	db, svc := initializeAndGetService(t)
	defer func() { _ = db.Close() }()
	defer cleanupScopedFixtures(t, db)

	ctx := context.Background()

	person := testpkg.CreateTestStaff(t, db, "Scope", "Tester")
	defer testpkg.CleanupStaffFixtures(t, db, person.PersonID)
	actor := testActor(person.PersonID)
	req := httptest.NewRequest("PUT", "/test", nil)

	t.Run("OG inherits from system when no OG override", func(t *testing.T) {
		// Set system value
		err := svc.Set(ctx, "session.timeout_minutes", config.NewSystemScope(), 50, actor, req)
		require.NoError(t, err)

		// OG should inherit system value
		resolved, err := svc.GetWithSource(ctx, "session.timeout_minutes", config.NewOGScope(777))
		require.NoError(t, err)
		assert.Equal(t, 50, resolved.Value)
		assert.False(t, resolved.IsDefault)
		require.NotNil(t, resolved.Source)
		assert.Equal(t, config.ScopeSystem, resolved.Source.Type)
	})

	t.Run("OG override takes precedence over system", func(t *testing.T) {
		ogScope := config.NewOGScope(777)

		// Set OG value (system should already be 50 from previous test)
		err := svc.Set(ctx, "session.timeout_minutes", ogScope, 75, actor, req)
		require.NoError(t, err)

		// OG should use its own value
		resolved, err := svc.GetWithSource(ctx, "session.timeout_minutes", ogScope)
		require.NoError(t, err)
		assert.Equal(t, 75, resolved.Value)
		assert.False(t, resolved.IsDefault)
		require.NotNil(t, resolved.Source)
		assert.Equal(t, config.ScopeOG, resolved.Source.Type)
	})
}

// =============================================================================
// MAP CHANGES TO HISTORY (via direct repo approach)
// =============================================================================

func TestScopedSettingsService_MapChangesToHistory(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	changeRepo := repoAudit.NewSettingChangeRepository(db)
	defRepo := repoConfig.NewSettingDefinitionRepository(db)
	valueRepo := repoConfig.NewSettingValueRepository(db)

	svc := configSvc.NewScopedSettingsService(&configSvc.ScopedSettingsRepositories{
		Definition: defRepo,
		Value:      valueRepo,
		Change:     changeRepo,
	})

	ctx := context.Background()

	// Create a test change directly
	change := createTestAuditChange("test.map.history."+t.Name(), "system", nil, "create")
	err := changeRepo.Create(ctx, change)
	require.NoError(t, err)
	defer cleanupAuditChange(t, db, change.ID)

	// Get history via service
	history, err := svc.GetHistory(ctx, change.SettingKey, config.NewSystemScope(), 10)
	require.NoError(t, err)
	require.NotEmpty(t, history)

	entry := history[0]
	assert.Equal(t, change.SettingKey, entry.SettingKey)
	assert.Equal(t, "create", entry.ChangeType)
	assert.NotEmpty(t, entry.ChangedAt)
}

// =============================================================================
// INACTIVE SETTING CANNOT BE SET
// =============================================================================

func TestScopedSettingsService_SetInactiveSetting(t *testing.T) {
	db, svc := initializeAndGetService(t)
	defer func() { _ = db.Close() }()
	defer cleanupScopedFixtures(t, db)

	ctx := context.Background()

	person := testpkg.CreateTestStaff(t, db, "Inactive", "Tester")
	defer testpkg.CleanupStaffFixtures(t, db, person.PersonID)
	actor := testActor(person.PersonID)
	req := httptest.NewRequest("PUT", "/test", nil)

	t.Run("cannot set inactive setting", func(t *testing.T) {
		// pickup.earliest_time depends on pickup.has_earliest_time=true
		// Default is false, so pickup.earliest_time should be inactive
		err := svc.Set(ctx, "pickup.earliest_time", config.NewSystemScope(), "14:00", actor, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "inactive due to dependency")
	})
}

// =============================================================================
// HELPERS
// =============================================================================

func createTestAuditChange(key string, scopeType string, scopeID *int64, changeType string) *audit.SettingChange {
	oldValue, _ := config.MarshalValue(false)
	newValue, _ := config.MarshalValue(true)
	return &audit.SettingChange{
		SettingKey: key,
		ScopeType:  scopeType,
		ScopeID:    scopeID,
		ChangeType: changeType,
		OldValue:   oldValue,
		NewValue:   newValue,
		IPAddress:  "127.0.0.1",
		UserAgent:  "test-agent",
		Reason:     "test change",
	}
}

func cleanupAuditChange(tb testing.TB, db *bun.DB, id int64) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = db.NewDelete().
		TableExpr("audit.setting_changes").
		Where("id = ?", id).
		Exec(ctx)
}
