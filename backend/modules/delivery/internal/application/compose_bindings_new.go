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
	Module  *delivery.Module
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
	return delivery.NewWorker(WorkerEngine{Worker: worker, Store: runtime.Store}), nil
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

type WorkerEngine struct {
	*Worker
	Store *WorkerCommands
}

func (e WorkerEngine) Backlog(ctx context.Context) (int, error) { return e.Store.Backlog(ctx) }
func (e WorkerEngine) RunOnce(ctx context.Context, batchSize, maxAttempts int) (deliveryContract.WorkerStats, error) {
	stats, err := e.Worker.RunOnce(ctx, batchSize, maxAttempts)
	return deliveryContract.WorkerStats(stats), err
}

type ProviderAdapter struct{ Provider Provider }

func (p ProviderAdapter) Send(ctx context.Context, intent domain.Intent) (domain.ProviderResult, error) {
	claimed := deliveryContract.ClaimedIntent{
		ID: intent.ID, TenantID: intent.TenantID, Transport: deliveryContract.Transport(intent.Transport),
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
			return domain.ProviderResult{}, fmt.Errorf("delivery provider: decode email recipient: %w", err)
		}
		claimed.EmailPayload = intent.Payload
		result, err := p.Provider.SendEmail(ctx, claimed)
		return ProviderResult(result, err)
	case deliveryContract.TransportPush:
		if err := json.Unmarshal(intent.Recipient, &claimed.PushRecipient); err != nil {
			return domain.ProviderResult{}, fmt.Errorf("delivery provider: decode push recipient: %w", err)
		}
		claimed.DeferredPayload = intent.Payload
		if err := json.Unmarshal(intent.Payload, &claimed.PushPayload); err != nil {
			return domain.ProviderResult{}, fmt.Errorf("delivery provider: decode push payload: %w", err)
		}
		result, err := p.Provider.SendPush(ctx, claimed)
		return ProviderResult(result, err)
	default:
		return domain.ProviderResult{}, fmt.Errorf("delivery provider: unknown transport %q", claimed.Transport)
	}
}
func ProviderResult(value deliveryContract.ProviderResult, err error) (domain.ProviderResult, error) {
	if errors.Is(err, deliveryContract.ErrCancelled) {
		err = fmt.Errorf("%w: %v", domain.ErrCancelled, err)
	}
	if errors.Is(err, deliveryContract.ErrDeferred) {
		err = fmt.Errorf("%w: %v", domain.ErrDeferred, err)
	}
	return domain.ProviderResult(value), err
}

type GuardianDisplayResolver interface {
	ResolveGuardianDisplays(context.Context, []int64) ([]deliveryContract.GuardianDisplay, error)
}
type GuardianDirectoryAdapter struct{ Resolver GuardianDisplayResolver }

func (a GuardianDirectoryAdapter) Resolve(ctx context.Context, ids []int64) (map[int64]domain.GuardianDisplay, error) {
	rows, err := a.Resolver.ResolveGuardianDisplays(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]domain.GuardianDisplay, len(rows))
	for _, row := range rows {
		result[row.GuardianProfileID] = domain.GuardianDisplay(row)
	}
	return result, nil
}

type ModuleEngine struct {
	Service *Service
	Store   *WorkerCommands
}

