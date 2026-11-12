package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// HolidayOwnChildCareDays reconciles Enrollment's booked days. A team's or
// parent's exception wins; a team edit survives while the booked times stay.
type HolidayOwnChildCareDays struct {
	plans enrollment.HolidayOwnChildPlans
}

func NewHolidayOwnChildCareDays(plans enrollment.HolidayOwnChildPlans) *HolidayOwnChildCareDays {
	if plans == nil {
		panic("holiday own-child care days: plans are required")
	}
	return &HolidayOwnChildCareDays{plans: plans}
}

type holidayOwnChildDayKey struct {
	studentID int64
	day       calendar.Date
}

// holidayOwnChildReason is the reason of a planned day: the marker and the
// booked times, which tell a changed booking from a time the team edited.
func holidayOwnChildReason(day enrollment.HolidayOwnChildDay) string {
	return fmt.Sprintf("%s (%s bis %s Uhr)", enrollment.HolidayOwnChildCareReason, day.Arrival, day.Pickup)
}

func isHolidayOwnChildReason(reason *string) bool {
	return reason != nil && strings.HasPrefix(*reason, enrollment.HolidayOwnChildCareReason)
}

func (c *HolidayOwnChildCareDays) SyncHolidayOwnChildDays(ctx context.Context, from, today calendar.Date, wanted []enrollment.HolidayOwnChildDay) error {
	byKey := make(map[holidayOwnChildDayKey]enrollment.HolidayOwnChildDay, len(wanted))
	for _, day := range wanted {
		key := holidayOwnChildDayKey{day.StudentID, day.Day}
		if _, ok := byKey[key]; !ok && !day.Day.Before(from) {
			byKey[key] = day
		}
	}
	arrivals, err := c.plans.HolidayOwnChildArrivals(ctx, from)
	if err != nil {
		return err
	}
	pickups, err := c.plans.HolidayOwnChildPickups(ctx, from)
	if err != nil {
		return err
	}
	arrivalDone, deleteArrivals := settleOwnChildExceptions(arrivals, byKey)
	pickupDone, deletePickups := settleOwnChildExceptions(pickups, byKey)
	changes := enrollment.HolidayOwnChildPlanChanges{DeleteArrivals: deleteArrivals, DeletePickups: deletePickups}
	if err := planMissingOwnChildDays(today, wanted, byKey, arrivalDone, pickupDone, &changes); err != nil {
		return err
	}
	return c.plans.ApplyHolidayOwnChildPlans(ctx, changes)
}

// settleOwnChildExceptions removes only entries whose booking went or changed.
// A pickup with an excused time belongs to the team now and always stays.
func settleOwnChildExceptions(existing []enrollment.HolidayOwnChildException, wanted map[holidayOwnChildDayKey]enrollment.HolidayOwnChildDay) (map[holidayOwnChildDayKey]bool, []int64) {
	done := map[holidayOwnChildDayKey]bool{}
	var removed []int64
	for _, exception := range existing {
		key := holidayOwnChildDayKey{exception.StudentID, exception.Day}
		day, booked := wanted[key]
		if !isHolidayOwnChildReason(exception.Reason) || exception.Excused || booked && *exception.Reason == holidayOwnChildReason(day) {
			done[key] = true
		} else {
			removed = append(removed, exception.ID)
		}
	}
	return done, removed
}

// planMissingOwnChildDays prepares the booked days from today on that carry
// no plan yet. The booking is their author, so no account has to be named.
func planMissingOwnChildDays(today calendar.Date, wanted []enrollment.HolidayOwnChildDay, byKey map[holidayOwnChildDayKey]enrollment.HolidayOwnChildDay, arrivalDone, pickupDone map[holidayOwnChildDayKey]bool, changes *enrollment.HolidayOwnChildPlanChanges) error {
	for _, candidate := range wanted {
		key := holidayOwnChildDayKey{candidate.StudentID, candidate.Day}
		day, ok := byKey[key]
		if !ok || day.Day.Before(today) || arrivalDone[key] && pickupDone[key] {
			continue
		}
		delete(byKey, key)
		if err := appendOwnChildPlans(changes, day, !arrivalDone[key], !pickupDone[key]); err != nil {
			return err
		}
	}
	return nil
}

func appendOwnChildPlans(changes *enrollment.HolidayOwnChildPlanChanges, day enrollment.HolidayOwnChildDay, arrival, pickup bool) error {
	arrivalAt, err := ownChildClock(day.Arrival)
	if err != nil {
		return err
	}
	pickupAt, err := ownChildClock(day.Pickup)
	if err != nil {
		return err
	}
	reason := holidayOwnChildReason(day)
	if arrival {
		changes.Arrivals = append(changes.Arrivals, enrollment.HolidayOwnChildPlan{StudentID: day.StudentID, Day: day.Day, Time: arrivalAt, Reason: reason})
	}
	if pickup {
		changes.Pickups = append(changes.Pickups, enrollment.HolidayOwnChildPlan{StudentID: day.StudentID, Day: day.Day, Time: pickupAt, Reason: reason})
	}
	return nil
}

func ownChildClock(value string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02 15:04", "2000-01-01 "+value)
	if err != nil {
		return time.Time{}, fmt.Errorf("holiday own-child care days: clock %q: %w", value, err)
	}
	return calendar.NormalizeWallClock(parsed), nil
}
