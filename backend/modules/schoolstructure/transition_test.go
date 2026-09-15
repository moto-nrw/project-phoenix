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

func TestTransitionModuleRejectsInvalidInputBeforeReachingTheEngine(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := schoolstructure.NewModule(engine)
	ctx := context.Background()
	now := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)

	_, err := module.FindTransition(ctx, 0)
	require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition)
	_, _, err = module.ListTransitions(ctx, schoolstructure.TransitionFilter{Limit: -1})
	require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition)
	_, err = module.ListTransitionHistory(ctx, -3)
	require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition)
	_, err = module.CreateTransition(ctx, schoolstructure.TransitionDraft{AcademicYear: "2026-2027"})
	require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition, "created_by is required")
	_, err = module.UpdateTransition(ctx, schoolstructure.TransitionUpdate{})
	require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition)
	require.ErrorIs(t, module.DeleteTransition(ctx, 0), schoolstructure.ErrInvalidTransition)
	_, err = module.LockTransitionForMutation(ctx, 0)
	require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition)
	require.ErrorIs(t, module.MarkTransitionApplied(ctx, 1, 0, now, nil), schoolstructure.ErrInvalidTransition)
	require.ErrorIs(t, module.MarkTransitionReverted(ctx, 1, 7, time.Time{}), schoolstructure.ErrInvalidTransition)
	assert.Zero(t, engine.calls, "invalid input must never reach persistence")

	require.NoError(t, module.AppendTransitionHistory(ctx, nil))
	require.NoError(t, module.AppendTransitionClassTeacherLedger(ctx, nil))
	require.NoError(t, module.AppendTransitionClassListLedger(ctx, nil))
	assert.Zero(t, engine.calls, "an empty append is answered without persistence")
}

func TestTransitionModuleForwardsValidCallsToTheEngine(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := schoolstructure.NewModule(engine)
	ctx := context.Background()
	now := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)

	// The recording engine echoes ids; they are not database rows.
	const echoedID, actorID, studentID = 42, 7, 3
	transition, err := module.FindTransition(ctx, echoedID)
	require.NoError(t, err)
	assert.Equal(t, int64(echoedID), transition.ID)
	assert.True(t, transition.IsDraft())
	assert.False(t, transition.CanApply(), "a draft without mappings cannot be applied")

	_, err = module.LockTransitionForMutation(ctx, echoedID)
	require.NoError(t, err)
	require.NoError(t, module.LockTransitions(ctx))
	require.NoError(t, module.MarkTransitionApplied(ctx, echoedID, actorID, now, nil))
	require.NoError(t, module.MarkTransitionReverted(ctx, echoedID, actorID, now))
	require.NoError(t, module.AppendTransitionHistory(ctx, []schoolstructure.TransitionHistoryEntry{{TransitionID: echoedID, StudentID: studentID}}))
	assert.Equal(t, 6, engine.calls)
}

func TestTransitionErrorCodesAreStable(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "not_found", schoolstructure.ErrorCode(schoolstructure.ErrTransitionNotFound))
	assert.Equal(t, "invalid", schoolstructure.ErrorCode(&schoolstructure.InvalidTransitionError{Reason: "x"}))
	assert.Equal(t, "conflict", schoolstructure.ErrorCode(schoolstructure.ErrTransitionStateConflict))
	assert.True(t, errors.Is(&schoolstructure.InvalidTransitionError{Reason: "x"}, schoolstructure.ErrInvalidTransition))
}
