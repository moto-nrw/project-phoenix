package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Actions of the trail of a run.
const (
	runEventCreate  = "create"
	runEventRead    = "read"
	runEventCorrect = "correct"
	runEventRelease = "release"
)

// RunStore persists the billing runs, the number ranges and the invoices of
// a Träger in its Träger transaction.
type RunStore interface {
	ListBillingRuns(ctx context.Context, organizationID int64) ([]parentbilling.BillingRun, error)
	// InsertBillingRun reports a second calculated run of the period as
	// ErrBillingRunOpen.
	InsertBillingRun(ctx context.Context, organizationID int64, run parentbilling.BillingRun) (parentbilling.BillingRun, error)
	// BillingRun reads a run, LockBillingRun reads it for update; both
	// report a missing run as ErrBillingRunNotFound.
	BillingRun(ctx context.Context, organizationID, runID int64) (parentbilling.BillingRun, error)
	LockBillingRun(ctx context.Context, organizationID, runID int64) (parentbilling.BillingRun, error)
	// PeriodContributions reads the frozen contributions of the period by
	// registration.
	PeriodContributions(ctx context.Context, organizationID, periodID int64) (map[int64]parentbilling.HolidayContribution, error)
	// IssuedRegistrations names the registrations of the period that an
	// invoice of the service already bills.
	IssuedRegistrations(ctx context.Context, organizationID int64, service string, periodID int64) (map[int64]bool, error)
	RunCorrections(ctx context.Context, organizationID, runID int64) ([]parentbilling.BillingCorrection, error)
	InsertRunCorrection(ctx context.Context, organizationID int64, run parentbilling.BillingRun, registrationID, accountID int64, correction parentbilling.BillingCorrection) error
	InsertRunEvent(ctx context.Context, organizationID, runID, accountID int64, action string, details map[string]any) error
	ListNumberRanges(ctx context.Context, organizationID int64) ([]parentbilling.NumberRange, error)
	SetNumberRange(ctx context.Context, organizationID int64, service, pattern string) (parentbilling.NumberRange, error)
	// ReserveNumbers raises the counter of the service and year by count
	// and returns the first of the reserved numbers. It holds the counter
	// until the transaction ends, so a parallel release waits.
	ReserveNumbers(ctx context.Context, organizationID int64, service string, year, count int) (int64, error)
	// IssueDocuments stores the numbered invoices with their positions; the
	// i-th invoice has the sequence first+i of the counter of year. A
	// number another invoice carries is ErrInvoiceNumberTaken.
	IssueDocuments(ctx context.Context, organizationID int64, run parentbilling.BillingRun, year int, first int64, documents []parentbilling.BillingDocument) error
	MarkRunReleased(ctx context.Context, organizationID, runID int64, issuedOn calendar.Date, count int, totalCents int64) (parentbilling.BillingRun, error)
	RunDocuments(ctx context.Context, organizationID, runID int64) ([]parentbilling.BillingDocument, error)
}

// BillingRuns serves parentbilling.BillingRuns. The period and its confirmed
// registrations come from Enrollment through the root-bound port; the
// frozen contributions, the runs and the invoices are the owner's own rows.
type BillingRuns struct {
	store         RunStore
	registrations parentbilling.HolidayBillingRegistrations
	today         func() calendar.Date
}

var _ parentbilling.BillingRuns = (*BillingRuns)(nil)

// NewBillingRuns builds the billing runs. today is the issue date of a
// release.
func NewBillingRuns(store RunStore, registrations parentbilling.HolidayBillingRegistrations, today func() calendar.Date) *BillingRuns {
	if store == nil || registrations == nil || today == nil {
		panic("parent billing: the store, the holiday registrations and the clock are required")
	}
	return &BillingRuns{store: store, registrations: registrations, today: today}
}

func validReviewer(reviewer parentbilling.HolidayReviewer) error {
	if reviewer.OrganizationID <= 0 || reviewer.AccountID <= 0 {
		return errTraegerRequired
	}
	return nil
}

// ListBillingRuns lists the runs of the Träger, newest first. The list
// carries no personal data.
func (b *BillingRuns) ListBillingRuns(ctx context.Context, organizationID int64) ([]parentbilling.BillingRun, error) {
	if organizationID <= 0 {
		return nil, errTraegerRequired
	}
	runs, err := b.store.ListBillingRuns(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("parent billing: list billing runs: %w", err)
	}
	return runs, nil
}

