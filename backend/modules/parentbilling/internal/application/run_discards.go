package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// Discarding an empty open run (#6626; holiday runs #6628). An open run stores nothing it would
// issue: its invoices or notices are a preview computed on every read
// (ADR 0119 point 2, ADR 0182). When that preview has nothing to issue and
// nothing to settle, the run can never be released, so it would hold its
// schools and month, or the year, for good. The Abrechnung discards it
// instead: the run stays with status "discarded", its time and a trail
// entry naming who; nothing is deleted.
//
// Invariants:
//   - Only a calculated run is discarded; a released one never (the trigger
//     refuses every change of a released or discarded run as well).
//   - A run with content is never discarded: the preview is computed under
//     the lock of the release (the monthly billing of the Träger, then the
//     run), so a price, rule, cost share or release that commits first is
//     seen, and one that waits finds the run discarded.
//   - A discarded run never changes and reads no contract or registration
//     again.

// runEventDiscard is the trail action of a discard.
const runEventDiscard = "discard"

// closedRunError is the refusal of a change of a run that is no longer
// calculated: a discarded run has its own (#6626).
func closedRunError(status string) error {
	if status == parentbilling.RunDiscarded {
		return parentbilling.ErrBillingRunDiscarded
	}
	return parentbilling.ErrBillingRunReleased
}

var _ parentbilling.CareRunDiscard = (*CareBilling)(nil)

// emptyCareReview tells whether the preview of a monthly run has nothing to
// issue and nothing to settle: no invoice, also no excluded one, no contract
// of a free month the release would keep, no booking without a price and no
// child held back for a duplicate contract.
func emptyCareReview(review reviewedMonth) bool {
	return len(review.invoices) == 0 && len(review.excluded) == 0 && review.free == 0 &&
		len(review.unpriced) == 0 && len(review.duplicates) == 0
}

// DiscardCareRun discards a calculated monthly run whose preview is empty.
// It holds the monthly billing of the Träger and the run like the release.
// The caller rolls back when this fails.
func (c *CareBilling) DiscardCareRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64) (parentbilling.CareRunDetail, error) {
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
	_, review, err := c.reviewed(ctx, reviewer.OrganizationID, run)
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	if !emptyCareReview(review) {
		return parentbilling.CareRunDetail{}, parentbilling.ErrBillingRunNotEmpty
	}
	discarded, err := c.store.MarkCareRunDiscarded(ctx, reviewer.OrganizationID, run.ID)
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	if err := c.store.InsertCareRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventDiscard,
		map[string]any{"school_ids": run.SchoolIDs}); err != nil {
		return parentbilling.CareRunDetail{}, fmt.Errorf("parent billing: record care run discard: %w", err)
	}
	return c.detail(ctx, reviewer, discarded)
}

// discardedDetail shows a discarded monthly run: its month and schools,
// without a preview, so it reads no contract and no person again.
func (c *CareBilling) discardedDetail(ctx context.Context, organizationID int64, detail parentbilling.CareRunDetail) (parentbilling.CareRunDetail, error) {
	detail.Invoices = []parentbilling.CareInvoice{}
	detail.DueDates = []parentbilling.RunDueDate{}
	detail.CostBearers = []parentbilling.CareCostBearerTotal{}
	schools, err := c.runSchoolNames(ctx, organizationID, detail.SchoolIDs)
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	detail.Schools = schools
	return detail, nil
}

// emptyNoticePreview tells whether a notice run has nothing to issue and
// nothing to settle: no notice, no booking without a price and no child
// held back. Contracts whose notice still holds or that a monthly invoice
// bills issue nothing.
func emptyNoticePreview(preview noticePreview) bool {
	return len(preview.notices) == 0 && len(preview.unpriced) == 0 && len(preview.duplicates) == 0
}

