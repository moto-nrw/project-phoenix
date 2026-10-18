package supervisiondashboard

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortSessionsByRoomKeepsReadOrderWithinARoom(t *testing.T) {
	t.Parallel()
	first, second, third := int64(31), int64(37), int64(41)
	sessions := []Session{{ID: first, RoomName: "Werkraum"}, {ID: second, RoomName: "Aula"}, {ID: third, RoomName: "Werkraum"}}

	SortSessionsByRoom(sessions, strings.Compare)

	ids := make([]int64, 0, len(sessions))
	for _, session := range sessions {
		ids = append(ids, session.ID)
	}
	assert.Equal(t, []int64{second, first, third}, ids)
}
