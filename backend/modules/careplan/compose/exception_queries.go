package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	careplanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	"github.com/uptrace/bun"
)

// NewExceptionQueries serves the exception reads over the database.
func NewExceptionQueries(db *bun.DB, observe func(Observation)) (careplanPorts.ExceptionQueries, error) {
	if db == nil || observe == nil {
		return nil, errors.New("care plan exception queries: database and observer are required")
	}
	return application.NewExceptionQueryService(postgres.New(postgres.CarePlanDatabase(db)), observe), nil
}
