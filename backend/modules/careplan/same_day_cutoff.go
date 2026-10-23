package careplan

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

// ErrPickupChangeCutoffPassed means a guardian tried to set, edit or remove
// today's one-day pickup change after the school's same-day cutoff (#3163).
var ErrPickupChangeCutoffPassed = carerequests.ErrPickupChangeCutoffPassed

// SameDayCutoff is a school's latest wall-clock time for a guardian change to
// the current day; see carerequests.SameDayCutoff.
type SameDayCutoff = carerequests.SameDayCutoff

// NewSameDayCutoff validates a resolved setting value. An empty value is a
// valid "no cutoff"; anything else must be HH:MM.
func NewSameDayCutoff(clock string, now time.Time) (SameDayCutoff, error) {
	return carerequests.NewSameDayCutoff(clock, now)
}

// NewSameDayCutoffAt validates a resolved setting value and judges each
// Closed call at the supplied current time.
func NewSameDayCutoffAt(clock string, now func() time.Time) (SameDayCutoff, error) {
	return carerequests.NewSameDayCutoffAt(clock, now)
}
