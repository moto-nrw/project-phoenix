package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

// RecurrenceLockDependencies wires the tenant recurrence gate
// (timetable.RecurrenceWriteLock). The gate takes transaction-scoped
// advisory locks on the caller's transaction over DB. GradeTransitionLockKey
// is School Structure's grade-transition gate key
// (schoolstructure.TransitionsLockKey), which this gate takes second.
type RecurrenceLockDependencies struct {
	DB                     *bun.DB
	GradeTransitionLockKey func(tenantID int64) string
}

// NewRecurrenceWriteLock composes the tenant recurrence gate.
func NewRecurrenceWriteLock(deps RecurrenceLockDependencies) (timetable.RecurrenceWriteLock, error) {
	if deps.DB == nil || deps.GradeTransitionLockKey == nil {
		return nil, errors.New("timetable recurrence lock: required dependency is nil")
	}
	return postgres.RecurrenceWriteLock{
		Store:                  postgres.New(postgres.DatabaseRuntime(deps.DB)),
		GradeTransitionLockKey: deps.GradeTransitionLockKey,
	}, nil
}
