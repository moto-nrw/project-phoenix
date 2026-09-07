package httpadapter

// Inbound contract for the room release (#3064): the release is an
// administrative decision, so a caller without the room-update permission must
// be refused server-side — not merely shown a form without the switch.

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func itoa(id int64) string { return strconv.FormatInt(id, 10) }

// roomIsReleased reads the stored release straight from the row, so the
// assertion does not lean on the same read path the write just used.
func roomIsReleased(t *testing.T, db *bun.DB, tenantID, roomID int64) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var released bool
	err := db.NewRaw(
		`SELECT is_open_room FROM facilities.rooms WHERE id = ? AND tenant_id = ?;`,
		roomID, tenantID,
	).Scan(ctx, &released)
	require.NoError(t, err, "load is_open_room for room %d", roomID)
	return released
}

func roomReleasePayload(name string, released bool) string {
	if released {
		return `{"name":"` + name + `","is_open_room":true}`
	}
	return `{"name":"` + name + `","is_open_room":false}`
}

func TestRoomRelease_RequiresTheRoomUpdatePermission(t *testing.T) {
	t.Parallel()
	tc := setupRoomsRoute(t)
	tenantID := newRoomFilterTestTenant(t, tc.db)
	claims := testutil.AdminTestClaimsForTenant(1, tenantID)

	room := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "ReleaseGuarded")

	t.Run("read-only caller cannot release a room", func(t *testing.T) {
		req := testutil.NewRequest(
			"PUT",
			"/"+itoa(room.ID),
			bytes.NewReader([]byte(roomReleasePayload(room.Name, true))),
		)
		req.Header.Set("Content-Type", "application/json")
		rr := testutil.ExecuteWithAuthPermissions(
			t, tc.router, req, claims, []string{permissions.RoomsRead},
		)
		assert.Equal(t, http.StatusForbidden, rr.Code,
			"rooms:read alone must not be enough to release a room. Body: %s", rr.Body.String())
		assert.False(t, roomIsReleased(t, tc.db, tenantID, room.ID),
			"the refused request must not have written the release")
	})

	t.Run("an authorized administrator can release it", func(t *testing.T) {
		req := testutil.NewRequest(
			"PUT",
			"/"+itoa(room.ID),
			bytes.NewReader([]byte(roomReleasePayload(room.Name, true))),
		)
		req.Header.Set("Content-Type", "application/json")
		rr := testutil.ExecuteWithAuthPermissions(
			t, tc.router, req, claims, []string{permissions.RoomsRead, permissions.RoomsUpdate},
		)
		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		assert.True(t, roomIsReleased(t, tc.db, tenantID, room.ID))
	})

	t.Run("and can revoke it again", func(t *testing.T) {
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
		assert.False(t, roomIsReleased(t, tc.db, tenantID, room.ID))
	})
}

// TestRoomRelease_IsCarriedOnTheReadContract pins the wire field the client
// maps: without it the form could never show the stored state.
func TestRoomRelease_IsCarriedOnTheReadContract(t *testing.T) {
	t.Parallel()
	tc := setupRoomsRoute(t)
	tenantID := newRoomFilterTestTenant(t, tc.db)
	claims := testutil.AdminTestClaimsForTenant(1, tenantID)

	room := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "ReleaseOnWire")
	releaseRoom(t, tc.db, tenantID, room.ID, true)

	req := testutil.NewRequest("GET", "/"+itoa(room.ID), nil)
	rr := testutil.ExecuteWithAuth(t, tc.router, req, claims)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"is_open_room":true`,
		"the read contract must carry the release so the room form can show it")
}
