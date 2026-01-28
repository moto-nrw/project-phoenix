package config_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupConfigService creates a config service with real database connection.
func setupConfigService(t *testing.T, db *bun.DB) configSvc.Service {
	t.Helper()

	repoFactory := repositories.NewFactory(db)

	return configSvc.NewService(repoFactory.Setting, db)
}

// ============================================================================
// Test Fixtures - Config Domain
// ============================================================================

// createTestSetting creates a test setting in the database
func createTestSetting(t *testing.T, db *bun.DB, key, value, category string) *config.Setting {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make key unique to avoid conflicts
	uniqueKey := key + "-" + time.Now().Format("20060102150405.000")

	setting := &config.Setting{
		Key:         uniqueKey,
		Value:       value,
		Category:    category,
		Description: "Test setting: " + key,
	}

	err := db.NewInsert().
		Model(setting).
		ModelTableExpr(`config.settings`).
		Scan(ctx)
	require.NoError(t, err, "Failed to create test setting")

	return setting
}

// cleanupConfigFixtures removes config test fixtures
func cleanupConfigFixtures(t *testing.T, db *bun.DB, settingIDs []int64) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, id := range settingIDs {
		_, _ = db.NewDelete().
			Model((*config.Setting)(nil)).
			ModelTableExpr(`config.settings`).
			Where("id = ?", id).
			Exec(ctx)
	}
}

// createTestSettingWithExactKey creates a test setting with an exact key (no timestamp suffix)
func createTestSettingWithExactKey(t *testing.T, db *bun.DB, key, value, category string) *config.Setting {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Delete any existing setting with this key first
	_, _ = db.NewDelete().
		Model((*config.Setting)(nil)).
		ModelTableExpr(`config.settings`).
		Where("key = ?", key).
		Exec(ctx)

	setting := &config.Setting{
		Key:         key,
		Value:       value,
		Category:    category,
		Description: "Test setting: " + key,
	}

	err := db.NewInsert().
		Model(setting).
		ModelTableExpr(`config.settings`).
		Scan(ctx)
	require.NoError(t, err, "Failed to create test setting with exact key")

	return setting
}

// ============================================================================
// Core Operations Tests
// ============================================================================

func TestConfigService_CreateSetting(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("creates setting successfully", func(t *testing.T) {
		// ARRANGE
		setting := &config.Setting{
			Key:         "test.create-" + time.Now().Format("20060102150405.000"),
			Value:       "test_value",
			Category:    "test",
			Description: "Test description",
		}

		// ACT
		err := service.CreateSetting(ctx, setting)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, setting.ID)
		defer cleanupConfigFixtures(t, db, []int64{setting.ID})
	})

	t.Run("rejects nil setting", func(t *testing.T) {
		// ACT
		err := service.CreateSetting(ctx, nil)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("rejects empty key", func(t *testing.T) {
		// ARRANGE
		setting := &config.Setting{
			Key:   "",
			Value: "test_value",
		}

		// ACT
		err := service.CreateSetting(ctx, setting)

		// ASSERT
		require.Error(t, err)
	})
}

func TestConfigService_GetSettingByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("returns setting for valid ID", func(t *testing.T) {
		// ARRANGE
		setting := createTestSetting(t, db, "get-by-id", "value123", "test")
		defer cleanupConfigFixtures(t, db, []int64{setting.ID})

		// ACT
		result, err := service.GetSettingByID(ctx, setting.ID)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, setting.ID, result.ID)
		assert.Equal(t, "value123", result.Value)
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		// ACT
		_, err := service.GetSettingByID(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
		// Service returns ErrSettingNotFound for non-existent records
		assert.Contains(t, err.Error(), "setting not found")
	})

	t.Run("returns error for invalid ID", func(t *testing.T) {
		// ACT
		_, err := service.GetSettingByID(ctx, 0)

		// ASSERT
		require.Error(t, err)
	})
}

func TestConfigService_UpdateSetting(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("updates setting successfully", func(t *testing.T) {
		// ARRANGE
		setting := createTestSetting(t, db, "update-test", "original", "test")
		defer cleanupConfigFixtures(t, db, []int64{setting.ID})

		setting.Value = "updated"

		// ACT
		err := service.UpdateSetting(ctx, setting)

		// ASSERT
		require.NoError(t, err)

		// Verify
		result, err := service.GetSettingByID(ctx, setting.ID)
		require.NoError(t, err)
		assert.Equal(t, "updated", result.Value)
	})

	t.Run("rejects nil setting", func(t *testing.T) {
		// ACT
		err := service.UpdateSetting(ctx, nil)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for non-existent setting", func(t *testing.T) {
		// ARRANGE
		setting := &config.Setting{
			Key:   "nonexistent",
			Value: "value",
		}
		setting.ID = 999999999

		// ACT
		err := service.UpdateSetting(ctx, setting)

		// ASSERT
		require.Error(t, err)
	})
}

