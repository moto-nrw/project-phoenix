package owners

import (
	"errors"

	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// PickupPlanStudents binds the Care Plan pickup schedules' bulk students to
// the People Directory. The caller hands them with its database to
// careplanCompose.NewPickupSchedules.
func PickupPlanStudents(people peopledirectory.Capability) (careplanCompose.PickupBulkStudents, error) {
	if people == nil {
		return nil, errors.New("pickup schedules: people directory is required")
	}
	return pickupPlanStudents{people}, nil
}
