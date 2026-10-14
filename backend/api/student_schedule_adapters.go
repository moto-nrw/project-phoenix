package api

import (
	"github.com/moto-nrw/project-phoenix/services"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

func NewArrivalScheduleRepository(capability careplan.Capability) timetableCompose.CareArrivalScheduleRows {
	return timetableCompose.NewCareArrivalScheduleRows(services.NewCarePlanRows(capability))
}
func NewArrivalExceptionRepository(capability careplan.Capability) timetableCompose.CareArrivalExceptionRows {
	return timetableCompose.NewCareArrivalExceptionRows(services.NewCarePlanRows(capability))
}

func NewPickupScheduleRepository(capability careplan.Capability) timetableCompose.CarePickupScheduleRows {
	return timetableCompose.NewCarePickupScheduleRows(services.NewCarePlanRows(capability))
}
func NewPickupExceptionRepository(capability careplan.Capability) timetableCompose.CarePickupExceptionRows {
	return timetableCompose.NewCarePickupExceptionRows(services.NewCarePlanRows(capability))
}
