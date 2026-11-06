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
		if !sameCare(servicesAt(read, application, latest.EffectiveFrom).services, activeServices(latest.Services, latest.EffectiveFrom)) {
			return nil, carecontract.SkipBeforeLaterVersion
		}
	}
	for _, day := range changeDays(read, application, latest, today) {
		booked := servicesAt(read, application, day)
		if sameCare(booked.services, activeServices(latest.Services, day)) {
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
// or the version's promise can change: the start and end of every booking
// and the day after each module's last day in the version. A day before
// today counts as today, since no version reaches into the past. Days after
// the version's end belong to the termination (#4682).
func changeDays(read schoolRead, application carecontract.Application, latest carecontract.Version, today calendar.Date) []calendar.Date {
	candidates := []calendar.Date{}
	add := func(day *calendar.Date) {
		if day == nil || day.IsZero() {
			return
		}
		candidate := *day
		if candidate.Before(today) {
			candidate = today
		}
		if !candidate.After(latest.EffectiveFrom) || (latest.EndsOn != nil && candidate.After(*latest.EndsOn)) {
			return
		}
		if !slices.Contains(candidates, candidate) {
			candidates = append(candidates, candidate)
		}
	}
	for _, booking := range application.Bookings {
		offering, ok := read.offerings[booking.CareOfferingID]
		if !ok {
			continue
		}
		clipped, ok := bookingWithinPhase(booking, offering, today)
		if !ok {
			continue
		}
		add(clipped.ValidFrom)
		add(clipped.ValidUntil)
	}
	for _, service := range latest.Services {
		if service.LastDay != nil {
			next := service.LastDay.AddDays(1)
			add(&next)
		}
	}
	slices.SortFunc(candidates, func(a, b calendar.Date) int { return a.Compare(b) })
	return candidates
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
