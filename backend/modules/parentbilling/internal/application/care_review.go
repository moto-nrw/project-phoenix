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

// The review of a monthly run before its release (#5916, ADR 0119
// addendum 2026-11-08). The preview is computed for the whole month of the
// Träger, so siblings and the rule chain see every child, and then narrowed
// to the schools of the run; an excluded invoice stays visible but is not
// issued. Nothing of the preview is stored.

// Trail actions of the review.
const (
	runEventExclude     = "exclude"
	runEventInclude     = "include"
	runEventInvoiceDate = "invoice_date"
)

// CareReviewStore persists the review of a monthly run.
type CareReviewStore interface {
	// OpenCareRunSchools names per school the open run of the month.
	OpenCareRunSchools(ctx context.Context, organizationID int64, month calendar.Date) (map[int64]int64, error)
	SetCareRunInvoiceDate(ctx context.Context, organizationID, runID int64, invoiceDate *calendar.Date) error
	// CareRunExclusions reads per invoice (contract and service) the
	// exclusion in force (#6346).
	CareRunExclusions(ctx context.Context, organizationID, runID int64) (map[parentbilling.CareInvoiceKey]parentbilling.CareExclusion, error)
}

// reviewedMonth is the preview of a calculated run: the invoices the
// release issues, those it leaves out, the kind of document they would be
// issued as (#4688) and the state the reviewer checks.
type reviewedMonth struct {
	careMonth
	excluded    []parentbilling.CareInvoice
	kind        parentbilling.DocumentKindState
	legalRemedy string
	fingerprint string
	// conflicts counts the changes in conflict on the invoices the release
	// issues (#6197).
	conflicts int
	// free counts the run's contracts a free month bills nothing for.
	free int
}

// reviewed computes the preview of a calculated run with the rule of the
// Träger now, narrowed to its schools and without its excluded invoices,
// with the changes of its invoices (#6197).
func (c *CareBilling) reviewed(ctx context.Context, organizationID int64, run parentbilling.CareRun) (string, reviewedMonth, error) {
	rule, month, err := c.preview(ctx, organizationID, run)
	if err != nil {
		return "", reviewedMonth{}, err
	}
	exclusions, err := c.store.CareRunExclusions(ctx, organizationID, run.ID)
	if err != nil {
		return "", reviewedMonth{}, err
	}
	review := narrowMonth(month, run.SchoolIDs, exclusions)
	if err := c.amendReviewed(ctx, organizationID, run, &review); err != nil {
		return "", reviewedMonth{}, err
	}
	// Every draft carries the kind of document the release would freeze,
	// so the fingerprint changes when the Abrechnung chooses another kind
	// after the review (#4688).
	if review.kind, review.legalRemedy, err = c.kinds.reviewed(ctx, organizationID, parentbilling.ServiceCare); err != nil {
		return "", reviewedMonth{}, err
	}
	markDocumentKind(review.kind.DocumentKind, review.invoices, review.excluded)
	review.fingerprint = previewFingerprint(run, rule, review)
	return rule, review, nil
}

