package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
)

type Provider interface {
	SendEmail(context.Context, deliveryContract.ClaimedIntent) (deliveryContract.ProviderResult, error)
	SendPush(context.Context, deliveryContract.ClaimedIntent) (deliveryContract.ProviderResult, error)
}

// Runtime is the composed Delivery module; NewWorker adds the worker.
type Runtime struct {
	Module  delivery.Capability
	Store   *WorkerCommands
	Observe func(domain.Observation)
}

// NewWorker composes the outbox worker over the store of runtime, sending
// through provider.
func NewWorkerBinding(runtime *Runtime, provider Provider) (*delivery.Worker, error) {
	if runtime == nil || runtime.Store == nil || runtime.Observe == nil || provider == nil {
		return nil, errors.New("delivery compose: runtime and provider are required")
	}
	worker := NewWorker(runtime.Store, ProviderAdapter{Provider: provider}, runtime.Observe)
	return delivery.NewWorker(workerEngine{Worker: worker, workerBacklog: runtime.Store}), nil
}

// workerEngine runs the worker and counts the backlog, the intents still
// waiting for delivery, straight from the worker's store.
type workerEngine struct {
	*Worker
	workerBacklog
}

type workerBacklog interface {
	Backlog(context.Context) (int, error)
}

// EmailStatus reports the durable state of an e-mail intent.
func (r *Runtime) EmailStatus(ctx context.Context, tenantID, id int64) (deliveryContract.State, bool, error) {
	if r == nil || r.Store == nil {
		return "", false, errors.New("delivery compose: service is not initialized")
	}
	if tenantID <= 0 || id <= 0 {
		return "", false, errors.New("delivery compose: tenant and e-mail intent are required")
	}
	intent, found, err := r.Store.EmailStatus(ctx, tenantID, id)
	if err != nil || !found {
		return "", found, err
	}
	return deliveryContract.State(intent.Status), true, nil
}

// NewPushDispatcher builds a targeted caller over the existing outbox store.
func NewPushDispatcher(runtime *Runtime, provider Provider) *delivery.PushDispatcher {
	worker := NewWorker(runtime.Store, ProviderAdapter{Provider: provider}, runtime.Observe)
	return delivery.NewPushDispatcher(func(ctx context.Context, tenantID, id int64) error {
		err := worker.DispatchPush(ctx, tenantID, id)
		if errors.Is(err, domain.ErrCancelled) {
			return fmt.Errorf("%w: %w", deliveryContract.ErrCancelled, err)
		}
		return err
	})
}

type ProviderAdapter struct{ Provider Provider }

func (p ProviderAdapter) Send(ctx context.Context, intent domain.Intent) (deliveryContract.ProviderResult, error) {
	claimed := deliveryContract.ClaimedIntent{
		ID: intent.ID, TenantID: intent.TenantID, OrganizationID: intent.OrganizationID, Transport: deliveryContract.Transport(intent.Transport),
		Template: intent.Template, RelatedEntityType: intent.RelatedEntityType,
		RelatedEntityID: intent.RelatedEntityID, Attempts: intent.Attempts,
	}
	if intent.LeaseToken != nil {
		claimed.LeaseToken = *intent.LeaseToken
	}
	if intent.LeaseExpiresAt != nil {
		claimed.LeaseExpiresAt = *intent.LeaseExpiresAt
	}
	switch claimed.Transport {
	case deliveryContract.TransportEmail:
		if err := json.Unmarshal(intent.Recipient, &claimed.EmailRecipient); err != nil {
			return deliveryContract.ProviderResult{}, fmt.Errorf("delivery provider: decode email recipient: %w", err)
		}
		claimed.EmailPayload = intent.Payload
		result, err := p.Provider.SendEmail(ctx, claimed)
		return ProviderResult(result, err)
	case deliveryContract.TransportPush:
		if err := json.Unmarshal(intent.Recipient, &claimed.PushRecipient); err != nil {
			return deliveryContract.ProviderResult{}, fmt.Errorf("delivery provider: decode push recipient: %w", err)
		}
		claimed.DeferredPayload = intent.Payload
		if err := json.Unmarshal(intent.Payload, &claimed.PushPayload); err != nil {
			return deliveryContract.ProviderResult{}, fmt.Errorf("delivery provider: decode push payload: %w", err)
		}
		result, err := p.Provider.SendPush(ctx, claimed)
		return ProviderResult(result, err)
	default:
		return deliveryContract.ProviderResult{}, fmt.Errorf("delivery provider: unknown transport %q", claimed.Transport)
	}
}
func ProviderResult(value deliveryContract.ProviderResult, err error) (deliveryContract.ProviderResult, error) {
	if errors.Is(err, deliveryContract.ErrCancelled) {
		err = fmt.Errorf("%w: %v", domain.ErrCancelled, err)
	}
	if errors.Is(err, deliveryContract.ErrDeferred) {
		err = fmt.Errorf("%w: %v", domain.ErrDeferred, err)
	}
	return value, err
}
