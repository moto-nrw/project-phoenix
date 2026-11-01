package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStaffQualificationTypeGrants pins 1.15.722 (#4808): kinds are school
// data under row-level security that are archived, never deleted.
func TestStaffQualificationTypeGrants(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	privilege := func(action string) bool {
		var granted bool
		require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', 'users.staff_qualification_types', ?)`, action).Scan(ctx, &granted))
		return granted
	}
	assert.True(t, privilege("SELECT"))
	assert.True(t, privilege("INSERT"))
	assert.True(t, privilege("UPDATE"))
	assert.False(t, privilege("DELETE"), "a kind is archived, not deleted")

	var forced bool
	require.NoError(t, db.NewRaw(`SELECT relforcerowsecurity FROM pg_class WHERE oid = 'users.staff_qualification_types'::regclass`).Scan(ctx, &forced))
	assert.True(t, forced)
}

// TestStaffQualificationTypeNamesAreUniqueWhileLive keeps one live kind per
// name and school; an archived kind frees its name.
func TestStaffQualificationTypeNamesAreUniqueWhileLive(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	school := testpkg.Tenant(t)
	insert := func(name string) error {
		_, err := db.ExecContext(ctx, `INSERT INTO users.staff_qualification_types (tenant_id, name) VALUES (?, ?)`, school, name)
		return err
	}
	require.NoError(t, insert("Hygieneschulung"))
	assert.ErrorContains(t, insert(" hygieneschulung "), "uq_staff_qualification_types_live_name")
	_, err := db.ExecContext(ctx, `UPDATE users.staff_qualification_types SET archived_at = NOW() WHERE tenant_id = ?`, school)
	require.NoError(t, err)
	assert.NoError(t, insert("Hygieneschulung"))
}

// TestStaffQualificationTypesMigrationRoundTrip rolls 1.15.722 back and
// forward: the down drops the kinds and restores the 1.15.721 erasure plan,
// the up puts the kinds back into it after the qualifications.
func TestStaffQualificationTypesMigrationRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()

	require.NoError(t, staffQualificationTypesDown(ctx, db))
	var exists bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('users.staff_qualification_types') IS NOT NULL`).Scan(ctx, &exists))
	assert.False(t, exists)
	for _, entry := range readErasurePlan(t, db) {
		assert.NotEqual(t, "users.staff_qualification_types", entry.Name)
	}

	require.NoError(t, staffQualificationTypesUp(ctx, db))
	position := map[string]int{}
	for index, entry := range readErasurePlan(t, db) {
		position[entry.Name] = index
	}
	require.Contains(t, position, "users.staff_qualification_types")
	assert.Greater(t, position["users.staff_qualification_types"], position["users.staff_qualifications"],
		"the qualifications point at the kinds and go first")
}
