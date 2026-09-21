package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Today is the Berlin calendar day in the owner's vocabulary. Compositions
// bind the Care Plan workflows' Today dependency to it.
func Today() careplan.Date { return careplan.Date(calendar.TodayDate()) }
