package application

import (
	"context"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type pickupExceptionLister interface {
	ListPickupExceptions(context.Context, ports.PickupExceptionFilter) ([]ports.PickupExceptionProjection, error)
}
type PartialAbsencePreview struct {
	reads interface {
		ListPartialAbsenceBlocks(context.Context, int64, string, time.Time, bool, []int64) ([]scheduleModels.PartialAbsenceBlock, error)
	}
	enrolled   func(context.Context, int64) (bool, error)
	exceptions pickupExceptionLister
	unit       func(context.Context, func(context.Context) error) error
}

func NewPartialAbsencePreview(reads interface {
	ListPartialAbsenceBlocks(context.Context, int64, string, time.Time, bool, []int64) ([]scheduleModels.PartialAbsenceBlock, error)
}, enrolled func(context.Context, int64) (bool, error), exceptions pickupExceptionLister, unit func(context.Context, func(context.Context) error) error) *PartialAbsencePreview {
	return &PartialAbsencePreview{reads: reads, enrolled: enrolled, exceptions: exceptions, unit: unit}
}

// FindPartialAbsenceBlocks lists the blocks of the ISO day that start at or
// after the clock and that the child is expected in or absent from without
// a manual or status-day decision.
func (p *PartialAbsencePreview) findPartialAbsenceBlocks(ctx context.Context, studentID int64, date string, from time.Time) ([]scheduleModels.PartialAbsenceBlock, error) {
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

func (p *PartialAbsencePreview) FindPartialAbsenceBlocks(ctx context.Context, studentID int64, date string, from time.Time) (rows []scheduleModels.PartialAbsenceBlock, err error) {
	err = p.unit(ctx, func(txCtx context.Context) error {
		rows, err = p.findPartialAbsenceBlocks(txCtx, studentID, date, from)
		return err
	})
	return
}
