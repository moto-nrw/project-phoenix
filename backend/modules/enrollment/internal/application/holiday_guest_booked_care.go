package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// HolidayGuestBookings lists the guests of the school in the caller's tenant
// transaction whose week overlaps [from, to], among studentIDs.
type HolidayGuestBookings interface {
	ListGuestBookings(ctx context.Context, studentIDs []int64, from, to calendar.Date) ([]enrollment.HolidayGuest, error)
}

// HolidayGuestBookedCare serves enrollment.HolidayGuestBookedCare (#5994).
type HolidayGuestBookedCare struct {
	store HolidayGuestBookings
}

var _ enrollment.HolidayGuestBookedCare = HolidayGuestBookedCare{}

// NewHolidayGuestBookedCare builds the read of the guests' booked days.
func NewHolidayGuestBookedCare(store HolidayGuestBookings) HolidayGuestBookedCare {
	if store == nil {
		panic("enrollment: the holiday guest store is required")
	}
	return HolidayGuestBookedCare{store: store}
}

// HolidayGuestBookedDays lists the booked days of each guest within
// [from, to]: every day of its week, the last one the day before a
// cancellation ended it. A time model whose clock cannot be read is an
// error, never a day without a booking.
func (c HolidayGuestBookedCare) HolidayGuestBookedDays(ctx context.Context, studentIDs []int64, from, to calendar.Date) (map[int64]map[calendar.Date]enrollment.HolidayGuestBookedDay, error) {
	result := map[int64]map[calendar.Date]enrollment.HolidayGuestBookedDay{}
	if len(studentIDs) == 0 || to.Before(from) {
		return result, nil
	}
	guests, err := c.store.ListGuestBookings(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	for _, guest := range guests {
		day, err := guestBookedDay(guest)
		if err != nil {
			return nil, err
		}
		first, last := guestBookedSpan(guest, from, to)
		for date := first; !date.After(last); date = date.AddDays(1) {
			if result[guest.StudentID] == nil {
				result[guest.StudentID] = map[calendar.Date]enrollment.HolidayGuestBookedDay{}
			}
			result[guest.StudentID][date] = day
		}
	}
	return result, nil
}

// guestBookedSpan is the part of the guest's week within [from, to] its
// booking still held: a cancellation ends it on ended_on.
func guestBookedSpan(guest enrollment.HolidayGuest, from, to calendar.Date) (calendar.Date, calendar.Date) {
	first, last := guest.StartsOn, guest.EndsOn
	if guest.EndedOn != nil && !guest.EndedOn.After(last) {
		last = guest.EndedOn.AddDays(-1)
	}
	if first.Before(from) {
		first = from
	}
	if last.After(to) {
		last = to
	}
	return first, last
}

func guestBookedDay(guest enrollment.HolidayGuest) (enrollment.HolidayGuestBookedDay, error) {
	starts, err := timeModelClock(guest.TimeModelStarts)
	if err != nil {
		return enrollment.HolidayGuestBookedDay{}, fmt.Errorf("enrollment: start of the time model of guest %d: %w", guest.StudentID, err)
	}
	ends, err := timeModelClock(guest.TimeModelEnds)
	if err != nil {
		return enrollment.HolidayGuestBookedDay{}, fmt.Errorf("enrollment: end of the time model of guest %d: %w", guest.StudentID, err)
	}
	return enrollment.HolidayGuestBookedDay{StartsAt: starts, EndsAt: ends}, nil
}

// timeModelClock reads a clock time "15:04" of a time model.
func timeModelClock(value string) (calendar.WallClock, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return calendar.WallClock{}, err
	}
	return calendar.NewWallClock(parsed.Hour(), parsed.Minute(), 0, 0)
}
