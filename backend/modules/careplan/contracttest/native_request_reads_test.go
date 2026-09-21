package contracttest_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestNativePendingRequestRemainsVisibleWhenDiffFails(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Pending", "Visible", "1a")
	parent := testpkg.CreateTestAccount(t, db, "parent")
	owner := careplantest.NewCarePlan(t, db)
	b := &requestDiffBoundary{fail: errors.New("directory unavailable")}
	diffs, err := compose.NewRequestDiffs(b, b, b, nil)
	require.NoError(t, err)
	reads, err := compose.NewRequestReads(owner, diffs, nil)
	require.NoError(t, err)
	pending, diff, err := reads.GetPendingForStudent(ctx, student.ID)
	require.NoError(t, err)
	require.Nil(t, pending)
	require.Nil(t, diff)
	require.Empty(t, b.calls)
	request, err := owner.CreateCareScheduleRequest(ctx, carerequests.Request{StudentID: student.ID, SubmittedBy: parent.ID, RequestKind: "weekly_schedule", Status: "pending", Payload: json.RawMessage(`{"weekdays":[{"weekday":1,"pickup":"14:00"}]}`)})
	require.NoError(t, err)
	pending, diff, err = reads.GetPendingForStudent(ctx, student.ID)
	require.NoError(t, err)
	require.NotNil(t, pending)
	require.Equal(t, request.ID, pending.ID)
	require.Equal(t, "pending", pending.Status)
	require.Nil(t, diff)
	require.Equal(t, []string{"student"}, b.calls)
}
