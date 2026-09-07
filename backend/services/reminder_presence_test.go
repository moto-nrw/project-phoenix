package services

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/require"
)

type reminderVisitFacts struct {
	rows   []studentpresence.OpenVisitRoom
	err    error
	roomID int64
}

func (f *reminderVisitFacts) ListOpenVisitRooms(_ context.Context, roomID int64) ([]studentpresence.OpenVisitRoom, error) {
	f.roomID = roomID
	return f.rows, f.err
}

func TestReminderVisitReaderGroupsWholeSchoolFacts(t *testing.T) {
	t.Parallel()
	facts := &reminderVisitFacts{roomID: -1, rows: []studentpresence.OpenVisitRoom{
		{RoomID: 10, StudentID: 1}, {RoomID: 10, StudentID: 2}, {RoomID: 20, StudentID: 2},
	}}
	reader := reminderVisitReader{source: facts}
	rooms, err := reader.ListOpenVisitStudentIDsByRoom(context.Background())
	require.NoError(t, err)
	require.Zero(t, facts.roomID)
	require.Equal(t, map[int64][]int64{10: {1, 2}, 20: {2}}, rooms)

	facts.err = errors.New("presence unavailable")
	rooms, err = reader.ListOpenVisitStudentIDsByRoom(context.Background())
	require.ErrorIs(t, err, facts.err)
	require.Nil(t, rooms, "failed reads must not expose partial room facts")
}
