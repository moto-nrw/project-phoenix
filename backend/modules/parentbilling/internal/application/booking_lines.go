package application

import (
	"math/big"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

// The lines of an exported document (#4821): one per revenue account and
// cost centers of its positions, the same lines in the DATEV file and in a
// Sollstellung that maps an account field. The lines of a document sum to
// its total: the total is split by the weight of each group of positions,
// using each care position after its reductions and cost shares. Any
// remaining invoice-wide adjustment falls on its groups in proportion. A
// rounding cent goes to the group with the largest remainder, the earlier
// one on a tie.
//
// A position issued before #4821, or one its release found no revenue
// account for, books on the values of the day of the export; the file
// counts it (ExportProtocolEntry.ResolvedPositions). A frozen position
// never takes a value of today.

// bookingLine is one line of a document: where it books and its amount,
// negative for a Storno or credit.
type bookingLine struct {
	accounts    parentbilling.PositionAccounts
	amountCents int64
}

// booksFallback matches a frozen fallback position to an emitted booking.
// A position without a revenue account uses the standard resolved at export.
func (line bookingLine) booksFallback(part parentbilling.ExportBookingPart) bool {
	return line.amountCents != 0 && line.accounts.CostCenter == part.Accounts.CostCenter &&
		line.accounts.CostCenter2 == part.Accounts.CostCenter2 &&
		(part.Accounts.RevenueAccount == "" || line.accounts.RevenueAccount == part.Accounts.RevenueAccount)
}

// lineKey groups the parts of a document by where they book.
type lineKey struct {
	revenue, costCenter, costCenter2 string
}

// documentLines splits the total of a document onto where its positions
// book and counts the positions resolved at export. A document without a
// position with an amount books its total where its first position books,
// on the standard account without any.
func (r bookingResolver) documentLines(invoice parentbilling.ExportInvoice) ([]bookingLine, int) {
	if invoice.TotalCents == 0 {
		return nil, 0
	}
	parts := bookingPartsAfterFunding(invoice)
	keys, weights, resolved := r.groupParts(invoice.TotalCents, parts)
	if len(keys) == 0 {
		// No position with an amount: the first position names where the
		// document books, the standard without one.
		accounts, fromToday := parentbilling.PositionAccounts{RevenueAccount: r.standard}, true
		if len(parts) > 0 {
			accounts, fromToday = r.partAccounts(parts[0])
		}
		accounts.Frozen = false
		return []bookingLine{{accounts: accounts, amountCents: invoice.TotalCents}}, boolCount(fromToday)
	}
	amounts := splitCents(invoice.TotalCents, weights)
	lines := make([]bookingLine, 0, len(keys))
	for i, key := range keys {
		if amounts[i] == 0 {
			continue
		}
		lines = append(lines, bookingLine{amountCents: amounts[i], accounts: parentbilling.PositionAccounts{
			RevenueAccount: key.revenue, CostCenter: key.costCenter, CostCenter2: key.costCenter2}})
	}
	return lines, resolved
}

// groupParts groups the parts by where they book, in the order they come.
// The weight of a group is the sum of its charges; a negative document with
// credits (a holiday Storno or credit) weighs by its credits, so a Storno
// relieves where the invoice's charges booked and not where its negated
// discounts did (#6624). A monthly Storno carries its invoice's charges.
func (r bookingResolver) groupParts(total int64, parts []parentbilling.ExportBookingPart) ([]lineKey, []int64, int) {
	credits := false
	for _, part := range parts {
		credits = credits || part.AmountCents < 0
	}
	charges := total >= 0 || !credits
	var keys []lineKey
	var weights []int64
	index := map[lineKey]int{}
	resolved := 0
	for _, part := range parts {
		weight := part.AmountCents
		if !charges {
			weight = -weight
		}
		if weight <= 0 {
			continue
		}
		accounts, fromToday := r.partAccounts(part)
		if fromToday {
			resolved++
		}
		key := lineKey{accounts.RevenueAccount, accounts.CostCenter, accounts.CostCenter2}
		i, seen := index[key]
		if !seen {
			i = len(keys)
			index[key] = i
			keys, weights = append(keys, key), append(weights, 0)
		}
		weights[i] += weight
	}
	return keys, weights, resolved
}

// partAccounts is where a part books: its frozen values, the values of
// today for a position without them. A frozen position without a revenue
// account (its release found no standard) takes today's standard and
// keeps its cost centers.
func (r bookingResolver) partAccounts(part parentbilling.ExportBookingPart) (parentbilling.PositionAccounts, bool) {
	if part.Accounts.Frozen {
		if part.Accounts.RevenueAccount != "" {
			return part.Accounts, false
		}
		accounts := part.Accounts
		accounts.RevenueAccount = r.standard
		return accounts, true
	}
	switch part.Kind {
	case parentbilling.BookingPartCare:
		return r.care(part.SchoolID, part.CareOfferingID, part.Day), true
	case parentbilling.BookingPartMeal:
		return r.meal(part.SchoolID, part.Day), true
	case parentbilling.BookingPartHolidayMeal:
		accounts := r.holiday(part.Day)
		accounts.RevenueAccount = r.holidayMealAccount()
		return accounts, true
	}
	return r.holiday(part.Day), true
}

// costCenterFallbacks names the weeks of the document whose net positions
// contribute to an exported line on the KOST1 of the holiday care (#6624),
// once per week, for the protocol of the file. A line without a week books like the first
// week of its child and adds no entry, unless the document has no week on
// the fallback at all.
func costCenterFallbacks(invoice parentbilling.ExportInvoice, lines []bookingLine) []parentbilling.CostCenterFallback {
	var fallbacks, weekless []parentbilling.CostCenterFallback
	seen := map[[3]string]bool{}
	for _, part := range bookingPartsAfterFunding(invoice) {
		if part.CostCenterFallback == "" || part.AmountCents == 0 {
			continue
		}
		if !slices.ContainsFunc(lines, func(line bookingLine) bool {
			return line.booksFallback(part)
		}) {
			continue
		}
		entry := parentbilling.CostCenterFallback{Number: invoice.Number, Reason: part.CostCenterFallback, CostCenter: part.Accounts.CostCenter}
		if part.WeekStartsOn == nil {
			if len(weekless) == 0 {
				weekless = append(weekless, entry)
			}
			continue
		}
		key := [3]string{part.WeekStartsOn.String(), entry.Reason, entry.CostCenter}
		if seen[key] {
			continue
		}
		seen[key] = true
		entry.WeekStartsOn, entry.WeekEndsOn = part.WeekStartsOn, part.WeekEndsOn
		fallbacks = append(fallbacks, entry)
	}
	if len(fallbacks) == 0 {
		return weekless
	}
	return fallbacks
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

// splitCents splits total by the positive weights: the floor of each share
// of |total|, the cents left to the largest remainders, then the sign of
// total. The shares sum to total.
func splitCents(total int64, weights []int64) []int64 {
	sign, magnitude := int64(1), total
	if total < 0 {
		sign, magnitude = -1, -total
	}
	sum := new(big.Int)
	for _, weight := range weights {
		sum.Add(sum, big.NewInt(weight))
	}
	shares := make([]int64, len(weights))
	remainders := make([]*big.Int, len(weights))
	left := magnitude
	for i, weight := range weights {
		product := new(big.Int).Mul(big.NewInt(magnitude), big.NewInt(weight))
		quotient, remainder := new(big.Int).QuoRem(product, sum, new(big.Int))
		shares[i], remainders[i] = quotient.Int64(), remainder
		left -= shares[i]
	}
	for ; left > 0; left-- {
		largest := 0
		for i := range remainders {
			if remainders[i].Cmp(remainders[largest]) > 0 {
				largest = i
			}
		}
		shares[largest]++
		remainders[largest] = big.NewInt(-1)
	}
	for i := range shares {
		shares[i] *= sign
	}
	return shares
}