// StartHolidayBillingRun starts a calculated run for a period of the
// Träger. A period has at most one calculated run.
func (b *BillingRuns) StartHolidayBillingRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64) (parentbilling.BillingRun, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.BillingRun{}, err
	}
	if periodID <= 0 {
		return parentbilling.BillingRun{}, parentbilling.ErrHolidayPeriodNotFound
	}
	period, err := b.registrations.HolidayBillingPeriod(ctx, reviewer, periodID)
	if err != nil {
		return parentbilling.BillingRun{}, err
	}
	if period.StartsOn.IsZero() || period.EndsOn.IsZero() {
		return parentbilling.BillingRun{}, parentbilling.ErrBillingRunEmpty
	}
	run, err := b.store.InsertBillingRun(ctx, reviewer.OrganizationID, parentbilling.BillingRun{
		Service: parentbilling.ServiceHoliday, HolidayPeriodID: period.ID, PeriodName: period.Name,
		ServiceFrom: period.StartsOn, ServiceUntil: period.EndsOn, Status: parentbilling.RunCalculated,
	})
	if err != nil {
		return parentbilling.BillingRun{}, err
	}
	if err := b.store.InsertRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventCreate, nil); err != nil {
		return parentbilling.BillingRun{}, fmt.Errorf("parent billing: record billing run: %w", err)
	}
	return run, nil
}

// BillingRun shows a run with its invoices: the preview of today while it
// is calculated, the issued invoices once released. It records the read.
func (b *BillingRuns) BillingRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64) (parentbilling.BillingRunDetail, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	run, err := b.store.BillingRun(ctx, reviewer.OrganizationID, runID)
	if err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	detail, err := b.detail(ctx, reviewer, run)
	if err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	if err := b.store.InsertRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventRead, nil); err != nil {
		return parentbilling.BillingRunDetail{}, fmt.Errorf("parent billing: record billing run read: %w", err)
	}
	return detail, nil
}

// CorrectBillingRun changes the amount of one child in a calculated run to
// the given total, with a reason. The correction is a position of its own;
// the earlier ones stay as its history.
func (b *BillingRuns) CorrectBillingRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, input parentbilling.BillingCorrectionInput) (parentbilling.BillingRunDetail, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	input, err := normalizeCorrection(input)
	if err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	run, err := b.lockCalculated(ctx, reviewer.OrganizationID, runID)
	if err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	current, err := b.preview(ctx, reviewer, run)
	if err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	child, ok := current.child(input.RegistrationID, input.ChildID)
	if !ok {
		return parentbilling.BillingRunDetail{}, parentbilling.ErrBillingFamilyNotFound
	}
	if input.TotalCents == child.TotalCents {
		return parentbilling.BillingRunDetail{}, validation.Errors{"total_cents": errors.New("is the current amount")}
	}
	correction := parentbilling.BillingCorrection{
		ChildID: child.ChildID, AmountCents: input.TotalCents - child.TotalCents, TotalBeforeCents: child.TotalCents,
		TotalAfterCents: input.TotalCents, Reason: input.Reason,
	}
	if err := b.store.InsertRunCorrection(ctx, reviewer.OrganizationID, run, input.RegistrationID, reviewer.AccountID, correction); err != nil {
		return parentbilling.BillingRunDetail{}, fmt.Errorf("parent billing: store correction: %w", err)
	}
	if err := b.store.InsertRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventCorrect,
		map[string]any{"registration_id": input.RegistrationID, "child_id": input.ChildID}); err != nil {
		return parentbilling.BillingRunDetail{}, fmt.Errorf("parent billing: record correction: %w", err)
	}
	return b.detail(ctx, reviewer, run)
}

