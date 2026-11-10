package application

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Keep the production command binding eligible for RunOnce's optional retention.
var _ interface {
	DeleteExpiredSystemEmails(context.Context, time.Time, int) (int, error)
} = (*WorkerCommands)(nil)

type retentionWorkerStore struct {
	workerStore
	calls  int
	before time.Time
	limit  int
}

func (s *retentionWorkerStore) DeleteExpiredSystemEmails(_ context.Context, before time.Time, limit int) (int, error) {
	s.calls++
	s.before, s.limit = before, limit
	return 0, nil
}

func TestWorkerRunOnceTriggersThirtyDayRetention(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		store := &retentionWorkerStore{}
		worker := NewWorker(store, &workerProvider{}, func(domain.Observation) {})
		now := time.Now()

		_, err := worker.RunOnce(context.Background(), 25, 6)

		require.NoError(t, err)
		require.Equal(t, 1, store.calls, "every worker tick runs the global retention")
		assert.Equal(t, now.Add(-30*24*time.Hour), store.before)
		assert.Equal(t, 25, store.limit)
	})
}
