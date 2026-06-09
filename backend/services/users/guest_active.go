package users

import (
	"time"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// GuestIsActive reports whether a guest instructor is active at the given
// reference time. The active-window decision lives in the service layer
// (Rule 12: models hold data, services hold rules) and takes `now` as a
// parameter so it is deterministic and testable.
//
// Policy: a guest with no start/end dates is always active. A guest with
// only a start date is active from that date onward; with only an end date,
// active up to and including it; with both, active inside the inclusive
// [start, end] window.
func GuestIsActive(g *userModels.Guest, now time.Time) bool {
	if g == nil {
		return false
	}

	// No dates set => always active.
	if g.StartDate == nil && g.EndDate == nil {
		return true
	}

	// Only start date set => active once it has been reached.
	if g.StartDate != nil && g.EndDate == nil {
		return !now.Before(*g.StartDate)
	}

	// Only end date set => active until it passes.
	if g.StartDate == nil && g.EndDate != nil {
		return !now.After(*g.EndDate)
	}

	// Both dates set => active inside the inclusive window.
	return !now.Before(*g.StartDate) && !now.After(*g.EndDate)
}
