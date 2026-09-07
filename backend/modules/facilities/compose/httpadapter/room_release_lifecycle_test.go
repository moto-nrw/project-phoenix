package httpadapter

// The lifecycle promise of #3064: "Deaktivieren beendet weder Besuche noch
// Betreuungen." Removing a release changes the room's standing configuration
// and nothing else — children who are in the room stay in it, and the room
// itself stays reachable and editable under the ordinary rules.

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func openVisitCount(t *testing.T, db *bun.DB, activeGroupID int64) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var count int
	err := db.NewRaw(
		`SELECT COUNT(*) FROM active.visits WHERE active_group_id = ? AND exit_time IS NULL;`,
		activeGroupID,
	).Scan(ctx, &count)
	require.NoError(t, err, "count open visits for active group %d", activeGroupID)
	return count
}

func activeGroupIsOpen(t *testing.T, db *bun.DB, activeGroupID int64) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var open bool
	err := db.NewRaw(
		`SELECT end_time IS NULL FROM active.groups WHERE id = ?;`,
		activeGroupID,
	).Scan(ctx, &open)
	require.NoError(t, err, "load end_time for active group %d", activeGroupID)
	return open
}

func TestRevokingAReleaseEndsNeitherVisitsNorSessions(t *testing.T) {
	t.Parallel()
	tc := setupRoomsRoute(t)
	tenantID := newRoomFilterTestTenant(t, tc.db)
	claims := testutil.AdminTestClaimsForTenant(1, tenantID)

	room := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "ReleaseLifecycle")
	releaseRoom(t, tc.db, tenantID, room.ID, true)

	// A running session in that room with a child currently inside it.
	activityGroup := testpkg.CreateTestActivityGroupForTenant(t, tc.db, tenantID, "Freispiel")
	activeGroup := testpkg.CreateTestActiveGroupWithIDsForTenant(
		t, tc.db, tenantID, activityGroup.ID, room.ID,
	)
	student := testpkg.CreateTestStudentForTenant(t, tc.db, tenantID, "Mara", "Muster", "1a")
	visit := testpkg.CreateTestVisitForTenant(
		t, tc.db, tenantID, student.ID, activeGroup.ID, time.Now().Add(-time.Hour), nil,
	)
	require.Positive(t, visit.ID)
	require.Equal(t, 1, openVisitCount(t, tc.db, activeGroup.ID))

	// The administrator switches the room off.
	req := testutil.NewRequest(
		"PUT",
		"/"+itoa(room.ID),
		bytes.NewReader([]byte(roomReleasePayload(room.Name, false))),
	)
	req.Header.Set("Content-Type", "application/json")
	rr := testutil.ExecuteWithAuthPermissions(
		t, tc.router, req, claims, []string{permissions.RoomsRead, permissions.RoomsUpdate},
	)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	require.False(t, roomIsReleased(t, tc.db, tenantID, room.ID))

	assert.Equal(t, 1, openVisitCount(t, tc.db, activeGroup.ID),
		"removing a release must not close a child's room stay")
	assert.True(t, activeGroupIsOpen(t, tc.db, activeGroup.ID),
		"removing a release must not end the running session either")

	// And the room is still reachable and editable under the ordinary rules.
	get := testutil.NewRequest("GET", "/"+itoa(room.ID), nil)
	getRR := testutil.ExecuteWithAuth(t, tc.router, get, claims)
	assert.Equal(t, http.StatusOK, getRR.Code,
		"a deactivated room must stay reachable, not disappear")
}
