package emailoutbox

import (
	"context"
	"errors"
	"fmt"

	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
)

type pushObjectBatchKey struct{}
type pushObjectBatch struct {
	provider *MailPushProvider
	allowed  map[int64]bool
	err      error
}

// PreparePushBatch reads through the consumer-owned port once for the entire
// worker batch. Decisions live only in this run's context, including errors.
func (p *MailPushProvider) PreparePushBatch(ctx context.Context, intents []delivery.ClaimedIntent) context.Context {
	if p.PushObjects == nil || len(intents) == 0 {
		return ctx
	}
	allowed, err := p.PushObjects(ctx, intents)
	return context.WithValue(ctx, pushObjectBatchKey{}, pushObjectBatch{provider: p, allowed: allowed, err: err})
}

func (p *MailPushProvider) requirePushObject(ctx context.Context, intent delivery.ClaimedIntent) error {
	if p.PushObjects == nil {
		return nil
	}
	batch, ok := ctx.Value(pushObjectBatchKey{}).(pushObjectBatch)
	if !ok || batch.provider != p {
		// Targeted dispatch has no worker batch, but must enforce the same gate.
		batch.allowed, batch.err = p.PushObjects(ctx, []delivery.ClaimedIntent{intent})
	}
	// Explicit allow decisions remain usable when another intent's lookup fails.
	if batch.allowed[intent.ID] {
		return nil
	}
	if batch.err != nil {
		// Owner errors may contain recipient data. The durable error and log retain
		// only the safe classification; a lookup failure remains retryable.
		return errors.New("delivery provider: object authorization lookup failed")
	}
	return fmt.Errorf("%w: object no longer valid or recipient no longer authorized", delivery.ErrCancelled)
}
