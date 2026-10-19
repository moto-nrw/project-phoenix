package delivery

import (
	"context"
	"errors"
	"testing"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingEngine struct {
	pushCalled bool
}

func (*recordingEngine) EnqueueEmail(context.Context, deliveryContract.EmailIntent) (deliveryContract.Enqueued, error) {
	return deliveryContract.Enqueued{}, nil
}

func (*recordingEngine) EnqueueDeferredPush(context.Context, deliveryContract.DeferredPushIntent) (deliveryContract.Enqueued, error) {
	return deliveryContract.Enqueued{}, nil
}

type recordingWorkerEngine struct {
	batchSize, maxAttempts int
	deadLettered           int
	err                    error
}

func (e *recordingWorkerEngine) RunOnce(_ context.Context, batchSize, maxAttempts int) (deliveryContract.WorkerStats, error) {
	e.batchSize, e.maxAttempts = batchSize, maxAttempts
	return deliveryContract.WorkerStats{Claimed: 4, DeadLettered: e.deadLettered}, e.err
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
func (e *recordingEngine) EnqueuePush(context.Context, deliveryContract.PushIntent) (deliveryContract.Enqueued, error) {
	e.pushCalled = true
	return deliveryContract.Enqueued{}, nil
}
func (*recordingEngine) Cancel(context.Context, int64, deliveryContract.Transport, deliveryContract.RelatedEntity, string) (int64, error) {
	return 0, nil
}
func (*recordingEngine) Statuses(context.Context, int64, deliveryContract.Transport, deliveryContract.RelatedEntity) ([]deliveryContract.Status, error) {
	return nil, nil
}
func (*recordingEngine) ReplaceEmailDeliveries(context.Context, int64, deliveryContract.RelatedEntity, []deliveryContract.EmailDelivery) error {
	return nil
}
func (*recordingEngine) DeleteEmailDeliveries(context.Context, int64, deliveryContract.RelatedEntity) (int64, error) {
	return 0, nil
}
func (*recordingEngine) EmailDeliveryStatuses(context.Context, int64, deliveryContract.RelatedEntity) ([]deliveryContract.EmailDeliveryStatus, error) {
	return nil, nil
}
func (*recordingEngine) AttachEmailOutbox(context.Context, int64, int64, int64) error { return nil }
func (*recordingEngine) ClaimFailedEmailDelivery(context.Context, int64, int64) (bool, error) {
	return false, nil
}

func TestEnqueuePushRejectsUntrustedEndpoint(t *testing.T) {
	t.Parallel()

	engine := &recordingEngine{}
	module := NewModule(engine)
	tenantID := int64(42)
	_, err := module.EnqueuePush(context.Background(), deliveryContract.PushIntent{
		TenantID: tenantID, Template: "appointment", IdempotencyKey: "appointment:1",
		Related:   deliveryContract.RelatedEntity{Type: "appointment", ID: 1},
		Recipient: deliveryContract.PushRecipient{Endpoint: "https://127.0.0.1/push", P256DH: "key", Auth: "auth"},
		Payload:   deliveryContract.PushPayload{Title: "Termin"},
	})

	require.Error(t, err)
	assert.False(t, engine.pushCalled)
}
