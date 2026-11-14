package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// careModuleHorizon is how far back and ahead the price list looks for
// booked modules: a year each way, as far as a new price may start back
// (#4684); each read stays within the span the contract owner answers.
const careModuleHorizon = 365

// CareStore persists the price list, the monthly runs and their invoices
// of a Träger in its Träger transaction.
type CareStore interface {
	// CarePriceLists reads every price and the heads of the price lists with
	// their free months in one read (#6196); ReplaceCarePrices writes the
	// head of its list and DeleteCarePrices removes it.
	CarePriceLists(ctx context.Context, organizationID int64) ([]parentbilling.CareTariff, []parentbilling.CarePriceListHead, error)
	ReplaceCarePrices(ctx context.Context, organizationID, accountID int64, list parentbilling.CarePriceList) error
	ReplaceCarePriceLists(ctx context.Context, organizationID, accountID int64, lists []parentbilling.CarePriceList) error
	DeleteCarePrices(ctx context.Context, organizationID, schoolID int64, validFrom calendar.Date) (int64, error)
	CareBilledUntil(ctx context.Context, organizationID int64) (map[int64]calendar.Date, error)
	// CareBilledPrices lists the months an issued invoice of the school
	// billed per module and weekday.
	CareBilledPrices(ctx context.Context, organizationID, schoolID int64) ([]parentbilling.CareBilledPrice, error)
	// LockCareBilling holds the monthly billing of the Träger until the
	// transaction ends.
	LockCareBilling(ctx context.Context, organizationID int64) error
	ListCareRuns(ctx context.Context, organizationID int64) ([]parentbilling.CareRun, error)
	// InsertCareRun reports a second calculated run of a month as
	// ErrCareRunOpen.
	InsertCareRun(ctx context.Context, organizationID, accountID int64, start parentbilling.CareRunStart) (parentbilling.CareRun, error)
	CareRun(ctx context.Context, organizationID, runID int64, lock bool) (parentbilling.CareRun, error)
	InvoicedContracts(ctx context.Context, organizationID int64, month calendar.Date) (map[int64]parentbilling.CareInvoice, error)
	InsertCareRunEvent(ctx context.Context, organizationID, runID, accountID int64, action string, details map[string]any) error
	IssueCareInvoices(ctx context.Context, organizationID int64, run parentbilling.CareRun, rule string, year int, first int64, invoices []parentbilling.CareInvoice) error
	MarkCareRunReleased(ctx context.Context, organizationID, runID int64, issuedOn calendar.Date, count int, totalCents int64, rule string) (parentbilling.CareRun, error)
	// MarkCareRunDiscarded discards a calculated run and closes its schools
	// (#6626).
	MarkCareRunDiscarded(ctx context.Context, organizationID, runID int64) (parentbilling.CareRun, error)
	// InsertCareRunHolds stores the children a release held back, as they
	// are at the release; CareRunHolds reads them (ADR 0158 E5).
	InsertCareRunHolds(ctx context.Context, organizationID, runID int64, holds []parentbilling.CareRunHold) error
	CareRunHolds(ctx context.Context, organizationID, runID int64) ([]parentbilling.CareRunHold, error)
	CareRunInvoices(ctx context.Context, organizationID, runID int64) ([]parentbilling.CareInvoice, error)
	// ListContributionRules and CareEvidences feed the rule chain (#4685).
	ListContributionRules(ctx context.Context, organizationID int64, service string) ([]parentbilling.ContributionRule, error)
	CareEvidences(ctx context.Context, organizationID int64, contractIDs []int64) ([]parentbilling.CareEvidence, error)
	// CareCostShares reads the cost bearer shares of the contracts with the
	// name of their cost bearer, all when contractIDs is nil (#4687).
	CareCostShares(ctx context.Context, organizationID int64, contractIDs []int64) ([]parentbilling.CareCostShare, error)
	// CostBearer reads one cost bearer of the Träger.
	CostBearer(ctx context.Context, organizationID, costBearerID int64) (parentbilling.CostBearer, error)
	// CareRunCancellations reads the Stornos of the invoices of a run.
	CareRunCancellations(ctx context.Context, organizationID, runID int64) ([]parentbilling.CareCancellation, error)
	// FamilyChildCounts reads the numbers of the Träger's families (#5918).
	FamilyChildCounts(ctx context.Context, organizationID int64) ([]parentbilling.FamilyChildCount, error)
	ListNumberRanges(ctx context.Context, organizationID int64) ([]parentbilling.NumberRange, error)
	ReserveNumbers(ctx context.Context, organizationID int64, service string, year, count int) (int64, error)
	// The terms, the family deviations and the due dates of the run decide
	// the recipient and the payment of every invoice (#4689).
	paymentTermsReader
	// CareReviewStore keeps the review of a run (#5916).
	CareReviewStore
	// CareAmendmentStore keeps the changes of its invoices (#6197).
	CareAmendmentStore
	// DocumentKindStore keeps the kind of document of each service (#4688).
	DocumentKindStore
	// BookingAssignmentReader names where the positions book (#4821).
	BookingAssignmentReader
}

