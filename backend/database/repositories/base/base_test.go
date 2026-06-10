package base_test

import (
	"encoding/json"
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

// Generic infrastructure test for base.Repository[T].
// Uses configModels.SettingValue as the sample tenant-scoped entity because
// it has a real DB table (config.setting_values), implements modelBase.Entity,
// and exercises every code path. Choice of sample entity is incidental — the
// behavior under test is base.Repository[T] generic CRUD.

const (
	baseTestTable      = "config.setting_values"
	baseTestEntityName = "SettingValue"
)

// uniqueKey generates a unique setting key for test rows.
func uniqueKey(prefix string) string {
	return fmt.Sprintf("test_base_repo.%s_%d", prefix, time.Now().UnixNano())
}

// jsonValue returns a JSON-encoded string value suitable for the jsonb column.
func jsonValue(s string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf("%q", s))
}

// newSettingValue builds a SettingValue with tenant 1 and the given key/value.
// updatedBy is optional (pass nil to leave the FK unset).
func newSettingValue(key, value string, updatedBy *int64) *configModels.SettingValue {
	sv := &configModels.SettingValue{
		SettingKey: key,
		Value:      jsonValue(value),
		UpdatedBy:  updatedBy,
	}
	sv.SetTenantID(1)
	return sv
}

// TestNewRepository tests repository creation
func TestNewRepository(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	require.NotNil(t, repo)
	assert.Equal(t, baseTestTable, repo.TableName)
	assert.Equal(t, baseTestEntityName, repo.EntityName)
	assert.NotNil(t, repo.DB)
}

// TestRepository_Create tests the Create method
func TestRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.TenantContext(1)

	sv := newSettingValue(uniqueKey("create"), "test_value", nil)

	// Cleanup after test
	defer func() {
		_, _ = db.NewDelete().Model(sv).
			ModelTableExpr(baseTestTable).
			Where("setting_key = ?", sv.SettingKey).
			Exec(ctx)
	}()

	err := repo.Create(ctx, sv)
	require.NoError(t, err)
	assert.NotZero(t, sv.ID)
}

// TestRepository_Create_NilEntity tests Create with nil entity
func TestRepository_Create_NilEntity(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.TenantContext(1)

	var nilSV *configModels.SettingValue
	err := repo.Create(ctx, nilSV)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil or zero value")
}

// TestRepository_FindByID tests the FindByID method
func TestRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.TenantContext(1)

	// Insert a test row using schema-qualified table
	sv := newSettingValue(uniqueKey("find"), "find_value", nil)
	_, err := db.NewInsert().Model(sv).ModelTableExpr(baseTestTable).Exec(ctx)
	require.NoError(t, err)

	// Cleanup after test
	defer func() {
		_, _ = db.NewDelete().Model(sv).ModelTableExpr(baseTestTable).Where("id = ?", sv.ID).Exec(ctx)
	}()

	// Test FindByID
	found, err := repo.FindByID(ctx, sv.ID)
	require.NoError(t, err)
	assert.Equal(t, sv.ID, found.ID)
	assert.Equal(t, sv.SettingKey, found.SettingKey)
	assert.JSONEq(t, string(sv.Value), string(found.Value))
}

// TestRepository_FindByID_NotFound tests FindByID with non-existent ID
func TestRepository_FindByID_NotFound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.TenantContext(1)

	_, err := repo.FindByID(ctx, 999999)
	require.Error(t, err)
}

// TestRepository_Update tests the Update method
func TestRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.TenantContext(1)

	// Insert a test row
	sv := newSettingValue(uniqueKey("update"), "original_value", nil)
	_, err := db.NewInsert().Model(sv).ModelTableExpr(baseTestTable).Exec(ctx)
	require.NoError(t, err)
	require.NotZero(t, sv.ID)

	// Cleanup after test
	defer func() {
		_, _ = db.NewDelete().Model(sv).
			ModelTableExpr(baseTestTable).
			Where("setting_key = ?", sv.SettingKey).
			Exec(ctx)
	}()

	// Update the value
	sv.Value = jsonValue("updated_value")
	err = repo.Update(ctx, sv)
	require.NoError(t, err)

	// Verify the update
	found, err := repo.FindByID(ctx, sv.ID)
	require.NoError(t, err)
	assert.JSONEq(t, `"updated_value"`, string(found.Value))
}

// TestRepository_Update_NilEntity tests Update with nil entity
func TestRepository_Update_NilEntity(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.TenantContext(1)

	var nilSV *configModels.SettingValue
	err := repo.Update(ctx, nilSV)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil or zero value")
}

