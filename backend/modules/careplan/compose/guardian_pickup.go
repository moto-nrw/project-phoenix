package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
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
	return postgres.GuardianPickupPermissions{Store: postgres.New(postgres.GuardianPickupDatabase(db)), Observe: observe}, nil
}
