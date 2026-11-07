package application

import (
	"fmt"
	"slices"
	"strings"

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
// without a price, the children with duplicate contracts and the count of
// contracts already billed.
type careMonth struct {
	invoices   []parentbilling.CareInvoice
	unpriced   []parentbilling.CareUnpriced
	duplicates []parentbilling.CareDuplicate
	billed     int
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
	duplicates, held := findDuplicates(in.month, in.rule, byContract, order)
	prices := newPriceBook(in.tariffs)
	result := careMonth{invoices: []parentbilling.CareInvoice{}, unpriced: []parentbilling.CareUnpriced{}, duplicates: duplicates}
	for _, contractID := range order {
		if in.invoiced[contractID] {
			result.billed++
			continue
		}
		if held[contractID] {
			continue
		}
		invoice, unpriced := billContract(in.month, in.rule, byContract[contractID], prices)
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

// childName is the name a child is compared by when a contract has no
// child record.
func childName(version parentbilling.CareContractVersion) string {
	return strings.ToLower(strings.TrimSpace(version.ChildFirstName)) + "|" + strings.ToLower(strings.TrimSpace(version.ChildLastName))
}

// sameChild tells whether two contracts name the same child: by the child
// record when both have one, else by name and date of birth, so a contract
// linked to the record and one that is not still meet.
func sameChild(a, b parentbilling.CareContractVersion) bool {
	if a.StudentID != nil && b.StudentID != nil {
		return *a.StudentID == *b.StudentID
	}
	if childName(a) != childName(b) {
		return false
	}
	if a.ChildBirthDate == nil || b.ChildBirthDate == nil {
		return a.ChildBirthDate == nil && b.ChildBirthDate == nil
	}
	return *a.ChildBirthDate == *b.ChildBirthDate
}

// slotKey is one module on one day of the month.
type slotKey struct {
	offeringID int64
	day        calendar.Date
}

// findDuplicates names every child whose contracts bill the same module on
// the same day of the month (Betragsregel 1), and holds both contracts of
// such a pair: none of them is billed, none silently left out. A child
// with a record matches its contracts without one by name and date of
// birth, so contracts are compared within the same name.
func findDuplicates(month calendar.Date, rule string, byContract map[int64][]parentbilling.CareContractVersion, order []int64) ([]parentbilling.CareDuplicate, map[int64]bool) {
	duplicates := []parentbilling.CareDuplicate{}
	held := map[int64]bool{}
	for _, pair := range sameChildPairs(byContract, order) {
		if shared := sharedSlots(month, rule, byContract[pair[0]], byContract[pair[1]]); len(shared) > 0 {
			duplicates = append(duplicates, shared...)
			held[pair[0]], held[pair[1]] = true, true
		}
	}
	return duplicates, held
}

// sameChildPairs lists every pair of contracts that name the same child, in
// the order of the contracts. Contracts meet through the child record, or
// through the name when one of them has no record; a record with another
// name on a later contract is still the same child.
func sameChildPairs(byContract map[int64][]parentbilling.CareContractVersion, order []int64) [][2]int64 {
	buckets, keys := childBuckets(byContract, order)
	seen := map[[2]int64]bool{}
	var pairs [][2]int64
	for _, key := range keys {
		contracts := buckets[key]
		for i := range contracts {
			for _, other := range contracts[i+1:] {
				pair := [2]int64{contracts[i], other}
				if seen[pair] || !sameChild(byContract[contracts[i]][0], byContract[other][0]) {
					continue
				}
				seen[pair] = true
				pairs = append(pairs, pair)
			}
		}
	}
	return pairs
}

// childBuckets puts every contract under its child's name and, when it
// has one, under its child record, in the order of the contracts.
func childBuckets(byContract map[int64][]parentbilling.CareContractVersion, order []int64) (map[string][]int64, []string) {
	buckets := map[string][]int64{}
	keys := []string{}
	add := func(key string, contractID int64) {
		if _, seen := buckets[key]; !seen {
			keys = append(keys, key)
		}
		buckets[key] = append(buckets[key], contractID)
	}
	for _, contractID := range order {
		version := byContract[contractID][0]
		add("name:"+childName(version), contractID)
		if version.StudentID != nil {
			add(fmt.Sprintf("student:%d", *version.StudentID), contractID)
		}
	}
	return buckets, keys
}

// billedSlots are the days the run bills each module of a contract on,
// under the rule of the month: the whole month after the version on the
// first, or the days each version holds pro rata.
func billedSlots(month calendar.Date, rule string, versions []parentbilling.CareContractVersion) map[slotKey]string {
	slots := map[slotKey]string{}
	for _, track := range tracksOf(month, rule, versions) {
		for _, run := range track.runs {
			for day := run.from; !day.After(run.until); day = day.AddDays(1) {
				if slices.Contains(run.weekdays, weekdayOf(day)) {
					slots[slotKey{track.offeringID, day}] = track.name
				}
			}
		}
	}
	return slots
}

// sharedSlots lists every module and weekday both contracts are billed for
// on a day of the month, in the order of the days.
func sharedSlots(month calendar.Date, rule string, first, second []parentbilling.CareContractVersion) []parentbilling.CareDuplicate {
	mine, theirs := billedSlots(month, rule, first), billedSlots(month, rule, second)
	type moduleDay struct {
		offeringID int64
		weekday    string
	}
	seen := map[moduleDay]bool{}
	shared := []parentbilling.CareDuplicate{}
	a, b := first[0], second[0]
	tracks := tracksOf(month, rule, second)
	for day := month; !day.After(monthEnd(month)); day = day.AddDays(1) {
		for _, track := range tracks {
			slot := slotKey{track.offeringID, day}
			name, both := mine[slot]
			key := moduleDay{track.offeringID, weekdayOf(day)}
			if _, billed := theirs[slot]; !both || !billed || seen[key] {
				continue
			}
			seen[key] = true
			shared = append(shared, parentbilling.CareDuplicate{
				ChildFirstName: a.ChildFirstName, ChildLastName: a.ChildLastName, ModuleName: name, Weekday: weekdayOf(day),
				Contracts: []parentbilling.CareDuplicateContract{
					{ContractID: a.ContractID, SchoolID: a.SchoolID, SchoolName: a.SchoolName},
					{ContractID: b.ContractID, SchoolID: b.SchoolID, SchoolName: b.SchoolName},
				},
			})
		}
	}
	return shared
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

// tracksOf follows every module of a contract through the month. Under the
// full month the version on the first day (or the first one of a contract
// that starts in the month) books the whole month with every module it has
// on that day. Pro rata a module's booked weekdays are read day by day;
// days with the same weekdays form one run, also across versions, so a
// module that does not change is one run (Betragsregel 2).
func tracksOf(month calendar.Date, rule string, versions []parentbilling.CareContractVersion) []moduleTrack {
	if rule != parentbilling.MonthRuleProRata {
		return fullMonthTracks(month, versions[0])
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
// books on the first billed day.
func fullMonthTracks(month calendar.Date, version parentbilling.CareContractVersion) []moduleTrack {
	first := month
	if version.From.After(month) {
		first = version.From
	}
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
func billContract(month calendar.Date, rule string, versions []parentbilling.CareContractVersion, prices priceBook) (parentbilling.CareInvoice, []parentbilling.CareUnpriced) {
	head := versions[0]
	invoice := parentbilling.CareInvoice{
		ContractID: head.ContractID, SchoolID: head.SchoolID, SchoolName: head.SchoolName,
		ChildFirstName: head.ChildFirstName, ChildLastName: head.ChildLastName, Positions: []parentbilling.CarePosition{},
	}
	daysInMonth := month.DaysUntil(monthEnd(month)) + 1
	var unpriced []parentbilling.CareUnpriced
	for _, track := range tracksOf(month, rule, versions) {
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
