package application

import (
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// bookingPartsAfterFunding reconstructs the family's part of each service
// from the issued cost shares. Care payable cents contain reductions, but
// not funding. Both-scope shares take care first, just as the rule chain
// did; their stored remainder belongs to meals. Review removals apply after
// funding, in the same order as the invoice calculation. Holiday reductions
// are netted within each child before its weeks are grouped by accounts.
func bookingPartsAfterFunding(invoice parentbilling.ExportInvoice) []parentbilling.ExportBookingPart {
	parts := slices.Clone(invoice.Parts)
	applyHolidayReductions(invoice.TotalCents, parts)
	var care, meals int64
	for _, part := range parts {
		switch part.Kind {
		case parentbilling.BookingPartCare:
			care += part.AmountCents
		case parentbilling.BookingPartMeal:
			meals += part.AmountCents
		}
	}
	careLeft, mealLeft := care, meals
	for _, share := range invoice.CostShares {
		taken := int64(0)
		if parentbilling.CostShareCoversCare(share.Scope) {
			want := share.AmountCents
			if share.Scope == parentbilling.CostShareScopeBoth && share.ShareBasisPoints != nil {
				want = costShareOf(care, &parentbilling.CareCostShare{ShareBasisPoints: share.ShareBasisPoints})
			}
			taken = min(want, careLeft, share.AmountCents)
			careLeft -= taken
		}
		if parentbilling.CostShareCoversMeal(share.Scope) {
			mealLeft -= min(share.AmountCents-taken, mealLeft)
		}
	}
	applyBookingFunding(parts, parentbilling.BookingPartCare, care, careLeft)
	applyBookingFunding(parts, parentbilling.BookingPartMeal, meals, mealLeft)
	return slices.DeleteFunc(parts, func(part parentbilling.ExportBookingPart) bool { return part.Removed })
}

// applyHolidayReductions nets each child's holiday reductions against that
// child's charges before grouping by accounts. A weekless reduction follows
// the child's weeks in proportion to their charges. A holiday Storno carries
// the negated positions, so netting with the opposite sign reverses the same
// allocation. Monthly positions already contain their reductions.
func applyHolidayReductions(total int64, parts []parentbilling.ExportBookingPart) {
	sign := int64(1)
	if total < 0 {
		sign = -1
	}
	children := map[int64][]int{}
	for i, part := range parts {
		if part.Kind == parentbilling.BookingPartHoliday {
			children[part.ChildID] = append(children[part.ChildID], i)
		}
	}
	for _, indices := range children {
		var net int64
		var charges []int
		var weights []int64
		for _, i := range indices {
			amount := sign * parts[i].AmountCents
			net += amount
			if amount > 0 {
				charges = append(charges, i)
				weights = append(weights, amount)
			}
			parts[i].AmountCents = 0
		}
		if len(charges) == 0 {
			continue
		}
		amounts := splitCents(max(0, net), weights)
		for j, i := range charges {
			parts[i].AmountCents = sign * amounts[j]
		}
	}
}

// applyBookingFunding deducts removals in full after funding and apportions
// the service's remaining amount over its surviving positions, keeping the
// funding and each rounding cent within that service.
func applyBookingFunding(parts []parentbilling.ExportBookingPart, kind string, before, after int64) {
	if before <= 0 || before == after {
		return
	}
	var weights []int64
	for _, part := range parts {
		if part.Kind != kind || part.AmountCents <= 0 {
			continue
		}
		if part.Removed {
			after -= part.AmountCents
			continue
		}
		weights = append(weights, part.AmountCents)
	}
	if len(weights) == 0 {
		return
	}
	amounts := splitCents(max(0, after), weights)
	i := 0
	for j := range parts {
		if parts[j].Kind == kind && parts[j].AmountCents > 0 && !parts[j].Removed {
			parts[j].AmountCents = amounts[i]
			i++
		}
	}
}
