package config_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	repoConfig "github.com/moto-nrw/project-phoenix/database/repositories/config"
	"github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// createTestDefinition creates a valid setting definition for testing
func createTestDefinition(key string) *config.SettingDefinition {
	defaultValue, _ := json.Marshal(map[string]any{"value": true})
	return &config.SettingDefinition{
		Key:              key,
		Type:             config.SettingTypeBool,
		DefaultValue:     defaultValue,
		Category:         "test",
		Description:      "Test setting for " + key,
		AllowedScopes:    []string{"system", "og"},
		ScopePermissions: map[string]string{"system": "config:write", "og": "config:write"},
		SortOrder:        10,
	}
}

// cleanupDefinition removes a test definition from the database
func cleanupDefinition(tb testing.TB, db *bun.DB, id int64) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = db.NewDelete().
		TableExpr("config.setting_definitions").
		Where("id = ?", id).
		Exec(ctx)
}

func TestSettingDefinitionRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoConfig.NewSettingDefinitionRepository(db)
	ctx := context.Background()

	t.Run("creates definition successfully", func(t *testing.T) {
		def := createTestDefinition("test.create." + t.Name())
		err := repo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		assert.NotZero(t, def.ID)
		assert.NotZero(t, def.CreatedAt)
	})

	t.Run("fails with invalid definition", func(t *testing.T) {
		def := &config.SettingDefinition{
			// Missing required fields
		}
		err := repo.Create(ctx, def)
		require.Error(t, err)
	})
}

func TestSettingDefinitionRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoConfig.NewSettingDefinitionRepository(db)
	ctx := context.Background()

	t.Run("finds existing definition", func(t *testing.T) {
		def := createTestDefinition("test.findbyid." + t.Name())
		err := repo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		found, err := repo.FindByID(ctx, def.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, def.Key, found.Key)
	})

	t.Run("returns nil for non-existent ID", func(t *testing.T) {
		found, err := repo.FindByID(ctx, 999999)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestSettingDefinitionRepository_FindByKey(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoConfig.NewSettingDefinitionRepository(db)
	ctx := context.Background()

	t.Run("finds existing definition by key", func(t *testing.T) {
		uniqueKey := "test.findbykey." + t.Name()
		def := createTestDefinition(uniqueKey)
		err := repo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		// Use def.Key since Validate() normalizes to lowercase
		found, err := repo.FindByKey(ctx, def.Key)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, def.Key, found.Key)
	})

	t.Run("returns nil for non-existent key", func(t *testing.T) {
		found, err := repo.FindByKey(ctx, "nonexistent.key")
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestSettingDefinitionRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoConfig.NewSettingDefinitionRepository(db)
	ctx := context.Background()

	t.Run("updates definition successfully", func(t *testing.T) {
		def := createTestDefinition("test.update." + t.Name())
		err := repo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		def.Description = "Updated description"
		err = repo.Update(ctx, def)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, def.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated description", found.Description)
	})
}

func TestSettingDefinitionRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoConfig.NewSettingDefinitionRepository(db)
	ctx := context.Background()

	t.Run("deletes definition successfully", func(t *testing.T) {
		def := createTestDefinition("test.delete." + t.Name())
		err := repo.Create(ctx, def)
		require.NoError(t, err)

		err = repo.Delete(ctx, def.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, def.ID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestSettingDefinitionRepository_FindByCategory(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoConfig.NewSettingDefinitionRepository(db)
	ctx := context.Background()

	t.Run("finds definitions by category", func(t *testing.T) {
		uniqueCategory := "testcategory" + t.Name()

		def1 := createTestDefinition("test.cat1." + t.Name())
		def1.Category = uniqueCategory
		err := repo.Create(ctx, def1)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def1.ID)

		def2 := createTestDefinition("test.cat2." + t.Name())
		def2.Category = uniqueCategory
		err = repo.Create(ctx, def2)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def2.ID)

		// Use def1.Category since Validate() normalizes to lowercase
		found, err := repo.FindByCategory(ctx, def1.Category)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 2)
	})
}

func TestSettingDefinitionRepository_FindByScope(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoConfig.NewSettingDefinitionRepository(db)
	ctx := context.Background()

	t.Run("finds definitions by scope", func(t *testing.T) {
		def := createTestDefinition("test.scope." + t.Name())
		def.AllowedScopes = []string{"system", "user"}
		def.ScopePermissions = map[string]string{"system": "config:write", "user": "config:write"}
		err := repo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		found, err := repo.FindByScope(ctx, config.ScopeUser)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 1)

		// Verify our definition is in the results
		var foundOurs bool
		for _, f := range found {
			if f.ID == def.ID {
				foundOurs = true
				break
			}
		}
		assert.True(t, foundOurs, "Our definition should be in results")
	})
}

func TestSettingDefinitionRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoConfig.NewSettingDefinitionRepository(db)
	ctx := context.Background()

	t.Run("lists definitions with filters", func(t *testing.T) {
		uniqueCategory := "listcat" + t.Name()
		def := createTestDefinition("test.list." + t.Name())
		def.Category = uniqueCategory
		err := repo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		// Use def.Category since Validate() normalizes to lowercase
		filters := map[string]interface{}{
			"category": def.Category,
		}
		found, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 1)
	})

	t.Run("lists with search filter", func(t *testing.T) {
		uniqueKey := "searchable.unique." + t.Name()
		def := createTestDefinition(uniqueKey)
		def.Description = "A unique searchable description"
		err := repo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		filters := map[string]interface{}{
			"search": "searchable",
		}
		found, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 1)
	})
}

