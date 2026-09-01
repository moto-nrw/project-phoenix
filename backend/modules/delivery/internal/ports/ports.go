package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
)

type Store interface {
	Enqueue(context.Context, domain.Intent) (domain.Enqueued, error)
	Claim(context.Context, domain.Transport, int, time.Time, time.Time) ([]domain.Intent, error)
	FinalizeSent(context.Context, domain.Transport, int64, string, json.RawMessage, time.Time) (bool, error)
	FinalizeCancelled(context.Context, domain.Transport, int64, string, string, time.Time) (bool, error)
	FinalizeFailure(context.Context, domain.Transport, int64, string, int, string, time.Time, int) (domain.FinalizeResult, error)
	Cancel(context.Context, int64, domain.Transport, string, int64, string, time.Time) (int64, error)
	Statuses(context.Context, int64, domain.Transport, string, int64) ([]domain.Intent, error)
	Backlog(context.Context) (int, error)
	OldestPendingAge(context.Context, time.Time) (time.Duration, error)
	ReplaceEmailDeliveries(context.Context, int64, string, int64, []domain.EmailDelivery) error
	DeleteEmailDeliveries(context.Context, int64, string, int64) (int64, error)
	EmailDeliveryStatuses(context.Context, int64, string, int64) ([]domain.EmailDeliveryStatus, error)
	AttachEmailOutbox(context.Context, int64, int64, int64) error
	ClaimFailedEmailDelivery(context.Context, int64, int64) (bool, error)
}

type Provider interface {
	Send(context.Context, domain.Intent) (domain.ProviderResult, error)
}

type GuardianDirectory interface {
	Resolve(context.Context, []int64) (map[int64]domain.GuardianDisplay, error)
}

type Observer func(domain.Observation)
