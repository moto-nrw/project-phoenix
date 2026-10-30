package delivery

import (
	"context"
	"errors"
	"testing"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingWorkerEngine struct {
	batchSize, maxAttempts int
	deadLettered           int
	deadLetters            []string
	err                    error
}

func (e *recordingWorkerEngine) RunOnce(_ context.Context, batchSize, maxAttempts int) (deliveryContract.WorkerStats, error) {
	e.batchSize, e.maxAttempts = batchSize, maxAttempts
	return deliveryContract.WorkerStats{Claimed: 4, DeadLettered: e.deadLettered, DeadLetters: e.deadLetters}, e.err
}

func (*recordingWorkerEngine) Backlog(context.Context) (int, error) { return 0, nil }

func TestWorkerValidatesRunInputsBeforeCallingEngine(t *testing.T) {
	t.Parallel()
	for _, input := range []struct {
		batchSize, maxAttempts int
		message                string
	}{
		{0, 6, "delivery worker: batch size must be positive"},
		{25, 0, "delivery worker: max attempts must be positive"},
		{25, -1, "delivery worker: max attempts must be positive"},
	} {
		engine := &recordingWorkerEngine{}
		processed, err := NewWorker(engine).RunOnce(context.Background(), input.batchSize, input.maxAttempts)
		require.EqualError(t, err, input.message)
		assert.Zero(t, processed)
		assert.Zero(t, engine.batchSize)
	}
}

func TestWorkerPassesRunPolicyAndPreservesResult(t *testing.T) {
	t.Parallel()
	failure := errors.New("claim failed")
	engine := &recordingWorkerEngine{err: failure}
	processed, err := NewWorker(engine).RunOnce(context.Background(), 25, 3)
	assert.Equal(t, 25, engine.batchSize)
	assert.Equal(t, 3, engine.maxAttempts)
	assert.Equal(t, 4, processed)
	assert.ErrorIs(t, err, failure)
}

// Issue #3640: a delivery that used up its attempts fails the run, so the
// scheduler reports the run to Sentry. A run with only retries does not.
func TestWorkerFailsRunWithDeadLetteredDeliveries(t *testing.T) {
	t.Parallel()

	processed, err := NewWorker(&recordingWorkerEngine{deadLettered: 2}).RunOnce(context.Background(), 25, 3)
	assert.Equal(t, 4, processed)
	require.ErrorIs(t, err, deliveryContract.ErrDeadLettered)
	assert.Contains(t, err.Error(), ": 2")

	_, err = NewWorker(&recordingWorkerEngine{}).RunOnce(context.Background(), 25, 3)
	assert.NoError(t, err)
}

// Sentry BACKEND-Y: the event of a dead-lettered run named only a count, so
// nobody could tell which delivery failed or why. The run's error carries
// transport, template, delivery id and last error of each dead letter.
func TestWorkerNamesWhyDeliveriesFailedForGood(t *testing.T) {
	t.Parallel()

	_, err := NewWorker(&recordingWorkerEngine{deadLettered: 1, deadLetters: []string{
		"email password_reset delivery 41: smtp: 554 relay denied",
	}}).RunOnce(context.Background(), 25, 3)

	require.ErrorIs(t, err, deliveryContract.ErrDeadLettered)
	assert.EqualError(t, err, "delivery worker: deliveries failed for good after all attempts: 1 "+
		"(email password_reset delivery 41: smtp: 554 relay denied)")
}
