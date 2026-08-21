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

// roomShape is what the migration's predicate looks at beyond the color.
// The zero value is meaningless on purpose — every insert states the shape it
// stages, because "which of these four columns differs" IS the test.
type roomShape struct {
	name     string
	isSystem bool
	capacity int
	category string
}

// autoProvisionedShape mirrors what both bootstrap paths write when they
// create the Schulhof room (schulhof_service.go / checkin_system_space.go).
func autoProvisionedShape() roomShape {
	return roomShape{name: "Schulhof", isSystem: true, capacity: 300, category: "Schulhof"}
}

// insertRoom inserts a room directly via SQL: the fixture helpers uniquify
// names, which would defeat the migration's name-based predicate, and
// Room.Validate would reject some of the staged hexes.
func insertRoom(t *testing.T, db *bun.DB, tenantID int64, shape roomShape, color *string) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var id int64
	err := db.NewRaw(`
		INSERT INTO facilities.rooms (tenant_id, name, color, is_system, capacity, category)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id;
	`, tenantID, shape.name, color, shape.isSystem, shape.capacity, shape.category).Scan(ctx, &id)
	require.NoError(t, err, "insert room %q", shape.name)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	autoProvisioned := "#7ED321"
	lowercaseVariant := "#7ed321"
	adminPicked := "#A3D977"

	// Every staged room lives in its own tenant: name and LOWER(color) are
	// both unique per tenant, and most of these cases stage the same hex.
	newTenant := func() int64 {
		t.Helper()
		id, _ := testpkg.CreateTestTenant(t, db)
		return id
	}

	// The row the migration exists for: a Schulhof in bootstrap shape,
	// carrying the bootstrap hex.
	yard := insertRoom(t, db, newTenant(), autoProvisionedShape(), &autoProvisioned)

	// Rooms written before the model upper-cased on write may hold the same
	// hex in lower case; LOWER() on both sides has to catch those too.
	legacyCase := insertRoom(t, db, newTenant(), autoProvisionedShape(), &lowercaseVariant)

	// A normal room that legitimately picked the hex must be left alone —
	// the predicate is scoped to the canonical Schulhof name.
	normalTenant := newTenant()
	normalRoom := insertRoom(t, db, normalTenant,
		roomShape{name: "Bastelraum", isSystem: false, capacity: 25, category: "Gruppenraum"},
		&autoProvisioned)
	// A Schulhof whose color an admin already changed must survive untouched.
	adminChosen := insertRoom(t, db, normalTenant, autoProvisionedShape(), &adminPicked)
	// A colorless room is already in the target state.
	colorless := insertRoom(t, db, normalTenant,
		roomShape{name: "Werkraum", isSystem: false, capacity: 20, category: "Gruppenraum"}, nil)

	// A room an admin named "Schulhof" by hand, before the name was reserved,
	// and gave #7ED321 to deliberately. Migration 1.15.168 backfilled
	// is_system on it by name alone, so the flag says nothing about where the
	// color came from — only the bootstrap's capacity/category shape does,
	// and this row does not carry it. Its color must survive.
	handNamedYard := insertRoom(t, db, newTenant(),
		roomShape{name: "Schulhof", isSystem: true, capacity: 40, category: "Außenbereich"},
		&autoProvisioned)

	// Same story for a Schulhof that was never flagged as a system room.
	unflaggedYard := insertRoom(t, db, newTenant(),
		roomShape{name: "Schulhof", isSystem: false, capacity: 300, category: "Schulhof"},
		&autoProvisioned)

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
	if color := roomColor(t, db, handNamedYard); assert.NotNil(t, color) {
		assert.Equal(t, autoProvisioned, *color,
			"a hand-made Schulhof that 1.15.168 flagged as system keeps its administrator-picked color")
	}
	if color := roomColor(t, db, unflaggedYard); assert.NotNil(t, color) {
		assert.Equal(t, autoProvisioned, *color,
			"a non-system room named Schulhof keeps its administrator-picked color")
	}

	// The originals are recoverable, and nothing else was backed up.
	var backedUp int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM audit.room_color_migration_backup
		WHERE room_id IN (?, ?);
	`, yard, legacyCase).Scan(ctx, &backedUp))
	assert.Equal(t, 2, backedUp, "cleared rows must be recoverable from the backup table")

	var preservedBackups int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM audit.room_color_migration_backup
		WHERE room_id IN (?, ?, ?, ?);
	`, normalRoom, adminChosen, handNamedYard, unflaggedYard).Scan(ctx, &preservedBackups))
	assert.Equal(t, 0, preservedBackups, "preserved rows must not be written to the backup table")

	// Idempotency: a second run must be a no-op on the data.
	require.NoError(t, schulhofRoomColorClearDefaultUp(ctx, db))
	assert.Nil(t, roomColor(t, db, yard))
	if color := roomColor(t, db, adminChosen); assert.NotNil(t, color) {
		assert.Equal(t, adminPicked, *color)
	}
}
