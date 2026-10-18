package application

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
)

type targetedEmailStore interface {
	ClaimEmail(context.Context, int64, int64, time.Time, time.Time) (domain.Intent, bool, error)
	RecordEmailRetry(context.Context, int64, string, int, time.Time) (bool, error)
}

// DispatchEmail sends the requested committed entry, never another caller's
// mail. A worker that won the claim is observed through the same terminal state.
func (w *Worker) DispatchEmail(ctx context.Context, tenantID, id int64) error {
	store, ok := w.store.(targetedEmailStore)
	if !ok {
		return errors.New("delivery: targeted email store is not configured")
	}
	for {
		now := time.Now()
		intent, acquired, err := store.ClaimEmail(ctx, tenantID, id, now, synchronousPushLease(ctx, w.leaseDuration))
		if err != nil {
			return err
		}
		switch domain.State(intent.Status) {
		case domain.StateSent:
			return nil
		case domain.StateDeadLetter:
			if intent.LastError != nil {
				return errors.New(*intent.LastError)
			}
			return errors.New("delivery: email previously failed")
		case domain.StateCancelled:
			return domain.ErrCancelled
		}
		if acquired {
			return w.deliverEmail(ctx, store, intent)
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (w *Worker) deliverEmail(ctx context.Context, store targetedEmailStore, intent domain.Intent) error {
	token, active, err := w.store.StartDelivery(ctx, intent.Transport, intent.ID, leaseToken(intent), synchronousPushLease(ctx, w.leaseDuration))
	if err != nil {
		return err
	}
	if !active {
		return errors.New("delivery: email lease was lost before dispatch")
	}
	result, attempts, sendErr := w.sendEmailAttempts(ctx, store, intent, token)
	finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), providerTimeout)
	defer cancel()
	if errors.Is(sendErr, domain.ErrCancelled) {
		done, err := w.store.FinalizeCancelled(finalizeCtx, intent.Transport, intent.ID, token, sendErr.Error(), time.Now())
		if err == nil && !done {
			err = errors.New("delivery: cancelled email lost its lease")
		}
		return errors.Join(sendErr, err)
	}
	if sendErr != nil {
		state, err := w.store.FinalizeFailure(finalizeCtx, intent.Transport, intent.ID, token, attempts, sendErr.Error(), time.Now(), 1)
		if err == nil && !state.Finalized {
			err = errors.New("delivery: failed email lost its lease")
		}
		return errors.Join(sendErr, err)
	}
	return w.finalizeSynchronousPush(finalizeCtx, intent, token, result)
}

func (w *Worker) sendEmailAttempts(ctx context.Context, store targetedEmailStore, intent domain.Intent, token string) (domain.ProviderResult, int, error) {
	policy := struct {
		MaxAttempts int             `json:"delivery_max_attempts"`
		Backoff     []time.Duration `json:"delivery_backoff"`
	}{MaxAttempts: 3, Backoff: []time.Duration{time.Second, 5 * time.Second, 15 * time.Second}}
	if err := json.Unmarshal(intent.Payload, &policy); err != nil {
		return domain.ProviderResult{}, intent.Attempts, err
	}
	for {
		result, sendErr := w.provider.Send(ctx, intent)
		attempts := intent.Attempts + 1
		if sendErr == nil || errors.Is(sendErr, domain.ErrCancelled) || attempts >= policy.MaxAttempts {
			return result, attempts, sendErr
		}
		delay := time.Duration(0)
		if len(policy.Backoff) > 0 {
			delay = policy.Backoff[min(attempts-1, len(policy.Backoff)-1)]
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return result, attempts, errors.Join(sendErr, ctx.Err())
		case <-timer.C:
		}
		updated, err := store.RecordEmailRetry(ctx, intent.ID, token, attempts, time.Now())
		if err != nil {
			return result, attempts, errors.Join(sendErr, err)
		}
		if !updated {
			return result, attempts, errors.New("delivery: email retry lost its lease")
		}
		intent.Attempts = attempts
	}
}
