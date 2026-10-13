package services

import (
	"context"

	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// pickupExcusalTimetable connects the Care Plan workflow to Timetable's
// later-pickup decisions and to the block preview, which joins the Timetable
// plan with the Student Presence execution and attendance (#2762).
type pickupExcusalTimetable struct {
	timetable.Capability
	partialAbsenceBlocks
}

func newPickupExcusalTimetable(capability timetable.Capability, preview timetableCompose.PartialAbsencePreview) pickupExcusalTimetable {
	return pickupExcusalTimetable{Capability: capability, partialAbsenceBlocks: newPartialAbsenceBlocks(preview)}
}

func (a pickupExcusalTimetable) RecordPickupDayExtension(ctx context.Context, input careplanCompose.PickupDayExtension) error {
	return a.Capability.RecordPickupDayExtension(ctx, timetable.PickupDayExtension(input))
}
func (a pickupExcusalTimetable) RecordPickupWeekdayExtension(ctx context.Context, input careplanCompose.PickupWeekdayExtension) error {
	return a.Capability.RecordPickupWeekdayExtension(ctx, timetable.PickupWeekdayExtension(input))
}
