// Tests for the is_system filtering on the rooms list endpoints: Schulhof is
// available to staff planning, while infrastructure rooms stay hidden unless
// the caller explicitly requests all system rooms.
package rooms_test

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/constants"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

var roomFilterTenantCounter int64 = 880_000 + time.Now().UnixNano()%100_000

func newRoomFilterTestTenant(t *testing.T, db *bun.DB) int64 {
	t.Helper()
	tenantID := atomic.AddInt64(&roomFilterTenantCounter, 1)
	testpkg.EnsureTestTenant(t, db, tenantID)
	t.Cleanup(func() { testpkg.CleanupTenantTestData(t, db, tenantID) })
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
	tc := setupTestContext(t)
	tenantID := newRoomFilterTestTenant(t, tc.db)
	claims := testutil.AdminTestClaimsForTenant(1, tenantID)

	normal := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "NormalRoom")
	system := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "SystemRoom")
	wc := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "WCRoom")
	schulhof := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "SchulhofRoom")

	markRoomAsSystem(t, tc.db, tenantID, system.ID)
	markRoomAsSystem(t, tc.db, tenantID, schulhof.ID)
	setRoomNameAndBuilding(t, tc.db, tenantID, wc.ID, constants.WCRoomName, "Sanitär")
	setRoomNameAndBuilding(t, tc.db, tenantID, schulhof.ID, constants.SchulhofRoomName, "Außengelände")

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
		setRoomNameAndBuilding(t, tc.db, tenantID, wc.ID, constants.WCRoomAliasName, "Sanitär")
		req := testutil.NewRequest("GET", "/", nil)
		rr := testutil.ExecuteWithAuth(t, tc.router, req, claims)
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		ids := roomResponseIDs(t, rr.Body.Bytes())
		assert.False(t, ids[wc.ID], "Toilette must be hidden even without an is_system flag")
	})
}

func TestGetAvailableRooms_IncludesOnlySchulhofFromSystemRoomsByDefault(t *testing.T) {
	tc := setupTestContext(t)
	tenantID := newRoomFilterTestTenant(t, tc.db)
	claims := testutil.AdminTestClaimsForTenant(1, tenantID)

	normal := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "AvailNormalRoom")
	system := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "AvailSystemRoom")
	wc := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "AvailWCRoom")
	schulhof := testpkg.CreateTestRoomForTenant(t, tc.db, tenantID, "AvailSchulhofRoom")

	markRoomAsSystem(t, tc.db, tenantID, system.ID)
	markRoomAsSystem(t, tc.db, tenantID, schulhof.ID)
	setRoomNameAndBuilding(t, tc.db, tenantID, wc.ID, constants.WCRoomName, "Sanitär")
	setRoomNameAndBuilding(t, tc.db, tenantID, schulhof.ID, constants.SchulhofRoomName, "Außengelände")

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
		setRoomNameAndBuilding(t, tc.db, tenantID, wc.ID, constants.WCRoomAliasName, "Sanitär")
		req := testutil.NewRequest("GET", "/available", nil)
		rr := testutil.ExecuteWithAuth(t, tc.router, req, claims)
		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		ids := roomResponseIDs(t, rr.Body.Bytes())
		assert.False(t, ids[wc.ID], "Toilette must be hidden even without an is_system flag")
	})
}
