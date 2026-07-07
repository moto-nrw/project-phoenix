// Tests for the is_system filtering on the rooms list endpoints
// (issue #923): auto-provisioned system rooms (Schulhof, WC) are hidden
// from staff-facing lists by default and only returned with
// ?include_system=true.
package rooms_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// markRoomAsSystem flags a room as system infrastructure, mirroring what
// the IoT auto-provisioning code does at creation time.
func markRoomAsSystem(t *testing.T, db *bun.DB, roomID int64) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("facilities.rooms").
		Set("is_system = TRUE").
		Where("id = ?", roomID).
		Exec(context.Background())
	require.NoError(t, err, "failed to flag room as system")
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

func TestListRooms_ExcludesSystemRoomsByDefault(t *testing.T) {
	tc := setupTestContext(t)

	normal := testpkg.CreateTestRoom(t, tc.db, "NormalRoom")
	system := testpkg.CreateTestRoom(t, tc.db, "SystemRoom")
	defer testpkg.CleanupActivityFixtures(t, tc.db, normal.ID, system.ID)

	markRoomAsSystem(t, tc.db, system.ID)

	t.Run("default_excludes_system", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/", nil)
		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		ids := roomResponseIDs(t, rr.Body.Bytes())
		assert.True(t, ids[normal.ID], "normal room should be listed")
		assert.False(t, ids[system.ID], "system room must be hidden by default")
	})

	t.Run("include_system_returns_system", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/?include_system=true", nil)
		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		ids := roomResponseIDs(t, rr.Body.Bytes())
		assert.True(t, ids[normal.ID], "normal room should be listed")
		assert.True(t, ids[system.ID], "system room must appear with include_system=true")
	})
}

func TestGetAvailableRooms_ExcludesSystemRoomsByDefault(t *testing.T) {
	tc := setupTestContext(t)

	normal := testpkg.CreateTestRoom(t, tc.db, "AvailNormalRoom")
	system := testpkg.CreateTestRoom(t, tc.db, "AvailSystemRoom")
	defer testpkg.CleanupActivityFixtures(t, tc.db, normal.ID, system.ID)

	markRoomAsSystem(t, tc.db, system.ID)

	t.Run("default_excludes_system", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/available", nil)
		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		ids := roomResponseIDs(t, rr.Body.Bytes())
		assert.True(t, ids[normal.ID], "normal room should be available")
		assert.False(t, ids[system.ID], "system room must be hidden by default")
	})

	t.Run("include_system_returns_system", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/available?include_system=true", nil)
		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		ids := roomResponseIDs(t, rr.Body.Bytes())
		assert.True(t, ids[normal.ID], "normal room should be available")
		assert.True(t, ids[system.ID], "system room must appear with include_system=true")
	})
}
