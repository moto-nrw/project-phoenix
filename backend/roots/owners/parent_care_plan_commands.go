package owners

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplancompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func NewParentCarePlanCommands(source careplan.Capability, excusal careplan.PickupAutoExcusal) parentportal.CarePlanCommands {
	if source == nil {
		return nil
	}
	absences, err := careplancompose.NewGuardianAbsences(source)
	if err != nil {
		panic(err)
	}
	pickups, err := careplancompose.NewGuardianPickupExceptions(source, excusal)
	if err != nil {
		panic(err)
	}
	return parentportal.NewCarePlanCommands(NewParentCareExceptions(source), NewParentGuardianAbsenceReports(absences), NewParentGuardianPickupExceptions(pickups))
}
