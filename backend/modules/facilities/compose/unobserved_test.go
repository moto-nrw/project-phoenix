package compose

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestDefaultFacilitiesDeletesRoomWithoutGuard(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	room := testpkg.CreateTestRoom(t, db, "Löschbarer Raum")
	rooms, err := NewUnobserved(db)
	require.NoError(t, err)

	require.NoError(t, rooms.DeleteRoom(testpkg.Ctx(t), room.ID))
	_, err = rooms.FindRoom(testpkg.Ctx(t), room.ID)
	require.Error(t, err)
}
