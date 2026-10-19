package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/uptrace/bun"
)

func NewCourseGroupQueries(db *bun.DB, observe func(Observation)) (timetable.CourseGroupQuery, error) {
	if db == nil || observe == nil {
		return nil, errors.New("course group queries: database and observer are required")
	}
	return application.NewCourseGroupQueries(postgres.New(databaseRuntime(db)), transaction{}, observe), nil
}
