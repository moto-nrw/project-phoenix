package application

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	schedule "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Which booking carries a Regeltermin (#5342). The booked care of an offering
// runs until the child is picked up, so a booking carries a Regeltermin of
// its weekday that ends no later than the offering's pickup time, when the
// offering fits the Regeltermin's kind (RosterOfferingKindFor: a Mensa list
// takes offerings with lunch, a care block care offerings, an AG course
// offerings). A Ganztag picked up at 14:30 carries the Randstunde of the class
// until 12:45, like the Randstunde booking itself.
//
// The times decide only when the Regeltermin has an end and every booking of
// the day has a pickup time. Otherwise the day keeps the earlier rule
// exactly, whatever the kinds: the Regeltermin counts as lost when the change
// drops more offerings on the day than it newly books. Schools without pickup
// times therefore see the earlier notices.
//
// A student may hold further approved enrollments the change leaves as they
// are (#5366). Where the times decide, their bookings carry a Regeltermin
// like the changed enrollment's, but only with a known pickup time: an
// unchanged booking is no part of the change's balance, so it never moves a
// day to the earlier rule and never carries without its time.
//
// Only schedules that are valid on a date of their weekday from the date on
// name weekdays and ends, by materialization's own validity rule
// (scheduleValidOn).

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

func clockMinutes(clock string) (int, bool) {
	for _, layout := range []string{"15:04:05", "15:04"} {
		if parsed, err := time.Parse(layout, clock); err == nil {
			return parsed.Hour()*60 + parsed.Minute(), true
		}
	}
	return 0, false
}

// runningSchedules keeps the schedules that are valid on a date of their
// weekday from the date on for at least one hand enrollment. Schedule and
// enrollment validity, including their selected calendar period, must overlap.
func (s *StudentTemplateDays) runningSchedules(ctx context.Context, schedules []timetable.Schedule, from calendar.Date, rows []*activityContract.StudentEnrollment) ([]timetable.Schedule, error) {
	if len(schedules) == 0 {
		return schedules, nil
	}
	templates, err := listLegacyGroups(ctx, s.deps.Groups, timetable.GroupFilter{IDs: scheduleTemplateIDs(schedules)}, "student template days: list schedule templates")
	if err != nil {
		return nil, err
	}
	periods, err := s.deps.Periods.FindActiveByTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("student template days: list calendar periods: %w", err)
	}
	byID := make(map[int64]*activityContract.Group, len(templates))
	for _, template := range templates {
		byID[template.ID] = template
	}
	running := make([]timetable.Schedule, 0, len(schedules))
	for _, sch := range schedules {
		if template := byID[sch.ActivityGroupID]; template != nil && scheduleRunsFrom(template, legacySchedule(sch), from, periods, rows) {
			running = append(running, sch)
		}
	}
	return running, nil
}

// scheduleRunsFrom reports whether the schedule is valid on a date of its
// weekday within the projection horizon from the date on and plans at least
// one enrollment on that same date.
func scheduleRunsFrom(template *activityContract.Group, sch *activityContract.Schedule, from calendar.Date, periods []*schedule.CalendarPeriod, rows []*activityContract.StudentEnrollment) bool {
	last := from.AddDays(timetable.MaxOccurrenceProjectionDays)
	for date := timetable.NextWeekdayOnOrAfter(from, sch.Weekday); !date.After(last); date = date.AddDays(7) {
		period, skip := scheduleValidOn(template, sch, date, periods, nil)
		switch skip {
		case candidateKept:
			for _, row := range rows {
				if row.ActivityGroupID == sch.ActivityGroupID && isEnrollmentValidOn(row, date, period.ID, sch.Weekday) {
					return true
				}
			}
		case candidateEnded:
			return false
		}
	}
	return false
}

func scheduleTemplateIDs(schedules []timetable.Schedule) []int64 {
	ids := make([]int64, 0, len(schedules))
	for _, sch := range schedules {
		ids = append(ids, sch.ActivityGroupID)
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// scheduleEnds reads the end of each template's schedule per weekday from
// the timeframes the schedules use.
func (s *StudentTemplateDays) scheduleEnds(ctx context.Context, schedules []timetable.Schedule) (templateDayEnds, error) {
	ends := make(templateDayEnds)
	wanted := make(map[int64]bool)
	for _, sch := range schedules {
		if sch.TimeframeID != nil {
			wanted[*sch.TimeframeID] = true
		}
	}
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
	bookings, others []timetable.StudentDayBooking
	netLoss          map[int]bool
}

func newBookingCarry(query timetable.ManualStudentPlanQuery) (bookingCarry, error) {
	carry := bookingCarry{bookings: query.Bookings, others: query.OtherBookings, netLoss: make(map[int]bool, len(query.NetLossWeekdays))}
	for _, booking := range slices.Concat(query.Bookings, query.OtherBookings) {
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
		kind := timetable.RosterOfferingKindFor(group.Type, group.ListKind)
		for weekday := range days[group.ID] {
			end, known := ends[group.ID][weekday]
			if c.keeps(weekday, kind, end, known) {
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

// keeps reports whether the Regeltermin keeps a booking on the weekday. With
// an end and every pickup time of the changed enrollment's day known, a
// fitting booking of any enrollment must reach the end; otherwise the day
// keeps it unless it is a net loss.
func (c bookingCarry) keeps(weekday int, kind timetable.RosterOfferingKind, end int, endKnown bool) bool {
	if !endKnown || !c.pickupsKnown(weekday) {
		return !c.netLoss[weekday]
	}
	for _, booking := range slices.Concat(c.bookings, c.others) {
		if booking.Weekday != weekday || !kind.AdmitsOffering(booking.CountsAsCare, booking.IncludesLunch) {
			continue
		}
		if pickup, known := clockMinutes(booking.PickupTime); known && end <= pickup {
			return true
		}
	}
	return false
}

// pickupsKnown reports whether every booking of the changed enrollment on
// the weekday has a pickup time.
func (c bookingCarry) pickupsKnown(weekday int) bool {
	for _, booking := range c.bookings {
		if _, known := clockMinutes(booking.PickupTime); booking.Weekday == weekday && !known {
			return false
		}
	}
	return true
}
