package timetablehttp

import (
	"context"
	"errors"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The reads the gap check adds for #5343: the Klassentag of the booked
// classes, the clock times of the Regeltermine and their rosters.

// offeringGapClockClasses are the classes of the bookings that can carry a
// care stretch: an offering on school days with a pickup time. Only their
// Klassentag is read.
func offeringGapClockClasses(bookings []timetable.OfferingWeekdayBooking) []string {
	classes := make([]string, 0)
	for _, booking := range bookings {
		class := timetable.OfferingGapClassKey(booking.SchoolClass)
		if booking.ClassDayApplies && len(booking.PickupTimes) > 0 && class != "" && !slices.Contains(classes, class) {
			classes = append(classes, class)
		}
	}
	slices.Sort(classes)
	return classes
}

// offeringGapArrivals reads the Klassentag of the classes that can carry a
// care stretch: the weekly arrival times and the class-wide changes in
// scope. Without such a class it reads nothing.
func (rs *Resource) offeringGapArrivals(ctx context.Context, scope timetable.OfferingGapScope, bookings []timetable.OfferingWeekdayBooking) (timetable.OfferingGapArrivals, error) {
	arrivals := timetable.OfferingGapArrivals{Weekly: map[string]map[int]int{}, Dated: map[string]map[calendar.Date]int{}}
	classes := offeringGapClockClasses(bookings)
	if len(classes) == 0 {
		return arrivals, nil
	}
	if rs.ClassArrivals == nil {
		return arrivals, errors.New("timetable offering gaps: class arrivals not wired")
	}
	weekly, err := rs.ClassArrivals.ListClassArrivalTimes(ctx, classes)
	if err != nil {
		return arrivals, err
	}
	changes, err := rs.ClassArrivals.ListClassArrivalExceptions(ctx, classes, scope.From.String(), scope.Until.String())
	if err != nil {
		return arrivals, err
	}
	arrivals.Weekly = weeklyArrivalMinutes(weekly)
	arrivals.Dated = datedArrivalMinutes(changes)
	return arrivals, nil
}

// weeklyArrivalMinutes keys the weekly arrival times by class key and ISO
// weekday; an unreadable day or time is left out.
func weeklyArrivalMinutes(weekly map[string]map[string]string) map[string]map[int]int {
	result := map[string]map[int]int{}
	for class, days := range weekly {
		key := timetable.OfferingGapClassKey(class)
		for day, hhmm := range days {
			weekday, dayOK := calendar.CanonicalDayToISOWeekday(day)
			minute, clockOK := timetable.ParseOfferingGapClock(hhmm)
			if !dayOK || !clockOK {
				continue
			}
			if result[key] == nil {
				result[key] = map[int]int{}
			}
			result[key][weekday] = minute
		}
	}
	return result
}

// datedArrivalMinutes keys the class-wide changes by class key and date.
func datedArrivalMinutes(changes []timetable.ClassArrivalException) map[string]map[calendar.Date]int {
	result := map[string]map[calendar.Date]int{}
	for _, change := range changes {
		date, err := calendar.ParseDate(change.Date)
		if err != nil {
			continue
		}
		key := timetable.OfferingGapClassKey(change.SchoolClass)
		if result[key] == nil {
			result[key] = map[calendar.Date]int{}
		}
		clock := calendar.WallClockFromTime(change.ArrivalTime)
		result[key][date] = clock.Hour()*60 + clock.Minute()
	}
	return result
}

// offeringGapNeedsAnyRoster reports whether a booking can carry a care
// stretch, so that any Regeltermin may cover it.
func offeringGapNeedsAnyRoster(scope timetable.OfferingGapScope, bookings []timetable.OfferingWeekdayBooking) bool {
	return slices.ContainsFunc(bookings, func(booking timetable.OfferingWeekdayBooking) bool {
		key := timetable.OfferingGapClassKey(booking.SchoolClass)
		return booking.ClassDayApplies && len(booking.PickupTimes) > 0 && (len(scope.Arrivals.Weekly[key]) > 0 || len(scope.Arrivals.Dated[key]) > 0)
	})
}

// attachOfferingGapRosters reads the rosters of the Regeltermine that can
// cover a booking in one batch: every one when allRosters, else those the
// strict day rule admits (no offering source, a known kind).
func (rs *Resource) attachOfferingGapRosters(ctx context.Context, templates []templateResponse, gapTemplates []timetable.OfferingGapTemplate, allRosters bool) error {
	ids := make([]int64, 0, len(templates))
	for i, template := range templates {
		if allRosters || (!gapTemplates[i].HasOfferingSource && gapTemplates[i].Kind != timetable.RosterOfferingAny) {
			ids = append(ids, template.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	if rs.RosterReview == nil {
		return errors.New("timetable offering gaps: roster review not wired")
	}
	rosters, err := rs.RosterReview.PlannedRosters(ctx, ids)
	if err != nil {
		return err
	}
	for i, template := range templates {
		if roster, ok := rosters[template.ID]; ok {
			gapTemplates[i].Roster = &roster
		}
	}
	return nil
}

// offeringGapSchedules are the clock windows of the template's schedules;
// a schedule without readable times is left out, so it covers nothing by
// its time.
func offeringGapSchedules(schedules []templateScheduleResponse) []timetable.OfferingGapSchedule {
	result := make([]timetable.OfferingGapSchedule, 0, len(schedules))
	for _, schedule := range schedules {
		start, startOK := timetable.ParseOfferingGapClock(schedule.StartTime)
		end, endOK := timetable.ParseOfferingGapClock(schedule.EndTime)
		if !startOK || !endOK {
			continue
		}
		window := timetable.OfferingGapSchedule{Weekday: schedule.Weekday, StartMinute: start, EndMinute: end}
		if date, err := calendar.ParseDate(schedule.ValidFrom); err == nil {
			window.ValidFrom = &date
		}
		if date, err := calendar.ParseDate(schedule.ValidUntil); err == nil {
			window.ValidUntil = &date
		}
		result = append(result, window)
	}
	return result
}
