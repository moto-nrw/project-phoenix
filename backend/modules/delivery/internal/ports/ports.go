package ports

import (
	"context"
	"encoding/json"
	"time"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
)

type Store interface {
	Enqueue(context.Context, domain.Intent) (domain.Enqueued, error)
	Claim(context.Context, domain.Transport, int, time.Time, time.Time) ([]domain.Intent, error)
	// StartDelivery atomically advances a claimed intent to its durable dispatch
	// fence. A cancellation that commits first prevents the provider call; once
	// the fence is committed, the provider call has logically begun.
	StartDelivery(context.Context, domain.Transport, int64, string, time.Time) (string, bool, error)
	// DeadLetterExpiredDispatches ends dispatch fences that expired without
	// finalization. Their provider outcome is unknown, so they are never retried.
	DeadLetterExpiredDispatches(context.Context, domain.Transport, time.Time) (int64, error)
	FinalizeSent(context.Context, domain.Transport, int64, string, json.RawMessage, time.Time) (bool, error)
	FinalizeCancelled(context.Context, domain.Transport, int64, string, string, time.Time) (bool, error)
	FinalizeDeferred(context.Context, domain.Transport, int64, string, string, time.Time) (bool, error)
	FinalizeFailure(context.Context, domain.Transport, int64, string, int, string, time.Time, int) (domain.FinalizeResult, error)
	Cancel(context.Context, int64, domain.Transport, string, int64, string, time.Time) (int64, error)
	Statuses(context.Context, int64, domain.Transport, string, int64) ([]domain.Intent, error)
	EmailStatus(context.Context, int64, int64) (domain.Intent, bool, error)
	Backlog(context.Context) (int, error)
	OldestPendingAge(context.Context, time.Time) (time.Duration, error)
	ReplaceEmailDeliveries(context.Context, int64, string, int64, []domain.EmailDelivery) error
	DeleteEmailDeliveries(context.Context, int64, string, int64) (int64, error)
	EmailDeliveryStatuses(context.Context, int64, string, int64) ([]deliveryContract.EmailDeliveryStatus, error)
	AttachEmailOutbox(context.Context, int64, int64, int64) error
	ClaimFailedEmailDelivery(context.Context, int64, int64) (bool, error)
}

type Provider interface {
	Send(context.Context, domain.Intent) (deliveryContract.ProviderResult, error)
}

type GuardianDirectory interface {
	ResolveGuardianDisplays(context.Context, []int64) ([]deliveryContract.GuardianDisplay, error)
}

type Observer func(domain.Observation)

// RetentionStore deletes the delivery data of one school whose retention
// window ended before the instant (#3955). Every statement runs on the
// school's tenant transaction.
type RetentionStore interface {
	// DeleteExpiredEmailDeliveryLogs removes the delivery logs of the
	// expired mails and the logs without a mail written before the instant.
	DeleteExpiredEmailDeliveryLogs(ctx context.Context, tenantID int64, before time.Time) (int, error)
	// DeleteExpiredEmailIntents removes the expired mails except the
	// key-only templates.
	DeleteExpiredEmailIntents(ctx context.Context, tenantID int64, before time.Time, keyOnlyTemplates []string) (int, error)
	// ClearExpiredKeyOnlyEmailIntents empties relation, recipient, content and
	// provider answer of the expired key-only mails not cleared yet.
	ClearExpiredKeyOnlyEmailIntents(ctx context.Context, tenantID int64, before time.Time, keyOnlyTemplates []string) (int, error)
	DeleteExpiredPushIntents(ctx context.Context, tenantID int64, before time.Time) (int, error)
}
