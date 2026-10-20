package api

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewGradeTransitionOwners builds the School Structure and Student Presence
// owners the grade transition workflow writes through, over the shared
// database and without observation, as the workflow composition did before
// (#4004, ADR 0059 point 6).
func NewGradeTransitionOwners(db transactionruntime.Database) (schoolstructure.Owner, studentpresence.Owner, error) {
	structure, err := schoolStructureCompose.New(schoolStructureCompose.Dependencies{DB: db.Pool(), Observe: func(schoolStructureCompose.Observation) {}})
	if err != nil {
		return nil, nil, err
	}
	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: db.Pool(), Observe: func(presenceCompose.Observation) {}})
	if err != nil {
		return nil, nil, err
	}
	return structure, presence, nil
}
