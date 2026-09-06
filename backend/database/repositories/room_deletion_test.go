package repositories_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestFactoryDeletesRoomThroughDefaultOwner(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	room := testpkg.CreateTestRoom(t, db, "Löschbarer Raum")
	factory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))

	require.NoError(t, factory.Room.Delete(testpkg.Ctx(t), room.ID))
	_, err := factory.Room.FindByID(testpkg.Ctx(t), room.ID)
	require.Error(t, err)
}
