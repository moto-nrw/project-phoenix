package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/feedback"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/domain"
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

func New(dependencies Dependencies) (*feedback.Module, error) {
	if dependencies.DB == nil || dependencies.Settings == nil || dependencies.Today == nil || dependencies.Observe == nil {
		return nil, errors.New("feedback compose: all dependencies are required")
	}
	store := postgres.New(func(ctx context.Context) (bun.IDB, int64, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, 0, errors.New("feedback postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, 0, fmt.Errorf("feedback postgres: unsupported transaction %T", transaction)
		}
		tenantID, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return nil, 0, err
		}
		return tx, tenantID.Int64(), nil
	})
	today := func() domain.Date {
		date, err := domain.ParseDate(string(dependencies.Today()))
		if err != nil {
			panic(fmt.Sprintf("feedback compose: invalid current date: %v", err))
		}
		return date
	}
	observe := func(observation ports.Observation) {
		observation.Err = application.MapPublicError(observation.Err, 0)
		dependencies.Observe(observation)
	}
	service := application.New(store, dependencies.Settings, TransactionFunc(tenant.NewTransactionRunner().RunInTx), observe, today)
	return application.NewPublicModule(service, dependencies.Observe), nil
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
