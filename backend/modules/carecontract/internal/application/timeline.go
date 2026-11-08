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
// The offerings a change concerns: their booking starts or ends on D, their
// promised end (last day, end of the version) is D-1, or no booking runs
// for them on D any more on a promised weekday; a version never promises unbooked care (#5928),
// which goes before rule 4. A D before today holds from today, unless a
// version started since then; all such overdue changes hold together.

// amendmentPlan is the next version of a contract and the version it
// replaces (rule 3), if any.
type amendmentPlan struct {
	version carecontract.Version
	replace *carecontract.Version
}

// change is a day a change holds from, with the days it comes from: the
// day itself, or for overdue changes every past day they happened on.
type change struct {
	day     calendar.Date
	origins []calendar.Date
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
	services := withChange(base, booked.services, changedOfferings(read, application, contract.Versions, next, base, booked.services))
	if len(services) == 0 && len(booked.services) > 0 {
		// Nothing the version promised is booked any more, but care goes on:
		// the version takes the continuing bookings, never an empty promise.
		services = booked.services
	}
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

// changedOfferings are the offerings the change concerns: a booking starts
// or ends on one of its days, a version's promise for them ends the day
// before, or no booking runs on a promised weekday of an offering on the
// change's day.
func changedOfferings(read schoolRead, application carecontract.Application, timeline []carecontract.Version,
	next change, base, booked []carecontract.Service) map[int64]bool {
	changed := map[int64]bool{}
	for _, origin := range next.origins {
		for _, booking := range application.Bookings {
			if clipped, ok := clippedBooking(read, booking); ok && (sameDay(clipped.ValidFrom, origin) || sameDay(clipped.ValidUntil, origin)) {
				changed[booking.CareOfferingID] = true
			}
		}
		for _, offering := range endedPromises(timeline, origin) {
			changed[offering] = true
		}
	}
	// A promised weekday of an offering without a running booking on it:
	// the offering takes its bookings, so the cancelled day drops out.
	for _, service := range base {
		if !slices.ContainsFunc(booked, func(b carecontract.Service) bool {
			return b.CareOfferingID == service.CareOfferingID && b.Weekday == service.Weekday
		}) {
			changed[service.CareOfferingID] = true
		}
	}
	return changed
}

// endedPromises are the offerings whose promise in a version ends the day
// before day: a module's last day, or the end of the whole version.
func endedPromises(timeline []carecontract.Version, day calendar.Date) []int64 {
	offerings := []int64{}
	for _, version := range timeline {
		ended := version.EndsOn != nil && version.EndsOn.AddDays(1) == day
		for _, service := range version.Services {
			if ended || (service.LastDay != nil && service.LastDay.AddDays(1) == day) {
				offerings = append(offerings, service.CareOfferingID)
			}
		}
	}
	return offerings
}

// clippedBooking is the booking within its offering's phase, ended ones
// included; false for an offering that is no care or an empty window.
func clippedBooking(read schoolRead, booking carecontract.Booking) (carecontract.Booking, bool) {
	offering, ok := read.offerings[booking.CareOfferingID]
	if !ok {
		return booking, false
	}
	return bookingWithinPhase(booking, offering, calendar.Date("0001-01-01"))
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

func versionAfter(timeline []carecontract.Version, day calendar.Date) (carecontract.Version, bool) {
	for _, version := range timeline {
		if version.EffectiveFrom.After(day) {
			return version, true
		}
	}
	return carecontract.Version{}, false
}

// changeDays are the days something changes: a booking starts or ends, a
// version starts, or a promise in a version ends the day before. Only a
// fixed term ends the contract itself; changes after it belong to the
// termination (#4682). A day before today holds from today, since no version
// reaches into the past, unless a version started since then: that version
// settled the day. Changes on the same day hold together.
func changeDays(read schoolRead, application carecontract.Application, contract AmendableContract, today calendar.Date) []change {
	window := changeWindow{today: today, timeline: contract.Versions,
		termEnd: endsOn(contract.Versions[0].Terms, contract.FirstFrom, nil)}
	byDay := map[calendar.Date][]calendar.Date{}
	for _, origin := range changeOrigins(read, application, contract.Versions) {
		if day, ok := window.day(origin); ok && !slices.Contains(byDay[day], *origin) {
			byDay[day] = append(byDay[day], *origin)
		}
	}
	changes := make([]change, 0, len(byDay))
	for day, origins := range byDay {
		slices.SortFunc(origins, func(a, b calendar.Date) int { return a.Compare(b) })
		changes = append(changes, change{day: day, origins: origins})
	}
	slices.SortFunc(changes, func(a, b change) int { return a.day.Compare(b.day) })
	return changes
}

// changeOrigins are the days something changes, before the window.
func changeOrigins(read schoolRead, application carecontract.Application, timeline []carecontract.Version) []*calendar.Date {
	origins := []*calendar.Date{}
	for _, booking := range application.Bookings {
		if clipped, ok := clippedBooking(read, booking); ok {
			origins = append(origins, clipped.ValidFrom, clipped.ValidUntil)
		}
	}
	for _, version := range timeline {
		start := version.EffectiveFrom
		origins = append(origins, &start, dayAfter(version.EndsOn))
		for _, service := range version.Services {
			origins = append(origins, dayAfter(service.LastDay))
		}
	}
	return origins
}

func dayAfter(day *calendar.Date) *calendar.Date {
	if day == nil {
		return nil
	}
	next := day.AddDays(1)
	return &next
}

// changeWindow bounds the days a change may hold from: not before today,
// not after a fixed term.
type changeWindow struct {
	today    calendar.Date
	termEnd  *calendar.Date
	timeline []carecontract.Version
}

// day is the day a change on origin holds from, or false outside the
// window. A past origin holds from today, unless a version started between
// it and today.
func (w changeWindow) day(origin *calendar.Date) (calendar.Date, bool) {
	if origin == nil || origin.IsZero() {
		return "", false
	}
	day := *origin
	if day.Before(w.today) {
		if started, ok := versionAfter(w.timeline, day); ok && !started.EffectiveFrom.After(w.today) {
			return "", false
		}
		day = w.today
	}
	if w.termEnd != nil && day.After(*w.termEnd) {
		return "", false
	}
	return day, true
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
