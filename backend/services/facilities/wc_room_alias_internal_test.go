package facilities

import (
	"context"
	"errors"
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
// here. Migration 1.15.48 (uniq_facilities_rooms_tenant_wc_alias) makes
// that scenario structurally impossible — duplicates per tenant are now
// rejected at the DB layer. The canonical-iteration order in
// FindToiletRoom is still exercised by the migration's own
// TestRoomsWCAliasUniqueUp_PrefersCanonicalWC.

func TestWCService_ensureWCRoom_IgnoresLowercaseWCRoom(t *testing.T) {
	// Contract per issue #1184 review: only the exact-case names "WC" and
	// "Toilette" are toilet system rooms. A lowercase "wc" must NOT be
	// silently adopted by FindToiletRoom — otherwise it would be used as
	// the WC special-room while remaining unprotected against rename/delete
	// (constants.IsSystemRoomName is exact-case) and invisible to the IoT
	// scan-fallback switch in api/iot/checkin/workflow.go (also exact-case).
	//
	// Practical consequence for tenants with a pre-existing lowercase room:
	// FindToiletRoom skips it and ensureWCRoom proceeds to create canonical
	// "WC", which then fails the case-insensitive duplicate guard in
	// CreateRoom (LOWER(name) = LOWER(?)). The result is a stuck state
	// surfaced as an error — the admin must rename the lowercase room
	// before the IoT WC button works. That's acceptable: no silent data
	// adoption, no invisible cross-layer drift.
	db := testpkg.SetupTestDB(t)

	cleanupWCRoomAliasArtifactsInternal(t, db)
	defer cleanupWCRoomAliasArtifactsInternal(t, db)

	service := setupWCServiceInternal(t, db)
	lowercaseWC := createWCRoomAliasRoomInternal(t, db, "wc")

	room, err := service.ensureWCRoom(testpkg.TenantContext(1))

	require.Error(t, err, "ensureWCRoom must not silently reuse lowercase wc and must surface the duplicate-name collision when auto-creating canonical WC")
	assert.Nil(t, room)

	// Lowercase room is untouched: its name stays "wc" and it remains a
	// regular admin-managed room (deletable/renamable via the rooms admin).
	var nameAfter string
	err = db.NewSelect().
		Table("facilities.rooms").
		Column("name").
		Where("id = ?", lowercaseWC.ID).
		Scan(testpkg.TenantContext(1), &nameAfter)
	require.NoError(t, err)
	assert.Equal(t, "wc", nameAfter)
}

// TestFindToiletRoom_SkipsLowercaseWCRoom asserts the contract directly at
// the FindToiletRoom layer: case-variants are skipped, ErrRoomNotFound is
// returned. Companion to the ensureWCRoom test above — that one asserts the
// downstream side-effect, this one asserts the lookup primitive.
func TestFindToiletRoom_SkipsLowercaseWCRoom(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	cleanupWCRoomAliasArtifactsInternal(t, db)
	defer cleanupWCRoomAliasArtifactsInternal(t, db)

	service := setupWCServiceInternal(t, db)
	_ = createWCRoomAliasRoomInternal(t, db, "wc")
	_ = createWCRoomAliasRoomInternal(t, db, "toilette")

	room, err := service.facilityService.FindToiletRoom(testpkg.TenantContext(1), 0)

	require.Error(t, err)
	assert.Nil(t, room)
	assert.True(t, errors.Is(err, ErrRoomNotFound))
}
