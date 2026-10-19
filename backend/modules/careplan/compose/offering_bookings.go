package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	applicationBindings "github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewOfferingBookings binds the owner to the ambient UnitOfWork, never to an
// unscoped pool. It can therefore participate in atomic Enrollment workflows.
func NewOfferingBookings() *careplan.OfferingBookings {
	return newOfferingBookings(applicationBindings.ObserveOfferingBookings())
}

func newOfferingBookings(observe func(careplan.OfferingBookingObservation)) *careplan.OfferingBookings {
	store := postgres.New(postgres.NewOfferingBookingsDatabase())
	return careplan.NewOfferingBookings(store, tenant.NewTransactionRunner(), observe)
}
