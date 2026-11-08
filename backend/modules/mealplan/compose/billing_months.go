package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/application"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// BillingMonthDependencies are what the Mensa-Monatsfreigabe (#4686) reads
// besides its own rows: the kitchen lists of the Meal Plan, the school
// calendar and the attendance the root binds.
type BillingMonthDependencies struct {
	Meals    mealplan.Registration
	Calendar mealplan.CalendarFinder
	Presence mealplan.PresenceFinder
}

// NewBillingMonths builds the Mensa-Monatsfreigabe.
func NewBillingMonths(dependencies BillingMonthDependencies) (mealplan.BillingMonths, error) {
	if dependencies.Meals == nil || dependencies.Calendar == nil || dependencies.Presence == nil {
		return nil, errors.New("meal plan billing months compose: all dependencies are required")
	}
	return application.NewBillingMonths(billingMonthStore(), dependencies.Meals.WeeklyList, dependencies.Calendar, dependencies.Presence,
		tenant.NewTransactionRunner().RunInTx, calendar.TodayDate), nil
}

// NewTraegerMealMonths builds the Träger's read of the released Mensa months
// of its schools (#4686).
func NewTraegerMealMonths() mealplan.TraegerMealMonths {
	return application.NewTraegerMealMonths(billingMonthStore())
}

func billingMonthStore() *postgres.BillingMonthStore {
	return postgres.NewBillingMonthStore(postgres.NewDatabase(tenant.TransactionFromContext, tenant.TenantFromContext), traegerDatabase)
}

// traegerDatabase is the Träger transaction of organizationID, never a
// school or administrative one.
func traegerDatabase(ctx context.Context, organizationID int64) (bun.IDB, error) {
	if organizationID <= 0 || tenant.TraegerTransactionFromContext(ctx) != organizationID {
		return nil, errors.New("meal plan postgres: the Träger transaction of this Träger is required")
	}
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("meal plan postgres: transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, fmt.Errorf("meal plan postgres: unsupported transaction %T", transaction)
	}
	return tx, nil
}
