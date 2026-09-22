package enrollment

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
)

// weekPatternApplies asks the School Calendar's single A/B-week engine
// whether a template schedule with weekPattern occurs on date inside period,
// so a catalog link is never accepted for a day the materializer would skip.
// A nil period means no alternation is configured.
func weekPatternApplies(weekPattern int, date timezone.Date, period *scheduleModels.CalendarPeriod) bool {
	if period == nil {
		return true
	}
	return schoolcalendar.WeekPatternApplies(weekPattern, date.String(), schoolcalendar.WeekCycleOf(period.WeekCycleLength, period.WeekCycleAnchor))
}
