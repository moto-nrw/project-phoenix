package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/feedback"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type TransactionFunc func(context.Context, func(context.Context) error) error

func (fn TransactionFunc) Run(ctx context.Context, callback func(context.Context) error) error {
	return fn(ctx, callback)
}

type Observation = ports.Observation

type Dependencies struct {
	DB       *bun.DB
	Settings interface {
		FeedbackEnabled(context.Context) (bool, error)
		FeedbackRetentionDays(context.Context) (int, error)
	}
	Today   func() feedback.Date
	Observe func(Observation)
}

func New(dependencies Dependencies) (feedback.Capability, error) {
	if dependencies.DB == nil || dependencies.Settings == nil || dependencies.Today == nil || dependencies.Observe == nil {
		return nil, errors.New("feedback compose: all dependencies are required")
	}
	store := postgres.New(postgres.NewDatabase(tenant.TransactionFromContext, tenant.TenantFromContext))
	today := application.NewCurrentDate(dependencies.Today)
	observe := ports.Observer(dependencies.Observe)
	service := application.New(store, dependencies.Settings, TransactionFunc(tenant.NewTransactionRunner().RunInTx), observe, today)
	return service, nil
}

// Settings are the feedback settings resolvers, bound at construction.
type Settings struct {
	enabled   func(context.Context) (bool, error)
	retention func(context.Context) (int, error)
}

// NewBoundSettings binds the resolvers; a missing one is a construction
// error.
func NewBoundSettings(enabled func(context.Context) (bool, error), retention func(context.Context) (int, error)) (*Settings, error) {
	if enabled == nil || retention == nil {
		return nil, errors.New("feedback settings: both resolvers are required")
	}
	return &Settings{enabled: enabled, retention: retention}, nil
}

func (s *Settings) FeedbackEnabled(ctx context.Context) (bool, error) { return s.enabled(ctx) }

func (s *Settings) FeedbackRetentionDays(ctx context.Context) (int, error) { return s.retention(ctx) }
