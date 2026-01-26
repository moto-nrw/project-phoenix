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

// cleanupValue removes a test value from the database
func cleanupValue(tb testing.TB, db *bun.DB, id int64) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = db.NewDelete().
		TableExpr("config.setting_values").
		Where("id = ?", id).
		Exec(ctx)
}

// createTestValue creates a valid setting value for testing
func createTestValue(definitionID int64, scopeType string, scopeID *int64) *config.SettingValue {
	valueBytes, _ := json.Marshal(map[string]any{"value": true})
	return &config.SettingValue{
		DefinitionID: definitionID,
		ScopeType:    scopeType,
		ScopeID:      scopeID,
		Value:        valueBytes,
	}
}

func TestSettingValueRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	defRepo := repoConfig.NewSettingDefinitionRepository(db)
	valueRepo := repoConfig.NewSettingValueRepository(db)
	ctx := context.Background()

	t.Run("creates value successfully for system scope", func(t *testing.T) {
		// Create a definition first
		def := createTestDefinition("test.value.create.system." + t.Name())
		err := defRepo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		// Create value
		value := createTestValue(def.ID, "system", nil)
		err = valueRepo.Create(ctx, value)
		require.NoError(t, err)
		defer cleanupValue(t, db, value.ID)

		assert.NotZero(t, value.ID)
	})

	t.Run("creates value successfully for og scope", func(t *testing.T) {
		def := createTestDefinition("test.value.create.og." + t.Name())
		err := defRepo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		ogID := int64(123)
		value := createTestValue(def.ID, "og", &ogID)
		err = valueRepo.Create(ctx, value)
		require.NoError(t, err)
		defer cleanupValue(t, db, value.ID)

		assert.NotZero(t, value.ID)
	})

	t.Run("fails with invalid value", func(t *testing.T) {
		value := &config.SettingValue{
			// Missing required fields
		}
		err := valueRepo.Create(ctx, value)
		require.Error(t, err)
	})
}

func TestSettingValueRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	defRepo := repoConfig.NewSettingDefinitionRepository(db)
	valueRepo := repoConfig.NewSettingValueRepository(db)
	ctx := context.Background()

	t.Run("finds existing value", func(t *testing.T) {
		def := createTestDefinition("test.value.findbyid." + t.Name())
		err := defRepo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		value := createTestValue(def.ID, "system", nil)
		err = valueRepo.Create(ctx, value)
		require.NoError(t, err)
		defer cleanupValue(t, db, value.ID)

		found, err := valueRepo.FindByID(ctx, value.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, value.ID, found.ID)
	})

	t.Run("returns nil for non-existent ID", func(t *testing.T) {
		found, err := valueRepo.FindByID(ctx, 999999)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestSettingValueRepository_FindByDefinitionAndScope(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	defRepo := repoConfig.NewSettingDefinitionRepository(db)
	valueRepo := repoConfig.NewSettingValueRepository(db)
	ctx := context.Background()

	t.Run("finds value for system scope", func(t *testing.T) {
		def := createTestDefinition("test.value.findbyscope.system." + t.Name())
		err := defRepo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		value := createTestValue(def.ID, "system", nil)
		err = valueRepo.Create(ctx, value)
		require.NoError(t, err)
		defer cleanupValue(t, db, value.ID)

		found, err := valueRepo.FindByDefinitionAndScope(ctx, def.ID, "system", nil)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, value.ID, found.ID)
	})

	t.Run("finds value for og scope", func(t *testing.T) {
		def := createTestDefinition("test.value.findbyscope.og." + t.Name())
		err := defRepo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		ogID := int64(456)
		value := createTestValue(def.ID, "og", &ogID)
		err = valueRepo.Create(ctx, value)
		require.NoError(t, err)
		defer cleanupValue(t, db, value.ID)

		found, err := valueRepo.FindByDefinitionAndScope(ctx, def.ID, "og", &ogID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, value.ID, found.ID)
	})

	t.Run("returns nil when not found", func(t *testing.T) {
		found, err := valueRepo.FindByDefinitionAndScope(ctx, 999999, "system", nil)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestSettingValueRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	defRepo := repoConfig.NewSettingDefinitionRepository(db)
	valueRepo := repoConfig.NewSettingValueRepository(db)
	ctx := context.Background()

	t.Run("updates value successfully", func(t *testing.T) {
		def := createTestDefinition("test.value.update." + t.Name())
		err := defRepo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		value := createTestValue(def.ID, "system", nil)
		err = valueRepo.Create(ctx, value)
		require.NoError(t, err)
		defer cleanupValue(t, db, value.ID)

		// Update the value
		newValueBytes, _ := json.Marshal(map[string]any{"value": false})
		value.Value = newValueBytes
		err = valueRepo.Update(ctx, value)
		require.NoError(t, err)

		found, err := valueRepo.FindByID(ctx, value.ID)
		require.NoError(t, err)
		// Compare parsed JSON (PostgreSQL may format differently)
		assert.JSONEq(t, string(newValueBytes), string(found.Value))
	})
}

func TestSettingValueRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	defRepo := repoConfig.NewSettingDefinitionRepository(db)
	valueRepo := repoConfig.NewSettingValueRepository(db)
	ctx := context.Background()

	t.Run("deletes value successfully", func(t *testing.T) {
		def := createTestDefinition("test.value.delete." + t.Name())
		err := defRepo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		value := createTestValue(def.ID, "system", nil)
		err = valueRepo.Create(ctx, value)
		require.NoError(t, err)

		err = valueRepo.Delete(ctx, value.ID)
		require.NoError(t, err)

		found, err := valueRepo.FindByID(ctx, value.ID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestSettingValueRepository_FindAllForScope(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	defRepo := repoConfig.NewSettingDefinitionRepository(db)
	valueRepo := repoConfig.NewSettingValueRepository(db)
	ctx := context.Background()

	t.Run("finds all values for scope", func(t *testing.T) {
		// Create two definitions
		def1 := createTestDefinition("test.value.findall1." + t.Name())
		err := defRepo.Create(ctx, def1)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def1.ID)

		def2 := createTestDefinition("test.value.findall2." + t.Name())
		err = defRepo.Create(ctx, def2)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def2.ID)

		// Create values for both
		value1 := createTestValue(def1.ID, "system", nil)
		err = valueRepo.Create(ctx, value1)
		require.NoError(t, err)
		defer cleanupValue(t, db, value1.ID)

		value2 := createTestValue(def2.ID, "system", nil)
		err = valueRepo.Create(ctx, value2)
		require.NoError(t, err)
		defer cleanupValue(t, db, value2.ID)

		found, err := valueRepo.FindAllForScope(ctx, "system", nil)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 2)
	})
}

func TestSettingValueRepository_FindByDefinition(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	defRepo := repoConfig.NewSettingDefinitionRepository(db)
	valueRepo := repoConfig.NewSettingValueRepository(db)
	ctx := context.Background()

	t.Run("finds all values for definition", func(t *testing.T) {
		def := createTestDefinition("test.value.findbydef." + t.Name())
		def.AllowedScopes = []string{"system", "og"}
		def.ScopePermissions = map[string]string{"system": "config:write", "og": "config:write"}
		err := defRepo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		// Create system scope value
		value1 := createTestValue(def.ID, "system", nil)
		err = valueRepo.Create(ctx, value1)
		require.NoError(t, err)
		defer cleanupValue(t, db, value1.ID)

		// Create og scope value
		ogID := int64(789)
		value2 := createTestValue(def.ID, "og", &ogID)
		err = valueRepo.Create(ctx, value2)
		require.NoError(t, err)
		defer cleanupValue(t, db, value2.ID)

		found, err := valueRepo.FindByDefinition(ctx, def.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 2)
	})
}

func TestSettingValueRepository_Upsert(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	defRepo := repoConfig.NewSettingDefinitionRepository(db)
	valueRepo := repoConfig.NewSettingValueRepository(db)
	ctx := context.Background()

	t.Run("inserts new value", func(t *testing.T) {
		def := createTestDefinition("test.value.upsert.new." + t.Name())
		err := defRepo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		value := createTestValue(def.ID, "system", nil)
		err = valueRepo.Upsert(ctx, value)
		require.NoError(t, err)
		defer cleanupValue(t, db, value.ID)

		assert.NotZero(t, value.ID)
	})

	t.Run("updates existing value", func(t *testing.T) {
		// Use og scope (non-NULL scope_id) to test upsert update behavior
		// Note: PostgreSQL unique constraints don't match NULL=NULL, so system scope
		// upsert would create new rows. This is a known PostgreSQL behavior.
		def := createTestDefinition("test.value.upsert.existing." + t.Name())
		def.AllowedScopes = []string{"og"}
		def.ScopePermissions = map[string]string{"og": "config:write"}
		err := defRepo.Create(ctx, def)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def.ID)

		scopeID := int64(42)
		value := createTestValue(def.ID, "og", &scopeID)
		err = valueRepo.Upsert(ctx, value)
		require.NoError(t, err)
		defer cleanupValue(t, db, value.ID)

		// Update via upsert with same definition/scope
		newValueBytes, _ := json.Marshal(map[string]any{"value": false})
		updateValue := createTestValue(def.ID, "og", &scopeID)
		updateValue.Value = newValueBytes
		err = valueRepo.Upsert(ctx, updateValue)
		require.NoError(t, err)

		// Verify the value was updated (not a new insert)
		found, err := valueRepo.FindByDefinitionAndScope(ctx, def.ID, "og", &scopeID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.JSONEq(t, string(newValueBytes), string(found.Value))
	})
}

func TestSettingValueRepository_DeleteByScope(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	defRepo := repoConfig.NewSettingDefinitionRepository(db)
	valueRepo := repoConfig.NewSettingValueRepository(db)
	ctx := context.Background()

	t.Run("deletes all values for scope", func(t *testing.T) {
		def1 := createTestDefinition("test.value.deletebyscope1." + t.Name())
		def1.AllowedScopes = []string{"og"}
		def1.ScopePermissions = map[string]string{"og": "config:write"}
		err := defRepo.Create(ctx, def1)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def1.ID)

		def2 := createTestDefinition("test.value.deletebyscope2." + t.Name())
		def2.AllowedScopes = []string{"og"}
		def2.ScopePermissions = map[string]string{"og": "config:write"}
		err = defRepo.Create(ctx, def2)
		require.NoError(t, err)
		defer cleanupDefinition(t, db, def2.ID)

		ogID := int64(999)
		value1 := createTestValue(def1.ID, "og", &ogID)
		err = valueRepo.Create(ctx, value1)
		require.NoError(t, err)
		// No defer cleanup - will be deleted by DeleteByScope

		value2 := createTestValue(def2.ID, "og", &ogID)
		err = valueRepo.Create(ctx, value2)
		require.NoError(t, err)
		// No defer cleanup - will be deleted by DeleteByScope

		count, err := valueRepo.DeleteByScope(ctx, "og", ogID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 2)

		// Verify deleted
		found1, err := valueRepo.FindByID(ctx, value1.ID)
		require.NoError(t, err)
		assert.Nil(t, found1)

		found2, err := valueRepo.FindByID(ctx, value2.ID)
		require.NoError(t, err)
		assert.Nil(t, found2)
	})
}
