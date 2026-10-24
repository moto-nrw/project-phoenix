package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertShiftTypeRaw inserts a schedule.shift_types row for a tenant and returns its id.
func insertShiftTypeRaw(t *testing.T, db *testpkg.DB, tenantID int64, name string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO schedule.shift_types (tenant_id, name, color, is_active, created_at, updated_at)
		VALUES (?, ?, '#83CD2D', TRUE, now(), now())
		RETURNING id
	`, tenantID, name).Scan(context.Background(), &id))
	return id
}

// TestCategoryShiftTypeFKEnforcesTenantIsolation verifies the composite FK
// rejects pointing a category at a shift type from a different tenant, and
// accepts a same-tenant reference.
func TestCategoryShiftTypeFKEnforcesTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.OwnTenantRows(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)
	testpkg.OwnTenantRows(t, db, tenantB)

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
