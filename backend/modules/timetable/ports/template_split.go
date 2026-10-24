package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// SeriesDeviations is the instance lifecycle's deviation machinery the split
// needs around its delete/materialize cycle (#1840): the day-wide staffing
// locks of the window and a snapshot of the old template's Vertretungsplan
// overrides that is reapplied to the successor's rematerialized occurrences.
// The composition root binds the retained instance service.
type SeriesDeviations interface {
	LockDeviationDays(ctx context.Context, tenantID int64, from, to calendar.Date) error
	SnapshotDeviations(ctx context.Context, from, to calendar.Date, templateID int64) (PreservedDeviations, error)
}

// PreservedDeviations is one snapshot of SeriesDeviations.
type PreservedDeviations interface {
	// Count is the number of snapshotted deviations.
	Count() int
	// Reapply writes the snapshot onto the given template's occurrences and
	// reports how many deviations it reapplied.
	Reapply(ctx context.Context, templateID int64, actorAccountID *int64) (int, error)
}
