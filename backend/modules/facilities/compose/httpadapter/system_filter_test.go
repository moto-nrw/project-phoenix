// Tests for the is_system filtering on the rooms list endpoints: Schulhof is
// available to staff planning, while infrastructure rooms stay hidden unless
// the caller explicitly requests all system rooms.
package httpadapter

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func newRoomFilterTestTenant(t *testing.T, db *bun.DB) int64 {
	t.Helper()
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	return tenantID
}

// markRoomAsSystem flags a room as system infrastructure, mirroring what
// the IoT auto-provisioning code does at creation time.
func markRoomAsSystem(t *testing.T, db *bun.DB, tenantID, roomID int64) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("facilities.rooms").
		Set("is_system = TRUE").
		Where("id = ?", roomID).
		Where("tenant_id = ?", tenantID).
		Exec(context.Background())
	require.NoError(t, err, "failed to flag room as system")
}

// releaseRoom marks a room as a permanently released open room, mirroring what
// migration 1.15.372 and the Schulhof provisioning do. Staff visibility does
// not depend on it (ADR 0019); the open-room filter does.
func releaseRoom(t *testing.T, db *bun.DB, tenantID, roomID int64, released bool) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("facilities.rooms").
		Set("is_open_room = ?", released).
		Where("id = ?", roomID).
		Where("tenant_id = ?", tenantID).
		Exec(context.Background())
	require.NoError(t, err, "failed to set room release")
}

func setRoomNameAndBuilding(t *testing.T, db *bun.DB, tenantID, roomID int64, name, building string) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("facilities.rooms").
		Set("name = ?", name).
		Set("building = ?", building).
		Where("id = ?", roomID).
		Where("tenant_id = ?", tenantID).
		Exec(context.Background())
	require.NoError(t, err, "failed to configure system room")
}

// roomResponseIDs extracts the "id" fields from a list response's data array.
func roomResponseIDs(t *testing.T, body []byte) map[int64]bool {
	t.Helper()
	response := testutil.ParseJSONResponse(t, body)
	data, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")

	ids := make(map[int64]bool, len(data))
	for _, entry := range data {
		item, ok := entry.(map[string]interface{})
		require.True(t, ok, "Expected list entry to be an object")
		id, ok := item["id"].(float64)
		require.True(t, ok, "Expected entry id to be a number")
		ids[int64(id)] = true
	}
	return ids
}

func TestListRooms_IncludesOnlySchulhofFromSystemRoomsByDefault(t *testing.T) {
	t.Parallel()
	tc := setupRoomsRoute(t)
	tenantID := newRoomFilterTestTenant(t, tc.db)
	claims := testutil.AdminTestClaimsForTenant(1, tenantID)

	normal := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "NormalRoom")
	system := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "SystemRoom")
	wc := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "WCRoom")
	schulhof := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "SchulhofRoom")

	markRoomAsSystem(t, tc.db, tenantID, system.ID)
	markRoomAsSystem(t, tc.db, tenantID, schulhof.ID)
	setRoomNameAndBuilding(t, tc.db, tenantID, wc.ID, facilitiesModule.WCRoomName, "Sanitär")
	setRoomNameAndBuilding(t, tc.db, tenantID, schulhof.ID, facilitiesModule.SchulhofRoomName, "Außengelände")
	releaseRoom(t, tc.db, tenantID, schulhof.ID, true)

	t.Run("default_excludes_system", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/", nil)
		rr := testutil.ExecuteWithAuth(t, tc.router, req, claims)
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		ids := roomResponseIDs(t, rr.Body.Bytes())
		assert.True(t, ids[normal.ID], "normal room should be listed")
		assert.False(t, ids[system.ID], "system room must be hidden by default")
		assert.False(t, ids[wc.ID], "WC must be hidden even without an is_system flag")
		assert.True(t, ids[schulhof.ID], "Schulhof must be listed for staff planning")
	})

	t.Run("building_filter_still_applies_to_schulhof", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?building=Test%20Building", nil)
		rr := testutil.ExecuteWithAuth(t, tc.router, req, claims)
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		ids := roomResponseIDs(t, rr.Body.Bytes())
		assert.True(t, ids[normal.ID], "matching normal room should be listed")
		assert.False(t, ids[schulhof.ID], "Schulhof must not bypass the building filter")
	})

	t.Run("include_system_returns_system", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?include_system=true", nil)
		rr := testutil.ExecuteWithAuth(t, tc.router, req, claims)
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		ids := roomResponseIDs(t, rr.Body.Bytes())
		assert.True(t, ids[normal.ID], "normal room should be listed")
		assert.True(t, ids[system.ID], "system room must appear with include_system=true")
		assert.True(t, ids[wc.ID], "WC must appear with include_system=true")
		assert.True(t, ids[schulhof.ID], "Schulhof must appear with include_system=true")
	})

	t.Run("toilette_alias_is_hidden", func(t *testing.T) {
		setRoomNameAndBuilding(t, tc.db, tenantID, wc.ID, facilitiesModule.WCRoomAliasName, "Sanitär")
		req := testutil.NewRequest("GET", "/", nil)
		rr := testutil.ExecuteWithAuth(t, tc.router, req, claims)
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		ids := roomResponseIDs(t, rr.Body.Bytes())
		assert.False(t, ids[wc.ID], "Toilette must be hidden even without an is_system flag")
	})
}

