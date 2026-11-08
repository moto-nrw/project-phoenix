package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// careMealHorizon is how many months back a run looks for released Mensa
// months not billed yet, the month of the run included.
const careMealHorizon = 12

// CareMealKey names a child's Mensa month at a school.
type CareMealKey = parentbilling.CareBilledMeal

// CareMealStore persists the Mensa prices and the billed Mensa months of a
// Träger in its Träger transaction. The positions of an issued invoice are
// stored and read with the invoice (IssueCareInvoices, CareRunInvoices).
type CareMealStore interface {
	CareMealTariffs(ctx context.Context, organizationID int64) ([]parentbilling.CareMealTariff, error)
	// ReplaceCareMealTariff sets the price of a school from a month on,
	// replacing one that starts on the same day.
	ReplaceCareMealTariff(ctx context.Context, organizationID, accountID int64, price parentbilling.CareMealPrice) error
	DeleteCareMealTariff(ctx context.Context, organizationID, schoolID int64, validFrom calendar.Date) (int64, error)
	// BilledMealMonths names the Mensa months of [from, to] an issued
	// invoice bills.
	BilledMealMonths(ctx context.Context, organizationID int64, from, to calendar.Date) ([]parentbilling.CareBilledMeal, error)
	// MealBilledUntil names per school the last Mensa month an issued
	// invoice bills.
	MealBilledUntil(ctx context.Context, organizationID int64) (map[int64]calendar.Date, error)
}

var _ parentbilling.CareMealBilling = (*CareBilling)(nil)

// CareMealTariffs reads the Mensa prices with the schools of the Träger.
func (c *CareBilling) CareMealTariffs(ctx context.Context, reviewer parentbilling.HolidayReviewer) (parentbilling.CareMealTariffs, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareMealTariffs{}, err
	}
	schools, err := c.schools.TraegerSchools(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.CareMealTariffs{}, fmt.Errorf("parent billing: read the schools: %w", err)
	}
	tariffs, err := c.mealStore.CareMealTariffs(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.CareMealTariffs{}, err
	}
	billed, err := c.mealStore.MealBilledUntil(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.CareMealTariffs{}, err
	}
	return parentbilling.CareMealTariffs{Schools: schools, Tariffs: mealTariffValidity(tariffs), BilledUntil: billed}, nil
}

// SaveCareMealTariff sets the Mensa of a school from a month on that no
// issued invoice bills yet.
func (c *CareBilling) SaveCareMealTariff(ctx context.Context, reviewer parentbilling.HolidayReviewer, price parentbilling.CareMealPrice) (parentbilling.CareMealTariffs, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareMealTariffs{}, err
	}
	if price.SchoolID <= 0 || !isMonthStart(price.ValidFrom) || price.AmountCents < 0 || price.AmountCents > parentbilling.MaxMealPriceCents ||
		(price.Mode != parentbilling.MealModeFlat && price.Mode != parentbilling.MealModePerMeal) {
		return parentbilling.CareMealTariffs{}, parentbilling.ErrCareMealTariffInvalid
	}
	if own, err := c.ownSchool(ctx, reviewer.OrganizationID, price.SchoolID); err != nil || !own {
		if err != nil {
			return parentbilling.CareMealTariffs{}, err
		}
		return parentbilling.CareMealTariffs{}, parentbilling.ErrCareMealTariffInvalid
	}
	if err := c.keepBilledMeals(ctx, reviewer.OrganizationID, price.SchoolID, price.ValidFrom); err != nil {
		return parentbilling.CareMealTariffs{}, err
	}
	if err := c.mealStore.ReplaceCareMealTariff(ctx, reviewer.OrganizationID, reviewer.AccountID, price); err != nil {
		return parentbilling.CareMealTariffs{}, err
	}
	return c.CareMealTariffs(ctx, reviewer)
}

