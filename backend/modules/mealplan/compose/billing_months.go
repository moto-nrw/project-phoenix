package compose

import (
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/mealplan"
	"github.com/moto-ogs/project-phoenix/modules/mealplan/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/modules/mealplan/internal/application"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// NewBillingMonths builds the Mensa-Monatsfreigabe (#4686) over the kitchen
// lists of the Meal Plan, the school calendar and the attendance the root
// binds.
func NewBillingMonths(meals mealplan.Registration, days mealplan.CalendarFinder, presence mealplan.PresenceFinder, availableFrom mealplan.AttendanceAvailableFrom) (mealplan.BillingMonths, error) {
	if meals == nil || days == nil || presence == nil || availableFrom == nil {
		return nil, errors.New("meal plan billing months compose: all dependencies are required")
	}
	return application.NewBillingMonths(billingMonthStore(), meals.WeeklyList, days, presence, availableFrom, calendar.TodayDate), nil
}

// NewTraegerMealMonths builds the Träger's read of the released Mensa months.
func NewTraegerMealMonths() mealplan.TraegerMealMonths {
	return application.NewTraegerMealMonths(billingMonthStore())
}

func billingMonthStore() *postgres.BillingMonthStore {
	return postgres.NewBillingMonthStore(postgres.NewDatabase(tenant.TransactionFromContext, tenant.TenantFromContext),
		postgres.NewTraegerDatabase(tenant.TransactionFromContext, tenant.TraegerTransactionFromContext))
}
