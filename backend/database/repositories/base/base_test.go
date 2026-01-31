package base_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// uniqueKey generates a unique key for test settings
func uniqueKey(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// TestNewRepository tests repository creation
func TestNewRepository(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.Setting](db, "config.settings", "Setting")
	require.NotNil(t, repo)
	assert.Equal(t, "config.settings", repo.TableName)
	assert.Equal(t, "Setting", repo.EntityName)
	assert.NotNil(t, repo.DB)
}

// TestRepository_Create tests the Create method
func TestRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.Setting](db, "config.settings", "Setting")
	ctx := context.Background()

	key := uniqueKey("test_base_repo_create")
	setting := &configModels.Setting{
		Key:         key,
		Value:       "test_value",
		Description: "Test setting for base repository",
		Category:    "test",
	}

	// Cleanup after test
	defer func() {
		_, _ = db.NewDelete().Model(setting).
			ModelTableExpr("config.settings").
			Where("key = ?", setting.Key).
			Exec(ctx)
	}()

	err := repo.Create(ctx, setting)
	require.NoError(t, err)
	assert.NotZero(t, setting.ID)
}

// TestRepository_Create_NilEntity tests Create with nil entity
func TestRepository_Create_NilEntity(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.Setting](db, "config.settings", "Setting")
	ctx := context.Background()

	var nilSetting *configModels.Setting
	err := repo.Create(ctx, nilSetting)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil or zero value")
}

// TestRepository_FindByID tests the FindByID method
func TestRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.Setting](db, "config.settings", "Setting")
	ctx := context.Background()

	// Create a test setting using schema-qualified table
	setting := &configModels.Setting{
		Key:         "test_base_repo_find",
		Value:       "find_value",
		Description: "Test setting for FindByID",
		Category:    "test",
	}
	_, err := db.NewInsert().Model(setting).ModelTableExpr("config.settings").Exec(ctx)
	require.NoError(t, err)

	// Cleanup after test
	defer func() {
		_, _ = db.NewDelete().Model(setting).ModelTableExpr("config.settings").Where("id = ?", setting.ID).Exec(ctx)
	}()

	// Test FindByID
	found, err := repo.FindByID(ctx, setting.ID)
	require.NoError(t, err)
	assert.Equal(t, setting.ID, found.ID)
	assert.Equal(t, setting.Key, found.Key)
	assert.Equal(t, setting.Value, found.Value)
}

// TestRepository_FindByID_NotFound tests FindByID with non-existent ID
func TestRepository_FindByID_NotFound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.Setting](db, "config.settings", "Setting")
	ctx := context.Background()

	_, err := repo.FindByID(ctx, 999999)
	require.Error(t, err)
}

// TestRepository_Update tests the Update method
func TestRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.Setting](db, "config.settings", "Setting")
	ctx := context.Background()

	// Create a test setting
	key := uniqueKey("test_base_repo_update")
	setting := &configModels.Setting{
		Key:         key,
		Value:       "original_value",
		Description: "Test setting for Update",
		Category:    "test",
	}
	_, err := db.NewInsert().Model(setting).ModelTableExpr("config.settings").Exec(ctx)
	require.NoError(t, err)
	require.NotZero(t, setting.ID)

	// Cleanup after test
	defer func() {
		_, _ = db.NewDelete().Model(setting).
			ModelTableExpr("config.settings").
			Where("key = ?", setting.Key).
			Exec(ctx)
	}()

	// Update the setting
	setting.Value = "updated_value"
	err = repo.Update(ctx, setting)
	require.NoError(t, err)

	// Verify the update
	found, err := repo.FindByID(ctx, setting.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated_value", found.Value)
}

// TestRepository_Update_NilEntity tests Update with nil entity
func TestRepository_Update_NilEntity(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.Setting](db, "config.settings", "Setting")
	ctx := context.Background()

	var nilSetting *configModels.Setting
	err := repo.Update(ctx, nilSetting)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil or zero value")
}

