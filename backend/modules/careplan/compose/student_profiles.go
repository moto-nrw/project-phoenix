package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/uptrace/bun"
)

func NewStudentProfiles(db *bun.DB, directory StudentDirectory, observe func(Observation)) (careplan.StudentProfileCommands, error) {
	if db == nil || directory == nil || observe == nil {
		return nil, errors.New("care plan: database, student directory and observer are required")
	}
	return application.NewStudentProfiles(postgres.NewDirectoryStore(postgres.New(postgres.CarePlanDatabase(db)), directory), observe), nil
}