// CareBilling serves parentbilling.CareBilling. The contract versions and
// their parties come from the Betreuungsvertrag through the root-bound
// port, the month rule from the settings of the Träger; the prices, runs
// and invoices are the owner's own rows.
type CareBilling struct {
	store     CareStore
	contracts parentbilling.CareContracts
	schools   parentbilling.CareSchools
	settings  parentbilling.CareBillingSettings
	renderer  TableRenderer
	kinds     documentKindGate
	today     func() calendar.Date
	// mealStore and meals bill the Mensa (#4686).
	mealStore CareMealStore
	meals     parentbilling.CareMeals
}

var _ parentbilling.CareBilling = (*CareBilling)(nil)

// NewCareBilling builds the monthly billing with its Mensa (#4686). today
// is the issue date of a release without an invoice date and the start of
// the price list's look ahead; renderer renders the review list (#5916);
// templates tell whether a fee notice has its legal remedy (#4688).
func NewCareBilling(store CareStore, contracts parentbilling.CareContracts, schools parentbilling.CareSchools,
	settings parentbilling.CareBillingSettings, renderer TableRenderer, mealStore CareMealStore, meals parentbilling.CareMeals, templates parentbilling.BillingDocumentTemplates,
	today func() calendar.Date,
) *CareBilling {
	if store == nil || contracts == nil || schools == nil || settings == nil || renderer == nil || mealStore == nil || meals == nil || today == nil {
		panic("parent billing: the store, the care contracts, the schools, the settings, the renderer, the Mensa and the clock are required")
	}
	return &CareBilling{store: store, contracts: contracts, schools: schools, settings: settings, renderer: renderer,
		mealStore: mealStore, meals: meals, kinds: newDocumentKindGate(store, templates), today: today}
}

// CareTariffs reads the price list with the schools of the Träger and the
// modules the contracts book from this month on.
func (c *CareBilling) CareTariffs(ctx context.Context, reviewer parentbilling.HolidayReviewer) (parentbilling.CareTariffs, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareTariffs{}, err
	}
	schools, err := c.schools.TraegerSchools(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.CareTariffs{}, fmt.Errorf("parent billing: read the schools: %w", err)
	}
	versions, err := c.bookedVersions(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.CareTariffs{}, err
	}
	tariffs, heads, err := c.store.CarePriceLists(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.CareTariffs{}, err
	}
	billed, err := c.store.CareBilledUntil(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.CareTariffs{}, err
	}
	modules := careModules(versions, tariffs)
	names := map[int64]string{}
	for _, school := range schools {
		names[school.ID] = school.Name
	}
	for i := range modules {
		if modules[i].SchoolName == "" {
			modules[i].SchoolName = names[modules[i].SchoolID]
		}
	}
	return parentbilling.CareTariffs{Schools: schools, Modules: modules, Tariffs: withValidity(tariffs), Lists: heads, BilledUntil: billed}, nil
}

