package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Which booking carries a Regeltermin (#5342). The booked care of an offering
// runs until the child is picked up, so a booking carries every Regeltermin
// of its weekday that ends no later than the offering's pickup time. A
// Ganztag picked up at 14:30 carries the Randstunde of the class until 12:45,
// like the Randstunde booking itself. A care Regeltermin needs an offering
// that counts as care; other Regeltermine take any offering.
//
// When an offering sets no pickup time or the Regeltermin has no end, the
// times cannot decide. Schools book that way, with a Randstunde and Ganztag
// variants on the same day, so such a booking proves nothing: the day then
// keeps the earlier rule and counts as lost only when the change drops more
// offerings on it than it newly books.

// templateDayEnds holds, per template and ISO weekday, the latest known end
// in minutes after midnight. A missing entry means no known end.
type templateDayEnds map[int64]map[int]int

// addClock records an "HH:MM" or "HH:MM:SS" end; an unreadable one is no
// known end.
func (e templateDayEnds) addClock(templateID int64, weekday int, clock string) {
	if minutes, ok := clockMinutes(clock); ok {
		e.add(templateID, weekday, minutes)
	}
}

func (e templateDayEnds) add(templateID int64, weekday, minutes int) {
	if e[templateID] == nil {
		e[templateID] = make(map[int]int)
	}
	if current, known := e[templateID][weekday]; !known || minutes > current {
		e[templateID][weekday] = minutes
	}
}

// merge takes the template's end on the weekday from another collection.
func (e templateDayEnds) merge(templateID int64, weekday int, other templateDayEnds) {
	if minutes, known := other[templateID][weekday]; known {
		e.add(templateID, weekday, minutes)
	}
}

// scheduleEndedBefore reports whether the schedule's exclusive valid_until
// lies on or before the date. An unreadable date keeps the schedule.
func scheduleEndedBefore(sch timetable.Schedule, from calendar.Date) bool {
	if sch.ValidUntil == nil {
		return false
	}
	until, err := calendar.ParseDate(*sch.ValidUntil)
	return err == nil && !until.After(from)
}

func clockMinutes(clock string) (int, bool) {
	for _, layout := range []string{"15:04:05", "15:04"} {
		if parsed, err := time.Parse(layout, clock); err == nil {
			return parsed.Hour()*60 + parsed.Minute(), true
		}
	}
	return 0, false
}

// scheduleEnds reads the end of each template's schedule per weekday from
// the timeframes the schedules use. A schedule that ended before the date
// plans nothing the change touches and gives no end.
func (s *StudentTemplateDays) scheduleEnds(ctx context.Context, schedules []timetable.Schedule, from calendar.Date) (templateDayEnds, error) {
	ends := make(templateDayEnds)
	reaching := make([]timetable.Schedule, 0, len(schedules))
	wanted := make(map[int64]bool)
	for _, sch := range schedules {
		if scheduleEndedBefore(sch, from) {
			continue
		}
		reaching = append(reaching, sch)
		if sch.TimeframeID != nil {
			wanted[*sch.TimeframeID] = true
		}
	}
	schedules = reaching
	if len(wanted) == 0 {
		return ends, nil
	}
	timeframes, err := s.deps.Timeframes.ListTimeframes(ctx, timetable.TimeframeFilter{})
	if err != nil {
		return nil, fmt.Errorf("student template days: list timeframes: %w", err)
	}
	endByID := make(map[int64]string, len(wanted))
	for _, timeframe := range timeframes {
		if wanted[timeframe.ID] && timeframe.EndTime != nil {
			endByID[timeframe.ID] = *timeframe.EndTime
		}
	}
	for _, sch := range schedules {
		if sch.TimeframeID != nil {
			ends.addClock(sch.ActivityGroupID, sch.Weekday, endByID[*sch.TimeframeID])
		}
	}
	return ends, nil
}

// bookingCarry decides, with the bookings after the change, which weekdays
// of a hand-maintained Regeltermin keep a booking.
type bookingCarry struct {
	bookings []timetable.StudentDayBooking
	netLoss  map[int]bool
}

func newBookingCarry(query timetable.ManualStudentPlanQuery) (bookingCarry, error) {
	carry := bookingCarry{bookings: query.Bookings, netLoss: make(map[int]bool, len(query.NetLossWeekdays))}
	for _, booking := range query.Bookings {
		if !isISOWeekday(booking.Weekday) {
			return carry, fmt.Errorf("%w: booking weekday %d is not an ISO weekday", timetable.ErrStudentTemplateDaysInvalid, booking.Weekday)
		}
	}
	for _, weekday := range query.NetLossWeekdays {
		if !isISOWeekday(weekday) {
			return carry, fmt.Errorf("%w: net-loss weekday %d is not an ISO weekday", timetable.ErrStudentTemplateDaysInvalid, weekday)
		}
		carry.netLoss[weekday] = true
	}
	return carry, nil
}

func isISOWeekday(weekday int) bool {
	return weekday >= timetable.WeekdayMonday && weekday <= timetable.WeekdaySunday
}

// dropCarriedDays removes the weekdays a booking carries from days and
// returns the templates that keep a weekday.
func (c bookingCarry) dropCarriedDays(groups []timetable.Group, days templateDays, ends templateDayEnds) []timetable.Group {
	kept := make([]timetable.Group, 0, len(groups))
	for _, group := range groups {
		for weekday := range days[group.ID] {
			end, known := ends[group.ID][weekday]
			if c.keeps(weekday, group.Type == timetable.GroupTypeCare, end, known) {
				delete(days[group.ID], weekday)
			}
		}
		if len(days[group.ID]) == 0 {
			delete(days, group.ID)
			continue
		}
		kept = append(kept, group)
	}
	return kept
}

// keeps reports whether the Regeltermin keeps a booking on the weekday: a
// booking carries it by its pickup time, or the times cannot decide and the
// day is no net loss.
func (c bookingCarry) keeps(weekday int, care bool, end int, endKnown bool) bool {
	switch carriedBy(c.bookings, weekday, care, end, endKnown) {
	case carried:
		return true
	case undecided:
		return !c.netLoss[weekday]
	}
	return false
}

type carryState int

const (
	notCarried carryState = iota
	undecided
	carried
)

// carriedBy compares the Regeltermin's end (minutes after midnight, if
// known) with the pickup time of each fitting booking of the weekday.
func carriedBy(bookings []timetable.StudentDayBooking, weekday int, care bool, end int, endKnown bool) carryState {
	state := notCarried
	for _, booking := range bookings {
		if booking.Weekday != weekday || (care && !booking.CountsAsCare) {
			continue
		}
		pickup, pickupKnown := clockMinutes(booking.PickupTime)
		if !pickupKnown || !endKnown {
			state = undecided
			continue
		}
		if end <= pickup {
			return carried
		}
	}
	return state
}
