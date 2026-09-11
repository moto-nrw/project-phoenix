package active_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	activeAPI "github.com/moto-nrw/project-phoenix/api/active"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVisitListPresenceCutoverPreservesStatusAndGroupFilters(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	student := testpkg.CreateTestStudent(t, tc.db, "Presence", "HTTP", "3a")
	activity := testpkg.CreateTestActivityGroup(t, tc.db, "Presence HTTP")
	room := testpkg.CreateTestRoom(t, tc.db, "Presence HTTP")
	group := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
	otherGroup := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
	otherStudent := testpkg.CreateTestStudent(t, tc.db, "Other", "HTTP", "3a")
	at := time.Now().Add(-time.Hour)
	exit := at.Add(10 * time.Minute)
	closed := testpkg.CreateTestVisit(t, tc.db, student.ID, group.ID, at, &exit)
	open := testpkg.CreateTestVisit(t, tc.db, student.ID, group.ID, at.Add(20*time.Minute), nil)
	testpkg.CreateTestVisit(t, tc.db, otherStudent.ID, otherGroup.ID, at, nil)
	for _, test := range []struct {
		status string
		id     int64
		active bool
	}{
		{"true", open.ID, true}, {"false", closed.ID, false},
	} {
		t.Run(test.status, func(t *testing.T) {
			url := fmt.Sprintf("/active/visits?active=%s&active_group_ids=%d", test.status, group.ID)
			req := testutil.NewJSONRequest(t, http.MethodGet, url, nil)
			rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(1), []string{permissions.GroupsRead})
			testutil.AssertSuccessResponse(t, rr, http.StatusOK)
			var response struct {
				Data    []activeAPI.VisitResponse `json:"data"`
				Message string                    `json:"message"`
			}
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
			require.Len(t, response.Data, 1)
			assert.Equal(t, "Visits retrieved successfully", response.Message)
			assert.Equal(t, test.id, response.Data[0].ID)
			assert.Equal(t, student.ID, response.Data[0].StudentID)
			assert.Equal(t, group.ID, response.Data[0].ActiveGroupID)
			assert.Equal(t, test.active, response.Data[0].IsActive)
			assert.NotContains(t, rr.Body.String(), "tenant_id")
			if test.active {
				assert.Nil(t, response.Data[0].CheckOutTime)
			} else {
				require.NotNil(t, response.Data[0].CheckOutTime)
				assert.WithinDuration(t, exit, *response.Data[0].CheckOutTime, time.Microsecond)
			}
		})
	}
}