// bookedVersions reads the versions in force in the year before and the
// year after the current month, so a module whose contracts ended can still
// get the price of a month not billed yet.
func (c *CareBilling) bookedVersions(ctx context.Context, organizationID int64) ([]parentbilling.CareContractVersion, error) {
	current := firstOfMonth(c.today())
	past := calendar.NewDate(current.Year()-1, current.Month(), 1)
	earlier, err := c.contracts.VersionsInForce(ctx, organizationID, past, current.AddDays(-1))
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the booked modules: %w", err)
	}
	later, err := c.contracts.VersionsInForce(ctx, organizationID, current, current.AddDays(careModuleHorizon))
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the booked modules: %w", err)
	}
	return append(earlier, later...), nil
}

// ownSchool reports whether the school belongs to the Träger.
func (c *CareBilling) ownSchool(ctx context.Context, organizationID, schoolID int64) (bool, error) {
	schools, err := c.schools.TraegerSchools(ctx, organizationID)
	if err != nil {
		return false, fmt.Errorf("parent billing: read the schools: %w", err)
	}
	for _, school := range schools {
		if school.ID == schoolID {
			return true, nil
		}
	}
	return false, nil
}

// careModules lists the modules the versions book and those the price
// list names, by school and module. Counts and weekdays come from the
// versions only.
func careModules(versions []parentbilling.CareContractVersion, tariffs []parentbilling.CareTariff) []parentbilling.CareModule {
	book := moduleBook{modules: map[moduleKey]*parentbilling.CareModule{}, contracts: map[moduleKey]map[int64]bool{}}
	for _, version := range versions {
		book.addVersion(version)
	}
	for _, tariff := range tariffs {
		book.ensure(moduleKey{tariff.SchoolID, tariff.CareOfferingID}, "", tariff.ModuleName)
	}
	result := make([]parentbilling.CareModule, 0, len(book.modules))
	for k, module := range book.modules {
		module.ContractCount = len(book.contracts[k])
		slices.Sort(module.BookedDaysPerWeek)
		slices.SortFunc(module.BookedWeekdays, func(a, b string) int { return compareOrdered(weekdayOrder[a], weekdayOrder[b]) })
		result = append(result, *module)
	}
	slices.SortFunc(result, func(a, b parentbilling.CareModule) int {
		return firstNonZero(compareOrdered(a.SchoolID, b.SchoolID), compareOrdered(a.EndsAt, b.EndsAt), compareOrdered(a.ModuleName, b.ModuleName))
	})
	return result
}

// moduleBook collects the modules of the price list with their contracts.
type moduleBook struct {
	modules   map[moduleKey]*parentbilling.CareModule
	contracts map[moduleKey]map[int64]bool
}

func (b moduleBook) ensure(k moduleKey, schoolName, moduleName string) *parentbilling.CareModule {
	module, ok := b.modules[k]
	if !ok {
		module = &parentbilling.CareModule{SchoolID: k.schoolID, SchoolName: schoolName, CareOfferingID: k.offeringID,
			ModuleName: moduleName, BookedWeekdays: []string{}, BookedDaysPerWeek: []int{}}
		b.modules[k] = module
		b.contracts[k] = map[int64]bool{}
	}
	return module
}

// addVersion adds the booked weekdays, day counts and the contract of a
// version to its modules.
func (b moduleBook) addVersion(version parentbilling.CareContractVersion) {
	for _, service := range version.Services {
		k := moduleKey{version.SchoolID, service.CareOfferingID}
		module := b.ensure(k, version.SchoolName, service.ModuleName)
		if module.EndsAt == "" {
			module.EndsAt = service.EndsAt
		}
		if !slices.Contains(module.BookedWeekdays, service.Weekday) {
			module.BookedWeekdays = append(module.BookedWeekdays, service.Weekday)
		}
		b.contracts[k][version.ContractID] = true
	}
	for offering, counts := range bookedDayCounts(version) {
		module := b.modules[moduleKey{version.SchoolID, offering}]
		for _, count := range counts {
			if !slices.Contains(module.BookedDaysPerWeek, count) {
				module.BookedDaysPerWeek = append(module.BookedDaysPerWeek, count)
			}
		}
	}
}

