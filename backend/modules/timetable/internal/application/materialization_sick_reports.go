package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// lockWindowDays takes the substitute day lock of every window day, ascending,
// after the recurrence gate: the order the instance lifecycle documents
// (LockDeviationDays) and the replan and template split already hold. The sick
// cascade marks a day's rows under the same lock, so a report committing
// while this run reads the reports either is visible to it or sees the rows
// it inserts (#6292). Without sick reports there is nothing to serialize.
func (s *materializationService) lockWindowDays(ctx context.Context, tenantID int64, from, to calendar.Date) error {
	if s.sickReports == nil {
		return nil
	}
	for day := from; !day.After(to); day = day.AddDays(1) {
		if err := s.dayLocks.AcquireTransactionLock(ctx, timetable.SubstituteDayLockKey(tenantID, day)); err != nil {
			return &ports.ScheduleError{Op: "materialize for tenant: lock day " + day.String(), Err: err}
		}
	}
	return nil
}

// loadSickDays indexes the window's open sick reports by staff member and
// full sick day; without a reader no supervisor is sick.
func (s *materializationService) loadSickDays(ctx context.Context, from, to calendar.Date) (map[staffDay]int64, error) {
	if s.sickReports == nil {
		return nil, nil
	}
	reports, err := s.sickReports.OpenSickReports(ctx, from, to)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "materialize for tenant: load sick reports", Err: err}
	}
	return sickDayIndex(reports, from, to), nil
}
