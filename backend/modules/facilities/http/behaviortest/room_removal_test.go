package httpadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type removalImpactBody struct {
	Data struct {
		Outcome       string `json:"outcome"`
		ActiveSession bool   `json:"active_session"`
		References    struct {
			UpcomingBlocks  int `json:"upcoming_blocks"`
			EducationGroups int `json:"education_groups"`
			PastBlocks      int `json:"past_blocks"`
		} `json:"references"`
	} `json:"data"`
}

// The removal impact names what still uses a room; DELETE refuses such a
// room with a code of its own instead of a 500, and archives a room only
// history names (#6845).
func TestRoomRemovalFollowsItsImpact(t *testing.T) {
	t.Parallel()
	tc := setupRoomsRoute(t)
	claims := testutil.AdminTestClaims(1)
	room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Removal %d", testpkg.Tenant(t)))
	group := testpkg.CreateTestEducationGroup(t, tc.db, "Removal group")
	_, err := tc.db.NewRaw(`UPDATE education.groups SET room_id = ? WHERE id = ?`, room.ID, group.ID).Exec(context.Background())
	require.NoError(t, err)
	upcoming := testpkg.CreateTestActivityInstance(t, tc.db, calendar.NewDate(2099, 3, 2), room.ID, testpkg.ActivityInstanceOpts{})
	testpkg.CreateTestActivityInstance(t, tc.db, calendar.NewDate(2020, 3, 2), room.ID, testpkg.ActivityInstanceOpts{})

	impact := readImpact(t, tc, room.ID)
	assert.Equal(t, "blocked", impact.Data.Outcome)
	assert.Equal(t, 1, impact.Data.References.EducationGroups)
	assert.Equal(t, 1, impact.Data.References.UpcomingBlocks)
	assert.Equal(t, 1, impact.Data.References.PastBlocks)

	rr := testutil.ExecuteWithAuth(t, tc.router, testutil.NewRequest(http.MethodDelete, fmt.Sprintf("/%d", room.ID), nil), claims)
	require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"code":"rooms.still_planned"`)
	assert.Contains(t, rr.Body.String(), `"education_groups":1`)

	_, err = tc.db.NewRaw(`UPDATE education.groups SET room_id = NULL WHERE id = ?`, group.ID).Exec(context.Background())
	require.NoError(t, err)
	_, err = tc.db.NewRaw(`UPDATE schedule.activity_instances SET status = 'cancelled' WHERE id = ?`, upcoming.ID).Exec(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "archive", readImpact(t, tc, room.ID).Data.Outcome)

	rr = testutil.ExecuteWithAuth(t, tc.router, testutil.NewRequest(http.MethodDelete, fmt.Sprintf("/%d", room.ID), nil), claims)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"outcome":"archive"`)

	rr = testutil.ExecuteWithAuth(t, tc.router, testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d", room.ID), nil), claims)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"archived_at"`, "the archived room still resolves for its history")

	rr = testutil.ExecuteWithAuth(t, tc.router, testutil.NewJSONRequest(t, http.MethodPut, fmt.Sprintf("/%d", room.ID), map[string]any{"name": "Neu"}), claims)
	require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"code":"rooms.archived"`)
}

func readImpact(t *testing.T, tc *testContext, roomID int64) removalImpactBody {
	t.Helper()
	rr := testutil.ExecuteWithAuth(t, tc.router, testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d/removal-impact", roomID), nil), testutil.AdminTestClaims(1))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var body removalImpactBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	return body
}
