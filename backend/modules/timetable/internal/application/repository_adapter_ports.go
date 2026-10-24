package application

import (
	"context"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
)

// PartialAbsences lists the blocks of a child's day an earlier pickup still
// excuses; the read needs facts of People Directory and Care Plan, so the
// composition root supplies it.
type PartialAbsences interface {
	FindPartialAbsenceBlocks(ctx context.Context, studentID int64, date string, from time.Time) ([]scheduleModels.PartialAbsenceBlock, error)
}
