package presence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeService "github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type historyRooms struct {
	rows map[int64]string
	err  error
}

func (r *historyRooms) names(context.Context, []int64) (map[int64]string, error) {
	return r.rows, r.err
}

func TestVisitHistoryPreservesInclusiveEntryRangeAndRoomLookupErrors(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "History", "Range", "3a")
	room := testpkg.CreateTestRoom(t, db, "History Room")
	activity := testpkg.CreateTestActivityGroup(t, db, "History Activity")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	start := time.Date(2032, 3, 2, 9, 0, 0, 0, timezone.Berlin)
	end := start.Add(time.Hour)
	for _, entry := range []time.Time{start.Add(-time.Minute), start, end, end.Add(time.Minute)} {
		exit := entry.Add(time.Minute)
		testpkg.CreateTestVisit(t, db, student.ID, group.ID, entry, &exit)
	}
	rooms := &historyRooms{rows: map[int64]string{room.ID: room.Name}}
	svc := activeService.NewStudentHistoryService(testSchoolPresence(t, db), rooms.names)
	rows, err := svc.GetVisitsByStudentAndTimeRange(testpkg.Ctx(t), student.ID, start, end)
	require.NoError(t, err)
	require.Len(t, rows, 2, "history selects entry times, including both boundaries")
	assert.True(t, rows[0].EntryTime.Equal(start))
	assert.True(t, rows[1].EntryTime.Equal(end))
	for _, row := range rows {
		require.NotNil(t, row.RoomID)
		assert.Equal(t, room.ID, *row.RoomID)
		assert.Equal(t, room.Name, row.RoomName)
	}
	rooms.err = errors.New("history room query failed")
	rows, err = svc.GetVisitsByStudentAndTimeRange(testpkg.Ctx(t), student.ID, start, end)
	require.ErrorIs(t, err, rooms.err)
	assert.Nil(t, rows, "failed enrichment must not return partial room history")
}
