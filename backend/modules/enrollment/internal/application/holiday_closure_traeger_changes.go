package application

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The changes a Träger closing day makes (#6434, #6457): its saved version
// against the stored one, for the schools the Träger still holds.

// traegerChanges are the changes a Träger closing day makes: its stored
// version no longer counts, the new one counts from the save on.
func (h *HolidayClosures) traegerChanges(ctx context.Context, organizationID int64, closure enrollment.HolidayClosure) ([]changedBooking, error) {
	if organizationID <= 0 {
		return nil, errors.New("enrollment: the Träger is required")
	}
	if closure.Previous != nil {
		traeger, err := h.traeger.HolidayTraeger(ctx, organizationID)
		if err != nil {
			return nil, err
		}
		current := make(map[int64]bool, len(traeger.Schools))
		for _, school := range traeger.Schools {
			current[school.ID] = true
		}
		// Historical references survive a transfer. Only current schools
		// can reopen here; newly requested schools still pass the calendar's
		// authorization below, without filtering.
		previous := *closure.Previous
		previous.SchoolIDs = nil
		for _, school := range closure.Previous.SchoolIDs {
			if current[school] {
				previous.SchoolIDs = append(previous.SchoolIDs, school)
			}
		}
		closure.Previous = &previous
	}
	schools, from, to := closureSpan(closure)
	if len(schools) == 0 {
		return nil, nil
	}
	start := max(from, h.today())
	if to.Before(start) {
		// Authorize the schools all the same: a refusal must not depend on
		// the dates.
		_, err := h.calendar.HolidayNoCareDays(ctx, schools, from, from)
		return nil, err
	}
	// The calendar authorizes every school before it reads anything.
	noCare, err := h.calendar.HolidayNoCareDays(ctx, schools, start, to)
	if err != nil {
		return nil, err
	}
	bookings, _, err := h.changes(ctx, organizationID, schools, start, to, traegerClosedOn(closure, noCareIndex(noCare, closure.TraegerClosingDayID)))
	if err != nil {
		return nil, err
	}
	return traegerChangesWithin(bookings, closure, start, to), nil
}

// closureSpan names the schools and days a Träger closing day touches: its
// new version unless it is deleted, and its stored version.
func closureSpan(closure enrollment.HolidayClosure) ([]int64, calendar.Date, calendar.Date) {
	var schools []int64
	var from, to calendar.Date
	widen := func(ids []int64, start, end calendar.Date) {
		if start.IsZero() || end.IsZero() || end.Before(start) {
			return
		}
		schools = append(schools, ids...)
		if from.IsZero() || start.Before(from) {
			from = start
		}
		if to.IsZero() || end.After(to) {
			to = end
		}
	}
	if !closure.Deleted {
		widen(closure.SchoolIDs, closure.StartsOn, closure.EndsOn)
	}
	if closure.Previous != nil {
		widen(closure.Previous.SchoolIDs, closure.Previous.StartsOn, closure.Previous.EndsOn)
	}
	return slices.Compact(slices.Sorted(slices.Values(schools))), from, to
}

// traegerClosedOn closes the days of the saved Träger closing day on top of
// the calendar, which still holds its stored version.
func traegerClosedOn(closure enrollment.HolidayClosure, index closedOn) closedOn {
	reason := strings.TrimSpace(closure.Reason)
	return func(schoolID int64, day calendar.Date) (bool, string, bool) {
		closed, name, holiday := index(schoolID, day)
		if holiday {
			return false, "", true
		}
		if !closure.Deleted && slices.Contains(closure.SchoolIDs, schoolID) && !day.Before(closure.StartsOn) && !day.After(closure.EndsOn) {
			return true, reason, false
		}
		return closed, name, false
	}
}
