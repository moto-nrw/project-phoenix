package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/adapters/postgres"
	applicationNative "github.com/moto-nrw/project-phoenix/modules/enrollment/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// New binds Enrollment to the ambient transaction. It never falls back to an
// unscoped pool when a caller forgets to establish its transaction.
func New() enrollment.Owner {
	return NewWithStore(newStore(), applicationNative.ObserveOfferingStorage())
}

// NewWithStore binds owner commands to a supplied store and the tenant runtime.
func NewWithStore(store applicationNative.Store, observe func(enrollment.OfferingStorageObservation)) enrollment.Owner {
	return applicationNative.NewService(store, tenant.NewTransactionRunner(), observe)
}

func newStore() *postgres.Store {
	return postgres.NewStore(postgres.NewDatabase(), applicationNative.NewTenantID())
}
