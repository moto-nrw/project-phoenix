package migrations

// Coverage for the 1.15.390 correction (#3280, ADR 0019): the release from
// 1.15.372 is revoked only where the canonical Schulhof hosts upcoming planned
// blocks of a regular activity. Dates are fixed far in the future or past so
// the verdict never depends on the day the suite runs.

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	blockPlanningFutureDate = "2099-06-01"
	blockPlanningPastDate   = "2000-06-01"
)

type blockPlanningInstance struct {
	date        string
	spontaneous bool
	status      string
	activityID  *int64
}

func newBlockPlanningTenant(t *testing.T, db *testpkg.DB) int64 {
	t.Helper()
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)
	return tenantID
}

func releaseBlockPlanningRoom(t *testing.T, db *testpkg.DB, roomID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewRaw(`UPDATE facilities.rooms SET is_open_room = TRUE WHERE id = ?;`, roomID).Exec(ctx)
	require.NoError(t, err, "release room %d", roomID)
}

func insertBlockPlanningActivity(t *testing.T, db *testpkg.DB, tenantID int64, name string, isSystem bool) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var categoryID int64
	err := db.NewRaw(`
		INSERT INTO activities.categories (tenant_id, name, is_system)
		VALUES (?, ?, ?)
		RETURNING id;
	`, tenantID, name+" Kategorie", isSystem).Scan(ctx, &categoryID)
	require.NoError(t, err, "insert category for %q", name)
	var activityID int64
	err = db.NewRaw(`
		INSERT INTO activities.groups (tenant_id, name, category_id, is_system)
		VALUES (?, ?, ?, ?)
		RETURNING id;
	`, tenantID, name, categoryID, isSystem).Scan(ctx, &activityID)
	require.NoError(t, err, "insert activity %q", name)
	return activityID
}

func insertBlockPlanningInstance(t *testing.T, db *testpkg.DB, tenantID, roomID int64, instance blockPlanningInstance) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	status := instance.status
	if status == "" {
		status = "planned"
	}
	_, err := db.NewRaw(`
		INSERT INTO schedule.activity_instances
			(tenant_id, date, title, start_time, end_time, room_id, activity_group_id, is_spontaneous, status)
		VALUES (?, ?::date, 'Block', '13:00', '14:00', ?, ?, ?, ?);
	`, tenantID, instance.date, roomID, instance.activityID, instance.spontaneous, status).Exec(ctx)
	require.NoError(t, err, "insert instance in room %d", roomID)
}

// releasedSchulhof stages a released canonical Schulhof, the state 1.15.372
// left behind.
func releasedSchulhof(t *testing.T, db *testpkg.DB, tenantID int64) int64 {
	t.Helper()
	roomID := insertOpenReleaseTestRoom(t, db, tenantID, "Schulhof", true)
	releaseBlockPlanningRoom(t, db, roomID)
	return roomID
}