// narrowMonth keeps the part of the month the schools bill and moves the
// excluded invoices aside: each by its own key, so excluding the Mensa
// invoice of its own of a contract keeps its care invoice and the other way
// round (#6346).
func narrowMonth(month careMonth, schoolIDs []int64, exclusions map[parentbilling.CareInvoiceKey]parentbilling.CareExclusion) reviewedMonth {
	inRun := func(schoolID int64) bool { return slices.Contains(schoolIDs, schoolID) }
	review := reviewedMonth{careMonth: careMonth{invoices: []parentbilling.CareInvoice{}, unpriced: []parentbilling.CareUnpriced{},
		duplicates: []parentbilling.CareDuplicate{}, mealGaps: []parentbilling.CareMealGap{},
		mealUnbilled: []parentbilling.CareMealUnbilled{}, mealChanged: []parentbilling.CareMealChanged{}, dueDates: month.dueDates, deviations: month.deviations,
		mandates: month.mandates},
		excluded: []parentbilling.CareInvoice{}}
	review.uncounted = uncountedInRun(month.uncountedSchools, inRun)
	review.billed = countInRun(month.billedBySchool, inRun)
	review.free = countInRun(month.freeBySchool, inRun)
	narrowFeeNotices(&review, month, inRun)
	for _, invoice := range month.invoices {
		if !inRun(invoice.SchoolID) {
			continue
		}
		if exclusion, ok := exclusions[careInvoiceKeyOf(invoice)]; ok {
			invoice.Exclusion = &exclusion
			review.excluded = append(review.excluded, invoice)
			continue
		}
		review.invoices = append(review.invoices, invoice)
	}
	for _, unpriced := range month.unpriced {
		if inRun(unpriced.SchoolID) {
			review.unpriced = append(review.unpriced, unpriced)
		}
	}
	narrowMeals(&review, month, inRun)
	for _, duplicate := range month.duplicates {
		if slices.ContainsFunc(duplicate.Contracts, func(contract parentbilling.CareDuplicateContract) bool {
			return inRun(contract.SchoolID)
		}) {
			review.duplicates = append(review.duplicates, duplicate)
		}
	}
	return review
}

// narrowFeeNotices keeps the Jahresbescheide of the run's schools that
// assess the month (#6219) and the contracts they took back (ADR 0182).
func narrowFeeNotices(review *reviewedMonth, month careMonth, inRun func(int64) bool) {
	review.feeNotices = []string{}
	for schoolID, numbers := range month.feeNoticesBySchool {
		if inRun(schoolID) {
			review.feeNotices = append(review.feeNotices, numbers...)
		}
	}
	slices.Sort(review.feeNotices)
	review.feeNoticeHeld = []parentbilling.CareFeeNoticeHold{}
	for _, hold := range month.feeNoticeHeld {
		if inRun(hold.SchoolID) {
			review.feeNoticeHeld = append(review.feeNoticeHeld, hold)
		}
	}
}

// countInRun adds the counts of the schools the run bills.
func countInRun(bySchool map[int64]int, inRun func(int64) bool) int {
	total := 0
	for schoolID, count := range bySchool {
		if inRun(schoolID) {
			total += count
		}
	}
	return total
}

// narrowMeals keeps the Mensa months of the run's schools (#4686): the
// months without a release, those no longer billable, and the children the
// run does not bill. The Mensa positions travel on their invoices.
func narrowMeals(review *reviewedMonth, month careMonth, inRun func(int64) bool) {
	for _, gap := range month.mealGaps {
		if inRun(gap.SchoolID) {
			review.mealGaps = append(review.mealGaps, gap)
		}
	}
	for _, unbilled := range month.mealUnbilled {
		if inRun(unbilled.SchoolID) {
			review.mealUnbilled = append(review.mealUnbilled, unbilled)
		}
	}
	for _, changed := range month.mealChanged {
		if inRun(changed.SchoolID) {
			review.mealChanged = append(review.mealChanged, changed)
		}
	}
}

// uncountedInRun counts the families without a confirmed number whose
// invoices go to a school of the run (#5918).
func uncountedInRun(families map[string][]int64, inRun func(int64) bool) int {
	count := 0
	for _, schools := range families {
		if slices.ContainsFunc(schools, inRun) {
			count++
		}
	}
	return count
}

// fingerprintInvoice is what the fingerprint covers of an invoice the
// review shows: the invoice as shown with its positions and reductions, the
// child record and the payable cents the rule chain allocated.
type fingerprintInvoice struct {
	Invoice   parentbilling.CareInvoice `json:"invoice"`
	StudentID *int64                    `json:"student_id"`
	Payable   []int64                   `json:"payable"`
}

func fingerprintInvoices(invoices []parentbilling.CareInvoice) []fingerprintInvoice {
	result := make([]fingerprintInvoice, len(invoices))
	for i, invoice := range invoices {
		payable := make([]int64, len(invoice.Positions))
		for j, position := range invoice.Positions {
			payable[j] = position.PayableCents
		}
		result[i] = fingerprintInvoice{Invoice: invoice, StudentID: invoice.StudentID, Payable: payable}
	}
	return result
}

