package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

// RoomUsage is the room deletion's question to Student Presence (running
// sessions) and Care Plan (care offerings).
type RoomUsage = ports.RoomUsage

type Dependencies struct {
	DB           *bun.DB
	DeletionLock func(context.Context) error
	// Usage decides whether a room may be deleted.
	Usage   RoomUsage
	Observe func(Observation)
}

// NewRooms composes the Facilities Catalog without the room deletion over
// DB and Observe alone, for the owners a root builds before the deletion
// guard's (#4080); New adds it.
func NewRooms(dependencies Dependencies) (facilities.Catalog, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("facilities compose: all dependencies are required")
	}
	observe := ports.Observer(dependencies.Observe)
	service := application.NewRooms(postgres.New(postgres.DatabaseRuntime(dependencies.DB)), transaction{}, observe)
	return service, nil
}

// New composes the Facilities module. Every operation requires the caller's
// tenant, which the store also applies as a defense-in-depth predicate.
func New(dependencies Dependencies) (facilities.Capability, error) {
	if dependencies.DB == nil || dependencies.DeletionLock == nil || dependencies.Usage == nil || dependencies.Observe == nil {
		return nil, errors.New("facilities compose: all dependencies are required")
	}
	store := postgres.New(postgres.DatabaseRuntime(dependencies.DB))
	observe := ports.Observer(dependencies.Observe)
	service := application.New(store, transaction{}, dependencies.DeletionLock, dependencies.Usage, observe)
	return service, nil
}

type transaction struct{}

func (transaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	return postgres.TransactionRunWriteBinding(ctx, callback, tenant.WithinCurrentTenant)
}

func (transaction) AcquireLock(ctx context.Context, key string) error {
	return tenant.AcquireLock(ctx, key, false)
}
