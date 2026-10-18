package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type commandStore struct {
	event any
	err   error
}

func (s *commandStore) Append(_ context.Context, event any) error {
	s.event = event
	return s.err
}

func TestCommandAppendObservesSuccessAndFailure(t *testing.T) {
	t.Parallel()

	forced := errors.New("forced append failure")
	for _, tc := range []struct {
		name string
		err  error
		rows int
	}{
		{name: "success", rows: 1},
		{name: "failure", err: forced, rows: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := &commandStore{err: tc.err}
			var observations []AppendObservation
			command, err := NewCommand(store, func(event AppendObservation) {
				observations = append(observations, event)
			})
			require.NoError(t, err)

			event := &struct{ ID int }{ID: 7}
			err = command.Append(context.Background(), event)
			require.ErrorIs(t, err, tc.err)
			assert.Same(t, event, store.event)
			require.Len(t, observations, 1)
			assert.Equal(t, "*struct { ID int }", observations[0].EventType)
			assert.Equal(t, tc.rows, observations[0].Rows)
			assert.ErrorIs(t, observations[0].Err, tc.err)
			assert.GreaterOrEqual(t, observations[0].Duration.Nanoseconds(), int64(0))
		})
	}
}

func TestNewCommandRequiresDependencies(t *testing.T) {
	t.Parallel()

	store := &commandStore{}
	_, err := NewCommand(nil, func(AppendObservation) {})
	require.Error(t, err)
	_, err = NewCommand(store, nil)
	require.Error(t, err)
}
