package compose

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Provider interface {
	SendEmail(context.Context, delivery.ClaimedIntent) (delivery.ProviderResult, error)
	SendPush(context.Context, delivery.ClaimedIntent) (delivery.ProviderResult, error)
}

type GuardianDisplayResolver interface {
	ResolveGuardianDisplays(context.Context, []int64) ([]delivery.GuardianDisplay, error)
}

// ModuleDependencies are the enqueue and state side of Delivery, which the
// producers and Identity & Access need before the mail renderers exist.
type ModuleDependencies struct {
	DB      *bun.DB
	People  GuardianDisplayResolver
	Observe func(delivery.Observation)
}

// Runtime is the composed Delivery module; NewWorker adds the worker.
type Runtime struct {
	Module  *delivery.Module
	store   *postgres.Store
	observe func(domain.Observation)
}

// NewModule composes the enqueue facade and the e-mail state without the
// worker; NewWorker adds the worker once the mail renderers exist.
func NewModule(dependencies ModuleDependencies) (*Runtime, error) {
	if dependencies.DB == nil || dependencies.People == nil || dependencies.Observe == nil {
		return nil, errors.New("delivery compose: all dependencies are required")
	}
	store := postgres.New(
		postgres.RequireTenantDatabase,
		func(ctx context.Context, callback func(context.Context, bun.IDB) error) error {
			return tenant.WithAdminTx(ctx, dependencies.DB, func(txCtx context.Context, tx bun.Tx) error {
				return callback(txCtx, tx)
			})
		},
	)
	observe := func(observation domain.Observation) {
		dependencies.Observe(delivery.Observation{
			Operation: observation.Operation, Transport: delivery.Transport(observation.Transport),
			Template: observation.Template, Duration: observation.Duration, Count: observation.Count, Err: observation.Err,
		})
	}
	service := application.NewService(store, guardianDirectoryAdapter{resolver: dependencies.People}, observe)
	return &Runtime{Module: delivery.NewModule(moduleEngine{service: service, store: store}), store: store, observe: observe}, nil
}

// NewWorker composes the outbox worker over the store of runtime, sending
// through provider.
func NewWorker(runtime *Runtime, provider Provider) (*delivery.Worker, error) {
	if runtime == nil || runtime.store == nil || runtime.observe == nil || provider == nil {
		return nil, errors.New("delivery compose: runtime and provider are required")
	}
	worker := application.NewWorker(runtime.store, providerAdapter{provider: provider}, runtime.observe)
	return delivery.NewWorker(workerEngine{Worker: worker}), nil
}

// EmailStatus reports the durable state of an e-mail intent.
func (r *Runtime) EmailStatus(ctx context.Context, tenantID, id int64) (delivery.State, bool, error) {
	if r == nil || r.store == nil {
		return "", false, errors.New("delivery compose: service is not initialized")
	}
	if tenantID <= 0 || id <= 0 {
		return "", false, errors.New("delivery compose: tenant and e-mail intent are required")
	}
	intent, found, err := r.store.EmailStatus(ctx, tenantID, id)
	if err != nil || !found {
		return "", found, err
	}
	return delivery.State(intent.Status), true, nil
}

type guardianDirectoryAdapter struct{ resolver GuardianDisplayResolver }

func (a guardianDirectoryAdapter) Resolve(ctx context.Context, ids []int64) (map[int64]domain.GuardianDisplay, error) {
	rows, err := a.resolver.ResolveGuardianDisplays(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]domain.GuardianDisplay, len(rows))
	for _, row := range rows {
		result[row.GuardianProfileID] = domain.GuardianDisplay(row)
	}
	return result, nil
}

type moduleEngine struct {
	service *application.Service
	store   *postgres.Store
}

func (e moduleEngine) EnqueueEmail(ctx context.Context, input delivery.EmailIntent) (delivery.Enqueued, error) {
	recipient, err := json.Marshal(input.Recipient)
	if err != nil {
		return delivery.Enqueued{}, fmt.Errorf("delivery: marshal email recipient: %w", err)
	}
	return e.enqueue(ctx, domain.EnqueueInput{
		TenantID: input.TenantID, Transport: domain.TransportEmail, Template: input.Template,
		IdempotencyKey: input.IdempotencyKey, Related: domain.RelatedEntity(input.Related),
		Recipient: recipient, Payload: input.Payload, DeliverAfter: input.DeliverAfter,
	}, delivery.TransportEmail)
}

