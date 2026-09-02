package repositories

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/uptrace/bun"
)

// NewSchoolStructure composes the group owner behind the legacy composition
// seam for test graphs that do not record observations. Production roots
// compose the module themselves (api/base.go) so runtime evidence is kept.
func NewSchoolStructure(db *bun.DB) (schoolstructure.Capability, error) {
	return schoolStructureCompose.New(schoolStructureCompose.Dependencies{
		DB:      db,
		Observe: func(schoolStructureCompose.Observation) {},
	})
}
