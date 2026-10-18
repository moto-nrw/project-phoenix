package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// DeferredPushIntent persists a feature-owned push request before its audience
// is revalidated at dispatch time. Delivery owns the same lease and terminal
// states as device-snapshot intents; the root binds the feature dispatcher.
type DeferredPushIntent struct {
	TenantID       int64
	Template       string
	IdempotencyKey string
	Related        RelatedEntity
	Payload        json.RawMessage
}

func (m *Module) EnqueueDeferredPush(ctx context.Context, intent DeferredPushIntent) (Enqueued, error) {
	if intent.TenantID <= 0 || strings.TrimSpace(intent.Template) == "" || strings.TrimSpace(intent.IdempotencyKey) == "" {
		return Enqueued{}, errors.New("delivery: deferred push requires tenant, template, and idempotency key")
	}
	if len(intent.Payload) == 0 || !json.Valid(intent.Payload) {
		return Enqueued{}, errors.New("delivery: invalid deferred push payload")
	}
	if err := validateRelated(intent.Related); err != nil {
		return Enqueued{}, err
	}
	return m.engine.EnqueueDeferredPush(ctx, intent)
}

type pushDispatchEngine interface {
	DispatchPush(context.Context, int64, int64) error
}

// PushDispatcher claims exactly one committed intent and returns its transport
// result. A transport failure is terminal, so a manual retry cannot send twice.
type PushDispatcher struct{ engine pushDispatchEngine }

func NewPushDispatcher(engine pushDispatchEngine) *PushDispatcher {
	return &PushDispatcher{engine: engine}
}

func (d *PushDispatcher) Deliver(ctx context.Context, tenantID, id int64) error {
	if tenantID <= 0 || id <= 0 {
		return errors.New("delivery: tenant and push intent are required")
	}
	return d.engine.DispatchPush(ctx, tenantID, id)
}
