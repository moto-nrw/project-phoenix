package api

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// CareScheduleRepositories are the retained schedule repositories served by
// the Care Plan owner.
type CareScheduleRepositories = timetableCompose.CareScheduleRepositories

// NewCareScheduleRepositories builds the retained schedule repositories over
// the Care Plan capability.
func NewCareScheduleRepositories(capability careplan.Capability) CareScheduleRepositories {
	return timetableCompose.NewCareScheduleRepositories(NewCarePlanRows(capability))
}

func NewPickupNoteRepository(capability careplan.Capability) timetablePorts.CarePickupNoteRows {
	return timetableCompose.NewCarePickupNoteRows(NewCarePlanRows(capability))
}