func (e moduleEngine) EnqueueDeferredPush(ctx context.Context, input delivery.DeferredPushIntent) (delivery.Enqueued, error) {
	return e.enqueue(ctx, domain.EnqueueInput{
		TenantID: input.TenantID, Transport: domain.TransportPush, Template: input.Template,
		IdempotencyKey: input.IdempotencyKey, Related: domain.RelatedEntity(input.Related),
		Recipient: json.RawMessage(`{"deferred":true}`), Payload: input.Payload,
	}, delivery.TransportPush)
}

// NewPushDispatcher builds a targeted caller over the existing outbox store.
func NewPushDispatcher(runtime *Runtime, provider Provider) *delivery.PushDispatcher {
	worker := application.NewWorker(runtime.store, providerAdapter{provider: provider}, runtime.observe)
	return delivery.NewPushDispatcher(func(ctx context.Context, tenantID, id int64) error {
		err := worker.DispatchPush(ctx, tenantID, id)
		if errors.Is(err, domain.ErrCancelled) {
			return fmt.Errorf("%w: %w", delivery.ErrCancelled, err)
		}
		return err
	})
}

func (e moduleEngine) EnqueuePush(ctx context.Context, input delivery.PushIntent) (delivery.Enqueued, error) {
	recipient, err := json.Marshal(input.Recipient)
	if err != nil {
		return delivery.Enqueued{}, fmt.Errorf("delivery: marshal push recipient: %w", err)
	}
	payload, err := json.Marshal(input.Payload)
	if err != nil {
		return delivery.Enqueued{}, fmt.Errorf("delivery: marshal push payload: %w", err)
	}
	return e.enqueue(ctx, domain.EnqueueInput{
		TenantID: input.TenantID, Transport: domain.TransportPush, Template: input.Template,
		IdempotencyKey: input.IdempotencyKey, Related: domain.RelatedEntity(input.Related),
		Recipient: recipient, Payload: payload,
	}, delivery.TransportPush)
}

func (e moduleEngine) enqueue(ctx context.Context, input domain.EnqueueInput, transport delivery.Transport) (delivery.Enqueued, error) {
	stored, err := e.service.Enqueue(ctx, input)
	if err != nil {
		if errors.Is(err, domain.ErrIdempotencyConflict) {
			return delivery.Enqueued{}, delivery.ErrIdempotencyConflict
		}
		return delivery.Enqueued{}, err
	}
	return delivery.Enqueued{ID: stored.ID, Transport: transport, Duplicate: stored.Duplicate}, nil
}

func (e moduleEngine) Cancel(ctx context.Context, tenantID int64, transport delivery.Transport, related delivery.RelatedEntity, reason string) (int64, error) {
	return e.service.Cancel(ctx, tenantID, domain.Transport(transport), domain.RelatedEntity(related), reason)
}

func (e moduleEngine) Statuses(ctx context.Context, tenantID int64, transport delivery.Transport, related delivery.RelatedEntity) ([]delivery.Status, error) {
	rows, err := e.service.Statuses(ctx, tenantID, domain.Transport(transport), domain.RelatedEntity(related))
	if err != nil {
		return nil, err
	}
	statuses := make([]delivery.Status, 0, len(rows))
	for _, row := range rows {
		var providerResult *delivery.ProviderResult
		if len(row.ProviderResult) > 0 {
			providerResult = &delivery.ProviderResult{}
			if err := json.Unmarshal(row.ProviderResult, providerResult); err != nil {
				return nil, fmt.Errorf("delivery: decode provider result for %s intent %d: %w", transport, row.ID, err)
			}
		}
		statuses = append(statuses, delivery.Status{
			ID: row.ID, IdempotencyKey: row.IdempotencyKey, Transport: transport, State: delivery.State(row.Status), Attempts: row.Attempts,
			LastError: row.LastError, ProviderResult: providerResult, NextAttemptAt: row.NextRetryAt,
			SentAt: row.SentAt, DeadLetterAt: row.DeadLetterAt, CancelledAt: row.CancelledAt,
		})
	}
	return statuses, nil
}

func (e moduleEngine) ReplaceEmailDeliveries(ctx context.Context, tenantID int64, related delivery.RelatedEntity, rows []delivery.EmailDelivery) error {
	values := make([]domain.EmailDelivery, 0, len(rows))
	for _, row := range rows {
		values = append(values, domain.EmailDelivery{
			ID: row.ID, OutboxID: row.OutboxID, GuardianProfileID: row.GuardianProfileID,
			AccountID: row.AccountID, RecipientEmail: row.RecipientEmail, Reachability: row.Reachability,
		})
	}
	return e.store.ReplaceEmailDeliveries(ctx, tenantID, related.Type, related.ID, values)
}

