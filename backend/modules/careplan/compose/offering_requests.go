package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/uptrace/bun"
)

// NewOfferingChangeRequestQueries binds the owner read without mutation dependencies.
func NewOfferingChangeRequestQueries(db *bun.DB, observe func(Observation)) (careplan.OfferingChangeRequestQuery, error) {
	if db == nil || observe == nil {
		return nil, errors.New("care plan offering request queries: database and observer are required")
	}
	return application.New(postgres.New(postgres.CarePlanDatabase(db)), observe), nil
}
