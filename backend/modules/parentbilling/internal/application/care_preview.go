package application

import (
	"maps"
	"slices"
	"strconv"

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
	invoiced   map[int64]parentbilling.CareInvoice
	recipients map[int64]parentbilling.CareRecipient
	// rules and evidences feed the rule chain (#4685); without them every
	// invoice is its base price.
	rules     []parentbilling.ContributionRule
	evidences []parentbilling.CareEvidence
	// costShares split the contribution with the cost bearers (#4687).
	costShares []parentbilling.CareCostShare
}

// careMonth is the preview of one month: the draft invoices, the bookings
// without a price, the children with duplicate contracts and the count of
// contracts already billed.
type careMonth struct {
	// dueDates are the run's due dates the payments used (#4689).
	dueDates parentbilling.RunDueDates
	// deviations are the family deviations the invoices took (#4689).
	deviations     []parentbilling.FamilyBilling
	invoices       []parentbilling.CareInvoice
	unpriced       []parentbilling.CareUnpriced
	duplicates     []parentbilling.CareDuplicate
	billed         int
	billedBySchool map[int64]int
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
// does one with a booked weekday without a price, which Unpriced names, and
// one of a child whose contracts book a weekday of a module twice, which
// Duplicates names. Two versions of one contract on one day are
// ErrCareVersionsOverlap: the run would otherwise bill a day twice.
func buildCareMonth(in careMonthInput) (careMonth, error) {
	byContract, order := versionsByContract(in.versions)
	for _, contractID := range order {
		if err := checkOneVersionPerDay(byContract[contractID]); err != nil {
			return careMonth{}, err
		}
	}
	duplicates, held := childMonthHolds(in.month, in.rule, byContract, order)
	entries := monthEntries(in.month, byContract, order)
	prices := newPriceBook(in.tariffs)
	result := careMonth{invoices: []parentbilling.CareInvoice{}, unpriced: []parentbilling.CareUnpriced{}, duplicates: duplicates}
	var chained []*careChainEntry
	childOf := childKeys(byContract, order)
	for _, contractID := range order {
		if _, issued := in.invoiced[contractID]; issued || held[contractID] {
			continue
		}
		invoice, unpriced := billContract(in.month, in.rule, byContract[contractID], entries[contractID], prices)
		if len(unpriced) > 0 {
			result.unpriced = append(result.unpriced, unpriced...)
			continue
		}
		if len(invoice.Positions) == 0 {
			continue
		}
		invoice.Recipient = in.recipients[contractID]
		invoice.FamilyEmail = parentbilling.FamilyKey(invoice.Recipient.Email)
		result.invoices = append(result.invoices, invoice)
		chained = append(chained, &careChainEntry{child: childOf[contractID], birthDate: byContract[contractID][0].ChildBirthDate})
	}
	// result.invoices no longer grows: the entries point into it.
	for i, entry := range chained {
		entry.invoice = &result.invoices[i]
	}
	chained = append(chained, issuedCareEntries(in.invoiced, childOf, byContract)...)
	result.billed = len(in.invoiced)
	result.billedBySchool = make(map[int64]int)
	for _, invoice := range in.invoiced {
		result.billedBySchool[invoice.SchoolID]++
	}
	applyCareChain(chained, careChainInput{month: in.month, rules: in.rules, evidences: in.evidences, costShares: in.costShares})
	slices.SortStableFunc(result.invoices, func(a, b parentbilling.CareInvoice) int {
		return firstNonZero(compareOrdered(a.SchoolName, b.SchoolName), compareOrdered(a.ChildLastName, b.ChildLastName),
			compareOrdered(a.ChildFirstName, b.ChildFirstName), compareOrdered(a.ContractID, b.ContractID))
	})
	return result, nil
}

// issuedCareEntries keeps earlier invoices independent of current pricing.
func issuedCareEntries(invoiced map[int64]parentbilling.CareInvoice, childOf map[int64]string,
	byContract map[int64][]parentbilling.CareContractVersion,
) []*careChainEntry {
	var entries []*careChainEntry
	// Issued invoices keep their saved recipient, base prices and net amounts,
	// even if current contracts, tariffs, rules or evidences have changed.
	for _, id := range slices.Sorted(maps.Keys(invoiced)) {
		invoice := invoiced[id]
		child := childOf[id]
		if child == "" {
			child = "contract:" + strconv.FormatInt(id, 10)
		}
		entry := &careChainEntry{invoice: &invoice, child: child, issued: true}
		if versions := byContract[id]; len(versions) > 0 {
			entry.birthDate = versions[0].ChildBirthDate
		}
		entries = append(entries, entry)
	}
	return entries
}

// versionsByContract groups the versions by contract, each by start, and
// keeps the order of the contracts.
func versionsByContract(versions []parentbilling.CareContractVersion) (map[int64][]parentbilling.CareContractVersion, []int64) {
	byContract := map[int64][]parentbilling.CareContractVersion{}
	order := []int64{}
	for _, version := range versions {
		if _, seen := byContract[version.ContractID]; !seen {
			order = append(order, version.ContractID)
		}
		byContract[version.ContractID] = append(byContract[version.ContractID], version)
	}
	for _, contractID := range order {
		slices.SortFunc(byContract[contractID], func(a, b parentbilling.CareContractVersion) int { return a.From.Compare(b.From) })
	}
	return byContract, order
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

// versionOn is the version of a contract that holds on day, if any.
func versionOn(versions []parentbilling.CareContractVersion, day calendar.Date) (parentbilling.CareContractVersion, bool) {
	for _, version := range versions {
		if !day.Before(version.From) && !day.After(version.Until) {
			return version, true
		}
	}
	return parentbilling.CareContractVersion{}, false
}

func weekdayOf(day calendar.Date) string {
	return [...]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}[day.Weekday()]
}

// moduleRun is a stretch of days a module is booked on the same weekdays.
type moduleRun struct {
	from, until calendar.Date
	version     int
	weekdays    []string
}

func (r moduleRun) days() int { return r.from.DaysUntil(r.until) + 1 }

// moduleTrack is one module of a contract with its runs in the month.
type moduleTrack struct {
	offeringID int64
	name       string
	endsAt     string
	runs       []moduleRun
}

// tracksOf follows every module of a contract through the month (E2 of
// ADR 0158, Nachtrag Kind-Monat). Under the full month a contract in force
// on the first day bills the whole month with every module its version on
// that day books. A contract that starts later bills the whole month by its
// first version only when it is its child's entry of the month (see
// monthEntries); else it bills nothing. Pro rata a module's booked weekdays are read day by day;
// days with the same weekdays form one run, also across versions, so a
// module that does not change is one run (Betragsregel 2).
func tracksOf(month calendar.Date, rule string, versions []parentbilling.CareContractVersion, entry bool) []moduleTrack {
	if rule != parentbilling.MonthRuleProRata {
		version, ok := versionOn(versions, month)
		switch {
		case ok:
			return fullMonthTracks(month, version, month)
		case entry:
			return fullMonthTracks(month, versions[0], versions[0].From)
		}
		return nil
	}
	tracks := map[int64]*moduleTrack{}
	var order []int64
	for day := month; !day.After(monthEnd(month)); day = day.AddDays(1) {
		version, ok := versionOn(versions, day)
		if !ok {
			continue
		}
		for offeringID, module := range modulesBookedOn(version, day) {
			track, seen := tracks[offeringID]
			if !seen {
				track = &moduleTrack{offeringID: offeringID, name: module.name, endsAt: module.endsAt}
				tracks[offeringID] = track
				order = append(order, offeringID)
			}
			track.extend(day, version.Number, module.weekdays)
		}
	}
	return sortedTracks(tracks, order)
}

// extend adds a day to the track: the last run grows when the day follows
// it on the same weekdays, else a new run starts.
func (t *moduleTrack) extend(day calendar.Date, version int, weekdays []string) {
	if n := len(t.runs); n > 0 {
		last := &t.runs[n-1]
		if last.until.AddDays(1) == day && slices.Equal(last.weekdays, weekdays) {
			last.until = day
			return
		}
	}
	t.runs = append(t.runs, moduleRun{from: day, until: day, version: version, weekdays: weekdays})
}

// bookedModule is what a version books of one module on a day.
type bookedModule struct {
	name     string
	endsAt   string
	weekdays []string
}

// modulesBookedOn lists the modules a version books on day with all their
// booked weekdays: a module counts from its first to its last booked day.
func modulesBookedOn(version parentbilling.CareContractVersion, day calendar.Date) map[int64]bookedModule {
	modules := map[int64]bookedModule{}
	for _, service := range version.Services {
		if service.LastDay != nil && service.LastDay.Before(day) {
			continue
		}
		module := modules[service.CareOfferingID]
		module.name, module.endsAt = service.ModuleName, service.EndsAt
		if !slices.Contains(module.weekdays, service.Weekday) {
			module.weekdays = append(module.weekdays, service.Weekday)
		}
		modules[service.CareOfferingID] = module
	}
	for offeringID, module := range modules {
		slices.SortFunc(module.weekdays, func(a, b string) int { return compareOrdered(weekdayOrder[a], weekdayOrder[b]) })
		modules[offeringID] = module
	}
	return modules
}

// fullMonthTracks books the whole month by one version: every module it
// books on the first day it bills from.
func fullMonthTracks(month calendar.Date, version parentbilling.CareContractVersion, first calendar.Date) []moduleTrack {
	tracks := map[int64]*moduleTrack{}
	var order []int64
	for offeringID, module := range modulesBookedOn(version, first) {
		tracks[offeringID] = &moduleTrack{offeringID: offeringID, name: module.name, endsAt: module.endsAt,
			runs: []moduleRun{{from: month, until: monthEnd(month), version: version.Number, weekdays: module.weekdays}}}
		order = append(order, offeringID)
	}
	return sortedTracks(tracks, order)
}

// sortedTracks orders the modules by their time band, so early care comes
// first.
func sortedTracks(tracks map[int64]*moduleTrack, order []int64) []moduleTrack {
	result := make([]moduleTrack, 0, len(order))
	for _, offeringID := range order {
		result = append(result, *tracks[offeringID])
	}
	slices.SortStableFunc(result, func(a, b moduleTrack) int {
		return firstNonZero(compareOrdered(a.endsAt, b.endsAt), compareOrdered(a.name, b.name), compareOrdered(a.offeringID, b.offeringID))
	})
	return result
}

// billContract computes the draft invoice of one contract, or the booked
// weekdays without a price.
func billContract(month calendar.Date, rule string, versions []parentbilling.CareContractVersion, entry bool, prices priceBook) (parentbilling.CareInvoice, []parentbilling.CareUnpriced) {
	head := versions[0]
	invoice := parentbilling.CareInvoice{
		ContractID: head.ContractID, SchoolID: head.SchoolID, SchoolName: head.SchoolName,
		ChildFirstName: head.ChildFirstName, ChildLastName: head.ChildLastName, StudentID: studentOf(versions),
		Positions: []parentbilling.CarePosition{},
	}
	daysInMonth := month.DaysUntil(monthEnd(month)) + 1
	var unpriced []parentbilling.CareUnpriced
	for _, track := range tracksOf(month, rule, versions, entry) {
		missing := missingPrices(head, track, prices, month)
		if len(missing) > 0 {
			unpriced = append(unpriced, missing...)
			continue
		}
		positions := trackPositions(head.SchoolID, track, prices, month, daysInMonth)
		for _, position := range positions {
			invoice.TotalCents += position.AmountCents
		}
		invoice.Positions = append(invoice.Positions, positions...)
	}
	return invoice, unpriced
}

// missingPrices names every booked weekday of a module without a price.
func missingPrices(head parentbilling.CareContractVersion, track moduleTrack, prices priceBook, month calendar.Date) []parentbilling.CareUnpriced {
	var missing []parentbilling.CareUnpriced
	seen := map[string]bool{}
	for _, run := range track.runs {
		for _, weekday := range run.weekdays {
			if _, ok := prices.on(head.SchoolID, track.offeringID, weekday, month); ok || seen[weekday] {
				continue
			}
			seen[weekday] = true
			missing = append(missing, parentbilling.CareUnpriced{
				ContractID: head.ContractID, SchoolID: head.SchoolID, SchoolName: head.SchoolName,
				ChildFirstName: head.ChildFirstName, ChildLastName: head.ChildLastName,
				CareOfferingID: track.offeringID, ModuleName: track.name, Weekday: weekday,
			})
		}
	}
	return missing
}

// trackPositions turns the runs of a module into positions. The module
// costs its exact share of the month, rounded half up once; the cents go
// to the runs and within a run to the weekdays by largest remainder, so the
// parts add up to the module to the cent (Betragsregel 2). A run over the
// whole month is the full monthly price.
func trackPositions(schoolID int64, track moduleTrack, prices priceBook, month calendar.Date, daysInMonth int) []parentbilling.CarePosition {
	// Every weekday of every run with its exact share price*days/daysInMonth.
	shares := make([][]int64, len(track.runs))
	runShares := make([]int64, len(track.runs))
	var total int64
	for i, run := range track.runs {
		shares[i] = make([]int64, len(run.weekdays))
		for j, weekday := range run.weekdays {
			price, _ := prices.on(schoolID, track.offeringID, weekday, month)
			shares[i][j] = price * int64(run.days())
			runShares[i] += shares[i][j]
		}
		total += runShares[i]
	}
	runCents := largestRemainder(runShares, int64(daysInMonth), roundHalfUp(total, int64(daysInMonth)))
	positions := make([]parentbilling.CarePosition, len(track.runs))
	for i, run := range track.runs {
		weekdayCents := largestRemainder(shares[i], int64(daysInMonth), runCents[i])
		position := parentbilling.CarePosition{
			CareOfferingID: track.offeringID, ModuleName: track.name, VersionNumber: run.version,
			ServiceFrom: run.from, ServiceUntil: run.until, Weekdays: make([]parentbilling.CareWeekdayPrice, len(run.weekdays)),
			AmountCents: runCents[i],
		}
		for j, weekday := range run.weekdays {
			price, _ := prices.on(schoolID, track.offeringID, weekday, month)
			position.Weekdays[j] = parentbilling.CareWeekdayPrice{Weekday: weekday, MonthlyCents: price, AmountCents: weekdayCents[j]}
		}
		positions[i] = position
	}
	return positions
}

// roundHalfUp divides numerator by denominator, rounding half up.
func roundHalfUp(numerator, denominator int64) int64 {
	return (2*numerator + denominator) / (2 * denominator)
}

// largestRemainder shares total cents among the parts numerators[i]/denominator:
// every part gets its floor, the cents left go one by one to the largest
// remainders, the earlier part first on a tie. The parts add up to total.
func largestRemainder(numerators []int64, denominator, total int64) []int64 {
	cents := make([]int64, len(numerators))
	order := make([]int, len(numerators))
	var sum int64
	for i, numerator := range numerators {
		cents[i] = numerator / denominator
		sum += cents[i]
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		return compareOrdered(numerators[b]%denominator, numerators[a]%denominator)
	})
	// The floors add up to at most total, and total exceeds them by less
	// than one cent per part.
	for k := 0; sum < total && k < len(order); k++ {
		cents[order[k]]++
		sum++
	}
	return cents
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

// studentOf is the child record of the latest version of the contract that
// names one.
func studentOf(versions []parentbilling.CareContractVersion) *int64 {
	for i := len(versions) - 1; i >= 0; i-- {
		if versions[i].StudentID != nil {
			id := *versions[i].StudentID
			return &id
		}
	}
	return nil
}
