package base_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
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

// newSettingValue builds a SettingValue in the calling test's tenant with the given key/value.
// updatedBy is optional (pass nil to leave the FK unset).
func newSettingValue(tb testing.TB, key, value string, updatedBy *int64) *configModels.SettingValue {
	sv := &configModels.SettingValue{
		SettingKey: key,
		Value:      jsonValue(value),
		UpdatedBy:  updatedBy,
	}
	sv.SetTenantID(testpkg.Tenant(tb))
	return sv
}

func TestAcquireXactLockReportsContendedWait(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	key := uniqueKey("lock_wait")

	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = holder.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key)
	require.NoError(t, err)
	ctx, evidence := testpkg.CaptureUnitOfWorkEvidence(ctx)
	testpkg.AttachLockWaitEvidence(db)
	release := time.AfterFunc(20*time.Millisecond, func() { _ = holder.Rollback() })
	defer release.Stop()

	require.NoError(t, base.AcquireXactLock(ctx, db, key))

	events := evidence()
	require.Len(t, events, 1)
	assert.Equal(t, "lock_wait", events[0].Kind)
	assert.GreaterOrEqual(t, events[0].Duration, 15*time.Millisecond)
}

// TestNewRepository tests repository creation
func TestNewRepository(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	require.NotNil(t, repo)
	assert.Equal(t, baseTestTable, repo.TableName)
	assert.Equal(t, baseTestEntityName, repo.EntityName)
	assert.NotNil(t, repo.DB)
}

// TestRepository_Create tests the Create method
func TestRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.Ctx(t)

	sv := newSettingValue(t, uniqueKey("create"), "test_value", nil)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.Ctx(t)

	var nilSV *configModels.SettingValue
	err := repo.Create(ctx, nilSV)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil or zero value")
}

// TestRepository_FindByID tests the FindByID method
func TestRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.Ctx(t)

	// Insert a test row using schema-qualified table
	sv := newSettingValue(t, uniqueKey("find"), "find_value", nil)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.Ctx(t)

	_, err := repo.FindByID(ctx, 999999)
	require.Error(t, err)
}

// TestRepository_Update tests the Update method
func TestRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.Ctx(t)

	// Insert a test row
	sv := newSettingValue(t, uniqueKey("update"), "original_value", nil)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.Ctx(t)

	var nilSV *configModels.SettingValue
	err := repo.Update(ctx, nilSV)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil or zero value")
}

// TestRepository_Delete tests the Delete method
func TestRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.Ctx(t)

	// Insert a test row
	sv := newSettingValue(t, uniqueKey("delete"), "delete_value", nil)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	acct := testpkg.CreateTestAccount(t, db, "base_repo_list")
	updatedBy := acct.ID

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.Ctx(t)

	// Insert two test rows tagged with this test's account
	settings := []*configModels.SettingValue{
		newSettingValue(t, uniqueKey("list_1"), "v1", &updatedBy),
		newSettingValue(t, uniqueKey("list_2"), "v2", &updatedBy),
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.Ctx(t)

	// Insert one row so the result slice is non-nil even on a freshly reset DB.
	// bun returns a nil slice when SELECT matches zero rows.
	sv := newSettingValue(t, uniqueKey("list_no_filters"), "v", nil)
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

// ============================================================================
// Generic query helpers (issue #585 services→repository refactor):
// CountWithOptions, OldestBefore, DeleteOlderThan, UpdateColumns.
//
// Aggregate tests (count/min/delete) run under their own tenant via
// UniqueTestTenantID + TenantScoped=true, so rows that other tests create in
// the shared config.setting_values table cannot affect the assertions.
// ============================================================================

// newTenantScopedRepo builds a base repository with the tenant_id
// defense-in-depth filter enabled, matching how domain repositories
// (students, groups, ...) construct their embedded generic.
func newTenantScopedRepo(db *bun.DB) *base.Repository[*configModels.SettingValue] {
	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	repo.TenantScoped = true
	return repo
}

// createSettingValueForTenant inserts a row for the given tenant and returns it.
func createSettingValueForTenant(t *testing.T, db *bun.DB, tenantID int64, key, value string) *configModels.SettingValue {
	t.Helper()
	sv := &configModels.SettingValue{
		SettingKey: key,
		Value:      jsonValue(value),
	}
	sv.SetTenantID(tenantID)
	_, err := db.NewInsert().Model(sv).ModelTableExpr(baseTestTable).Exec(testpkg.TenantContext(tenantID))
	require.NoError(t, err)
	return sv
}

// setSettingValueCreatedAt overwrites created_at so date-window tests have
// deterministic timestamps (the column default is current_timestamp).
func setSettingValueCreatedAt(t *testing.T, db *bun.DB, id int64, createdAt time.Time) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr(baseTestTable).
		Set("created_at = ?", createdAt).
		Where("id = ?", id).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
}

