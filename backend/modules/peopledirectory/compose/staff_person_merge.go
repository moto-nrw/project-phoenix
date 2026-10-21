package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/postgres"
)

// NewStaffPersonMerge builds the restricted administrative repair capability.
func NewStaffPersonMerge() peopledirectory.StaffPersonMerge {
	return postgres.New(postgres.NewDatabase())
}
