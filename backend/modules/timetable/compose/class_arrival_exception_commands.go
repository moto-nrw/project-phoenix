package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/uptrace/bun"
)

func NewClassArrivals(db *bun.DB, observe func(Observation)) (timetable.ClassArrivals, error) {
	if db == nil || observe == nil {
		return nil, errors.New("class arrival exceptions: database and observer are required")
	}
	return application.NewClassArrivals(postgres.New(databaseRuntime(db)), transaction{}, observe), nil
}