// SaveCarePriceList sets the prices of a school from a month on. A month an
// issued invoice of the school already bills keeps its prices: the list
// starts after it.
func (c *CareBilling) SaveCarePriceList(ctx context.Context, reviewer parentbilling.HolidayReviewer, list parentbilling.CarePriceList) (parentbilling.CareTariffs, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareTariffs{}, err
	}
	list, err := normalizePriceList(list)
	if err != nil {
		return parentbilling.CareTariffs{}, err
	}
	if own, err := c.ownSchool(ctx, reviewer.OrganizationID, list.SchoolID); err != nil || !own {
		if err != nil {
			return parentbilling.CareTariffs{}, err
		}
		return parentbilling.CareTariffs{}, validation.Errors{"school_id": errors.New("is no school of the Träger")}
	}
	if err := c.keepBilledPrices(ctx, reviewer.OrganizationID, list.SchoolID, list.ValidFrom, &list); err != nil {
		return parentbilling.CareTariffs{}, err
	}
	if err := c.store.ReplaceCarePrices(ctx, reviewer.OrganizationID, reviewer.AccountID, list); err != nil {
		return parentbilling.CareTariffs{}, err
	}
	return c.CareTariffs(ctx, reviewer)
}

// SaveCareModulePrices merges a school's complete module batch in memory,
// checks billed months once, and persists every affected month as one batch.
// It deliberately builds no organization-wide response view.
func (c *CareBilling) SaveCareModulePrices(ctx context.Context, reviewer parentbilling.HolidayReviewer, modules []parentbilling.CareModulePriceList) error {
	if err := validReviewer(reviewer); err != nil {
		return err
	}
	if len(modules) == 0 {
		return nil
	}
	schoolID, err := moduleBatchSchool(modules)
	if err != nil {
		return err
	}
	if own, err := c.ownSchool(ctx, reviewer.OrganizationID, schoolID); err != nil || !own {
		if err != nil {
			return err
		}
		return validation.Errors{"school_id": errors.New("is no school of the Träger")}
	}
	if err := c.store.LockCareBilling(ctx, reviewer.OrganizationID); err != nil {
		return err
	}
	tariffs, heads, err := c.store.CarePriceLists(ctx, reviewer.OrganizationID)
	if err != nil {
		return err
	}
	billed, err := c.store.CareBilledPrices(ctx, reviewer.OrganizationID, schoolID)
	if err != nil {
		return err
	}
	lists, after, headsAfter, err := mergeModulePrices(tariffs, heads, modules)
	if err != nil {
		return err
	}
	earliest := lists[0].ValidFrom
	if changesBilledPrice(schoolID, earliest, billed, newPriceBook(tariffs), newPriceBook(after)) || freesBilledMonth(schoolID, earliest, billed, newFreeMonths(headsAfter)) {
		return parentbilling.ErrCareTariffMonthBilled
	}
	return c.store.ReplaceCarePriceLists(ctx, reviewer.OrganizationID, reviewer.AccountID, lists)
}

func moduleBatchSchool(modules []parentbilling.CareModulePriceList) (int64, error) {
	schoolID := modules[0].SchoolID
	for _, module := range modules {
		if module.SchoolID != schoolID || schoolID <= 0 || module.CareOfferingID <= 0 {
			return schoolID, validation.Errors{"care_offering_id": errors.New("modules must belong to one school")}
		}
	}
	return schoolID, nil
}

// DeleteCarePriceList removes the prices of a school that start on a month
// no invoice bills yet.
func (c *CareBilling) DeleteCarePriceList(ctx context.Context, reviewer parentbilling.HolidayReviewer, schoolID int64, validFrom calendar.Date) (parentbilling.CareTariffs, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareTariffs{}, err
	}
	if schoolID <= 0 || !isMonthStart(validFrom) {
		return parentbilling.CareTariffs{}, parentbilling.ErrCareTariffNotFound
	}
	if own, err := c.ownSchool(ctx, reviewer.OrganizationID, schoolID); err != nil || !own {
		if err != nil {
			return parentbilling.CareTariffs{}, err
		}
		return parentbilling.CareTariffs{}, parentbilling.ErrCareTariffNotFound
	}
	if err := c.keepBilledPrices(ctx, reviewer.OrganizationID, schoolID, validFrom, nil); err != nil {
		return parentbilling.CareTariffs{}, err
	}
	deleted, err := c.store.DeleteCarePrices(ctx, reviewer.OrganizationID, schoolID, validFrom)
	if err != nil {
		return parentbilling.CareTariffs{}, err
	}
	if deleted == 0 {
		return parentbilling.CareTariffs{}, parentbilling.ErrCareTariffNotFound
	}
	return c.CareTariffs(ctx, reviewer)
}

