package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type PartialAbsenceStore interface {
	PickupExcusalStore
	FindByStudentIDAndDateRange(context.Context, int64, calendar.Date, calendar.Date) ([]*careplan.PickupException, error)
	Create(context.Context, *careplan.PickupException) error
	Delete(context.Context, int64) error
}
type PartialAbsenceConflicts interface {
	HasFullDayStatus(context.Context, int64, calendar.Date) (bool, error)
	PendingExcusedDates(context.Context, int64) ([]calendar.Date, error)
}
type PartialAbsenceSyncer interface {
	Sync(context.Context, int64) (bool, error)
}
