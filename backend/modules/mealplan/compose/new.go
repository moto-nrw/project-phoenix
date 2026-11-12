package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/mealplan"
	"github.com/moto-ogs/project-phoenix/modules/mealplan/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/modules/mealplan/internal/application"
	"github.com/moto-ogs/project-phoenix/modules/mealplan/internal/ports"
	"github.com/moto-ogs/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type transaction struct{}

func (transaction) Run(ctx context.Context, callback func(context.Context) error) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, callback)
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
	Observe         func(Observation)
	Now             func() time.Time
	Participants    mealplan.ParticipantFinder
	ParticipantDays mealplan.ParticipantDaysFinder
	StudentNames    mealplan.StudentNameFinder
	Calendar        mealplan.CalendarFinder
}

func New(dependencies Dependencies) (mealplan.Capability, error) {
	if dependencies.DB == nil || dependencies.Settings == nil || dependencies.Observe == nil || dependencies.Now == nil || dependencies.Participants == nil || dependencies.ParticipantDays == nil || dependencies.StudentNames == nil || dependencies.Calendar == nil {
		return nil, errors.New("meal plan compose: all dependencies are required")
	}
	store := postgres.New(postgres.NewDatabase(tenant.TransactionFromContext, tenant.TenantFromContext))
	return application.New(store, application.NewParticipantDirectory(dependencies.Participants, dependencies.StudentNames, dependencies.ParticipantDays), application.NewSchoolCalendar(dependencies.Calendar), dependencies.Settings, transaction{}, dependencies.Observe, dependencies.Now), nil
}

// Settings are the application-owned bound resolvers.
type Settings = application.Settings

func NewBoundSettings(mealPlanEnabled, mealRegistrationEnabled func(context.Context) (bool, error), orderingSettings func(context.Context) (OrderingSettings, error)) (*Settings, error) {
	return application.NewBoundSettings(mealPlanEnabled, mealRegistrationEnabled, orderingSettings)
}
