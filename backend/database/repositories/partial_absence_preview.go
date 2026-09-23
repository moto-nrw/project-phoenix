package repositories

import (
	"context"
	"fmt"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

// PartialAbsencePreview lists the blocks of a child's day an earlier pickup
// still excuses. Since the presence cutover (#2762) the joined read of the
// plan and the attendance lives in the Timetable projection; the two facts
// the read needs from other owners stay with them: People Directory says
// whether the child is still enrolled (a graduate gets no course blocks
// the roster does not list yet), Care Plan names the automatic excusals
// whose rows an approval may claim again.
type PartialAbsencePreview struct {
	reads      timetableCompose.PresenceReads
	students   peopledirectory.StudentQuery
	exceptions carePlanCompose.ExceptionQueries
}

func NewPartialAbsencePreview(db *bun.DB) PartialAbsencePreview {
	exceptions, err := carePlanCompose.NewExceptionQueries(db, func(carePlanCompose.Observation) {})
	if err != nil {
		panic(fmt.Sprintf("partial absence preview: compose care plan exception queries: %v", err))
	}
	return PartialAbsencePreview{reads: mustPresenceReads(db), students: MustNewPeopleDirectory(db), exceptions: exceptions}
}

// FindPartialAbsenceBlocks lists the blocks of the ISO day that start at or
// after the clock and that the child is expected in or absent from without
// a manual or status-day decision.
func (p PartialAbsencePreview) FindPartialAbsenceBlocks(ctx context.Context, studentID int64, date string, from time.Time) ([]scheduleModels.PartialAbsenceBlock, error) {
	students, err := p.students.ListStudentsByID(ctx, []int64{studentID})
	if err != nil {
		return nil, err
	}
	enrolled := false
	for _, student := range students {
		if student.ID == studentID && !student.IsAlumnus() {
			enrolled = true
		}
	}
	exceptions, err := pickupExceptionDirectory{query: p.exceptions}.ListPickupExceptions(ctx, timetableCompose.PickupExceptionFilter{StudentIDs: []int64{studentID}, Date: date})
	if err != nil {
		return nil, err
	}
	autoIDs := make([]int64, 0, len(exceptions))
	for _, exception := range exceptions {
		if exception.ExcusedAuto {
			autoIDs = append(autoIDs, exception.ID)
		}
	}
	return p.reads.ListPartialAbsenceBlocks(ctx, studentID, date, from, enrolled, autoIDs)
}
