package application

import (
	"slices"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// booked is the care one application child books at its school: the
// services per weekday and module, the modules, and the dates of the
// bookings behind them.
type booked struct {
	services  []carecontract.Service
	offerings []int64
	phaseName string
	price     *int64
	unpriced  bool
	starts    []*calendar.Date
	until     *calendar.Date
}

// bookedCare collects the current bookings of the child's care offerings
// into services: per booked weekday the module with its end time on that
// day. Offerings that are no care (an AG, the lunch) are not in read and
// drop out. A weekday may carry several modules (early and afternoon care).
func bookedCare(read schoolRead, application carecontract.Application, today calendar.Date) booked {
	result := booked{services: []carecontract.Service{}}
	open := false
	for _, booking := range concurrentBookings(currentBookings(application.Bookings, read.offerings, today), today) {
		offering, ok := read.offerings[booking.CareOfferingID]
		if !ok {
			continue
		}
		days := effectiveBookingDays(booking, offering)
		added := false
		for _, day := range weekdayOrder {
			if !slices.Contains(days, day) {
				continue
			}
			result.services = append(result.services, carecontract.Service{
				Weekday: day, CareOfferingID: offering.ID, ModuleName: offering.Name, EndsAt: offering.EndTimes[day],
				LastDay: lastDay(booking.ValidUntil),
			})
			added = true
		}
		if !added {
			continue
		}
		result.offerings = append(result.offerings, offering.ID)
		result.addPrice(offering.PriceCents)
		result.phaseName = firstText(result.phaseName, offering.PhaseName)
		result.starts = append(result.starts, booking.ValidFrom)
		switch {
		case booking.ValidUntil == nil:
			open = true
		case result.until == nil || booking.ValidUntil.After(*result.until):
			result.until = booking.ValidUntil
		}
	}
	if open {
		result.until = nil
	}
	if result.unpriced {
		result.price = nil
	}
	sortServices(result.services)
	return result
}

// effectiveBookingDays resolves the fixed schedule even when the booking
// stores no selection; a choice offering keeps only its selected days.
func effectiveBookingDays(booking carecontract.Booking, offering carecontract.Offering) []string {
	if len(booking.Days) == 0 && offering.DaysOfWeekMode == "fixed" {
		return offering.AvailableDays
	}
	return booking.Days
}

// addPrice keeps the monthly price of the module. A contract over several
// modules has no price yet: an offering's price is for the whole month on
// its days, and adding them up would overcharge a family that books each
// module on a few weekdays only. The tariffs per weekday and module of #4684
// price such contracts.
func (b *booked) addPrice(cents *int) {
	if cents == nil || b.price != nil || len(b.offerings) > 1 {
		b.unpriced = true
		return
	}
	price := int64(*cents)
	b.price = &price
}

// lastDay is the inclusive last day of a booking whose end is exclusive.
func lastDay(until *calendar.Date) *calendar.Date {
	if until == nil {
		return nil
	}
	last := until.AddDays(-1)
	return &last
}

// boundServices keeps a module's last day only where it ends before the
// version: a module that runs as long as the version needs no own end.
func boundServices(services []carecontract.Service, versionEnd *calendar.Date) []carecontract.Service {
	bound := make([]carecontract.Service, len(services))
	for i, service := range services {
		if service.LastDay != nil && versionEnd != nil && !service.LastDay.Before(*versionEnd) {
			service.LastDay = nil
		}
		bound[i] = service
	}
	return bound
}

// sortServices orders the services by weekday, then by end time, so early
// care comes before the afternoon.
func sortServices(services []carecontract.Service) {
	slices.SortStableFunc(services, func(a, b carecontract.Service) int {
		return firstNonZero(slices.Index(weekdayOrder, a.Weekday)-slices.Index(weekdayOrder, b.Weekday),
			compareEnds(a.EndsAt, b.EndsAt), strings.Compare(a.ModuleName, b.ModuleName))
	})
}

func compareEnds(a, b string) int {
	switch {
	case a == b:
		return 0
	case a == "":
		return 1
	case b == "":
		return -1
	}
	return strings.Compare(a, b)
}

// effectiveFrom is the date the contract takes effect: the earliest start of
// its bookings, else the activation of the child, else the start of the
// service of the phase, else today.
func (b booked) effectiveFrom(application carecontract.Application, today calendar.Date) calendar.Date {
	// The bookings all run on one day (concurrentBookings); the version
	// holds from the latest of their starts, when every module applies.
	var latest *calendar.Date
	for _, start := range b.starts {
		if start != nil && !start.IsZero() && (latest == nil || start.After(*latest)) {
			latest = start
		}
	}
	for _, date := range []*calendar.Date{latest, application.ActivateOn, application.ServiceStartDate} {
		if date != nil && !date.IsZero() {
			return *date
		}
	}
	return today
}

// concurrentBookings keeps the bookings that run on one reference day: the
// start of the earliest booking, or today when that has passed. A module
// that starts only later, a switch from one module to another for instance,
// is not part of this version; a later version adds it (#4681). Without this,
// a contract would promise two modules over the whole period that the child
// books one after the other.
func concurrentBookings(bookings []carecontract.Booking, today calendar.Date) []carecontract.Booking {
	reference := today
	if earliest := earliestStart(bookings); earliest != nil && earliest.After(today) {
		reference = *earliest
	}
	result := make([]carecontract.Booking, 0, len(bookings))
	for _, booking := range bookings {
		if booking.ValidFrom != nil && booking.ValidFrom.After(reference) {
			continue
		}
		if booking.ValidUntil != nil && !booking.ValidUntil.After(reference) {
			continue
		}
		result = append(result, booking)
	}
	return result
}

// earliestStart is the first start of the bookings; nil when one has none
// (it runs already).
func earliestStart(bookings []carecontract.Booking) *calendar.Date {
	var earliest *calendar.Date
	for _, booking := range bookings {
		if booking.ValidFrom == nil {
			return nil
		}
		if earliest == nil || booking.ValidFrom.Before(*earliest) {
			earliest = booking.ValidFrom
		}
	}
	return earliest
}

// currentBookings keeps per offering the booking that holds today or starts
// later, the earliest of them; ended bookings drop out.
func currentBookings(bookings []carecontract.Booking, offerings map[int64]carecontract.Offering, today calendar.Date) []carecontract.Booking {
	chosen := map[int64]carecontract.Booking{}
	order := []int64{}
	for _, booking := range bookings {
		offering, ok := offerings[booking.CareOfferingID]
		if !ok {
			continue
		}
		booking, ok = bookingWithinPhase(booking, offering, today)
		if !ok {
			continue
		}
		previous, seen := chosen[booking.CareOfferingID]
		if !seen {
			order = append(order, booking.CareOfferingID)
		}
		if !seen || startsBefore(booking.ValidFrom, previous.ValidFrom) {
			chosen[booking.CareOfferingID] = booking
		}
	}
	result := make([]carecontract.Booking, 0, len(order))
	for _, id := range order {
		result = append(result, chosen[id])
	}
	return result
}

// bookingWithinPhase intersects the half-open booking with the offering's
// phase, whose service end is inclusive. Empty and ended windows drop out.
func bookingWithinPhase(booking carecontract.Booking, offering carecontract.Offering, today calendar.Date) (carecontract.Booking, bool) {
	if offering.ServiceStart != nil && (booking.ValidFrom == nil || booking.ValidFrom.Before(*offering.ServiceStart)) {
		booking.ValidFrom = offering.ServiceStart
	}
	if offering.ServiceEnd != nil {
		until := offering.ServiceEnd.AddDays(1)
		if booking.ValidUntil == nil || booking.ValidUntil.After(until) {
			booking.ValidUntil = &until
		}
	}
	if booking.ValidUntil != nil && (!booking.ValidUntil.After(today) || (booking.ValidFrom != nil && !booking.ValidFrom.Before(*booking.ValidUntil))) {
		return booking, false
	}
	return booking, true
}

func startsBefore(a, b *calendar.Date) bool {
	switch {
	case a == nil:
		return b != nil
	case b == nil:
		return false
	}
	return a.Before(*b)
}

// endsOn is the last day of a contract: the day before its last booking
// ends, or the end of a fixed term when that comes first. An open or
// school-year term ends with the bookings, if at all.
func endsOn(terms carecontract.TemplateInput, from calendar.Date, until *calendar.Date) *calendar.Date {
	var end *calendar.Date
	if until != nil {
		last := until.AddDays(-1)
		end = &last
	}
	if terms.TermKind == carecontract.TermFixedMonths && terms.TermMonths != nil {
		last := fixedTermEnd(from, *terms.TermMonths)
		if end == nil || last.Before(*end) {
			end = &last
		}
	}
	if end != nil && end.Before(from) {
		return nil
	}
	return end
}

// templateIndex holds the conditions of every offering by school and
// offering.
type templateIndex map[[2]int64]carecontract.TemplateInput

// terms are the conditions of a contract over its modules. Every module
// needs conditions, and all of them must agree: one contract has one term,
// one notice period and one statute.
func (t templateIndex) terms(schoolID int64, offerings []int64) (carecontract.TemplateInput, string) {
	var terms *carecontract.TemplateInput
	for _, offering := range offerings {
		found, ok := t[[2]int64{schoolID, offering}]
		if !ok {
			return carecontract.TemplateInput{}, carecontract.SkipTemplateMissing
		}
		if terms != nil && !sameTerms(*terms, found) {
			return carecontract.TemplateInput{}, carecontract.SkipTemplatesDiffer
		}
		terms = &found
	}
	if terms == nil {
		return carecontract.TemplateInput{}, carecontract.SkipTemplateMissing
	}
	return *terms, ""
}

func sameTerms(a, b carecontract.TemplateInput) bool {
	if (a.TermMonths == nil) != (b.TermMonths == nil) || (a.TermMonths != nil && *a.TermMonths != *b.TermMonths) {
		return false
	}
	a.TermMonths, b.TermMonths = nil, nil
	return a == b
}