// DeleteCareMealTariff removes a Mensa price of a month no invoice bills.
func (c *CareBilling) DeleteCareMealTariff(ctx context.Context, reviewer parentbilling.HolidayReviewer, schoolID int64, validFrom calendar.Date) (parentbilling.CareMealTariffs, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareMealTariffs{}, err
	}
	if schoolID <= 0 || !isMonthStart(validFrom) {
		return parentbilling.CareMealTariffs{}, parentbilling.ErrCareMealTariffNotFound
	}
	if err := c.keepBilledMeals(ctx, reviewer.OrganizationID, schoolID, validFrom); err != nil {
		return parentbilling.CareMealTariffs{}, err
	}
	deleted, err := c.mealStore.DeleteCareMealTariff(ctx, reviewer.OrganizationID, schoolID, validFrom)
	if err != nil {
		return parentbilling.CareMealTariffs{}, err
	}
	if deleted == 0 {
		return parentbilling.CareMealTariffs{}, parentbilling.ErrCareMealTariffNotFound
	}
	return c.CareMealTariffs(ctx, reviewer)
}

// keepBilledMeals holds the monthly billing and refuses a change of the
// Mensa of a school from validFrom on when an issued invoice bills a Mensa
// month of the school from then on: the price it used stays.
func (c *CareBilling) keepBilledMeals(ctx context.Context, organizationID, schoolID int64, validFrom calendar.Date) error {
	if err := c.store.LockCareBilling(ctx, organizationID); err != nil {
		return fmt.Errorf("parent billing: hold the monthly billing: %w", err)
	}
	billed, err := c.mealStore.MealBilledUntil(ctx, organizationID)
	if err != nil {
		return err
	}
	if until, ok := billed[schoolID]; ok && !until.Before(validFrom) {
		return parentbilling.ErrCareMealTariffMonthBilled
	}
	return nil
}

// mealTariffValidity sets ValidUntil to the day before the next price of
// the same school.
func mealTariffValidity(tariffs []parentbilling.CareMealTariff) []parentbilling.CareMealTariff {
	result := slices.Clone(tariffs)
	slices.SortFunc(result, func(a, b parentbilling.CareMealTariff) int {
		return firstNonZero(compareOrdered(a.SchoolID, b.SchoolID), a.ValidFrom.Compare(b.ValidFrom))
	})
	for i := range result {
		result[i].ValidUntil = nil
		if i+1 < len(result) && result[i+1].SchoolID == result[i].SchoolID {
			until := result[i+1].ValidFrom.AddDays(-1)
			result[i].ValidUntil = &until
		}
	}
	return result
}

// careMealInput is everything the Mensa of a run is computed from.
type careMealInput struct {
	month      calendar.Date
	today      calendar.Date
	schools    map[int64]string
	tariffs    []parentbilling.CareMealTariff
	releases   []parentbilling.CareMealRelease
	billed     map[CareMealKey]bool
	versions   []parentbilling.CareContractVersion
	invoiced   map[int64]bool
	recipients map[int64]parentbilling.CareRecipient
}

// addMeals reads the Mensa of the run and adds it to the preview.
func (c *CareBilling) addMeals(ctx context.Context, organizationID int64, run parentbilling.CareRun, month *careMonth,
	versions []parentbilling.CareContractVersion, invoiced map[int64]bool, recipients map[int64]parentbilling.CareRecipient,
) error {
	tariffs, err := c.mealStore.CareMealTariffs(ctx, organizationID)
	if err != nil || len(tariffs) == 0 {
		return err
	}
	from := calendar.NewDate(run.BillingMonth.Year(), run.BillingMonth.Month()-(careMealHorizon-1), 1)
	releases, err := c.meals.ReleasedMealMonths(ctx, organizationID, monthEnd(run.BillingMonth))
	if err != nil {
		return fmt.Errorf("parent billing: read the released Mensa months: %w", err)
	}
	billedMeals, err := c.mealStore.BilledMealMonths(ctx, organizationID, from, run.BillingMonth)
	if err != nil {
		return err
	}
	billed := make(map[CareMealKey]bool, len(billedMeals))
	for _, meal := range billedMeals {
		billed[meal] = true
	}
	schools, err := c.schools.TraegerSchools(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("parent billing: read the schools: %w", err)
	}
	names := make(map[int64]string, len(schools))
	for _, school := range schools {
		names[school.ID] = school.Name
	}
	billMeals(month, careMealInput{month: run.BillingMonth, today: c.today(), schools: names, tariffs: tariffs, releases: releases,
		billed: billed, versions: versions, invoiced: invoiced, recipients: recipients})
	return nil
}

