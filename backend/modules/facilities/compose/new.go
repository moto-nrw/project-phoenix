package compose

import (
	"context"
	"errors"
	"fmt"

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
func NewRooms(dependencies Dependencies) (*facilities.Rooms, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("facilities compose: all dependencies are required")
	}
	observe := observer(dependencies.Observe)
	service := application.NewRooms(postgres.New(databaseRuntime(dependencies.DB)), transaction{}, observe)
	return facilities.NewRooms(engine{Engine: application.NewEngine(service, observe)}), nil
}

// New composes the Facilities module. Every operation requires the caller's
// tenant, which the store also applies as a defense-in-depth predicate.
func New(dependencies Dependencies) (*facilities.Module, error) {
	if dependencies.DB == nil || dependencies.DeletionLock == nil || dependencies.Usage == nil || dependencies.Observe == nil {
		return nil, errors.New("facilities compose: all dependencies are required")
	}
	store := postgres.New(databaseRuntime(dependencies.DB))
	observe := observer(dependencies.Observe)
	service := application.New(store, transaction{}, dependencies.DeletionLock, dependencies.Usage, observe)
	return facilities.NewModule(engine{Engine: application.NewEngine(service, observe)}), nil
}

// observer reports every observation with the public error.
func observer(observe func(Observation)) ports.Observer {
	return func(observation Observation) {
		observation.Err = application.MapError(observation.Err)
		observe(observation)
	}
}

func databaseRuntime(db *bun.DB) postgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		tenantID, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("facilities postgres: tenant is required: %w", err)
		}
		transaction, hasTransaction := tenant.TransactionFromContext(ctx)
		if !hasTransaction {
			return db, tenantID.Int64(), nil
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, tenantID.Int64(), nil
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID.Int64(), nil
			}
			return db, tenantID.Int64(), nil
		default:
			return nil, 0, fmt.Errorf("facilities postgres: unsupported transaction %T", transaction)
		}
	}
}

type engine struct{ application.Engine }

func (engine) RequireTenant(ctx context.Context) error {
	_, err := tenant.TenantFromContext(ctx)
	return err
}

type transaction struct{}

func (transaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	return tenant.WithinCurrentTenant(ctx, callback)
}

func (transaction) AcquireLock(ctx context.Context, key string) error {
	return tenant.AcquireLock(ctx, key, false)
}
