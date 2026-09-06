package active_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	activeAPI "github.com/moto-nrw/project-phoenix/api/active"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingVisitReadback struct{ activeAPI.PresenceQueries }

func (failingVisitReadback) FindVisit(context.Context, int64) (*studentpresence.Visit, error) {
	return nil, errors.New("private readback failure")
}

func TestVisitEndReadbackFailureRollsBackAndCanBeRetried(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	queries := tc.resource.Presence
	student := testpkg.CreateTestStudent(t, tc.db, "Visit", "Rollback", "3a")
	activity := testpkg.CreateTestActivityGroup(t, tc.db, "Visit rollback")
	room := testpkg.CreateTestRoom(t, tc.db, "Visit rollback")
	group := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
	visit := testpkg.CreateTestVisit(t, tc.db, student.ID, group.ID, time.Now().Add(-time.Hour), nil)
	tc.resource.Presence = failingVisitReadback{PresenceQueries: queries}
	request := func() int {
		t.Helper()
		req := testutil.NewJSONRequest(t, http.MethodPost, fmt.Sprintf("/active/visits/%d/end", visit.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(1), []string{permissions.GroupsUpdate})
		assert.NotContains(t, rr.Body.String(), "private readback failure")
		return rr.Code
	}
	require.Equal(t, http.StatusInternalServerError, request())
	stored, err := queries.FindVisit(testpkg.Ctx(t), visit.ID)
	require.NoError(t, err)
	assert.Nil(t, stored.ExitTime, "the failed response must roll back the authoritative visit write")
	tc.resource.Presence = queries
	require.Equal(t, http.StatusOK, request())
	stored, err = queries.FindVisit(testpkg.Ctx(t), visit.ID)
	require.NoError(t, err)
	assert.NotNil(t, stored.ExitTime)
}
