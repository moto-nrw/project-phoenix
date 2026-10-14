package api

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// StudentScheduleRepositories contains the Care Plan compatibility adapters
// still consumed by legacy schedule services.
type StudentScheduleRepositories struct {
	ArrivalSchedule  timetableCompose.CareArrivalScheduleRows
	ArrivalException timetableCompose.CareArrivalExceptionRows
	ArrivalNote      timetableCompose.CareArrivalNoteRows
	PickupSchedule   timetableCompose.CarePickupScheduleRows
	PickupException  timetableCompose.CarePickupExceptionRows
	PickupNote       timetableCompose.CarePickupNoteRows
	StatusDay        *services.StudentStatusDayRepository
}

// CareScheduleRepositories are the retained schedule repositories served by
// the Care Plan owner.
type CareScheduleRepositories = timetableCompose.CareScheduleRepositories

// NewCareScheduleRepositories builds the retained schedule repositories over
// the Care Plan capability.
func NewCareScheduleRepositories(capability careplan.Capability) CareScheduleRepositories {
	return timetableCompose.NewCareScheduleRepositories(services.NewCarePlanRows(capability))
}

func NewArrivalNoteRepository(capability careplan.Capability) timetableCompose.CareArrivalNoteRows {
	return timetableCompose.NewCareArrivalNoteRows(services.NewCarePlanRows(capability))
}

func NewPickupNoteRepository(capability careplan.Capability) timetableCompose.CarePickupNoteRows {
	return timetableCompose.NewCarePickupNoteRows(services.NewCarePlanRows(capability))
}
