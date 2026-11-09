package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
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
}

func NewWorker(store ports.Store, provider ports.Provider, observe ports.Observer) *Worker {
	if store == nil || provider == nil || observe == nil {
		panic("delivery worker: store, provider, and observer are required")
	}
	return &Worker{store: store, provider: provider, observe: observe, leaseDuration: defaultLeaseDuration}
}

func (w *Worker) RunOnce(ctx context.Context, batchSize, maxAttempts int) (stats deliveryContract.WorkerStats, err error) {
	if maxAttempts <= 0 {
		return stats, errors.New("delivery worker: max attempts must be positive")
	}
	// System identity mail has no school whose nightly retention can visit it.
	if retention, ok := w.store.(interface {
		DeleteExpiredSystemEmails(context.Context, time.Time, int) (int, error)
	}); ok && batchSize > 0 {
		count, err := retention.DeleteExpiredSystemEmails(ctx, time.Now().Add(-domain.RetentionWindow), batchSize)
		w.observe(domain.Observation{Operation: "system_email_retention", Count: count, Err: err})
		if err != nil {
			return stats, err
		}
	}
	emailLimit := (batchSize + 1) / 2
	pushLimit := batchSize / 2
	if err := w.runTransport(ctx, domain.TransportEmail, emailLimit, maxAttempts, &stats); err != nil {
		return stats, err
	}
	if pushLimit > 0 {
		err = w.runTransport(ctx, domain.TransportPush, pushLimit, maxAttempts, &stats)
	}
	age, ageErr := w.store.OldestPendingAge(ctx, time.Now())
	w.observe(domain.Observation{Operation: "oldest_pending_age", Duration: age, Count: 1, Err: ageErr})
	if err == nil && ageErr != nil {
		err = ageErr
	}
	return stats, err
}

func (w *Worker) runTransport(ctx context.Context, transport domain.Transport, limit, maxAttempts int, stats *deliveryContract.WorkerStats) error {
	now := time.Now()
	unknown, err := w.store.DeadLetterExpiredDispatches(ctx, transport, now)
	if err != nil {
		w.observe(domain.Observation{Operation: "dispatch_outcome_unknown", Transport: string(transport), Err: err})
		return fmt.Errorf("delivery worker: dead-letter expired %s dispatches: %w", transport, err)
	}
	if unknown > 0 {
		stats.DeadLettered += int(unknown)
		stats.DeadLetters = append(stats.DeadLetters, fmt.Sprintf("%d %s dispatches with unknown outcome: dispatch fence expired without finalization", unknown, transport))
		w.observe(domain.Observation{Operation: "dispatch_outcome_unknown", Transport: string(transport), Count: int(unknown), Err: errors.New("dispatch fence expired without finalization")})
	}
	claimStarted := time.Now()
	rows, err := w.store.Claim(ctx, transport, limit, now, now.Add(w.leaseDuration))
	w.observe(domain.Observation{Operation: "claim", Transport: string(transport), Duration: time.Since(claimStarted), Count: len(rows), Err: err})
	if err != nil {
		return fmt.Errorf("delivery worker: claim %s: %w", transport, err)
	}
	stats.Claimed += len(rows)
	if batch, ok := w.provider.(interface {
		PrepareBatch(context.Context, []domain.Intent) context.Context
	}); ok && transport == domain.TransportPush {
		ctx = batch.PrepareBatch(ctx, rows)
	}
	for index := range rows {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Before the dispatch fence: a mail behind the fence with an unknown
		// outcome is never retried, and this one has not been sent.
		if w.heldBackBySuppression(ctx, rows[index], stats) {
			continue
		}
		w.process(ctx, rows[index], maxAttempts, stats)
	}
	return nil
}

