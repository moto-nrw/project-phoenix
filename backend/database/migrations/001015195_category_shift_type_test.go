package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func categoryShiftTypeColumnExists(t *testing.T, db *bun.DB) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'activities' AND table_name = 'categories'
				AND column_name = 'shift_type_id'
		)
	`).Scan(context.Background(), &exists))
	return exists
}

func categoryShiftTypeFKExists(t *testing.T, db *bun.DB) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.table_constraints
			WHERE constraint_schema = 'activities'
				AND table_name = 'categories'
				AND constraint_name = 'fk_activities_categories_shift_type'
		)
	`).Scan(context.Background(), &exists))
	return exists
}

// insertShiftTypeRaw inserts a schedule.shift_types row for a tenant and returns its id.
func insertShiftTypeRaw(t *testing.T, db *bun.DB, tenantID int64, name string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO schedule.shift_types (tenant_id, name, color, is_active, created_at, updated_at)
		VALUES (?, ?, '#83CD2D', TRUE, now(), now())
		RETURNING id
	`, tenantID, name).Scan(context.Background(), &id))
	return id
}

// TestCategoryShiftTypeMigration verifies migration 1.15.195: Up adds the
// activities.categories.shift_type_id column + cross-schema tenant FK, Down
// removes them, and the round-trip is idempotent (#1837 follow-up / #1836).
func TestCategoryShiftTypeMigration(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()

	if !categoryShiftTypeColumnExists(t, db) {
		require.NoError(t, categoryShiftTypeUp(ctx, db))
	}
	t.Cleanup(func() {
		if !categoryShiftTypeColumnExists(t, db) {
			require.NoError(t, categoryShiftTypeUp(ctx, db))
		}
	})
	require.True(t, categoryShiftTypeColumnExists(t, db), "baseline: shift_type_id column present")
	require.True(t, categoryShiftTypeFKExists(t, db), "baseline: FK present")

	require.NoError(t, categoryShiftTypeDown(ctx, db))
	assert.False(t, categoryShiftTypeColumnExists(t, db), "Down should drop the column")
	assert.False(t, categoryShiftTypeFKExists(t, db), "Down should drop the FK")

	require.NoError(t, categoryShiftTypeUp(ctx, db))
	assert.True(t, categoryShiftTypeColumnExists(t, db), "Up should add the column")
	assert.True(t, categoryShiftTypeFKExists(t, db), "Up should add the FK")
	require.NoError(t, categoryShiftTypeUp(ctx, db), "Up must be idempotent")
}

// TestCategoryShiftTypeFKEnforcesTenantIsolation verifies the composite FK
// rejects pointing a category at a shift type from a different tenant, and
// accepts a same-tenant reference.
func TestCategoryShiftTypeFKEnforcesTenantIsolation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()

	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)
	t.Cleanup(func() {
		testpkg.CleanupTenantTestData(t, db, tenantA)
		testpkg.CleanupTenantTestData(t, db, tenantB)
		_, _ = db.NewDelete().TableExpr("schedule.shift_types").Where("tenant_id IN (?)", bun.List([]int64{tenantA, tenantB})).Exec(ctx)
		_, _ = db.NewDelete().TableExpr("platform.schools").Where("id IN (?)", bun.List([]int64{tenantA, tenantB})).Exec(ctx)
		_, _ = db.NewDelete().TableExpr("platform.organizations").Where("id IN (?)", bun.List([]int64{tenantA, tenantB})).Exec(ctx)
	})

	stA := insertShiftTypeRaw(t, db, tenantA, fmt.Sprintf("A-%d", tenantA))
	catB := testpkg.CreateTestActivityCategoryForTenant(t, db, tenantB, "FK-CatB")

	// Cross-tenant reference must be rejected by the composite FK.
	_, err := db.NewRaw(
		`UPDATE activities.categories SET shift_type_id = ? WHERE id = ? AND tenant_id = ?`,
		stA, catB.ID, tenantB,
	).Exec(ctx)
	require.Error(t, err, "category must not reference a foreign-tenant shift type")
	assert.Contains(t, err.Error(), "fk_activities_categories_shift_type")

	// Same-tenant reference is accepted.
	stB := insertShiftTypeRaw(t, db, tenantB, fmt.Sprintf("B-%d", tenantB))
	_, err = db.NewRaw(
		`UPDATE activities.categories SET shift_type_id = ? WHERE id = ? AND tenant_id = ?`,
		stB, catB.ID, tenantB,
	).Exec(ctx)
	require.NoError(t, err, "same-tenant reference must be accepted")
}