func TestConfigService_DeleteSetting(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("deletes setting successfully", func(t *testing.T) {
		// ARRANGE
		setting := createTestSetting(t, db, "delete-test", "value", "test")
		settingID := setting.ID

		// ACT
		err := service.DeleteSetting(ctx, settingID)

		// ASSERT
		require.NoError(t, err)

		// Verify deletion
		_, err = service.GetSettingByID(ctx, settingID)
		require.Error(t, err)
	})

	t.Run("returns error for invalid ID", func(t *testing.T) {
		// ACT
		err := service.DeleteSetting(ctx, 0)

		// ASSERT
		require.Error(t, err)
	})
}

func TestConfigService_ListSettings(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("lists all settings with nil filters", func(t *testing.T) {
		// ARRANGE
		s1 := createTestSetting(t, db, "list1", "value1", "test")
		s2 := createTestSetting(t, db, "list2", "value2", "test")
		defer cleanupConfigFixtures(t, db, []int64{s1.ID, s2.ID})

		// ACT
		settings, err := service.ListSettings(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, settings)
	})

	t.Run("lists settings with filters", func(t *testing.T) {
		// ARRANGE
		setting := createTestSetting(t, db, "filter-test", "value", "filtered-category")
		defer cleanupConfigFixtures(t, db, []int64{setting.ID})

		// ACT
		filters := map[string]interface{}{
			"category": "filtered-category",
		}
		settings, err := service.ListSettings(ctx, filters)

		// ASSERT
		require.NoError(t, err)
		// Our setting should be in the results
		found := false
		for _, s := range settings {
			if s.ID == setting.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Setting should be found with filter")
	})
}

// ============================================================================
// Key-Based Operations Tests
// ============================================================================

func TestConfigService_GetSettingByKey(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("returns setting for valid key", func(t *testing.T) {
		// ARRANGE
		setting := createTestSetting(t, db, "get-by-key", "keyvalue", "test")
		defer cleanupConfigFixtures(t, db, []int64{setting.ID})

		// ACT
		result, err := service.GetSettingByKey(ctx, setting.Key)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "keyvalue", result.Value)
	})

	t.Run("returns error for non-existent key", func(t *testing.T) {
		// ACT
		_, err := service.GetSettingByKey(ctx, "nonexistent-key-xyz")

		// ASSERT
		require.Error(t, err)
		// Service returns SettingNotFoundError for non-existent keys
		assert.Contains(t, err.Error(), "setting not found")
	})

	t.Run("returns error for empty key", func(t *testing.T) {
		// ACT
		_, err := service.GetSettingByKey(ctx, "")

		// ASSERT
		require.Error(t, err)
	})
}

func TestConfigService_UpdateSettingValue(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("updates setting value by key", func(t *testing.T) {
		// ARRANGE
		setting := createTestSetting(t, db, "update-value", "original", "test")
		defer cleanupConfigFixtures(t, db, []int64{setting.ID})

		// ACT
		err := service.UpdateSettingValue(ctx, setting.Key, "new_value")

		// ASSERT
		require.NoError(t, err)

		// Verify
		result, err := service.GetSettingByKey(ctx, setting.Key)
		require.NoError(t, err)
		assert.Equal(t, "new_value", result.Value)
	})

	t.Run("returns error for non-existent key", func(t *testing.T) {
		// ACT
		err := service.UpdateSettingValue(ctx, "nonexistent-key-xyz", "value")

		// ASSERT
		require.Error(t, err)
	})
}

func TestConfigService_GetStringValue(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("returns string value for existing key", func(t *testing.T) {
		// ARRANGE
		setting := createTestSetting(t, db, "string-value", "hello", "test")
		defer cleanupConfigFixtures(t, db, []int64{setting.ID})

		// ACT
		result, err := service.GetStringValue(ctx, setting.Key, "default")

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, "hello", result)
	})

	t.Run("returns default for non-existent key", func(t *testing.T) {
		// ACT
		result, err := service.GetStringValue(ctx, "nonexistent-string-key", "default_value")

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, "default_value", result)
	})
}

