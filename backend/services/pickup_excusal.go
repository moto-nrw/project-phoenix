package services

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// pickupExcusalTimetable connects the Care Plan workflow to Timetable's
// public block commands, preview, and later-pickup decisions.
type pickupExcusalTimetable struct{ timetable.Capability }

func (a pickupExcusalTimetable) FindPartialAbsenceBlocks(ctx context.Context, id int64, date timezone.Date, clock time.Time) ([]carerequests.Block, error) {
	rows, err := a.ListPartialAbsenceBlocks(ctx, id, date.String(), clock)
	if err != nil {
		return nil, err
	}
	blocks := make([]carerequests.Block, 0, len(rows))
	for _, row := range rows {
		blocks = append(blocks, carerequests.Block{ID: row.ID, Title: row.Title, StartTime: row.StartTime, EndTime: row.EndTime})
	}
	return blocks, nil
}
func (a pickupExcusalTimetable) RecordPickupDayExtension(ctx context.Context, input careplanCompose.PickupDayExtension) error {
	return a.Capability.RecordPickupDayExtension(ctx, timetable.PickupDayExtension(input))
}
func (a pickupExcusalTimetable) RecordPickupWeekdayExtension(ctx context.Context, input careplanCompose.PickupWeekdayExtension) error {
	return a.Capability.RecordPickupWeekdayExtension(ctx, timetable.PickupWeekdayExtension(input))
}
