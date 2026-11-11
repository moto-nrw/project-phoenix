package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

type GuardianPickupPermissions struct {
	Store   *Store
	Observe func(ports.Observation)
}

func (p GuardianPickupPermissions) CreateGuardianPickupPermission(ctx context.Context, permission careplan.GuardianPickupPermission) error {
	started := time.Now()
	stats, err := p.Store.CreateGuardianPickupPermission(ctx, permission)
	p.Observe(ports.Observation{Operation: "create_guardian_pickup_permission", Duration: time.Since(started), Stats: stats, Err: err})
	return err
}
func (p GuardianPickupPermissions) ChangeGuardianPickupPermission(ctx context.Context, change careplan.GuardianPickupPermissionChange) (bool, error) {
	started := time.Now()
	found, stats, err := p.Store.ChangeGuardianPickupPermission(ctx, change)
	p.Observe(ports.Observation{Operation: "change_guardian_pickup_permission", Duration: time.Since(started), Stats: stats, Err: err})
	return found, err
}

// The data import's batch slice of the pickup permission (#5552).
var _ careplan.GuardianPickupPermissionBatches = GuardianPickupPermissions{}

func (p GuardianPickupPermissions) CreateGuardianPickupPermissions(ctx context.Context, permissions []careplan.GuardianPickupPermission) error {
	started := time.Now()
	stats, err := p.Store.CreateGuardianPickupPermissions(ctx, permissions)
	p.Observe(ports.Observation{Operation: "create_guardian_pickup_permissions", Duration: time.Since(started), Stats: stats, Err: err})
	return err
}