func TestRepository_CountWithOptions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.EnsureTestTenant(t, db, otherTenantID)

	repo := newTenantScopedRepo(db)
	ctx := testpkg.TenantContext(tenantID)

	prefix := uniqueKey("count_options")
	first := createSettingValueForTenant(t, db, tenantID, prefix+"_a", "v1")
	createSettingValueForTenant(t, db, tenantID, prefix+"_b", "v2")
	createSettingValueForTenant(t, db, tenantID, prefix+"_c", "v3")
	// Same key prefix under another tenant — must not be counted.
	createSettingValueForTenant(t, db, otherTenantID, prefix+"_other", "v4")

	t.Run("nil options counts all tenant rows", func(t *testing.T) {
		count, err := repo.CountWithOptions(ctx, nil)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("like filter", func(t *testing.T) {
		options := modelBase.NewQueryOptions()
		options.Filter = modelBase.NewFilter().Like("setting_key", prefix+"%")
		count, err := repo.CountWithOptions(ctx, options)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("equal filter", func(t *testing.T) {
		options := modelBase.NewQueryOptions()
		options.Filter = modelBase.NewFilter().Equal("setting_key", first.SettingKey)
		count, err := repo.CountWithOptions(ctx, options)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}

func TestRepository_OldestBefore(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	repo := newTenantScopedRepo(db)
	ctx := testpkg.TenantContext(tenantID)

	older := time.Date(2020, 1, 15, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2021, 3, 10, 12, 0, 0, 0, time.UTC)

	prefix := uniqueKey("oldest_before")
	first := createSettingValueForTenant(t, db, tenantID, prefix+"_old", "v1")
	second := createSettingValueForTenant(t, db, tenantID, prefix+"_new", "v2")
	setSettingValueCreatedAt(t, db, first.ID, older)
	setSettingValueCreatedAt(t, db, second.ID, newer)

	t.Run("nil cutoff returns absolute minimum", func(t *testing.T) {
		oldest, err := repo.OldestBefore(ctx, "created_at", nil)
		require.NoError(t, err)
		require.NotNil(t, oldest)
		assert.Equal(t, timezone.DateFromTime(older), *oldest)
	})

	t.Run("cutoff between rows returns only the older one", func(t *testing.T) {
		cutoff := timezone.NewDate(2020, 6, 1)
		oldest, err := repo.OldestBefore(ctx, "created_at", &cutoff)
		require.NoError(t, err)
		require.NotNil(t, oldest)
		assert.Equal(t, timezone.DateFromTime(older), *oldest)
	})

	t.Run("cutoff before all rows returns nil", func(t *testing.T) {
		cutoff := timezone.NewDate(2019, 1, 1)
		oldest, err := repo.OldestBefore(ctx, "created_at", &cutoff)
		require.NoError(t, err)
		assert.Nil(t, oldest)
	})
}

func TestRepository_DeleteOlderThan(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.EnsureTestTenant(t, db, otherTenantID)

	repo := newTenantScopedRepo(db)
	ctx := testpkg.TenantContext(tenantID)

	older := time.Date(2020, 1, 15, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2021, 3, 10, 12, 0, 0, 0, time.UTC)

	prefix := uniqueKey("delete_older")
	expired := createSettingValueForTenant(t, db, tenantID, prefix+"_expired", "v1")
	kept := createSettingValueForTenant(t, db, tenantID, prefix+"_kept", "v2")
	foreign := createSettingValueForTenant(t, db, otherTenantID, prefix+"_foreign", "v3")
	setSettingValueCreatedAt(t, db, expired.ID, older)
	setSettingValueCreatedAt(t, db, kept.ID, newer)
	setSettingValueCreatedAt(t, db, foreign.ID, older)

	cutoff := timezone.NewDate(2020, 12, 31)
	deleted, err := repo.DeleteOlderThan(ctx, "created_at", cutoff)
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)

	// The newer row survives.
	remaining, err := repo.CountWithOptions(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, remaining)

	// The other tenant's expired row is untouched despite matching the cutoff.
	foreignCount, err := repo.CountWithOptions(testpkg.TenantContext(otherTenantID), nil)
	require.NoError(t, err)
	assert.Equal(t, 1, foreignCount)
}

func TestRepository_UpdateColumns(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := base.NewRepository[*configModels.SettingValue](db, baseTestTable, baseTestEntityName)
	ctx := testpkg.Ctx(t)

	sv := newSettingValue(t, uniqueKey("update_columns"), "original", nil)
	_, err := db.NewInsert().Model(sv).ModelTableExpr(baseTestTable).Exec(ctx)
	require.NoError(t, err)
	defer func() {
		_, _ = db.NewDelete().TableExpr(baseTestTable).Where("id = ?", sv.ID).Exec(ctx)
	}()

	t.Run("updates only the named columns", func(t *testing.T) {
		originalKey := sv.SettingKey
		sv.Value = jsonValue("changed")
		sv.SettingKey = originalKey + "_mutated_in_memory"

		updated, err := repo.UpdateColumns(ctx, sv, "value")
		require.NoError(t, err)
		assert.EqualValues(t, 1, updated)

		found, err := repo.FindByID(ctx, sv.ID)
		require.NoError(t, err)
		assert.JSONEq(t, `"changed"`, string(found.Value))
		assert.Equal(t, originalKey, found.SettingKey, "setting_key must not be written when only value is named")

		sv.SettingKey = originalKey
	})

	t.Run("returns zero rows for missing entity without error", func(t *testing.T) {
		ghost := newSettingValue(t, uniqueKey("update_columns_ghost"), "ghost", nil)
		ghost.ID = 999999999
		updated, err := repo.UpdateColumns(ctx, ghost, "value")
		require.NoError(t, err)
		assert.Equal(t, int64(0), updated)
	})

	t.Run("rejects empty column list", func(t *testing.T) {
		_, err := repo.UpdateColumns(ctx, sv)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one column required")
	})

	t.Run("rejects nil entity", func(t *testing.T) {
		var nilSV *configModels.SettingValue
		_, err := repo.UpdateColumns(ctx, nilSV, "value")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil or zero value")
	})
}