func (e moduleEngine) DeleteEmailDeliveries(ctx context.Context, tenantID int64, related delivery.RelatedEntity) (int64, error) {
	return e.store.DeleteEmailDeliveries(ctx, tenantID, related.Type, related.ID)
}

func (e moduleEngine) EmailDeliveryStatuses(ctx context.Context, tenantID int64, related delivery.RelatedEntity) ([]delivery.EmailDeliveryStatus, error) {
	rows, err := e.service.EmailDeliveryStatuses(ctx, tenantID, domain.RelatedEntity(related))
	if err != nil {
		return nil, err
	}
	statuses := make([]delivery.EmailDeliveryStatus, 0, len(rows))
	for _, row := range rows {
		statuses = append(statuses, delivery.EmailDeliveryStatus{
			DeliveryID: row.DeliveryID, GuardianProfileID: row.GuardianProfileID, AccountID: row.AccountID,
			FirstName: row.FirstName, LastName: row.LastName, RecipientEmail: row.RecipientEmail,
			Reachability: row.Reachability, EmailStatus: row.EmailStatus, LastError: row.LastError,
			SentAt: row.SentAt, Attempts: row.Attempts,
		})
	}
	return statuses, nil
}

func (e moduleEngine) AttachEmailOutbox(ctx context.Context, tenantID, deliveryID, outboxID int64) error {
	return e.store.AttachEmailOutbox(ctx, tenantID, deliveryID, outboxID)
}

func (e moduleEngine) ClaimFailedEmailDelivery(ctx context.Context, tenantID, deliveryID int64) (bool, error) {
	return e.store.ClaimFailedEmailDelivery(ctx, tenantID, deliveryID)
}

type workerEngine struct{ *application.Worker }

func (e workerEngine) RunOnce(ctx context.Context, batchSize, maxAttempts int) (delivery.WorkerStats, error) {
	stats, err := e.Worker.RunOnce(ctx, batchSize, maxAttempts)
	return delivery.WorkerStats(stats), err
}

func (e workerEngine) DispatchEmail(ctx context.Context, tenantID, id int64) error {
	err := e.Worker.DispatchEmail(ctx, tenantID, id)
	if errors.Is(err, domain.ErrCancelled) {
		return fmt.Errorf("%w: %w", delivery.ErrCancelled, err)
	}
	return err
}

type providerAdapter struct{ provider Provider }

func (p providerAdapter) Send(ctx context.Context, intent domain.Intent) (domain.ProviderResult, error) {
	claimed := delivery.ClaimedIntent{
		ID: intent.ID, TenantID: intent.TenantID, Transport: delivery.Transport(intent.Transport),
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
	case delivery.TransportEmail:
		if err := json.Unmarshal(intent.Recipient, &claimed.EmailRecipient); err != nil {
			return domain.ProviderResult{}, fmt.Errorf("delivery provider: decode email recipient: %w", err)
		}
		claimed.EmailPayload = intent.Payload
		result, err := p.provider.SendEmail(ctx, claimed)
		return providerResult(result, err)
	case delivery.TransportPush:
		if err := json.Unmarshal(intent.Recipient, &claimed.PushRecipient); err != nil {
			return domain.ProviderResult{}, fmt.Errorf("delivery provider: decode push recipient: %w", err)
		}
		claimed.DeferredPayload = intent.Payload
		if err := json.Unmarshal(intent.Payload, &claimed.PushPayload); err != nil {
			return domain.ProviderResult{}, fmt.Errorf("delivery provider: decode push payload: %w", err)
		}
		result, err := p.provider.SendPush(ctx, claimed)
		return providerResult(result, err)
	default:
		return domain.ProviderResult{}, fmt.Errorf("delivery provider: unknown transport %q", claimed.Transport)
	}
}

func providerResult(value delivery.ProviderResult, err error) (domain.ProviderResult, error) {
	if errors.Is(err, delivery.ErrCancelled) {
		err = fmt.Errorf("%w: %v", domain.ErrCancelled, err)
	}
	if errors.Is(err, delivery.ErrDeferred) {
		err = fmt.Errorf("%w: %v", domain.ErrDeferred, err)
	}
	return domain.ProviderResult(value), err
}
