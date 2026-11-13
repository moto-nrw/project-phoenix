package application

import (
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// applyMealCostShares lets the cost bearer shares that cover the Mensa take
// their part of the Mensa positions of the drafts (#4686, ADR 0168 point
// 12). No rule reduces the Mensa, so the part comes off its full amount. It
// runs after the Mensa joined the invoices and after a split onto an
// invoice of its own (ADR 0167), in the order of the kinds like the chain:
//
//   - a share in basis points takes its part of the Mensa, rounded half up
//     to the cent; a share of both scopes so took its part of the modules
//     in the chain, each part rounded on its own;
//   - a fixed share takes what its monthly amount leaves after what it took
//     of the contract in the month already: of the modules in the chain,
//     on another invoice of the run or on an issued invoice of the month;
//   - each takes at most what is left of the Mensa, so the family never
//     pays less than nothing.
//
// The part joins the share of the same kind on the invoice, so the invoice
// names each cost bearer once.
func applyMealCostShares(month *careMonth, shares []parentbilling.CareCostShare, billingMonth calendar.Date,
	invoiced map[int64]parentbilling.CareInvoice,
) {
	for i := range month.invoices {
		invoice := &month.invoices[i]
		var meals int64
		for _, position := range invoice.MealPositions {
			meals += position.AmountCents
		}
		left := meals
		for _, kind := range parentbilling.CostShareKinds {
			share := careCostShareInMonth(shares, invoice.ContractID, kind, billingMonth)
			if share == nil || !parentbilling.CostShareCoversMeal(share.Scope) {
				continue
			}
			want := costShareOf(meals, share)
			if share.ShareCents != nil {
				want -= fixedShareTaken(month, invoiced, invoice.ContractID, share)
			}
			part := min(want, left)
			if part <= 0 {
				continue
			}
			addInvoiceCostShare(invoice, share, part)
			left -= part
		}
		invoice.TotalCents -= meals - left
	}
}

// fixedShareTaken is what a fixed share already took of the contract in the
// month: on the drafts of the run and on the issued invoice of the month.
func fixedShareTaken(month *careMonth, invoiced map[int64]parentbilling.CareInvoice, contractID int64,
	share *parentbilling.CareCostShare,
) int64 {
	var taken int64
	count := func(invoice parentbilling.CareInvoice) {
		for _, part := range invoice.CostShares {
			if part.Kind == share.Kind && part.CostBearerID == share.CostBearerID {
				taken += part.AmountCents
			}
		}
	}
	for _, invoice := range month.invoices {
		if invoice.ContractID == contractID {
			count(invoice)
		}
	}
	if issued, ok := invoiced[contractID]; ok {
		count(issued)
	}
	return taken
}

// addInvoiceCostShare adds a part of a share to the invoice: to the part of
// the same kind it already carries, else as a new part in the order of the
// kinds.
func addInvoiceCostShare(invoice *parentbilling.CareInvoice, share *parentbilling.CareCostShare, part int64) {
	for i := range invoice.CostShares {
		if invoice.CostShares[i].Kind == share.Kind {
			invoice.CostShares[i].AmountCents += part
			return
		}
	}
	invoice.CostShares = append(invoice.CostShares, parentbilling.CareInvoiceCostShare{
		Kind: share.Kind, CostBearerID: share.CostBearerID, CostBearerName: share.CostBearerName,
		ShareBasisPoints: share.ShareBasisPoints, ShareCents: share.ShareCents, Scope: share.Scope, AmountCents: part,
	})
	slices.SortStableFunc(invoice.CostShares, func(a, b parentbilling.CareInvoiceCostShare) int {
		return slices.Index(parentbilling.CostShareKinds, a.Kind) - slices.Index(parentbilling.CostShareKinds, b.Kind)
	})
}
