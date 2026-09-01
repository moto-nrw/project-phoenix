package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/ports"
)

const defaultLeaseDuration = 2 * time.Minute
const providerTimeout = 30 * time.Second

var retryBackoff = []time.Duration{
	time.Minute,
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	12 * time.Hour,
	24 * time.Hour,
}

type Worker struct {
	store         ports.Store
	provider      ports.Provider
	observe       ports.Observer
	leaseDuration time.Duration
	mu            sync.RWMutex
	maxAttempts   int
}

func NewWorker(store ports.Store, provider ports.Provider, observe ports.Observer) *Worker {
	if store == nil || provider == nil || observe == nil {
		panic("delivery worker: store, provider, and observer are required")
	}
	return &Worker{store: store, provider: provider, observe: observe, leaseDuration: defaultLeaseDuration, maxAttempts: len(retryBackoff)}
}

func (w *Worker) SetMaxAttempts(attempts int) {
	if attempts <= 0 {
		return
	}
	w.mu.Lock()
	w.maxAttempts = attempts
	w.mu.Unlock()
}

func (w *Worker) RunOnce(ctx context.Context, batchSize int) (stats domain.WorkerStats, err error) {
	emailLimit := (batchSize + 1) / 2
	pushLimit := batchSize / 2
	if err := w.runTransport(ctx, domain.TransportEmail, emailLimit, &stats); err != nil {
		return stats, err
	}
	if pushLimit > 0 {
		err = w.runTransport(ctx, domain.TransportPush, pushLimit, &stats)
	}
	age, ageErr := w.store.OldestPendingAge(ctx, time.Now())
	w.observe(domain.Observation{Operation: "oldest_pending_age", Duration: age, Count: 1, Err: ageErr})
	if err == nil && ageErr != nil {
		err = ageErr
	}
	return stats, err
}

func (w *Worker) runTransport(ctx context.Context, transport domain.Transport, limit int, stats *domain.WorkerStats) error {
	now := time.Now()
	claimStarted := time.Now()
	rows, err := w.store.Claim(ctx, transport, limit, now, now.Add(w.leaseDuration))
	w.observe(domain.Observation{Operation: "claim", Transport: string(transport), Duration: time.Since(claimStarted), Count: len(rows), Err: err})
	if err != nil {
		return fmt.Errorf("delivery worker: claim %s: %w", transport, err)
	}
	stats.Claimed += len(rows)
	for index := range rows {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		renewed, renewErr := w.store.RenewLease(ctx, transport, rows[index].ID, leaseToken(rows[index]), time.Now().Add(w.leaseDuration))
		if renewErr != nil {
			w.observe(domain.Observation{Operation: "renew_error", Transport: string(transport), Template: rows[index].Template, Count: 1, Err: renewErr})
			continue
		}
		if !renewed {
			stats.LeaseLost++
			w.observe(domain.Observation{Operation: "stale_renew", Transport: string(transport), Template: rows[index].Template, Count: 1})
			continue
		}
		w.process(ctx, rows[index], stats)
	}
	return nil
}

func (w *Worker) process(ctx context.Context, intent domain.Intent, stats *domain.WorkerStats) {
	started := time.Now()
	providerCtx, cancel := context.WithTimeout(ctx, providerTimeout)
	result, sendErr := w.provider.Send(providerCtx, intent)
	cancel()
	w.observe(domain.Observation{Operation: "provider", Transport: string(intent.Transport), Template: intent.Template, Duration: time.Since(started), Count: boolCount(sendErr == nil), Err: sendErr})
	if errors.Is(sendErr, domain.ErrCancelled) {
		w.finalizeCancelled(ctx, intent, sendErr, stats)
		return
	}
	if sendErr != nil {
		w.finalizeFailure(ctx, intent, sendErr, stats)
		return
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		w.finalizeFailure(ctx, intent, fmt.Errorf("encode provider result: %w", err), stats)
		return
	}
	finalized, err := w.store.FinalizeSent(ctx, intent.Transport, intent.ID, leaseToken(intent), encoded, time.Now())
	if err != nil {
		w.observe(domain.Observation{Operation: "finalize_error", Transport: string(intent.Transport), Template: intent.Template, Count: 1, Err: err})
		return
	}
	if !finalized {
		stats.LeaseLost++
		w.observe(domain.Observation{Operation: "stale_finalize", Transport: string(intent.Transport), Template: intent.Template, Count: 1})
		return
	}
	stats.Sent++
	w.observe(domain.Observation{Operation: "sent", Transport: string(intent.Transport), Template: intent.Template, Count: 1})
}

func (w *Worker) finalizeCancelled(ctx context.Context, intent domain.Intent, reason error, stats *domain.WorkerStats) {
	finalized, err := w.store.FinalizeCancelled(ctx, intent.Transport, intent.ID, leaseToken(intent), reason.Error(), time.Now())
	if err != nil {
		w.observe(domain.Observation{Operation: "finalize_error", Transport: string(intent.Transport), Template: intent.Template, Count: 1, Err: err})
		return
	}
	if !finalized {
		stats.LeaseLost++
		w.observe(domain.Observation{Operation: "stale_finalize", Transport: string(intent.Transport), Template: intent.Template, Count: 1})
		return
	}
	stats.Cancelled++
	w.observe(domain.Observation{Operation: "cancelled", Transport: string(intent.Transport), Template: intent.Template, Count: 1})
}

func (w *Worker) finalizeFailure(ctx context.Context, intent domain.Intent, sendErr error, stats *domain.WorkerStats) {
	attempts := intent.Attempts + 1
	w.mu.RLock()
	maxAttempts := w.maxAttempts
	w.mu.RUnlock()
	nextAttempt := time.Now().Add(backoff(attempts))
	result, err := w.store.FinalizeFailure(ctx, intent.Transport, intent.ID, leaseToken(intent), attempts, sendErr.Error(), nextAttempt, maxAttempts)
	if err != nil {
		w.observe(domain.Observation{Operation: "finalize_error", Transport: string(intent.Transport), Template: intent.Template, Count: 1, Err: err})
		return
	}
	if !result.Finalized {
		stats.LeaseLost++
		w.observe(domain.Observation{Operation: "stale_finalize", Transport: string(intent.Transport), Template: intent.Template, Count: 1})
		return
	}
	if result.State == string(domain.StateDeadLetter) {
		stats.DeadLettered++
		w.observe(domain.Observation{Operation: "dead_letter", Transport: string(intent.Transport), Template: intent.Template, Count: 1, Err: sendErr})
		return
	}
	stats.Retried++
	w.observe(domain.Observation{Operation: "retry", Transport: string(intent.Transport), Template: intent.Template, Count: 1, Err: sendErr})
}

func (w *Worker) Backlog(ctx context.Context) (int, error) { return w.store.Backlog(ctx) }

func leaseToken(intent domain.Intent) string {
	if intent.LeaseToken == nil {
		panic("delivery worker: claimed intent has no lease token")
	}
	return *intent.LeaseToken
}

func backoff(attempts int) time.Duration {
	if attempts <= 1 {
		return retryBackoff[0]
	}
	index := attempts - 1
	if index >= len(retryBackoff) {
		index = len(retryBackoff) - 1
	}
	return retryBackoff[index]
}
