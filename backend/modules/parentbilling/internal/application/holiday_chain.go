package application

import (
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The rule chain of the holiday contribution (ADR 0088 section 4, ADR 0115):
// base price, sibling rule, then the exemption or the cost takeover by a
// cost bearer (#5627). It is pure; the caller reads the rules, the settings
// and the bookings and decides whether the exemption or which cost bearer
// applies.

// holidayRules are the rules of one Träger valid on one day and its choices
// in the chain (#5664); without a choice the default applies.
type holidayRules struct {
	sibling   *parentbilling.ContributionRule
	exemption *parentbilling.ContributionRule
	chain     parentbilling.HolidayChainSettings
}

// chainOrDefault fills in the default of a choice the caller left open.
func chainOrDefault(chain parentbilling.HolidayChainSettings) parentbilling.HolidayChainSettings {
	if chain.SiblingRank == "" {
		chain.SiblingRank = parentbilling.SiblingRankHighestContribution
	}
	if chain.CostTakeoverOrder == "" {
		chain.CostTakeoverOrder = parentbilling.CostTakeoverAfterDiscount
	}
	return chain
}

// validHolidayRules picks the holiday rules valid on day. Rules of one kind
// never overlap, so at most one of each kind is valid.
func validHolidayRules(rules []parentbilling.ContributionRule, day calendar.Date) holidayRules {
	var valid holidayRules
	for i := range rules {
		rule := &rules[i]
		if rule.Service != parentbilling.ServiceHoliday || day.Before(rule.ValidFrom) || (rule.ValidUntil != nil && day.After(*rule.ValidUntil)) {
			continue
		}
		switch rule.Kind {
		case parentbilling.RuleSibling:
			valid.sibling = rule
		case parentbilling.RuleBenefitsExemption:
			valid.exemption = rule
		}
	}
	return valid
}

// firstHolidayDay is the first requested day, including waiting bookings.
// Eligibility can apply before a place is available; pricing still filters
// waiting bookings separately. Without a request no rule applies.
func firstHolidayDay(children []parentbilling.HolidayChild) calendar.Date {
	var first calendar.Date
	for _, child := range children {
		for _, booking := range child.Bookings {
			day := booking.WeekStartsOn
			if len(booking.Days) > 0 {
				day = slices.Min(booking.Days)
			}
			if first == "" || day.Before(first) {
				first = day
			}
		}
	}
	return first
}

// computeHolidayContribution runs the chain over the children of one
// registration. exemption applies the exemption rule when there is one; a
// cost bearer takes over what is left after the sibling rule (#5627). The
// caller never passes both.
func computeHolidayContribution(children []parentbilling.HolidayChild, rules holidayRules, exemption bool, bearer *parentbilling.CostBearer) parentbilling.HolidayContribution {
	return computeHolidaySiblings([]holidayMember{{children: children, exemption: exemption, bearer: bearer}}, rules)[0]
}

// holidayMember is one registration of a sibling group (#5628) as the chain
// sees it: its children, whether its exemption applies and the cost bearer
// that takes over its contribution (#5627).
type holidayMember struct {
	children  []parentbilling.HolidayChild
	exemption bool
	bearer    *parentbilling.CostBearer
}

// computeHolidaySiblings runs the chain over the registrations of a sibling
// group, in the order they were sent, and returns one contribution per
// registration. The sibling rule ranks the children of all of them
// together; the exemption and the cost takeover stay a matter of each
// registration. The exemption applies after the sibling discount, the cost
// takeover too unless the Träger lets the cost bearer pay the full price
// (#5664): its children then keep their rank but get no discount.
func computeHolidaySiblings(members []holidayMember, rules holidayRules) []parentbilling.HolidayContribution {
	chain := chainOrDefault(rules.chain)
	results := make([]parentbilling.HolidayContribution, len(members))
	var ranked []rankedChild
	for m, member := range members {
		member.children = placedHolidayChildren(member.children)
		results[m] = parentbilling.HolidayContribution{
			Children:           make([]parentbilling.HolidayChildContribution, len(member.children)),
			ExemptionAvailable: rules.exemption != nil,
			BenefitsExemption:  member.exemption && rules.exemption != nil,
			Chain:              chain,
		}
		for i, child := range member.children {
			results[m].Children[i] = holidayBaseLines(child)
		}
		fullPrice := member.bearer != nil && !results[m].BenefitsExemption &&
			chain.CostTakeoverOrder == parentbilling.CostTakeoverBeforeDiscount
		ranked = append(ranked, formOrder(member.children, results[m].Children, fullPrice)...)
	}
	applySiblingRule(ranked, rules.sibling, chain.SiblingRank)
	for m, member := range members {
		finishHolidayContribution(&results[m], rules, member.bearer)
	}
	return results
}

// finishHolidayContribution applies the exemption or the cost takeover of
// one registration to what the sibling rule left and sums its children.
func finishHolidayContribution(result *parentbilling.HolidayContribution, rules holidayRules, bearer *parentbilling.CostBearer) {
	switch {
	case result.BenefitsExemption:
		for i := range result.Children {
			applyExemption(&result.Children[i], rules.exemption)
		}
	case bearer != nil:
		result.CostBearerID, result.CostBearerName = &bearer.ID, bearer.Name
		for i := range result.Children {
			applyCostTakeover(&result.Children[i], bearer)
		}
	}
	for i := range result.Children {
		result.TotalCents += result.Children[i].TotalCents
	}
}

// rankedChild is a child as the sibling rule ranks it: its contribution, its
// age and whether a cost bearer pays its full price (#5664).
type rankedChild struct {
	contribution *parentbilling.HolidayChildContribution
	birthDate    calendar.Date
	ageRank      int
	fullPrice    bool
}

// formOrder points to the children of one registration in the order of the
// form. contributions run parallel to children.
func formOrder(children []parentbilling.HolidayChild, contributions []parentbilling.HolidayChildContribution, fullPrice bool) []rankedChild {
	ordered := make([]rankedChild, len(contributions))
	for i := range contributions {
		ordered[i] = rankedChild{
			contribution: &contributions[i], birthDate: children[i].BirthDate, ageRank: children[i].AgeRank, fullPrice: fullPrice,
		}
	}
	slices.SortStableFunc(ordered, func(a, b rankedChild) int { return a.contribution.Position - b.contribution.Position })
	return ordered
}

// olderFirst orders two children by age; a child of unknown age comes after
// the others. Children of one calculation either all have a date of birth or
// all the age rank of a quote.
func olderFirst(a, b rankedChild) int {
	known := func(child rankedChild) bool { return child.birthDate != "" || child.ageRank > 0 }
	switch {
	case known(a) && !known(b):
		return -1
	case !known(a) && known(b):
		return 1
	case a.birthDate != "" && b.birthDate != "":
		return a.birthDate.Compare(b.birthDate)
	}
	return a.ageRank - b.ageRank
}

// placedHolidayChildren leaves out the bookings on the waiting list (#4699):
// they are not billed, and a child with only such bookings ranks as a child
// without a booking.
func placedHolidayChildren(children []parentbilling.HolidayChild) []parentbilling.HolidayChild {
	placed := make([]parentbilling.HolidayChild, len(children))
	for i, child := range children {
		child.Bookings = slices.DeleteFunc(slices.Clone(child.Bookings), func(booking parentbilling.HolidayBooking) bool { return booking.Waitlisted })
		placed[i] = child
	}
	return placed
}

// holidayBaseLines writes one base line per booked week, in the order of
// the weeks.
func holidayBaseLines(child parentbilling.HolidayChild) parentbilling.HolidayChildContribution {
	bookings := slices.Clone(child.Bookings)
	slices.SortStableFunc(bookings, func(a, b parentbilling.HolidayBooking) int { return a.WeekStartsOn.Compare(b.WeekStartsOn) })
	contribution := parentbilling.HolidayChildContribution{
		ChildID: child.ID, Position: child.Position, Lines: make([]parentbilling.HolidayContributionLine, 0, len(bookings)+2),
	}
	for _, booking := range bookings {
		startsOn, endsOn := booking.WeekStartsOn, booking.WeekEndsOn
		line := parentbilling.HolidayContributionLine{
			Kind: parentbilling.LineBase, AmountCents: booking.PriceCents,
			WeekStartsOn: &startsOn, WeekEndsOn: &endsOn, TimeModelName: booking.TimeModelName, WeekID: booking.WeekID,
		}
		if booking.BookedDays > 0 {
			days := booking.BookedDays
			line.BookedDays = &days
		}
		contribution.Lines = append(contribution.Lines, line)
		contribution.TotalCents += booking.PriceCents
	}
	return contribution
}

// applySiblingRule lowers the contribution of every child from the rule's
// rank on. children come in the order of the registrations, then of the
// form. Children with a booking count; the first child is the one with the
// highest base price or, if the Träger ranks so, the oldest (#5664). A tie
// keeps the given order, so the discount never lands on the cheaper child by
// chance. A child whose full price a cost bearer pays holds its rank and gets
// no discount.
func applySiblingRule(children []rankedChild, rule *parentbilling.ContributionRule, siblingRank string) {
	if rule == nil || rule.SiblingFromChild == nil {
		return
	}
	ranked := make([]rankedChild, 0, len(children))
	for _, child := range children {
		if len(child.contribution.Lines) > 0 {
			ranked = append(ranked, child)
		}
	}
	slices.SortStableFunc(ranked, func(a, b rankedChild) int {
		if siblingRank == parentbilling.SiblingRankOldestChild {
			return olderFirst(a, b)
		}
		switch {
		case a.contribution.TotalCents > b.contribution.TotalCents:
			return -1
		case a.contribution.TotalCents < b.contribution.TotalCents:
			return 1
		}
		return 0
	})
	for index, entry := range ranked {
		rank := index + 1
		if rank < *rule.SiblingFromChild || entry.fullPrice {
			continue
		}
		child := entry.contribution
		discount := siblingDiscount(child.TotalCents, rule)
		if discount == 0 {
			continue
		}
		child.Lines = append(child.Lines, parentbilling.HolidayContributionLine{
			Kind: parentbilling.LineSibling, AmountCents: -discount, ChildRank: &rank,
			DiscountBasisPoints: rule.DiscountBasisPoints, DiscountCents: rule.DiscountCents,
		})
		child.TotalCents -= discount
	}
}

// siblingDiscount is the discount of the rule on base, rounded half up to
// whole cent and never more than base.
func siblingDiscount(base int64, rule *parentbilling.ContributionRule) int64 {
	if base <= 0 {
		return 0
	}
	var discount int64
	switch {
	case rule.DiscountBasisPoints != nil:
		discount = (base*int64(*rule.DiscountBasisPoints) + parentbilling.BasisPointsWhole/2) / parentbilling.BasisPointsWhole
	case rule.DiscountCents != nil:
		discount = *rule.DiscountCents
	}
	return min(discount, base)
}

// applyExemption frees the child from what is left of its contribution and
// writes the rule's note, even when nothing is left.
func applyExemption(child *parentbilling.HolidayChildContribution, rule *parentbilling.ContributionRule) {
	// A zero line persists the confirmed exemption even while every week
	// waits. Otherwise the stored contribution loses that choice on read.
	child.Lines = append(child.Lines, parentbilling.HolidayContributionLine{
		Kind: parentbilling.LineBenefitsExemption, AmountCents: -child.TotalCents, Note: rule.Note,
	})
	child.TotalCents = 0
}

// applyCostTakeover lets the cost bearer pay what is left of the child's
// contribution and copies its ID and name into the line, even when nothing
// is left (#5627).
func applyCostTakeover(child *parentbilling.HolidayChildContribution, bearer *parentbilling.CostBearer) {
	// A zero line persists the confirmed cost bearer even while every week
	// waits (#4699): the stored contribution names it only in its lines, and
	// a later move up would otherwise bill the family.
	id := bearer.ID
	child.Lines = append(child.Lines, parentbilling.HolidayContributionLine{
		Kind: parentbilling.LineCostTakeover, AmountCents: -child.TotalCents, CostBearerID: &id, CostBearerName: bearer.Name,
	})
	child.TotalCents = 0
}