func TestSettingDefinitionRepository_FindByGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoConfig.NewSettingDefinitionRepository(db)
	ctx := context.Background()

	t.Run("finds definitions by group name", func(t *testing.T) {
		uniqueGroup := "testgroup" + t.Name()

		def1 := createTestDefinition("test.group1." + t.Name())
		def1.GroupName = uniqueGroup
		err := repo.Create(ctx, def1)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def1.ID)

		def2 := createTestDefinition("test.group2." + t.Name())
		def2.GroupName = uniqueGroup
		err = repo.Create(ctx, def2)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def2.ID)

		found, err := repo.FindByGroup(ctx, uniqueGroup)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 2)

		for _, f := range found {
			assert.Equal(t, uniqueGroup, f.GroupName)
		}
	})

	t.Run("returns empty for non-existent group", func(t *testing.T) {
		found, err := repo.FindByGroup(ctx, "nonexistent_group_name")
		require.NoError(t, err)
		assert.Empty(t, found)
	})
}

func TestSettingDefinitionRepository_List_WithScopeFilter(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoConfig.NewSettingDefinitionRepository(db)
	ctx := context.Background()

	t.Run("filters by scope type", func(t *testing.T) {
		def := createTestDefinition("test.list.scope." + t.Name())
		def.AllowedScopes = []string{"user", "system"}
		def.ScopePermissions = map[string]string{"user": "self", "system": "config:write"}
		err := repo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		filters := map[string]interface{}{
			"scope": "user",
		}
		found, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 1)

		// Verify our definition is in the results
		var foundOurs bool
		for _, f := range found {
			if f.ID == def.ID {
				foundOurs = true
				break
			}
		}
		assert.True(t, foundOurs, "Our definition should be in results")
	})

	t.Run("filters by group name", func(t *testing.T) {
		uniqueGroup := "listgroup" + t.Name()
		def := createTestDefinition("test.list.group." + t.Name())
		def.GroupName = uniqueGroup
		err := repo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		filters := map[string]interface{}{
			"group": uniqueGroup,
		}
		found, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 1)

		for _, f := range found {
			assert.Equal(t, uniqueGroup, f.GroupName)
		}
	})

	t.Run("returns all with empty filters", func(t *testing.T) {
		def := createTestDefinition("test.list.empty." + t.Name())
		err := repo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		found, err := repo.List(ctx, map[string]interface{}{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 1)
	})

	t.Run("combines multiple filters", func(t *testing.T) {
		uniqueCategory := "multifilter" + t.Name()
		uniqueGroup := "multigroup" + t.Name()

		def := createTestDefinition("test.list.multi." + t.Name())
		def.Category = uniqueCategory
		def.GroupName = uniqueGroup
		def.AllowedScopes = []string{"system", "og"}
		def.ScopePermissions = map[string]string{"system": "config:write", "og": "config:write"}
		err := repo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		filters := map[string]interface{}{
			"category": def.Category,
			"group":    uniqueGroup,
			"scope":    "og",
		}
		found, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 1)
	})
}

func TestSettingDefinitionRepository_ListAll(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoConfig.NewSettingDefinitionRepository(db)
	ctx := context.Background()

	t.Run("lists all definitions", func(t *testing.T) {
		def := createTestDefinition("test.listall." + t.Name())
		err := repo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		found, err := repo.ListAll(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 1)
	})
}

func TestSettingDefinitionRepository_Upsert(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoConfig.NewSettingDefinitionRepository(db)
	ctx := context.Background()

	t.Run("inserts new definition", func(t *testing.T) {
		uniqueKey := "test.upsert.new." + t.Name()
		def := createTestDefinition(uniqueKey)
		err := repo.Upsert(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		assert.NotZero(t, def.ID)

		// Use def.Key since Validate() normalizes to lowercase
		found, err := repo.FindByKey(ctx, def.Key)
		require.NoError(t, err)
		require.NotNil(t, found)
	})

	t.Run("updates existing definition", func(t *testing.T) {
		uniqueKey := "test.upsert.existing." + t.Name()
		def := createTestDefinition(uniqueKey)
		err := repo.Upsert(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		originalID := def.ID
		def.Description = "Updated via upsert"
		err = repo.Upsert(ctx, def)
		require.NoError(t, err)

		// ID should be the same (update, not insert)
		assert.Equal(t, originalID, def.ID)

		// Use def.Key since Validate() normalizes to lowercase
		found, err := repo.FindByKey(ctx, def.Key)
		require.NoError(t, err)
		assert.Equal(t, "Updated via upsert", found.Description)
	})
}
