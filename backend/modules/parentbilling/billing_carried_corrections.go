package parentbilling

import "errors"

// The corrections a reissued invoice carries over (#5663, ADR 0119 addendum).
// The preview of the next run proposes every correction of the cancelled
// invoice it replaces, with the same amount and reason and its origin. The
// Abrechnung changes or removes each before the release. A credit of the
// cancelled invoice is a document of its own: the preview names it and does
// not carry it.

// ErrCarriedCorrectionNotFound reports a position that is no proposal of the
// preview of the run.
var ErrCarriedCorrectionNotFound = errors.New("the correction is not proposed in this billing run")

// BillingCarriedCorrection is a correction of a cancelled invoice the preview
// proposes. PositionID is its position on the cancelled invoice and names the
// proposal. AmountCents is what the draft applies; it differs from
// SourceAmountCents only when the proposal stops at zero, because the
// contribution of the child is lower now than on the cancelled invoice.
type BillingCarriedCorrection struct {
	PositionID        int64  `json:"position_id"`
	SourceNumber      string `json:"source_number"`
	AmountCents       int64  `json:"amount_cents"`
	SourceAmountCents int64  `json:"source_amount_cents"`
	Reason            string `json:"reason"`
	// ChildID and SourceDocumentID place the proposal; TotalBeforeCents is the
	// child's amount before it, which the history of the release records.
	ChildID          int64 `json:"-"`
	SourceDocumentID int64 `json:"-"`
	TotalBeforeCents int64 `json:"-"`
}

// BillingCarriedCorrectionChange replaces a proposal with a correction of the
// run: the new amount of the child and its reason.
type BillingCarriedCorrectionChange struct {
	TotalCents int64
	Reason     string
}