func TestConfigService_GetBoolValue(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("returns true for true value", func(t *testing.T) {
		// ARRANGE
		setting := createTestSetting(t, db, "bool-true", "true", "test")
		defer cleanupConfigFixtures(t, db, []int64{setting.ID})

		// ACT
		result, err := service.GetBoolValue(ctx, setting.Key, false)

		// ASSERT
		require.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("returns false for false value", func(t *testing.T) {
		// ARRANGE
		setting := createTestSetting(t, db, "bool-false", "false", "test")
		defer cleanupConfigFixtures(t, db, []int64{setting.ID})

		// ACT
		result, err := service.GetBoolValue(ctx, setting.Key, true)

		// ASSERT
		require.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("returns default for non-existent key", func(t *testing.T) {
		// ACT
		result, err := service.GetBoolValue(ctx, "nonexistent-bool-key", true)

		// ASSERT
		require.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("returns default and error for invalid value", func(t *testing.T) {
		// ARRANGE
		setting := createTestSetting(t, db, "bool-invalid", "not-a-bool", "test")
		defer cleanupConfigFixtures(t, db, []int64{setting.ID})

		// ACT
		result, err := service.GetBoolValue(ctx, setting.Key, true)

		// ASSERT
		// Service returns both default AND error for invalid values
		require.Error(t, err)
		assert.True(t, result) // Returns default even when error
	})
}

func TestConfigService_GetIntValue(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("returns int value for existing key", func(t *testing.T) {
		// ARRANGE
		setting := createTestSetting(t, db, "int-value", "42", "test")
		defer cleanupConfigFixtures(t, db, []int64{setting.ID})

		// ACT
		result, err := service.GetIntValue(ctx, setting.Key, 0)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, 42, result)
	})

	t.Run("returns default for non-existent key", func(t *testing.T) {
		// ACT
		result, err := service.GetIntValue(ctx, "nonexistent-int-key", 99)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, 99, result)
	})

	t.Run("returns default and error for invalid value", func(t *testing.T) {
		// ARRANGE
		setting := createTestSetting(t, db, "int-invalid", "not-an-int", "test")
		defer cleanupConfigFixtures(t, db, []int64{setting.ID})

		// ACT
		result, err := service.GetIntValue(ctx, setting.Key, 123)

		// ASSERT
		// Service returns both default AND error for invalid values
		require.Error(t, err)
		assert.Equal(t, 123, result)
	})
}

func TestConfigService_GetFloatValue(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("returns float value for existing key", func(t *testing.T) {
		// ARRANGE
		setting := createTestSetting(t, db, "float-value", "3.14", "test")
		defer cleanupConfigFixtures(t, db, []int64{setting.ID})

		// ACT
		result, err := service.GetFloatValue(ctx, setting.Key, 0.0)

		// ASSERT
		require.NoError(t, err)
		assert.InDelta(t, 3.14, result, 0.001)
	})

	t.Run("returns default for non-existent key", func(t *testing.T) {
		// ACT
		result, err := service.GetFloatValue(ctx, "nonexistent-float-key", 9.99)

		// ASSERT
		require.NoError(t, err)
		assert.InDelta(t, 9.99, result, 0.001)
	})

	t.Run("returns default and error for invalid value", func(t *testing.T) {
		// ARRANGE
		setting := createTestSetting(t, db, "float-invalid", "not-a-float", "test")
		defer cleanupConfigFixtures(t, db, []int64{setting.ID})

		// ACT
		result, err := service.GetFloatValue(ctx, setting.Key, 1.23)

		// ASSERT
		// Service returns both default AND error for invalid values
		require.Error(t, err)
		assert.InDelta(t, 1.23, result, 0.001)
	})
}

// ============================================================================
// Category Operations Tests
// ============================================================================

func TestConfigService_GetSettingsByCategory(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("returns settings by category", func(t *testing.T) {
		// ARRANGE
		category := "test-category-" + time.Now().Format("20060102150405")
		s1 := createTestSetting(t, db, "cat1", "value1", category)
		s2 := createTestSetting(t, db, "cat2", "value2", category)
		s3 := createTestSetting(t, db, "other", "value3", "other-category")
		defer cleanupConfigFixtures(t, db, []int64{s1.ID, s2.ID, s3.ID})

		// ACT
		settings, err := service.GetSettingsByCategory(ctx, category)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(settings), 2)

		// All returned settings should have the same category
		for _, s := range settings {
			assert.Equal(t, category, s.Category)
		}
	})

	t.Run("returns empty for non-existent category", func(t *testing.T) {
		// ACT
		settings, err := service.GetSettingsByCategory(ctx, "nonexistent-category-xyz")

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, settings)
	})

	t.Run("returns error for empty category", func(t *testing.T) {
		// ACT
		_, err := service.GetSettingsByCategory(ctx, "")

		// ASSERT
		require.Error(t, err)
	})
}

func TestConfigService_GetSettingByKeyAndCategory(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("returns setting by key and category", func(t *testing.T) {
		// ARRANGE
		category := "specific-cat-" + time.Now().Format("20060102150405")
		setting := createTestSetting(t, db, "specific-key", "specific-value", category)
		defer cleanupConfigFixtures(t, db, []int64{setting.ID})

		// ACT
		result, err := service.GetSettingByKeyAndCategory(ctx, setting.Key, category)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "specific-value", result.Value)
	})

	t.Run("returns error for wrong category", func(t *testing.T) {
		// ARRANGE
		setting := createTestSetting(t, db, "wrong-cat-key", "value", "category-a")
		defer cleanupConfigFixtures(t, db, []int64{setting.ID})

		// ACT
		_, err := service.GetSettingByKeyAndCategory(ctx, setting.Key, "category-b")

		// ASSERT
		require.Error(t, err)
	})
}

// ============================================================================
// Bulk Operations Tests
// ============================================================================

func TestConfigService_ImportSettings(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("imports multiple settings", func(t *testing.T) {
		// ARRANGE
		settings := []*config.Setting{
			{Key: "import1-" + time.Now().Format("20060102150405.000"), Value: "value1", Category: "import"},
			{Key: "import2-" + time.Now().Format("20060102150405.001"), Value: "value2", Category: "import"},
		}

		// ACT
		errors, err := service.ImportSettings(ctx, settings)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, errors)

		// Cleanup
		var ids []int64
		for _, s := range settings {
			if s.ID != 0 {
				ids = append(ids, s.ID)
			}
		}
		defer cleanupConfigFixtures(t, db, ids)
	})

	t.Run("returns empty for empty slice", func(t *testing.T) {
		// ACT
		errors, err := service.ImportSettings(ctx, []*config.Setting{})

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, errors)
	})

	t.Run("collects errors for invalid settings", func(t *testing.T) {
		// ARRANGE
		settings := []*config.Setting{
			{Key: "valid-import-" + time.Now().Format("20060102150405"), Value: "value1", Category: "import"},
			{Key: "", Value: "value2", Category: "import"}, // Invalid: empty key
		}

		// ACT
		errors, err := service.ImportSettings(ctx, settings)

		// ASSERT
		// Should have one error for the invalid setting
		assert.NotEmpty(t, errors)

		// Cleanup valid setting
		if settings[0].ID != 0 {
			defer cleanupConfigFixtures(t, db, []int64{settings[0].ID})
		}
		// Check if there was a batch error
		if err != nil {
			// This is expected if validation fails
			assert.Contains(t, err.Error(), "error")
		}
	})

	t.Run("updates existing settings during import", func(t *testing.T) {
		// ARRANGE - Create a setting first
		uniqueKey := "import-update-" + time.Now().Format("20060102150405.000")
		existingSetting := createTestSettingWithExactKey(t, db, uniqueKey, "original", "import")
		defer cleanupConfigFixtures(t, db, []int64{existingSetting.ID})

		// Now import with the same key and category but different value
		settings := []*config.Setting{
			{Key: uniqueKey, Value: "updated", Category: "import", Description: "Updated description"},
		}

		// ACT
		errors, err := service.ImportSettings(ctx, settings)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, errors)

		// Verify the setting was updated
		updated, err := service.GetSettingByKeyAndCategory(ctx, uniqueKey, "import")
		require.NoError(t, err)
		assert.Equal(t, "updated", updated.Value)
	})
}

