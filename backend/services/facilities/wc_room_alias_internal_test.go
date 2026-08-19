package facilities

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func createWCRoomAliasRoomInternal(t *testing.T, db *bun.DB, name string) *facilities.Room {
	t.Helper()

	ctx := testpkg.Ctx(t)
	room := &facilities.Room{
		Name:     name,
		Building: "Test Building",
	}
	room.SetTenantID(testpkg.Tenant(t))

	err := db.NewInsert().
		Model(room).
		ModelTableExpr(`facilities.rooms`).
		Scan(ctx)
	require.NoError(t, err)

	return room
}

func TestWCService_ensureWCRoom_ReusesExistingToiletteAlias(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupWCServiceInternal(t, db)
	aliasRoom := createWCRoomAliasRoomInternal(t, db, constants.WCRoomAliasName)

	room, err := service.ensureWCRoom(testpkg.Ctx(t))

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
	t.Parallel()

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

	service := setupWCServiceInternal(t, db)
	lowercaseWC := createWCRoomAliasRoomInternal(t, db, "wc")

	room, err := service.ensureWCRoom(testpkg.Ctx(t))

	require.Error(t, err, "ensureWCRoom must not silently reuse lowercase wc and must surface the duplicate-name collision when auto-creating canonical WC")
	assert.Nil(t, room)

	// Lowercase room is untouched: its name stays "wc" and it remains a
	// regular admin-managed room (deletable/renamable via the rooms admin).
	var nameAfter string
	err = db.NewSelect().
		Table("facilities.rooms").
		Column("name").
		Where("id = ?", lowercaseWC.ID).
		Scan(testpkg.Ctx(t), &nameAfter)
	require.NoError(t, err)
	assert.Equal(t, "wc", nameAfter)
}

// TestFindToiletRoom_SkipsLowercaseWCRoom asserts the contract directly at
// the FindToiletRoom layer: case-variants are skipped, ErrRoomNotFound is
// returned. Companion to the ensureWCRoom test above — that one asserts the
// downstream side-effect, this one asserts the lookup primitive.
func TestFindToiletRoom_SkipsLowercaseWCRoom(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupWCServiceInternal(t, db)
	_ = createWCRoomAliasRoomInternal(t, db, "wc")
	_ = createWCRoomAliasRoomInternal(t, db, "toilette")

	room, err := service.facilityService.FindToiletRoom(testpkg.Ctx(t), 0)

	require.Error(t, err)
	assert.Nil(t, room)
	assert.True(t, errors.Is(err, ErrRoomNotFound))
}
