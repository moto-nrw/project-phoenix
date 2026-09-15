package application

import (
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// reviewPlanFacts is bounded to one child and the dates used by their request.
// Stored care-offering pickup rows are obsolete materializations, not facts.
type reviewPlanFacts struct {
	arrivals       []careplan.ArrivalSchedule
	pickups        []careplan.PickupSchedule
	classTimes     map[string]string
	bookings       []reviewPlanSelection
	authoritative  bool
	departureModes map[string][]string
}

type reviewPlanSelection struct {
	offering     careplan.CareOffering
	selectedDays []string
	from         careplan.Date
	until        careplan.Date
}

var reviewWeekdays = [...]string{"mon", "tue", "wed", "thu", "fri"}

func (f reviewPlanFacts) weekly(today careplan.Date) (careplan.CareWeeklyPlan, error) {
	plan := careplan.CareWeeklyPlan{
		ArrivalDays: map[int]bool{}, ArrivalTimes: map[int]string{},
		DepartureModes: f.departureModes,
	}
	stored := make(map[int]careplan.ArrivalSchedule, len(f.arrivals))
	for _, row := range f.arrivals {
		stored[row.Weekday] = row
	}
	start := today.StartOfISOWeek()
	for i, key := range reviewWeekdays {
		weekday := i + 1
		row, hasStored := stored[weekday]
		careDay := hasStored
		if f.authoritative {
			careDay = f.booked(start.AddDays(i), key)
		}
		if !careDay {
			continue
		}
		plan.ArrivalDays[weekday] = true
		if hasStored && !row.ExpectedArrival.IsZero() {
			plan.ArrivalTimes[weekday] = row.ExpectedArrival.Format("15:04")
		} else if parsed, err := time.Parse("15:04", f.classTimes[key]); err == nil {
			plan.ArrivalTimes[weekday] = parsed.Format("15:04")
		}
	}
	var err error
	plan.PickupTimes, err = f.pickupWeek(today)
	return plan, err
}

func (f reviewPlanFacts) booked(date careplan.Date, day string) bool {
	for _, booking := range f.bookings {
		if !reviewBookingCovers(booking, date) {
			continue
		}
		offering := booking.offering
		if !offering.IsActive || !offering.CountsAsCare {
			continue
		}
		for _, candidate := range reviewBookingDays(booking, offering) {
			if strings.ToLower(strings.TrimSpace(candidate)) == day {
				return true
			}
		}
	}
	return false
}

func (f reviewPlanFacts) pickupWeek(date careplan.Date) (map[int]string, error) {
	times := map[int]string{}
	for _, booking := range f.bookings {
		if !reviewBookingCovers(booking, date) {
			continue
		}
		offering := booking.offering
		if !offering.IsActive || !offering.CountsAsCare {
			continue
		}
		for _, day := range reviewBookingDays(booking, offering) {
			key := strings.ToLower(strings.TrimSpace(day))
			weekday := reviewWeekday(key)
			value := offering.PickupTimes[key]
			if weekday == 0 || value == "" {
				continue
			}
			parsed, err := time.Parse("15:04", value)
			if err != nil {
				return nil, fmt.Errorf("project pickup baselines: offering %d has invalid pickup time %q for %s: %w", offering.ID, value, key, err)
			}
			clock := parsed.Format("15:04")
			if clock > times[weekday] {
				times[weekday] = clock
			}
		}
	}
	for _, row := range f.pickups {
		if row.Source == careplan.ScheduleSourceCareOffering || row.Weekday < 1 || row.Weekday > 5 {
			continue
		}
		if f.authoritative && !f.booked(date, reviewWeekdays[row.Weekday-1]) {
			continue
		}
		times[row.Weekday] = row.PickupTime.Format("15:04")
	}
	return times, nil
}

func reviewBookingCovers(booking reviewPlanSelection, date careplan.Date) bool {
	return (booking.from.IsZero() || !date.Before(booking.from)) &&
		(booking.until.IsZero() || date.Before(booking.until))
}

func reviewBookingDays(booking reviewPlanSelection, offering careplan.CareOffering) []string {
	if len(booking.selectedDays) > 0 || offering.DaysOfWeekMode != "fixed" {
		return booking.selectedDays
	}
	return offering.AvailableDays
}

func reviewWeekday(key string) int {
	for i, day := range reviewWeekdays {
		if key == day {
			return i + 1
		}
	}
	return 0
}
