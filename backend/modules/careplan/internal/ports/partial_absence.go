package ports

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	calendar "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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

// PartialAbsenceReplacer removes staff-entered partial absences so an
// approved full-day request can take their place (#5698). It joins the
// caller's unit of work.
type PartialAbsenceReplacer interface {
	ReplaceManualPartialAbsences(ctx context.Context, studentID int64, dates []calendar.Date) error
}
