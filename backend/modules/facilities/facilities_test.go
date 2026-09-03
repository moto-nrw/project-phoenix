package facilities_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingEngine struct {
	findID  int64
	listIDs []int64
	calls   int
}

func (e *recordingEngine) FindRoom(_ context.Context, id int64) (facilities.Room, error) {
	e.calls++
	e.findID = id
	return facilities.Room{ID: id, Name: "Igelraum"}, nil
}

func (e *recordingEngine) ListRoomsByID(_ context.Context, ids []int64) ([]facilities.Room, error) {
	e.calls++
	e.listIDs = ids
	result := make([]facilities.Room, 0, len(ids))
	for _, id := range ids {
		result = append(result, facilities.Room{ID: id, Name: "Igelraum"})
	}
	return result, nil
}

func (e *recordingEngine) LockRoomsByID(_ context.Context, ids []int64) ([]facilities.Room, error) {
	return e.ListRoomsByID(context.Background(), ids)
}

func TestModuleRejectsInvalidIdentifiersBeforeReachingTheEngine(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := facilities.NewModule(engine)
	ctx := context.Background()

	_, err := module.FindRoom(ctx, 0)
	require.ErrorIs(t, err, facilities.ErrInvalidRoom)

	_, err = module.ListRoomsByID(ctx, []int64{7, -1})
	require.ErrorIs(t, err, facilities.ErrInvalidRoom)

	assert.Zero(t, engine.calls, "invalid input must never reach persistence")
}

func TestModuleAnswersEmptyListWithoutPersistence(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := facilities.NewModule(engine)

	rooms, err := module.ListRoomsByID(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, rooms)
	assert.Zero(t, engine.calls)
}

func TestModuleForwardsValidReadsToTheEngine(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := facilities.NewModule(engine)
	ctx := context.Background()

	room, err := module.FindRoom(ctx, 42)
	require.NoError(t, err)
	assert.Equal(t, int64(42), engine.findID)
	assert.Equal(t, "Igelraum", room.Name)

	rooms, err := module.ListRoomsByID(ctx, []int64{3, 5})
	require.NoError(t, err)
	assert.Equal(t, []int64{3, 5}, engine.listIDs)
	assert.Len(t, rooms, 2)
}

func TestModuleForwardsRoomLocksToTheEngine(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := facilities.NewModule(engine)

	rooms, err := module.LockRoomsByID(context.Background(), []int64{3, 5})
	require.NoError(t, err)
	assert.Len(t, rooms, 2)
	assert.Equal(t, []int64{3, 5}, engine.listIDs)
}

func TestErrorCodeIsStable(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "none", facilities.ErrorCode(nil))
	assert.Equal(t, "not_found", facilities.ErrorCode(facilities.ErrRoomNotFound))
	assert.Equal(t, "invalid", facilities.ErrorCode(facilities.ErrInvalidRoom))
	assert.Equal(t, "internal_error", facilities.ErrorCode(errors.New("boom")))
}

func TestNewModulePanicsWithoutEngine(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { facilities.NewModule(nil) })
}