func TestGetAvailableRooms_IncludesOnlySchulhofFromSystemRoomsByDefault(t *testing.T) {
	t.Parallel()
	tc := setupRoomsRoute(t)
	tenantID := newRoomFilterTestTenant(t, tc.db)
	claims := testutil.AdminTestClaimsForTenant(1, tenantID)

	normal := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "AvailNormalRoom")
	system := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "AvailSystemRoom")
	wc := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "AvailWCRoom")
	schulhof := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "AvailSchulhofRoom")

	markRoomAsSystem(t, tc.db, tenantID, system.ID)
	markRoomAsSystem(t, tc.db, tenantID, schulhof.ID)
	setRoomNameAndBuilding(t, tc.db, tenantID, wc.ID, facilitiesModule.WCRoomName, "Sanitär")
	setRoomNameAndBuilding(t, tc.db, tenantID, schulhof.ID, facilitiesModule.SchulhofRoomName, "Außengelände")
	releaseRoom(t, tc.db, tenantID, schulhof.ID, true)

	t.Run("default_excludes_system", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/available", nil)
		rr := testutil.ExecuteWithAuth(t, tc.router, req, claims)
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		ids := roomResponseIDs(t, rr.Body.Bytes())
		assert.True(t, ids[normal.ID], "normal room should be available")
		assert.False(t, ids[system.ID], "system room must be hidden by default")
		assert.False(t, ids[wc.ID], "WC must be hidden even without an is_system flag")
		assert.True(t, ids[schulhof.ID], "Schulhof must be available for staff planning")
	})

	t.Run("include_system_returns_system", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/available?include_system=true", nil)
		rr := testutil.ExecuteWithAuth(t, tc.router, req, claims)
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		ids := roomResponseIDs(t, rr.Body.Bytes())
		assert.True(t, ids[normal.ID], "normal room should be available")
		assert.True(t, ids[system.ID], "system room must appear with include_system=true")
		assert.True(t, ids[wc.ID], "WC must appear with include_system=true")
		assert.True(t, ids[schulhof.ID], "Schulhof must appear with include_system=true")
	})

	t.Run("toilette_alias_is_hidden", func(t *testing.T) {
		setRoomNameAndBuilding(t, tc.db, tenantID, wc.ID, facilitiesModule.WCRoomAliasName, "Sanitär")
		req := testutil.NewRequest("GET", "/available", nil)
		rr := testutil.ExecuteWithAuth(t, tc.router, req, claims)
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		ids := roomResponseIDs(t, rr.Body.Bytes())
		assert.False(t, ids[wc.ID], "Toilette must be hidden even without an is_system flag")
	})
}

