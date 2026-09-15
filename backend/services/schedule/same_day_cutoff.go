package schedule

import (
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// ErrPickupChangeCutoffPassed means a guardian tried to set, edit or remove
// today's one-day pickup change after the school's same-day cutoff (#3163).
var ErrPickupChangeCutoffPassed = errors.New("schedule: same-day pickup change cutoff has passed")

// sameDayCutoffLayout is the wall-clock layout of a cutoff setting (HH:MM).
const sameDayCutoffLayout = "15:04"

// SameDayCutoff is a school's latest wall-clock time for a guardian change to
// the current day, together with the instant the change is judged at. It is
// resolved by the caller that knows the tenant, like ReasonRequired. The zero
// value has no cutoff and never closes a day.
//
// Only today is ever closed: the cutoff of a later day lies in the future, and
// past days are refused by the date window before this is asked. Each request
// kind resolves its own setting into this value, so a second kind reuses the
// check without sharing the pickup setting.
type SameDayCutoff struct {
	// Clock is the cutoff as HH:MM; empty means no cutoff.
	Clock string
	// Now is the instant the change is judged at.
	Now time.Time
}

// NewSameDayCutoff validates a resolved setting value. An empty value is a
// valid "no cutoff"; anything else must be HH:MM.
func NewSameDayCutoff(clock string, now time.Time) (SameDayCutoff, error) {
	if clock == "" {
		return SameDayCutoff{Now: now}, nil
	}
	if _, err := time.Parse(sameDayCutoffLayout, clock); err != nil {
		return SameDayCutoff{}, fmt.Errorf("schedule: invalid same-day cutoff %q: %w", clock, err)
	}
	return SameDayCutoff{Clock: clock, Now: now}, nil
}

// Closed reports whether a guardian may no longer change the given day. The
// cutoff minute itself is still open: at exactly 11:00 a change to today is
// accepted, one second later it is not. Now is compared in Europe/Berlin, so
// the wall-clock cutoff holds across summer and winter time.
func (c SameDayCutoff) Closed(date timezone.Date) bool {
	if c.Clock == "" {
		return false
	}
	now := c.Now.In(timezone.Berlin)
	if date != timezone.DateFromTime(now) {
		return false
	}
	clock, err := time.Parse(sameDayCutoffLayout, c.Clock)
	if err != nil {
		// NewSameDayCutoff rejects this; a hand-built value fails closed.
		return true
	}
	cutoff := time.Date(now.Year(), now.Month(), now.Day(), clock.Hour(), clock.Minute(), 0, 0, timezone.Berlin)
	return now.After(cutoff)
}
