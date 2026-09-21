package careplan

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// WeeklyPickupSnapshot captures the regular pickup times before a weekly write.
type WeeklyPickupSnapshot map[int]string

// PickupAutoExcusal derives block absences from early pickups and records
// later-pickup planning tasks. Except ResyncFutureExceptions, callers must
// hold the student and care-day locks in their tenant transaction.
type PickupAutoExcusal interface {
	Sync(context.Context, int64) (bool, error)
	Preview(context.Context, int64, calendar.Date, time.Time) ([]carerequests.Block, error)
	DetachForDate(context.Context, int64, calendar.Date) error
	DetachRow(context.Context, *PickupException) error
	ResyncFutureExceptions(context.Context, int64) error
	ReleaseBeforeDelete(context.Context, *PickupException) error
	SnapshotWeeklyPickups(context.Context, int64, calendar.Date) (WeeklyPickupSnapshot, error)
	RecordWeeklyPickupChanges(context.Context, int64, calendar.Date, WeeklyPickupSnapshot) error
}

func (e *PickupException) HasManualPartialAbsence() bool {
	return e.ExcusedFrom != nil && !e.ExcusedAuto
}

// NormalizeWallClockTimes re-anchors scanned TIME values before a full-row update.
func (e *PickupException) NormalizeWallClockTimes() {
	if e.PickupTime != nil {
		clock := calendar.NormalizeWallClock(*e.PickupTime)
		e.PickupTime = &clock
	}
	if e.ExcusedFrom != nil {
		clock := calendar.NormalizeWallClock(*e.ExcusedFrom)
		e.ExcusedFrom = &clock
	}
}
