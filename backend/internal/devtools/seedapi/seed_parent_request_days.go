package api

import "time"

// seedSchoolDayAfter is the n-th Monday-to-Friday day after today. The
// product screenshots let Felix's parent pick the next school day (absence)
// and the one after (pickup change), so the parent's own open pickup change
// sits on the third school day and never on a day a shot picks.
func seedSchoolDayAfter(today seedDate, n int) seedDate {
	day := today
	for n > 0 {
		day = day.AddDays(1)
		if day.Weekday() != time.Saturday && day.Weekday() != time.Sunday {
			n--
		}
	}
	return day
}

// seedWalkingHomeAloneDate keeps the signed way-home request separate from
// the open pickup change on the third school day and the screenshot dates.
func seedWalkingHomeAloneDate(today seedDate) seedDate {
	return seedSchoolDayAfter(today, 4)
}