// ListCareRuns lists the monthly runs, the latest month first, and
// suggests the month a new run bills: the current month, or the first one
// after it without a released run. The list carries no personal data.
func (c *CareBilling) ListCareRuns(ctx context.Context, organizationID int64) (parentbilling.CareRunList, error) {
	if organizationID <= 0 {
		return parentbilling.CareRunList{}, errTraegerRequired
	}
	runs, err := c.store.ListCareRuns(ctx, organizationID)
	if err != nil {
		return parentbilling.CareRunList{}, fmt.Errorf("parent billing: list care runs: %w", err)
	}
	return parentbilling.CareRunList{Runs: runs, SuggestedMonth: suggestedMonth(c.today(), runs)}, nil
}

// suggestedMonth is the first month from today's on without a released run.
func suggestedMonth(today calendar.Date, runs []parentbilling.CareRun) calendar.Date {
	released := map[calendar.Date]bool{}
	for _, run := range runs {
		if run.Status == parentbilling.RunReleased {
			released[run.BillingMonth] = true
		}
	}
	month := firstOfMonth(today)
	for released[month] {
		month = monthEnd(month).AddDays(1)
	}
	return month
}

// StartCareRun starts a calculated run of a month for its schools and with
// its invoice date (#5916). A school is in at most one calculated run per
// month; without schools the run bills every school of the Träger that no
// open run of the month bills.
func (c *CareBilling) StartCareRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, start parentbilling.CareRunStart) (parentbilling.CareRun, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareRun{}, err
	}
	if !isMonthStart(start.BillingMonth) {
		return parentbilling.CareRun{}, validation.Errors{"billing_month": errors.New("must be the first day of a month")}
	}
	if err := checkInvoiceDate(start.BillingMonth, start.InvoiceDate); err != nil {
		return parentbilling.CareRun{}, err
	}
	schools, err := c.runSchools(ctx, reviewer.OrganizationID, start.BillingMonth, start.SchoolIDs)
	if err != nil {
		return parentbilling.CareRun{}, err
	}
	start.SchoolIDs = schools
	run, err := c.store.InsertCareRun(ctx, reviewer.OrganizationID, reviewer.AccountID, start)
	if err != nil {
		return parentbilling.CareRun{}, err
	}
	if err := c.store.InsertCareRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventCreate,
		map[string]any{"school_ids": run.SchoolIDs}); err != nil {
		return parentbilling.CareRun{}, fmt.Errorf("parent billing: record care run: %w", err)
	}
	return run, nil
}

// CareRun shows a run with the preview of today or its issued invoices and
// records the read.
func (c *CareBilling) CareRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64) (parentbilling.CareRunDetail, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	if runID <= 0 {
		return parentbilling.CareRunDetail{}, parentbilling.ErrCareRunNotFound
	}
	run, err := c.store.CareRun(ctx, reviewer.OrganizationID, runID, false)
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	detail, err := c.detail(ctx, reviewer, run)
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	if err := c.store.InsertCareRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventRead,
		map[string]any{"contract_ids": shownContracts(detail), "meal_student_ids": shownMealChildren(detail)}); err != nil {
		return parentbilling.CareRunDetail{}, fmt.Errorf("parent billing: record care run read: %w", err)
	}
	return detail, nil
}

