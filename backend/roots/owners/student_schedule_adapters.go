package owners

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

func NewArrivalScheduleRepository(capability careplan.Capability) timetablePorts.CareArrivalScheduleRows {
	return timetableCompose.NewCareArrivalScheduleRows(NewCarePlanRows(capability))
}

func NewArrivalExceptionRepository(capability careplan.Capability) timetablePorts.CareArrivalExceptionRows {
	return timetableCompose.NewCareArrivalExceptionRows(NewCarePlanRows(capability))
}

func NewPickupScheduleRepository(capability careplan.Capability) timetablePorts.CarePickupScheduleRows {
	return timetableCompose.NewCarePickupScheduleRows(NewCarePlanRows(capability))
}

func NewPickupExceptionRepository(capability careplan.Capability) timetablePorts.CarePickupExceptionRows {
	return timetableCompose.NewCarePickupExceptionRows(NewCarePlanRows(capability))
}
