package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Booking of days and the bound of care days per child (#4701); the rules
// are on enrollment.HolidayChildDayLimitRefusal and holiday_days.go.

// holidayBookingKept reports whether kept, the child's bookings so far,
// books the time model of booking in its week and every requested day.
func holidayBookingKept(kept []enrollment.HolidayBookingInput, booking enrollment.HolidayBookingInput) bool {
	return slices.ContainsFunc(kept, func(k enrollment.HolidayBookingInput) bool {
		return k.WeekID == booking.WeekID && k.TimeModelID == booking.TimeModelID &&
			!slices.ContainsFunc(booking.Days, func(day calendar.Date) bool {
				return !slices.Contains(k.Days, day)
			})
	})
}

// validateHolidayBookingDays checks the days of a booking against its week
// (#4701): a week by days needs at least one of its care days and only
// those, a week booked as a whole names none. The days come normalized.
func validateHolidayBookingDays(week enrollment.HolidayWeek, days []calendar.Date) error {
	if week.BookingUnit != enrollment.HolidayBookingUnitDay {
		if len(days) > 0 {
			return errors.New("the week is booked as a whole")
		}
		return nil
	}
	if len(days) == 0 {
		return errors.New("at least one day is required")
	}
	careDays := enrollment.HolidayWeekCareDays(week.StartsOn, week.EndsOn)
	for _, day := range days {
		if !slices.Contains(careDays, day) {
			return fmt.Errorf("%s is no care day of the week", day)
		}
	}
	return nil
}

// normalizeHolidayBookings sorts the days of each booking and drops their
// repetitions.
func normalizeHolidayBookings(bookings []enrollment.HolidayBookingInput) []enrollment.HolidayBookingInput {
	normalized := make([]enrollment.HolidayBookingInput, len(bookings))
	for i, booking := range bookings {
		booking.Days = enrollment.NormalizeHolidayDays(booking.Days)
		normalized[i] = booking
	}
	return normalized
}

// holidayBookedDayCount counts the care days the bookings take (#4701): the
// days of a booking of days, every care day of the week for a whole week. A
// booking of a week outside the period counts nothing; it is refused apart.
func holidayBookedDayCount(bookings []enrollment.HolidayBookingInput, period enrollment.HolidayPeriod) int {
	weeks := map[int64]enrollment.HolidayWeek{}
	for _, week := range period.Weeks {
		weeks[week.ID] = week
	}
	count := 0
	for _, booking := range bookings {
		if week, ok := weeks[booking.WeekID]; ok {
			count += len(enrollment.HolidayBookedDays(week, booking.Days))
		}
	}
	return count
}

// refuseHolidayDayLimit refuses the bookings of the child at position child
// that take more care days than the period allows (#4701). before is the
// number of days the child booked so far: a rebooking of a child already
// over a lowered bound may keep or lower its days, not raise them.
func refuseHolidayDayLimit(child int, bookings []enrollment.HolidayBookingInput, period enrollment.HolidayPeriod, before int) error {
	if period.MaxDaysPerChild == nil {
		return nil
	}
	limit := *period.MaxDaysPerChild
	days := holidayBookedDayCount(bookings, period)
	if days <= limit || (before > limit && days <= before) {
		return nil
	}
	return &enrollment.HolidayChildDayLimitRefusal{Child: child, Limit: limit, Days: days}
}

// refuseRaisedDayLimit applies the bound of care days to a change of the
// period (#6506): a booking of a whole week takes the care days of the
// week's current dates, so new dates of a booked week change the days its
// children take. A child may keep or lower its days, as with a rebooking;
// it may not get more days than the written bound allows. The caller holds
// the lock of the period, which every change of a child's bookings takes
// too, so the bookings read here stay those of the commit. A change that
// moves no stored week reads nothing.
func (h *HolidayPeriods) refuseRaisedDayLimit(ctx context.Context, organizationID, periodID int64, input enrollment.HolidayPeriodInput, current enrollment.HolidayPeriod) error {
	if input.MaxDaysPerChild == nil {
		return nil
	}
	written := enrollment.HolidayPeriod{MaxDaysPerChild: input.MaxDaysPerChild}
	moved := map[int64]int{}
	for i, week := range input.Weeks {
		if week.ID == 0 {
			continue
		}
		written.Weeks = append(written.Weeks, enrollment.HolidayWeek{ID: week.ID, StartsOn: week.StartsOn, EndsOn: week.EndsOn, BookingUnit: week.BookingUnit})
		if before, ok := holidayStoredWeek(current, week.ID); ok && (before.StartsOn != week.StartsOn || before.EndsOn != week.EndsOn) {
			moved[week.ID] = i
		}
	}
	if len(moved) == 0 {
		return nil
	}
	children, err := h.store.HolidayChildBookings(ctx, organizationID, periodID)
	if err != nil {
		return fmt.Errorf("enrollment: read holiday child bookings: %w", err)
	}
	for _, bookings := range children {
		var refusal *enrollment.HolidayChildDayLimitRefusal
		if !errors.As(refuseHolidayDayLimit(0, bookings, written, holidayBookedDayCount(bookings, current)), &refusal) {
			continue
		}
		week := holidayRaisingWeek(bookings, moved, current, written)
		return &enrollment.HolidayWeekDayLimitRefusal{Week: week, StartsEarlier: holidayStartsEarlier(input.Weeks[week], current), Limit: refusal.Limit, Days: refusal.Days}
	}
	return nil
}

// holidayRaisingWeek is the position of the first moved week in which the
// bookings take more days than before; the bookings take more days in all,
// so one of them does.
func holidayRaisingWeek(bookings []enrollment.HolidayBookingInput, moved map[int64]int, current, written enrollment.HolidayPeriod) int {
	week := -1
	for _, booking := range bookings {
		index, ok := moved[booking.WeekID]
		if !ok || (week >= 0 && week <= index) {
			continue
		}
		single := []enrollment.HolidayBookingInput{booking}
		if holidayBookedDayCount(single, written) > holidayBookedDayCount(single, current) {
			week = index
		}
	}
	return max(week, 0)
}

// holidayStartsEarlier reports whether a moved week starts earlier than
// stored; otherwise its end grew it.
func holidayStartsEarlier(week enrollment.HolidayWeekInput, current enrollment.HolidayPeriod) bool {
	before, ok := holidayStoredWeek(current, week.ID)
	return ok && week.StartsOn.Before(before.StartsOn)
}

func holidayStoredWeek(period enrollment.HolidayPeriod, weekID int64) (enrollment.HolidayWeek, bool) {
	for _, week := range period.Weeks {
		if week.ID == weekID {
			return week, true
		}
	}
	return enrollment.HolidayWeek{}, false
}

// fullHolidayDays names the days of each week by days whose places are all
// given (#4701): the parents see them before they choose. It names no child.
// A period without a week by days with a capacity reads nothing.
func (h *HolidayRegistrations) fullHolidayDays(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod) (map[int64][]calendar.Date, error) {
	full := map[int64][]calendar.Date{}
	if !slices.ContainsFunc(period.Weeks, func(week enrollment.HolidayWeek) bool {
		return week.BookingUnit == enrollment.HolidayBookingUnitDay && week.Capacity != nil
	}) {
		return full, nil
	}
	places, err := h.deps.Places.ReadHolidayWeekPlaces(ctx, organizationID, period.ID)
	if err != nil {
		return nil, fmt.Errorf("enrollment: read holiday day places: %w", err)
	}
	for _, week := range places {
		if week.Capacity == nil {
			continue
		}
		for _, day := range week.Days {
			if day.Placed >= *week.Capacity {
				full[week.WeekID] = append(full[week.WeekID], day.Date)
			}
		}
	}
	return full, nil
}

// validateHolidayBookings requires at least one week, each week of the
// period at most once and a time model of that week. A child takes part in
// only one of the weeks that share a day (#5689). An archived time model
// takes no new booking; a booking of kept, the child's bookings so far, may
// stay on it (#5994).
func validateHolidayBookings(bookings []enrollment.HolidayBookingInput, period enrollment.HolidayPeriod, kept []enrollment.HolidayBookingInput) error {
	if len(bookings) == 0 {
		return errors.New("at least one week is required")
	}
	weeks := map[int64]enrollment.HolidayWeek{}
	for _, week := range period.Weeks {
		weeks[week.ID] = week
	}
	errs := validation.Errors{}
	seen := map[int64]bool{}
	for i, booking := range bookings {
		week, ok := weeks[booking.WeekID]
		switch {
		case !ok:
			errs[fmt.Sprint(i)] = errors.New("the week is not a week of this period")
		case seen[booking.WeekID]:
			errs[fmt.Sprint(i)] = errors.New("the week is booked twice")
		case !holidayWeekOffers(week, booking.TimeModelID, holidayBookingKept(kept, booking)):
			errs[fmt.Sprint(i)] = errors.New("the time model is not a bookable time model of this week")
		case holidayBookingOverlaps(week, bookings[:i], weeks):
			errs[fmt.Sprint(i)] = errors.New("the week shares a day with another booked week")
		default:
			if daysErr := validateHolidayBookingDays(week, booking.Days); daysErr != nil {
				errs[fmt.Sprint(i)] = validation.Errors{"days": daysErr}
			}
		}
		seen[booking.WeekID] = true
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func holidayWeekOffers(week enrollment.HolidayWeek, timeModelID int64, kept bool) bool {
	return slices.ContainsFunc(week.TimeModels, func(model enrollment.HolidayTimeModel) bool {
		return model.ID == timeModelID && (!model.Archived || kept)
	})
}

// holidayBookingOverlaps reports whether week shares a day with a week of the
// earlier bookings. A booking of an unknown week is refused on its own.
func holidayBookingOverlaps(week enrollment.HolidayWeek, earlier []enrollment.HolidayBookingInput, weeks map[int64]enrollment.HolidayWeek) bool {
	return slices.ContainsFunc(earlier, func(booking enrollment.HolidayBookingInput) bool {
		other, ok := weeks[booking.WeekID]
		return ok && other.ID != week.ID && holidayDaysOverlap(week.StartsOn, week.EndsOn, other.StartsOn, other.EndsOn)
	})
}
