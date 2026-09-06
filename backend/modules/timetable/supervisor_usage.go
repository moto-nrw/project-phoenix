package timetable

import "context"

// CountPlannedSupervisorsByCalendarPeriod reports assignment references used
// by the School Calendar deletion preview, not actual room supervision.
func (m *Module) CountPlannedSupervisorsByCalendarPeriod(ctx context.Context) (map[int64]int, error) {
	return m.engine.CountPlannedSupervisorsByCalendarPeriod(ctx)
}