// ReleaseCareRun issues the invoices of the preview, numbered from the
// number range of the monthly care without a gap, and closes the run. It
// holds the run and the monthly billing of the Träger, so a parallel
// release waits and then finds the run released. The caller rolls back
// when this fails.
func (c *CareBilling) ReleaseCareRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, release parentbilling.CareRelease) (parentbilling.CareRunDetail, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	if runID <= 0 {
		return parentbilling.CareRunDetail{}, parentbilling.ErrCareRunNotFound
	}
	if err := c.store.LockCareBilling(ctx, reviewer.OrganizationID); err != nil {
		return parentbilling.CareRunDetail{}, fmt.Errorf("parent billing: hold the monthly billing: %w", err)
	}
	run, err := c.store.CareRun(ctx, reviewer.OrganizationID, runID, true)
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	if run.Status != parentbilling.RunCalculated {
		return parentbilling.CareRunDetail{}, closedRunError(run.Status)
	}
	rule, month, issuedOn, err := c.issueReviewed(ctx, reviewer.OrganizationID, run, release)
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	released, err := c.store.MarkCareRunReleased(ctx, reviewer.OrganizationID, run.ID, issuedOn,
		len(month.invoices), month.total(), rule)
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	if err := c.store.InsertCareRunHolds(ctx, reviewer.OrganizationID, run.ID, holdSnapshot(month.duplicates)); err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	if err := c.store.InsertCareRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventRelease,
		map[string]any{"document_count": len(month.invoices), "total_cents": month.total(), "month_rule": rule,
			"free_of_charge":     month.free,
			"duplicate_children": duplicateChildren(month.duplicates), "issued_on": issuedOn.String(),
			"excluded_contract_ids": excludedContracts(month.excluded),
			"excluded_invoices":     excludedInvoices(month.excluded)}); err != nil {
		return parentbilling.CareRunDetail{}, fmt.Errorf("parent billing: record care release: %w", err)
	}
	released.FreeOfCharge = month.free
	return c.detail(ctx, reviewer, released)
}

// issue numbers the invoices from the counter of the issue year and stores
// them. A number another invoice of the Träger already carries is
// ErrInvoiceNumberTaken.
func (c *CareBilling) issue(ctx context.Context, organizationID int64, run parentbilling.CareRun, rule, pattern string,
	issuedOn calendar.Date, invoices []parentbilling.CareInvoice,
) error {
	year := numberYear(pattern, issuedOn.Year())
	first, err := c.store.ReserveNumbers(ctx, organizationID, parentbilling.ServiceCare, year, len(invoices))
	if err != nil {
		return fmt.Errorf("parent billing: reserve invoice numbers: %w", err)
	}
	for i := range invoices {
		invoices[i].Number = formatInvoiceNumber(pattern, issuedOn.Year(), first+int64(i))
		invoices[i].IssuedOn = &issuedOn
	}
	resolver, err := bookingResolverOf(ctx, c.store, c.settings, organizationID)
	if err != nil {
		return err
	}
	freezeCareAccounts(resolver, invoices)
	return c.store.IssueCareInvoices(ctx, organizationID, run, rule, year, first, invoices)
}

// preview computes the draft invoices of a calculated run with the rule of
// the Träger now.
func (c *CareBilling) preview(ctx context.Context, organizationID int64, run parentbilling.CareRun) (string, careMonth, error) {
	rule, err := c.settings.CareMonthRule(ctx, organizationID)
	if err != nil {
		return "", careMonth{}, fmt.Errorf("parent billing: read the month rule: %w", err)
	}
	return c.monthWith(ctx, organizationID, run, rule)
}

