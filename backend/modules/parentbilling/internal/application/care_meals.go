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
	// BilledMealMonths names the Mensa months of [from, to] already billed
	// or reserved for a cancelled invoice's reissue outside the run month to.
	BilledMealMonths(ctx context.Context, organizationID int64, from, to calendar.Date) ([]parentbilling.CareBilledMeal, error)
	// MealBilledUntil names per school the last Mensa month an issued
	// invoice bills.
	MealBilledUntil(ctx context.Context, organizationID int64) (map[int64]calendar.Date, error)
	// MealInvoicedContracts names the contracts an issued, not cancelled
	// Mensa invoice of their own bills in month (ADR 0167).
	MealInvoicedContracts(ctx context.Context, organizationID int64, month calendar.Date) (map[int64]bool, error)
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
	invoiced   map[int64]parentbilling.CareInvoice
	recipients map[int64]parentbilling.CareRecipient
	attendance map[int64]calendar.Date
	// ownInvoice: the Mensa travels on an invoice of its own (ADR 0167);
	// mealInvoiced names the contracts such an invoice bills this month.
	ownInvoice    bool
	mealInvoiced  map[int64]bool
	childNames    map[mealNameKey]int64
	contracts     map[int64][]parentbilling.CareContractVersion
	contractOrder []int64
}

type mealNameKey struct {
	schoolID int64
	name     string
}

// mealChildNames keeps a name only when it identifies one child across
// releases and linked contracts. Zero means several children share it.
func mealChildNames(in careMealInput) map[mealNameKey]int64 {
	names := map[mealNameKey]int64{}
	remember := func(schoolID, studentID int64, first, last string) {
		key := mealNameKey{schoolID: schoolID, name: mealChildName(first, last)}
		if previous, seen := names[key]; seen && previous != studentID {
			names[key] = 0
			return
		}
		names[key] = studentID
	}
	for _, release := range in.releases {
		for _, child := range release.Children {
			remember(release.SchoolID, child.StudentID, child.FirstName, child.LastName)
		}
	}
	for _, version := range in.versions {
		if version.StudentID != nil {
			remember(version.SchoolID, *version.StudentID, version.ChildFirstName, version.ChildLastName)
		}
	}
	return names
}

// addMeals reads the Mensa of the run and adds it to the preview.
func (c *CareBilling) addMeals(ctx context.Context, organizationID int64, run parentbilling.CareRun, month *careMonth,
	versions []parentbilling.CareContractVersion, invoiced map[int64]parentbilling.CareInvoice, recipients map[int64]parentbilling.CareRecipient,
	invoiceService string,
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
	attendance, err := c.meals.MealAttendanceFrom(ctx, mealSchools(tariffs))
	if err != nil {
		return fmt.Errorf("parent billing: read the attendance window of the schools: %w", err)
	}
	schools, err := c.schools.TraegerSchools(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("parent billing: read the schools: %w", err)
	}
	names := make(map[int64]string, len(schools))
	for _, school := range schools {
		names[school.ID] = school.Name
	}
	in := careMealInput{month: run.BillingMonth, today: c.today(), schools: names, tariffs: tariffs, releases: releases,
		billed: billed, versions: versions, invoiced: invoiced, recipients: recipients, attendance: attendance,
		ownInvoice: invoiceService == parentbilling.ServiceMeal}
	if in.ownInvoice {
		if in.mealInvoiced, err = c.mealStore.MealInvoicedContracts(ctx, organizationID, run.BillingMonth); err != nil {
			return err
		}
	}
	billMeals(month, in)
	if in.ownInvoice {
		splitMealInvoices(month)
	}
	return nil
}

// splitMealInvoices puts the Mensa of each contract on an invoice of its
// own (#4686, ADR 0167 point 1): the Mensa positions leave the care's
// invoice with their amount, and an invoice with nothing but Mensa becomes
// the Mensa invoice. Recipient and payment follow later, by the terms of
// each invoice's service.
func splitMealInvoices(month *careMonth) {
	split := make([]parentbilling.CareInvoice, 0, len(month.invoices))
	for _, invoice := range month.invoices {
		if len(invoice.MealPositions) == 0 {
			split = append(split, invoice)
			continue
		}
		meal := invoice
		meal.Service, meal.Positions, meal.Reductions = parentbilling.ServiceMeal, []parentbilling.CarePosition{}, []parentbilling.CareReduction{}
		meal.TotalCents = 0
		for _, position := range invoice.MealPositions {
			meal.TotalCents += position.AmountCents
		}
		if len(invoice.Positions) > 0 || len(invoice.Reductions) > 0 {
			invoice.TotalCents -= meal.TotalCents
			invoice.MealPositions = []parentbilling.CareMealPosition{}
			split = append(split, invoice)
		}
		split = append(split, meal)
	}
	month.invoices = split
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
	in.childNames = mealChildNames(in)
	in.contracts, in.contractOrder = versionsByContract(in.versions)
	month.mealGaps, month.mealUnbilled = []parentbilling.CareMealGap{}, []parentbilling.CareMealUnbilled{}
	from := calendar.NewDate(in.month.Year(), in.month.Month()-(careMealHorizon-1), 1)
	released := map[CareMealKey]parentbilling.CareMealRelease{}
	for _, release := range in.releases {
		released[CareMealKey{SchoolID: release.SchoolID, Month: release.Month}] = release
	}
	for _, school := range mealSchools(in.tariffs) {
		for m := from; !m.After(in.month); m = monthEnd(m).AddDays(1) {
			tariff, priced := mealTariffOn(in.tariffs, school, m)
			if !priced {
				continue
			}
			release, ok := released[CareMealKey{SchoolID: school, Month: m}]
			if !ok {
				addMealGap(month, in, school, m)
				continue
			}
			billRelease(month, in, tariff, release)
		}
	}
	sortMealInvoices(month)
}

