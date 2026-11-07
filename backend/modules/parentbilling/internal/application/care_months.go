package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// careModuleHorizon is how far back and ahead the price list looks for
// booked modules: a year each way, as far as a new price may start back
// (#4684); each read stays within the span the contract owner answers.
const careModuleHorizon = 365

// CareStore persists the price list, the monthly runs and their invoices
// of a Träger in its Träger transaction.
type CareStore interface {
	CareTariffs(ctx context.Context, organizationID int64) ([]parentbilling.CareTariff, error)
	ReplaceCarePrices(ctx context.Context, organizationID, accountID int64, list parentbilling.CarePriceList) error
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
	InsertCareRun(ctx context.Context, organizationID, accountID int64, month calendar.Date) (parentbilling.CareRun, error)
	CareRun(ctx context.Context, organizationID, runID int64, lock bool) (parentbilling.CareRun, error)
	InvoicedContracts(ctx context.Context, organizationID int64, month calendar.Date) (map[int64]bool, error)
	InsertCareRunEvent(ctx context.Context, organizationID, runID, accountID int64, action string, details map[string]any) error
	IssueCareInvoices(ctx context.Context, organizationID int64, run parentbilling.CareRun, rule string, year int, first int64, invoices []parentbilling.CareInvoice) error
	MarkCareRunReleased(ctx context.Context, organizationID, runID int64, issuedOn calendar.Date, count int, totalCents int64, rule string, held []int64) (parentbilling.CareRun, error)
	CareRunInvoices(ctx context.Context, organizationID, runID int64) ([]parentbilling.CareInvoice, error)
	ListNumberRanges(ctx context.Context, organizationID int64) ([]parentbilling.NumberRange, error)
	ReserveNumbers(ctx context.Context, organizationID int64, service string, year, count int) (int64, error)
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
	today     func() calendar.Date
}

var _ parentbilling.CareBilling = (*CareBilling)(nil)

