package active_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	activeAPI "github.com/moto-nrw/project-phoenix/api/active"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBinaryCreateVisitPreservesSuccessfulNoOp(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	ctx := testpkg.Ctx(t)
	require.NoError(t, tc.resource.SettingsService.SetValue(ctx, configModel.KeyPresenceMode, "binary", nil, nil))
	student := testpkg.CreateTestStudent(t, tc.db, "Binary", "Visit", "3a")
	activity := testpkg.CreateTestActivityGroup(t, tc.db, "Binary visit")
	room := testpkg.CreateTestRoom(t, tc.db, "Binary visit")
	group := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
	queries := tc.resource.Presence
	// A successful no-op must not attempt any visit readback.
	tc.resource.Presence = failingVisitReadback{PresenceQueries: queries}
	at := time.Now().Truncate(time.Second)
	req := testutil.NewJSONRequest(t, http.MethodPost, "/active/visits", map[string]any{
		"student_id": student.ID, "active_group_id": group.ID, "check_in_time": at,
	})
	rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(1), []string{permissions.GroupsCreate})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var response struct {
		Data    activeAPI.VisitResponse `json:"data"`
		Message string                  `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	assert.Zero(t, response.Data.ID)
	assert.Equal(t, student.ID, response.Data.StudentID)
	assert.Equal(t, group.ID, response.Data.ActiveGroupID)
	assert.True(t, at.Equal(response.Data.CheckInTime))
	assert.Equal(t, "Visit created successfully", response.Message)
	visits, err := queries.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	assert.Empty(t, visits)
}
