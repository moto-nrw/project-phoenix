package timetable

import "context"

// CalendarPeriodReferences counts the Timetable-owned planning rows that
// reference one School Calendar period. The School Calendar deletion preview
// renders them as usage; they are planning references, not actual room
// supervision or attendance.
type CalendarPeriodReferences struct {
	ActivityGroups     int
	Schedules          int
	StudentEnrollments int
	Supervisors        int
	ActivityInstances  int
}

// CountCalendarPeriodReferences reports, per calendar period of the caller's
// school, how many Timetable rows reference it. Periods without references
// are omitted. All Timetable tables are read in one statement (#3124).
func (m *Module) CountCalendarPeriodReferences(ctx context.Context) (map[int64]CalendarPeriodReferences, error) {
	return m.engine.CountCalendarPeriodReferences(ctx)
}
