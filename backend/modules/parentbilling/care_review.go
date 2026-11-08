package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The review of a monthly run before its release (#5916, ADR 0119 addendum
// 2026-11-08, decision D3/E1 A of the SJR alignment). The Abrechnung picks
// the month and the schools of a run, sets its invoice date, leaves single
// invoices out with a reason and downloads the preview as a review list.
// The preview stays computed on every read; no draft is stored or edited.
// The release checks the preview again in its transaction and refuses a
// state that changed since the review. Issued invoices never change; a
// correction is a Storno and a new invoice (#4690, ADR 0165).
//
// Invariants:
//   - A school is in at most one open run per month, also when two runs
//     start at once (uq_care_run_schools_open). Runs of disjoint schools
//     may be checked side by side.
//   - Every invoice of a release carries the invoice date of its run, the
//     day of the release when the run has none.
//   - An exclusion names its run, contract, reason, account and time and is
//     only ever appended; a return to the run is another entry. An excluded
//     contract is not issued and comes back in the next run of the month.
//   - The release issues exactly the reviewed preview: its fingerprint
//     covers schools, invoice date, rule, exclusions and every amount,
//     recipient and position of the invoices it issues.

// Limits of the review.
const (
	// MaxExclusionReasonLength is the longest reason of an exclusion.
	MaxExclusionReasonLength = 500
)

var (
	// ErrCareExclusionConflict reports an exclusion of an invoice the
	// preview does not issue, or a return of one that is not excluded.
	ErrCareExclusionConflict = errors.New("the preview has no such invoice to exclude or return")
)

// CareRunScope is what a run bills: its schools and invoice date. It is
// part of CareRun.
type CareRunScope struct {
	// SchoolIDs are the schools the run bills, ascending.
	SchoolIDs []int64 `json:"school_ids"`
	// InvoiceDate is the invoice date the release puts on every invoice; nil
	// takes the day of the release.
	InvoiceDate *calendar.Date `json:"invoice_date"`
}

// CareRunStart starts a run: the first day of the month, its schools (none
// takes every school of the Träger) and an optional invoice date.
type CareRunStart struct {
	BillingMonth calendar.Date
	SchoolIDs    []int64
	InvoiceDate  *calendar.Date
}

// CareExclusion is why an invoice of a preview is left out of the release:
// the reason, the account and the time of its entry in the trail.
type CareExclusion struct {
	Reason     string    `json:"reason"`
	AccountID  *int64    `json:"account_id"`
	ExcludedAt time.Time `json:"excluded_at"`
}

// CareRunReviewState is the review of a calculated run. It is part of
// CareRunDetail; a released run leaves it empty.
type CareRunReviewState struct {
	// Schools names the schools of the run.
	Schools []CareSchool `json:"schools"`
	// ExcludedCount and ExcludedCents sum the invoices left out; the
	// preview count and sum leave them out.
	ExcludedCount int   `json:"excluded_count"`
	ExcludedCents int64 `json:"excluded_cents"`
	// Fingerprint names the reviewed state; the release sends it back.
	Fingerprint string `json:"fingerprint"`
}

// CareRunSchool is a school of the Träger a new run of a month may bill;
// OpenRunID names the open run of the month that bills it already.
type CareRunSchool struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	OpenRunID *int64 `json:"open_run_id"`
}

// CareRelease is what the reviewer saw before the release: the count and
// sum of the invoices and the fingerprint of the preview.
type CareRelease struct {
	DocumentCount int
	TotalCents    int64
	Fingerprint   string
}

// CareRunReview reviews a calculated monthly run. Every operation names the
// person of the session and runs in the Träger transaction of its Träger;
// every change and every download is recorded in the run's trail.
type CareRunReview interface {
	SetCareRunInvoiceDate(ctx context.Context, reviewer HolidayReviewer, runID int64, invoiceDate *calendar.Date) (CareRunDetail, error)
	ExcludeCareInvoice(ctx context.Context, reviewer HolidayReviewer, runID, contractID int64, reason string) (CareRunDetail, error)
	IncludeCareInvoice(ctx context.Context, reviewer HolidayReviewer, runID, contractID int64) (CareRunDetail, error)
	ExportCareRunReview(ctx context.Context, reviewer HolidayReviewer, runID int64, format string) (ExportFile, error)
	// CareRunSchools lists the schools of the Träger with the open run of
	// the month that bills each; it names no person.
	CareRunSchools(ctx context.Context, organizationID int64, month calendar.Date) ([]CareRunSchool, error)
}