// addMealGap names an ended month a school bills its Mensa for without a
// release, and whether its attendance is past the school's retention. A
// school that no longer belongs to the Träger keeps its prices for the
// months it billed, but waits for no release.
func addMealGap(month *careMonth, in careMealInput, school int64, m calendar.Date) {
	name, active := in.schools[school]
	if !active || !monthEnd(m).Before(in.today) {
		return
	}
	from, known := in.attendance[school]
	month.mealGaps = append(month.mealGaps, parentbilling.CareMealGap{SchoolID: school, SchoolName: name, Month: m,
		Expired: known && m.Before(from)})
}

// billRelease bills each child of a release not billed for its month yet.
func billRelease(month *careMonth, in careMealInput, tariff parentbilling.CareMealTariff, release parentbilling.CareMealRelease) {
	for _, child := range release.Children {
		key := CareMealKey{SchoolID: release.SchoolID, StudentID: child.StudentID, Month: release.Month}
		if in.billed[key] {
			continue
		}
		if position, ok := mealPosition(tariff, key, child); ok {
			placeMeal(month, in, position, child, in.schools[release.SchoolID])
		}
	}
}

// sortMealInvoices orders the Mensa months of each invoice and the invoices
// as the preview does.
func sortMealInvoices(month *careMonth) {
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
	contracts := mealChildContracts(in, position.SchoolID, child)
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
		if mealInvoiceIssued(in, contractID) {
			unbilled.Reason = parentbilling.MealUnbilledContractBilled
			month.mealUnbilled = append(month.mealUnbilled, unbilled)
			return
		}
	}
	versions := in.contracts[contracts[0]]
	head := versions[0]
	recipient := in.recipients[head.ContractID]
	month.invoices = append(month.invoices, parentbilling.CareInvoice{
		ContractID: head.ContractID, Service: parentbilling.ServiceCare, SchoolID: head.SchoolID, SchoolName: head.SchoolName,
		ChildFirstName: head.ChildFirstName, ChildLastName: head.ChildLastName, StudentID: studentOf(versions), Recipient: recipient,
		FamilyEmail: parentbilling.FamilyKey(recipient.Email), Reductions: []parentbilling.CareReduction{},
		Positions: []parentbilling.CarePosition{}, MealPositions: []parentbilling.CareMealPosition{position}, TotalCents: position.AmountCents,
	})
}

// mealInvoiceIssued tells whether the invoice that would carry a contract's
// Mensa this month is issued already: the care's invoice when the Mensa
// travels with it, else the Mensa invoice of its own.
func mealInvoiceIssued(in careMealInput, contractID int64) bool {
	if in.ownInvoice {
		return in.mealInvoiced[contractID]
	}
	_, issued := in.invoiced[contractID]
	return issued
}

// mealChildContracts names the contracts at the school in the month of the run
// whose child is the child of the release. Exact links take precedence;
// a name-only fallback must identify one child and one unlinked contract.
func mealChildContracts(in careMealInput, schoolID int64, child parentbilling.CareMealChild) []int64 {
	name := mealChildName(child.FirstName, child.LastName)
	exact, fallback := []int64{}, []int64{}
	for _, id := range in.contractOrder {
		contract := in.contracts[id]
		head := contract[len(contract)-1]
		if head.SchoolID != schoolID {
			continue
		}
		if student := studentOf(contract); student != nil {
			if *student == child.StudentID {
				exact = append(exact, id)
			}
		} else if mealChildName(head.ChildFirstName, head.ChildLastName) == name {
			fallback = append(fallback, id)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	if len(fallback) == 1 && in.childNames[mealNameKey{schoolID: schoolID, name: name}] == child.StudentID {
		return fallback
	}
	return nil
}

func mealChildName(first, last string) string {
	return normalizedName(first + " " + last)
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
