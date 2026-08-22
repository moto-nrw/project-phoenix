package migrations

import (
	"context"
	"testing"
	"time"

	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func busColumnExists(t *testing.T, db *bun.DB) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'users' AND table_name = 'students'
				AND column_name = 'bus'
		)
	`).Scan(context.Background(), &exists))
	return exists
}

// TestStudentsDropBusColumnMigration verifies migration 1.15.119 (#1582):
// Up drops the legacy bus column, Down rebuilds it from bus_days, and the
// bus_days source of truth survives the round-trip unchanged.
func TestStudentsDropBusColumnMigration(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	// Normalize to the post-migration baseline (bus dropped) regardless of the
	// shared test DB's current state. Up references the bus column in its
	// backfill, so it is only safe to run while the column still exists.
	if busColumnExists(t, db) {
		require.NoError(t, studentsDropBusColumnUp(ctx, db))
	}
	require.False(t, busColumnExists(t, db), "baseline: bus column should be dropped")

	tenantID := time.Now().UnixNano()
	testpkg.EnsureTestTenant(t, db, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupTenantTestData(t, db, tenantID)
		if busColumnExists(t, db) {
			require.NoError(t, studentsDropBusColumnUp(ctx, db))
		}
	})

	busKid := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Bus", "Kid", "3a")
	noBus := testpkg.CreateTestStudentForTenant(t, db, tenantID, "No", "Bus", "3a")

	setBusDays := func(id int64, jsonValue string) {
		_, err := db.NewRaw(
			`UPDATE users.students SET bus_days = ?::jsonb WHERE id = ?`,
			jsonValue, id,
		).Exec(ctx)
		require.NoError(t, err)
	}
	setBusDays(busKid.ID, `{"mon":true,"fri":true}`)
	setBusDays(noBus.ID, `{}`)

	// Down: re-add the bus column and backfill it from bus_days.HasAny().
	require.NoError(t, studentsDropBusColumnDown(ctx, db))
	require.True(t, busColumnExists(t, db), "Down should re-add the bus column")

	readBus := func(id int64) bool {
		var bus bool
		require.NoError(t, db.NewRaw(`SELECT bus FROM users.students WHERE id = ?`, id).Scan(ctx, &bus))
		return bus
	}
	assert.True(t, readBus(busKid.ID), "student with bus_days should backfill bus=true")
	assert.False(t, readBus(noBus.ID), "student with empty bus_days should backfill bus=false")

	// Up: drop the column again; bus_days must be preserved exactly.
	require.NoError(t, studentsDropBusColumnUp(ctx, db))
	require.False(t, busColumnExists(t, db), "Up should drop the bus column")

	// bun scans a scalar jsonb column into a custom map type via a struct field
	// (the same shape the student repository uses to hydrate bus_days).
	var row struct {
		BusDays usersModel.BusDays `bun:"bus_days"`
	}
	require.NoError(t, db.NewRaw(`SELECT bus_days FROM users.students WHERE id = ?`, busKid.ID).Scan(ctx, &row))
	assert.Equal(t, usersModel.BusDays{
		usersModel.BusDayMonday: true,
		usersModel.BusDayFriday: true,
	}, row.BusDays.Normalize(), "bus_days must survive the drop unchanged")
}