// TestRepository_Delete tests the Delete method
func TestRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.Setting](db, "config.settings", "Setting")
	ctx := context.Background()

	// Create a test setting
	setting := &configModels.Setting{
		Key:         "test_base_repo_delete",
		Value:       "delete_value",
		Description: "Test setting for Delete",
		Category:    "test",
	}
	_, err := db.NewInsert().Model(setting).ModelTableExpr("config.settings").Exec(ctx)
	require.NoError(t, err)

	// Delete the setting
	err = repo.Delete(ctx, setting.ID)
	require.NoError(t, err)

	// Verify the delete
	var count int
	count, err = db.NewSelect().Model((*configModels.Setting)(nil)).
		ModelTableExpr("config.settings").
		Where("id = ?", setting.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// TestRepository_List tests the List method
func TestRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.Setting](db, "config.settings", "Setting")
	ctx := context.Background()

	// Create test settings
	settings := []*configModels.Setting{
		{Key: "test_base_list_1", Value: "v1", Description: "Test 1", Category: "base_test"},
		{Key: "test_base_list_2", Value: "v2", Description: "Test 2", Category: "base_test"},
	}
	for _, s := range settings {
		_, err := db.NewInsert().Model(s).ModelTableExpr("config.settings").Exec(ctx)
		require.NoError(t, err)
	}

	// Cleanup after test
	defer func() {
		for _, s := range settings {
			_, _ = db.NewDelete().Model(s).ModelTableExpr("config.settings").Where("id = ?", s.ID).Exec(ctx)
		}
	}()

	// Test List with filter
	results, err := repo.List(ctx, map[string]interface{}{"category": "base_test"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 2)
}

// TestRepository_List_NoFilters tests List with empty filters
func TestRepository_List_NoFilters(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.Setting](db, "config.settings", "Setting")
	ctx := context.Background()

	results, err := repo.List(ctx, nil)
	require.NoError(t, err)
	assert.NotNil(t, results)
}

// TestRepository_Count tests the Count method
func TestRepository_Count(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.Setting](db, "config.settings", "Setting")
	ctx := context.Background()

	// Create test settings
	settings := []*configModels.Setting{
		{Key: "test_base_count_1", Value: "v1", Description: "Test 1", Category: "count_test"},
		{Key: "test_base_count_2", Value: "v2", Description: "Test 2", Category: "count_test"},
		{Key: "test_base_count_3", Value: "v3", Description: "Test 3", Category: "count_test"},
	}
	for _, s := range settings {
		_, err := db.NewInsert().Model(s).ModelTableExpr("config.settings").Exec(ctx)
		require.NoError(t, err)
	}

	// Cleanup after test
	defer func() {
		for _, s := range settings {
			_, _ = db.NewDelete().Model(s).ModelTableExpr("config.settings").Where("id = ?", s.ID).Exec(ctx)
		}
	}()

	// Test Count with filter
	count, err := repo.Count(ctx, map[string]interface{}{"category": "count_test"})
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

// TestRepository_Count_NoFilters tests Count with empty filters
func TestRepository_Count_NoFilters(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.Setting](db, "config.settings", "Setting")
	ctx := context.Background()

	count, err := repo.Count(ctx, nil)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 0)
}

// TestRepository_Transaction tests the Transaction method
func TestRepository_Transaction(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.Setting](db, "config.settings", "Setting")
	ctx := context.Background()

	var createdID int64

	// Cleanup after test
	defer func() {
		if createdID > 0 {
			_, _ = db.NewDelete().Model((*configModels.Setting)(nil)).
				ModelTableExpr("config.settings").
				Where("id = ?", createdID).Exec(ctx)
		}
	}()

	err := repo.Transaction(ctx, func(tx bun.Tx) error {
		setting := &configModels.Setting{
			Key:         "test_base_transaction",
			Value:       "tx_value",
			Description: "Test setting for Transaction",
			Category:    "test",
		}
		_, err := tx.NewInsert().Model(setting).ModelTableExpr("config.settings").Exec(ctx)
		if err != nil {
			return err
		}
		createdID = setting.ID
		return nil
	})
	require.NoError(t, err)
	assert.NotZero(t, createdID)
}

// TestRepository_Transaction_Rollback tests Transaction rollback on error
func TestRepository_Transaction_Rollback(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.Setting](db, "config.settings", "Setting")
	ctx := context.Background()

	testKey := "test_base_transaction_rollback"

	err := repo.Transaction(ctx, func(tx bun.Tx) error {
		setting := &configModels.Setting{
			Key:         testKey,
			Value:       "rollback_value",
			Description: "Test setting for Transaction rollback",
			Category:    "test",
		}
		_, err := tx.NewInsert().Model(setting).ModelTableExpr("config.settings").Exec(ctx)
		if err != nil {
			return err
		}
		// Force rollback by returning error
		return assert.AnError
	})
	require.Error(t, err)

	// Verify the insert was rolled back
	var count int
	count, err = db.NewSelect().Model((*configModels.Setting)(nil)).
		ModelTableExpr("config.settings").
		Where("key = ?", testKey).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
