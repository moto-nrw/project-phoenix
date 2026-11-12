package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// keepBilledPrices holds the monthly billing and refuses a change of the
// price list of a school from validFrom on (replacement nil deletes it) that
// would change a price an issued invoice billed: every module and weekday
// or scale step billed in a month from validFrom on keeps its price and its
// sibling price, and a month an issued invoice billed keeps being billed. A
// booked weekday or scale step without a price may still get one, also in
// a billed month, so a contract a run left out can be billed by the next
// run of the month.
func (c *CareBilling) keepBilledPrices(ctx context.Context, organizationID, schoolID int64, validFrom calendar.Date, replacement *parentbilling.CarePriceList) error {
	if err := c.store.LockCareBilling(ctx, organizationID); err != nil {
		return fmt.Errorf("parent billing: hold the monthly billing: %w", err)
	}
	billed, err := c.store.CareBilledPrices(ctx, organizationID, schoolID)
	if err != nil {
		return err
	}
	tariffs, heads, err := c.store.CarePriceLists(ctx, organizationID)
	if err != nil {
		return err
	}
	after, headsAfter := priceListsAfter(tariffs, heads, schoolID, validFrom, replacement)
	if changesBilledPrice(schoolID, validFrom, billed, newPriceBook(tariffs), newPriceBook(after)) ||
		freesBilledMonth(schoolID, validFrom, billed, newFreeMonths(headsAfter)) {
		return parentbilling.ErrCareTariffMonthBilled
	}
	return nil
}

// priceListsAfter are the prices and heads once the list of the school from
// validFrom is replaced (nil: deleted).
func priceListsAfter(tariffs []parentbilling.CareTariff, heads []parentbilling.CarePriceListHead, schoolID int64,
	validFrom calendar.Date, replacement *parentbilling.CarePriceList,
) ([]parentbilling.CareTariff, []parentbilling.CarePriceListHead) {
	after := make([]parentbilling.CareTariff, 0, len(tariffs))
	for _, tariff := range tariffs {
		if tariff.SchoolID != schoolID || tariff.ValidFrom != validFrom {
			after = append(after, tariff)
		}
	}
	headsAfter := make([]parentbilling.CarePriceListHead, 0, len(heads)+1)
	for _, head := range heads {
		if head.SchoolID != schoolID || head.ValidFrom != validFrom {
			headsAfter = append(headsAfter, head)
		}
	}
	if replacement == nil {
		return after, headsAfter
	}
	for _, price := range replacement.Prices {
		after = append(after, parentbilling.CareTariff{SchoolID: schoolID, CareOfferingID: price.CareOfferingID, ModuleName: price.ModuleName,
			Weekday: price.Weekday, DaysPerWeek: price.DaysPerWeek, ValidFrom: validFrom,
			MonthlyCents: price.MonthlyCents, SiblingCents: price.SiblingCents, MealExtra: price.MealExtra})
	}
	return after, append(headsAfter, parentbilling.CarePriceListHead{SchoolID: schoolID, ValidFrom: validFrom, FreeMonths: replacement.FreeMonths})
}

// changesBilledPrice reports a billed weekday or scale step whose price or
// sibling price in its month differs between the two price lists.
func changesBilledPrice(schoolID int64, validFrom calendar.Date, billed []parentbilling.CareBilledPrice, before, after priceBook) bool {
	for _, entry := range billed {
		if entry.Month.Before(validFrom) {
			continue
		}
		lookup := func(book priceBook) (unitPrice, bool) {
			if entry.DaysPerWeek != nil {
				return book.scale(schoolID, entry.CareOfferingID, *entry.DaysPerWeek, entry.Month)
			}
			return book.weekday(schoolID, entry.CareOfferingID, entry.Weekday, entry.Month)
		}
		was, wasSet := lookup(before)
		now, nowSet := lookup(after)
		if wasSet != nowSet || was != now {
			return true
		}
	}
	return false
}

// freesBilledMonth reports a month an issued invoice billed that the new
// price lists would make free of charge.
func freesBilledMonth(schoolID int64, validFrom calendar.Date, billed []parentbilling.CareBilledPrice, after freeMonths) bool {
	return slices.ContainsFunc(billed, func(entry parentbilling.CareBilledPrice) bool {
		return !entry.Month.Before(validFrom) && after.free(schoolID, entry.Month)
	})
}

