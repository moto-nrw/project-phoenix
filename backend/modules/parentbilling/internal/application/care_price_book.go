package application

import (
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The price book of the monthly run (#4684, #6196). A module of a school has
// one price form from a month on: the form of its latest price that starts
// on or before the month. Within that form a weekday or a scale step takes
// its latest price; a price of the other form never counts, so a module
// that changes its form bills only by the new one.

// moduleKey names one module of a school.
type moduleKey struct {
	schoolID   int64
	offeringID int64
}

// unitPrice is the price of a weekday or a scale step: for the first child
// and for every further child (the first child's without a sibling price).
type unitPrice struct {
	monthly int64
	sibling int64
}

// priceBook holds the prices of every module by start, oldest first.
type priceBook map[moduleKey][]parentbilling.CareTariff

func newPriceBook(tariffs []parentbilling.CareTariff) priceBook {
	book := priceBook{}
	for _, tariff := range tariffs {
		key := moduleKey{tariff.SchoolID, tariff.CareOfferingID}
		book[key] = append(book[key], tariff)
	}
	for key := range book {
		slices.SortStableFunc(book[key], func(a, b parentbilling.CareTariff) int { return a.ValidFrom.Compare(b.ValidFrom) })
	}
	return book
}

// module is the form of a module's prices on day and whether they exclude
// the meal; ok is false while the module has no price yet.
func (b priceBook) module(schoolID, offeringID int64, day calendar.Date) (form string, mealExtra, ok bool) {
	tariffs := b[moduleKey{schoolID, offeringID}]
	for i := len(tariffs) - 1; i >= 0; i-- {
		if !tariffs[i].ValidFrom.After(day) {
			return tariffs[i].PriceForm(), tariffs[i].MealExtra, true
		}
	}
	return "", false, false
}

// latest is the latest price of the module's form on day that matches.
func (b priceBook) latest(schoolID, offeringID int64, day calendar.Date, form string, matches func(parentbilling.CareTariff) bool) (unitPrice, bool) {
	if current, _, ok := b.module(schoolID, offeringID, day); !ok || current != form {
		return unitPrice{}, false
	}
	tariffs := b[moduleKey{schoolID, offeringID}]
	for i := len(tariffs) - 1; i >= 0; i-- {
		tariff := tariffs[i]
		if tariff.ValidFrom.After(day) || tariff.PriceForm() != form || !matches(tariff) {
			continue
		}
		price := unitPrice{monthly: tariff.MonthlyCents, sibling: tariff.MonthlyCents}
		if tariff.SiblingCents != nil {
			price.sibling = *tariff.SiblingCents
		}
		return price, true
	}
	return unitPrice{}, false
}

// on is the monthly price of a weekday that holds on day: the latest that
// starts on or before it, while the module is priced per weekday.
func (b priceBook) on(schoolID, offeringID int64, weekday string, day calendar.Date) (int64, bool) {
	price, ok := b.weekday(schoolID, offeringID, weekday, day)
	return price.monthly, ok
}

func (b priceBook) weekday(schoolID, offeringID int64, weekday string, day calendar.Date) (unitPrice, bool) {
	return b.latest(schoolID, offeringID, day, parentbilling.PriceFormWeekday,
		func(tariff parentbilling.CareTariff) bool { return tariff.Weekday == weekday })
}

// scale is the monthly price of a module booked on days days per week,
// while the module is priced by a scale.
func (b priceBook) scale(schoolID, offeringID int64, days int, day calendar.Date) (unitPrice, bool) {
	return b.latest(schoolID, offeringID, day, parentbilling.PriceFormDaysPerWeek,
		func(tariff parentbilling.CareTariff) bool { return *tariff.DaysPerWeek == days })
}

// tariffKey names a price within its module: a weekday or a scale step.
type tariffKey struct {
	schoolID   int64
	offeringID int64
	weekday    string
	days       int
}

func keyOf(tariff parentbilling.CareTariff) tariffKey {
	key := tariffKey{schoolID: tariff.SchoolID, offeringID: tariff.CareOfferingID, weekday: tariff.Weekday}
	if tariff.DaysPerWeek != nil {
		key.days = *tariff.DaysPerWeek
	}
	return key
}

// withValidity sets the last day of every price: the day before the next
// price of the same module and key.
func withValidity(tariffs []parentbilling.CareTariff) []parentbilling.CareTariff {
	byKey := map[tariffKey][]parentbilling.CareTariff{}
	for _, tariff := range tariffs {
		byKey[keyOf(tariff)] = append(byKey[keyOf(tariff)], tariff)
	}
	result := make([]parentbilling.CareTariff, 0, len(tariffs))
	for _, list := range byKey {
		slices.SortFunc(list, func(a, b parentbilling.CareTariff) int { return a.ValidFrom.Compare(b.ValidFrom) })
		for i := range list {
			if i+1 < len(list) {
				until := list[i+1].ValidFrom.AddDays(-1)
				list[i].ValidUntil = &until
			}
			result = append(result, list[i])
		}
	}
	slices.SortFunc(result, func(a, b parentbilling.CareTariff) int {
		ka, kb := keyOf(a), keyOf(b)
		return firstNonZero(compareOrdered(a.SchoolID, b.SchoolID), a.ValidFrom.Compare(b.ValidFrom),
			compareOrdered(a.ModuleName, b.ModuleName), compareOrdered(a.CareOfferingID, b.CareOfferingID),
			compareOrdered(weekdayOrder[ka.weekday], weekdayOrder[kb.weekday]), compareOrdered(ka.days, kb.days))
	})
	return result
}

// freeMonths names per school the price-list heads by start, oldest first.
type freeMonths map[int64][]parentbilling.CarePriceListHead

func newFreeMonths(heads []parentbilling.CarePriceListHead) freeMonths {
	book := freeMonths{}
	for _, head := range heads {
		book[head.SchoolID] = append(book[head.SchoolID], head)
	}
	for school := range book {
		slices.SortFunc(book[school], func(a, b parentbilling.CarePriceListHead) int { return a.ValidFrom.Compare(b.ValidFrom) })
	}
	return book
}

// free reports whether the price list of the school in force in month
// makes the month free of charge.
func (f freeMonths) free(schoolID int64, month calendar.Date) bool {
	heads := f[schoolID]
	for i := len(heads) - 1; i >= 0; i-- {
		if !heads[i].ValidFrom.After(month) {
			return slices.Contains(heads[i].FreeMonths, int(month.Month()))
		}
	}
	return false
}