// previewFingerprint names the reviewed state of a run: everything the
// review shows. That is the schools, the invoice date and the month rule;
// every invoice the release issues and every excluded one, each with its
// recipient, positions and reductions (so a new evidence or rule changes
// it); the bookings without a price; the children held back for a
// duplicate contract; the count of contracts already billed and of those a
// free month leaves out (#6196); the due
// dates of the run per payment method, which every invoice's payment also
// carries (#4689); and the Mensa (#4686): its positions on the invoices,
// the months without a release, the months no longer billable and the
// children left out; and the Jahresbescheide that bill a month of the
// run's contracts (#6219). A change of any of them since the review makes the
// release refuse (#5916).
func previewFingerprint(run parentbilling.CareRun, rule string, review reviewedMonth) string {
	return contentFingerprint(struct {
		Run         int64                            `json:"run"`
		Schools     []int64                          `json:"schools"`
		InvoiceDate *calendar.Date                   `json:"invoice_date"`
		Rule        string                           `json:"rule"`
		Invoices    []fingerprintInvoice             `json:"invoices"`
		Excluded    []fingerprintInvoice             `json:"excluded"`
		Unpriced    []parentbilling.CareUnpriced     `json:"unpriced"`
		Duplicates  []parentbilling.CareDuplicate    `json:"duplicates"`
		Billed      int                              `json:"billed"`
		Free        int                              `json:"free"`
		DueDates    []parentbilling.RunDueDate       `json:"due_dates"`
		Uncounted   int                              `json:"uncounted_families"`
		MealGaps    []parentbilling.CareMealGap      `json:"meal_gaps"`
		MealUnbill  []parentbilling.CareMealUnbilled `json:"meal_unbilled"`
		MealChanged []parentbilling.CareMealChanged  `json:"meal_changed"`
		LegalRemedy string                           `json:"legal_remedy"`
		FeeNotices  []string                         `json:"fee_notices"`
	}{run.ID, run.SchoolIDs, run.InvoiceDate, rule, fingerprintInvoices(review.invoices),
		fingerprintInvoices(review.excluded), review.unpriced, review.duplicates, review.billed, review.free,
		review.dueDates.InPaymentOrder(), review.uncounted, review.mealGaps, review.mealUnbilled, review.mealChanged,
		review.legalRemedy, review.feeNotices})
}

// runSchools resolves the schools a new run bills: the given ones, each a
// school of the Träger, or every school of the Träger without an open run
// of the month when none is given.
func (c *CareBilling) runSchools(ctx context.Context, organizationID int64, month calendar.Date, requested []int64) ([]int64, error) {
	schools, err := c.schools.TraegerSchools(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the schools: %w", err)
	}
	own := make([]int64, len(schools))
	for i, school := range schools {
		own[i] = school.ID
	}
	if len(requested) == 0 {
		open, err := c.store.OpenCareRunSchools(ctx, organizationID, month)
		if err != nil {
			return nil, err
		}
		free := slices.DeleteFunc(slices.Clone(own), func(id int64) bool { _, taken := open[id]; return taken })
		if len(free) == 0 && len(own) > 0 {
			return nil, parentbilling.ErrCareRunOpen
		}
		requested = free
	}
	result := []int64{}
	for _, id := range requested {
		if !slices.Contains(own, id) {
			return nil, validation.Errors{"school_ids": errors.New("names a school that is not a school of the Träger")}
		}
		if !slices.Contains(result, id) {
			result = append(result, id)
		}
	}
	if len(result) == 0 {
		return nil, validation.Errors{"school_ids": errors.New("needs at least one school")}
	}
	slices.Sort(result)
	return result, nil
}

