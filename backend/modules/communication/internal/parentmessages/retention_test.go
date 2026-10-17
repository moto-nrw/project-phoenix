package messaging

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type retentionWindow struct {
	days int
	err  error
}

func (w retentionWindow) ParentMessageRetentionDays(context.Context) (int, error) {
	return w.days, w.err
}

type recordingRetentionStore struct {
	cutoff time.Time
	calls  int
}

func (s *recordingRetentionStore) DeleteMessagesBefore(_ context.Context, cutoff time.Time) (int64, int64, error) {
	s.calls++
	s.cutoff = cutoff
	return 4, 1, nil
}

func TestParentMessageRetentionDeletesBeforeTheRecordedWindow(t *testing.T) {
	t.Parallel()
	store := &recordingRetentionStore{}
	now := time.Date(2026, time.October, 14, 2, 0, 0, 0, time.UTC)

	result, err := Retention{Store: store, Settings: retentionWindow{days: 365}, Now: func() time.Time { return now }}.
		CleanupExpiredMessages(context.Background())

	require.NoError(t, err)
	assert.Equal(t, RetentionResult{MessagesDeleted: 4, ThreadsDeleted: 1, RetentionDays: 365}, result)
	assert.Equal(t, time.Date(2025, time.October, 14, 2, 0, 0, 0, time.UTC), store.cutoff)
}

func TestParentMessageRetentionNeverDeletesOnAnUnknownWindow(t *testing.T) {
	t.Parallel()

	for name, window := range map[string]retentionWindow{
		"unreadable":   {err: errors.New("settings unavailable")},
		"not positive": {days: 0},
	} {
		store := &recordingRetentionStore{}
		_, err := Retention{Store: store, Settings: window}.CleanupExpiredMessages(context.Background())

		require.ErrorIs(t, err, ErrRetentionUnresolved, name)
		assert.Zero(t, store.calls, "%s: nothing is deleted on a guessed window", name)
	}
}
