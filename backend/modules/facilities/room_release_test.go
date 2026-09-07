package facilities_test

// Behavior of the room release ("offener Raum", #3064) at the module's public
// boundary: what a caller may express, what the owner refuses, and what an
// omitted opinion means.

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolPtr(value bool) *bool { return &value }

func TestIsToiletRoomName(t *testing.T) {
	t.Parallel()

	assert.True(t, facilities.IsToiletRoomName(facilities.WCRoomName))
	assert.True(t, facilities.IsToiletRoomName(facilities.WCRoomAliasName))
	assert.False(t, facilities.IsToiletRoomName(facilities.SchulhofRoomName))
	assert.False(t, facilities.IsToiletRoomName("Igelraum"))
	assert.False(t, facilities.IsToiletRoomName(""),
		"an empty name is not a toilet room")
	assert.False(t, facilities.IsToiletRoomName("wc"),
		"matching is exact-case, matching the rest of the system-room contract")
	assert.False(t, facilities.IsToiletRoomName("Toiletten"),
		"only the two canonical names count, not anything starting with them")
}

func TestCreateRoomCarriesTheReleaseToThePersistenceOwner(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := facilities.NewModule(engine)

	room, err := module.CreateRoom(context.Background(), facilities.CreateRoom{
		Name: "Turnhalle", IsOpenRoom: true,
	})
	require.NoError(t, err)
	assert.True(t, room.IsOpenRoom)
	require.NotNil(t, engine.createInput)
	assert.True(t, engine.createInput.IsOpenRoom)
}

func TestCreateRoomDefaultsToUnreleased(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := facilities.NewModule(engine)

	room, err := module.CreateRoom(context.Background(), facilities.CreateRoom{Name: "Werkraum"})
	require.NoError(t, err)
	assert.False(t, room.IsOpenRoom,
		"release is an explicit administrative decision, never a default")
	require.NotNil(t, engine.createInput)
	assert.False(t, engine.createInput.IsOpenRoom)
}

func TestCreateRoomRefusesToReleaseAToiletRoom(t *testing.T) {
	t.Parallel()

	for _, name := range []string{facilities.WCRoomName, facilities.WCRoomAliasName} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			engine := &recordingEngine{}
			module := facilities.NewModule(engine)

			_, err := module.CreateRoom(context.Background(), facilities.CreateRoom{
				Name: name, IsSystem: true, IsOpenRoom: true,
			})
			require.ErrorIs(t, err, facilities.ErrToiletRoomNotReleasable)
			assert.Nil(t, engine.createInput,
				"a refused release must never reach persistence")
			assert.Equal(t, []string{"create_room"}, engine.rejections)
		})
	}
}

func TestCreateToiletRoomWithoutReleaseIsAllowed(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := facilities.NewModule(engine)

	_, err := module.CreateRoom(context.Background(), facilities.CreateRoom{
		Name: facilities.WCRoomName, IsSystem: true,
	})
	require.NoError(t, err, "the guard rejects the release, not the toilet room itself")
	require.NotNil(t, engine.createInput)
	assert.False(t, engine.createInput.IsOpenRoom)
}

func TestUpdateRoomWithoutAnOpinionLeavesTheReleaseUntouched(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := facilities.NewModule(engine)

	_, err := module.UpdateRoom(context.Background(), facilities.UpdateRoom{
		ID: 7, Name: "Turnhalle",
	})
	require.NoError(t, err)
	require.NotNil(t, engine.updateInput)
	assert.Nil(t, engine.updateInput.IsOpenRoom,
		"an unrelated edit must not carry an opinion about the release")
}

func TestUpdateRoomCarriesBothReleaseVerdicts(t *testing.T) {
	t.Parallel()

	for _, released := range []bool{true, false} {
		t.Run(map[bool]string{true: "release", false: "revoke"}[released], func(t *testing.T) {
			t.Parallel()
			engine := &recordingEngine{}
			module := facilities.NewModule(engine)

			room, err := module.UpdateRoom(context.Background(), facilities.UpdateRoom{
				ID: 7, Name: "Turnhalle", IsOpenRoom: boolPtr(released),
			})
			require.NoError(t, err)
			assert.Equal(t, released, room.IsOpenRoom)
			require.NotNil(t, engine.updateInput)
			require.NotNil(t, engine.updateInput.IsOpenRoom)
			assert.Equal(t, released, *engine.updateInput.IsOpenRoom)
		})
	}
}

func TestUpdateRoomRefusesToReleaseAToiletRoom(t *testing.T) {
	t.Parallel()

	for _, name := range []string{facilities.WCRoomName, facilities.WCRoomAliasName} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			engine := &recordingEngine{}
			module := facilities.NewModule(engine)

			_, err := module.UpdateRoom(context.Background(), facilities.UpdateRoom{
				ID: 7, Name: name, IsOpenRoom: boolPtr(true),
			})
			require.ErrorIs(t, err, facilities.ErrToiletRoomNotReleasable)
			assert.Nil(t, engine.updateInput,
				"a refused release must never reach persistence")
			assert.Equal(t, []string{"update_room"}, engine.rejections)
		})
	}
}

// Revoking a release on a toilet room is not the case the guard defends
// against: a stored TRUE there would be a data problem, and refusing to clear
// it would leave an administrator with no way out.
func TestUpdateRoomAllowsRevokingAToiletRoomRelease(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := facilities.NewModule(engine)

	_, err := module.UpdateRoom(context.Background(), facilities.UpdateRoom{
		ID: 7, Name: facilities.WCRoomName, IsOpenRoom: boolPtr(false),
	})
	require.NoError(t, err)
	require.NotNil(t, engine.updateInput)
	require.NotNil(t, engine.updateInput.IsOpenRoom)
	assert.False(t, *engine.updateInput.IsOpenRoom)
}

func TestReleaseIsIndependentOfTheSchulhofNameGuard(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := facilities.NewModule(engine)

	// A released ordinary room is exactly the point of #3062: schools must be
	// able to express the same arrangement for a gym or craft area, not only
	// for the yard.
	room, err := module.CreateRoom(context.Background(), facilities.CreateRoom{
		Name: "Turnhalle", IsOpenRoom: true,
	})
	require.NoError(t, err)
	assert.True(t, room.IsOpenRoom)

	// The reserved-name guard still applies and is unaffected by the release.
	_, err = module.CreateRoom(context.Background(), facilities.CreateRoom{
		Name: facilities.SchulhofRoomName, IsOpenRoom: true,
	})
	require.ErrorIs(t, err, facilities.ErrSystemRoomNameReserved,
		"releasing a room must not become a way around the reserved Schulhof name")
}

func TestErrorCodeCoversTheReleaseRefusal(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "toilet_room_not_releasable",
		facilities.ErrorCode(facilities.ErrToiletRoomNotReleasable))
}
