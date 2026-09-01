package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type workerStore struct {
	claimed            []domain.Intent
	finalizeSent       bool
	finalizeSentErr    error
	failureResult      domain.FinalizeResult
	finalizeCancelled  bool
	sentToken          string
	cancelledToken     string
	failureToken       string
	failureAttempts    int
	failureMaxAttempts int
	renewed            bool
	renewToken         string
	statuses           []domain.EmailDeliveryStatus
}

func (s *workerStore) RenewLease(_ context.Context, _ domain.Transport, _ int64, token string, _ time.Time) (bool, error) {
	s.renewToken = token
	return s.renewed, nil
}

func (*workerStore) Enqueue(context.Context, domain.Intent) (domain.Enqueued, error) {
	return domain.Enqueued{}, nil
}

func (s *workerStore) Claim(_ context.Context, transport domain.Transport, _ int, _ time.Time, _ time.Time) ([]domain.Intent, error) {
	if transport != domain.TransportEmail {
		return nil, nil
	}
	token := "lease-token"
	rows := append([]domain.Intent(nil), s.claimed...)
	for index := range rows {
		rows[index].LeaseToken = &token
	}
	return rows, nil
}

func (s *workerStore) FinalizeSent(_ context.Context, _ domain.Transport, _ int64, token string, _ json.RawMessage, _ time.Time) (bool, error) {
	s.sentToken = token
	return s.finalizeSent, s.finalizeSentErr
}

func (s *workerStore) FinalizeCancelled(_ context.Context, _ domain.Transport, _ int64, token, _ string, _ time.Time) (bool, error) {
	s.cancelledToken = token
	return s.finalizeCancelled, nil
}

func (s *workerStore) FinalizeFailure(_ context.Context, _ domain.Transport, _ int64, token string, attempts int, _ string, _ time.Time, maxAttempts int) (domain.FinalizeResult, error) {
	s.failureToken = token
	s.failureAttempts = attempts
	s.failureMaxAttempts = maxAttempts
	return s.failureResult, nil
}

func (*workerStore) Cancel(context.Context, int64, domain.Transport, string, int64, string, time.Time) (int64, error) {
	return 0, nil
}

func (*workerStore) Statuses(context.Context, int64, domain.Transport, string, int64) ([]domain.Intent, error) {
	return nil, nil
}

func (*workerStore) Backlog(context.Context) (int, error) { return 0, nil }
func (*workerStore) OldestPendingAge(context.Context, time.Time) (time.Duration, error) {
	return 0, nil
}
func (*workerStore) ReplaceEmailDeliveries(context.Context, int64, string, int64, []domain.EmailDelivery) error {
	return nil
}
func (*workerStore) DeleteEmailDeliveries(context.Context, int64, string, int64) (int64, error) {
	return 0, nil
}
func (s *workerStore) EmailDeliveryStatuses(context.Context, int64, string, int64) ([]domain.EmailDeliveryStatus, error) {
	return s.statuses, nil
}
func (*workerStore) AttachEmailOutbox(context.Context, int64, int64, int64) error { return nil }
func (*workerStore) ClaimFailedEmailDelivery(context.Context, int64, int64) (bool, error) {
	return false, nil
}

type workerProvider struct {
	err         error
	calls       int
	hasDeadline bool
}

func (p *workerProvider) Send(ctx context.Context, _ domain.Intent) (domain.ProviderResult, error) {
	p.calls++
	_, p.hasDeadline = ctx.Deadline()
	return domain.ProviderResult{MessageID: "provider-1"}, p.err
}

func claimedEmail(attempts int) domain.Intent {
	return domain.Intent{ID: 41, TenantID: 7, Transport: domain.TransportEmail, Template: "welcome", Attempts: attempts}
}

