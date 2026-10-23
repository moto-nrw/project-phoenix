package compose

import (
	"context"
	"errors"
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

type Dependencies struct {
	DB           *bun.DB
	Settings     ports.Settings
	Calendar     ports.Calendar
	Observe      func(Observation)
	Now          func() time.Time
	Participants mealplan.ParticipantFinder
}

func New(dependencies Dependencies) (mealplan.Capability, error) {
	if dependencies.DB == nil || dependencies.Settings == nil || dependencies.Calendar == nil || dependencies.Observe == nil || dependencies.Now == nil || dependencies.Participants == nil {
		return nil, errors.New("meal plan compose: all dependencies are required")
	}
	store := postgres.New(postgres.NewDatabase(tenant.TransactionFromContext, tenant.TenantFromContext))
	service := application.New(store, application.NewParticipantDirectory(dependencies.Participants), dependencies.Settings, dependencies.Calendar, TransactionFunc(tenant.NewTransactionRunner().RunInTx), dependencies.Observe, dependencies.Now)
	return service, nil
}

// Settings are the meal plan's settings resolvers, bound at construction.
type Settings struct {
	mealPlanEnabled         func(context.Context) (bool, error)
	mealRegistrationEnabled func(context.Context) (bool, error)
	mealRegistrationCutoff  func(context.Context) (mealplan.Cutoffs, error)
}

// NewBoundSettings binds the resolvers; a missing one is a construction
// error.
func NewBoundSettings(mealPlanEnabled, mealRegistrationEnabled func(context.Context) (bool, error), mealRegistrationCutoff func(context.Context) (mealplan.Cutoffs, error)) (*Settings, error) {
	if mealPlanEnabled == nil || mealRegistrationEnabled == nil || mealRegistrationCutoff == nil {
		return nil, errors.New("meal plan settings: resolvers are required")
	}
	return &Settings{mealPlanEnabled: mealPlanEnabled, mealRegistrationEnabled: mealRegistrationEnabled, mealRegistrationCutoff: mealRegistrationCutoff}, nil
}

func (s *Settings) MealPlanEnabled(ctx context.Context) (bool, error) { return s.mealPlanEnabled(ctx) }

func (s *Settings) MealRegistrationEnabled(ctx context.Context) (bool, error) {
	return s.mealRegistrationEnabled(ctx)
}

func (s *Settings) MealParticipationCutoffs(ctx context.Context) (mealplan.Cutoffs, error) {
	return s.mealRegistrationCutoff(ctx)
}