// DiscardCareNoticeRun discards a calculated notice run whose preview is
// empty, under the lock of the release. The caller rolls back when this
// fails.
func (n *CareFeeNotices) DiscardCareNoticeRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64) (parentbilling.CareNoticeRunDetail, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.CareNoticeRunDetail{}, err
	}
	if runID <= 0 {
		return parentbilling.CareNoticeRunDetail{}, parentbilling.ErrCareNoticeRunNotFound
	}
	if err := n.billing.store.LockCareBilling(ctx, reviewer.OrganizationID); err != nil {
		return parentbilling.CareNoticeRunDetail{}, fmt.Errorf("parent billing: hold the monthly billing: %w", err)
	}
	run, err := n.store.CareNoticeRun(ctx, reviewer.OrganizationID, runID, true)
	if err != nil {
		return parentbilling.CareNoticeRunDetail{}, err
	}
	if run.Status != parentbilling.RunCalculated {
		return parentbilling.CareNoticeRunDetail{}, closedRunError(run.Status)
	}
	preview, err := n.preview(ctx, reviewer.OrganizationID, run)
	if err != nil {
		return parentbilling.CareNoticeRunDetail{}, err
	}
	if !emptyNoticePreview(preview) {
		return parentbilling.CareNoticeRunDetail{}, parentbilling.ErrBillingRunNotEmpty
	}
	discarded, err := n.store.MarkCareNoticeRunDiscarded(ctx, reviewer.OrganizationID, run.ID)
	if err != nil {
		return parentbilling.CareNoticeRunDetail{}, err
	}
	if err := n.store.InsertCareNoticeRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventDiscard,
		map[string]any{"school_ids": run.SchoolIDs}); err != nil {
		return parentbilling.CareNoticeRunDetail{}, fmt.Errorf("parent billing: record notice run discard: %w", err)
	}
	return n.detail(ctx, reviewer.OrganizationID, discarded)
}

// emptyHolidayPreview tells whether a holiday run has nothing to issue and
// nothing to settle (#6628): no invoice, no cancelled invoice to issue
// again and no confirmed registration still waiting for its contribution.
// Corrections the run recorded for a registration it no longer bills issue
// nothing.
func emptyHolidayPreview(current preview) bool {
	return len(current.documents) == 0 && current.unbilled == 0
}

// DiscardBillingRun discards a calculated holiday run whose preview is
// empty (#6628). It takes the locks of the release in its order: the run,
// then the invoices of its period, so a release, a cancellation that asks
// for a new invoice or a sibling change that commits first is seen, and one
// that waits finds the run discarded. The caller rolls back when this
// fails.
func (b *BillingRuns) DiscardBillingRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64) (parentbilling.BillingRunDetail, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	run, err := b.lockCalculated(ctx, reviewer.OrganizationID, runID)
	if err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	if err := b.store.LockPeriodBilling(ctx, reviewer.OrganizationID, run.Service, run.HolidayPeriodID); err != nil {
		return parentbilling.BillingRunDetail{}, fmt.Errorf("parent billing: hold the invoices of the period: %w", err)
	}
	current, err := b.preview(ctx, reviewer, run)
	if err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	if !emptyHolidayPreview(current) {
		return parentbilling.BillingRunDetail{}, parentbilling.ErrBillingRunNotEmpty
	}
	discarded, err := b.store.MarkRunDiscarded(ctx, reviewer.OrganizationID, run.ID)
	if err != nil {
		return parentbilling.BillingRunDetail{}, err
	}
	if err := b.store.InsertRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventDiscard,
		map[string]any{"holiday_period_id": run.HolidayPeriodID}); err != nil {
		return parentbilling.BillingRunDetail{}, fmt.Errorf("parent billing: record billing run discard: %w", err)
	}
	return b.detail(ctx, reviewer, discarded)
}

// discardedHolidayDetail shows a discarded holiday run: its period without
// a preview, so it reads no registration and no person again.
func discardedHolidayDetail(detail *parentbilling.BillingRunDetail) {
	detail.Documents = []parentbilling.BillingDocument{}
	detail.DueDates = []parentbilling.RunDueDate{}
}
