package careplan

import "github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"

// ErrPickupChangeCutoffPassed means a guardian tried to set, edit or remove
// today's one-day pickup change after the school's same-day cutoff (#3163).
var ErrPickupChangeCutoffPassed = carerequests.ErrPickupChangeCutoffPassed

// SameDayCutoff is a school's latest wall-clock time for a guardian change to
// the current day; see carerequests.SameDayCutoff.
type SameDayCutoff = carerequests.SameDayCutoff