// monthWith computes the month of a run under a month rule: the drafts of
// the contracts no invoice of the month bills yet, the bookings without a
// price and the children with a duplicate contract.
func (c *CareBilling) monthWith(ctx context.Context, organizationID int64, run parentbilling.CareRun, rule string) (string, careMonth, error) {
	if rule != parentbilling.MonthRuleVersionOnFirst && rule != parentbilling.MonthRuleProRata {
		return "", careMonth{}, fmt.Errorf("parent billing: unknown month rule %q", rule)
	}
	in, err := c.careMonthInput(ctx, organizationID, run, rule)
	if err != nil {
		return "", careMonth{}, err
	}
	month, err := buildCareMonth(in)
	if err != nil {
		return rule, month, err
	}
	// After the rule chain: the chain counts the family by the address of
	// the registration, the payer only receives the invoice (#4689).
	scope, err := debtorNumberScopeOf(ctx, c.settings, organizationID)
	if err != nil {
		return rule, month, err
	}
	payments, err := readInvoicePayments(ctx, c.store, organizationID, run.ID, 0, scope)
	if err != nil {
		return rule, month, err
	}
	payments.contractChildren = contractChildKeys(in.versions)
	// The Mensa joins after the chain: no rule reduces it (ADR 0168). Its
	// terms say on whose invoice it travels (ADR 0167). The shares that
	// cover the Mensa take their part of it there (ADR 0168 point 12).
	if err := c.addMeals(ctx, organizationID, run, &month, in.versions, in.invoiced, in.recipients,
		payments.terms.InvoiceService(parentbilling.ServiceMeal)); err != nil {
		return rule, month, err
	}
	applyMealCostShares(&month, in.costShares, run.BillingMonth, in.invoiced)
	// The payment term counts from the invoice date the release puts on the
	// invoices (#5916), today without one.
	issuedOn := c.today()
	if run.InvoiceDate != nil {
		issuedOn = *run.InvoiceDate
	}
	for i := range month.invoices {
		payments.careInvoice(&month.invoices[i], issuedOn)
	}
	month.dueDates, month.deviations, month.mandates = payments.run, payments.deviationsRead(), payments.mandatesRead()
	month.childNumbers = payments.childNumbersRead()
	return rule, month, nil
}

// careMonthInput reads what the month of a run is computed from: the
// contract versions in force, prices, issued invoices, parties, rules,
// evidences, cost bearer shares and the numbers of the families.
func (c *CareBilling) careMonthInput(ctx context.Context, organizationID int64, run parentbilling.CareRun, rule string) (careMonthInput, error) {
	in := careMonthInput{month: run.BillingMonth, rule: rule}
	var err error
	if in.versions, err = c.contracts.VersionsInForce(ctx, organizationID, run.BillingMonth, monthEnd(run.BillingMonth)); err != nil {
		return in, fmt.Errorf("parent billing: read the contract versions: %w", err)
	}
	contracts := versionContracts(in.versions)
	if in.tariffs, in.lists, err = c.store.CarePriceLists(ctx, organizationID); err != nil {
		return in, err
	}
	if in.invoiced, err = c.store.InvoicedContracts(ctx, organizationID, run.BillingMonth); err != nil {
		return in, err
	}
	if in.recipients, err = c.contracts.Recipients(ctx, organizationID, contracts); err != nil {
		return in, fmt.Errorf("parent billing: read the contract parties: %w", err)
	}
	if in.rules, err = c.store.ListContributionRules(ctx, organizationID, parentbilling.ServiceCare); err != nil {
		return in, fmt.Errorf("parent billing: read the care rules: %w", err)
	}
	if in.evidences, err = c.store.CareEvidences(ctx, organizationID, contracts); err != nil {
		return in, err
	}
	if in.costShares, err = c.store.CareCostShares(ctx, organizationID, contracts); err != nil {
		return in, err
	}
	in.familyCounts, err = c.store.FamilyChildCounts(ctx, organizationID)
	return in, err
}

// detail is the run with its preview or its issued invoices and the number
// range the release uses.
func (c *CareBilling) detail(ctx context.Context, reviewer parentbilling.HolidayReviewer, run parentbilling.CareRun) (parentbilling.CareRunDetail, error) {
	detail := parentbilling.CareRunDetail{CareRun: run, Unpriced: []parentbilling.CareUnpriced{}, Duplicates: []parentbilling.CareDuplicate{},
		MealGaps: []parentbilling.CareMealGap{}, MealUnbilled: []parentbilling.CareMealUnbilled{}, MealChanged: []parentbilling.CareMealChanged{}}
	if run.Status == parentbilling.RunDiscarded {
		return c.discardedDetail(ctx, reviewer.OrganizationID, detail)
	}
	if run.Status == parentbilling.RunReleased {
		if err := c.releasedDetail(ctx, reviewer.OrganizationID, &detail); err != nil {
			return parentbilling.CareRunDetail{}, err
		}
	} else if err := c.reviewDetail(ctx, reviewer, &detail); err != nil {
		return parentbilling.CareRunDetail{}, err
	} else {
		detail.CostBearers = careCostBearerTotals(detail.Invoices, nil)
	}
	detail.DuplicateChildren = duplicateChildren(detail.Duplicates)
	withMealPositions(detail.Invoices)
	for _, invoice := range detail.Invoices {
		if invoice.Exclusion != nil {
			continue
		}
		detail.PreviewCount++
		detail.PreviewCents += invoice.TotalCents
	}
	pattern, err := numberPatternOf(ctx, c.store, reviewer.OrganizationID, parentbilling.ServiceCare)
	switch {
	case errors.Is(err, parentbilling.ErrNumberRangeMissing):
	case err != nil:
		return parentbilling.CareRunDetail{}, err
	default:
		detail.NumberPattern, detail.NumberRangeSet = pattern, true
	}
	return detail, nil
}

