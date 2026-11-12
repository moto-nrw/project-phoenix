package compose

import (
	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// PickupChangeTeam tells the child's team about a changed one-day pickup
// time (#6855) and serves its unread markers. The care request decisions
// call it as their PickupChangeNotifier; the student routes read it.
type PickupChangeTeam interface {
	careplan.PickupChangeNotifier
	careplan.PickupChangeTeamNotices
}

// NewPickupChangeTeam builds the service over the caller's tenant
// transaction. today nil is the school's today; observe nil records nothing.
func NewPickupChangeTeam(reach careplan.PickupChangeTeamReach, directory StudentDirectory, observe func(Observation), today func() calendar.Date) PickupChangeTeam {
	if observe == nil {
		observe = func(Observation) {}
	}
	store := postgres.NewPickupChangeTeamStore(postgres.New(postgres.AmbientCarePlanDatabase), directory)
	return application.NewPickupChangeTeam(store, reach, observe, tenant.NewTransactionRunner().RunInTx, today)
}