// CareRunSchools lists the schools of the Träger for a new run of a month,
// each with the open run of the month that bills it.
func (c *CareBilling) CareRunSchools(ctx context.Context, organizationID int64, month calendar.Date) ([]parentbilling.CareRunSchool, error) {
	if organizationID <= 0 {
		return nil, errTraegerRequired
	}
	if !isMonthStart(month) {
		return nil, validation.Errors{"billing_month": errors.New("must be the first day of a month")}
	}
	schools, err := c.schools.TraegerSchools(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the schools: %w", err)
	}
	open, err := c.store.OpenCareRunSchools(ctx, organizationID, month)
	if err != nil {
		return nil, err
	}
	result := make([]parentbilling.CareRunSchool, len(schools))
	for i, school := range schools {
		result[i] = parentbilling.CareRunSchool{ID: school.ID, Name: school.Name}
		if runID, taken := open[school.ID]; taken {
			result[i].OpenRunID = &runID
		}
	}
	return result, nil
}

// checkInvoiceDate accepts an invoice date from the first day of the month
// before the billed month to the last day of the month after it: a run in
// advance, within the month (SJR: October on 30.10.) or right after it.
func checkInvoiceDate(month calendar.Date, invoiceDate *calendar.Date) error {
	if invoiceDate == nil {
		return nil
	}
	earliest := firstOfMonth(month.AddDays(-1))
	latest := monthEnd(monthEnd(month).AddDays(1))
	if invoiceDate.IsZero() || invoiceDate.Before(earliest) || invoiceDate.After(latest) {
		return validation.Errors{"invoice_date": fmt.Errorf("must lie between %s and %s", earliest, latest)}
	}
	return nil
}

// openRun locks a run of the Träger for a change of its review; a released
// run is ErrBillingRunReleased. The lock orders it with the release.
func (c *CareBilling) openRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64) (parentbilling.CareRun, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareRun{}, err
	}
	if runID <= 0 {
		return parentbilling.CareRun{}, parentbilling.ErrCareRunNotFound
	}
	run, err := c.store.CareRun(ctx, reviewer.OrganizationID, runID, true)
	if err != nil {
		return parentbilling.CareRun{}, err
	}
	if run.Status != parentbilling.RunCalculated {
		return parentbilling.CareRun{}, parentbilling.ErrBillingRunReleased
	}
	return run, nil
}

// SetCareRunInvoiceDate sets the invoice date of a calculated run; nil
// takes the day of the release.
func (c *CareBilling) SetCareRunInvoiceDate(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, invoiceDate *calendar.Date) (parentbilling.CareRunDetail, error) {
	run, err := c.openRun(ctx, reviewer, runID)
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	if err := checkInvoiceDate(run.BillingMonth, invoiceDate); err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	if err := c.store.SetCareRunInvoiceDate(ctx, reviewer.OrganizationID, run.ID, invoiceDate); err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	var value any
	if invoiceDate != nil {
		value = invoiceDate.String()
	}
	if err := c.store.InsertCareRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventInvoiceDate,
		map[string]any{"invoice_date": value}); err != nil {
		return parentbilling.CareRunDetail{}, fmt.Errorf("parent billing: record the invoice date: %w", err)
	}
	return c.CareRun(ctx, reviewer, run.ID)
}

// checkExclusionService accepts the service of an invoice of a run; an
// exclusion names exactly one (#6346).
func checkExclusionService(service string) error {
	if !slices.Contains(parentbilling.ExcludableServices, service) {
		return validation.Errors{"service": errors.New("must be care or meal")}
	}
	return nil
}

// exclusionEvent names the one invoice of a trail entry: its contract and
// its service.
func exclusionEvent(key parentbilling.CareInvoiceKey) map[string]any {
	return map[string]any{"contract_ids": []int64{key.ContractID}, "services": []string{key.Service}}
}