func (w *Worker) process(ctx context.Context, intent domain.Intent, maxAttempts int, stats *deliveryContract.WorkerStats) {
	var policy struct {
		MaxAttempts int `json:"delivery_max_attempts"`
	}
	if json.Unmarshal(intent.Payload, &policy) == nil && policy.MaxAttempts > 0 {
		maxAttempts = policy.MaxAttempts
	}
	started := time.Now()
	dispatchToken, active, err := w.store.StartDelivery(ctx, intent.Transport, intent.ID, leaseToken(intent), time.Now().Add(w.leaseDuration))
	if err != nil {
		w.observe(domain.Observation{Operation: "lease_fence_error", Transport: string(intent.Transport), Template: intent.Template, Count: 1, Err: err})
		return
	}
	if !active {
		stats.LeaseLost++
		w.observe(domain.Observation{Operation: "stale_lease_fence", Transport: string(intent.Transport), Template: intent.Template, Count: 1})
		return
	}
	providerCtx, cancel := context.WithTimeout(ctx, providerTimeout)
	result, sendErr := w.provider.Send(providerCtx, intent)
	cancel()
	w.observe(domain.Observation{Operation: "provider", Transport: string(intent.Transport), Template: intent.Template, Duration: time.Since(started), Count: boolCount(sendErr == nil), Err: sendErr})
	if errors.Is(sendErr, domain.ErrCancelled) {
		w.finalizeCancelled(ctx, intent, dispatchToken, sendErr, stats)
		return
	}
	if errors.Is(sendErr, domain.ErrDeferred) {
		w.finalizeDeferred(ctx, intent, dispatchToken, sendErr, stats)
		return
	}
	if errors.Is(sendErr, domain.ErrPermanent) {
		// No retry can change the answer: dead-letter on this attempt.
		maxAttempts = intent.Attempts + 1
		w.recordRecipientRejection(ctx, intent, sendErr)
	}
	if sendErr != nil {
		w.finalizeFailure(ctx, intent, dispatchToken, sendErr, maxAttempts, stats)
		return
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		w.finalizeFailure(ctx, intent, dispatchToken, fmt.Errorf("encode provider result: %w", err), maxAttempts, stats)
		return
	}
	finalized, err := w.store.FinalizeSent(ctx, intent.Transport, intent.ID, dispatchToken, encoded, time.Now())
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

// heldBackBySuppression ends a mail that stays back before it is sent. A failed
// read keeps the mail for a later attempt: the holds decide, so they must be known.
func (w *Worker) heldBackBySuppression(ctx context.Context, intent domain.Intent, stats *deliveryContract.WorkerStats) bool {
	hold, err := w.mailHold(ctx, intent)
	if err != nil {
		w.finalizeDeferred(ctx, intent, leaseToken(intent), err, stats)
		return true
	}
	if hold != nil {
		w.finalizeCancelled(ctx, intent, leaseToken(intent), hold, stats)
		w.recordHeldSend(ctx, intent, hold)
	}
	return hold != nil
}

func (w *Worker) finalizeCancelled(ctx context.Context, intent domain.Intent, token string, reason error, stats *deliveryContract.WorkerStats) {
	finalized, err := w.store.FinalizeCancelled(ctx, intent.Transport, intent.ID, token, reason.Error(), time.Now())
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

func (w *Worker) finalizeDeferred(ctx context.Context, intent domain.Intent, token string, reason error, stats *deliveryContract.WorkerStats) {
	finalized, err := w.store.FinalizeDeferred(ctx, intent.Transport, intent.ID, token, reason.Error(), time.Now().Add(retryBackoff[0]))
	if err != nil {
		w.observe(domain.Observation{Operation: "finalize_error", Transport: string(intent.Transport), Template: intent.Template, Count: 1, Err: err})
		return
	}
	if !finalized {
		stats.LeaseLost++
		w.observe(domain.Observation{Operation: "stale_finalize", Transport: string(intent.Transport), Template: intent.Template, Count: 1})
		return
	}
	stats.Retried++
	w.observe(domain.Observation{Operation: "deferred", Transport: string(intent.Transport), Template: intent.Template, Count: 1, Err: reason})
}

func (w *Worker) finalizeFailure(ctx context.Context, intent domain.Intent, token string, sendErr error, maxAttempts int, stats *deliveryContract.WorkerStats) {
	attempts := intent.Attempts + 1
	delay := backoff(attempts)
	var policy struct {
		Backoff []time.Duration `json:"delivery_backoff"`
	}
	if intent.Transport == domain.TransportEmail && json.Unmarshal(intent.Payload, &policy) == nil && len(policy.Backoff) > 0 {
		index := min(attempts-1, len(policy.Backoff)-1)
		delay = policy.Backoff[index]
	}
	nextAttempt := time.Now().Add(delay)
	result, err := w.store.FinalizeFailure(ctx, intent.Transport, intent.ID, token, attempts, sendErr.Error(), nextAttempt, maxAttempts)
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
		stats.DeadLetters = append(stats.DeadLetters, fmt.Sprintf("%s %s delivery %d: %v", intent.Transport, intent.Template, intent.ID, sendErr))
		w.observe(domain.Observation{Operation: "dead_letter", Transport: string(intent.Transport), Template: intent.Template, Count: 1, Err: sendErr})
		return
	}
	stats.Retried++
	w.observe(domain.Observation{Operation: "retry", Transport: string(intent.Transport), Template: intent.Template, Count: 1, Err: sendErr})
}

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