// NewCareBilling builds the monthly billing. today is the issue date of a
// release and the start of the price list's look ahead.
func NewCareBilling(store CareStore, contracts parentbilling.CareContracts, schools parentbilling.CareSchools,
	settings parentbilling.CareBillingSettings, today func() calendar.Date,
) *CareBilling {
	if store == nil || contracts == nil || schools == nil || settings == nil || today == nil {
		panic("parent billing: the store, the care contracts, the schools, the settings and the clock are required")
	}
	return &CareBilling{store: store, contracts: contracts, schools: schools, settings: settings, today: today}
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
	tariffs, err := c.store.CareTariffs(ctx, reviewer.OrganizationID)
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
	return parentbilling.CareTariffs{Schools: schools, Modules: modules, Tariffs: withValidity(tariffs), BilledUntil: billed}, nil
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
	type key struct{ school, offering int64 }
	modules := map[key]*parentbilling.CareModule{}
	contracts := map[key]map[int64]bool{}
	ensure := func(k key, schoolName, moduleName string) *parentbilling.CareModule {
		module, ok := modules[k]
		if !ok {
			module = &parentbilling.CareModule{SchoolID: k.school, SchoolName: schoolName, CareOfferingID: k.offering,
				ModuleName: moduleName, BookedWeekdays: []string{}}
			modules[k] = module
			contracts[k] = map[int64]bool{}
		}
		return module
	}
	for _, version := range versions {
		for _, service := range version.Services {
			k := key{version.SchoolID, service.CareOfferingID}
			module := ensure(k, version.SchoolName, service.ModuleName)
			if module.EndsAt == "" {
				module.EndsAt = service.EndsAt
			}
			if !slices.Contains(module.BookedWeekdays, service.Weekday) {
				module.BookedWeekdays = append(module.BookedWeekdays, service.Weekday)
			}
			contracts[k][version.ContractID] = true
		}
	}
	for _, tariff := range tariffs {
		ensure(key{tariff.SchoolID, tariff.CareOfferingID}, "", tariff.ModuleName)
	}
	result := make([]parentbilling.CareModule, 0, len(modules))
	for k, module := range modules {
		module.ContractCount = len(contracts[k])
		slices.SortFunc(module.BookedWeekdays, func(a, b string) int { return compareOrdered(weekdayOrder[a], weekdayOrder[b]) })
		result = append(result, *module)
	}
	slices.SortFunc(result, func(a, b parentbilling.CareModule) int {
		return firstNonZero(compareOrdered(a.SchoolID, b.SchoolID), compareOrdered(a.EndsAt, b.EndsAt), compareOrdered(a.ModuleName, b.ModuleName))
	})
	return result
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
	if err := c.keepBilledPrices(ctx, reviewer.OrganizationID, list.SchoolID, list.ValidFrom, list.Prices); err != nil {
		return parentbilling.CareTariffs{}, err
	}
	if err := c.store.ReplaceCarePrices(ctx, reviewer.OrganizationID, reviewer.AccountID, list); err != nil {
		return parentbilling.CareTariffs{}, err
	}
	return c.CareTariffs(ctx, reviewer)
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

// keepBilledPrices holds the monthly billing and refuses a change of the
// price list of a school from validFrom on (replacement nil deletes it) that
// would change a price an issued invoice billed: every module and weekday
// billed in a month from validFrom on keeps its price. A booked weekday
// without a price may still get one, also in a billed month, so a contract
// a run left out can be billed by the next run of the month.
func (c *CareBilling) keepBilledPrices(ctx context.Context, organizationID, schoolID int64, validFrom calendar.Date, replacement []parentbilling.CarePrice) error {
	if err := c.store.LockCareBilling(ctx, organizationID); err != nil {
		return fmt.Errorf("parent billing: hold the monthly billing: %w", err)
	}
	billed, err := c.store.CareBilledPrices(ctx, organizationID, schoolID)
	if err != nil {
		return err
	}
	tariffs, err := c.store.CareTariffs(ctx, organizationID)
	if err != nil {
		return err
	}
	after := make([]parentbilling.CareTariff, 0, len(tariffs)+len(replacement))
	for _, tariff := range tariffs {
		if tariff.SchoolID != schoolID || tariff.ValidFrom != validFrom {
			after = append(after, tariff)
		}
	}
	for _, price := range replacement {
		after = append(after, parentbilling.CareTariff{SchoolID: schoolID, CareOfferingID: price.CareOfferingID,
			Weekday: price.Weekday, ValidFrom: validFrom, MonthlyCents: price.MonthlyCents})
	}
	if changesBilledPrice(schoolID, validFrom, billed, newPriceBook(tariffs), newPriceBook(after)) {
		return parentbilling.ErrCareTariffMonthBilled
	}
	return nil
}

// changesBilledPrice reports a billed weekday whose price in its month
// differs between the two price lists.
func changesBilledPrice(schoolID int64, validFrom calendar.Date, billed []parentbilling.CareBilledPrice, before, after priceBook) bool {
	for _, entry := range billed {
		if entry.Month.Before(validFrom) {
			continue
		}
		was, wasSet := before.on(schoolID, entry.CareOfferingID, entry.Weekday, entry.Month)
		now, nowSet := after.on(schoolID, entry.CareOfferingID, entry.Weekday, entry.Month)
		if wasSet != nowSet || was != now {
			return true
		}
	}
	return false
}

// normalizePriceList checks a price list: a school, the first day of a
// month, known weekdays, one price per module and weekday, amounts in cent.
func normalizePriceList(list parentbilling.CarePriceList) (parentbilling.CarePriceList, error) {
	errs := validation.Errors{}
	if list.SchoolID <= 0 {
		errs["school_id"] = errors.New("is required")
	}
	if !isMonthStart(list.ValidFrom) {
		errs["valid_from"] = errors.New("must be the first day of a month")
	}
	seen := map[priceKey]bool{}
	for i, price := range list.Prices {
		key := priceKey{offeringID: price.CareOfferingID, weekday: price.Weekday}
		switch {
		case price.CareOfferingID <= 0 || weekdayOrder[price.Weekday] == 0:
			errs["prices"] = errors.New("needs a module and a weekday")
		case seen[key]:
			errs["prices"] = errors.New("names a module and weekday twice")
		case price.MonthlyCents < 0 || price.MonthlyCents > parentbilling.MaxMonthlyPriceCents:
			errs["prices"] = errors.New("must be amounts in cent, not negative")
		case strings.TrimSpace(price.ModuleName) == "" || len([]rune(price.ModuleName)) > parentbilling.MaxModuleNameLength:
			errs["prices"] = errors.New("needs the name of the module")
		}
		seen[key] = true
		price.ModuleName = strings.TrimSpace(price.ModuleName)
		list.Prices[i] = price
	}
	if len(errs) > 0 {
		return list, errs
	}
	return list, nil
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

// StartCareRun starts a calculated run of a month. A month has at most one
// calculated run.
func (c *CareBilling) StartCareRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, month calendar.Date) (parentbilling.CareRun, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareRun{}, err
	}
	if !isMonthStart(month) {
		return parentbilling.CareRun{}, validation.Errors{"billing_month": errors.New("must be the first day of a month")}
	}
	run, err := c.store.InsertCareRun(ctx, reviewer.OrganizationID, reviewer.AccountID, month)
	if err != nil {
		return parentbilling.CareRun{}, err
	}
	if err := c.store.InsertCareRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventCreate, nil); err != nil {
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
		map[string]any{"contract_ids": shownContracts(detail)}); err != nil {
		return parentbilling.CareRunDetail{}, fmt.Errorf("parent billing: record care run read: %w", err)
	}
	return detail, nil
}

