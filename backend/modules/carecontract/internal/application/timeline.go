package application

import (
	"slices"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The timeline of a contract's versions (#4681). An active contract follows
// its child's bookings: when the booked care on a day differs from what the
// version in force on that day promises, the contract gets a new version from
// that day. A change before a version that is already planned goes in front
// of it: the new version holds from its day to the day before the planned
// one, and the planned version stays as it is. Whether the planned version
// has to take the change over is not decided by #4681; it does not happen
// automatically.

// nextAmendment is the version that brings the contract in line with the
// bookings on the first change day, or nil with the reason why none can be
// made. Nil without a reason when the bookings still match.
func nextAmendment(read schoolRead, application carecontract.Application, contract AmendableContract,
	today calendar.Date) (*carecontract.Version, string) {
	reason := ""
	for _, day := range changeDays(read, application, contract, today) {
		current, starts := versionAt(contract.Versions, day)
		booked := servicesAt(read, application, day)
		if sameCare(booked.services, promised(current, day)) {
			continue
		}
		if starts {
			// A planned version starts on this very day; it stays as it is.
			reason = carecontract.SkipBeforeLaterVersion
			continue
		}
		if version := amendmentVersion(contract, current, booked, day); version != nil {
			return version, ""
		}
		return nil, carecontract.SkipCareEnds
	}
	return nil, reason
}

// amendmentVersion is the new version from day with the booked care,
// following current, the version in force before it; nil when the care ends
// on that day. It is wirksam as the newest version, and abgeloest when a
// planned version starts later: then it holds until the day before that one.
func amendmentVersion(contract AmendableContract, current carecontract.Version, booked booked, day calendar.Date) *carecontract.Version {
	if len(booked.services) == 0 {
		return nil
	}
	ends := endsOn(current.Terms, contract.FirstFrom, booked.until)
	if ends != nil && ends.Before(day) {
		return nil
	}
	status := carecontract.VersionEffective
	if _, ok := versionAfter(contract.Versions, day); ok {
		status = carecontract.VersionSuperseded
	}
	return &carecontract.Version{
		Number: contract.NextNumber, Status: status, EffectiveFrom: day, EndsOn: ends,
		ChildFirstName: current.ChildFirstName, ChildLastName: current.ChildLastName, ChildBirthDate: current.ChildBirthDate,
		SchoolName: current.SchoolName, PhaseName: firstText(booked.phaseName, current.PhaseName),
		Services: boundServices(booked.services, ends), PriceCents: booked.price, Terms: current.Terms,
		Source: carecontract.SourceBookingChange,
	}
}

// versionAt is the effective version in force on day (the latest start on
// or before it) and whether it starts on that very day. The timeline is
// ordered by effective date and day lies after its first start.
func versionAt(timeline []carecontract.Version, day calendar.Date) (carecontract.Version, bool) {
	current := timeline[0]
	for _, version := range timeline[1:] {
		if version.EffectiveFrom.After(day) {
			break
		}
		current = version
	}
	return current, current.EffectiveFrom == day
}

// versionAfter is the first effective version that starts after day.
func versionAfter(timeline []carecontract.Version, day calendar.Date) (carecontract.Version, bool) {
	for _, version := range timeline {
		if version.EffectiveFrom.After(day) {
			return version, true
		}
	}
	return carecontract.Version{}, false
}

// changeDays are the days on which the booked care or a version's promise
// can change: the start and end of every booking, the day after each
// module's last day and the day after each version's end. A version that
// ends because its module ends is followed by the module the child switches
// to (ADR 0153: "ein späterer Wechsel des Moduls kommt mit einer neuen
// Version"); only a fixed term ends the contract itself, and days after it
// belong to the termination (#4682). A day before today counts as today,
// since no version reaches into the past, unless a version started since:
// that version settled the day.
func changeDays(read schoolRead, application carecontract.Application, contract AmendableContract, today calendar.Date) []calendar.Date {
	window := changeWindow{after: contract.FirstFrom, today: today, timeline: contract.Versions,
		termEnd: endsOn(contract.Latest.Terms, contract.FirstFrom, nil)}
	days := []*calendar.Date{}
	for _, booking := range application.Bookings {
		if offering, ok := read.offerings[booking.CareOfferingID]; ok {
			// Bookings that ended since the contract began count too: their
			// end is a change even when the run sees it late.
			if clipped, ok := bookingWithinPhase(booking, offering, contract.FirstFrom); ok {
				days = append(days, clipped.ValidFrom, clipped.ValidUntil)
			}
		}
	}
	for _, version := range contract.Versions {
		days = append(days, dayAfter(version.EndsOn))
		for _, service := range version.Services {
			days = append(days, dayAfter(service.LastDay))
		}
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

// changeWindow bounds the days a new version may start on: after the first
// version, not before today, not after a fixed term.
type changeWindow struct {
	after, today calendar.Date
	termEnd      *calendar.Date
	timeline     []carecontract.Version
}

// candidate keeps a day in the window. A day before today moves to today,
// unless a version started between it and today.
func (w changeWindow) candidate(day *calendar.Date) (calendar.Date, bool) {
	if day == nil || day.IsZero() {
		return "", false
	}
	candidate := *day
	if candidate.Before(w.today) {
		if started, ok := versionAfter(w.timeline, candidate); ok && !started.EffectiveFrom.After(w.today) {
			return "", false
		}
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