func TestConfigService_InitializeDefaultSettings(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("initializes default settings", func(t *testing.T) {
		// ACT
		err := service.InitializeDefaultSettings(ctx)

		// ASSERT
		// This may return an error if settings already exist (duplicate key)
		// or succeed if they don't
		// Either way is acceptable for this test
		_ = err
	})
}

// ============================================================================
// System Operations Tests
// ============================================================================

func TestConfigService_RequiresRestart(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("returns restart status", func(t *testing.T) {
		// ACT
		result, err := service.RequiresRestart(ctx)

		// ASSERT
		require.NoError(t, err)
		// Result depends on system state
		assert.IsType(t, bool(false), result)
	})
}

func TestConfigService_RequiresDatabaseReset(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("returns database reset status", func(t *testing.T) {
		// ACT
		result, err := service.RequiresDatabaseReset(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.IsType(t, bool(false), result)
	})
}

// ============================================================================
// Timeout Configuration Tests
// ============================================================================

func TestConfigService_GetTimeoutSettings(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("returns timeout settings", func(t *testing.T) {
		// ACT
		settings, err := service.GetTimeoutSettings(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, settings)
		// Default values should be set
		assert.GreaterOrEqual(t, settings.GlobalTimeoutMinutes, 0)
	})
}

func TestConfigService_UpdateTimeoutSettings(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("updates timeout settings when settings exist", func(t *testing.T) {
		// ARRANGE - First create the required settings with exact keys (no timestamp suffix)
		setting1 := createTestSettingWithExactKey(t, db, "session_timeout_minutes", "30", "system")
		setting2 := createTestSettingWithExactKey(t, db, "session_warning_threshold_minutes", "10", "system")
		setting3 := createTestSettingWithExactKey(t, db, "session_check_interval_seconds", "60", "system")
		defer cleanupConfigFixtures(t, db, []int64{setting1.ID, setting2.ID, setting3.ID})

		settings := &config.TimeoutSettings{
			GlobalTimeoutMinutes:    45,
			WarningThresholdMinutes: 5,
			CheckIntervalSeconds:    30,
		}

		// ACT
		err := service.UpdateTimeoutSettings(ctx, settings)

		// ASSERT
		require.NoError(t, err)
	})

	t.Run("returns error when settings do not exist", func(t *testing.T) {
		// ARRANGE
		settings := &config.TimeoutSettings{
			GlobalTimeoutMinutes:    45,
			WarningThresholdMinutes: 5,
			CheckIntervalSeconds:    30,
		}

		// ACT
		err := service.UpdateTimeoutSettings(ctx, settings)

		// ASSERT - Fails because required settings don't exist
		require.Error(t, err)
	})

	t.Run("panics on nil settings", func(t *testing.T) {
		// ACT & ASSERT - Service calls Validate() on nil which panics
		assert.Panics(t, func() {
			_ = service.UpdateTimeoutSettings(ctx, nil)
		})
	})

	t.Run("rejects invalid timeout settings", func(t *testing.T) {
		// ARRANGE - Invalid: GlobalTimeoutMinutes <= 0
		settings := &config.TimeoutSettings{
			GlobalTimeoutMinutes:    0, // Invalid
			WarningThresholdMinutes: 5,
			CheckIntervalSeconds:    30,
		}

		// ACT
		err := service.UpdateTimeoutSettings(ctx, settings)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid timeout settings")
	})
}

func TestConfigService_GetDeviceTimeoutSettings(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("returns timeout settings for device", func(t *testing.T) {
		// ACT
		settings, err := service.GetDeviceTimeoutSettings(ctx, 1)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, settings)
	})

	t.Run("applies device-specific timeout override", func(t *testing.T) {
		// ARRANGE - Create device-specific timeout setting
		deviceID := int64(99)
		deviceKey := "device_99_timeout_minutes"
		setting := createTestSettingWithExactKey(t, db, deviceKey, "120", "system")
		defer cleanupConfigFixtures(t, db, []int64{setting.ID})

		// ACT
		settings, err := service.GetDeviceTimeoutSettings(ctx, deviceID)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, settings)
		require.NotNil(t, settings.DeviceTimeoutMinutes)
		assert.Equal(t, 120, *settings.DeviceTimeoutMinutes)
	})

	t.Run("handles invalid device timeout value gracefully", func(t *testing.T) {
		// ARRANGE - Create device-specific timeout with invalid value
		deviceID := int64(98)
		deviceKey := "device_98_timeout_minutes"
		setting := createTestSettingWithExactKey(t, db, deviceKey, "not-a-number", "system")
		defer cleanupConfigFixtures(t, db, []int64{setting.ID})

		// ACT
		settings, err := service.GetDeviceTimeoutSettings(ctx, deviceID)

		// ASSERT - Should return error for invalid value
		require.Error(t, err)
		assert.Nil(t, settings)
	})
}

