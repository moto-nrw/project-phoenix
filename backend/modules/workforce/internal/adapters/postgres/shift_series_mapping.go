package postgres

import "github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"

// The series row mapping between the stored schedule.staff_shift_series
// columns and the domain rule (#1889, #3820).

func staffShiftSeriesFromDomain(value domain.StaffShiftSeries) (*staffShiftSeriesRow, error) {
	start, err := requiredClock(value.StartTime, "start time")
	if err != nil {
		return nil, err
	}
	end, err := requiredClock(value.EndTime, "end time")
	if err != nil {
		return nil, err
	}
	weekdays := make([]int16, 0, len(value.Weekdays))
	for _, weekday := range value.Weekdays {
		weekdays = append(weekdays, int16(weekday)) // #nosec G115 -- validated ISO weekday 1..7
	}
	return &staffShiftSeriesRow{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Weekdays: weekdays,
		StartTime: start, EndTime: end, BreakMinutes: value.BreakMinutes, ShiftTypeID: value.ShiftTypeID,
		Notes: value.Notes, CalendarPeriodID: value.CalendarPeriodID, WeekPattern: value.WeekPattern,
		ValidFrom: calendarDate(value.ValidFrom), ValidUntil: optionalCalendarDate(value.ValidUntil),
		SeriesRootID: value.SeriesRootID, RetainedOccurrenceShiftID: value.RetainedOccurrenceShiftID,
		IncludeSchoolBreaks: value.IncludeSchoolBreaks, CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}, nil
}

func staffShiftSeriesToDomain(row staffShiftSeriesRow) domain.StaffShiftSeries {
	weekdays := make([]int, 0, len(row.Weekdays))
	for _, weekday := range row.Weekdays {
		weekdays = append(weekdays, int(weekday))
	}
	return domain.StaffShiftSeries{
		ID: row.ID, TenantID: row.TenantID, StaffID: row.StaffID, Weekdays: weekdays,
		StartTime: wallClockString(row.StartTime), EndTime: wallClockString(row.EndTime), BreakMinutes: row.BreakMinutes,
		ShiftTypeID: row.ShiftTypeID, Notes: row.Notes, CalendarPeriodID: row.CalendarPeriodID, WeekPattern: row.WeekPattern,
		ValidFrom: string(row.ValidFrom), ValidUntil: calendarDateString(row.ValidUntil),
		SeriesRootID: row.SeriesRootID, RetainedOccurrenceShiftID: row.RetainedOccurrenceShiftID,
		IncludeSchoolBreaks: row.IncludeSchoolBreaks, CreatedBy: row.CreatedBy, UpdatedBy: row.UpdatedBy,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