// TestRepository_Delete tests the Delete method
func TestRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.TenantContext(1)

	// Insert a test row
	sv := newSettingValue(uniqueKey("delete"), "delete_value", nil)
	_, err := db.NewInsert().Model(sv).ModelTableExpr(baseTestTable).Exec(ctx)
	require.NoError(t, err)

	// Delete the row
	err = repo.Delete(ctx, sv.ID)
	require.NoError(t, err)

	// Verify the delete
	var count int
	count, err = db.NewSelect().Model((*configModels.SettingValue)(nil)).
		ModelTableExpr(baseTestTable).
		Where("id = ?", sv.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// TestRepository_List tests the List method.
// Uses a real auth.accounts row and stamps `updated_by` so the filter scopes
// to this test's rows only — required because base_test.go runs in parallel
// with other tests touching config.setting_values.
func TestRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	acct := testpkg.CreateTestAccount(t, db, "base_repo_list")
	updatedBy := acct.ID

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.TenantContext(1)

	// Insert two test rows tagged with this test's account
	settings := []*configModels.SettingValue{
		newSettingValue(uniqueKey("list_1"), "v1", &updatedBy),
		newSettingValue(uniqueKey("list_2"), "v2", &updatedBy),
	}
	for _, s := range settings {
		_, err := db.NewInsert().Model(s).ModelTableExpr(baseTestTable).Exec(ctx)
		require.NoError(t, err)
	}

	// Cleanup after test
	defer func() {
		for _, s := range settings {
			_, _ = db.NewDelete().Model(s).ModelTableExpr(baseTestTable).Where("id = ?", s.ID).Exec(ctx)
		}
	}()

	// Test List with filter on updated_by (unique to this test)
	results, err := repo.List(ctx, map[string]interface{}{"updated_by": updatedBy})
	require.NoError(t, err)
	assert.Equal(t, 2, len(results))
}

// TestRepository_List_NoFilters tests List with empty filters
func TestRepository_List_NoFilters(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.TenantContext(1)

	// Insert one row so the result slice is non-nil even on a freshly reset DB.
	// bun returns a nil slice when SELECT matches zero rows.
	sv := newSettingValue(uniqueKey("list_no_filters"), "v", nil)
	_, err := db.NewInsert().Model(sv).ModelTableExpr(baseTestTable).Exec(ctx)
	require.NoError(t, err)
	defer func() {
		_, _ = db.NewDelete().Model(sv).ModelTableExpr(baseTestTable).Where("id = ?", sv.ID).Exec(ctx)
	}()

	results, err := repo.List(ctx, nil)
	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.GreaterOrEqual(t, len(results), 1)
}

// TestRepository_Count tests the Count method
func TestRepository_Count(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	acct := testpkg.CreateTestAccount(t, db, "base_repo_count")
	updatedBy := acct.ID

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.TenantContext(1)

	// Insert three test rows tagged with this test's account
	settings := []*configModels.SettingValue{
		newSettingValue(uniqueKey("count_1"), "v1", &updatedBy),
		newSettingValue(uniqueKey("count_2"), "v2", &updatedBy),
		newSettingValue(uniqueKey("count_3"), "v3", &updatedBy),
	}
	for _, s := range settings {
		_, err := db.NewInsert().Model(s).ModelTableExpr(baseTestTable).Exec(ctx)
		require.NoError(t, err)
	}

	// Cleanup after test
	defer func() {
		for _, s := range settings {
			_, _ = db.NewDelete().Model(s).ModelTableExpr(baseTestTable).Where("id = ?", s.ID).Exec(ctx)
		}
	}()

	// Test Count with filter on updated_by (unique to this test)
	count, err := repo.Count(ctx, map[string]interface{}{"updated_by": updatedBy})
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

// TestRepository_Count_NoFilters tests Count with empty filters
func TestRepository_Count_NoFilters(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.TenantContext(1)

	count, err := repo.Count(ctx, nil)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 0)
}

// TestRepository_Transaction tests the Transaction method
func TestRepository_Transaction(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.TenantContext(1)

	var createdID int64

	// Cleanup after test
	defer func() {
		if createdID > 0 {
			_, _ = db.NewDelete().Model((*configModels.SettingValue)(nil)).
				ModelTableExpr(baseTestTable).
				Where("id = ?", createdID).Exec(ctx)
		}
	}()

	err := repo.Transaction(ctx, func(tx bun.Tx) error {
		sv := newSettingValue(uniqueKey("transaction"), "tx_value", nil)
		_, err := tx.NewInsert().Model(sv).ModelTableExpr(baseTestTable).Exec(ctx)
		if err != nil {
			return err
		}
		createdID = sv.ID
		return nil
	})
	require.NoError(t, err)
	assert.NotZero(t, createdID)
}

// TestRepository_Transaction_Rollback tests Transaction rollback on error
func TestRepository_Transaction_Rollback(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.TenantContext(1)

	testKey := uniqueKey("transaction_rollback")

	err := repo.Transaction(ctx, func(tx bun.Tx) error {
		sv := newSettingValue(testKey, "rollback_value", nil)
		_, err := tx.NewInsert().Model(sv).ModelTableExpr(baseTestTable).Exec(ctx)
		if err != nil {
			return err
		}
		// Force rollback by returning error
		return assert.AnError
	})
	require.Error(t, err)

	// Verify the insert was rolled back
	var count int
	count, err = db.NewSelect().Model((*configModels.SettingValue)(nil)).
		ModelTableExpr(baseTestTable).
		Where("setting_key = ?", testKey).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