func (e ModuleEngine) EnqueueEmail(ctx context.Context, input deliveryContract.EmailIntent) (deliveryContract.Enqueued, error) {
	recipient, err := json.Marshal(input.Recipient)
	if err != nil {
		return deliveryContract.Enqueued{}, fmt.Errorf("delivery: marshal email recipient: %w", err)
	}
	return e.enqueue(ctx, domain.EnqueueInput{
		TenantID: input.TenantID, Transport: domain.TransportEmail, Template: input.Template,
		IdempotencyKey: input.IdempotencyKey, Related: domain.RelatedEntity(input.Related),
		Recipient: recipient, Payload: input.Payload, DeliverAfter: input.DeliverAfter,
	}, deliveryContract.TransportEmail)
}
func (e ModuleEngine) EnqueueDeferredPush(ctx context.Context, input deliveryContract.DeferredPushIntent) (deliveryContract.Enqueued, error) {
	return e.enqueue(ctx, domain.EnqueueInput{
		TenantID: input.TenantID, Transport: domain.TransportPush, Template: input.Template,
		IdempotencyKey: input.IdempotencyKey, Related: domain.RelatedEntity(input.Related),
		Recipient: json.RawMessage(`{"deferred":true}`), Payload: input.Payload,
	}, deliveryContract.TransportPush)
}
func (e ModuleEngine) EnqueuePush(ctx context.Context, input deliveryContract.PushIntent) (deliveryContract.Enqueued, error) {
	recipient, err := json.Marshal(input.Recipient)
	if err != nil {
		return deliveryContract.Enqueued{}, fmt.Errorf("delivery: marshal push recipient: %w", err)
	}
	payload, err := json.Marshal(input.Payload)
	if err != nil {
		return deliveryContract.Enqueued{}, fmt.Errorf("delivery: marshal push payload: %w", err)
	}
	return e.enqueue(ctx, domain.EnqueueInput{
		TenantID: input.TenantID, Transport: domain.TransportPush, Template: input.Template,
		IdempotencyKey: input.IdempotencyKey, Related: domain.RelatedEntity(input.Related),
		Recipient: recipient, Payload: payload,
	}, deliveryContract.TransportPush)
}
func (e ModuleEngine) enqueue(ctx context.Context, input domain.EnqueueInput, transport deliveryContract.Transport) (deliveryContract.Enqueued, error) {
	stored, err := e.Service.Enqueue(ctx, input)
	if err != nil {
		if errors.Is(err, domain.ErrIdempotencyConflict) {
			return deliveryContract.Enqueued{}, deliveryContract.ErrIdempotencyConflict
		}
		return deliveryContract.Enqueued{}, err
	}
	return deliveryContract.Enqueued{ID: stored.ID, Transport: transport, Duplicate: stored.Duplicate}, nil
}
func (e ModuleEngine) Cancel(ctx context.Context, tenantID int64, transport deliveryContract.Transport, related deliveryContract.RelatedEntity, reason string) (int64, error) {
	return e.Service.Cancel(ctx, tenantID, domain.Transport(transport), domain.RelatedEntity(related), reason)
}
func (e ModuleEngine) Statuses(ctx context.Context, tenantID int64, transport deliveryContract.Transport, related deliveryContract.RelatedEntity) ([]deliveryContract.Status, error) {
	rows, err := e.Service.Statuses(ctx, tenantID, domain.Transport(transport), domain.RelatedEntity(related))
	if err != nil {
		return nil, err
	}
	statuses := make([]deliveryContract.Status, 0, len(rows))
	for _, row := range rows {
		var providerResult *deliveryContract.ProviderResult
		if len(row.ProviderResult) > 0 {
			providerResult = &deliveryContract.ProviderResult{}
			if err := json.Unmarshal(row.ProviderResult, providerResult); err != nil {
				return nil, fmt.Errorf("delivery: decode provider result for %s intent %d: %w", transport, row.ID, err)
			}
		}
		statuses = append(statuses, deliveryContract.Status{
			ID: row.ID, IdempotencyKey: row.IdempotencyKey, Transport: transport, State: deliveryContract.State(row.Status), Attempts: row.Attempts,
			LastError: row.LastError, ProviderResult: providerResult, NextAttemptAt: row.NextRetryAt,
			SentAt: row.SentAt, DeadLetterAt: row.DeadLetterAt, CancelledAt: row.CancelledAt,
		})
	}
	return statuses, nil
}
func (e ModuleEngine) ReplaceEmailDeliveries(ctx context.Context, tenantID int64, related deliveryContract.RelatedEntity, rows []deliveryContract.EmailDelivery) error {
	values := make([]domain.EmailDelivery, 0, len(rows))
	for _, row := range rows {
		values = append(values, domain.EmailDelivery{
			ID: row.ID, OutboxID: row.OutboxID, GuardianProfileID: row.GuardianProfileID,
			AccountID: row.AccountID, RecipientEmail: row.RecipientEmail, Reachability: row.Reachability,
		})
	}
	return e.Store.ReplaceEmailDeliveries(ctx, tenantID, related.Type, related.ID, values)
}
func (e ModuleEngine) DeleteEmailDeliveries(ctx context.Context, tenantID int64, related deliveryContract.RelatedEntity) (int64, error) {
	return e.Store.DeleteEmailDeliveries(ctx, tenantID, related.Type, related.ID)
}
func (e ModuleEngine) EmailDeliveryStatuses(ctx context.Context, tenantID int64, related deliveryContract.RelatedEntity) ([]deliveryContract.EmailDeliveryStatus, error) {
	rows, err := e.Service.EmailDeliveryStatuses(ctx, tenantID, domain.RelatedEntity(related))
	if err != nil {
		return nil, err
	}
	statuses := make([]deliveryContract.EmailDeliveryStatus, 0, len(rows))
	for _, row := range rows {
		statuses = append(statuses, deliveryContract.EmailDeliveryStatus{
			DeliveryID: row.DeliveryID, GuardianProfileID: row.GuardianProfileID, AccountID: row.AccountID,
			FirstName: row.FirstName, LastName: row.LastName, RecipientEmail: row.RecipientEmail,
			Reachability: row.Reachability, EmailStatus: row.EmailStatus, LastError: row.LastError,
			SentAt: row.SentAt, Attempts: row.Attempts,
		})
	}
	return statuses, nil
}
func (e ModuleEngine) AttachEmailOutbox(ctx context.Context, tenantID, deliveryID, outboxID int64) error {
	return e.Store.AttachEmailOutbox(ctx, tenantID, deliveryID, outboxID)
}
func (e ModuleEngine) ClaimFailedEmailDelivery(ctx context.Context, tenantID, deliveryID int64) (bool, error) {
	return e.Store.ClaimFailedEmailDelivery(ctx, tenantID, deliveryID)
}