// releasedDetail shows what the release of a run stored (E5): its invoices
// and the snapshot of the children it held back. It reads no contract again,
// so a later change, withdrawal or new contract does not reach it.
func (c *CareBilling) releasedDetail(ctx context.Context, organizationID int64, detail *parentbilling.CareRunDetail) error {
	invoices, err := c.store.CareRunInvoices(ctx, organizationID, detail.ID)
	if err != nil {
		return err
	}
	keyIssuedPositions(invoices)
	detail.Invoices = invoices
	holds, err := c.store.CareRunHolds(ctx, organizationID, detail.ID)
	if err != nil {
		return err
	}
	detail.Duplicates = holdView(holds)
	detail.DocumentKind = issuedCareDocumentKind(invoices)
	// The dates of the release, which its invoices copied.
	payments, err := readInvoicePayments(ctx, c.store, organizationID, detail.ID, 0, parentbilling.DebtorNumberScopePayer)
	if err != nil {
		return err
	}
	detail.DueDates = payments.run.InPaymentOrder()
	cancellations, err := c.store.CareRunCancellations(ctx, organizationID, detail.ID)
	if err != nil {
		return err
	}
	detail.CostBearers = careCostBearerTotals(invoices, cancelledInvoices(cancellations))
	return nil
}

// duplicateChildren counts the children behind duplicate entries: contracts
// that meet in an entry, directly or through a third one, are one child.
func duplicateChildren(duplicates []parentbilling.CareDuplicate) int {
	child := map[int64]int64{}
	root := func(id int64) int64 {
		for child[id] != id {
			id = child[id]
		}
		return id
	}
	for _, duplicate := range duplicates {
		for _, contract := range duplicate.Contracts {
			if _, seen := child[contract.ContractID]; !seen {
				child[contract.ContractID] = contract.ContractID
			}
		}
		for _, contract := range duplicate.Contracts[1:] {
			child[root(contract.ContractID)] = root(duplicate.Contracts[0].ContractID)
		}
	}
	children := 0
	for id := range child {
		if root(id) == id {
			children++
		}
	}
	return children
}

func versionContracts(versions []parentbilling.CareContractVersion) []int64 {
	ids := []int64{}
	for _, version := range versions {
		if !slices.Contains(ids, version.ContractID) {
			ids = append(ids, version.ContractID)
		}
	}
	return ids
}

// shownContracts names the contracts whose child a detail shows: on an
// invoice, without a price or as a duplicate.
func shownContracts(detail parentbilling.CareRunDetail) []int64 {
	ids := []int64{}
	for _, invoice := range detail.Invoices {
		ids = append(ids, invoice.ContractID)
	}
	for _, unpriced := range detail.Unpriced {
		if !slices.Contains(ids, unpriced.ContractID) {
			ids = append(ids, unpriced.ContractID)
		}
	}
	for _, duplicate := range detail.Duplicates {
		for _, contract := range duplicate.Contracts {
			if !slices.Contains(ids, contract.ContractID) {
				ids = append(ids, contract.ContractID)
			}
		}
	}
	return ids
}

// issuedCareDocumentKind is the kind a released monthly run froze: the kind
// of its invoices, which one release issued together (#4688).
func issuedCareDocumentKind(invoices []parentbilling.CareInvoice) string {
	if len(invoices) == 0 {
		return ""
	}
	return invoices[0].DocumentKind
}
