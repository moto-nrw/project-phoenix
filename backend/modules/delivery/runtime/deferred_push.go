package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
)

func (m *Module) EnqueueDeferredPush(ctx context.Context, intent deliveryContract.DeferredPushIntent) (deliveryContract.Enqueued, error) {
	intent.Template = strings.TrimSpace(intent.Template)
	intent.IdempotencyKey = strings.TrimSpace(intent.IdempotencyKey)
	intent.Related.Type = strings.TrimSpace(intent.Related.Type)
	if intent.TenantID <= 0 || intent.Template == "" || intent.IdempotencyKey == "" {
		return deliveryContract.Enqueued{}, errors.New("delivery: deferred push requires tenant, template, and idempotency key")
	}
	if len(intent.Payload) == 0 || !json.Valid(intent.Payload) {
		return deliveryContract.Enqueued{}, errors.New("delivery: invalid deferred push payload")
	}
	if err := validateRelated(intent.Related); err != nil {
		return deliveryContract.Enqueued{}, err
	}
	return m.engine.EnqueueDeferredPush(ctx, intent)
}

// PushDispatcher claims exactly one committed intent and returns its transport
// result. A transport failure is terminal, so a manual retry cannot send twice.
type PushDispatcher struct {
	dispatch func(context.Context, int64, int64) error
}

func NewPushDispatcher(dispatch func(context.Context, int64, int64) error) *PushDispatcher {
	return &PushDispatcher{dispatch: dispatch}
}

func (d *PushDispatcher) Deliver(ctx context.Context, tenantID, id int64) error {
	if tenantID <= 0 || id <= 0 {
		return errors.New("delivery: tenant and push intent are required")
	}
	return d.dispatch(ctx, tenantID, id)
}