// ReleaseBillingRun issues the invoices of the preview, numbered from the
// number range of the service without a gap, and closes the run. It refuses
// when the preview no longer matches what the Abrechnung checked. The
// caller rolls the transaction back when this fails.
func (b *BillingRuns) ReleaseBillingRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, release parentbilling.BillingRelease) (parentbilling.BillingRunDetail, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	run, err := b.lockCalculated(ctx, reviewer.OrganizationID, runID)
	if err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	pattern, err := b.numberPattern(ctx, reviewer.OrganizationID, run.Service)
	if err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	current, err := b.preview(ctx, reviewer, run)
	if err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	if len(current.documents) == 0 {
		return parentbilling.BillingRunDetail{}, parentbilling.ErrBillingRunEmpty
	}
	if len(current.documents) != release.DocumentCount || current.total() != release.TotalCents {
		return parentbilling.BillingRunDetail{}, parentbilling.ErrBillingRunChanged
	}
	issuedOn := b.today()
	year := numberYear(pattern, issuedOn.Year())
	first, err := b.store.ReserveNumbers(ctx, reviewer.OrganizationID, run.Service, year, len(current.documents))
	if err != nil {
		return parentbilling.BillingRunDetail{}, fmt.Errorf("parent billing: reserve invoice numbers: %w", err)
	}
	documents := current.documents
	for i := range documents {
		documents[i].Number = formatInvoiceNumber(pattern, issuedOn.Year(), first+int64(i))
		documents[i].IssuedOn = &issuedOn
	}
	if err := b.store.IssueDocuments(ctx, reviewer.OrganizationID, run, year, first, documents); err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	released, err := b.store.MarkRunReleased(ctx, reviewer.OrganizationID, run.ID, issuedOn, len(documents), current.total())
	if err != nil {
		return parentbilling.BillingRunDetail{}, fmt.Errorf("parent billing: release billing run: %w", err)
	}
	if err := b.store.InsertRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventRelease,
		map[string]any{"document_count": len(documents), "total_cents": current.total()}); err != nil {
		return parentbilling.BillingRunDetail{}, fmt.Errorf("parent billing: record release: %w", err)
	}
	return b.detail(ctx, reviewer, released)
}

// ListNumberRanges lists the number range of every service, with the
// number the next invoice would get today. A service without a pattern
// shows an empty one.
func (b *BillingRuns) ListNumberRanges(ctx context.Context, organizationID int64) ([]parentbilling.NumberRange, error) {
	if organizationID <= 0 {
		return nil, errTraegerRequired
	}
	ranges, err := b.store.ListNumberRanges(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("parent billing: list number ranges: %w", err)
	}
	for _, numberRange := range ranges {
		if numberRange.Service == parentbilling.ServiceHoliday {
			return b.withExamples(ranges), nil
		}
	}
	return b.withExamples(append(ranges, parentbilling.NumberRange{Service: parentbilling.ServiceHoliday})), nil
}

// SetNumberRange sets the pattern of a service. Issued invoices keep their
// numbers; the counter goes on.
func (b *BillingRuns) SetNumberRange(ctx context.Context, organizationID int64, service, pattern string) (parentbilling.NumberRange, error) {
	if organizationID <= 0 {
		return parentbilling.NumberRange{}, errTraegerRequired
	}
	if strings.TrimSpace(service) != parentbilling.ServiceHoliday {
		return parentbilling.NumberRange{}, validation.Errors{"service": errors.New("unknown service")}
	}
	pattern, err := normalizeNumberPattern(pattern)
	if err != nil {
		return parentbilling.NumberRange{}, err
	}
	numberRange, err := b.store.SetNumberRange(ctx, organizationID, parentbilling.ServiceHoliday, pattern)
	if err != nil {
		return parentbilling.NumberRange{}, fmt.Errorf("parent billing: set number range: %w", err)
	}
	return b.withExamples([]parentbilling.NumberRange{numberRange})[0], nil
}

// withExamples names the first number of this year for every pattern. It
// shows the form of a number, not the next number of the counter.
func (b *BillingRuns) withExamples(ranges []parentbilling.NumberRange) []parentbilling.NumberRange {
	year := b.today().Year()
	for i := range ranges {
		if ranges[i].Pattern != "" {
			ranges[i].Example = formatInvoiceNumber(ranges[i].Pattern, year, 1)
		}
	}
	return ranges
}

// lockCalculated holds a calculated run until the transaction ends, so
// corrections and the release of one run run one after another.
func (b *BillingRuns) lockCalculated(ctx context.Context, organizationID, runID int64) (parentbilling.BillingRun, error) {
	if runID <= 0 {
		return parentbilling.BillingRun{}, parentbilling.ErrBillingRunNotFound
	}
	run, err := b.store.LockBillingRun(ctx, organizationID, runID)
	if err != nil {
		return parentbilling.BillingRun{}, err
	}
	if run.Status != parentbilling.RunCalculated {
		return parentbilling.BillingRun{}, parentbilling.ErrBillingRunReleased
	}
	return run, nil
}

func (b *BillingRuns) numberPattern(ctx context.Context, organizationID int64, service string) (string, error) {
	ranges, err := b.store.ListNumberRanges(ctx, organizationID)
	if err != nil {
		return "", fmt.Errorf("parent billing: read number range: %w", err)
	}
	for _, numberRange := range ranges {
		if numberRange.Service == service && numberRange.Pattern != "" {
			return numberRange.Pattern, nil
		}
	}
	return "", parentbilling.ErrNumberRangeMissing
}

