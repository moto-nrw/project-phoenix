package presence

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/stretchr/testify/require"
)

func TestCurrentActivityCapacityPreservesUnlimitedSemantics(t *testing.T) {
	t.Parallel()
	for _, capacity := range []int{-1, 0, 12} {
		template := &ports.SessionActivity{ID: 1, MaxParticipants: capacity}
		rows := buildCurrentActivities(
			[]*ports.SessionActivity{template},
			[]*ports.ActiveGroup{{GroupID: &template.ID, RoomID: 2}},
			map[int64][]studentpresence.Visit{},
		)
		require.Len(t, rows, 1)
		if capacity <= 0 {
			require.Nil(t, rows[0].MaxCapacity)
		} else {
			require.NotNil(t, rows[0].MaxCapacity)
			require.Equal(t, capacity, *rows[0].MaxCapacity)
		}
		require.Equal(t, "active", rows[0].Status)
	}
}

// TestCurrentActivityCountsTheSessionNotTheRoom pins the count the
// overbooked state rests on (#3634): the open visits of the activity's own
// session, the same number the terminal compares with the limit. Two
// sessions sharing a room must not inherit each other's children.
func TestCurrentActivityCountsTheSessionNotTheRoom(t *testing.T) {
	t.Parallel()
	football := &ports.SessionActivity{ID: 1, Name: "Fußball", MaxParticipants: 2}
	crafts := &ports.SessionActivity{ID: 2, Name: "Basteln", MaxParticipants: 2}
	footballSession := &ports.ActiveGroup{ID: 10, GroupID: &football.ID, RoomID: 7}
	craftsSession := &ports.ActiveGroup{ID: 11, GroupID: &crafts.ID, RoomID: 7}
	visits := map[int64][]studentpresence.Visit{
		10: {{StudentID: 100, ActiveGroupID: 10}, {StudentID: 101, ActiveGroupID: 10}, {StudentID: 102, ActiveGroupID: 10}},
		11: {{StudentID: 103, ActiveGroupID: 11}},
	}

	rows := buildCurrentActivities(
		[]*ports.SessionActivity{football, crafts},
		[]*ports.ActiveGroup{footballSession, craftsSession},
		visits,
	)

	require.Len(t, rows, 2)
	require.Equal(t, "Fußball", rows[0].Name)
	require.Equal(t, 3, rows[0].Participants)
	require.Equal(t, "overbooked", rows[0].Status)
	require.Equal(t, "Basteln", rows[1].Name)
	require.Equal(t, 1, rows[1].Participants)
	require.Equal(t, "active", rows[1].Status)
}

// TestActiveGroupsSummaryCarriesTheActivityLimit pins the "Laufende
// Betreuung" rows (#3634): a session whose activity has a limit reports its
// own open visits against that limit; a session without one keeps the room's
// count and carries no limit.
func TestActiveGroupsSummaryCarriesTheActivityLimit(t *testing.T) {
	t.Parallel()
	football := &ports.SessionActivity{ID: 1, Name: "Fußball", MaxParticipants: 2}
	reading := &ports.SessionActivity{ID: 2, Name: "Lesen"}
	footballSession := &ports.ActiveGroup{ID: 10, GroupID: &football.ID, RoomID: 7}
	readingSession := &ports.ActiveGroup{ID: 11, GroupID: &reading.ID, RoomID: 7}
	visits := map[int64][]studentpresence.Visit{
		10: {{StudentID: 100}, {StudentID: 101}, {StudentID: 102}},
		11: {{StudentID: 103}},
	}
	roomData := &dashboardRoomData{
		roomByID:        map[int64]*ports.SessionRoom{},
		roomStudentsMap: map[int64]map[int64]struct{}{7: {100: {}, 101: {}, 102: {}, 103: {}}},
	}

	rows := buildActiveGroupsSummary(
		[]*ports.ActiveGroup{footballSession, readingSession},
		map[int64]*ports.SessionActivity{1: football, 2: reading},
		roomData,
		visits,
	)

	require.Len(t, rows, 2)
	require.NotNil(t, rows[0].MaxCapacity)
	require.Equal(t, 2, *rows[0].MaxCapacity)
	require.Equal(t, 3, rows[0].StudentCount, "the session's children, not the room's")
	require.Nil(t, rows[1].MaxCapacity)
	require.Equal(t, 4, rows[1].StudentCount, "without a limit the room's count stays")
}
