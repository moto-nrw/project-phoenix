package repositories

import (
	"errors"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// NewTimetableRecurrenceLock composes the Timetable owner's tenant recurrence
// gate (#3424 slice S2) over the caller's transaction and School Structure's
// grade-transition key. Every recurrence writer of the composition roots
// takes the gate through it, so the lock order stays the same everywhere.
func NewTimetableRecurrenceLock(db *bun.DB) (timetable.RecurrenceWriteLock, error) {
	if db == nil {
		return nil, errors.New("timetable recurrence lock: database is required")
	}
	return timetableCompose.NewRecurrenceWriteLock(timetableCompose.RecurrenceLockDependencies{
		DB:                     db,
		GradeTransitionLockKey: schoolstructure.TransitionsLockKey,
	})
}
