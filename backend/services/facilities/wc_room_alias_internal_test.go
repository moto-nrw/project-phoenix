package facilities

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func cleanupWCRoomAliasArtifactsInternal(t *testing.T, db *bun.DB) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	type stmt struct {
		sql  string
		args []interface{}
	}
	stmts := []stmt{
		{
			`DELETE FROM active.groups WHERE room_id IN (SELECT id FROM facilities.rooms WHERE LOWER(name) IN (LOWER(?), LOWER(?)))`,
			[]interface{}{constants.WCRoomName, constants.WCRoomAliasName},
		},
		{
			`DELETE FROM activities.schedules WHERE activity_group_id IN (SELECT id FROM activities.groups WHERE name = ?)`,
			[]interface{}{constants.WCActivityName},
		},
		{
			`DELETE FROM activities.student_enrollments WHERE activity_group_id IN (SELECT id FROM activities.groups WHERE name = ?)`,
			[]interface{}{constants.WCActivityName},
		},
		{
			`DELETE FROM activities.groups WHERE name = ?`,
			[]interface{}{constants.WCActivityName},
		},
		{
			`DELETE FROM activities.categories WHERE name = ?`,
			[]interface{}{constants.WCCategoryName},
		},
		{
			`DELETE FROM facilities.rooms WHERE LOWER(name) IN (LOWER(?), LOWER(?))`,
			[]interface{}{constants.WCRoomName, constants.WCRoomAliasName},
		},
	}

	for _, s := range stmts {
		if _, err := db.NewRaw(s.sql, s.args...).Exec(ctx); err != nil {
			t.Logf("wc alias cleanup: %v (stmt: %s)", err, s.sql)
		}
	}
}

func createWCRoomAliasRoomInternal(t *testing.T, db *bun.DB, name string) *facilities.Room {
	t.Helper()

	ctx := testpkg.TenantContext(1)
	room := &facilities.Room{
		Name:     name,
		Building: "Test Building",
	}
	room.SetTenantID(1)

	err := db.NewInsert().
		Model(room).
		ModelTableExpr(`facilities.rooms`).
		Scan(ctx)
	require.NoError(t, err)

	return room
}

func TestWCService_ensureWCRoom_ReusesExistingToiletteAlias(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupWCRoomAliasArtifactsInternal(t, db)
	defer cleanupWCRoomAliasArtifactsInternal(t, db)

	service := setupWCServiceInternal(t, db)
	aliasRoom := createWCRoomAliasRoomInternal(t, db, constants.WCRoomAliasName)

	room, err := service.ensureWCRoom(testpkg.TenantContext(1))

	require.NoError(t, err)
	require.NotNil(t, room)
	assert.Equal(t, aliasRoom.ID, room.ID)
	assert.Equal(t, constants.WCRoomAliasName, room.Name)
}

// Note: a "prefers canonical when both aliases coexist" test used to live
// here. Migration 1.15.47 (uniq_facilities_rooms_tenant_wc_alias) makes
// that scenario structurally impossible — duplicates per tenant are now
// rejected at the DB layer. The canonical-iteration order in
// FindToiletRoom is still exercised by the migration's own
// TestRoomsWCAliasUniqueUp_PrefersCanonicalWC plus the lowercase-reuse
// test below.

func TestWCService_ensureWCRoom_ReusesLowercaseWCRoom(t *testing.T) {
	// Pre-existing tenants with a lowercase "wc" room have always reached
	// the toilet flow because RoomRepository.FindByName matches via
	// LOWER(name) = LOWER(?). FindToiletRoom must preserve that contract.
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	cleanupWCRoomAliasArtifactsInternal(t, db)
	defer cleanupWCRoomAliasArtifactsInternal(t, db)

	service := setupWCServiceInternal(t, db)
	lowercaseWC := createWCRoomAliasRoomInternal(t, db, "wc")

	room, err := service.ensureWCRoom(testpkg.TenantContext(1))

	require.NoError(t, err)
	require.NotNil(t, room)
	assert.Equal(t, lowercaseWC.ID, room.ID)
}
