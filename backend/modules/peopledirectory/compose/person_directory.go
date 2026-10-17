package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// NewPersonDirectory composes the retained person directory over the
// collaborators in deps (#3752).
func NewPersonDirectory(deps peoplePorts.PersonDirectoryDependencies) peoplePorts.PersonDirectory {
	return application.NewRetainedPersonDirectory(deps)
}

// NewStaffDirectory binds the staff directory over the retained
// repositories.
func NewStaffDirectory(deps peoplePorts.StaffDirectoryDependencies) peoplePorts.StaffDirectory {
	return application.NewStaffDirectory(deps)
}