// TestRoomLists_KeepTheSchulhofSelectableWithoutRelease pins ADR 0019: the
// open-room release decides whether the yard is an open room, not whether
// staff can pick it. Schools that plan blocks in the Schulhof revoke the
// release (#3276) and must still find the yard in the planner, in spontaneous
// activities and in the available-rooms list. A revoked release previously
// hid the yard from every one of those lists.
func TestRoomLists_KeepTheSchulhofSelectableWithoutRelease(t *testing.T) {
	t.Parallel()
	tc := setupRoomsRoute(t)
	tenantID := newRoomFilterTestTenant(t, tc.db)
	claims := testutil.AdminTestClaimsForTenant(1, tenantID)

	normal := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "RevokedNormalRoom")
	schulhof := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "RevokedSchulhofRoom")

	markRoomAsSystem(t, tc.db, tenantID, schulhof.ID)
	setRoomNameAndBuilding(t, tc.db, tenantID, schulhof.ID, facilitiesModule.SchulhofRoomName, "Außengelände")
	releaseRoom(t, tc.db, tenantID, schulhof.ID, true)

	t.Run("released_schulhof_is_listed", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/", nil)
		rr := testutil.ExecuteWithAuth(t, tc.router, req, claims)
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		ids := roomResponseIDs(t, rr.Body.Bytes())
		assert.True(t, ids[schulhof.ID], "a released Schulhof must be listed")
	})

	releaseRoom(t, tc.db, tenantID, schulhof.ID, false)

	t.Run("revoked_schulhof_stays_in_the_room_list", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/", nil)
		rr := testutil.ExecuteWithAuth(t, tc.router, req, claims)
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		ids := roomResponseIDs(t, rr.Body.Bytes())
		assert.True(t, ids[normal.ID], "unrelated rooms stay listed")
		assert.True(t, ids[schulhof.ID],
			"a Schulhof without release must stay selectable for planning")
	})

	t.Run("revoked_schulhof_stays_in_available_rooms", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/available", nil)
		rr := testutil.ExecuteWithAuth(t, tc.router, req, claims)
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		ids := roomResponseIDs(t, rr.Body.Bytes())
		assert.True(t, ids[schulhof.ID],
			"the available-rooms endpoint must apply the same rule")
	})

	t.Run("revoked_schulhof_is_not_an_open_room", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?is_open_room=true", nil)
		rr := testutil.ExecuteWithAuth(t, tc.router, req, claims)
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		ids := roomResponseIDs(t, rr.Body.Bytes())
		assert.False(t, ids[schulhof.ID],
			"visibility must not bring the yard back into the open-room navigation")
	})

	t.Run("revoked_schulhof_is_reachable_with_include_system", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?include_system=true", nil)
		rr := testutil.ExecuteWithAuth(t, tc.router, req, claims)
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		ids := roomResponseIDs(t, rr.Body.Bytes())
		assert.True(t, ids[schulhof.ID], "include_system still returns every room")
	})
}

// TestRoomLists_HideOtherSystemRoomsRegardlessOfRelease keeps the WC hidden
// even if a stale row carries a release, and keeps a non-Schulhof system room
// out of staff lists: only the yard is exempt by name.
func TestRoomLists_HideOtherSystemRoomsRegardlessOfRelease(t *testing.T) {
	t.Parallel()
	tc := setupRoomsRoute(t)
	tenantID := newRoomFilterTestTenant(t, tc.db)
	claims := testutil.AdminTestClaimsForTenant(1, tenantID)

	system := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "ReleasedSystemRoom")
	wc := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "ReleasedWCRoom")
	markRoomAsSystem(t, tc.db, tenantID, system.ID)
	markRoomAsSystem(t, tc.db, tenantID, wc.ID)
	setRoomNameAndBuilding(t, tc.db, tenantID, wc.ID, facilitiesModule.WCRoomName, "Sanitär")
	releaseRoom(t, tc.db, tenantID, system.ID, true)
	releaseRoom(t, tc.db, tenantID, wc.ID, true)

	for _, path := range []string{"/", "/available"} {
		t.Run(path, func(t *testing.T) {
			req := testutil.NewRequest("GET", path, nil)
			rr := testutil.ExecuteWithAuth(t, tc.router, req, claims)
			assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

			ids := roomResponseIDs(t, rr.Body.Bytes())
			assert.False(t, ids[system.ID], "a non-Schulhof system room stays hidden")
			assert.False(t, ids[wc.ID], "a WC stays hidden even with a stale release")
		})
	}
}
