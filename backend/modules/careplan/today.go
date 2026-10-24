package careplan

import (
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Today is the Berlin calendar day in the owner's vocabulary. Compositions
// bind the Care Plan workflows' Today dependency to it.
func Today() Date { return Date(calendar.TodayDate()) }
