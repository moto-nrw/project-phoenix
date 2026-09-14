package active

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDashboardGroupRoomsPreservesOptionalRoomSemantics(t *testing.T) {
	t.Parallel()
	assigned := &EducationGroupRoom{ID: 7}
	assigned.RoomID = &assigned.ID
	zero := &EducationGroupRoom{}
	zero.RoomID = &zero.ID
	unassigned := &EducationGroupRoom{ID: 9}
	groups := []*EducationGroupRoom{assigned, zero, unassigned}
	data := &dashboardGroupData{educationGroupRooms: make(map[int64]bool)}
	buildEducationGroupRoomsSet(groups, data)
	require.Equal(t, map[int64]bool{assigned.ID: true}, data.educationGroupRooms)
	lookup := buildGroupToRoomLookup(groups)
	require.Equal(t, map[int64]int64{assigned.ID: assigned.ID, zero.ID: 0}, lookup)
	require.NotContains(t, lookup, unassigned.ID)
	require.Empty(t, buildGroupToRoomLookup(nil))
	require.Empty(t, buildGroupToRoomLookup([]*EducationGroupRoom{}))

	data.studentHomeRoomMap = make(map[int64]int64)
	buildStudentHomeRoomMap(map[int64]int64{101: assigned.ID, 102: zero.ID, 103: unassigned.ID, 104: 404}, groups, data)
	require.Equal(t, map[int64]int64{101: assigned.ID, 102: 0}, data.studentHomeRoomMap,
		"unknown or unassigned groups have no home room; an explicit zero room stays distinct")
}