// mealTariffOn is the Mensa price of a school in a month, if any.
func mealTariffOn(tariffs []parentbilling.CareMealTariff, schoolID int64, month calendar.Date) (parentbilling.CareMealTariff, bool) {
	var found parentbilling.CareMealTariff
	ok := false
	for _, tariff := range tariffs {
		if tariff.SchoolID == schoolID && !tariff.ValidFrom.After(month) && (!ok || tariff.ValidFrom.After(found.ValidFrom)) {
			found, ok = tariff, true
		}
	}
	return found, ok
}

// billMeals adds to the preview of a run the Mensa months of the horizon
// (#4686): a released month of a school with a Mensa price goes, for each
// of its children not billed for it yet, onto the invoice of the child's
// contract at that school in the month of the run. A contract without an
// invoice of its own (no booked module this month) gets one for its Mensa.
// A contract an earlier run of the month issued keeps its meals for the
// next month; a held or unpriced contract stays out with its reason
// shown elsewhere. An ended month without a release is a gap.
func billMeals(month *careMonth, in careMealInput) {
	month.mealGaps, month.mealUnbilled = []parentbilling.CareMealGap{}, []parentbilling.CareMealUnbilled{}
	from := calendar.NewDate(in.month.Year(), in.month.Month()-(careMealHorizon-1), 1)
	released := map[[2]any]parentbilling.CareMealRelease{}
	for _, release := range in.releases {
		released[[2]any{release.SchoolID, release.Month}] = release
	}
	for _, school := range mealSchools(in.tariffs) {
		for m := from; !m.After(in.month); m = monthEnd(m).AddDays(1) {
			if _, priced := mealTariffOn(in.tariffs, school, m); !priced {
				continue
			}
			release, ok := released[[2]any{school, m}]
			if !ok {
				if monthEnd(m).Before(in.today) {
					month.mealGaps = append(month.mealGaps, parentbilling.CareMealGap{SchoolID: school, SchoolName: in.schools[school], Month: m})
				}
				continue
			}
			tariff, _ := mealTariffOn(in.tariffs, school, m)
			for _, child := range release.Children {
				key := CareMealKey{SchoolID: school, StudentID: child.StudentID, Month: m}
				if in.billed[key] {
					continue
				}
				position, ok := mealPosition(tariff, key, child)
				if !ok {
					continue
				}
				placeMeal(month, in, position, child, in.schools[school])
			}
		}
	}
	for i := range month.invoices {
		slices.SortFunc(month.invoices[i].MealPositions, func(a, b parentbilling.CareMealPosition) int { return a.MealMonth.Compare(b.MealMonth) })
	}
	slices.SortStableFunc(month.invoices, func(a, b parentbilling.CareInvoice) int {
		return firstNonZero(compareOrdered(a.SchoolName, b.SchoolName), compareOrdered(a.ChildLastName, b.ChildLastName),
			compareOrdered(a.ChildFirstName, b.ChildFirstName), compareOrdered(a.ContractID, b.ContractID))
	})
}

// mealSchools lists the schools with a Mensa price.
func mealSchools(tariffs []parentbilling.CareMealTariff) []int64 {
	schools := []int64{}
	for _, tariff := range tariffs {
		if !slices.Contains(schools, tariff.SchoolID) {
			schools = append(schools, tariff.SchoolID)
		}
	}
	slices.Sort(schools)
	return schools
}

// mealPosition prices a child's Mensa month: per meal the meals of the
// month at the price each, flat the price once. A month without a meal
// bills nothing per meal.
func mealPosition(tariff parentbilling.CareMealTariff, key CareMealKey, child parentbilling.CareMealChild) (parentbilling.CareMealPosition, bool) {
	position := parentbilling.CareMealPosition{SchoolID: key.SchoolID, StudentID: key.StudentID, MealMonth: key.Month,
		Mode: tariff.Mode, UnitCents: tariff.AmountCents}
	switch tariff.Mode {
	case parentbilling.MealModePerMeal:
		if child.MealDays <= 0 {
			return parentbilling.CareMealPosition{}, false
		}
		position.Quantity = child.MealDays
		position.AmountCents = int64(child.MealDays) * tariff.AmountCents
	case parentbilling.MealModeFlat:
		position.Quantity = 1
		position.AmountCents = tariff.AmountCents
	default:
		return parentbilling.CareMealPosition{}, false
	}
	return position, true
}

