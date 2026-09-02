package schoolstructure_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingEngine struct {
	findID  int64
	listIDs []int64
	calls   int
}

func (e *recordingEngine) FindGroupByID(_ context.Context, id int64) (schoolstructure.Group, error) {
	e.calls++
	e.findID = id
	return schoolstructure.Group{ID: id, Name: "Igel"}, nil
}

func (e *recordingEngine) ListGroupsByID(_ context.Context, ids []int64) ([]schoolstructure.Group, error) {
	e.calls++
	e.listIDs = ids
	result := make([]schoolstructure.Group, 0, len(ids))
	for _, id := range ids {
		result = append(result, schoolstructure.Group{ID: id, Name: "Igel"})
	}
	return result, nil
}

func TestModuleRejectsInvalidIdentifiersBeforeReachingTheEngine(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := schoolstructure.NewModule(engine)
	ctx := context.Background()

	_, err := module.FindGroup(ctx, 0)
	require.ErrorIs(t, err, schoolstructure.ErrInvalidGroup)

	_, err = module.ListGroupsByID(ctx, []int64{7, -1})
	require.ErrorIs(t, err, schoolstructure.ErrInvalidGroup)

	assert.Zero(t, engine.calls, "invalid input must never reach persistence")
}

func TestModuleAnswersEmptyListWithoutPersistence(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := schoolstructure.NewModule(engine)

	groups, err := module.ListGroupsByID(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, groups)
	assert.Zero(t, engine.calls)
}

func TestModuleForwardsValidReadsToTheEngine(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := schoolstructure.NewModule(engine)
	ctx := context.Background()

	group, err := module.FindGroup(ctx, 42)
	require.NoError(t, err)
	assert.Equal(t, int64(42), engine.findID)
	assert.Equal(t, "Igel", group.Name)

	groups, err := module.ListGroupsByID(ctx, []int64{3, 5})
	require.NoError(t, err)
	assert.Equal(t, []int64{3, 5}, engine.listIDs)
	assert.Len(t, groups, 2)
}

func TestErrorCodeIsStable(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "none", schoolstructure.ErrorCode(nil))
	assert.Equal(t, "not_found", schoolstructure.ErrorCode(schoolstructure.ErrGroupNotFound))
	assert.Equal(t, "invalid", schoolstructure.ErrorCode(schoolstructure.ErrInvalidGroup))
	assert.Equal(t, "internal_error", schoolstructure.ErrorCode(errors.New("boom")))
}

func TestNewModulePanicsWithoutEngine(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { schoolstructure.NewModule(nil) })
}
