package planning

import (
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// seriesOptions binds the series service's optional collaborators. A nil
// calendar is left unbound, so the service sees no typed-nil reader.
func SeriesOptions(broadcaster realtime.Broadcaster, today func() calendar.Date, nonWorkingDays SeriesCalendarReads) []StaffShiftSeriesOption {
	options := []StaffShiftSeriesOption{
		WithStaffShiftSeriesBroadcaster(broadcaster),
		WithStaffShiftSeriesToday(today),
	}
	if nonWorkingDays != nil {
		options = append(options, WithStaffShiftSeriesNonWorkingDays(SchoolCalendarNonWorkingDays(nonWorkingDays)))
	}
	return options
}
