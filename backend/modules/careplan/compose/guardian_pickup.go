package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewGuardianPickupPermissions composes the pickup-permission commands (#2756)
// from the database alone, like the companion slice: the relationship's unit
// of work in People Directory needs them before the full Care Plan exists.
// Every command joins the caller's tenant or administrative transaction.
func NewGuardianPickupPermissions(db *bun.DB, observe func(Observation)) (careplan.GuardianPickupPermissions, error) {
	if db == nil || observe == nil {
		return nil, errors.New("care plan guardian pickup permissions: database and observer are required")
	}
	return guardianPickupPermissions{store: postgres.New(guardianPickupDatabase(db)), observe: observe}, nil
}

// guardianPickupDatabase resolves the ambient transaction without requiring a
// tenant in context: the commands name their school, and the store holds a
// tenant transaction to that school.
func guardianPickupDatabase(db *bun.DB) postgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		database, err := tenant.CurrentTransaction[bun.IDB](ctx)
		return database, tenant.FromContext(ctx), err
	}
}

type guardianPickupPermissions struct {
	store   *postgres.Store
	observe func(Observation)
}

func (p guardianPickupPermissions) CreateGuardianPickupPermission(ctx context.Context, permission careplan.GuardianPickupPermission) error {
	started := time.Now()
	stats, err := p.store.CreateGuardianPickupPermission(ctx, permission)
	p.observe(Observation{Operation: "create_guardian_pickup_permission", Duration: time.Since(started), Stats: stats, Err: err})
	return err
}

func (p guardianPickupPermissions) ChangeGuardianPickupPermission(ctx context.Context, change careplan.GuardianPickupPermissionChange) (bool, error) {
	started := time.Now()
	found, stats, err := p.store.ChangeGuardianPickupPermission(ctx, change)
	p.observe(Observation{Operation: "change_guardian_pickup_permission", Duration: time.Since(started), Stats: stats, Err: err})
	return found, err
}
