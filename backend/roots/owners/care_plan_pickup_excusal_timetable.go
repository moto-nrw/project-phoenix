package owners

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// pickupExcusalTimetable connects the Care Plan workflow to Timetable's
// later-pickup decisions and to the block preview, which joins the Timetable
// plan with the Student Presence execution and attendance (#2762).
type pickupExcusalTimetable struct {
	timetable.Capability
	partialAbsenceBlocks
}

func NewPickupExcusalTimetable(capability timetable.Capability, preview timetableCompose.PartialAbsencePreview) pickupExcusalTimetable {
	return pickupExcusalTimetable{Capability: capability, partialAbsenceBlocks: newPartialAbsenceBlocks(preview)}
}

func (a pickupExcusalTimetable) RecordPickupDayExtension(ctx context.Context, input careplanCompose.PickupDayExtension) error {
	return a.Capability.RecordPickupDayExtension(ctx, timetable.PickupDayExtension(input))
}

func (a pickupExcusalTimetable) RecordPickupWeekdayExtension(ctx context.Context, input careplanCompose.PickupWeekdayExtension) error {
	return a.Capability.RecordPickupWeekdayExtension(ctx, timetable.PickupWeekdayExtension(input))
}

// partialAbsenceBlocks previews the blocks of a child's day an earlier pickup
// can still excuse.
type partialAbsenceBlocks struct {
	preview timetableCompose.PartialAbsencePreview
}

func newPartialAbsenceBlocks(preview timetableCompose.PartialAbsencePreview) partialAbsenceBlocks {
	return partialAbsenceBlocks{preview: preview}
}

func (p partialAbsenceBlocks) FindPartialAbsenceBlocks(ctx context.Context, studentID int64, date timezone.Date, clock time.Time) ([]carerequests.Block, error) {
	rows, err := p.preview.FindPartialAbsenceBlocks(ctx, studentID, date.String(), clock)
	if err != nil {
		return nil, err
	}
	blocks := make([]carerequests.Block, 0, len(rows))
	for _, row := range rows {
		blocks = append(blocks, carerequests.Block{ID: row.ID, Title: row.Title, StartTime: row.StartTime, EndTime: row.EndTime})
	}
	return blocks, nil
}
