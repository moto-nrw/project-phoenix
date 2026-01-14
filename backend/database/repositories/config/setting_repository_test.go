package config_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

func setupSettingRepo(_ *testing.T, db *bun.DB) config.SettingRepository {
	repoFactory := repositories.NewFactory(db)
	return repoFactory.Setting
}

// cleanupSettingRecords removes settings directly
func cleanupSettingRecords(t *testing.T, db *bun.DB, settingIDs ...int64) {
	t.Helper()
	if len(settingIDs) == 0 {
		return
	}

	ctx := context.Background()
	_, err := db.NewDelete().
		TableExpr("config.settings").
		Where("id IN (?)", bun.In(settingIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup settings: %v", err)
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestSettingRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupSettingRepo(t, db)
	ctx := context.Background()

	t.Run("creates setting with valid data", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("test_key_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:         uniqueKey,
			Value:       "test_value",
			Category:    "test",
			Description: "Test setting",
		}

		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		assert.NotZero(t, setting.ID)

		cleanupSettingRecords(t, db, setting.ID)
	})

	t.Run("creates setting with restart flag", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("restart_key_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:             uniqueKey,
			Value:           "restart_value",
			Category:        "system",
			RequiresRestart: true,
		}

		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		assert.True(t, setting.RequiresRestart)

		cleanupSettingRecords(t, db, setting.ID)
	})

	t.Run("create with nil setting should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestSettingRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupSettingRepo(t, db)
	ctx := context.Background()

	t.Run("finds existing setting", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("findbyid_key_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:      uniqueKey,
			Value:    "findbyid_value",
			Category: "test",
		}
		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		defer cleanupSettingRecords(t, db, setting.ID)

		found, err := repo.FindByID(ctx, setting.ID)
		require.NoError(t, err)
		assert.Equal(t, setting.ID, found.ID)
	})

	t.Run("returns nil for non-existent setting", func(t *testing.T) {
		found, err := repo.FindByID(ctx, int64(999999))
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestSettingRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupSettingRepo(t, db)
	ctx := context.Background()

	t.Run("updates setting", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("update_key_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:      uniqueKey,
			Value:    "original_value",
			Category: "test",
		}
		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		defer cleanupSettingRecords(t, db, setting.ID)

		setting.Value = "updated_value"
		err = repo.Update(ctx, setting)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, setting.ID)
		require.NoError(t, err)
		assert.Equal(t, "updated_value", found.Value)
	})
}

func TestSettingRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupSettingRepo(t, db)
	ctx := context.Background()

	t.Run("deletes existing setting", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("delete_key_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:      uniqueKey,
			Value:    "delete_value",
			Category: "test",
		}
		err := repo.Create(ctx, setting)
		require.NoError(t, err)

		err = repo.Delete(ctx, setting.ID)
		require.NoError(t, err)

		// After delete, FindByID should return nil for not found
		found, err := repo.FindByID(ctx, setting.ID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestSettingRepository_FindByKey(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupSettingRepo(t, db)
	ctx := context.Background()

	t.Run("finds setting by key", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("findbykey_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:      uniqueKey,
			Value:    "findbykey_value",
			Category: "test",
		}
		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		defer cleanupSettingRecords(t, db, setting.ID)

		found, err := repo.FindByKey(ctx, uniqueKey)
		require.NoError(t, err)
		assert.Equal(t, setting.ID, found.ID)
	})

	t.Run("returns nil for non-existent key", func(t *testing.T) {
		found, err := repo.FindByKey(ctx, "nonexistent_key_12345")
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestSettingRepository_FindByCategory(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupSettingRepo(t, db)
	ctx := context.Background()

	t.Run("finds settings by category", func(t *testing.T) {
		uniqueCategory := fmt.Sprintf("cat_%d", time.Now().UnixNano())
		setting1 := &config.Setting{
			Key:      fmt.Sprintf("key1_%d", time.Now().UnixNano()),
			Value:    "value1",
			Category: uniqueCategory,
		}
		setting2 := &config.Setting{
			Key:      fmt.Sprintf("key2_%d", time.Now().UnixNano()),
			Value:    "value2",
			Category: uniqueCategory,
		}

		err := repo.Create(ctx, setting1)
		require.NoError(t, err)
		err = repo.Create(ctx, setting2)
		require.NoError(t, err)
		defer cleanupSettingRecords(t, db, setting1.ID, setting2.ID)

		settings, err := repo.FindByCategory(ctx, uniqueCategory)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(settings), 2)

		// All returned settings should be in this category
		for _, s := range settings {
			assert.Equal(t, uniqueCategory, s.Category)
		}
	})
}

func TestSettingRepository_FindByKeyAndCategory(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupSettingRepo(t, db)
	ctx := context.Background()

	t.Run("finds setting by key and category", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("keycat_%d", time.Now().UnixNano())
		uniqueCategory := fmt.Sprintf("catkey_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:      uniqueKey,
			Value:    "keycat_value",
			Category: uniqueCategory,
		}
		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		defer cleanupSettingRecords(t, db, setting.ID)

		found, err := repo.FindByKeyAndCategory(ctx, uniqueKey, uniqueCategory)
		require.NoError(t, err)
		assert.Equal(t, setting.ID, found.ID)
	})
}

func TestSettingRepository_GetValue(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupSettingRepo(t, db)
	ctx := context.Background()

	t.Run("gets setting value", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("getvalue_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:      uniqueKey,
			Value:    "expected_value",
			Category: "test",
		}
		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		defer cleanupSettingRecords(t, db, setting.ID)

		value, err := repo.GetValue(ctx, uniqueKey)
		require.NoError(t, err)
		assert.Equal(t, "expected_value", value)
	})
}

func TestSettingRepository_GetBoolValue(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupSettingRepo(t, db)
	ctx := context.Background()

	t.Run("gets true boolean value", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("boolvalue_true_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:      uniqueKey,
			Value:    "true",
			Category: "test",
		}
		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		defer cleanupSettingRecords(t, db, setting.ID)

		value, err := repo.GetBoolValue(ctx, uniqueKey)
		require.NoError(t, err)
		assert.True(t, value)
	})

	t.Run("gets false boolean value", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("boolvalue_false_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:      uniqueKey,
			Value:    "false",
			Category: "test",
		}
		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		defer cleanupSettingRecords(t, db, setting.ID)

		value, err := repo.GetBoolValue(ctx, uniqueKey)
		require.NoError(t, err)
		assert.False(t, value)
	})
}

func TestSettingRepository_UpdateValue(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupSettingRepo(t, db)
	ctx := context.Background()

	t.Run("updates setting value by key", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("updatevalue_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:      uniqueKey,
			Value:    "old_value",
			Category: "test",
		}
		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		defer cleanupSettingRecords(t, db, setting.ID)

		err = repo.UpdateValue(ctx, uniqueKey, "new_value")
		require.NoError(t, err)

		found, err := repo.FindByKey(ctx, uniqueKey)
		require.NoError(t, err)
		assert.Equal(t, "new_value", found.Value)
	})
}

func TestSettingRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupSettingRepo(t, db)
	ctx := context.Background()

	t.Run("lists all settings", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("list_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:      uniqueKey,
			Value:    "list_value",
			Category: "test",
		}
		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		defer cleanupSettingRecords(t, db, setting.ID)

		settings, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, settings)
	})

	t.Run("lists with key_like filter", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("likekey_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:      uniqueKey,
			Value:    "like_value",
			Category: "test",
		}
		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		defer cleanupSettingRecords(t, db, setting.ID)

		filters := map[string]interface{}{
			"key_like": "likekey",
		}
		settings, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, settings)
	})

	t.Run("lists with category_like filter", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("catlike_%d", time.Now().UnixNano())
		uniqueCategory := fmt.Sprintf("testcategory_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:      uniqueKey,
			Value:    "catlike_value",
			Category: uniqueCategory,
		}
		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		defer cleanupSettingRecords(t, db, setting.ID)

		filters := map[string]interface{}{
			"category_like": "testcategory",
		}
		settings, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, settings)
	})

	t.Run("lists with value_like filter", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("vallike_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:      uniqueKey,
			Value:    "searchable_value_content",
			Category: "test",
		}
		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		defer cleanupSettingRecords(t, db, setting.ID)

		filters := map[string]interface{}{
			"value_like": "searchable",
		}
		settings, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, settings)
	})

	t.Run("lists with requires_restart filter", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("restart_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:             uniqueKey,
			Value:           "restart_value",
			Category:        "test",
			RequiresRestart: true,
		}
		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		defer cleanupSettingRecords(t, db, setting.ID)

		filters := map[string]interface{}{
			"requires_restart": true,
		}
		settings, err := repo.List(ctx, filters)
		require.NoError(t, err)

		for _, s := range settings {
			assert.True(t, s.RequiresRestart)
		}
	})

	t.Run("lists with requires_db_reset filter", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("dbreset_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:             uniqueKey,
			Value:           "dbreset_value",
			Category:        "test",
			RequiresDBReset: true,
		}
		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		defer cleanupSettingRecords(t, db, setting.ID)

		filters := map[string]interface{}{
			"requires_db_reset": true,
		}
		settings, err := repo.List(ctx, filters)
		require.NoError(t, err)

		for _, s := range settings {
			assert.True(t, s.RequiresDBReset)
		}
	})

	t.Run("lists with category filter (default case)", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("defcat_%d", time.Now().UnixNano())
		uniqueCategory := fmt.Sprintf("defaultcat_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:      uniqueKey,
			Value:    "defcat_value",
			Category: uniqueCategory,
		}
		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		defer cleanupSettingRecords(t, db, setting.ID)

		filters := map[string]interface{}{
			"category": uniqueCategory,
		}
		settings, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, settings)
	})

	t.Run("lists with nil filter value", func(t *testing.T) {
		uniqueKey := fmt.Sprintf("nilfilter_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:      uniqueKey,
			Value:    "nilfilter_value",
			Category: "test",
		}
		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		defer cleanupSettingRecords(t, db, setting.ID)

		filters := map[string]interface{}{
			"key_like": nil,
		}
		settings, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, settings)
	})
}

func TestSettingRepository_GetFullKey(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupSettingRepo(t, db)
	ctx := context.Background()

	t.Run("constructs full key from category and key", func(t *testing.T) {
		fullKey, err := repo.GetFullKey(ctx, "system", "timeout")
		require.NoError(t, err)
		assert.Equal(t, "system.timeout", fullKey)
	})

	t.Run("normalizes key with spaces", func(t *testing.T) {
		fullKey, err := repo.GetFullKey(ctx, "system", "session timeout")
		require.NoError(t, err)
		assert.Equal(t, "system.session_timeout", fullKey)
	})

	t.Run("normalizes to lowercase", func(t *testing.T) {
		fullKey, err := repo.GetFullKey(ctx, "SYSTEM", "TIMEOUT")
		require.NoError(t, err)
		assert.Equal(t, "system.timeout", fullKey)
	})
}

func TestSettingRepository_Update_EdgeCases(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupSettingRepo(t, db)
	ctx := context.Background()

	t.Run("update with nil setting should fail", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("update with invalid setting should fail", func(t *testing.T) {
		// Create a valid setting first
		uniqueKey := fmt.Sprintf("updateinvalid_%d", time.Now().UnixNano())
		setting := &config.Setting{
			Key:      uniqueKey,
			Value:    "valid_value",
			Category: "test",
		}
		err := repo.Create(ctx, setting)
		require.NoError(t, err)
		defer cleanupSettingRecords(t, db, setting.ID)

		// Make it invalid
		setting.Key = "" // Invalid - empty key
		err = repo.Update(ctx, setting)
		assert.Error(t, err)
	})
}

func TestSettingRepository_Create_Validation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupSettingRepo(t, db)
	ctx := context.Background()

	t.Run("create with empty key should fail", func(t *testing.T) {
		setting := &config.Setting{
			Key:      "",
			Value:    "value",
			Category: "test",
		}
		err := repo.Create(ctx, setting)
		assert.Error(t, err)
	})
}
