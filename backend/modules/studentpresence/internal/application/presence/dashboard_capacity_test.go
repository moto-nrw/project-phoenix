package presence

import (
	"testing"

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
			&dashboardRoomData{roomStudentsMap: map[int64]map[int64]struct{}{}},
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
