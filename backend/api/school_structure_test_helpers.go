package api

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewSchoolStructure composes the group owner behind the legacy composition
// seam for test graphs that do not record observations. Production roots
// compose the module themselves (api/base.go) so runtime evidence is kept.
func NewSchoolStructure(db transactionruntime.Database) (schoolstructure.Capability, error) {
	return schoolStructureCompose.New(schoolStructureCompose.Dependencies{
		DB:      db.Pool(),
		Observe: func(schoolStructureCompose.Observation) {},
	})
}
