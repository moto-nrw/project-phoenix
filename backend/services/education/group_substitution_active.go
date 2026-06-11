package education

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/education"
)

// SubstitutionIsActive reports whether the substitution is in effect on
// checkDate, i.e. checkDate falls within the inclusive [StartDate, EndDate]
// window. The active-window business decision lives in the service layer
// (Rule 12: models hold data, services hold rules) and takes the reference
// date as a parameter so it is deterministic and testable.
func SubstitutionIsActive(gs *education.GroupSubstitution, checkDate timezone.Date) bool {
	if gs == nil {
		return false
	}
	return !checkDate.Before(gs.StartDate) && !checkDate.After(gs.EndDate)
}

// SubstitutionIsActiveNow reports whether the substitution is in effect at
// the supplied current time, comparing the instant's Berlin calendar day
// against the inclusive date window. Callers pass the clock explicitly
// instead of the method reading time.Now() internally, keeping the
// wall-clock decision out of the model and injectable for tests.
func SubstitutionIsActiveNow(gs *education.GroupSubstitution, now time.Time) bool {
	return SubstitutionIsActive(gs, timezone.DateFromTime(now))
}