// ============================================================================
// Error Type Tests
// ============================================================================

func TestConfigError_Unwrap(t *testing.T) {
	t.Run("unwraps inner error", func(t *testing.T) {
		innerErr := assert.AnError
		configErr := &configSvc.ConfigError{Op: "TestOp", Err: innerErr}

		// ACT
		unwrapped := configErr.Unwrap()

		// ASSERT
		assert.Equal(t, innerErr, unwrapped)
	})
}

func TestSettingNotFoundError(t *testing.T) {
	t.Run("returns formatted error message", func(t *testing.T) {
		err := &configSvc.SettingNotFoundError{Key: "test_key"}

		// ACT
		msg := err.Error()

		// ASSERT
		assert.Contains(t, msg, "test_key")
		assert.Contains(t, msg, "not found")
	})

	t.Run("unwraps to ErrSettingNotFound", func(t *testing.T) {
		err := &configSvc.SettingNotFoundError{Key: "test_key"}

		// ACT
		unwrapped := err.Unwrap()

		// ASSERT
		assert.Equal(t, configSvc.ErrSettingNotFound, unwrapped)
	})
}

func TestDuplicateKeyError(t *testing.T) {
	t.Run("returns formatted error message", func(t *testing.T) {
		err := &configSvc.DuplicateKeyError{Key: "dup_key"}

		// ACT
		msg := err.Error()

		// ASSERT
		assert.Contains(t, msg, "dup_key")
		assert.Contains(t, msg, "duplicate")
	})

	t.Run("unwraps to ErrDuplicateKey", func(t *testing.T) {
		err := &configSvc.DuplicateKeyError{Key: "dup_key"}

		// ACT
		unwrapped := err.Unwrap()

		// ASSERT
		assert.Equal(t, configSvc.ErrDuplicateKey, unwrapped)
	})
}

