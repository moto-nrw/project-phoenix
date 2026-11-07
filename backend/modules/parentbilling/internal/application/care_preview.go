package application

import (
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// weekdayOrder sorts the booked weekdays from Monday to Sunday.
var weekdayOrder = map[string]int{"mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6, "sun": 7}

// careMonthInput is everything one month of the monthly run is computed
// from: the versions in force, the price list, the contracts an issued
// invoice of the month already bills and the recipients.
type careMonthInput struct {
	month      calendar.Date
	rule       string
	versions   []parentbilling.CareContractVersion
	tariffs    []parentbilling.CareTariff
	invoiced   map[int64]bool
	recipients map[int64]parentbilling.CareRecipient
}

// careMonth is the preview of one month: the draft invoices, the bookings
// without a price and the count of contracts already billed.
type careMonth struct {
	invoices []parentbilling.CareInvoice
	unpriced []parentbilling.CareUnpriced
	billed   int
}

func (m careMonth) total() int64 {
	var total int64
	for _, invoice := range m.invoices {
		total += invoice.TotalCents
	}
	return total
}

// monthEnd is the last day of the month that starts on first.
func monthEnd(first calendar.Date) calendar.Date {
	return calendar.NewDate(first.Year(), first.Month()+1, 1).AddDays(-1)
}

// firstOfMonth is the first day of the month of day.
func firstOfMonth(day calendar.Date) calendar.Date {
	return calendar.NewDate(day.Year(), day.Month(), 1)
}

// buildCareMonth bills every contract of the month by the rule of the
// Träger. A contract an issued invoice of the month bills stays out; so
// does one with a booked weekday without a price, which Unpriced names. Two
// versions of one contract on one day are ErrCareVersionsOverlap: the run
// would otherwise bill a day twice.
func buildCareMonth(in careMonthInput) (careMonth, error) {
	byContract := map[int64][]parentbilling.CareContractVersion{}
	order := []int64{}
	for _, version := range in.versions {
		if _, seen := byContract[version.ContractID]; !seen {
			order = append(order, version.ContractID)
		}
		byContract[version.ContractID] = append(byContract[version.ContractID], version)
	}
	prices := newPriceBook(in.tariffs)
	result := careMonth{invoices: []parentbilling.CareInvoice{}, unpriced: []parentbilling.CareUnpriced{}}
	for _, contractID := range order {
		versions := byContract[contractID]
		if err := checkOneVersionPerDay(versions); err != nil {
			return careMonth{}, err
		}
		if in.invoiced[contractID] {
			result.billed++
			continue
		}
		invoice, unpriced := billContract(in.month, in.rule, versions, prices)
		if len(unpriced) > 0 {
			result.unpriced = append(result.unpriced, unpriced...)
			continue
		}
		if len(invoice.Positions) == 0 {
			continue
		}
		invoice.Recipient = in.recipients[contractID]
		result.invoices = append(result.invoices, invoice)
	}
	slices.SortStableFunc(result.invoices, func(a, b parentbilling.CareInvoice) int {
		return firstNonZero(compareOrdered(a.SchoolName, b.SchoolName), compareOrdered(a.ChildLastName, b.ChildLastName),
			compareOrdered(a.ChildFirstName, b.ChildFirstName), compareOrdered(a.ContractID, b.ContractID))
	})
	return result, nil
}

// checkOneVersionPerDay refuses versions of one contract whose days meet.
func checkOneVersionPerDay(versions []parentbilling.CareContractVersion) error {
	sorted := slices.Clone(versions)
	slices.SortFunc(sorted, func(a, b parentbilling.CareContractVersion) int { return a.From.Compare(b.From) })
	for i := 1; i < len(sorted); i++ {
		if !sorted[i].From.After(sorted[i-1].Until) {
			return parentbilling.ErrCareVersionsOverlap
		}
	}
	return nil
}

// billedSegment is a version with the days of the month it bills.
type billedSegment struct {
	version parentbilling.CareContractVersion
	from    calendar.Date
	until   calendar.Date
}

// segmentsOf picks the versions the rule bills: by the version on the
// first day (or the first version of a contract that starts in the month)
// for the full month, or every version for its own days.
func segmentsOf(month calendar.Date, rule string, versions []parentbilling.CareContractVersion) []billedSegment {
	sorted := slices.Clone(versions)
	slices.SortFunc(sorted, func(a, b parentbilling.CareContractVersion) int { return a.From.Compare(b.From) })
	if rule == parentbilling.MonthRuleProRata {
		segments := make([]billedSegment, len(sorted))
		for i, version := range sorted {
			segments[i] = billedSegment{version: version, from: version.From, until: version.Until}
		}
		return segments
	}
	return []billedSegment{{version: sorted[0], from: month, until: monthEnd(month)}}
}

// billContract computes the draft invoice of one contract, or the booked
// weekdays without a price.
func billContract(month calendar.Date, rule string, versions []parentbilling.CareContractVersion, prices priceBook) (parentbilling.CareInvoice, []parentbilling.CareUnpriced) {
	head := versions[0]
	invoice := parentbilling.CareInvoice{
		ContractID: head.ContractID, SchoolID: head.SchoolID, SchoolName: head.SchoolName,
		ChildFirstName: head.ChildFirstName, ChildLastName: head.ChildLastName, Positions: []parentbilling.CarePosition{},
	}
	daysInMonth := month.DaysUntil(monthEnd(month)) + 1
	var unpriced []parentbilling.CareUnpriced
	for _, segment := range segmentsOf(month, rule, versions) {
		for _, module := range modulesOf(segment, rule) {
			position := parentbilling.CarePosition{
				CareOfferingID: module.offeringID, ModuleName: module.name, VersionNumber: segment.version.Number,
				ServiceFrom: segment.from, ServiceUntil: module.until, Weekdays: []parentbilling.CareWeekdayPrice{},
			}
			for _, weekday := range module.weekdays {
				price, ok := prices.on(head.SchoolID, module.offeringID, weekday, month)
				if !ok {
					unpriced = append(unpriced, parentbilling.CareUnpriced{
						ContractID: head.ContractID, SchoolID: head.SchoolID, SchoolName: head.SchoolName,
						ChildFirstName: head.ChildFirstName, ChildLastName: head.ChildLastName,
						CareOfferingID: module.offeringID, ModuleName: module.name, Weekday: weekday,
					})
					continue
				}
				amount := price
				if rule == parentbilling.MonthRuleProRata {
					amount = shareOfMonth(price, segment.from.DaysUntil(module.until)+1, daysInMonth)
				}
				position.Weekdays = append(position.Weekdays, parentbilling.CareWeekdayPrice{Weekday: weekday, MonthlyCents: price, AmountCents: amount})
				position.AmountCents += amount
			}
			invoice.Positions = append(invoice.Positions, position)
			invoice.TotalCents += position.AmountCents
		}
	}
	return invoice, unpriced
}

// shareOfMonth is the part of a monthly price for days of a month with
// daysInMonth days, rounded half up to the cent.
func shareOfMonth(monthlyCents int64, days, daysInMonth int) int64 {
	if days >= daysInMonth {
		return monthlyCents
	}
	return (monthlyCents*int64(days)*2 + int64(daysInMonth)) / (2 * int64(daysInMonth))
}

// bookedModule is one module of a segment with its booked weekdays and the
// last day it bills.
type bookedModule struct {
	offeringID int64
	name       string
	endsAt     string
	weekdays   []string
	until      calendar.Date
}

// modulesOf groups the services of a segment by module, in the order of
// their time band (end time), so early care comes first. Under the full month a module counts while it is
// booked on the first billed day; pro rata it bills up to its last booked
// day.
func modulesOf(segment billedSegment, rule string) []bookedModule {
	byOffering := map[int64]*bookedModule{}
	var modules []*bookedModule
	for _, service := range segment.version.Services {
		until, booked := serviceUntil(segment, rule, service)
		if !booked {
			continue
		}
		module, ok := byOffering[service.CareOfferingID]
		if !ok {
			module = &bookedModule{offeringID: service.CareOfferingID, name: service.ModuleName, endsAt: service.EndsAt, until: until}
			byOffering[service.CareOfferingID] = module
			modules = append(modules, module)
		}
		if until.After(module.until) {
			module.until = until
		}
		if !slices.Contains(module.weekdays, service.Weekday) {
			module.weekdays = append(module.weekdays, service.Weekday)
		}
	}
	result := make([]bookedModule, len(modules))
	for i, module := range modules {
		slices.SortFunc(module.weekdays, func(a, b string) int { return compareOrdered(weekdayOrder[a], weekdayOrder[b]) })
		result[i] = *module
	}
	slices.SortStableFunc(result, func(a, b bookedModule) int {
		return firstNonZero(compareOrdered(a.endsAt, b.endsAt), compareOrdered(a.name, b.name))
	})
	return result
}

// serviceUntil is the last day a service bills in its segment, and whether
// it bills at all: a module whose last booked day lies before the segment
// does not; pro rata it stops after its last booked day.
func serviceUntil(segment billedSegment, rule string, service parentbilling.CareContractService) (calendar.Date, bool) {
	if service.LastDay == nil {
		return segment.until, true
	}
	if service.LastDay.Before(segment.from) {
		return calendar.Date(""), false
	}
	if rule == parentbilling.MonthRuleProRata && service.LastDay.Before(segment.until) {
		return *service.LastDay, true
	}
	return segment.until, true
}

// priceKey names the price of one weekday of a module at a school.
type priceKey struct {
	schoolID   int64
	offeringID int64
	weekday    string
}

// priceBook holds the prices of every key by start, oldest first.
type priceBook map[priceKey][]parentbilling.CareTariff

func newPriceBook(tariffs []parentbilling.CareTariff) priceBook {
	book := priceBook{}
	for _, tariff := range tariffs {
		key := priceKey{tariff.SchoolID, tariff.CareOfferingID, tariff.Weekday}
		book[key] = append(book[key], tariff)
	}
	for key := range book {
		slices.SortFunc(book[key], func(a, b parentbilling.CareTariff) int { return a.ValidFrom.Compare(b.ValidFrom) })
	}
	return book
}

// on is the monthly price that holds on day: the latest that starts on or
// before it.
func (b priceBook) on(schoolID, offeringID int64, weekday string, day calendar.Date) (int64, bool) {
	tariffs := b[priceKey{schoolID, offeringID, weekday}]
	for i := len(tariffs) - 1; i >= 0; i-- {
		if !tariffs[i].ValidFrom.After(day) {
			return tariffs[i].MonthlyCents, true
		}
	}
	return 0, false
}

// withValidity sets the last day of every price: the day before the next
// price of the same key.
func withValidity(tariffs []parentbilling.CareTariff) []parentbilling.CareTariff {
	book := newPriceBook(tariffs)
	result := make([]parentbilling.CareTariff, 0, len(tariffs))
	for _, list := range book {
		for i := range list {
			if i+1 < len(list) {
				until := list[i+1].ValidFrom.AddDays(-1)
				list[i].ValidUntil = &until
			}
			result = append(result, list[i])
		}
	}
	slices.SortFunc(result, func(a, b parentbilling.CareTariff) int {
		return firstNonZero(compareOrdered(a.SchoolID, b.SchoolID), a.ValidFrom.Compare(b.ValidFrom),
			compareOrdered(a.ModuleName, b.ModuleName), compareOrdered(weekdayOrder[a.Weekday], weekdayOrder[b.Weekday]))
	})
	return result
}

// isMonthStart reports a first day of a month.
func isMonthStart(day calendar.Date) bool { return !day.IsZero() && day.Day() == 1 }

// compareOrdered orders two values ascending.
func compareOrdered[T int | int64 | string](a, b T) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// firstNonZero is the first decisive comparison.
func firstNonZero(comparisons ...int) int {
	for _, comparison := range comparisons {
		if comparison != 0 {
			return comparison
		}
	}
	return 0
}