func TestSchulhofReleaseBlockPlanning_RevokesOnlyYardsWithPlannedBlocks(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	// A school running its GT blocks in the yard (OGS am Berg).
	blockTenant := newBlockPlanningTenant(t, db)
	blockYard := releasedSchulhof(t, db, blockTenant)
	gt1 := insertBlockPlanningActivity(t, db, blockTenant, "GT 1", false)
	insertBlockPlanningInstance(t, db, blockTenant, blockYard, blockPlanningInstance{date: blockPlanningFutureDate, activityID: &gt1})

	// A planned instance without an activity is still a block.
	titleTenant := newBlockPlanningTenant(t, db)
	titleYard := releasedSchulhof(t, db, titleTenant)
	insertBlockPlanningInstance(t, db, titleTenant, titleYard, blockPlanningInstance{date: blockPlanningFutureDate})

	// A kiosk school: the yard only hosts its Freispiel system activity.
	kioskTenant := newBlockPlanningTenant(t, db)
	kioskYard := releasedSchulhof(t, db, kioskTenant)
	freispiel := insertBlockPlanningActivity(t, db, kioskTenant, "Schulhof Freispiel", true)
	insertBlockPlanningInstance(t, db, kioskTenant, kioskYard, blockPlanningInstance{date: blockPlanningFutureDate, activityID: &freispiel})

	// Spontaneous, past and cancelled instances do not describe planned use.
	incidentalTenant := newBlockPlanningTenant(t, db)
	incidentalYard := releasedSchulhof(t, db, incidentalTenant)
	football := insertBlockPlanningActivity(t, db, incidentalTenant, "Fußball", false)
	insertBlockPlanningInstance(t, db, incidentalTenant, incidentalYard, blockPlanningInstance{date: blockPlanningFutureDate, activityID: &football, spontaneous: true})
	insertBlockPlanningInstance(t, db, incidentalTenant, incidentalYard, blockPlanningInstance{date: blockPlanningPastDate, activityID: &football})
	insertBlockPlanningInstance(t, db, incidentalTenant, incidentalYard, blockPlanningInstance{date: blockPlanningFutureDate, activityID: &football, status: "cancelled"})

	// Blocks in another room of the school leave the yard alone.
	otherRoomTenant := newBlockPlanningTenant(t, db)
	otherRoomYard := releasedSchulhof(t, db, otherRoomTenant)
	groupRoom := insertOpenReleaseTestRoom(t, db, otherRoomTenant, "Gruppenraum 1", false)
	gt2 := insertBlockPlanningActivity(t, db, otherRoomTenant, "GT 2", false)
	insertBlockPlanningInstance(t, db, otherRoomTenant, groupRoom, blockPlanningInstance{date: blockPlanningFutureDate, activityID: &gt2})

	// A school's own room named Schulhof was never provisioned: not ours to touch.
	selfMadeTenant := newBlockPlanningTenant(t, db)
	selfMadeYard := insertOpenReleaseTestRoom(t, db, selfMadeTenant, "Schulhof", false)
	releaseBlockPlanningRoom(t, db, selfMadeYard)
	gt3 := insertBlockPlanningActivity(t, db, selfMadeTenant, "GT 3", false)
	insertBlockPlanningInstance(t, db, selfMadeTenant, selfMadeYard, blockPlanningInstance{date: blockPlanningFutureDate, activityID: &gt3})

	require.NoError(t, schulhofReleaseBlockPlanningUp(ctx, db))

	assert.False(t, roomIsOpenRoom(t, db, blockYard), "a yard with planned blocks loses the release")
	assert.False(t, roomIsOpenRoom(t, db, titleYard), "a planned instance without an activity counts as a block")
	assert.True(t, roomIsOpenRoom(t, db, kioskYard), "a kiosk yard keeps the release")
	assert.True(t, roomIsOpenRoom(t, db, incidentalYard), "spontaneous, past and cancelled instances keep the release")
	assert.True(t, roomIsOpenRoom(t, db, otherRoomYard), "blocks in other rooms keep the yard released")
	assert.True(t, roomIsOpenRoom(t, db, selfMadeYard), "a self-made Schulhof is left untouched")
}

// TestSchulhofReleaseBlockPlanning_IsIdempotentAndNeverReleases guards the
// re-run path and the direction of the change: an unreleased yard with
// planned blocks stays unreleased, and a second pass changes nothing.
func TestSchulhofReleaseBlockPlanning_IsIdempotentAndNeverReleases(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := newBlockPlanningTenant(t, db)
	yard := insertOpenReleaseTestRoom(t, db, tenantID, "Schulhof", true)
	gt := insertBlockPlanningActivity(t, db, tenantID, "GT 4", false)
	insertBlockPlanningInstance(t, db, tenantID, yard, blockPlanningInstance{date: blockPlanningFutureDate, activityID: &gt})

	kioskTenant := newBlockPlanningTenant(t, db)
	kioskYard := releasedSchulhof(t, db, kioskTenant)

	require.NoError(t, schulhofReleaseBlockPlanningUp(ctx, db))
	require.NoError(t, schulhofReleaseBlockPlanningUp(ctx, db))

	assert.False(t, roomIsOpenRoom(t, db, yard), "an unreleased yard stays unreleased")
	assert.True(t, roomIsOpenRoom(t, db, kioskYard), "a second pass keeps the kiosk yard released")
}
