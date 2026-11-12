package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/delivery/mail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type targetedEmailWorkerStore struct {
	workerStore
	intent  domain.Intent
	retries int
}

func (s *targetedEmailWorkerStore) ClaimEmail(context.Context, int64, int64, time.Time, time.Time) (domain.Intent, bool, error) {
	return s.intent, true, nil
}

func (s *targetedEmailWorkerStore) RecordEmailRetry(context.Context, int64, string, int, time.Time) (bool, error) {
	s.retries++
	return true, nil
}

func synchronousEmailIntent(t *testing.T) domain.Intent {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"delivery_max_attempts": 3,
		"delivery_backoff":      []time.Duration{time.Hour, time.Hour},
	})
	require.NoError(t, err)
	token := "lease"
	intent := claimedEmail(0)
	intent.LeaseToken = &token
	intent.Payload = payload
	return intent
}

// Without a transport the attempts still run to the limit, so the stored
// outcome is the one of an exhausted mail, but none of them waits.
func TestDispatchEmailDoesNotWaitOutTheBackoffOfAMissingTransport(t *testing.T) {
	t.Parallel()

	store := &targetedEmailWorkerStore{workerStore: workerStore{renewed: true, failureResult: domain.FinalizeResult{Finalized: true}}}
	store.intent = synchronousEmailIntent(t)
	provider := &workerProvider{err: mail.ErrDeliveryUnavailable}
	worker := NewWorker(store, provider, func(domain.Observation) {})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := worker.DispatchEmail(ctx, 7, 41)

	require.ErrorIs(t, err, mail.ErrDeliveryUnavailable)
	require.NoError(t, ctx.Err(), "the dispatch returned before the backoff")
	assert.Equal(t, 3, provider.calls)
	assert.Equal(t, 2, store.retries)
	assert.Equal(t, 3, store.failureAttempts)
}

// Any other transport failure keeps its spacing.
func TestDispatchEmailKeepsTheBackoffOfATransportFailure(t *testing.T) {
	t.Parallel()

	store := &targetedEmailWorkerStore{workerStore: workerStore{renewed: true, failureResult: domain.FinalizeResult{Finalized: true}}}
	store.intent = synchronousEmailIntent(t)
	provider := &workerProvider{err: errors.New("smtp: connection refused")}
	worker := NewWorker(store, provider, func(domain.Observation) {})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := worker.DispatchEmail(ctx, 7, 41)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, 1, provider.calls)
	assert.Zero(t, store.retries)
}
