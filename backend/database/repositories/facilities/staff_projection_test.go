package facilities_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The occupancy projection reads the supervising staff IDs only; School
// Membership turns them into the persons the room list renders (#2667).
func TestRoomOccupancyNamesTheSupervisingPersons(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	factory := repositories.NewFactory(db)

	staff := testpkg.CreateTestStaff(t, db, "Raum", "Aufsicht")
	activity := testpkg.CreateTestActivityGroup(t, db, "Raum Angebot")
	room := testpkg.CreateTestRoom(t, db, "Belegter Raum")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")

	row, err := factory.Room.FindWithOccupancy(ctx, room.ID)
	require.NoError(t, err)
	assert.Equal(t, []int64{staff.ID}, row.SupervisorStaffIDs)
	assert.Equal(t, []int64{staff.PersonID}, row.SupervisorPersonIDs)

	rows, err := factory.Room.ListWithOccupancy(ctx, nil)
	require.NoError(t, err)
	var found bool
	for _, listed := range rows {
		if listed.ID != room.ID {
			continue
		}
		found = true
		assert.Equal(t, []int64{staff.PersonID}, listed.SupervisorPersonIDs)
	}
	assert.True(t, found)
}