// ExcludeCareInvoice leaves one invoice of the preview out of the release of
// the run, with a reason: the invoice of the contract for the service the
// key names, and no other (#6346). It comes back in the next run of the
// month.
func (c *CareBilling) ExcludeCareInvoice(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, key parentbilling.CareInvoiceKey, reason string) (parentbilling.CareRunDetail, error) {
	reason = strings.TrimSpace(reason)
	if err := checkExclusionService(key.Service); err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	switch {
	case reason == "":
		return parentbilling.CareRunDetail{}, validation.Errors{"reason": errors.New("is required")}
	case len([]rune(reason)) > parentbilling.MaxExclusionReasonLength:
		return parentbilling.CareRunDetail{}, validation.Errors{"reason": fmt.Errorf("must be at most %d characters", parentbilling.MaxExclusionReasonLength)}
	}
	run, err := c.openRun(ctx, reviewer, runID)
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	_, review, err := c.reviewed(ctx, reviewer.OrganizationID, run)
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	index := slices.IndexFunc(review.invoices, func(invoice parentbilling.CareInvoice) bool { return careInvoiceKeyOf(invoice) == key })
	if index < 0 {
		return parentbilling.CareRunDetail{}, parentbilling.ErrCareExclusionConflict
	}
	details := exclusionEvent(key)
	details["school_id"], details["reason"] = review.invoices[index].SchoolID, reason
	if err := c.store.InsertCareRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventExclude, details); err != nil {
		return parentbilling.CareRunDetail{}, fmt.Errorf("parent billing: record the exclusion: %w", err)
	}
	return c.CareRun(ctx, reviewer, run.ID)
}

// IncludeCareInvoice takes the one excluded invoice the key names back into
// the release; an exclusion of the contract's other invoice stays (#6346).
func (c *CareBilling) IncludeCareInvoice(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, key parentbilling.CareInvoiceKey) (parentbilling.CareRunDetail, error) {
	if err := checkExclusionService(key.Service); err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	run, err := c.openRun(ctx, reviewer, runID)
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	exclusions, err := c.store.CareRunExclusions(ctx, reviewer.OrganizationID, run.ID)
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	if _, excluded := exclusions[key]; !excluded {
		return parentbilling.CareRunDetail{}, parentbilling.ErrCareExclusionConflict
	}
	if err := c.store.InsertCareRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventInclude,
		exclusionEvent(key)); err != nil {
		return parentbilling.CareRunDetail{}, fmt.Errorf("parent billing: record the return: %w", err)
	}
	return c.CareRun(ctx, reviewer, run.ID)
}

// reviewDetail fills the review of a calculated run into its detail: the
// invoices the release issues, then the excluded ones, and the schools.
func (c *CareBilling) reviewDetail(ctx context.Context, reviewer parentbilling.HolidayReviewer, detail *parentbilling.CareRunDetail) error {
	organizationID := reviewer.OrganizationID
	rule, review, err := c.reviewed(ctx, organizationID, detail.CareRun)
	if err != nil {
		return err
	}
	// The payer details the review shows are read from the family
	// deviations; their trail names the read (#4689).
	if err := recordDeviationReads(ctx, c.store, reviewer, review.deviations); err != nil {
		return err
	}
	// So are the masked accounts of the SEPA mandates (#4695).
	if err := recordMandateReads(ctx, c.store, reviewer, review.mandates); err != nil {
		return err
	}
	detail.DueDates = review.dueDates.InPaymentOrder()
	detail.MonthRule, detail.Unpriced, detail.Billed = rule, review.unpriced, review.billed
	detail.FreeOfCharge = review.free
	detail.Duplicates, detail.MealGaps, detail.MealUnbilled = review.duplicates, review.mealGaps, review.mealUnbilled
	detail.MealChanged = review.mealChanged
	detail.UncountedFamilies = review.uncounted
	detail.FeeNoticeContracts = len(review.feeNotices)
	detail.FeeNoticeHeld = review.feeNoticeHeld
	detail.Invoices = append(review.invoices, review.excluded...)
	detail.Fingerprint = review.fingerprint
	detail.AmendmentConflicts = review.conflicts
	if detail.Amendable, err = c.settings.CareAmendable(ctx, organizationID); err != nil {
		return fmt.Errorf("parent billing: read the changes the Träger allows: %w", err)
	}
	detail.DocumentKindState = review.kind
	for _, invoice := range review.excluded {
		detail.ExcludedCount++
		detail.ExcludedCents += invoice.TotalCents
	}
	schools, err := c.schools.TraegerSchools(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("parent billing: read the schools: %w", err)
	}
	detail.Schools = []parentbilling.CareSchool{}
	for _, school := range schools {
		if slices.Contains(detail.SchoolIDs, school.ID) {
			detail.Schools = append(detail.Schools, school)
		}
	}
	return nil
}

