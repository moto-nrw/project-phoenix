package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	"github.com/moto-ogs/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/delivery/mail"
	delivery "github.com/moto-ogs/project-phoenix/modules/delivery/runtime"
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
	// EmailWakeups reports committed e-mail enqueues; nil polls only.
	EmailWakeups func(context.Context) <-chan struct{}
	// EmailWebhooks takes in what mail providers report.
	EmailWebhooks *EmailWebhooks
	// EmailReconciliation compares open mails with the provider's record and
	// changes a school's list. On plain SMTP it compares nothing, and lifting a
	// block only touches the school's list.
	EmailReconciliation *EmailReconciliation
	// EmailOutcomes reads what became of a school's mails.
	EmailOutcomes delivery.EmailOutcomes
	// TraegerMails reads back and repeats Träger mails by key (ADR 0196).
	TraegerMails delivery.TraegerMails
}

// emailOutcomeRead answers from the store and, for a block at the provider, from
// the provider.
type emailOutcomeRead struct {
	*WorkerCommands
	*EmailReconciliation
}

// NewRuntime composes the module for this deployment's mail provider.
func NewRuntime(module delivery.Capability, store *WorkerCommands, observe func(domain.Observation), wakeups func(context.Context) <-chan struct{}, provider mail.ProviderConfig) *Runtime {
	webhooks, reconciliation := NewEmailEvents(provider, store, observe)
	return &Runtime{Module: module, Store: store, Observe: observe, EmailWakeups: wakeups, EmailWebhooks: webhooks, EmailReconciliation: reconciliation, EmailOutcomes: emailOutcomeRead{WorkerCommands: store, EmailReconciliation: reconciliation}, TraegerMails: traegerMailsOf(module)}
}

// NewWorker composes the outbox worker over the store of runtime, sending
// through provider.
func NewWorkerBinding(runtime *Runtime, provider Provider) (*delivery.Worker, error) {
	if runtime == nil || runtime.Store == nil || runtime.Observe == nil || provider == nil {
		return nil, errors.New("delivery compose: runtime and provider are required")
	}
	worker := NewWorker(runtime.Store, ProviderAdapter{Provider: provider}, runtime.Observe)
	return delivery.NewWorker(workerEngine{Worker: worker, workerBacklog: runtime.Store, wakeups: runtime.EmailWakeups, EmailReconciliation: runtime.EmailReconciliation}), nil
}

// workerEngine runs the worker and counts the backlog, the intents still
// waiting for delivery, straight from the worker's store.
type workerEngine struct {
	*Worker
	workerBacklog
	*EmailReconciliation
	wakeups func(context.Context) <-chan struct{}
}

// EmailWakeups reports committed e-mail enqueues until ctx ends; a nil
// channel when the runtime has no listener.
func (e workerEngine) EmailWakeups(ctx context.Context) <-chan struct{} {
	if e.wakeups == nil {
		return nil
	}
	return e.wakeups(ctx)
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

// PrepareBatch lets the provider resolve object authority once per claimed
// push batch. The returned context is local to this run, never a retry cache.
func (p ProviderAdapter) PrepareBatch(ctx context.Context, intents []domain.Intent) context.Context {
	batch, ok := p.Provider.(interface {
		PreparePushBatch(context.Context, []deliveryContract.ClaimedIntent) context.Context
	})
	if !ok {
		return ctx
	}
	pushes := make([]deliveryContract.ClaimedIntent, 0, len(intents))
	for _, intent := range intents {
		var recipient deliveryContract.PushRecipient
		if json.Unmarshal(intent.Recipient, &recipient) != nil || recipient.Deferred {
			continue
		}
		pushes = append(pushes, deliveryContract.ClaimedIntent{
			ID: intent.ID, TenantID: intent.TenantID, Template: intent.Template,
			RelatedEntityType: intent.RelatedEntityType, RelatedEntityID: intent.RelatedEntityID,
			PushRecipient: recipient,
		})
	}
	return batch.PreparePushBatch(ctx, pushes)
}

func (p ProviderAdapter) Send(ctx context.Context, intent domain.Intent) (deliveryContract.ProviderResult, error) {
	claimed := deliveryContract.ClaimedIntent{
		ID: intent.ID, TenantID: intent.TenantID, OrganizationID: intent.OrganizationID, Transport: deliveryContract.Transport(intent.Transport),
		Template: intent.Template, DeliveryToken: intent.DeliveryToken, Security: intent.Security, UseFallback: intent.UseFallback, RelatedEntityType: intent.RelatedEntityType,
		RelatedEntityID: intent.RelatedEntityID, Attempts: intent.Attempts, CreatedAt: intent.CreatedAt,
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
	if errors.Is(err, deliveryContract.ErrThrottled) {
		err = fmt.Errorf("%w: %w: %v", domain.ErrDeferred, domain.ErrThrottled, err)
	} else if errors.Is(err, deliveryContract.ErrDeferred) {
		err = fmt.Errorf("%w: %v", domain.ErrDeferred, err)
	}
	if errors.Is(err, deliveryContract.ErrRecipientRejected) {
		return value, fmt.Errorf("%w: %w: %v", domain.ErrPermanent, domain.ErrRecipientRejected, err)
	}
	if errors.Is(err, deliveryContract.ErrPermanent) {
		err = fmt.Errorf("%w: %v", domain.ErrPermanent, err)
	}
	return value, err
}

// traegerMailsOf is the Träger mail side of the composed service; a test
// double of the capability has none.
func traegerMailsOf(module delivery.Capability) delivery.TraegerMails {
	mails, _ := module.(delivery.TraegerMails)
	return mails
}
