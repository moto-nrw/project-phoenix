package migrations

// Coverage for the 1.15.370 room-release backfill (#3064): existing schools
// must keep their Schulhof permanently available without a new daily step,
// while every other room — including the toilet system rooms and a room a
// school merely named "Schulhof" itself — starts unreleased.

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertOpenReleaseTestRoom inserts a room directly via SQL so the test can
// stage an exact room name — the fixture helpers uniquify names, which would
// defeat the migration's name-based predicate. is_open_room is deliberately
// left to its column default so the staged state matches a database that has
// not been through the backfill yet.
func insertOpenReleaseTestRoom(t *testing.T, db *testpkg.DB, tenantID int64, name string, isSystem bool) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var id int64
	err := db.NewRaw(`
		INSERT INTO facilities.rooms (tenant_id, name, is_system)
		VALUES (?, ?, ?)
		RETURNING id;
	`, tenantID, name, isSystem).Scan(ctx, &id)
	require.NoError(t, err, "insert room %q", name)
	return id
}

func roomIsOpenRoom(t *testing.T, db *testpkg.DB, roomID int64) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var isOpenRoom bool
	err := db.NewRaw(`SELECT is_open_room FROM facilities.rooms WHERE id = ?;`, roomID).Scan(ctx, &isOpenRoom)
	require.NoError(t, err, "load is_open_room for room %d", roomID)
	return isOpenRoom
}

// TestRoomsOpenRelease_ReleasesOnlyCanonicalSchulhof stages the room set a
// deployed school carries into 1.15.370 and runs the backfill. The
// load-bearing fixtures are the two rooms the predicate must tell apart: the
// auto-provisioned Schulhof (is_system) and a school's own room that happens
// to carry the same name.
func TestRoomsOpenRelease_ReleasesOnlyCanonicalSchulhof(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)

	// The yard every existing school already uses as a permanent destination.
	schulhof := insertOpenReleaseTestRoom(t, db, tenantID, "Schulhof", true)
	// Ordinary rooms are an administrator's decision, never the migration's.
	groupRoom := insertOpenReleaseTestRoom(t, db, tenantID, "Gruppenraum Grün", false)
	// The toilet system room must not inherit the yard's release: a released
	// WC would offer children a destination the product never intended.
	wc := insertOpenReleaseTestRoom(t, db, tenantID, "WC", true)

	// A second tenant for the two rows that cannot coexist with the ones
	// above: UNIQUE(tenant_id, name) rules out a second "Schulhof", and
	// uniq_facilities_rooms_tenant_wc_alias allows a school only ONE toilet
	// room, so "Toilette" needs its own school to be covered at all.
	//
	// The self-made Schulhof is the row the is_system predicate exists for: a
	// school that created its own room named "Schulhof" before the
	// reservation guards landed never got the auto-provisioned infrastructure
	// and must not be released.
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	testpkg.OwnTenantRows(t, db, otherTenantID)
	selfMadeSchulhof := insertOpenReleaseTestRoom(t, db, otherTenantID, "Schulhof", false)
	toilette := insertOpenReleaseTestRoom(t, db, otherTenantID, "Toilette", true)

	require.NoError(t, roomsOpenReleaseUp(ctx, db))

	assert.True(t, roomIsOpenRoom(t, db, schulhof),
		"the canonical Schulhof must be released so existing schools need no new daily step")
	assert.False(t, roomIsOpenRoom(t, db, groupRoom),
		"an ordinary room must start unreleased")
	assert.False(t, roomIsOpenRoom(t, db, wc),
		"the WC system room must not be released")
	assert.False(t, roomIsOpenRoom(t, db, toilette),
		"the Toilette system room must not be released")
	assert.False(t, roomIsOpenRoom(t, db, selfMadeSchulhof),
		"a school's own room named Schulhof was never auto-provisioned and must stay unreleased")
}

// TestRoomsOpenRelease_IsIdempotent guards the re-run path. The column add is
// IF NOT EXISTS and the backfill is a plain UPDATE, so a second pass must
// reach the same verdicts rather than erroring or widening the release.
func TestRoomsOpenRelease_IsIdempotent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)

	schulhof := insertOpenReleaseTestRoom(t, db, tenantID, "Schulhof", true)
	groupRoom := insertOpenReleaseTestRoom(t, db, tenantID, "Turnhalle", false)

	require.NoError(t, roomsOpenReleaseUp(ctx, db))
	require.NoError(t, roomsOpenReleaseUp(ctx, db))

	assert.True(t, roomIsOpenRoom(t, db, schulhof))
	assert.False(t, roomIsOpenRoom(t, db, groupRoom))
}

// TestRoomsOpenRelease_DefaultsToUnreleased pins the column contract that the
// whole feature rests on: a room created without an opinion is not an open
// room. Room release is an explicit administrative decision.
func TestRoomsOpenRelease_DefaultsToUnreleased(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)

	room := insertOpenReleaseTestRoom(t, db, tenantID, "Werkraum", false)

	assert.False(t, roomIsOpenRoom(t, db, room),
		"is_open_room must default to FALSE for a newly created room")
}

// TestRoomsOpenRelease_PreservesExistingRoomData asserts what the migration
// must NOT do. #3064 requires room identities, live visits and activity
// associations to survive the cutover untouched, so the backfill may only
// ever write the one new column.
func TestRoomsOpenRelease_PreservesExistingRoomData(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)

	schulhof := insertOpenReleaseTestRoom(t, db, tenantID, "Schulhof", true)
	before := loadOpenReleaseRoomSnapshot(t, db, schulhof)

	require.NoError(t, roomsOpenReleaseUp(ctx, db))

	after := loadOpenReleaseRoomSnapshot(t, db, schulhof)
	assert.Equal(t, before.id, after.id, "room identity must survive the migration")
	assert.Equal(t, before.name, after.name, "room name must survive the migration")
	assert.Equal(t, before.tenantID, after.tenantID, "tenant assignment must survive the migration")
	assert.Equal(t, before.capacity, after.capacity, "capacity must survive the migration")
	assert.Equal(t, before.isSystem, after.isSystem, "the system-room flag must survive the migration")
	assert.Equal(t, before.createdAt, after.createdAt, "created_at must survive the migration")
}

type openReleaseRoomSnapshot struct {
	id        int64
	tenantID  int64
	name      string
	capacity  int
	isSystem  bool
	createdAt time.Time
}

func loadOpenReleaseRoomSnapshot(t *testing.T, db *testpkg.DB, roomID int64) openReleaseRoomSnapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var snap openReleaseRoomSnapshot
	err := db.NewRaw(`
		SELECT id, tenant_id, name, capacity, is_system, created_at
		FROM facilities.rooms WHERE id = ?;
	`, roomID).Scan(ctx, &snap.id, &snap.tenantID, &snap.name, &snap.capacity, &snap.isSystem, &snap.createdAt)
	require.NoError(t, err, "load room snapshot for room %d", roomID)
	return snap
}
