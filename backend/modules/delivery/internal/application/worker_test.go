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

	stats, err := worker.RunOnce(context.Background(), 1, 6)

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

	stats, err := worker.RunOnce(context.Background(), 1, 6)

	require.NoError(t, err)
	assert.Equal(t, 1, stats.LeaseLost)
	assert.Zero(t, stats.Sent)
}

func TestWorkerSkipsIntentWhenLeaseRenewalIsStale(t *testing.T) {
	t.Parallel()
	store := &workerStore{claimed: []domain.Intent{claimedEmail(0)}}
	provider := &workerProvider{}
	worker := NewWorker(store, provider, func(domain.Observation) {})

	stats, err := worker.RunOnce(context.Background(), 1, 6)

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

	stats, err := worker.RunOnce(context.Background(), 1, 6)

	require.NoError(t, err)
	assert.Equal(t, 1, stats.Retried)
	assert.Equal(t, 1, store.failureAttempts)
	assert.NotEmpty(t, store.failureToken)
}

func TestWorkerCancelsNonRetryableProviderDecision(t *testing.T) {
	t.Parallel()
	store := &workerStore{claimed: []domain.Intent{claimedEmail(0)}, renewed: true, finalizeCancelled: true}
	worker := NewWorker(store, &workerProvider{err: fmt.Errorf("guardian access revoked: %w", domain.ErrCancelled)}, func(domain.Observation) {})

	stats, err := worker.RunOnce(context.Background(), 1, 6)

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

	stats, err := worker.RunOnce(context.Background(), 1, 2)

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

	first, err := worker.RunOnce(context.Background(), 1, 6)
	require.NoError(t, err)
	assert.Zero(t, first.Sent)
	store.finalizeSentErr = nil
	second, err := worker.RunOnce(context.Background(), 1, 6)
	require.NoError(t, err)
	assert.Equal(t, 1, second.Sent)
	assert.Equal(t, 2, provider.calls)
}

func TestWorkerRejectsInvalidRetryLimitBeforeClaim(t *testing.T) {
	t.Parallel()
	for _, limit := range []int{0, -1} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			worker := NewWorker(&workerStore{}, &workerProvider{}, func(domain.Observation) { t.Error("invalid policy must not start work") })
			stats, err := worker.RunOnce(context.Background(), 1, limit)
			require.EqualError(t, err, "delivery worker: max attempts must be positive")
			assert.Equal(t, domain.WorkerStats{}, stats)
		})
	}
}

type concurrentRetryStore struct{ workerStore }

func (*concurrentRetryStore) Claim(_ context.Context, transport domain.Transport, _ int, _ time.Time, _ time.Time) ([]domain.Intent, error) {
	intent := claimedEmail(1)
	intent.Transport = transport
	token := "concurrent-lease"
	intent.LeaseToken = &token
	return []domain.Intent{intent}, nil
}

func (*concurrentRetryStore) RenewLease(context.Context, domain.Transport, int64, string, time.Time) (bool, error) {
	return true, nil
}

func (*concurrentRetryStore) FinalizeFailure(_ context.Context, _ domain.Transport, _ int64, _ string, attempts int, _ string, _ time.Time, maxAttempts int) (domain.FinalizeResult, error) {
	state := domain.StatePending
	if attempts >= maxAttempts {
		state = domain.StateDeadLetter
	}
	return domain.FinalizeResult{Finalized: true, State: string(state)}, nil
}

type blockedRetryProvider struct {
	started chan struct{}
	release chan struct{}
}

func (p blockedRetryProvider) Send(ctx context.Context, intent domain.Intent) (domain.ProviderResult, error) {
	if intent.Transport == domain.TransportEmail {
		p.started <- struct{}{}
		select {
		case <-p.release:
		case <-ctx.Done():
			return domain.ProviderResult{}, ctx.Err()
		}
	}
	return domain.ProviderResult{}, errors.New("provider rejected")
}

func TestWorkerConcurrentRunsKeepIndependentRetryLimits(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	provider := blockedRetryProvider{started: make(chan struct{}, 2), release: make(chan struct{})}
	worker := NewWorker(&concurrentRetryStore{}, provider, func(domain.Observation) {})
	type result struct {
		stats domain.WorkerStats
		err   error
	}
	low, high := make(chan result, 1), make(chan result, 1)
	run := func(limit int, results chan<- result) {
		stats, err := worker.RunOnce(ctx, 2, limit)
		results <- result{stats, err}
	}
	go run(2, low)
	select {
	case <-provider.started:
	case <-ctx.Done():
		t.Fatal("first run did not reach provider")
	}
	go run(6, high)
	select {
	case <-provider.started:
	case <-ctx.Done():
		t.Fatal("second run did not overlap first run")
	}
	close(provider.release)
	first, second := <-low, <-high
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	assert.Equal(t, domain.WorkerStats{Claimed: 2, DeadLettered: 2}, first.stats)
	assert.Equal(t, domain.WorkerStats{Claimed: 2, Retried: 2}, second.stats)
}