// excludedContracts names the contracts of the excluded invoices, each once.
func excludedContracts(invoices []parentbilling.CareInvoice) []int64 {
	ids := []int64{}
	for _, invoice := range invoices {
		if !slices.Contains(ids, invoice.ContractID) {
			ids = append(ids, invoice.ContractID)
		}
	}
	return ids
}

// excludedInvoices names each excluded invoice by its contract and service,
// so the trail of a release says which invoice of a contract stayed out
// (#6346).
func excludedInvoices(invoices []parentbilling.CareInvoice) []map[string]any {
	keys := make([]map[string]any, len(invoices))
	for i, invoice := range invoices {
		key := careInvoiceKeyOf(invoice)
		keys[i] = map[string]any{"contract_id": key.ContractID, "service": key.Service}
	}
	return keys
}

// issueReviewed checks the review again in the release's transaction (the
// schools, the exclusions, the invoice date and every invoice must be what
// the reviewer saw) and issues the invoices on the invoice date of the run,
// or today without one. A run whose invoices are all excluded issues none
// and takes no number.
func (c *CareBilling) issueReviewed(ctx context.Context, organizationID int64, run parentbilling.CareRun,
	release parentbilling.CareRelease,
) (string, reviewedMonth, calendar.Date, error) {
	rule, month, err := c.reviewed(ctx, organizationID, run)
	if err != nil {
		return "", reviewedMonth{}, "", err
	}
	// A run of a free month bills nothing and still closes, so its schools
	// are free for the next run and the release keeps the free contracts
	// (#6196).
	if len(month.invoices) == 0 && len(month.excluded) == 0 && month.free == 0 {
		return "", reviewedMonth{}, "", parentbilling.ErrBillingRunEmpty
	}
	if month.conflicts > 0 {
		return "", reviewedMonth{}, "", parentbilling.ErrCareAmendmentsUnresolved
	}
	if len(month.invoices) != release.DocumentCount || month.total() != release.TotalCents ||
		month.fingerprint == "" || month.fingerprint != release.Fingerprint {
		return "", reviewedMonth{}, "", parentbilling.ErrBillingRunChanged
	}
	// A Träger that blocks direct debits without a mandate issues none of
	// them (MissingMandateBlock, ADR 0171); an invoice of 0 € collects
	// nothing and needs no mandate.
	if slices.ContainsFunc(month.invoices, func(invoice parentbilling.CareInvoice) bool {
		return debitBlocked(invoice.TotalCents, invoice.Payment)
	}) {
		return "", reviewedMonth{}, "", parentbilling.ErrDirectDebitWithoutMandate
	}
	issuedOn := c.today()
	if run.InvoiceDate != nil {
		issuedOn = *run.InvoiceDate
	}
	if len(month.invoices) == 0 {
		return rule, month, issuedOn, nil
	}
	// No fee notice without its legal remedy (#4688); the drafts already
	// carry the kind the fingerprint covers.
	if month.kind.LegalRemedyMissing {
		return "", reviewedMonth{}, "", parentbilling.ErrLegalRemedyMissing
	}
	pattern, err := numberPatternOf(ctx, c.store, organizationID, parentbilling.ServiceCare)
	if err != nil {
		return "", reviewedMonth{}, "", err
	}
	if err := c.issue(ctx, organizationID, run, rule, pattern, issuedOn, month.invoices); err != nil {
		return "", reviewedMonth{}, "", err
	}
	return rule, month, issuedOn, nil
}

// markDocumentKind puts the kind the release would freeze on every draft. A
// Mensa invoice of its own stays an invoice: the kind belongs to the care
// contribution (#4688); bundled Mensa lines go with their care invoice.
func markDocumentKind(kind string, lists ...[]parentbilling.CareInvoice) {
	for _, drafts := range lists {
		for i := range drafts {
			drafts[i].DocumentKind = kind
			if drafts[i].Service == parentbilling.ServiceMeal {
				drafts[i].DocumentKind = parentbilling.DocumentKindInvoice
			}
		}
	}
}
