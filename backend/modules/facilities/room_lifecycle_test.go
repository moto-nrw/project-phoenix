package facilities_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type lifecycleEngine struct {
	created facilities.CreateRoom
	updated facilities.UpdateRoom
	deleted int64
	calls   int
}

func (e *lifecycleEngine) FindRoom(context.Context, int64) (facilities.Room, error) {
	return facilities.Room{}, nil
}

func (e *lifecycleEngine) FindRoomForUpdate(context.Context, int64) (facilities.Room, error) {
	return facilities.Room{}, nil
}

func (e *lifecycleEngine) FindRoomByName(context.Context, string) (facilities.Room, error) {
	return facilities.Room{}, nil
}

func (e *lifecycleEngine) FindToiletRoom(context.Context, int64) (facilities.Room, error) {
	return facilities.Room{}, nil
}

func (e *lifecycleEngine) ListRooms(context.Context, facilities.RoomFilter) ([]facilities.Room, error) {
	return nil, nil
}

func (e *lifecycleEngine) ListRoomsByID(context.Context, []int64) ([]facilities.Room, error) {
	return nil, nil
}

func (e *lifecycleEngine) LockRoomsByID(context.Context, []int64) ([]facilities.Room, error) {
	return nil, nil
}

func (e *lifecycleEngine) CreateRoom(_ context.Context, input facilities.CreateRoom) (facilities.Room, error) {
	e.calls++
	e.created = input
	return facilities.Room{Name: input.Name, Color: input.Color}, nil
}

func (e *lifecycleEngine) UpdateRoom(_ context.Context, input facilities.UpdateRoom) (facilities.Room, error) {
	e.calls++
	e.updated = input
	return facilities.Room{ID: input.ID, Name: input.Name, Color: input.Color}, nil
}

func (e *lifecycleEngine) DeleteRoom(_ context.Context, id int64) error {
	e.calls++
	e.deleted = id
	return nil
}

func TestCreateRoomNormalizesAtPublicSeam(t *testing.T) {
	t.Parallel()

	color := "abc"
	engine := &lifecycleEngine{}
	module := facilities.NewModule(engine)

	created, err := module.CreateRoom(context.Background(), facilities.CreateRoom{
		Name:  "  Igelraum  ",
		Color: &color,
	})

	require.NoError(t, err)
	require.NotNil(t, engine.created.Color)
	assert.Equal(t, "Igelraum", engine.created.Name)
	assert.Equal(t, "#AABBCC", *engine.created.Color)
	assert.Equal(t, "Igelraum", created.Name)
	assert.Equal(t, "#AABBCC", *created.Color)
	assert.Equal(t, 1, engine.calls)
}

func TestUpdateRoomNormalizesAtPublicSeam(t *testing.T) {
	t.Parallel()

	color := "#abc"
	engine := &lifecycleEngine{}
	module := facilities.NewModule(engine)

	updated, err := module.UpdateRoom(context.Background(), facilities.UpdateRoom{
		ID:    41,
		Name:  "  Fuchsbau  ",
		Color: &color,
	})

	require.NoError(t, err)
	assert.Equal(t, int64(41), engine.updated.ID)
	assert.Equal(t, "Fuchsbau", engine.updated.Name)
	require.NotNil(t, engine.updated.Color)
	assert.Equal(t, "#AABBCC", *engine.updated.Color)
	assert.Equal(t, "Fuchsbau", updated.Name)
	assert.Equal(t, 1, engine.calls)
}

func TestDeleteRoomValidatesIdentifierAtPublicSeam(t *testing.T) {
	t.Parallel()

	engine := &lifecycleEngine{}
	module := facilities.NewModule(engine)

	err := module.DeleteRoom(context.Background(), 0)
	require.ErrorIs(t, err, facilities.ErrInvalidRoom)
	assert.Zero(t, engine.calls)

	err = module.DeleteRoom(context.Background(), 41)
	require.NoError(t, err)
	assert.Equal(t, int64(41), engine.deleted)
	assert.Equal(t, 1, engine.calls)
}

func TestRoomCommandsRejectInvalidValuesBeforePersistence(t *testing.T) {
	t.Parallel()

	invalidCapacity := 0
	invalidColor := "not-a-color"
	reservedColor := "#5080D8"
	tests := []struct {
		name    string
		input   facilities.CreateRoom
		wantErr error
	}{
		{name: "blank name", input: facilities.CreateRoom{Name: "  "}, wantErr: facilities.ErrInvalidRoom},
		{name: "non-positive capacity", input: facilities.CreateRoom{Name: "Igelraum", Capacity: &invalidCapacity}, wantErr: facilities.ErrInvalidRoom},
		{name: "invalid color", input: facilities.CreateRoom{Name: "Igelraum", Color: &invalidColor}, wantErr: facilities.ErrInvalidRoom},
		{name: "reserved status color", input: facilities.CreateRoom{Name: "Igelraum", Color: &reservedColor}, wantErr: facilities.ErrRoomColorReserved},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine := &lifecycleEngine{}
			module := facilities.NewModule(engine)

			_, err := module.CreateRoom(context.Background(), test.input)

			require.ErrorIs(t, err, test.wantErr)
			assert.Zero(t, engine.calls)
		})
	}
}

func TestCreateRoomReservesSchulhofNameForSystemProvisioning(t *testing.T) {
	t.Parallel()

	engine := &lifecycleEngine{}
	module := facilities.NewModule(engine)

	_, err := module.CreateRoom(context.Background(), facilities.CreateRoom{Name: "schulhof"})
	require.ErrorIs(t, err, facilities.ErrSystemRoomNameReserved)
	assert.Zero(t, engine.calls)

	created, err := module.CreateRoom(context.Background(), facilities.CreateRoom{
		Name: facilities.SchulhofRoomName, IsSystem: true,
	})
	require.NoError(t, err)
	assert.Equal(t, facilities.SchulhofRoomName, created.Name)
	assert.Equal(t, 1, engine.calls)
}

func TestCreateRoomKeepsReservedColorUserMessage(t *testing.T) {
	t.Parallel()

	color := "#5080D8"
	module := facilities.NewModule(&lifecycleEngine{})

	_, err := module.CreateRoom(context.Background(), facilities.CreateRoom{Name: "Igelraum", Color: &color})

	require.ErrorIs(t, err, facilities.ErrRoomColorReserved)
	assert.Equal(t, "Diese Farbe ist für Statusbadges reserviert und kann nicht für Räume verwendet werden", err.Error())
}
