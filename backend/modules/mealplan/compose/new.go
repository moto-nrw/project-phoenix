package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type TransactionFunc func(context.Context, func(context.Context) error) error

func (fn TransactionFunc) Run(ctx context.Context, callback func(context.Context) error) error {
	return fn(ctx, callback)
}

type Observation = ports.Observation

// OrderingSettings are the school's meal ordering settings as stored.
type OrderingSettings = ports.OrderingSettings

type Dependencies struct {
	DB       *bun.DB
	Settings interface {
		MealPlanEnabled(context.Context) (bool, error)
		MealRegistrationEnabled(context.Context) (bool, error)
		MealOrderingSettings(context.Context) (OrderingSettings, error)
	}
	Observe      func(Observation)
	Now          func() time.Time
	Participants mealplan.ParticipantFinder
	StudentNames mealplan.StudentNameFinder
	Calendar     mealplan.CalendarFinder
}

func New(dependencies Dependencies) (*mealplan.Module, error) {
	if dependencies.DB == nil || dependencies.Settings == nil || dependencies.Observe == nil || dependencies.Now == nil || dependencies.Participants == nil || dependencies.StudentNames == nil || dependencies.Calendar == nil {
		return nil, errors.New("meal plan compose: all dependencies are required")
	}
	store := postgres.New(func(ctx context.Context) (bun.IDB, int64, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, 0, errors.New("meal plan postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, 0, fmt.Errorf("meal plan postgres: unsupported transaction %T", transaction)
		}
		tenantID, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return nil, 0, err
		}
		return tx, tenantID.Int64(), nil
	})
	service := application.New(store, application.NewParticipantDirectory(dependencies.Participants, dependencies.StudentNames), application.NewSchoolCalendar(dependencies.Calendar), dependencies.Settings, TransactionFunc(tenant.NewTransactionRunner().RunInTx), dependencies.Observe, dependencies.Now)
	return application.NewPublicModule(service), nil
}

// Settings are the meal plan's settings resolvers, bound at construction.
type Settings struct {
	mealPlanEnabled         func(context.Context) (bool, error)
	mealRegistrationEnabled func(context.Context) (bool, error)
	orderingSettings        func(context.Context) (OrderingSettings, error)
}

// NewBoundSettings binds the resolvers; a missing one is a construction
// error.
func NewBoundSettings(mealPlanEnabled, mealRegistrationEnabled func(context.Context) (bool, error), orderingSettings func(context.Context) (OrderingSettings, error)) (*Settings, error) {
	if mealPlanEnabled == nil || mealRegistrationEnabled == nil || orderingSettings == nil {
		return nil, errors.New("meal plan settings: resolvers are required")
	}
	return &Settings{mealPlanEnabled: mealPlanEnabled, mealRegistrationEnabled: mealRegistrationEnabled, orderingSettings: orderingSettings}, nil
}

func (s *Settings) MealPlanEnabled(ctx context.Context) (bool, error) { return s.mealPlanEnabled(ctx) }

func (s *Settings) MealRegistrationEnabled(ctx context.Context) (bool, error) {
	return s.mealRegistrationEnabled(ctx)
}

func (s *Settings) MealOrderingSettings(ctx context.Context) (OrderingSettings, error) {
	return s.orderingSettings(ctx)
}