// placeMeal puts a position on the invoice of the child's contract.
func placeMeal(month *careMonth, in careMealInput, position parentbilling.CareMealPosition, child parentbilling.CareMealChild, schoolName string) {
	contracts := mealChildContracts(in.versions, position.SchoolID, child)
	unbilled := parentbilling.CareMealUnbilled{SchoolID: position.SchoolID, SchoolName: schoolName, StudentID: child.StudentID,
		ChildFirstName: child.FirstName, ChildLastName: child.LastName, Month: position.MealMonth, MealDays: child.MealDays,
		Reason: parentbilling.MealUnbilledNoContract}
	if len(contracts) == 0 {
		month.mealUnbilled = append(month.mealUnbilled, unbilled)
		return
	}
	for _, contractID := range contracts {
		for i := range month.invoices {
			if month.invoices[i].ContractID == contractID {
				month.invoices[i].MealPositions = append(month.invoices[i].MealPositions, position)
				month.invoices[i].TotalCents += position.AmountCents
				return
			}
		}
	}
	skipped := heldOrUnpriced(month)
	for _, contractID := range contracts {
		if skipped[contractID] {
			return
		}
	}
	for _, contractID := range contracts {
		if in.invoiced[contractID] {
			unbilled.Reason = parentbilling.MealUnbilledContractBilled
			month.mealUnbilled = append(month.mealUnbilled, unbilled)
			return
		}
	}
	head := firstVersion(in.versions, contracts[0])
	month.invoices = append(month.invoices, parentbilling.CareInvoice{
		ContractID: head.ContractID, SchoolID: head.SchoolID, SchoolName: head.SchoolName,
		ChildFirstName: head.ChildFirstName, ChildLastName: head.ChildLastName, Recipient: in.recipients[head.ContractID],
		Positions: []parentbilling.CarePosition{}, MealPositions: []parentbilling.CareMealPosition{position}, TotalCents: position.AmountCents,
	})
}

// mealChildContracts names the contracts at the school in the month of the run
// whose child is the child of the release: by its record, or by its name
// for a contract without one.
func mealChildContracts(versions []parentbilling.CareContractVersion, schoolID int64, child parentbilling.CareMealChild) []int64 {
	name := mealChildName(child.FirstName, child.LastName)
	ids := []int64{}
	for _, version := range versions {
		if version.SchoolID != schoolID || slices.Contains(ids, version.ContractID) {
			continue
		}
		if (version.StudentID != nil && *version.StudentID == child.StudentID) ||
			(version.StudentID == nil && mealChildName(version.ChildFirstName, version.ChildLastName) == name) {
			ids = append(ids, version.ContractID)
		}
	}
	return ids
}

func mealChildName(first, last string) string {
	return normalizedName(first + " " + last)
}

func firstVersion(versions []parentbilling.CareContractVersion, contractID int64) parentbilling.CareContractVersion {
	for _, version := range versions {
		if version.ContractID == contractID {
			return version
		}
	}
	return parentbilling.CareContractVersion{ContractID: contractID}
}

// heldOrUnpriced names the contracts the preview holds back or leaves out
// for a missing price.
func heldOrUnpriced(month *careMonth) map[int64]bool {
	skipped := map[int64]bool{}
	for _, unpriced := range month.unpriced {
		skipped[unpriced.ContractID] = true
	}
	for _, duplicate := range month.duplicates {
		for _, contract := range duplicate.Contracts {
			skipped[contract.ContractID] = true
		}
	}
	return skipped
}

// shownMealChildren names the children without a contract whose Mensa a
// detail shows; the children on an invoice are named by its contract.
func shownMealChildren(detail parentbilling.CareRunDetail) []int64 {
	ids := []int64{}
	for _, unbilled := range detail.MealUnbilled {
		if !slices.Contains(ids, unbilled.StudentID) {
			ids = append(ids, unbilled.StudentID)
		}
	}
	return ids
}

// withMealPositions answers an invoice without Mensa with an empty list.
func withMealPositions(invoices []parentbilling.CareInvoice) {
	for i := range invoices {
		if invoices[i].MealPositions == nil {
			invoices[i].MealPositions = []parentbilling.CareMealPosition{}
		}
	}
}
