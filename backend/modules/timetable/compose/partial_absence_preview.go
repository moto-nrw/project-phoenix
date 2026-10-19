package compose

import (
	"context"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// PickupExceptionLister lists a child's pickup exceptions (Care Plan).
type PickupExceptionLister interface {
	ListPickupExceptions(context.Context, ports.PickupExceptionFilter) ([]ports.PickupExceptionProjection, error)
}

// PartialAbsencePreview lists the blocks of a child's day an earlier pickup
// still excuses. Since the presence cutover (#2762) the joined read of the
// plan and the attendance lives in the Timetable projection; the two facts
// the read needs from other owners stay with them: People Directory says
// whether the child is still enrolled (a graduate gets no course blocks
// the roster does not list yet), Care Plan names the automatic excusals
// whose rows an approval may claim again.
type PartialAbsencePreview struct {
	reads      PresenceReads
	enrolled   func(context.Context, int64) (bool, error)
	exceptions PickupExceptionLister
}

// NewPartialAbsencePreview binds the preview: enrolled answers whether the
// child is still enrolled, exceptions lists the child's pickup exceptions.
func NewPartialAbsencePreview(reads PresenceReads, enrolled func(context.Context, int64) (bool, error), exceptions PickupExceptionLister) PartialAbsencePreview {
	return PartialAbsencePreview{reads: reads, enrolled: enrolled, exceptions: exceptions}
}

// FindPartialAbsenceBlocks lists the blocks of the ISO day that start at or
// after the clock and that the child is expected in or absent from without
// a manual or status-day decision.
func (p PartialAbsencePreview) FindPartialAbsenceBlocks(ctx context.Context, studentID int64, date string, from time.Time) ([]scheduleModels.PartialAbsenceBlock, error) {
	enrolled, err := p.enrolled(ctx, studentID)
	if err != nil {
		return nil, err
	}
	exceptions, err := p.exceptions.ListPickupExceptions(ctx, ports.PickupExceptionFilter{StudentIDs: []int64{studentID}, Date: date})
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