func TestWorkerFinalizesWithClaimLeaseToken(t *testing.T) {
	t.Parallel()
	store := &workerStore{claimed: []domain.Intent{claimedEmail(0)}, renewed: true, finalizeSent: true}
	provider := &workerProvider{}
	worker := NewWorker(store, provider, func(domain.Observation) {})

	stats, err := worker.RunOnce(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, 1, stats.Claimed)
	assert.Equal(t, 1, stats.Sent)
	assert.NotEmpty(t, store.sentToken)
	assert.Equal(t, store.sentToken, store.renewToken)
	assert.Equal(t, 1, provider.calls)
	assert.True(t, provider.hasDeadline)
}

func TestWorkerCountsStaleFinalizeAsLeaseLoss(t *testing.T) {
	t.Parallel()
	store := &workerStore{claimed: []domain.Intent{claimedEmail(0)}, renewed: true, finalizeSent: false}
	worker := NewWorker(store, &workerProvider{}, func(domain.Observation) {})

	stats, err := worker.RunOnce(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, 1, stats.LeaseLost)
	assert.Zero(t, stats.Sent)
}

func TestWorkerSkipsIntentWhenLeaseRenewalIsStale(t *testing.T) {
	t.Parallel()
	store := &workerStore{claimed: []domain.Intent{claimedEmail(0)}}
	provider := &workerProvider{}
	worker := NewWorker(store, provider, func(domain.Observation) {})

	stats, err := worker.RunOnce(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, 1, stats.LeaseLost)
	assert.Zero(t, provider.calls)
}

func TestWorkerRetriesProviderTimeout(t *testing.T) {
	t.Parallel()
	store := &workerStore{
		claimed:       []domain.Intent{claimedEmail(0)},
		renewed:       true,
		failureResult: domain.FinalizeResult{Finalized: true, State: string(domain.StatePending)},
	}
	worker := NewWorker(store, &workerProvider{err: context.DeadlineExceeded}, func(domain.Observation) {})

	stats, err := worker.RunOnce(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, 1, stats.Retried)
	assert.Equal(t, 1, store.failureAttempts)
	assert.NotEmpty(t, store.failureToken)
}

func TestWorkerCancelsNonRetryableProviderDecision(t *testing.T) {
	t.Parallel()
	store := &workerStore{claimed: []domain.Intent{claimedEmail(0)}, renewed: true, finalizeCancelled: true}
	worker := NewWorker(store, &workerProvider{err: fmt.Errorf("guardian access revoked: %w", domain.ErrCancelled)}, func(domain.Observation) {})

	stats, err := worker.RunOnce(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, 1, stats.Cancelled)
	assert.Zero(t, stats.Retried)
	assert.NotEmpty(t, store.cancelledToken)
}

func TestWorkerDeadLettersAtAttemptLimit(t *testing.T) {
	t.Parallel()
	store := &workerStore{
		claimed:       []domain.Intent{claimedEmail(1)},
		renewed:       true,
		failureResult: domain.FinalizeResult{Finalized: true, State: string(domain.StateDeadLetter)},
	}
	worker := NewWorker(store, &workerProvider{err: errors.New("provider rejected")}, func(domain.Observation) {})
	worker.SetMaxAttempts(2)

	stats, err := worker.RunOnce(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, 1, stats.DeadLettered)
	assert.Equal(t, 2, store.failureAttempts)
	assert.Equal(t, 2, store.failureMaxAttempts)
}

// A provider may accept a message and the process may die before finalize.
// The expired lease is reclaimed and the provider sees the intent again: this
// is intentionally at-least-once delivery, fenced against stale state writes.
func TestWorkerRetriesAfterSendSucceededButFinalizeFailed(t *testing.T) {
	t.Parallel()
	store := &workerStore{
		claimed:         []domain.Intent{claimedEmail(0)},
		renewed:         true,
		finalizeSent:    true,
		finalizeSentErr: errors.New("database unavailable after provider acceptance"),
	}
	provider := &workerProvider{}
	worker := NewWorker(store, provider, func(domain.Observation) {})

	first, err := worker.RunOnce(context.Background(), 1)
	require.NoError(t, err)
	assert.Zero(t, first.Sent)
	store.finalizeSentErr = nil
	second, err := worker.RunOnce(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 1, second.Sent)
	assert.Equal(t, 2, provider.calls)
}