// normalizePriceList checks a price list: a school, the first day of a
// month, free months of the year, and per price a module with either a
// known weekday or a scale step from 1 to 7 days per week, once each, with
// amounts in cent and a sibling price not above the first child's. All
// prices of one module in a list share their form and their meal note.
func normalizePriceList(list parentbilling.CarePriceList) (parentbilling.CarePriceList, error) {
	errs := validation.Errors{}
	if list.SchoolID <= 0 {
		errs["school_id"] = errors.New("is required")
	}
	if !isMonthStart(list.ValidFrom) {
		errs["valid_from"] = errors.New("must be the first day of a month")
	}
	free := []int{}
	for _, month := range list.FreeMonths {
		if month < 1 || month > 12 {
			errs["free_months"] = errors.New("must be months from 1 to 12")
		} else if !slices.Contains(free, month) {
			free = append(free, month)
		}
	}
	slices.Sort(free)
	list.FreeMonths = free
	seen := map[tariffKey]bool{}
	type moduleShape struct {
		form string
		meal bool
	}
	modules := map[int64]moduleShape{}
	for i, price := range list.Prices {
		price.ModuleName = strings.TrimSpace(price.ModuleName)
		tariff := parentbilling.CareTariff{CareOfferingID: price.CareOfferingID, Weekday: price.Weekday, DaysPerWeek: price.DaysPerWeek}
		if message := priceError(price); message != "" {
			errs["prices"] = errors.New(message)
		} else if seen[keyOf(tariff)] {
			errs["prices"] = errors.New("names a module and weekday or scale step twice")
		}
		seen[keyOf(tariff)] = true
		shape := moduleShape{form: tariff.PriceForm(), meal: price.MealExtra}
		if known, ok := modules[price.CareOfferingID]; ok && known != shape {
			errs["prices"] = errors.New("a module has one price form and one meal note")
		}
		modules[price.CareOfferingID] = shape
		list.Prices[i] = price
	}
	if len(errs) > 0 {
		return list, errs
	}
	return list, nil
}

// priceError names what is wrong with one price, empty when nothing is.
func priceError(price parentbilling.CarePrice) string {
	switch {
	case price.CareOfferingID <= 0:
		return "needs a module and a weekday or scale step"
	case (price.Weekday == "") == (price.DaysPerWeek == nil):
		return "needs either a weekday or a number of days per week"
	case price.Weekday != "" && weekdayOrder[price.Weekday] == 0:
		return "needs a module and a weekday or scale step"
	case price.DaysPerWeek != nil && (*price.DaysPerWeek < 1 || *price.DaysPerWeek > parentbilling.MaxDaysPerWeek):
		return "days per week must be between 1 and 7"
	case price.MonthlyCents < 0 || price.MonthlyCents > parentbilling.MaxMonthlyPriceCents:
		return "must be amounts in cent, not negative"
	case price.SiblingCents != nil && (*price.SiblingCents < 0 || *price.SiblingCents > price.MonthlyCents):
		return "a sibling price is an amount in cent, not above the price of the first child"
	case price.ModuleName == "" || len([]rune(price.ModuleName)) > parentbilling.MaxModuleNameLength:
		return "needs the name of the module"
	}
	return ""
}

// modulePriceList is the price list of the module's school from its month
// with the module's prices replaced: the other modules of that month stay,
// and the free months come from the list of that month or the one before.
func modulePriceList(tariffs []parentbilling.CareTariff, heads []parentbilling.CarePriceListHead, module parentbilling.CareModulePriceList) parentbilling.CarePriceList {
	list := parentbilling.CarePriceList{SchoolID: module.SchoolID, ValidFrom: module.ValidFrom, FreeMonths: []int{}}
	var latest calendar.Date
	for _, head := range heads {
		if head.SchoolID == module.SchoolID && !head.ValidFrom.After(module.ValidFrom) && (latest == "" || head.ValidFrom.After(latest)) {
			latest, list.FreeMonths = head.ValidFrom, slices.Clone(head.FreeMonths)
		}
	}
	for _, tariff := range tariffs {
		if tariff.SchoolID == module.SchoolID && tariff.ValidFrom == module.ValidFrom && tariff.CareOfferingID != module.CareOfferingID {
			list.Prices = append(list.Prices, parentbilling.CarePrice{
				CareOfferingID: tariff.CareOfferingID, ModuleName: tariff.ModuleName, Weekday: tariff.Weekday,
				DaysPerWeek: tariff.DaysPerWeek, MonthlyCents: tariff.MonthlyCents, SiblingCents: tariff.SiblingCents, MealExtra: tariff.MealExtra,
			})
		}
	}
	for _, price := range module.Prices {
		price.CareOfferingID, price.ModuleName = module.CareOfferingID, module.ModuleName
		list.Prices = append(list.Prices, price)
	}
	return list
}

// mergeModulePrices combines all changes of a month before validation. The
// result keeps the other modules and free months; input order cannot lose a
// preceding module of the same batch.
func mergeModulePrices(tariffs []parentbilling.CareTariff, heads []parentbilling.CarePriceListHead, modules []parentbilling.CareModulePriceList) ([]parentbilling.CarePriceList, []parentbilling.CareTariff, []parentbilling.CarePriceListHead, error) {
	after, headsAfter := tariffs, heads
	byMonth := map[calendar.Date]parentbilling.CarePriceList{}
	seen := map[[2]string]bool{}
	for _, module := range modules {
		key := [2]string{module.ValidFrom.String(), fmt.Sprint(module.CareOfferingID)}
		if seen[key] {
			return nil, nil, nil, validation.Errors{"prices": errors.New("module repeated in a month")}
		}
		seen[key] = true
		list, err := normalizePriceList(modulePriceList(after, headsAfter, module))
		if err != nil {
			return nil, nil, nil, err
		}
		byMonth[module.ValidFrom] = list
		after, headsAfter = priceListsAfter(after, headsAfter, module.SchoolID, module.ValidFrom, &list)
	}
	lists := make([]parentbilling.CarePriceList, 0, len(byMonth))
	for _, list := range byMonth {
		lists = append(lists, list)
	}
	slices.SortFunc(lists, func(a, b parentbilling.CarePriceList) int { return a.ValidFrom.Compare(b.ValidFrom) })
	return lists, after, headsAfter, nil
}
