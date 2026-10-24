package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/uptrace/bun"
)

func NewClassArrivalQueries(db *bun.DB, observe func(Observation)) (timetable.ClassArrivalQuery, error) {
	if db == nil || observe == nil {
		return nil, errors.New("class arrival queries: database and observer are required")
	}
	return application.NewClassArrivalQueries(postgres.New(postgres.DatabaseRuntime(db)), transaction{}, observe), nil
}
