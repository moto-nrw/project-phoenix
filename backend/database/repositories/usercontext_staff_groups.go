package repositories

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// NewUserContextStaffGroups composes the School Structure staff group reads
// (#3499) the caller context resolves the caller's groups, substitutions and
// school classes through.
func NewUserContextStaffGroups(groups schoolstructure.Query, membership schoolmembership.Capability, workTime workforce.Capability) (schoolstructure.StaffGroupQuery, error) {
	return schoolStructureCompose.NewStaffGroups(schoolStructureCompose.StaffGroupsDependencies{
		Groups: groups, Assignments: membership, Substitutions: workTime,
	})
}
