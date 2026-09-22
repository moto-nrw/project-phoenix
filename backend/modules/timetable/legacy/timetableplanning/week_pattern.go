package timetableplanning

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
)

// shouldMaterializeWeekPattern asks the School Calendar's A/B-week engine
// whether a planning row with weekPattern occurs on date inside period. A nil
// period means no alternation is configured.
func shouldMaterializeWeekPattern(weekPattern int, date timezone.Date, period *schedule.CalendarPeriod) bool {
	if period == nil {
		return true
	}
	return schoolcalendar.WeekPatternApplies(weekPattern, date.String(), schoolcalendar.WeekCycleOf(period.WeekCycleLength, period.WeekCycleAnchor))
}
