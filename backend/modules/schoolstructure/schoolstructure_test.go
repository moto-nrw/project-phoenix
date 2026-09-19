package schoolstructure_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingEngine struct {
	findID  int64
	listIDs []int64
	calls   int
}

func (e *recordingEngine) ListGroups(context.Context, int) ([]schoolstructure.Group, error) {
	e.calls++
	return nil, nil
}

func TestModuleRejectsInvalidListingLimitsBeforeReachingTheEngine(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := schoolstructure.NewModule(engine)
	for _, limit := range []int{-1, 0, 1001} {
		_, err := module.ListGroups(t.Context(), limit)
		require.ErrorIs(t, err, schoolstructure.ErrInvalidGroup)
	}
	assert.Zero(t, engine.calls)
}

func (e *recordingEngine) FindGroup(_ context.Context, id int64) (schoolstructure.Group, error) {
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

func (e *recordingEngine) CountStudentTransitionHistory(context.Context, int64) (int, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) AnonymizeStudentTransitionHistory(context.Context, int64) (int64, error) {
	e.calls++
	return 0, nil
}

// The grade-transition ledger methods count like every other engine call so
// the guard tests in transition_test.go can prove invalid input never
// reaches persistence.

func (e *recordingEngine) FindTransition(_ context.Context, id int64, _ string) (schoolstructure.Transition, error) {
	e.calls++
	return schoolstructure.Transition{ID: id, Status: schoolstructure.TransitionStatusDraft}, nil
}

func (e *recordingEngine) ListTransitions(context.Context, schoolstructure.TransitionFilter) ([]schoolstructure.Transition, int, error) {
	e.calls++
	return nil, 0, nil
}

func (e *recordingEngine) ListTransitionHistory(context.Context, int64) ([]schoolstructure.TransitionHistoryEntry, error) {
	e.calls++
	return nil, nil
}

func (e *recordingEngine) ListTransitionClassTeacherLedger(context.Context, int64) ([]schoolstructure.TransitionClassTeacherEntry, error) {
	e.calls++
	return nil, nil
}

func (e *recordingEngine) ListTransitionClassListLedger(context.Context, int64) ([]schoolstructure.TransitionClassListEntry, error) {
	e.calls++
	return nil, nil
}

func (e *recordingEngine) CreateTransition(_ context.Context, draft schoolstructure.TransitionDraft) (schoolstructure.Transition, error) {
	e.calls++
	return schoolstructure.Transition{AcademicYear: draft.AcademicYear, CreatedBy: draft.CreatedBy}, nil
}

func (e *recordingEngine) UpdateTransition(_ context.Context, update schoolstructure.TransitionUpdate) (schoolstructure.Transition, error) {
	e.calls++
	return schoolstructure.Transition{ID: update.ID}, nil
}

func (e *recordingEngine) DeleteTransition(context.Context, int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) LockTransitions(context.Context) error {
	e.calls++
	return nil
}

func (e *recordingEngine) LockLatestAppliedTransition(context.Context) (schoolstructure.Transition, bool, error) {
	e.calls++
	return schoolstructure.Transition{}, false, nil
}

func (e *recordingEngine) MarkTransitionApplied(context.Context, int64, int64, time.Time, *int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) MarkTransitionReverted(context.Context, int64, int64, time.Time) error {
	e.calls++
	return nil
}

func (e *recordingEngine) AppendTransitionHistory(context.Context, []schoolstructure.TransitionHistoryEntry) error {
	e.calls++
	return nil
}

func (e *recordingEngine) AppendTransitionClassTeacherLedger(context.Context, []schoolstructure.TransitionClassTeacherEntry) error {
	e.calls++
	return nil
}

func (e *recordingEngine) AppendTransitionClassListLedger(context.Context, []schoolstructure.TransitionClassListEntry) error {
	e.calls++
	return nil
}
