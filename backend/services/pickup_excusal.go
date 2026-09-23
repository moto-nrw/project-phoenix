package services

import (
	"context"

	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/uptrace/bun"
)

// pickupExcusalTimetable connects the Care Plan workflow to Timetable's
// later-pickup decisions and to the block preview, which joins the Timetable
// plan with the Student Presence execution and attendance (#2762).
type pickupExcusalTimetable struct {
	timetable.Capability
	partialAbsenceBlocks
}

func newPickupExcusalTimetable(capability timetable.Capability, db *bun.DB) pickupExcusalTimetable {
	return pickupExcusalTimetable{Capability: capability, partialAbsenceBlocks: newPartialAbsenceBlocks(db)}
}

func (a pickupExcusalTimetable) RecordPickupDayExtension(ctx context.Context, input careplanCompose.PickupDayExtension) error {
	return a.Capability.RecordPickupDayExtension(ctx, timetable.PickupDayExtension(input))
}
func (a pickupExcusalTimetable) RecordPickupWeekdayExtension(ctx context.Context, input careplanCompose.PickupWeekdayExtension) error {
	return a.Capability.RecordPickupWeekdayExtension(ctx, timetable.PickupWeekdayExtension(input))
}
