//go:build testhooks

package owners

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewSchoolStructure composes the group owner for test graphs that do not record observations. Production roots
// compose the module themselves (api/base.go) so runtime evidence is kept.
// Its only consumer is a Data Import suite, which may not import School
// Structure's composition and builds the owner through the root (ADR 0053
// point 7.2, #4005).
func NewSchoolStructure(db transactionruntime.Database) (schoolstructure.Capability, error) {
	return schoolStructureCompose.New(schoolStructureCompose.Dependencies{
		DB:      db.Pool(),
		Observe: func(schoolStructureCompose.Observation) {},
	})
}