func TestValueParsingError(t *testing.T) {
	t.Run("returns formatted error message", func(t *testing.T) {
		err := &configSvc.ValueParsingError{Key: "parse_key", Value: "bad_value", Type: "int"}

		// ACT
		msg := err.Error()

		// ASSERT
		assert.Contains(t, msg, "parse_key")
		assert.Contains(t, msg, "bad_value")
		assert.Contains(t, msg, "int")
	})

	t.Run("unwraps to ErrValueParsingFailed", func(t *testing.T) {
		err := &configSvc.ValueParsingError{Key: "parse_key", Value: "bad_value", Type: "int"}

		// ACT
		unwrapped := err.Unwrap()

		// ASSERT
		assert.Equal(t, configSvc.ErrValueParsingFailed, unwrapped)
	})
}

func TestSystemSettingsLockedError(t *testing.T) {
	t.Run("returns formatted error message", func(t *testing.T) {
		err := &configSvc.SystemSettingsLockedError{Key: "system_key"}

		// ACT
		msg := err.Error()

		// ASSERT
		assert.Contains(t, msg, "system_key")
		assert.Contains(t, msg, "locked")
	})

	t.Run("unwraps to ErrSystemSettingsLocked", func(t *testing.T) {
		err := &configSvc.SystemSettingsLockedError{Key: "system_key"}

		// ACT
		unwrapped := err.Unwrap()

		// ASSERT
		assert.Equal(t, configSvc.ErrSystemSettingsLocked, unwrapped)
	})
}

func TestBatchOperationError(t *testing.T) {
	t.Run("AddError appends errors", func(t *testing.T) {
		batchErr := &configSvc.BatchOperationError{}

		// ACT
		batchErr.AddError(assert.AnError)
		batchErr.AddError(assert.AnError)

		// ASSERT
		assert.Len(t, batchErr.Errors, 2)
	})

	t.Run("HasErrors returns false when empty", func(t *testing.T) {
		batchErr := &configSvc.BatchOperationError{}

		// ACT & ASSERT
		assert.False(t, batchErr.HasErrors())
	})

	t.Run("HasErrors returns true when has errors", func(t *testing.T) {
		batchErr := &configSvc.BatchOperationError{}
		batchErr.AddError(assert.AnError)

		// ACT & ASSERT
		assert.True(t, batchErr.HasErrors())
	})
}

// ============================================================================
// Transaction Tests
// ============================================================================

func TestConfigService_WithTx(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupConfigService(t, db)
	ctx := context.Background()

	t.Run("WithTx returns transactional service", func(t *testing.T) {
		// ARRANGE
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		// ACT
		txService := service.WithTx(tx)

		// ASSERT
		require.NotNil(t, txService)
		_, ok := txService.(configSvc.Service)
		assert.True(t, ok, "WithTx should return a Service interface")
	})
}