// preview reads the confirmed registrations through Enrollment, which
// records the read, and builds the draft invoices of the run.
func (b *BillingRuns) preview(ctx context.Context, reviewer parentbilling.HolidayReviewer, run parentbilling.BillingRun) (preview, error) {
	registrations, err := b.registrations.ConfirmedHolidayRegistrations(ctx, reviewer, run.HolidayPeriodID)
	if err != nil {
		return preview{}, err
	}
	contributions, err := b.store.PeriodContributions(ctx, reviewer.OrganizationID, run.HolidayPeriodID)
	if err != nil {
		return preview{}, fmt.Errorf("parent billing: read holiday contributions: %w", err)
	}
	issued, err := b.store.IssuedRegistrations(ctx, reviewer.OrganizationID, run.Service, run.HolidayPeriodID)
	if err != nil {
		return preview{}, fmt.Errorf("parent billing: read issued invoices: %w", err)
	}
	corrections, err := b.store.RunCorrections(ctx, reviewer.OrganizationID, run.ID)
	if err != nil {
		return preview{}, fmt.Errorf("parent billing: read corrections: %w", err)
	}
	return buildPreview(previewInput{
		run: run, registrations: registrations, contributions: contributions, issued: issued, corrections: corrections,
	}), nil
}

// detail is the run with the preview or its issued invoices and the
// number range the release uses.
func (b *BillingRuns) detail(ctx context.Context, reviewer parentbilling.HolidayReviewer, run parentbilling.BillingRun) (parentbilling.BillingRunDetail, error) {
	detail := parentbilling.BillingRunDetail{BillingRun: run, BilledInAdvance: run.Service == parentbilling.ServiceHoliday}
	if run.Status == parentbilling.RunReleased {
		documents, err := b.store.RunDocuments(ctx, reviewer.OrganizationID, run.ID)
		if err != nil {
			return parentbilling.BillingRunDetail{}, fmt.Errorf("parent billing: read invoices: %w", err)
		}
		corrections, err := b.store.RunCorrections(ctx, reviewer.OrganizationID, run.ID)
		if err != nil {
			return parentbilling.BillingRunDetail{}, fmt.Errorf("parent billing: read corrections: %w", err)
		}
		detail.Documents = withHistory(documents, corrections)
	} else {
		current, err := b.preview(ctx, reviewer, run)
		if err != nil {
			return parentbilling.BillingRunDetail{}, err
		}
		detail.Documents, detail.Unbilled = current.documents, current.unbilled
	}
	for _, document := range detail.Documents {
		detail.PreviewCount++
		detail.PreviewCents += document.TotalCents
	}
	pattern, err := b.numberPattern(ctx, reviewer.OrganizationID, run.Service)
	switch {
	case errors.Is(err, parentbilling.ErrNumberRangeMissing):
	case err != nil:
		return parentbilling.BillingRunDetail{}, err
	default:
		detail.NumberPattern, detail.NumberRangeSet = pattern, true
	}
	return detail, nil
}

// withHistory adds the corrections of the run to the children of the issued
// invoices. A child whose registration is gone keeps its correction
// positions on the invoice.
func withHistory(documents []parentbilling.BillingDocument, corrections []parentbilling.BillingCorrection) []parentbilling.BillingDocument {
	byChild := map[int64][]parentbilling.BillingCorrection{}
	for _, correction := range corrections {
		byChild[correction.ChildID] = append(byChild[correction.ChildID], correction)
	}
	for i := range documents {
		for j := range documents[i].Children {
			child := &documents[i].Children[j]
			child.Corrections = byChild[child.ChildID]
			if child.Corrections == nil {
				child.Corrections = []parentbilling.BillingCorrection{}
			}
		}
	}
	return documents
}

// normalizeCorrection checks a correction: a child, an amount that is not
// negative and a reason.
func normalizeCorrection(input parentbilling.BillingCorrectionInput) (parentbilling.BillingCorrectionInput, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	errs := validation.Errors{}
	if input.RegistrationID <= 0 {
		errs["registration_id"] = errors.New("is required")
	}
	if input.ChildID <= 0 {
		errs["child_id"] = errors.New("is required")
	}
	if input.TotalCents < 0 || input.TotalCents > parentbilling.MaxDiscountCents {
		errs["total_cents"] = errors.New("must be an amount in cent, not negative")
	}
	switch {
	case input.Reason == "":
		errs["reason"] = errors.New("is required")
	case len([]rune(input.Reason)) > parentbilling.MaxCorrectionReasonLength:
		errs["reason"] = errors.New("is too long")
	}
	if len(errs) > 0 {
		return input, errs
	}
	return input, nil
}
