package compose

import (
	"github.com/moto-ogs/project-phoenix/modules/mealplan"
	"github.com/moto-ogs/project-phoenix/modules/mealplan/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/modules/mealplan/internal/application"
	"github.com/moto-ogs/project-phoenix/tenant"
)

type PastMealDay = mealplan.PastMealDay

// NewParticipationHistory is the dev CLI's past-registration writer.
func NewParticipationHistory() mealplan.ParticipationHistory {
	store := postgres.New(postgres.NewDatabase(tenant.TransactionFromContext, tenant.TenantFromContext))
	return application.NewParticipationHistory(store, transaction{})
}
