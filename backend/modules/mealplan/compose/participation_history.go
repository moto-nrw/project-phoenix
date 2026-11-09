package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewParticipationHistory is the dev CLI's past-registration writer.
func NewParticipationHistory() mealplan.ParticipationHistory {
	store := postgres.New(postgres.NewDatabase(tenant.TransactionFromContext, tenant.TenantFromContext))
	return application.NewParticipationHistory(store, TransactionFunc(tenant.NewTransactionRunner().RunInTx))
}
