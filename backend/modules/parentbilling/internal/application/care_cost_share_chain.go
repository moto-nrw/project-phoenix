package application

import (
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The last step of the monthly rule chain (#4687, ADR 0169): the cost
// bearer shares. They are no reduction: the contribution stays what the
// rules make it, and the cap of a later run of the month counts it in full.
// The shares only split who pays it. So they run after the cap, on what the
// family would pay, and leave the payable cents of the positions alone.
//
// A share holds for the month like an evidence (ADR 0164 point 6): the one
// valid on the first, else the first that starts within the month. The
// kinds apply in the order of parentbilling.CostShareKinds. A share in basis
// points is a part of the whole contribution, rounded half up to the cent; a
// fixed share is its amount. Each takes at most what is left, so the shares
// never take more than the contribution and the family's rest is never
// negative.
//
// A share covers the modules, the Mensa or both (#4686, ADR 0168 point 12).
// The chain runs on the modules only, so it applies the shares that cover
// the care; applyMealCostShares applies the Mensa part after the Mensa
// joined the invoices.

// careCostShareInMonth is the share of a kind that holds for a contract in
// the month.
func careCostShareInMonth(shares []parentbilling.CareCostShare, contractID int64, kind string, month calendar.Date) *parentbilling.CareCostShare {
	end := monthEnd(month)
	var found *parentbilling.CareCostShare
	for i := range shares {
		share := &shares[i]
		if share.ContractID != contractID || share.Kind != kind || share.ValidFrom.After(end) || share.ValidUntil.Before(month) {
			continue
		}
		if found == nil || share.ValidFrom.Before(found.ValidFrom) {
			found = share
		}
	}
	return found
}

// costShareOf is what a share takes of a contribution before the limit of
// what is left.
func costShareOf(contribution int64, share *parentbilling.CareCostShare) int64 {
	switch {
	case contribution <= 0:
		return 0
	case share.ShareBasisPoints != nil:
		return (contribution*int64(*share.ShareBasisPoints) + parentbilling.BasisPointsWhole/2) / parentbilling.BasisPointsWhole
	case share.ShareCents != nil:
		return *share.ShareCents
	}
	return 0
}

// applyCostShares splits the total of a draft between the cost bearers and
// the family. An issued invoice keeps the shares it copied.
func (e *careChainEntry) applyCostShares(in careChainInput) {
	if e.issued {
		return
	}
	contribution := e.invoice.TotalCents
	left := contribution
	shares := []parentbilling.CareInvoiceCostShare{}
	for _, kind := range parentbilling.CostShareKinds {
		share := careCostShareInMonth(in.costShares, e.invoice.ContractID, kind, in.month)
		if share == nil || !parentbilling.CostShareCoversCare(share.Scope) {
			continue
		}
		part := min(costShareOf(contribution, share), left)
		if part <= 0 {
			continue
		}
		shares = append(shares, parentbilling.CareInvoiceCostShare{
			Kind: share.Kind, CostBearerID: share.CostBearerID, CostBearerName: share.CostBearerName,
			ShareBasisPoints: share.ShareBasisPoints, ShareCents: share.ShareCents, Scope: share.Scope, AmountCents: part,
		})
		left -= part
	}
	e.invoice.CostShares = shares
	e.invoice.TotalCents = left
}

// careCostBearerTotals sums what each cost bearer pays of the invoices, in
// the order the invoices name them first. Excluded invoices do not count;
// cancelled ones retain the export entry without adding to counts or amounts.
func careCostBearerTotals(invoices []parentbilling.CareInvoice, cancelled map[int64]bool) []parentbilling.CareCostBearerTotal {
	totals := []parentbilling.CareCostBearerTotal{}
	index := map[int64]int{}
	for _, invoice := range invoices {
		if invoice.Exclusion != nil {
			continue
		}
		counted := map[int64]bool{}
		for _, share := range invoice.CostShares {
			i, ok := index[share.CostBearerID]
			if !ok {
				i = len(totals)
				index[share.CostBearerID] = i
				totals = append(totals, parentbilling.CareCostBearerTotal{CostBearerID: share.CostBearerID, CostBearerName: share.CostBearerName})
			}
			if invoice.ID != 0 && cancelled[invoice.ID] {
				continue
			}
			if !counted[share.CostBearerID] {
				totals[i].InvoiceCount++
				counted[share.CostBearerID] = true
			}
			totals[i].AmountCents += share.AmountCents
		}
	}
	return totals
}

// cancelledInvoices names the invoices of a released run a Storno took
// back; a calculated run has none.
func cancelledInvoices(cancellations []parentbilling.CareCancellation) map[int64]bool {
	cancelled := make(map[int64]bool, len(cancellations))
	for _, cancellation := range cancellations {
		cancelled[cancellation.InvoiceID] = true
	}
	return cancelled
}
