package migrations

// Coverage for the 1.15.305 data migration (#2405): the Schulhof room's color
// became admin-configurable, and the auto-provisioned #7ED321 has to go so the
// documented orange default applies and the hex stops squatting on the
// per-tenant uniqueness index.

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// insertRoomWithExactColor inserts a room directly via SQL: the fixture
// helpers uniquify names, which would defeat the migration's name-based
// predicate, and Room.Validate would reject some of the staged hexes.
func insertRoomWithExactColor(t *testing.T, db *bun.DB, tenantID int64, name string, color *string) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var id int64
	err := db.NewRaw(`
		INSERT INTO facilities.rooms (tenant_id, name, color, is_system)
		VALUES (?, ?, ?, TRUE)
		RETURNING id;
	`, tenantID, name, color).Scan(ctx, &id)
	require.NoError(t, err, "insert room %q", name)
	return id
}

func roomColor(t *testing.T, db *bun.DB, roomID int64) *string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var color *string
	err := db.NewRaw(`SELECT color FROM facilities.rooms WHERE id = ?;`, roomID).Scan(ctx, &color)
	require.NoError(t, err, "load color for room %d", roomID)
	return color
}

func TestSchulhofRoomColorClearDefault(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID, _ := testpkg.CreateTestTenant(t, db)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	autoProvisioned := "#7ED321"
	lowercaseVariant := "#7ed321"
	adminPicked := "#A3D977"

	// The row the migration exists for: a Schulhof carrying the bootstrap hex.
	yard := insertRoomWithExactColor(t, db, tenantID, "Schulhof", &autoProvisioned)
	// Rooms written before the model upper-cased on write may hold the same
	// hex in lower case; LOWER() on both sides has to catch those too. Lives
	// in its own tenant because the per-tenant color index would reject a
	// second Schulhof-coloured row here.
	otherTenantID, _ := testpkg.CreateTestTenant(t, db)
	defer testpkg.CleanupTenantTestData(t, db, otherTenantID)
	legacyCase := insertRoomWithExactColor(t, db, otherTenantID, "Schulhof", &lowercaseVariant)
	// A normal room that legitimately picked the hex must be left alone —
	// the predicate is scoped to the canonical Schulhof name. Own tenant
	// again: the uniqueness index is per tenant on LOWER(color).
	adminTenantID, _ := testpkg.CreateTestTenant(t, db)
	defer testpkg.CleanupTenantTestData(t, db, adminTenantID)
	normalRoom := insertRoomWithExactColor(t, db, adminTenantID, "Bastelraum", &autoProvisioned)
	// A Schulhof whose color an admin already changed must survive untouched.
	adminChosen := insertRoomWithExactColor(t, db, adminTenantID, "Schulhof", &adminPicked)
	// A colorless room is already in the target state.
	colorless := insertRoomWithExactColor(t, db, adminTenantID, "Werkraum", nil)

	require.NoError(t, schulhofRoomColorClearDefaultUp(ctx, db))

	assert.Nil(t, roomColor(t, db, yard),
		"the auto-provisioned Schulhof color must be cleared so the orange default applies")
	assert.Nil(t, roomColor(t, db, legacyCase),
		"a lower-cased copy of the same hex must be cleared as well")
	if color := roomColor(t, db, normalRoom); assert.NotNil(t, color) {
		assert.Equal(t, autoProvisioned, *color,
			"a normal room that picked the hex deliberately must keep it")
	}
	if color := roomColor(t, db, adminChosen); assert.NotNil(t, color) {
		assert.Equal(t, adminPicked, *color,
			"an admin-chosen Schulhof color must survive the migration")
	}
	assert.Nil(t, roomColor(t, db, colorless))

	// The originals are recoverable.
	var backedUp int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM audit.room_color_migration_backup
		WHERE room_id IN (?, ?);
	`, yard, legacyCase).Scan(ctx, &backedUp))
	assert.Equal(t, 2, backedUp, "cleared rows must be recoverable from the backup table")

	// Idempotency: a second run must be a no-op on the data.
	require.NoError(t, schulhofRoomColorClearDefaultUp(ctx, db))
	assert.Nil(t, roomColor(t, db, yard))
	if color := roomColor(t, db, adminChosen); assert.NotNil(t, color) {
		assert.Equal(t, adminPicked, *color)
	}
}
