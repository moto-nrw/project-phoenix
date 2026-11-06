package application

import (
	"slices"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The timeline of a contract's versions (#4681, PR body "Zeitleiste der
// Versionen"). A change on day D concerns the offerings whose booking starts
// or ends on D; that is how the school's approval writes it. Its version is
// the version in force on D plus this change: the services of that version
// stay, only the changed offerings take their booking from D on.
//
//  1. No approval gets lost: every change holds from D until another version
//     starts.
//  2. D inside a version that starts before D: a new version from D; the
//     earlier one ends on D-1 by the next effective date.
//  3. D = E, the first day of a version (planned, or from an earlier change
//     on the same D): that version is replaced by one from D with its
//     services plus the change and a new number; it becomes ersetzt.
//  4. E > D: a version that starts later stays as it is; it does not take
//     the change over.
//
// A D before today holds from today, unless a version started since then.

// amendmentPlan is the next version of a contract and the version it
// replaces (rule 3), if any.
type amendmentPlan struct {
	version carecontract.Version
	replace *carecontract.Version
}

// change is a day on which bookings change: origin is the day of the
// booking boundary, day the day the change holds from (origin, or today).
type change struct {
	origin, day calendar.Date
}

// nextAmendment is the first change of the contract that its timeline does
// not carry yet; nil without a reason when it carries them all.
func nextAmendment(read schoolRead, application carecontract.Application, contract AmendableContract,
	today calendar.Date) (*amendmentPlan, string) {
	for _, next := range changeDays(read, application, contract, today) {
		plan, reason := planChange(read, application, contract, next)
		if plan != nil || reason != "" {
			return plan, reason
		}
	}
	return nil, ""
}

// planChange is the version a change makes, or nil when the timeline
// carries it already.
func planChange(read schoolRead, application carecontract.Application, contract AmendableContract, next change) (*amendmentPlan, string) {
	current, inForce := versionAt(contract.Versions, next.day)
	base := []carecontract.Service{}
	if inForce {
		base = promised(current, next.day)
	}
	booked := servicesAt(read, application, next.day)
	services := withChange(base, booked.services, changedOfferings(read, application, next.origin))
	if sameCare(services, base) {
		return nil, ""
	}
	if len(services) == 0 {
		return nil, carecontract.SkipCareEnds
	}
	template := contract.Versions[0]
	if inForce {
		template = current
	}
	ends := endsOn(template.Terms, contract.FirstFrom, booked.until)
	if ends != nil && ends.Before(next.day) {
		return nil, carecontract.SkipCareEnds
	}
	version := carecontract.Version{
		Number: contract.NextNumber, Status: carecontract.VersionEffective, EffectiveFrom: next.day, EndsOn: ends,
		ChildFirstName: template.ChildFirstName, ChildLastName: template.ChildLastName, ChildBirthDate: template.ChildBirthDate,
		SchoolName: template.SchoolName, PhaseName: firstText(booked.phaseName, template.PhaseName),
		Services: boundServices(services, ends), PriceCents: singleModulePrice(read, services), Terms: template.Terms,
		Source: carecontract.SourceBookingChange,
	}
	plan := &amendmentPlan{version: version}
	if inForce && current.EffectiveFrom == next.day {
		replaced := current
		plan.replace, plan.version.Status = &replaced, current.Status
	} else if _, later := versionAfter(contract.Versions, next.day); later {
		plan.version.Status = carecontract.VersionSuperseded
	}
	return plan, ""
}

// withChange is base with the changed offerings taken from the bookings:
// every service of another offering stays, the changed offerings take what
// the bookings say on the day.
func withChange(base, booked []carecontract.Service, changed map[int64]bool) []carecontract.Service {
	services := []carecontract.Service{}
	for _, service := range base {
		if !changed[service.CareOfferingID] {
			services = append(services, service)
		}
	}
	for _, service := range booked {
		if changed[service.CareOfferingID] {
			services = append(services, service)
		}
	}
	sortServices(services)
	return services
}

// changedOfferings are the offerings whose booking starts or ends on day.
func changedOfferings(read schoolRead, application carecontract.Application, day calendar.Date) map[int64]bool {
	changed := map[int64]bool{}
	for _, booking := range application.Bookings {
		offering, ok := read.offerings[booking.CareOfferingID]
		if !ok {
			continue
		}
		clipped, ok := bookingWithinPhase(booking, offering, calendar.Date("0001-01-01"))
		if ok && (sameDay(clipped.ValidFrom, day) || sameDay(clipped.ValidUntil, day)) {
			changed[booking.CareOfferingID] = true
		}
	}
	return changed
}

func sameDay(date *calendar.Date, day calendar.Date) bool { return date != nil && *date == day }

// singleModulePrice keeps the monthly price of a version with one module,
// as the first version does (ADR 0153); several modules have none.
func singleModulePrice(read schoolRead, services []carecontract.Service) *int64 {
	offerings := []int64{}
	for _, service := range services {
		if !slices.Contains(offerings, service.CareOfferingID) {
			offerings = append(offerings, service.CareOfferingID)
		}
	}
	if len(offerings) != 1 {
		return nil
	}
	price := read.offerings[offerings[0]].PriceCents
	if price == nil {
		return nil
	}
	cents := int64(*price)
	return &cents
}

// versionAt is the effective version in force on day: the latest start on or
// before it; false before the first version.
func versionAt(timeline []carecontract.Version, day calendar.Date) (carecontract.Version, bool) {
	var current carecontract.Version
	found := false
	for _, version := range timeline {
		if version.EffectiveFrom.After(day) {
			break
		}
		current, found = version, true
	}
	return current, found
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

// changeDays are the changes of the bookings: every day a booking starts or
// ends. Only a fixed term ends the contract itself; changes after it belong
// to the termination (#4682). A day before today holds from today, since no
// version reaches into the past, unless a version started since then: that
// version settled the day.
func changeDays(read schoolRead, application carecontract.Application, contract AmendableContract, today calendar.Date) []change {
	window := changeWindow{today: today, timeline: contract.Versions,
		termEnd: endsOn(contract.Versions[0].Terms, contract.FirstFrom, nil)}
	changes := []change{}
	for _, booking := range application.Bookings {
		offering, ok := read.offerings[booking.CareOfferingID]
		if !ok {
			continue
		}
		clipped, ok := bookingWithinPhase(booking, offering, calendar.Date("0001-01-01"))
		if !ok {
			continue
		}
		for _, origin := range []*calendar.Date{clipped.ValidFrom, clipped.ValidUntil} {
			if next, ok := window.change(origin); ok && !slices.Contains(changes, next) {
				changes = append(changes, next)
			}
		}
	}
	slices.SortFunc(changes, func(a, b change) int {
		return firstNonZero(a.day.Compare(b.day), a.origin.Compare(b.origin))
	})
	return changes
}

// changeWindow bounds the days a change may hold from: not before today,
// not after a fixed term.
type changeWindow struct {
	today    calendar.Date
	termEnd  *calendar.Date
	timeline []carecontract.Version
}

// change is the change of a booking boundary on origin, or false outside
// the window. A past origin holds from today, unless a version started
// between it and today.
func (w changeWindow) change(origin *calendar.Date) (change, bool) {
	if origin == nil || origin.IsZero() {
		return change{}, false
	}
	day := *origin
	if day.Before(w.today) {
		if started, ok := versionAfter(w.timeline, day); ok && !started.EffectiveFrom.After(w.today) {
			return change{}, false
		}
		day = w.today
	}
	if w.termEnd != nil && day.After(*w.termEnd) {
		return change{}, false
	}
	return change{origin: *origin, day: day}, true
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
