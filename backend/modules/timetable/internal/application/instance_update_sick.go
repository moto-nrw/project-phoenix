package application

import (
	"context"

	scheduleModel "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// sickDaysOn indexes the open sick reports of one day (#6292). The caller
// holds the day lock, the lock the sick cascade marks the day's rows under.
func (s *InstanceLifecycleService) sickDaysOn(ctx context.Context, day calendar.Date) (map[staffDay]int64, error) {
	if s.deps.SickReports == nil {
		return nil, nil
	}
	reports, err := s.deps.SickReports.OpenSickReports(ctx, day, day)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "update instance: load sick reports", Err: err}
	}
	return sickDayIndex(reports, day, day), nil
}

// applySickReportOfDay gives an added or moved assignment the sick state of
// the block's date: absent with the report's stamp on a full sick day,
// released from a carried stamp otherwise. A manual absence stays, as the
// cascade leaves an already absent row alone.
func applySickReportOfDay(row *scheduleModel.InstanceStaff, absenceID int64) {
	switch {
	case row.IsAbsent && row.SickAbsenceID == nil:
	case absenceID > 0:
		stampSickAbsence(row, absenceID)
	case row.SickAbsenceID != nil:
		copyAbsenceState(row, &scheduleModel.InstanceStaff{})
	}
}
