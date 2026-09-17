package migrations

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertColoredRoom(t *testing.T, db *testpkg.DB, tenantID int64, name, color string) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var id int64
	err := db.NewRaw(`
		INSERT INTO facilities.rooms (tenant_id, name, building, floor, capacity, category, color)
		VALUES (?, ?, 'Test Building', 0, 10, 'Other', ?)
		RETURNING id;
	`, tenantID, name, color).Scan(ctx, &id)
	require.NoError(t, err, "insert room %q", name)
	return id
}

func roomColorByID(t *testing.T, db *testpkg.DB, id int64) *string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var color *string
	err := db.NewRaw(`SELECT color FROM facilities.rooms WHERE id = ?;`, id).Scan(ctx, &color)
	require.NoError(t, err)
	return color
}

func TestRoomsRetireAtSchoolColorUp_ClearsSchuleHexAndLeavesOthers(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)

	schuleID := insertColoredRoom(t, db, tenantID, "Atelier", "#217a78")
	otherID := insertColoredRoom(t, db, tenantID, "Werkraum", "#A3D977")

	require.NoError(t, roomsRetireAtSchoolColorUp(context.Background(), db))

	assert.Nil(t, roomColorByID(t, db, schuleID), "Schule status hex must be cleared so the room stays editable")
	assert.Equal(t, "#A3D977", *roomColorByID(t, db, otherID), "unrelated room colors must stay")

	var backupColor string
	err := db.NewRaw(`
		SELECT color FROM audit.room_color_migration_backup
		WHERE tenant_id = ? AND room_id = ?;
	`, tenantID, schuleID).Scan(context.Background(), &backupColor)
	require.NoError(t, err)
	assert.Equal(t, "#217a78", backupColor)
}

func TestRoomsRetireAtSchoolColorUp_NoMatchingRooms(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	require.NoError(t, roomsRetireAtSchoolColorUp(context.Background(), db))
}
