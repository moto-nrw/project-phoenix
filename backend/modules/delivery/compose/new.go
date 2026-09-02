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

type Dependencies struct {
	DB       *bun.DB
	Provider Provider
	People   GuardianDisplayResolver
	Observe  func(delivery.Observation)
}

type Runtime struct {
	Module *delivery.Module
	Worker *delivery.Worker
}

func New(dependencies Dependencies) (*Runtime, error) {
	if dependencies.DB == nil || dependencies.Provider == nil || dependencies.People == nil || dependencies.Observe == nil {
		return nil, errors.New("delivery compose: all dependencies are required")
	}
	store := postgres.New(
		func(ctx context.Context, expectedTenantID int64) (bun.IDB, error) {
			transaction, ok := tenant.TransactionFromContext(ctx)
			if !ok {
				return nil, errors.New("delivery postgres: transaction is required")
			}
			tx, ok := transaction.(bun.Tx)
			if !ok {
				return nil, fmt.Errorf("delivery postgres: unsupported transaction %T", transaction)
			}
			actualTenantID, err := tenant.TenantFromContext(ctx)
			if err != nil {
				return nil, fmt.Errorf("delivery postgres: tenant is required: %w", err)
			}
			if actualTenantID.Int64() != expectedTenantID {
				return nil, fmt.Errorf("delivery postgres: intent tenant %d does not match transaction tenant %d", expectedTenantID, actualTenantID.Int64())
			}
			return tx, nil
		},
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
	worker := application.NewWorker(store, providerAdapter{provider: dependencies.Provider}, observe)
	return &Runtime{
		Module: delivery.NewModule(moduleEngine{service: service}),
		Worker: delivery.NewWorker(workerEngine{worker: worker}),
	}, nil
}

type guardianDirectoryAdapter struct{ resolver GuardianDisplayResolver }

func (a guardianDirectoryAdapter) Resolve(ctx context.Context, ids []int64) (map[int64]domain.GuardianDisplay, error) {
	rows, err := a.resolver.ResolveGuardianDisplays(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]domain.GuardianDisplay, len(rows))
	for _, row := range rows {
		result[row.GuardianProfileID] = domain.GuardianDisplay{
			GuardianProfileID: row.GuardianProfileID, FirstName: row.FirstName, LastName: row.LastName,
		}
	}
	return result, nil
}

type moduleEngine struct{ service *application.Service }

func (e moduleEngine) EnqueueEmail(ctx context.Context, input delivery.EmailIntent) (delivery.Enqueued, error) {
	recipient, err := json.Marshal(input.Recipient)
	if err != nil {
		return delivery.Enqueued{}, fmt.Errorf("delivery: marshal email recipient: %w", err)
	}
	return e.enqueue(ctx, domain.EnqueueInput{
		TenantID: input.TenantID, Transport: domain.TransportEmail, Template: input.Template,
		IdempotencyKey: input.IdempotencyKey, Related: toDomainRelated(input.Related),
		Recipient: recipient, Payload: input.Payload,
	}, delivery.TransportEmail)
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
		IdempotencyKey: input.IdempotencyKey, Related: toDomainRelated(input.Related),
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
	return e.service.Cancel(ctx, tenantID, domain.Transport(transport), toDomainRelated(related), reason)
}

func (e moduleEngine) Statuses(ctx context.Context, tenantID int64, transport delivery.Transport, related delivery.RelatedEntity) ([]delivery.Status, error) {
	rows, err := e.service.Statuses(ctx, tenantID, domain.Transport(transport), toDomainRelated(related))
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
			ID: row.ID, Transport: transport, State: delivery.State(row.Status), Attempts: row.Attempts,
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
	return e.service.ReplaceEmailDeliveries(ctx, tenantID, toDomainRelated(related), values)
}

func (e moduleEngine) DeleteEmailDeliveries(ctx context.Context, tenantID int64, related delivery.RelatedEntity) (int64, error) {
	return e.service.DeleteEmailDeliveries(ctx, tenantID, toDomainRelated(related))
}

func (e moduleEngine) EmailDeliveryStatuses(ctx context.Context, tenantID int64, related delivery.RelatedEntity) ([]delivery.EmailDeliveryStatus, error) {
	rows, err := e.service.EmailDeliveryStatuses(ctx, tenantID, toDomainRelated(related))
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
	return e.service.AttachEmailOutbox(ctx, tenantID, deliveryID, outboxID)
}

func (e moduleEngine) ClaimFailedEmailDelivery(ctx context.Context, tenantID, deliveryID int64) (bool, error) {
	return e.service.ClaimFailedEmailDelivery(ctx, tenantID, deliveryID)
}

func toDomainRelated(value delivery.RelatedEntity) domain.RelatedEntity {
	return domain.RelatedEntity{Type: value.Type, ID: value.ID}
}

type workerEngine struct{ worker *application.Worker }

func (e workerEngine) RunOnce(ctx context.Context, batchSize int) (delivery.WorkerStats, error) {
	stats, err := e.worker.RunOnce(ctx, batchSize)
	return delivery.WorkerStats{
		Claimed: stats.Claimed, Sent: stats.Sent, Cancelled: stats.Cancelled, Retried: stats.Retried,
		DeadLettered: stats.DeadLettered, LeaseLost: stats.LeaseLost,
	}, err
}

func (e workerEngine) Backlog(ctx context.Context) (int, error) { return e.worker.Backlog(ctx) }
func (e workerEngine) SetMaxAttempts(attempts int)              { e.worker.SetMaxAttempts(attempts) }

type providerAdapter struct{ provider Provider }

func (p providerAdapter) Send(ctx context.Context, intent domain.Intent) (domain.ProviderResult, error) {
	claimed := delivery.ClaimedIntent{
		ID: intent.ID, TenantID: intent.TenantID, Transport: delivery.Transport(intent.Transport),
		Template: intent.Template, Attempts: intent.Attempts,
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
	return toDomainProviderResult(value), err
}

func toDomainProviderResult(value delivery.ProviderResult) domain.ProviderResult {
	return domain.ProviderResult{MessageID: value.MessageID, StatusCode: value.StatusCode, Details: value.Details}
}
