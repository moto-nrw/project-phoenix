package application

import (
	"slices"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The timeline of a contract's versions (#4681). An active contract follows
// its child's bookings: when the booked care on a day after the newest
// version differs from what that version promises, the contract gets a new
// version from that day.

// nextAmendment is the version that brings the contract in line with the
// bookings: at the first day after its newest version, and not before
// today, on which the booked care differs from what that version promises.
// Nil without a reason when the bookings still match.
func nextAmendment(read schoolRead, application carecontract.Application, contract AmendableContract,
	today calendar.Date) (*carecontract.Version, string) {
	latest := contract.Latest
	if !latest.EffectiveFrom.Before(today) {
		// The newest version is still ahead: it must match the bookings on
		// its own first day, or a change came in before it.
		if !sameCare(servicesAt(read, application, latest.EffectiveFrom).services, promised(latest, latest.EffectiveFrom)) {
			return nil, carecontract.SkipBeforeLaterVersion
		}
	}
	for _, day := range changeDays(read, application, contract, today) {
		booked := servicesAt(read, application, day)
		if sameCare(booked.services, promised(latest, day)) {
			continue
		}
		if len(booked.services) == 0 {
			return nil, carecontract.SkipCareEnds
		}
		ends := endsOn(latest.Terms, contract.FirstFrom, booked.until)
		if ends != nil && ends.Before(day) {
			return nil, carecontract.SkipCareEnds
		}
		return &carecontract.Version{
			Number: contract.NextNumber, Status: carecontract.VersionEffective, EffectiveFrom: day, EndsOn: ends,
			ChildFirstName: latest.ChildFirstName, ChildLastName: latest.ChildLastName, ChildBirthDate: latest.ChildBirthDate,
			SchoolName: latest.SchoolName, PhaseName: firstText(booked.phaseName, latest.PhaseName),
			Services: boundServices(booked.services, ends), PriceCents: booked.price, Terms: latest.Terms,
			Source: carecontract.SourceBookingChange,
		}, ""
	}
	return nil, ""
}

// changeDays are the days after the newest version on which the booked care
// or the version's promise can change: the start and end of every booking,
// the day after each module's last day and the day after the version's end.
// A version that ends because its module ends is followed by the module the
// child switches to (ADR 0153: "ein späterer Wechsel des Moduls kommt mit
// einer neuen Version"); only a fixed term ends the contract itself, and
// days after it belong to the termination (#4682). A day before today counts
// as today, since no version reaches into the past.
func changeDays(read schoolRead, application carecontract.Application, contract AmendableContract, today calendar.Date) []calendar.Date {
	latest := contract.Latest
	window := changeWindow{after: latest.EffectiveFrom, today: today, termEnd: endsOn(latest.Terms, contract.FirstFrom, nil)}
	// Today is always a candidate: a change whose booking boundary already
	// passed (a module that ended before the run saw it) shows on today.
	days := []*calendar.Date{&today, dayAfter(latest.EndsOn)}
	for _, booking := range application.Bookings {
		if offering, ok := read.offerings[booking.CareOfferingID]; ok {
			if clipped, ok := bookingWithinPhase(booking, offering, today); ok {
				days = append(days, clipped.ValidFrom, clipped.ValidUntil)
			}
		}
	}
	for _, service := range latest.Services {
		days = append(days, dayAfter(service.LastDay))
	}
	candidates := []calendar.Date{}
	for _, day := range days {
		if candidate, ok := window.candidate(day); ok && !slices.Contains(candidates, candidate) {
			candidates = append(candidates, candidate)
		}
	}
	slices.SortFunc(candidates, func(a, b calendar.Date) int { return a.Compare(b) })
	return candidates
}

// changeWindow bounds the days a new version may start on: after the newest
// version, not before today, not after a fixed term.
type changeWindow struct {
	after, today calendar.Date
	termEnd      *calendar.Date
}

// candidate moves a day before today to today and keeps it when it lies in
// the window.
func (w changeWindow) candidate(day *calendar.Date) (calendar.Date, bool) {
	if day == nil || day.IsZero() {
		return "", false
	}
	candidate := *day
	if candidate.Before(w.today) {
		candidate = w.today
	}
	if !candidate.After(w.after) || (w.termEnd != nil && candidate.After(*w.termEnd)) {
		return "", false
	}
	return candidate, true
}

func dayAfter(day *calendar.Date) *calendar.Date {
	if day == nil {
		return nil
	}
	next := day.AddDays(1)
	return &next
}

// servicesAt is the care the child books on one day: every booking that
// runs on it, with its module's end time per weekday and its last day.
func servicesAt(read schoolRead, application carecontract.Application, day calendar.Date) booked {
	running := carecontract.Application{Bookings: []carecontract.Booking{}}
	for _, booking := range application.Bookings {
		offering, ok := read.offerings[booking.CareOfferingID]
		if !ok {
			continue
		}
		clipped, ok := bookingWithinPhase(booking, offering, day)
		if ok && (clipped.ValidFrom == nil || !clipped.ValidFrom.After(day)) {
			running.Bookings = append(running.Bookings, clipped)
		}
	}
	return bookedCare(read, running, day)
}

// promised is what the version promises on a day: nothing after its end,
// else its services that still run.
func promised(version carecontract.Version, day calendar.Date) []carecontract.Service {
	if version.EndsOn != nil && day.After(*version.EndsOn) {
		return []carecontract.Service{}
	}
	return activeServices(version.Services, day)
}

// activeServices are the services of a version that still run on a day:
// a module past its last day drops out.
func activeServices(services []carecontract.Service, day calendar.Date) []carecontract.Service {
	active := []carecontract.Service{}
	for _, service := range services {
		if service.LastDay == nil || !service.LastDay.Before(day) {
			active = append(active, service)
		}
	}
	return active
}

// sameCare compares the care of two service lists per weekday, module and
// end time. Names and last days do not count: a renamed module is the same
// care, and the last day is checked by the day after it.
func sameCare(a, b []carecontract.Service) bool {
	key := func(services []carecontract.Service) []string {
		keys := make([]string, len(services))
		for i, service := range services {
			keys[i] = service.Weekday + "|" + strconv.FormatInt(service.CareOfferingID, 10) + "|" + service.EndsAt
		}
		slices.Sort(keys)
		return slices.Compact(keys)
	}
	return slices.Equal(key(a), key(b))
}
