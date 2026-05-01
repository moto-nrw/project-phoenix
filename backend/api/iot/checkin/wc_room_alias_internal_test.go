package checkin

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func cleanupWCRoomAliasTestArtifacts(t *testing.T, db *bun.DB) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	aliasArgs := []interface{}{constants.WCRoomName, constants.WCRoomAliasName}
	type stmt struct {
		sql  string
		args []interface{}
	}
	stmts := []stmt{
		{
			`DELETE FROM active.attendance WHERE visit_id IN (SELECT v.id FROM active.visits v JOIN active.groups ag ON ag.id = v.active_group_id JOIN facilities.rooms r ON r.id = ag.room_id WHERE LOWER(r.name) IN (LOWER(?), LOWER(?)))`,
			aliasArgs,
		},
		{
			`DELETE FROM active.visits WHERE active_group_id IN (SELECT ag.id FROM active.groups ag JOIN facilities.rooms r ON r.id = ag.room_id WHERE LOWER(r.name) IN (LOWER(?), LOWER(?)))`,
			aliasArgs,
		},
		{
			`DELETE FROM active.group_supervisors WHERE group_id IN (SELECT ag.id FROM active.groups ag JOIN facilities.rooms r ON r.id = ag.room_id WHERE LOWER(r.name) IN (LOWER(?), LOWER(?)))`,
			aliasArgs,
		},
		{
			`DELETE FROM active.groups WHERE room_id IN (SELECT id FROM facilities.rooms WHERE LOWER(name) IN (LOWER(?), LOWER(?)))`,
			aliasArgs,
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
			aliasArgs,
		},
	}

	for _, s := range stmts {
		_, _ = db.NewRaw(s.sql, s.args...).Exec(ctx)
	}
}

func createWCRoomAliasRoom(t *testing.T, db *bun.DB, name string) *facilities.Room {
	t.Helper()

	ctx := tenant.WithTenantID(context.Background(), 1)
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

func TestEnsureWCRoom_ReusesExistingToiletteAlias(t *testing.T) {
	// setupInternalTestResource uses SetupAPITest under the hood, so this file
	// still follows the hermetic DB setup pattern enforced by TestHermeticTestPatterns.
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	cleanupWCRoomAliasTestArtifacts(t, tc.db)
	defer cleanupWCRoomAliasTestArtifacts(t, tc.db)

	aliasRoom := createWCRoomAliasRoom(t, tc.db, constants.WCRoomAliasName)

	room, err := tc.rs.ensureWCRoom(tenant.WithTenantID(context.Background(), 1))

	require.NoError(t, err)
	require.NotNil(t, room)
	assert.Equal(t, aliasRoom.ID, room.ID)
	assert.Equal(t, constants.WCRoomAliasName, room.Name)
}

// Note: a "prefers canonical when both aliases coexist" test used to live
// here. Migration 1.15.48 (uniq_facilities_rooms_tenant_wc_alias) makes
// that scenario structurally impossible — duplicates per tenant are now
// rejected at the DB layer. The canonical-iteration order in
// FindToiletRoom is still exercised by the migration's own
// TestRoomsWCAliasUniqueUp_PrefersCanonicalWC.

func TestEnsureWCRoom_IgnoresLowercaseWCRoom(t *testing.T) {
	// Contract per issue #1184 review: only exact-case "WC" and "Toilette"
	// are toilet system rooms. The IoT auto-create path in this package
	// (ensureWCRoom) goes through services/facilities.FindToiletRoom which
	// re-filters via IsWCRoomName, so a lowercase "wc" must not be silently
	// adopted here either.
	//
	// Practical consequence: ensureWCRoom does not return the lowercase
	// row, falls through to CreateRoom("WC"), and CreateRoom's
	// case-insensitive duplicate guard (LOWER(name) = LOWER(?)) rejects
	// because "wc" already exists. The retry path then calls
	// FindToiletRoom again — still skips the lowercase row — and surfaces
	// the original create error. Stuck state requiring admin cleanup, no
	// silent cross-layer drift. See companion test
	// TestWCService_ensureWCRoom_IgnoresLowercaseWCRoom in
	// services/facilities for the same assertion at the service layer.
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	cleanupWCRoomAliasTestArtifacts(t, tc.db)
	defer cleanupWCRoomAliasTestArtifacts(t, tc.db)

	lowercaseWC := createWCRoomAliasRoom(t, tc.db, "wc")

	room, err := tc.rs.ensureWCRoom(tenant.WithTenantID(context.Background(), 1))

	require.Error(t, err, "ensureWCRoom must not silently reuse lowercase wc; the duplicate-name collision must surface as an error")
	assert.Nil(t, room)

	// Lowercase room is untouched.
	var nameAfter string
	err = tc.db.NewSelect().
		Table("facilities.rooms").
		Column("name").
		Where("id = ?", lowercaseWC.ID).
		Scan(tenant.WithTenantID(context.Background(), 1), &nameAfter)
	require.NoError(t, err)
	assert.Equal(t, "wc", nameAfter)
}