// ReleaseCareRun issues the invoices of the preview, numbered from the
// number range of the monthly care without a gap, and closes the run. It
// holds the run and the monthly billing of the Träger, so a parallel
// release waits and then finds the run released. The caller rolls back
// when this fails.
func (c *CareBilling) ReleaseCareRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, release parentbilling.BillingRelease) (parentbilling.CareRunDetail, error) {
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
		return parentbilling.CareRunDetail{}, parentbilling.ErrBillingRunReleased
	}
	pattern, err := numberPatternOf(ctx, c.store, reviewer.OrganizationID, parentbilling.ServiceCare)
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	rule, month, err := c.preview(ctx, reviewer.OrganizationID, run)
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	if len(month.invoices) == 0 {
		return parentbilling.CareRunDetail{}, parentbilling.ErrBillingRunEmpty
	}
	if len(month.invoices) != release.DocumentCount || month.total() != release.TotalCents {
		return parentbilling.CareRunDetail{}, parentbilling.ErrBillingRunChanged
	}
	if err := c.issue(ctx, reviewer.OrganizationID, run, rule, pattern, month.invoices); err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	released, err := c.store.MarkCareRunReleased(ctx, reviewer.OrganizationID, run.ID, *month.invoices[0].IssuedOn,
		len(month.invoices), month.total(), rule, heldContracts(month.duplicates))
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	if err := c.store.InsertCareRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventRelease,
		map[string]any{"document_count": len(month.invoices), "total_cents": month.total(), "month_rule": rule,
			"duplicate_children": duplicateChildren(month.duplicates)}); err != nil {
		return parentbilling.CareRunDetail{}, fmt.Errorf("parent billing: record care release: %w", err)
	}
	return c.detail(ctx, reviewer, released)
}

// issue numbers the invoices from the counter of the issue year and stores
// them. A number another invoice of the Träger already carries is
// ErrInvoiceNumberTaken.
func (c *CareBilling) issue(ctx context.Context, organizationID int64, run parentbilling.CareRun, rule, pattern string, invoices []parentbilling.CareInvoice) error {
	issuedOn := c.today()
	year := numberYear(pattern, issuedOn.Year())
	first, err := c.store.ReserveNumbers(ctx, organizationID, parentbilling.ServiceCare, year, len(invoices))
	if err != nil {
		return fmt.Errorf("parent billing: reserve invoice numbers: %w", err)
	}
	for i := range invoices {
		invoices[i].Number = formatInvoiceNumber(pattern, issuedOn.Year(), first+int64(i))
		invoices[i].IssuedOn = &issuedOn
	}
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
	versions, err := c.contracts.VersionsInForce(ctx, organizationID, run.BillingMonth, monthEnd(run.BillingMonth))
	if err != nil {
		return "", careMonth{}, fmt.Errorf("parent billing: read the contract versions: %w", err)
	}
	tariffs, err := c.store.CareTariffs(ctx, organizationID)
	if err != nil {
		return "", careMonth{}, err
	}
	invoiced, err := c.store.InvoicedContracts(ctx, organizationID, run.BillingMonth)
	if err != nil {
		return "", careMonth{}, err
	}
	recipients, err := c.contracts.Recipients(ctx, organizationID, versionContracts(versions))
	if err != nil {
		return "", careMonth{}, fmt.Errorf("parent billing: read the contract parties: %w", err)
	}
	month, err := buildCareMonth(careMonthInput{month: run.BillingMonth, rule: rule, versions: versions, tariffs: tariffs,
		invoiced: invoiced, recipients: recipients})
	return rule, month, err
}

// detail is the run with its preview or its issued invoices and the number
// range the release uses.
func (c *CareBilling) detail(ctx context.Context, reviewer parentbilling.HolidayReviewer, run parentbilling.CareRun) (parentbilling.CareRunDetail, error) {
	detail := parentbilling.CareRunDetail{CareRun: run, Unpriced: []parentbilling.CareUnpriced{}, Duplicates: []parentbilling.CareDuplicate{}}
	if run.Status == parentbilling.RunReleased {
		invoices, err := c.store.CareRunInvoices(ctx, reviewer.OrganizationID, run.ID)
		if err != nil {
			return parentbilling.CareRunDetail{}, err
		}
		detail.Invoices = invoices
		// A released run shows what its release stored (E5): its invoices
		// and the contracts it held back, by child. Nothing is computed
		// again, and a contract that came later is not among them.
		if len(run.HeldContractIDs) > 0 {
			versions, err := c.contracts.VersionsInForce(ctx, reviewer.OrganizationID, run.BillingMonth, monthEnd(run.BillingMonth))
			if err != nil {
				return parentbilling.CareRunDetail{}, fmt.Errorf("parent billing: read the held contracts: %w", err)
			}
			detail.Duplicates = heldView(run.HeldContractIDs, versions)
		}
	} else {
		rule, month, err := c.preview(ctx, reviewer.OrganizationID, run)
		if err != nil {
			return parentbilling.CareRunDetail{}, err
		}
		detail.MonthRule, detail.Invoices, detail.Unpriced, detail.Billed = rule, month.invoices, month.unpriced, month.billed
		detail.Duplicates = month.duplicates
	}
	detail.DuplicateChildren = duplicateChildren(detail.Duplicates)
	for _, invoice := range detail.Invoices {
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

// heldContracts are the contracts of the duplicates, sorted.
func heldContracts(duplicates []parentbilling.CareDuplicate) []int64 {
	ids := []int64{}
	for _, duplicate := range duplicates {
		for _, contract := range duplicate.Contracts {
			if !slices.Contains(ids, contract.ContractID) {
				ids = append(ids, contract.ContractID)
			}
		}
	}
	slices.Sort(ids)
	return ids
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
